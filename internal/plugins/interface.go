package plugins

import (
	"context"

	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/hintoric/cli/internal/plugins/proto"
)

// Dispatcher is what plugin implementers provide. The host calls RunCommand;
// the plugin returns the exit code it would have used as a standalone process.
type Dispatcher interface {
	RunCommand(info *proto.AdditionalInfo, args []string) (int32, error)
}

// HintPluginV1 is the hcplugin glue type for Dispatcher over gRPC.
// Plugin authors construct one of these in their main(), passing their Impl.
type HintPluginV1 struct {
	hcplugin.Plugin
	Impl Dispatcher
}

// GRPCServer is called by hcplugin to stand up the plugin-side gRPC server.
func (p *HintPluginV1) GRPCServer(_ *hcplugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterHintPluginServer(s, &serverV1{impl: p.Impl})
	return nil
}

// GRPCClient is called by hcplugin on the host to obtain a typed client.
func (p *HintPluginV1) GRPCClient(_ context.Context, _ *hcplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &clientV1{client: proto.NewHintPluginClient(c)}, nil
}

// clientV1 is the host-side proxy.
type clientV1 struct {
	client proto.HintPluginClient
}

// RunCommand performs the round-trip RPC to the plugin process.
func (c *clientV1) RunCommand(info *proto.AdditionalInfo, args []string) (int32, error) {
	resp, err := c.client.RunCommand(context.Background(), &proto.RunCommandRequest{
		Info: info,
		Args: args,
	})
	if err != nil {
		return -1, err
	}
	return resp.GetExitCode(), nil
}

// serverV1 is the plugin-side gRPC server adapter that wraps the plugin's Dispatcher.
type serverV1 struct {
	proto.UnimplementedHintPluginServer
	impl Dispatcher
}

func (s *serverV1) RunCommand(_ context.Context, req *proto.RunCommandRequest) (*proto.RunCommandResponse, error) {
	code, err := s.impl.RunCommand(req.GetInfo(), req.GetArgs())
	if err != nil {
		return nil, err
	}
	return &proto.RunCommandResponse{ExitCode: code}, nil
}

// HandshakeConfig builds the cookie config for a given plugin shortname/cookie.
func HandshakeConfig(shortname, cookieValue string) hcplugin.HandshakeConfig {
	return hcplugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "plugin_" + shortname,
		MagicCookieValue: cookieValue,
	}
}

// PluginSet builds the plugin set map keyed by ProtocolVersion=1 for the given Impl.
// Used both by host (Impl=nil, just for type info) and plugins (Impl=their dispatcher).
func PluginSet(impl Dispatcher) map[int]hcplugin.PluginSet {
	return map[int]hcplugin.PluginSet{
		1: {"main": &HintPluginV1{Impl: impl}},
	}
}
