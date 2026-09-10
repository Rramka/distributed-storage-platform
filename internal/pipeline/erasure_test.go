package pipeline

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"testing"
)

func TestEncodeReconstructSizes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		n    int
	}{
		{"empty", 0},
		{"one", 1},
		{"sub_shard", 7},
		{"exact_data_shards", ECData},
		{"short_final", 100},
		{"one_kib", 1024},
		{"exact_multiple", 160},
		{"64kib", 64 * 1024},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chunk := make([]byte, tc.n)
			if tc.n > 0 {
				if _, err := rand.Read(chunk); err != nil {
					t.Fatal(err)
				}
			}
			shards, err := EncodeChunk(chunk)
			if err != nil {
				t.Fatal(err)
			}
			if len(shards) != ECTotal {
				t.Fatalf("shard count %d", len(shards))
			}
			got, err := ReconstructChunk(shards, tc.n)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, chunk) {
				t.Fatalf("mismatch len %d vs %d", len(got), tc.n)
			}
		})
	}
}

func TestReconstructAnyTenOfSixteen(t *testing.T) {
	t.Parallel()
	const trials = 32
	for i := 0; i < trials; i++ {
		n := 1 + i*97
		chunk := make([]byte, n)
		if _, err := rand.Read(chunk); err != nil {
			t.Fatal(err)
		}
		shards, err := EncodeChunk(chunk)
		if err != nil {
			t.Fatal(err)
		}
		drop := pickDistinct(t, ECParity, ECTotal)
		partial := make([][]byte, ECTotal)
		for i, s := range shards {
			if drop[i] {
				continue
			}
			partial[i] = s
		}
		got, err := ReconstructChunk(partial, n)
		if err != nil {
			t.Fatalf("trial %d: %v", i, err)
		}
		if !bytes.Equal(got, chunk) {
			t.Fatalf("trial %d mismatch", i)
		}
	}
}

func TestReconstructTooFewShards(t *testing.T) {
	t.Parallel()
	chunk := bytes.Repeat([]byte{0xab}, 200)
	shards, err := EncodeChunk(chunk)
	if err != nil {
		t.Fatal(err)
	}
	partial := make([][]byte, ECTotal)
	for i := 0; i < ECData-1; i++ {
		partial[i] = shards[i]
	}
	if _, err := ReconstructChunk(partial, len(chunk)); err != ErrShards {
		t.Fatalf("got %v want ErrShards", err)
	}
	if _, err := ReconstructChunk(shards[:8], len(chunk)); err != ErrShards {
		t.Fatalf("short slice: %v", err)
	}
}

func pickDistinct(t *testing.T, k, n int) map[int]bool {
	t.Helper()
	out := map[int]bool{}
	for len(out) < k {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
		if err != nil {
			t.Fatal(err)
		}
		out[int(i.Int64())] = true
	}
	return out
}
