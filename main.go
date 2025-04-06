package main

import (
	"grain/cmd"
	"os"

	"grain/internal/cli"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Cobra typically handles errors, but catch any top-level ones
		cli.PrintError(err)
		os.Exit(1)
	}
}
