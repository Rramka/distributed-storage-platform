package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/auth"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/ratelimit"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestLiveCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live stack test")
	}
	pg := os.Getenv("POSTGRES_URL")
	if pg == "" {
		pg = "postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	st, err := store.Open(ctx, pg)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(st.Close)

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	metaMux := httpserver.NewMux("metadata")
	metadata.Mount(metaMux, &metadata.StoreService{Store: st})
	metaSrv := httptest.NewServer(metaMux)
	t.Cleanup(metaSrv.Close)

	gwMux := httpserver.NewMux("gateway")
	gw := New(gwMux, metadata.NewClient(metaSrv.URL), ratelimit.New(rdb))
	gwSrv := httptest.NewServer(gw)
	t.Cleanup(gwSrv.Close)

	email := "e2e-" + uuid.NewString() + "@example.com"
	password := "password1"
	reg := post(t, gwSrv.URL+"/v1/auth/register", "", map[string]string{"email": email, "password": password})
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", reg.StatusCode, slurp(t, reg))
	}

	req, err := http.NewRequest(http.MethodPost, gwSrv.URL+"/v1/auth/api-keys", bytes.NewBufferString(`{"label":"e2e"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(email, password)
	keyResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if keyResp.StatusCode != http.StatusCreated {
		t.Fatalf("api-keys %d %s", keyResp.StatusCode, slurp(t, keyResp))
	}
	var keyBody struct {
		Secret string `json:"secret"`
		ID     string `json:"id"`
	}
	decodeResp(t, keyResp, &keyBody)
	if !auth.ValidSecretFormat(keyBody.Secret) {
		t.Fatalf("secret %q", keyBody.Secret)
	}

	bkt := post(t, gwSrv.URL+"/v1/buckets", keyBody.Secret, map[string]string{"name": "videos"})
	if bkt.StatusCode != http.StatusCreated {
		t.Fatalf("bucket %d %s", bkt.StatusCode, slurp(t, bkt))
	}
	var bucket store.Bucket
	decodeResp(t, bkt, &bucket)

	fold := post(t, gwSrv.URL+"/v1/folders", keyBody.Secret, map[string]string{
		"bucket_id": bucket.ID.String(),
		"path":      "/photos/2026",
	})
	if fold.StatusCode != http.StatusCreated {
		t.Fatalf("folder %d %s", fold.StatusCode, slurp(t, fold))
	}
	var folder store.File
	decodeResp(t, fold, &folder)

	list, err := http.NewRequest(http.MethodGet, gwSrv.URL+"/v1/files?bucket_id="+bucket.ID.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	list.Header.Set("X-Api-Key", keyBody.Secret)
	listResp, err := http.DefaultClient.Do(list)
	if err != nil {
		t.Fatal(err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list %d %s", listResp.StatusCode, slurp(t, listResp))
	}

	ren := put(t, gwSrv.URL+"/v1/rename", keyBody.Secret, map[string]string{
		"file_id":  folder.ID.String(),
		"new_path": "/pics",
	})
	if ren.StatusCode != http.StatusOK {
		t.Fatalf("rename %d %s", ren.StatusCode, slurp(t, ren))
	}

	del, err := http.NewRequest(http.MethodDelete, gwSrv.URL+"/v1/file/"+folder.ID.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	del.Header.Set("X-Api-Key", keyBody.Secret)
	delResp, err := http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("rm %d %s", delResp.StatusCode, slurp(t, delResp))
	}
	_ = delResp.Body.Close()

	delB, err := http.NewRequest(http.MethodDelete, gwSrv.URL+"/v1/buckets/"+bucket.ID.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	delB.Header.Set("X-Api-Key", keyBody.Secret)
	delBResp, err := http.DefaultClient.Do(delB)
	if err != nil {
		t.Fatal(err)
	}
	if delBResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete bucket %d %s", delBResp.StatusCode, slurp(t, delBResp))
	}
	_ = delBResp.Body.Close()
}

func post(t *testing.T, url, apiKey string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func put(t *testing.T, url, apiKey string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeResp(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatal(err)
	}
}

func slurp(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.String()
}
