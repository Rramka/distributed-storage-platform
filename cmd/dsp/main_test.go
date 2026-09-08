package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "usage: dsp") {
		t.Fatalf("stdout: %q", stdout.String())
	}
}

func TestNoArgs(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d want 2", code)
	}
	if !strings.Contains(stderr.String(), "M0 skeleton") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}
