package agent

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rramka/distributed-storage-platform/internal/receipts"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

func (a *Agent) fragmentMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /fragments/{id}", a.handlePut)
	mux.HandleFunc("GET /fragments/{id}", a.handleGet)
	mux.HandleFunc("DELETE /fragments/{id}", a.handleDelete)
	mux.HandleFunc("GET /challenge", a.handleChallenge)
	return mux
}

func ticketFrom(r *http.Request) string {
	if t := r.Header.Get("X-DSP-Ticket"); t != "" {
		return t
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Ticket ") {
		return strings.TrimPrefix(a, "Ticket ")
	}
	return r.URL.Query().Get("ticket")
}

func (a *Agent) handlePut(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid fragment id", http.StatusBadRequest)
		return
	}
	tk, err := a.tickets.Verify(ticketFrom(r), tickets.OpPut, a.id.ID, id)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.store.ConsumeNonce(tk.Nonce, tk.ExpiresAt); err != nil {
		http.Error(w, "replay", http.StatusForbidden)
		return
	}
	meta, err := a.store.Put(id, r.Body, tk.SHA256, tk.MaxBytes, tk.ExpiresAt)
	if errors.Is(err, ErrHash) || errors.Is(err, ErrTooLarge) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "store failed", http.StatusInternalServerError)
		return
	}
	rec, err := receipts.Sign(a.id.Priv, a.id.ID, id, meta.SHA256, uint64(meta.Size), meta.StoredAt)
	if err != nil {
		http.Error(w, "sign failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"receipt": rec.Raw})
}

func (a *Agent) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid fragment id", http.StatusBadRequest)
		return
	}
	tk, err := a.tickets.Verify(ticketFrom(r), tickets.OpGet, a.id.ID, id)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.store.ConsumeNonce(tk.Nonce, tk.ExpiresAt); err != nil {
		http.Error(w, "replay", http.StatusForbidden)
		return
	}
	f, meta, err := a.store.Get(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	if len(tk.SHA256) == 32 && subtle.ConstantTimeCompare(tk.SHA256, meta.SHA256) != 1 && !allZero(tk.SHA256) {
		http.Error(w, "hash mismatch", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	if rng := r.Header.Get("Range"); rng != "" {
		http.ServeContent(w, r, "", meta.StoredAt, f)
		return
	}
	_, _ = io.Copy(w, f)
}

func (a *Agent) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid fragment id", http.StatusBadRequest)
		return
	}
	_, err = a.tickets.Verify(ticketFrom(r), tickets.OpDelete, a.id.ID, id)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.store.Delete(id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Agent) handleChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("fragment_id"))
	if err != nil {
		http.Error(w, "invalid fragment id", http.StatusBadRequest)
		return
	}
	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		http.Error(w, "invalid offset", http.StatusBadRequest)
		return
	}
	length, err := strconv.ParseInt(r.URL.Query().Get("length"), 10, 64)
	if err != nil || length <= 0 {
		http.Error(w, "invalid length", http.StatusBadRequest)
		return
	}
	if length > MaxChallengeRange {
		http.Error(w, "length exceeds max", http.StatusBadRequest)
		return
	}
	nonce, err := hex.DecodeString(r.URL.Query().Get("nonce"))
	if err != nil || len(nonce) == 0 {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}
	tk, err := a.tickets.Verify(ticketFrom(r), tickets.OpChallenge, a.id.ID, id)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if tk.MaxBytes > 0 && uint64(length) > tk.MaxBytes {
		http.Error(w, "length exceeds max", http.StatusBadRequest)
		return
	}
	if err := a.store.ConsumeNonce(tk.Nonce, tk.ExpiresAt); err != nil {
		http.Error(w, "replay", http.StatusForbidden)
		return
	}
	rng, err := a.store.ReadRange(id, offset, length)
	if errors.Is(err, ErrNotStored) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, ErrTooLarge) {
		http.Error(w, "range", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	h := sha256.New()
	_, _ = h.Write(nonce)
	_, _ = h.Write(rng)
	sum := h.Sum(nil)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"sha256": hex.EncodeToString(sum)})
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}
