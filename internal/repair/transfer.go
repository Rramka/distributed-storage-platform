package repair

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/agent"
	"github.com/google/uuid"
)

func fragmentURL(endpoint, fragID string) (url, serverName string, err error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		port = "7443"
	}
	serverName = host
	if override := os.Getenv("REPAIR_ENDPOINT_HOST"); override != "" {
		host = override
	}
	return "https://" + net.JoinHostPort(host, port) + "/fragments/" + fragID, serverName, nil
}

func putFragment(ctx context.Context, caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string, data []byte) (string, error) {
	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return "", err
	}
	rawURL, serverName, err := fragmentURL(endpoint, fragID)
	if err != nil {
		return "", err
	}
	cl := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(caCert, nid, serverName)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("repair.put: http %d %s", resp.StatusCode, raw)
	}
	var out struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.Receipt, nil
}

func getFragment(ctx context.Context, caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string) ([]byte, error) {
	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return nil, err
	}
	rawURL, serverName, err := fragmentURL(endpoint, fragID)
	if err != nil {
		return nil, err
	}
	cl := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(caCert, nid, serverName)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repair.get: http %d %s", resp.StatusCode, raw)
	}
	return raw, nil
}

func getChallenge(ctx context.Context, caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string, offset, length int, nonce []byte) ([]byte, error) {
	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return nil, err
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		port = "7443"
	}
	serverName := host
	if override := os.Getenv("REPAIR_ENDPOINT_HOST"); override != "" {
		host = override
	}
	u := "https://" + net.JoinHostPort(host, port) + "/challenge?fragment_id=" + fragID +
		"&offset=" + strconv.Itoa(offset) +
		"&length=" + strconv.Itoa(length) +
		"&nonce=" + hex.EncodeToString(nonce)
	cl := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(caCert, nid, serverName)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repair.challenge: http %d %s", resp.StatusCode, raw)
	}
	var out struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	sum, err := hex.DecodeString(out.SHA256)
	if err != nil {
		return nil, err
	}
	return sum, nil
}

func hashMatches(data []byte, wantHex string) bool {
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return bytes.Equal(sum[:], want)
}
