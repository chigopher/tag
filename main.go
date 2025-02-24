package main

import (
	"os"

	"github.com/chigopher/tag/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
