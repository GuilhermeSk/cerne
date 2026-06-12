package main

import (
	"os"

	"gitlab.com/acme/cerne/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		os.Exit(1)
	}
}
