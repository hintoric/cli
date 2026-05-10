// Command hint-hello is the first published plugin of the Hintoric CLI.
// It exists primarily to exercise the registry / sign / verify / dispatch
// pipeline end-to-end with a real (if trivial) consumer.
package main

import (
	"context"
	"fmt"
	"strings"

	hcplugin "github.com/hashicorp/go-plugin"

	"github.com/hintoric/cli/internal/plugins"
	"github.com/hintoric/cli/internal/plugins/proto"
)

// cookie is a fixed magic-cookie value the host uses to validate that this
// binary really is a hint plugin. Must match the MagicCookieValue declared
// for "hello" in the published plugins.toml.
const cookie = "h1nt-hell0-c00k1e-2026"

type impl struct{}

func (impl) RunCommand(_ context.Context, _ *proto.AdditionalInfo, args []string, pctx *plugins.PluginContext) (int32, error) {
	greeting := "world"
	if len(args) > 0 {
		greeting = strings.Join(args, " ")
	}
	// pctx.Stdout proxies via the host's CoreCLIHelper — bytes appear on the
	// user's terminal regardless of hcplugin's stdio capture.
	fmt.Fprintf(pctx.Stdout, "hello, %s — from the Hintoric CLI plugin demo\n", greeting)
	return 0, nil
}

func main() {
	hcplugin.Serve(&hcplugin.ServeConfig{
		HandshakeConfig:  plugins.HandshakeConfig("hello", cookie),
		VersionedPlugins: plugins.PluginSet(impl{}),
		GRPCServer:       hcplugin.DefaultGRPCServer,
	})
}
