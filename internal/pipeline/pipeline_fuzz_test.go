package pipeline

import (
	"bytes"
	"io"
	"testing"
)

func FuzzChunkDecrypt(f *testing.F) {
	fk, err := GenerateFileKey()
	if err != nil {
		f.Fatal(err)
	}
	var buf bytes.Buffer
	prefix, _, _, _, err := Encrypt(&buf, bytes.NewReader([]byte("seed-corpus")), fk)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3})
	f.Add(append([]byte(nil), prefix...))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = Decrypt(io.Discard, bytes.NewReader(data), fk, prefix)
	})
}
