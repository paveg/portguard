package main

import (
	"os"

	"github.com/paveg/portguard/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
