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

type agentSpec struct {
	Index   int
	Owner   int
	Country string
	Region  string
	ASN     int
	Port    int
}

func fleet() []agentSpec {
	// 16 agents, 8 owners × 2, regions 3/3/3/3/2/2, ASNs grouped independently.
	regions := []string{
		"us-east", "us-east", "us-east",
		"us-west", "us-west", "us-west",
		"eu-west", "eu-west", "eu-west",
		"eu-central", "eu-central", "eu-central",
		"ap-south", "ap-south",
		"ap-northeast", "ap-northeast",
	}
	countries := []string{
		"US", "US", "US",
		"US", "US", "US",
		"IE", "IE", "IE",
		"DE", "DE", "DE",
		"IN", "IN",
		"JP", "JP",
	}
	asns := []int{
		64501, 64502, 64503,
		64504, 64505, 64506,
		64501, 64502, 64503,
		64504, 64505, 64506,
		64501, 64502,
		64503, 64504,
	}
	out := make([]agentSpec, 16)
	for i := 0; i < 16; i++ {
		out[i] = agentSpec{
			Index:   i + 1,
			Owner:   i / 2,
			Country: countries[i],
			Region:  regions[i],
			ASN:     asns[i],
			Port:    7443 + i,
		}
	}
	return out
}

func main() {
	api := os.Getenv("DSP_API_URL")
	if api == "" {
		api = "http://gateway:8080"
	}
	seedDir := os.Getenv("SEED_DIR")
	if seedDir == "" {
		seedDir = "/seed"
	}
	if err := waitHealthy(api+"/healthz", 2*time.Minute); err != nil {
		slog.Error("gateway", "err", err)
		os.Exit(1)
	}

	customerEmail := "provider@example.com"
	customerPass := "provider1"
	if err := postJSON(api+"/v1/auth/register", map[string]string{"email": customerEmail, "password": customerPass}, nil); err != nil {
		slog.Warn("register customer", "err", err)
	}
	custKey, err := mintAPIKey(api, customerEmail, customerPass, "fleet")
	if err != nil {
		slog.Error("customer api-keys", "err", err)
		os.Exit(1)
	}
	_ = os.MkdirAll(seedDir, 0o755)
	if err := os.WriteFile(filepath.Join(seedDir, "api.key"), []byte(custKey), 0o644); err != nil {
		slog.Error("write api.key", "err", err)
		os.Exit(1)
	}

	ownerKeys := make([]string, 8)
	for o := 0; o < 8; o++ {
		email := fmt.Sprintf("fleet-provider-%d@example.com", o+1)
		pass := "provider1"
		if err := postJSON(api+"/v1/auth/register", map[string]string{"email": email, "password": pass}, nil); err != nil {
			slog.Warn("register provider", "owner", o+1, "err", err)
		}
		key, err := mintAPIKey(api, email, pass, "nodes")
		if err != nil {
			slog.Error("provider api-keys", "owner", o+1, "err", err)
			os.Exit(1)
		}
		ownerKeys[o] = key
	}

	for _, a := range fleet() {
		body, _ := json.Marshal(map[string]any{
			"endpoint": fmt.Sprintf("127.0.0.1:%d", a.Port),
			"country":  a.Country,
			"region":   a.Region,
			"asn":      a.ASN,
		})
		req, _ := http.NewRequest(http.MethodPost, api+"/v1/nodes/registration-codes", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", ownerKeys[a.Owner])
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error("code", "err", err)
			os.Exit(1)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			slog.Error("code", "i", a.Index, "status", resp.StatusCode, "body", string(raw))
			os.Exit(1)
		}
		var out struct {
			Secret string `json:"secret"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			slog.Error("decode code", "err", err)
			os.Exit(1)
		}
		path := filepath.Join(seedDir, fmt.Sprintf("agent%d.code", a.Index))
		if err := os.WriteFile(path, []byte(out.Secret), 0o644); err != nil {
			slog.Error("write code", "err", err)
			os.Exit(1)
		}
		slog.Info("wrote registration code", "path", path, "endpoint", a.Port, "region", a.Region, "asn", a.ASN, "owner", a.Owner+1)
	}
}

func mintAPIKey(api, email, pass, label string) (string, error) {
	body, _ := json.Marshal(map[string]string{"label": label})
	req, _ := http.NewRequest(http.MethodPost, api+"/v1/auth/api-keys", bytes.NewReader(body))
	req.SetBasicAuth(email, pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http %d %s", resp.StatusCode, raw)
	}
	var key struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(raw, &key); err != nil {
		return "", err
	}
	return key.Secret, nil
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
