package main

import (
	"os"

	"local/codex-safe-git/internal/mcp"
)

func main() {
	os.Exit(mcp.Main(os.Stdin, os.Stdout))
}
