package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	api := os.Getenv("DSP_API_URL")
	if api == "" {
		api = "http://gateway:8080"
	}
	seedDir := os.Getenv("SEED_DIR")
	if seedDir == "" {
		seedDir = "/seed"
	}
	email := "provider@example.com"
	pass := "provider1"
	if err := waitHealthy(api+"/healthz", 2*time.Minute); err != nil {
		slog.Error("gateway", "err", err)
		os.Exit(1)
	}
	if err := postJSON(api+"/v1/auth/register", map[string]string{"email": email, "password": pass}, nil); err != nil {
		slog.Warn("register", "err", err)
	}
	var key struct {
		Secret string `json:"secret"`
	}
	req, _ := http.NewRequest(http.MethodPost, api+"/v1/auth/api-keys", bytes.NewBufferString(`{"label":"fleet"}`))
	req.SetBasicAuth(email, pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("api-keys", "err", err)
		os.Exit(1)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Error("api-keys", "status", resp.StatusCode, "body", string(raw))
		os.Exit(1)
	}
	if err := json.Unmarshal(raw, &key); err != nil {
		slog.Error("decode key", "err", err)
		os.Exit(1)
	}
	_ = os.MkdirAll(seedDir, 0o755)
	_ = os.WriteFile(filepath.Join(seedDir, "api.key"), []byte(key.Secret), 0o644)

	for i := 1; i <= 5; i++ {
		ep := fmt.Sprintf("127.0.0.1:%d", 7442+i)
		body, _ := json.Marshal(map[string]string{"endpoint": ep})
		req, _ := http.NewRequest(http.MethodPost, api+"/v1/nodes/registration-codes", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", key.Secret)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error("code", "err", err)
			os.Exit(1)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			slog.Error("code", "i", i, "status", resp.StatusCode, "body", string(raw))
			os.Exit(1)
		}
		var out struct {
			Secret string `json:"secret"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			slog.Error("decode code", "err", err)
			os.Exit(1)
		}
		path := filepath.Join(seedDir, fmt.Sprintf("agent%d.code", i))
		if err := os.WriteFile(path, []byte(out.Secret), 0o644); err != nil {
			slog.Error("write code", "err", err)
			os.Exit(1)
		}
		slog.Info("wrote registration code", "path", path, "endpoint", ep)
	}
}

func waitHealthy(url string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}

func postJSON(url string, body any, dst any) error {
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d %s", resp.StatusCode, raw)
	}
	if dst != nil {
		return json.Unmarshal(raw, dst)
	}
	return nil
}
