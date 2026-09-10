// Package events wraps NATS JetStream for node-state events and repair jobs.
// docs/06-scheduler-and-repair.md, docs/02-system-architecture.md.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	StreamNodeEvents = "NODE_EVENTS"
	StreamRepairJobs = "REPAIR_JOBS"

	SubjNodeOnline  = "node.online"
	SubjNodeSuspect = "node.suspect"
	SubjNodeOffline = "node.offline"

	SubjRepairCritical = "repair.critical"
	SubjRepairHigh     = "repair.high"
	SubjRepairNormal   = "repair.normal"

	nodeEventsMaxAge  = 24 * time.Hour
	repairJobsMaxAge  = 24 * time.Hour
	defaultAckWait    = 2 * time.Minute
	defaultMaxDeliver = 50
)

// NodeEvent is published on online/suspect/offline transitions.
type NodeEvent struct {
	NodeID uuid.UUID `json:"node_id"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	At     time.Time `json:"at"`
}

// RepairJob is a chunk that needs reconstruction.
type RepairJob struct {
	ChunkID uuid.UUID `json:"chunk_id"`
	Healthy int       `json:"healthy"`
}

// Bus is a JetStream connection with the two M4 streams declared.
type Bus struct {
	nc *nats.Conn
	js jetstream.JetStream
}

// Connect opens NATS, creates JetStream context, and ensures streams exist.
func Connect(ctx context.Context, url string) (*Bus, error) {
	if url == "" {
		return nil, fmt.Errorf("events.connect: NATS_URL is empty")
	}
	nc, err := nats.Connect(url, nats.Name("dsp"), nats.Timeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("events.connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("events.connect: %w", err)
	}
	b := &Bus{nc: nc, js: js}
	if err := b.ensureStreams(ctx); err != nil {
		nc.Close()
		return nil, err
	}
	return b, nil
}

func (b *Bus) ensureStreams(ctx context.Context) error {
	_, err := b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       StreamNodeEvents,
		Subjects:   []string{"node.>"},
		Retention:  jetstream.LimitsPolicy,
		MaxAge:     nodeEventsMaxAge,
		Storage:    jetstream.FileStorage,
		Duplicates: time.Minute,
	})
	if err != nil {
		return fmt.Errorf("events.ensureStreams: %w", err)
	}
	_, err = b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       StreamRepairJobs,
		Subjects:   []string{"repair.>"},
		Retention:  jetstream.WorkQueuePolicy,
		MaxAge:     repairJobsMaxAge,
		Storage:    jetstream.FileStorage,
		Duplicates: time.Minute,
	})
	if err != nil {
		return fmt.Errorf("events.ensureStreams: %w", err)
	}
	return nil
}

// Close drains the NATS connection.
func (b *Bus) Close() {
	if b == nil || b.nc == nil {
		return
	}
	_ = b.nc.Drain()
}

// PublishNode emits a node-state transition.
func (b *Bus) PublishNode(ctx context.Context, ev NodeEvent) error {
	if b == nil {
		return nil
	}
	subj, ok := nodeSubject(ev.To)
	if !ok {
		return fmt.Errorf("events.publishNode: unknown status %q", ev.To)
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("events.publishNode: %w", err)
	}
	id := fmt.Sprintf("node:%s:%s:%d", ev.NodeID, ev.To, ev.At.Unix())
	if _, err := b.js.Publish(ctx, subj, payload, jetstream.WithMsgID(id)); err != nil {
		return fmt.Errorf("events.publishNode: %w", err)
	}
	return nil
}

func nodeSubject(status string) (string, bool) {
	switch status {
	case "online":
		return SubjNodeOnline, true
	case "suspect":
		return SubjNodeSuspect, true
	case "offline":
		return SubjNodeOffline, true
	default:
		return "", false
	}
}

// PrioritySubject returns the repair job subject for a healthy-placement count.
// Empty means no job (≥ 13 healthy).
func PrioritySubject(healthy int) string {
	switch {
	case healthy >= 13:
		return ""
	case healthy <= 10:
		return SubjRepairCritical
	case healthy == 11:
		return SubjRepairHigh
	default:
		return SubjRepairNormal
	}
}

// PublishRepair enqueues a reconstruction job at the priority matching Healthy.
func (b *Bus) PublishRepair(ctx context.Context, job RepairJob) error {
	if b == nil {
		return nil
	}
	subj := PrioritySubject(job.Healthy)
	if subj == "" {
		return nil
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("events.publishRepair: %w", err)
	}
	id := fmt.Sprintf("repair:%s:%d", job.ChunkID, job.Healthy)
	if _, err := b.js.Publish(ctx, subj, payload, jetstream.WithMsgID(id)); err != nil {
		return fmt.Errorf("events.publishRepair: %w", err)
	}
	return nil
}

// Handler processes one message body. Return nil to ack; non-nil nak/retry.
type Handler func(ctx context.Context, subject string, data []byte) error

// Consume pulls from a durable consumer until ctx is cancelled.
func (b *Bus) Consume(ctx context.Context, stream, durable, filter string, h Handler) error {
	if b == nil {
		return fmt.Errorf("events.consume: nil bus")
	}
	cons, err := b.js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: filter,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       defaultAckWait,
		MaxDeliver:    defaultMaxDeliver,
	})
	if err != nil {
		return fmt.Errorf("events.consume: %w", err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if err := h(ctx, msg.Subject(), msg.Data()); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("events.consume: %w", err)
	}
	defer cc.Stop()
	<-ctx.Done()
	return ctx.Err()
}

// FetchOne waits up to wait for a single message from a durable consumer.
func (b *Bus) FetchOne(ctx context.Context, stream, durable, filter string, wait time.Duration) (subject string, data []byte, ack func() error, nak func() error, err error) {
	if b == nil {
		return "", nil, nil, nil, fmt.Errorf("events.fetch: nil bus")
	}
	cons, err := b.js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: filter,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       defaultAckWait,
		MaxDeliver:    defaultMaxDeliver,
	})
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("events.fetch: %w", err)
	}
	batch, err := cons.Fetch(1, jetstream.FetchMaxWait(wait))
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("events.fetch: %w", err)
	}
	var msg jetstream.Msg
	for m := range batch.Messages() {
		msg = m
	}
	if err := batch.Error(); err != nil {
		return "", nil, nil, nil, err
	}
	if msg == nil {
		return "", nil, nil, nil, nats.ErrTimeout
	}
	return msg.Subject(), msg.Data(), msg.Ack, msg.Nak, nil
}

// ErrTimeout is returned by FetchOne when no message arrived.
func IsTimeout(err error) bool {
	return errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, jetstream.ErrNoMessages)
}
