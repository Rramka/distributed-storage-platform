package agent

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"testing/quick"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

func testAgent(t *testing.T) (*Agent, *tickets.Signer, uuid.UUID) {
	t.Helper()
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	signer, err := tickets.NewSignerFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	nodeID := uuid.New()
	a := &Agent{
		id:      identity{ID: nodeID},
		store:   st,
		tickets: tickets.NewVerifier(signer.PublicKey()),
	}
	return a, signer, nodeID
}

func putFrag(t *testing.T, a *Agent, payload []byte) uuid.UUID {
	t.Helper()
	id := uuid.New()
	sum := sha256.Sum256(payload)
	if _, err := a.store.Put(id, bytes.NewReader(payload), sum[:], uint64(len(payload)+16), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return id
}

func challengeTicket(t *testing.T, s *tickets.Signer, node, frag uuid.UUID, max uint64) string {
	t.Helper()
	sum := sha256.Sum256([]byte("unused"))
	tk, err := s.Sign(tickets.Ticket{
		Op:         tickets.OpChallenge,
		FragmentID: frag,
		NodeID:     node,
		SHA256:     sum[:],
		MaxBytes:   max,
		ExpiresAt:  time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	return tk.Raw
}

func TestChallengeRoundTrip(t *testing.T) {
	t.Parallel()
	a, signer, nodeID := testAgent(t)
	payload := []byte("ciphertext-range-proof-bytes-0123456789")
	frag := putFrag(t, a, payload)
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)

	offset, length := 5, 8
	nonce := []byte("0123456789abcdef")
	want := sha256.Sum256(append(append([]byte{}, nonce...), payload[offset:offset+length]...))

	wire := challengeTicket(t, signer, nodeID, frag, 4096)
	u := srv.URL + "/challenge?fragment_id=" + frag.String() +
		"&offset=" + strconv.Itoa(offset) +
		"&length=" + strconv.Itoa(length) +
		"&nonce=" + hex.EncodeToString(nonce)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-DSP-Ticket", wire)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d %s", resp.StatusCode, b)
	}
	var out struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("got %s want %s", out.SHA256, hex.EncodeToString(want[:]))
	}
}

func TestChallengeRejects(t *testing.T) {
	t.Parallel()
	a, signer, nodeID := testAgent(t)
	payload := bytes.Repeat([]byte("x"), 128)
	frag := putFrag(t, a, payload)
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)

	good := challengeTicket(t, signer, nodeID, frag, 64)
	nonce := hex.EncodeToString([]byte("0123456789abcdef"))
	base := func() string {
		return srv.URL + "/challenge?fragment_id=" + frag.String() + "&offset=0&length=8&nonce=" + nonce
	}

	tests := []struct {
		name   string
		url    string
		ticket string
		want   int
	}{
		{"wrong op", base(), func() string {
			sum := sha256.Sum256([]byte("unused"))
			tk, _ := signer.Sign(tickets.Ticket{Op: tickets.OpGet, FragmentID: frag, NodeID: nodeID, SHA256: sum[:], MaxBytes: 64, ExpiresAt: time.Now().Add(time.Hour)})
			return tk.Raw
		}(), http.StatusForbidden},
		{"wrong node", base(), challengeTicket(t, signer, uuid.New(), frag, 64), http.StatusForbidden},
		{"expired", base(), func() string {
			sum := sha256.Sum256([]byte("unused"))
			tk, _ := signer.Sign(tickets.Ticket{Op: tickets.OpChallenge, FragmentID: frag, NodeID: nodeID, SHA256: sum[:], MaxBytes: 64, ExpiresAt: time.Now().Add(-time.Minute)})
			return tk.Raw
		}(), http.StatusForbidden},
		{"oversized", srv.URL + "/challenge?fragment_id=" + frag.String() + "&offset=0&length=" + strconv.Itoa(MaxChallengeRange+1) + "&nonce=" + nonce, good, http.StatusBadRequest},
		{"ticket max", srv.URL + "/challenge?fragment_id=" + frag.String() + "&offset=0&length=65&nonce=" + nonce, good, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, tt.url, nil)
			req.Header.Set("X-DSP-Ticket", tt.ticket)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.want {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status %d want %d %s", resp.StatusCode, tt.want, b)
			}
		})
	}

	t.Run("replay", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, base(), nil)
		req.Header.Set("X-DSP-Ticket", good)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("first %d", resp.StatusCode)
		}
		req2, _ := http.NewRequest(http.MethodGet, base(), nil)
		req2.Header.Set("X-DSP-Ticket", good)
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusForbidden {
			t.Fatalf("replay %d", resp2.StatusCode)
		}
	})
}

func TestChallengePropertyRandomRange(t *testing.T) {
	t.Parallel()
	a, signer, nodeID := testAgent(t)
	payload := bytes.Repeat([]byte("p"), 512)
	frag := putFrag(t, a, payload)
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)

	fn := func(off, ln uint8, nonceByte byte) bool {
		offset := int(off) % len(payload)
		length := int(ln)%32 + 1
		if offset+length > len(payload) {
			length = len(payload) - offset
		}
		if length <= 0 {
			return true
		}
		nonce := bytes.Repeat([]byte{nonceByte}, 16)
		want := sha256.Sum256(append(append([]byte{}, nonce...), payload[offset:offset+length]...))
		wire := challengeTicket(t, signer, nodeID, frag, uint64(MaxChallengeRange))
		u := srv.URL + "/challenge?fragment_id=" + frag.String() +
			"&offset=" + strconv.Itoa(offset) +
			"&length=" + strconv.Itoa(length) +
			"&nonce=" + hex.EncodeToString(nonce)
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("X-DSP-Ticket", wire)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var out struct {
			SHA256 string `json:"sha256"`
		}
		if json.NewDecoder(resp.Body).Decode(&out) != nil {
			return false
		}
		return out.SHA256 == hex.EncodeToString(want[:])
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 32}); err != nil {
		t.Fatal(err)
	}
}
