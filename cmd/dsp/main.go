package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "dsp — distributed storage platform CLI")
	fmt.Fprintln(stderr, "M0 skeleton: put/get are not implemented yet. See docs/10-mvp-roadmap.md (solo builder track).")
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(stdout, "usage: dsp <command>")
		fmt.Fprintln(stdout, "commands will land in M1 (metadata) and M2 (put/get)")
		return 0
	}
	return 2
}
