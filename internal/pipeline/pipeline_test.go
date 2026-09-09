package pipeline

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()
	sizes := []int{0, 1, 15, SegmentSize - 1, SegmentSize, SegmentSize + 1, SegmentSize * 2, SegmentSize*2 + 7}
	fk, err := GenerateFileKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range sizes {
		n := n
		t.Run(itoa(n), func(t *testing.T) {
			t.Parallel()
			plain := make([]byte, n)
			if n > 0 {
				_, _ = rand.Read(plain)
			}
			var cipherBuf bytes.Buffer
			prefix, psz, _, sum, err := Encrypt(&cipherBuf, bytes.NewReader(plain), fk)
			if err != nil {
				t.Fatal(err)
			}
			if psz != int64(n) {
				t.Fatalf("plain size %d want %d", psz, n)
			}
			gotSum := sha256.Sum256(cipherBuf.Bytes())
			if !bytes.Equal(sum, gotSum[:]) {
				t.Fatal("content hash")
			}
			var out bytes.Buffer
			if err := Decrypt(&out, bytes.NewReader(cipherBuf.Bytes()), fk, prefix); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), plain) {
				t.Fatalf("mismatch len %d vs %d", out.Len(), n)
			}
		})
	}
}

func TestTamperFails(t *testing.T) {
	t.Parallel()
	fk, _ := GenerateFileKey()
	plain := bytes.Repeat([]byte("hello world "), 8000)
	var buf bytes.Buffer
	prefix, _, _, _, err := Encrypt(&buf, bytes.NewReader(plain), fk)
	if err != nil {
		t.Fatal(err)
	}
	ct := buf.Bytes()

	flip := append([]byte(nil), ct...)
	flip[len(flip)/2] ^= 0x01
	if err := Decrypt(io.Discard, bytes.NewReader(flip), fk, prefix); err != ErrAuth {
		t.Fatalf("flip: %v", err)
	}

	trunc := ct[:len(ct)-20]
	if err := Decrypt(io.Discard, bytes.NewReader(trunc), fk, prefix); err == nil {
		t.Fatal("truncated succeeded")
	}

	// Swap two full segments if present.
	seg := SegmentSize + GCMTagSize
	if len(ct) > NoncePrefixLen+2*seg {
		swapped := append([]byte(nil), ct...)
		a := NoncePrefixLen
		b := NoncePrefixLen + seg
		copy(swapped[a:a+seg], ct[b:b+seg])
		copy(swapped[b:b+seg], ct[a:a+seg])
		if err := Decrypt(io.Discard, bytes.NewReader(swapped), fk, prefix); err != ErrAuth {
			t.Fatalf("swap: %v", err)
		}
	}
}

func TestWrongPassphrase(t *testing.T) {
	t.Parallel()
	fk, _ := GenerateFileKey()
	mk, p, err := DeriveMasterKey("correct horse", TestKDF())
	if err != nil {
		t.Fatal(err)
	}
	wrapped, nonce, err := WrapFileKey(mk, fk)
	if err != nil {
		t.Fatal(err)
	}
	meta := NewMeta(p, wrapped, nonce, bytes.Repeat([]byte{1}, NoncePrefixLen))
	if _, err := UnlockFK("wrong battery", meta); err != ErrPassphrase {
		t.Fatalf("wrong pw: %v", err)
	}
	got, err := UnlockFK("correct horse", meta)
	if err != nil || !bytes.Equal(got, fk) {
		t.Fatalf("unlock %v", err)
	}
}

func TestSplitChunks(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte{7}, 50)
	var chunks []ChunkInfo
	err := SplitChunks(bytes.NewReader(data), 16, func(c ChunkInfo, b []byte) error {
		if c.SizeBytes != len(b) {
			t.Fatalf("size")
		}
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 4 { // 16+16+16+2
		t.Fatalf("n chunks %d", len(chunks))
	}
	if chunks[3].Seq != 3 || chunks[3].SizeBytes != 2 {
		t.Fatalf("%+v", chunks[3])
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
