package main

import (
	"fmt"
	"os"

	"aegion-dynamic/api-console/cli"
)

func main() {
	if err := cli.RunUI(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
