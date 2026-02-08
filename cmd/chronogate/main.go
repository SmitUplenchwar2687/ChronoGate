package main

import (
	"os"

	"github.com/SmitUplenchwar2687/ChronoGate/internal/chronogatecli"
)

func main() {
	if err := chronogatecli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
