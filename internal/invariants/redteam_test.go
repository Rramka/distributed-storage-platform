package invariants

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/pipeline"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

const (
	redteamMarker     = "PLAINTEXT_SECRET_MARKER"
	redteamPassphrase = "correct-horse-W11-passphrase"
)

func TestRedTeamDBDumpDecryptsNothing(t *testing.T) {
	s := store.OpenForTest(t)

	ctx := context.Background()
	fk, err := pipeline.GenerateFileKey()
	if err != nil {
		t.Fatal(err)
	}
	mk, kdf, err := pipeline.DeriveMasterKey(redteamPassphrase, pipeline.TestKDF())
	if err != nil {
		t.Fatal(err)
	}
	wrapped, wrapNonce, err := pipeline.WrapFileKey(mk, fk)
	if err != nil {
		t.Fatal(err)
	}
	var ct bytes.Buffer
	plain := []byte(redteamMarker + "-never-in-postgres")
	prefix, _, _, contentSHA, err := pipeline.Encrypt(&ct, bytes.NewReader(plain), fk)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := pipeline.MarshalMeta(pipeline.NewMeta(kdf, wrapped, wrapNonce, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SeedCommittedFleet(ctx, meta, contentSHA, int64(len(plain))); err != nil {
		t.Fatal(err)
	}

	dump, err := pgDump()
	if err != nil {
		t.Fatal(err)
	}
	needles := [][]byte{[]byte(redteamMarker), []byte(redteamPassphrase), fk, mk}
	if err := CheckZeroKnowledge(nil, [][]byte{dump}, needles); err != nil {
		t.Fatal(err)
	}

	if err := s.Exec(ctx, `CREATE TABLE IF NOT EXISTS redteam_scratch (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.Exec(context.Background(), `DROP TABLE IF EXISTS redteam_scratch`)
	})
	if err := s.Exec(ctx, `INSERT INTO redteam_scratch (v) VALUES ($1)`, redteamMarker); err != nil {
		t.Fatal(err)
	}
	planted, err := pgDump()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckZeroKnowledge(nil, [][]byte{planted}, [][]byte{[]byte(redteamMarker)}); err == nil {
		t.Fatal("expected planted needle to be detected")
	}
}

func pgDump() ([]byte, error) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable"
	}
	cmd := exec.Command("pg_dump", url)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	compose := os.Getenv("DSP_COMPOSE")
	if compose == "" {
		compose = findComposeFile()
	}
	fallback := exec.Command("docker", "compose", "-f", compose, "exec", "-T", "postgres", "pg_dump", "-U", "dsp", "dsp")
	out2, err2 := fallback.CombinedOutput()
	if err2 != nil {
		return nil, fmt.Errorf("pg_dump: %s; compose: %s (%w)", bytes.TrimSpace(out), bytes.TrimSpace(out2), err2)
	}
	return out2, nil
}

func findComposeFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return "deploy/compose/docker-compose.yml"
	}
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "deploy", "compose", "docker-compose.yml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "deploy/compose/docker-compose.yml"
}
