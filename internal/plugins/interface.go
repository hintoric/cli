package plugins

import (
	"context"
	"io"
	"sync"

	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/hintoric/cli/internal/plugins/proto"
)

// Dispatcher is what plugin implementers provide.
//
//   - ctx     — propagates host cancellation. Plugins doing long-running work
//     should observe ctx.Done() and abort.
//   - info    — terminal state, dimensions, filtered env from the host.
//   - args    — argv as the user typed (after `hint <plugin>`).
//   - pctx    — stdout/stderr writers proxied back to the host's terminal.
type Dispatcher interface {
	RunCommand(ctx context.Context, info *proto.AdditionalInfo, args []string, pctx *PluginContext) (int32, error)
}

// PluginContext bundles the streams a plugin is allowed to write to. Both
// writers proxy bytes back to the host via CoreCLIHelper.Print, so any
// fmt.Fprint / fmt.Fprintln targeting these reaches the user's terminal.
type PluginContext struct {
	Stdout io.Writer
	Stderr io.Writer
}

// HintPluginV1 is the hcplugin glue type for Dispatcher over gRPC.
type HintPluginV1 struct {
	hcplugin.Plugin
	Impl Dispatcher
}

// GRPCServer is called by hcplugin to stand up the plugin-side gRPC server.
// The plugin process registers HintPluginServer; its RunCommand handler
// dials the host's CoreCLIHelper using the broker id in the request.
func (p *HintPluginV1) GRPCServer(broker *hcplugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterHintPluginServer(s, &serverV1{impl: p.Impl, broker: broker})
	return nil
}

// GRPCClient is called by hcplugin on the host to obtain a typed client.
func (p *HintPluginV1) GRPCClient(_ context.Context, broker *hcplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &clientV1{client: proto.NewHintPluginClient(c), broker: broker}, nil
}

// clientV1 is the host-side proxy. Each RunCommand call spins up a
// CoreCLIHelper server on the broker for the lifetime of the call.
type clientV1 struct {
	client proto.HintPluginClient
	broker *hcplugin.GRPCBroker
}

// RunCommand performs the round-trip RPC to the plugin process.
// stdout/stderr are the writers the helper service forwards Print calls to.
func (c *clientV1) RunCommand(ctx context.Context, info *proto.AdditionalInfo, args []string, stdout, stderr io.Writer) (int32, error) {
	helper := &helperServer{stdout: stdout, stderr: stderr}

	var srv *grpc.Server
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		srv = grpc.NewServer(opts...)
		proto.RegisterCoreCLIHelperServer(srv, helper)
		return srv
	}

	brokerID := c.broker.NextId()
	go c.broker.AcceptAndServe(brokerID, serverFunc)
	// Defer Stop so a panic / RPC error mid-RunCommand still tears down
	// the helper goroutine. (Stripe's V3 leaks on the error path; we don't.)
	defer func() {
		if srv != nil {
			srv.Stop()
		}
	}()

	resp, err := c.client.RunCommand(ctx, &proto.RunCommandRequest{
		Info:           info,
		Args:           args,
		HelperBrokerId: brokerID,
	})
	if err != nil {
		return -1, err
	}
	return resp.GetExitCode(), nil
}

// serverV1 is the plugin-side gRPC handler.
type serverV1 struct {
	proto.UnimplementedHintPluginServer
	impl   Dispatcher
	broker *hcplugin.GRPCBroker
}

func (s *serverV1) RunCommand(ctx context.Context, req *proto.RunCommandRequest) (*proto.RunCommandResponse, error) {
	conn, err := s.broker.Dial(req.GetHelperBrokerId())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	helperClient := proto.NewCoreCLIHelperClient(conn)
	pluginCtx := &PluginContext{
		Stdout: &helperWriter{ctx: ctx, client: helperClient, stream: proto.Stream_STREAM_STDOUT},
		Stderr: &helperWriter{ctx: ctx, client: helperClient, stream: proto.Stream_STREAM_STDERR},
	}

	code, err := s.impl.RunCommand(ctx, req.GetInfo(), req.GetArgs(), pluginCtx)
	if err != nil {
		return nil, err
	}
	return &proto.RunCommandResponse{ExitCode: code}, nil
}

// helperServer runs in the host process; it receives Print() RPCs from the
// plugin and writes the bytes to whichever writer the host configured.
// A mutex serializes writes so concurrent unary RPCs from the plugin don't
// interleave bytes mid-line.
type helperServer struct {
	proto.UnimplementedCoreCLIHelperServer
	stdout io.Writer
	stderr io.Writer
	mu     sync.Mutex
}

func (h *helperServer) Print(_ context.Context, req *proto.PrintRequest) (*proto.PrintResponse, error) {
	w := h.stdout
	if req.GetStream() == proto.Stream_STREAM_STDERR {
		w = h.stderr
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, err := w.Write(req.GetData()); err != nil {
		return nil, err
	}
	return &proto.PrintResponse{}, nil
}

// helperWriter is the plugin-side io.Writer. fmt.Fprintln(ctx.Stdout, ...) →
// Write(p) → CoreCLIHelper.Print → host writes to its os.Stdout.
type helperWriter struct {
	ctx    context.Context
	client proto.CoreCLIHelperClient
	stream proto.Stream
}

func (w *helperWriter) Write(p []byte) (int, error) {
	if _, err := w.client.Print(w.ctx, &proto.PrintRequest{
		Data:   p,
		Stream: w.stream,
	}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// HandshakeConfig builds the cookie config for a given plugin shortname/cookie.
func HandshakeConfig(shortname, cookieValue string) hcplugin.HandshakeConfig {
	return hcplugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "plugin_" + shortname,
		MagicCookieValue: cookieValue,
	}
}

// PluginSet builds the plugin set map keyed by ProtocolVersion=1.
func PluginSet(impl Dispatcher) map[int]hcplugin.PluginSet {
	return map[int]hcplugin.PluginSet{
		1: {"main": &HintPluginV1{Impl: impl}},
	}
}
