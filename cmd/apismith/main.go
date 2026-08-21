package main

import (
	"os"

	"aegion-dynamic/api-console/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
