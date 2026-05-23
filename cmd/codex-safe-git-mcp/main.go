package main

import (
	"os"

	"github.com/K1-R1/codex-safe-git/internal/mcp"
)

func main() {
	if code, handled := handleArgs(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	os.Exit(mcp.Main(os.Stdin, os.Stdout))
}
