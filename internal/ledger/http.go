package ledger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
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

// WindowEarning is one type/window line on GET /earnings.
type WindowEarning struct {
	Window     time.Time `json:"window"`
	Type       string    `json:"type"`
	AmountUCRD int64     `json:"amount_ucrd"`
}

// NodeEarning is one node line on GET /earnings.
type NodeEarning struct {
	NodeID        uuid.UUID       `json:"node_id"`
	BytesHeld     int64           `json:"bytes_held"`
	BytesServed   int64           `json:"bytes_served"`
	Uptime30d     float32         `json:"uptime_ratio_30d"`
	AuditPass30d  float32         `json:"audit_pass_rate_30d"`
	ReliabilityBP int64           `json:"reliability_bp"`
	StorageUCRD   int64           `json:"storage_ucrd"`
	EgressUCRD    int64           `json:"egress_ucrd"`
	Windows       []WindowEarning `json:"windows"`
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
		from, to := parseWindowRange(r)
		view, err := l.Earnings(r.Context(), id, from, to)
		if err != nil {
			apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
	})
}

func parseWindowRange(r *http.Request) (time.Time, time.Time) {
	from, _ := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	to, _ := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	return from, to
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

// Earnings summarizes provider accruals, broken down by node, window, and type.
func (l *Ledger) Earnings(ctx context.Context, userID uuid.UUID, from, to time.Time) (EarningsView, error) {
	view := EarningsView{Nodes: []NodeEarning{}}
	var lines []store.LedgerEarningLine
	acct, err := l.Store.MustAccount(ctx, OwnerProvider, ptr(userID), KindEarnings)
	if err == nil {
		bal, err := l.Store.AccountBalance(ctx, acct.ID)
		if err != nil {
			return EarningsView{}, err
		}
		view.EarningsUCRD = bal
		lines, err = l.Store.ListProviderEarnings(ctx, acct.ID, from, to)
		if err != nil {
			return EarningsView{}, err
		}
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
	byID := map[uuid.UUID][]store.LedgerEarningLine{}
	for _, ln := range lines {
		byID[ln.NodeID] = append(byID[ln.NodeID], ln)
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
			Windows:       []WindowEarning{},
		}
		if st, ok := latest[n.ID]; ok {
			row.BytesServed = st.BytesServed
		}
		for _, ln := range byID[n.ID] {
			row.Windows = append(row.Windows, WindowEarning{Window: ln.Window, Type: ln.EntryType, AmountUCRD: ln.Amount})
			switch ln.EntryType {
			case TypeStorageEarning:
				row.StorageUCRD += ln.Amount
			case TypeEgressEarning:
				row.EgressUCRD += ln.Amount
			}
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

func (c *Client) Earnings(ctx context.Context, userID uuid.UUID, from, to time.Time) (EarningsView, error) {
	var v EarningsView
	path := "/internal/users/" + userID.String() + "/earnings"
	q := url.Values{}
	if !from.IsZero() {
		q.Set("from", from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		q.Set("to", to.UTC().Format(time.RFC3339))
	}
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	err := c.get(ctx, path, &v)
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
