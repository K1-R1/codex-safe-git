package main

import (
	"os"

	"github.com/K1-R1/codex-safe-git/internal/mcp"
)

func main() {
	os.Exit(mcp.Main(os.Stdin, os.Stdout))
}
