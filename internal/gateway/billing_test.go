package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/auth"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/ledger"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/ratelimit"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestStorageAndEarningsAuthScope(t *testing.T) {
	st := store.OpenForTest(t)
	led := &ledger.Ledger{Store: st, Rates: ledger.DefaultRates()}
	ledMux := httpserver.NewMux("ledger")
	ledger.Mount(ledMux, led)
	ledSrv := httptest.NewServer(ledMux)
	t.Cleanup(ledSrv.Close)

	metaMux := httpserver.NewMux("metadata")
	metadata.Mount(metaMux, &metadata.StoreService{Store: st})
	metaSrv := httptest.NewServer(metaMux)
	t.Cleanup(metaSrv.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	gwMux := httpserver.NewMux("gateway")
	gw := New(gwMux, metadata.NewClient(metaSrv.URL), ratelimit.New(rdb))
	gw.WithLedger(ledger.NewClient(ledSrv.URL))
	gwSrv := httptest.NewServer(gw)
	t.Cleanup(gwSrv.Close)

	secretA := registerKey(t, gwSrv.URL, "bill-a-"+uuid.NewString()+"@example.com")
	secretB := registerKey(t, gwSrv.URL, "bill-b-"+uuid.NewString()+"@example.com")

	unauth, err := http.NewRequest(http.MethodGet, gwSrv.URL+"/v1/storage", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(unauth)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth storage %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	storA := getJSON(t, gwSrv.URL+"/v1/storage", secretA)
	if storA.StatusCode != http.StatusOK {
		t.Fatalf("storage A %d %s", storA.StatusCode, slurp(t, storA))
	}
	var viewA ledger.StorageView
	decodeResp(t, storA, &viewA)
	if viewA.BytesStored != 0 {
		t.Fatalf("empty account stored %d", viewA.BytesStored)
	}

	earnA := getJSON(t, gwSrv.URL+"/v1/earnings", secretA)
	if earnA.StatusCode != http.StatusOK {
		t.Fatalf("earnings A %d %s", earnA.StatusCode, slurp(t, earnA))
	}
	var evA ledger.EarningsView
	decodeResp(t, earnA, &evA)
	if evA.Nodes == nil {
		t.Fatal("nodes should be empty list")
	}

	storB := getJSON(t, gwSrv.URL+"/v1/storage", secretB)
	if storB.StatusCode != http.StatusOK {
		t.Fatalf("storage B %d", storB.StatusCode)
	}
	_ = storB.Body.Close()
}

func registerKey(t *testing.T, base, email string) string {
	t.Helper()
	reg := post(t, base+"/v1/auth/register", "", map[string]string{"email": email, "password": "password1"})
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", reg.StatusCode, slurp(t, reg))
	}
	_ = reg.Body.Close()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/api-keys", bytes.NewBufferString(`{"label":"t"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(email, "password1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("api-keys %d %s", resp.StatusCode, slurp(t, resp))
	}
	var keyBody struct {
		Secret string `json:"secret"`
	}
	decodeResp(t, resp, &keyBody)
	if !auth.ValidSecretFormat(keyBody.Secret) {
		t.Fatalf("secret %q", keyBody.Secret)
	}
	return keyBody.Secret
}

func getJSON(t *testing.T, url, apiKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
