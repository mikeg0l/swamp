package main

import (
	"fmt"
	"os"

	"swamp/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.ExecuteWithVersion(version); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
