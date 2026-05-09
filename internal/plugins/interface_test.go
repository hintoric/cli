package plugins

import (
	"testing"

	"github.com/hintoric/cli/internal/plugins/proto"
)

type fakeDispatcher struct {
	gotArgs []string
}

func (f *fakeDispatcher) RunCommand(_ *proto.AdditionalInfo, args []string) (int32, error) {
	f.gotArgs = args
	return 42, nil
}

func TestPluginSetWiring(t *testing.T) {
	d := &fakeDispatcher{}
	set := PluginSet(d)
	if _, ok := set[1]["main"]; !ok {
		t.Fatal("expected protocol 1 with 'main' plugin")
	}
	hc := HandshakeConfig("foo", "bar")
	if hc.MagicCookieKey != "plugin_foo" {
		t.Errorf("MagicCookieKey got %q", hc.MagicCookieKey)
	}
	if hc.MagicCookieValue != "bar" {
		t.Errorf("MagicCookieValue got %q", hc.MagicCookieValue)
	}
}
