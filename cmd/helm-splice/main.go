package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/t63065488/helm-splice/pkg/splice"
)

func main() {
	envFlag := flag.String("env", "", "value to substitute for {{ env }} tokens")
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: helm-splice [--env ENV] <values-file-or-uri>")
		fmt.Fprintln(os.Stderr, "       helm-splice <certFile> <keyFile> <caFile> <splice-uri>")
		os.Exit(2)
	}

	// Helm downloader plugins pass arguments as: <certFile> <keyFile> <caFile> <full-URI>
	// The target URI/path is always the last positional argument.
	rawURI := args[len(args)-1]
	path, uriEnv := splice.CleanURI(rawURI)

	// Environment precedence: URI query param (?env=) > --env flag > HELM_SPLICE_ENV > SPLICE_ENV
	env := uriEnv
	if env == "" {
		env = *envFlag
	}
	if env == "" {
		env = os.Getenv("HELM_SPLICE_ENV")
	}
	if env == "" {
		env = os.Getenv("SPLICE_ENV")
	}

	out, err := splice.ResolveFileToYAML(path, splice.Options{Env: env})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintln(os.Stderr, "writing output:", err)
		os.Exit(1)
	}
}
