package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr, nil); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "usage: dsp") {
		t.Fatalf("stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "register") {
		t.Fatalf("expected register in help: %q", stdout.String())
	}
}

func TestNoArgs(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr, nil); code != 2 {
		t.Fatalf("exit %d want 2", code)
	}
	if !strings.Contains(stdout.String(), "usage: dsp") {
		t.Fatalf("stdout: %q", stdout.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"nope"}, &stdout, &stderr, nil); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestMissingFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"register"}, "register requires"},
		{[]string{"api-keys", "create"}, "api-keys create requires -label"},
		{[]string{"api-keys", "create", "-label", "cli"}, "Basic auth"},
		{[]string{"buckets", "create"}, "-name"},
		{[]string{"folders", "create"}, "-bucket-id"},
		{[]string{"ls"}, "-bucket-id"},
		{[]string{"mv"}, "-file-id"},
		{[]string{"rm"}, "-file-id"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr, nil)
			if code != 1 {
				t.Fatalf("exit %d", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr %q want %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestSplitFlags(t *testing.T) {
	t.Parallel()
	got := splitFlags([]string{"-email", "a@b.com", "-password", "secret"})
	if got["email"] != "a@b.com" || got["password"] != "secret" {
		t.Fatalf("%v", got)
	}
}
