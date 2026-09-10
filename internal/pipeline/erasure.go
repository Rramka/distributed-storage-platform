package pipeline

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

const (
	ECData   = 10
	ECParity = 6
	ECTotal  = ECData + ECParity
)

var (
	ErrShards = errors.New("pipeline: insufficient or invalid shards")
)

// EncodeChunk expands a ciphertext chunk into 16 equal shards (10 data + 6 parity).
// An empty chunk is padded so Split can run; ReconstructChunk trims via chunkSize.
func EncodeChunk(chunk []byte) ([][]byte, error) {
	enc, err := reedsolomon.New(ECData, ECParity)
	if err != nil {
		return nil, fmt.Errorf("pipeline.encode: %w", err)
	}
	data := chunk
	if len(data) == 0 {
		data = []byte{0}
	}
	shards, err := enc.Split(data)
	if err != nil {
		return nil, fmt.Errorf("pipeline.encode: %w", err)
	}
	if err := enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("pipeline.encode: %w", err)
	}
	return shards, nil
}

// ReconstructChunk rebuilds a ciphertext chunk from at least 10 of 16 shards.
// Missing entries in shards must be nil. chunkSize trims Split padding.
func ReconstructChunk(shards [][]byte, chunkSize int) ([]byte, error) {
	if len(shards) != ECTotal {
		return nil, ErrShards
	}
	if chunkSize < 0 {
		return nil, ErrShards
	}
	if chunkSize == 0 {
		return []byte{}, nil
	}
	present := 0
	for _, s := range shards {
		if s != nil {
			present++
		}
	}
	if present < ECData {
		return nil, ErrShards
	}
	enc, err := reedsolomon.New(ECData, ECParity)
	if err != nil {
		return nil, fmt.Errorf("pipeline.reconstruct: %w", err)
	}
	work := make([][]byte, ECTotal)
	for i, s := range shards {
		if s == nil {
			continue
		}
		work[i] = append([]byte(nil), s...)
	}
	if err := enc.Reconstruct(work); err != nil {
		return nil, fmt.Errorf("pipeline.reconstruct: %w", err)
	}
	var buf bytes.Buffer
	if err := enc.Join(&buf, work, chunkSize); err != nil {
		return nil, fmt.Errorf("pipeline.reconstruct: %w", err)
	}
	return buf.Bytes(), nil
}

// ReconstructShards fills nil slots in shards in place. Requires ≥ 10 present.
func ReconstructShards(shards [][]byte) error {
	if len(shards) != ECTotal {
		return ErrShards
	}
	present := 0
	for _, s := range shards {
		if s != nil {
			present++
		}
	}
	if present < ECData {
		return ErrShards
	}
	enc, err := reedsolomon.New(ECData, ECParity)
	if err != nil {
		return fmt.Errorf("pipeline.reconstructShards: %w", err)
	}
	if err := enc.Reconstruct(shards); err != nil {
		return fmt.Errorf("pipeline.reconstructShards: %w", err)
	}
	return nil
}
