package main

import (
	"fmt"
	"os"

	"github.com/hintoric/cli/internal/buildcmd"
)

func main() {
	if err := buildcmd.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
