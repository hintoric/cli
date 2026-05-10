// hint-echo is a minimal hint plugin used in integration tests. It writes its
// args (one per line) to ctx.Stdout and exits 0, unless the first arg is
// "fail", in which case it exits 7.
package main

import (
	"context"
	"fmt"
	"strings"

	hcplugin "github.com/hashicorp/go-plugin"

	"github.com/hintoric/cli/internal/plugins"
	"github.com/hintoric/cli/internal/plugins/proto"
)

const cookie = "h1nt-ech0-c00k1e"

type impl struct{}

func (impl) RunCommand(_ context.Context, _ *proto.AdditionalInfo, args []string, pctx *plugins.PluginContext) (int32, error) {
	if len(args) > 0 && args[0] == "fail" {
		return 7, nil
	}
	fmt.Fprintln(pctx.Stdout, strings.Join(args, "\n"))
	return 0, nil
}

func main() {
	hcplugin.Serve(&hcplugin.ServeConfig{
		HandshakeConfig:  plugins.HandshakeConfig("echo", cookie),
		VersionedPlugins: plugins.PluginSet(impl{}),
		GRPCServer:       hcplugin.DefaultGRPCServer,
	})
}
