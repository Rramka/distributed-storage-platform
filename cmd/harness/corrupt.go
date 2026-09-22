package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const challengeBlock = 4096

func cmdCorrupt(args []string) error {
	agent, frac, err := parseCorruptArgs(args)
	if err != nil {
		return err
	}
	paths, err := listFragPaths(agent)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("corrupt: no fragments on %s", agent)
	}
	n := int(math.Ceil(frac * float64(len(paths))))
	if n < 1 {
		n = 1
	}
	if n > len(paths) {
		n = len(paths)
	}
	var ids []string
	for i := 0; i < n; i++ {
		id, err := corruptFragFile(agent, paths[i])
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	fmt.Println("corrupt", agent, "frac", frac, "fragments", strings.Join(ids, " "))
	return nil
}

func parseCorruptArgs(args []string) (agent string, frac float64, err error) {
	frac = 0.25
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--frac", "-frac", "-f":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("usage: harness corrupt <agent> [--frac 0.25]")
			}
			frac, err = strconv.ParseFloat(args[i], 64)
			if err != nil || frac <= 0 || frac > 1 {
				return "", 0, fmt.Errorf("corrupt: frac must be in (0, 1]")
			}
		default:
			rest = append(rest, args[i])
		}
	}
	if len(rest) < 1 {
		return "", 0, fmt.Errorf("usage: harness corrupt <agent> [--frac 0.25]")
	}
	return rest[0], frac, nil
}

func listFragPaths(agent string) ([]string, error) {
	cmd := composeCmd("exec", "-T", agent, "find", "/data/fragments", "-type", "f", "-name", "*.frag")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("corrupt: list %s: %w", agent, err)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

func corruptFragFile(agent, path string) (string, error) {
	raw, err := composeCmd("exec", "-T", agent, "cat", path).Output()
	if err != nil {
		return "", fmt.Errorf("corrupt: read %s: %w", path, err)
	}
	flipped := flipEveryBlock(raw, challengeBlock)
	write := composeCmd("exec", "-T", agent, "sh", "-c", "cat > "+shellSafe(path))
	write.Stdin = bytes.NewReader(flipped)
	write.Stderr = os.Stderr
	if err := write.Run(); err != nil {
		return "", fmt.Errorf("corrupt: write %s: %w", path, err)
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".frag"), nil
}

func flipEveryBlock(data []byte, block int) []byte {
	out := append([]byte(nil), data...)
	if block <= 0 {
		block = challengeBlock
	}
	for i := 0; i < len(out); i += block {
		out[i] ^= 0xff
	}
	return out
}

func shellSafe(path string) string {
	if strings.ContainsAny(path, " \t\n'\"$\\") {
		return strconv.Quote(path)
	}
	return path
}
