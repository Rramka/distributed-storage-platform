package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.LookupEnv))
}

type getenvFunc func(string) (string, bool)

func run(args []string, stdout, stderr io.Writer, getenv getenvFunc) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printUsage(stdout)
		if len(args) == 0 {
			return 2
		}
		return 0
	}

	c := newClient(getenv)
	var err error
	switch args[0] {
	case "register":
		err = cmdRegister(args[1:], stdout, stderr, c, getenv)
	case "api-keys":
		err = cmdAPIKeys(args[1:], stdout, stderr, c, getenv)
	case "buckets":
		err = cmdBuckets(args[1:], stdout, stderr, c)
	case "folders":
		err = cmdFolders(args[1:], stdout, stderr, c)
	case "ls":
		err = cmdLS(args[1:], stdout, stderr, c)
	case "mv":
		err = cmdMV(args[1:], stdout, stderr, c)
	case "rm":
		err = cmdRM(args[1:], stdout, stderr, c)
	case "put":
		err = cmdPut(args[1:], stdout, stderr, c, getenv)
	case "get":
		err = cmdGet(args[1:], stdout, stderr, c, getenv)
	case "nodes":
		err = cmdNodes(args[1:], stdout, stderr, c)
	case "provider":
		err = cmdProvider(args[1:], stdout, stderr, c)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: dsp <command>")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  register              create an account")
	fmt.Fprintln(w, "  api-keys create|list|revoke")
	fmt.Fprintln(w, "  buckets create|list|delete")
	fmt.Fprintln(w, "  folders create")
	fmt.Fprintln(w, "  ls                    list files/folders")
	fmt.Fprintln(w, "  mv                    rename a file or folder")
	fmt.Fprintln(w, "  rm                    soft-delete a file or folder")
	fmt.Fprintln(w, "  put                   upload a local file (encrypted)")
	fmt.Fprintln(w, "  get                   download a file")
	fmt.Fprintln(w, "  nodes                 list provider nodes")
	fmt.Fprintln(w, "  provider codes create mint an agent registration code")
	fmt.Fprintln(w, "env: DSP_API_URL DSP_API_KEY DSP_EMAIL DSP_PASSWORD DSP_PASSPHRASE DSP_CA_FILE")
}

func envOr(getenv getenvFunc, key, fallback string) string {
	if getenv == nil {
		return fallback
	}
	if v, ok := getenv(key); ok && v != "" {
		return v
	}
	return fallback
}

func splitFlags(args []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			out[name] = args[i+1]
			i++
			continue
		}
		out[name] = "true"
	}
	return out
}
