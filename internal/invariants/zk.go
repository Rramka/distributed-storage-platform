package invariants

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// CheckZeroKnowledge fails if any needle (plaintext fixture or unwrapped key)
// appears in the given directory trees or blobs (e.g. a pg_dump).
func CheckZeroKnowledge(roots []string, blobs [][]byte, needles [][]byte) error {
	if len(needles) == 0 {
		return nil
	}
	for _, b := range blobs {
		if err := scanBytes("blob", b, needles); err != nil {
			return err
		}
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, err := os.ReadFile(root + "/" + path)
			if err != nil {
				return nil
			}
			return scanBytes(root+"/"+path, raw, needles)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func scanBytes(name string, data []byte, needles [][]byte) error {
	for _, n := range needles {
		if len(n) == 0 {
			continue
		}
		if bytes.Contains(data, n) {
			return fmt.Errorf("invariants.zeroknowledge: needle found in %s", name)
		}
	}
	return nil
}

func needlesFromEnv() [][]byte {
	raw := os.Getenv("DSP_ZK_NEEDLE")
	if raw == "" {
		return nil
	}
	var out [][]byte
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, []byte(p))
		}
	}
	return out
}

func dataDirsFromEnv() []string {
	raw := os.Getenv("DSP_DATA_DIRS")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
