package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/invariants"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: harness kill|drain|flap|status|partition|corrupt|throttle|m4")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	switch cmd {
	case "kill":
		return cmdKill(args)
	case "drain":
		return cmdDrain(args)
	case "flap":
		return cmdFlap(args)
	case "status":
		return cmdStatus(args)
	case "partition", "corrupt", "throttle":
		return fmt.Errorf("%s: not implemented (needs challenge/network slice)", cmd)
	case "m4":
		return cmdM4(args)
	default:
		return fmt.Errorf("unknown verb %q", cmd)
	}
}

func composeFile() string {
	if v := os.Getenv("DSP_COMPOSE"); v != "" {
		return v
	}
	return "deploy/compose/docker-compose.yml"
}

func compose(args ...string) *exec.Cmd {
	all := append([]string{"compose", "-f", composeFile()}, args...)
	c := exec.Command("docker", all...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func openStore() (*store.Store, error) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return store.Open(ctx, url)
}

func parseFileFlag(args []string) (uuid.UUID, int, []string) {
	n := 6
	var file uuid.UUID
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--n":
			i++
			n, _ = strconv.Atoi(args[i])
		case "-file", "--file":
			i++
			file = uuid.MustParse(args[i])
		default:
			rest = append(rest, args[i])
		}
	}
	if n <= 0 {
		n = 6
	}
	return file, n, rest
}

func holders(ctx context.Context, s *store.Store, fileID uuid.UUID) ([]store.Node, error) {
	if fileID != uuid.Nil {
		return s.FileHolders(ctx, fileID)
	}
	return s.AnyCommittedHolders(ctx)
}

func cmdKill(args []string) error {
	fileID, n, _ := parseFileFlag(args)
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ctx := context.Background()
	nodes, err := holders(ctx, s, fileID)
	if err != nil {
		return err
	}
	if len(nodes) < n {
		return fmt.Errorf("only %d holders, want %d", len(nodes), n)
	}
	var names []string
	for i := 0; i < n; i++ {
		name, err := agentService(nodes[i].Endpoint)
		if err != nil {
			return err
		}
		names = append(names, name)
	}
	fmt.Println("kill", strings.Join(names, " "))
	return compose(append([]string{"kill"}, names...)...).Run()
}

func cmdDrain(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: harness drain <agent>")
	}
	if err := compose("kill", "-s", "SIGTERM", args[0]).Run(); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	return compose("stop", args[0]).Run()
}

func cmdFlap(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: harness flap <agent> [period]")
	}
	period := 5 * time.Second
	if len(args) > 1 {
		if d, err := time.ParseDuration(args[1]); err == nil {
			period = d
		}
	}
	if err := compose("kill", args[0]).Run(); err != nil {
		return err
	}
	time.Sleep(period)
	return compose("start", args[0]).Run()
}

func cmdStatus(args []string) error {
	fileID, _, _ := parseFileFlag(args)
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ctx := context.Background()
	if fileID == uuid.Nil {
		health, err := s.AnyChunkHealth(ctx)
		if err != nil {
			return err
		}
		for id, n := range health {
			fmt.Printf("%s %d/16\n", id, n)
		}
		return nil
	}
	health, err := s.ChunkHealthByFile(ctx, fileID)
	if err != nil {
		return err
	}
	for id, n := range health {
		fmt.Printf("%s %d/16\n", id, n)
	}
	return nil
}

func cmdM4(args []string) error {
	started := time.Now().UTC()
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ctx := context.Background()
	nodes, err := s.AnyCommittedHolders(ctx)
	if err != nil {
		return err
	}
	if len(nodes) < 16 {
		return fmt.Errorf("need a committed file with 16 holders, have %d (dsp put first)", len(nodes))
	}
	killSet := nodes[:6]
	var names []string
	for _, n := range killSet {
		name, err := agentService(n.Endpoint)
		if err != nil {
			return err
		}
		names = append(names, name)
	}
	fmt.Println("## Chaos report")
	fmt.Println("- Scenario: M4 kill-6-of-16")
	fmt.Println("- Started:", started.Format(time.RFC3339))
	fmt.Println("- Kill set:", strings.Join(names, ", "))
	if err := compose(append([]string{"kill"}, names...)...).Run(); err != nil {
		return err
	}
	fmt.Println("- Download during failure: run `dsp get` now (not waited by harness)")
	deadline := time.Now().Add(10 * time.Minute)
	var final map[uuid.UUID]int
	for time.Now().Before(deadline) {
		h, err := s.AnyChunkHealth(ctx)
		if err != nil {
			return err
		}
		final = h
		all := len(h) > 0
		for _, n := range h {
			if n < 16 {
				all = false
				break
			}
		}
		if all {
			break
		}
		time.Sleep(5 * time.Second)
	}
	lat := time.Since(started).Round(time.Second)
	fmt.Println("- Repair latency:", lat)
	minH := 16
	for _, n := range final {
		if n < minH {
			minH = n
		}
	}
	fmt.Printf("- Final healthy placements: min %d/16 across %d chunks\n", minH, len(final))
	if err := invariants.CheckAll(ctx, s); err != nil {
		fmt.Println("- Notes:", err)
		return err
	}
	fmt.Println("- Notes: invariants passed")
	return nil
}

func agentService(endpoint string) (string, error) {
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return "", err
	}
	idx := p - 7443 + 1
	if idx < 1 || idx > 24 {
		return "", fmt.Errorf("endpoint %s is not a compose agent", endpoint)
	}
	return fmt.Sprintf("agent%d", idx), nil
}
