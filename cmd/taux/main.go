package main

import (
	"os"

	"github.com/glory0216/taux/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
