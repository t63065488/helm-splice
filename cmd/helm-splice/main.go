package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/t63065488/helm-splice/pkg/splice"
)

func main() {
	env := flag.String("env", "", "value to substitute for {{ env }} tokens")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: helm-splice [--env ENV] <values-file>")
		os.Exit(2)
	}
	path := args[0]
	out, err := splice.ResolveFileToYAML(path, splice.Options{Env: *env})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintln(os.Stderr, "writing output:", err)
		os.Exit(1)
	}
}
