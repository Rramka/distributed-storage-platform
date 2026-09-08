package httpserver

import (
	"testing"
)

func TestAddrFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		httpAddr string
		fallback string
		want     string
	}{
		{
			name:     "fallback when HTTP_ADDR unset",
			httpAddr: "",
			fallback: ":8080",
			want:     ":8080",
		},
		{
			name:     "HTTP_ADDR when set",
			httpAddr: ":9090",
			fallback: ":8080",
			want:     ":9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", tt.httpAddr)
			if got := AddrFromEnv(tt.fallback); got != tt.want {
				t.Fatalf("AddrFromEnv(%q) = %q, want %q", tt.fallback, got, tt.want)
			}
		})
	}
}
