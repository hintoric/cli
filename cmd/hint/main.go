package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/hintoric/cli/internal/cmd"
)

//go:embed pubkey.pem
var pubKeyPEM []byte

var version = "0.0.0-dev"

func main() {
	if err := cmd.Root(version, pubKeyPEM).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
