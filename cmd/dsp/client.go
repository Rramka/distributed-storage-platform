package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
)

type client struct {
	base  string
	key   string
	http  *http.Client
	email string
	pass  string
}

func newClient(getenv getenvFunc) *client {
	return &client{
		base:  strings.TrimRight(envOr(getenv, "DSP_API_URL", "http://127.0.0.1:8080"), "/"),
		key:   envOr(getenv, "DSP_API_KEY", ""),
		email: envOr(getenv, "DSP_EMAIL", ""),
		pass:  envOr(getenv, "DSP_PASSWORD", ""),
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *client) do(method, path string, body any, basic bool) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if basic {
		req.SetBasicAuth(c.email, c.pass)
	} else if c.key != "" {
		req.Header.Set("X-Api-Key", c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode >= 400 {
		var env apierr.Envelope
		if json.Unmarshal(raw, &env) == nil && env.Error.Code != "" {
			return resp.StatusCode, raw, fmt.Errorf("%s: %s", env.Error.Code, env.Error.Message)
		}
		return resp.StatusCode, raw, fmt.Errorf("http %d", resp.StatusCode)
	}
	return resp.StatusCode, raw, nil
}

func cmdRegister(args []string, stdout, stderr io.Writer, c *client, getenv getenvFunc) error {
	f := splitFlags(args)
	if v := f["email"]; v != "" {
		c.email = v
	}
	if v := f["password"]; v != "" {
		c.pass = v
	}
	if c.email == "" || c.pass == "" {
		return fmt.Errorf("register requires -email and -password (or DSP_EMAIL / DSP_PASSWORD)")
	}
	_, raw, err := c.do(http.MethodPost, "/v1/auth/register", map[string]string{
		"email":    c.email,
		"password": c.pass,
	}, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdAPIKeys(args []string, stdout, stderr io.Writer, c *client, getenv getenvFunc) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dsp api-keys create|list|revoke")
	}
	f := splitFlags(args[1:])
	if v := f["email"]; v != "" {
		c.email = v
	}
	if v := f["password"]; v != "" {
		c.pass = v
	}
	switch args[0] {
	case "create":
		label := f["label"]
		if label == "" {
			return fmt.Errorf("api-keys create requires -label")
		}
		if c.email == "" || c.pass == "" {
			return fmt.Errorf("api-keys create requires Basic auth (-email/-password or DSP_EMAIL/DSP_PASSWORD)")
		}
		_, raw, err := c.do(http.MethodPost, "/v1/auth/api-keys", map[string]string{"label": label}, true)
		if err != nil {
			return err
		}
		_, err = stdout.Write(raw)
		return err
	case "list":
		_, raw, err := c.do(http.MethodGet, "/v1/auth/api-keys", nil, false)
		if err != nil {
			return err
		}
		_, err = stdout.Write(raw)
		return err
	case "revoke":
		id := f["id"]
		if id == "" {
			return fmt.Errorf("api-keys revoke requires -id")
		}
		_, _, err := c.do(http.MethodDelete, "/v1/auth/api-keys/"+url.PathEscape(id), nil, false)
		return err
	default:
		return fmt.Errorf("usage: dsp api-keys create|list|revoke")
	}
}

func cmdBuckets(args []string, stdout, stderr io.Writer, c *client) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dsp buckets create|list|delete")
	}
	f := splitFlags(args[1:])
	switch args[0] {
	case "create":
		name := f["name"]
		if name == "" {
			return fmt.Errorf("buckets create requires -name")
		}
		_, raw, err := c.do(http.MethodPost, "/v1/buckets", map[string]string{"name": name}, false)
		if err != nil {
			return err
		}
		_, err = stdout.Write(raw)
		return err
	case "list":
		_, raw, err := c.do(http.MethodGet, "/v1/buckets", nil, false)
		if err != nil {
			return err
		}
		_, err = stdout.Write(raw)
		return err
	case "delete":
		id := f["id"]
		if id == "" {
			return fmt.Errorf("buckets delete requires -id")
		}
		_, _, err := c.do(http.MethodDelete, "/v1/buckets/"+url.PathEscape(id), nil, false)
		return err
	default:
		return fmt.Errorf("usage: dsp buckets create|list|delete")
	}
}

func cmdFolders(args []string, stdout, stderr io.Writer, c *client) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: dsp folders create -bucket-id ID -path /folder")
	}
	f := splitFlags(args[1:])
	bid := first(f, "bucket-id", "bucket_id")
	p := f["path"]
	if bid == "" || p == "" {
		return fmt.Errorf("folders create requires -bucket-id and -path")
	}
	_, raw, err := c.do(http.MethodPost, "/v1/folders", map[string]string{"bucket_id": bid, "path": p}, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdLS(args []string, stdout, stderr io.Writer, c *client) error {
	f := splitFlags(args)
	bid := first(f, "bucket-id", "bucket_id")
	if bid == "" {
		return fmt.Errorf("ls requires -bucket-id")
	}
	q := url.Values{}
	q.Set("bucket_id", bid)
	if p := f["prefix"]; p != "" {
		q.Set("prefix", p)
	}
	_, raw, err := c.do(http.MethodGet, "/v1/files?"+q.Encode(), nil, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdMV(args []string, stdout, stderr io.Writer, c *client) error {
	f := splitFlags(args)
	fid := first(f, "file-id", "file_id")
	np := first(f, "new-path", "new_path")
	if fid == "" || np == "" {
		return fmt.Errorf("mv requires -file-id and -new-path")
	}
	_, raw, err := c.do(http.MethodPut, "/v1/rename", map[string]string{"file_id": fid, "new_path": np}, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdRM(args []string, stdout, stderr io.Writer, c *client) error {
	f := splitFlags(args)
	id := first(f, "file-id", "id", "file_id")
	if id == "" {
		return fmt.Errorf("rm requires -file-id")
	}
	_, _, err := c.do(http.MethodDelete, "/v1/file/"+url.PathEscape(id), nil, false)
	return err
}

func first(f map[string]string, names ...string) string {
	for _, n := range names {
		if v := f[n]; v != "" {
			return v
		}
	}
	return ""
}
