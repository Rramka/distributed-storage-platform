package main

import (
	"bytes"
	"testing"
)

func TestFlipEveryBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    []byte
		block int
		want  []byte
	}{
		{"empty", nil, 4096, []byte{}},
		{"one byte", []byte{0x00}, 4096, []byte{0xff}},
		{"exact block", bytes.Repeat([]byte{0x11}, 4), 4, append([]byte{0xee}, bytes.Repeat([]byte{0x11}, 3)...)},
		{"two blocks", bytes.Repeat([]byte{0x00}, 8), 4, func() []byte {
			b := bytes.Repeat([]byte{0x00}, 8)
			b[0], b[4] = 0xff, 0xff
			return b
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := flipEveryBlock(tt.in, tt.block)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			if len(tt.in) > 0 && bytes.Equal(got, tt.in) {
				t.Fatal("input mutated or not flipped")
			}
		})
	}
}

func TestParseCorruptArgs(t *testing.T) {
	t.Parallel()
	agent, frac, err := parseCorruptArgs([]string{"agent3", "--frac", "0.5"})
	if err != nil || agent != "agent3" || frac != 0.5 {
		t.Fatalf("got %s %v %v", agent, frac, err)
	}
	if _, _, err := parseCorruptArgs(nil); err == nil {
		t.Fatal("expected usage error")
	}
}
