package ledger

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

// StorageView is GET /storage.
type StorageView struct {
	BytesStored int64 `json:"bytes_stored"`
	BalanceUCRD int64 `json:"balance_ucrd"`
	ChargesUCRD int64 `json:"charges_ucrd"`
}

// NodeEarning is one node line on GET /earnings.
type NodeEarning struct {
	NodeID        uuid.UUID `json:"node_id"`
	BytesHeld     int64     `json:"bytes_held"`
	BytesServed   int64     `json:"bytes_served"`
	Uptime30d     float32   `json:"uptime_ratio_30d"`
	AuditPass30d  float32   `json:"audit_pass_rate_30d"`
	ReliabilityBP int64     `json:"reliability_bp"`
}

// EarningsView is GET /earnings.
type EarningsView struct {
	EarningsUCRD int64         `json:"earnings_ucrd"`
	Nodes        []NodeEarning `json:"nodes"`
}

// Mount registers internal ledger HTTP.
func Mount(mux *http.ServeMux, l *Ledger) {
	mux.HandleFunc("GET /internal/users/{userID}/storage", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", apierr.NewRequestID())
			return
		}
		view, err := l.Storage(r.Context(), id)
		if err != nil {
			apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
	})
	mux.HandleFunc("GET /internal/users/{userID}/earnings", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", apierr.NewRequestID())
			return
		}
		view, err := l.Earnings(r.Context(), id)
		if err != nil {
			apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
	})
}

// Storage summarizes customer usage and balance.
func (l *Ledger) Storage(ctx context.Context, userID uuid.UUID) (StorageView, error) {
	bytes, err := l.Store.CustomerStoredBytes(ctx, userID)
	if err != nil {
		return StorageView{}, err
	}
	view := StorageView{BytesStored: bytes}
	acct, err := l.Store.MustAccount(ctx, OwnerCustomer, ptr(userID), KindBalance)
	if err == nil {
		bal, err := l.Store.AccountBalance(ctx, acct.ID)
		if err != nil {
			return StorageView{}, err
		}
		view.BalanceUCRD = bal
		if bal < 0 {
			view.ChargesUCRD = -bal
		}
	}
	return view, nil
}

// Earnings summarizes provider accruals.
func (l *Ledger) Earnings(ctx context.Context, userID uuid.UUID) (EarningsView, error) {
	view := EarningsView{Nodes: []NodeEarning{}}
	acct, err := l.Store.MustAccount(ctx, OwnerProvider, ptr(userID), KindEarnings)
	if err == nil {
		bal, err := l.Store.AccountBalance(ctx, acct.ID)
		if err != nil {
			return EarningsView{}, err
		}
		view.EarningsUCRD = bal
	}
	nodes, err := l.Store.ListNodesByOwner(ctx, userID)
	if err != nil {
		return EarningsView{}, err
	}
	held, err := l.Store.ListNodeHeldBytes(ctx)
	if err != nil {
		return EarningsView{}, err
	}
	byNode := map[uuid.UUID]store.NodeHeld{}
	for _, h := range held {
		byNode[h.NodeID] = h
	}
	latest, err := l.Store.LatestNodeStats(ctx)
	if err != nil {
		return EarningsView{}, err
	}
	for _, n := range nodes {
		up, audit, err := l.Store.NodeReliability(ctx, n.ID)
		if err != nil {
			return EarningsView{}, err
		}
		row := NodeEarning{
			NodeID:        n.ID,
			Uptime30d:     up,
			AuditPass30d:  audit,
			ReliabilityBP: ReliabilityBP(up, audit, n.Reputation),
			BytesHeld:     byNode[n.ID].SizeBytes,
		}
		if st, ok := latest[n.ID]; ok {
			row.BytesServed = st.BytesServed
		}
		view.Nodes = append(view.Nodes, row)
	}
	return view, nil
}

// Client talks to the ledger service.
type Client struct {
	base   string
	client *http.Client
}

// NewClient returns a ledger HTTP client.
func NewClient(base string) *Client {
	return &Client{base: base, client: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) Storage(ctx context.Context, userID uuid.UUID) (StorageView, error) {
	var v StorageView
	err := c.get(ctx, "/internal/users/"+userID.String()+"/storage", &v)
	return v, err
}

func (c *Client) Earnings(ctx context.Context, userID uuid.UUID) (EarningsView, error) {
	var v EarningsView
	err := c.get(ctx, "/internal/users/"+userID.String()+"/earnings", &v)
	return v, err
}

func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmtErr(resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

type httpErr struct{ code int }

func (e httpErr) Error() string { return http.StatusText(e.code) }

func fmtErr(code int) error { return httpErr{code: code} }
