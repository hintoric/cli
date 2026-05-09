package plugins

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/hintoric/cli/internal/signer"
)

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return priv
}

const sampleManifest = `
SchemaVersion = 1
GeneratedAt = "2026-05-08T11:41:00Z"

[[Plugin]]
  Shortname = "pipeline"
  Shortdesc = "Run Hintoric pipelines"
  Binary = "hint-pipeline"
  MagicCookieValue = "h1nt-p1pel1ne-c00k1e"

  [[Plugin.Release]]
    Version = "0.1.0"
    OS = "darwin"
    Arch = "arm64"
    Sum = "f3a98b2cb1d4e8c6a9b3f7e2d5c1a4b8e6f9c2d5a8b1e4f7c9b6e3d8a5f2c1d4"

  [[Plugin.Release]]
    Version = "0.1.0"
    OS = "linux"
    Arch = "amd64"
    Sum = "abc123def456789abc123def456789abc123def456789abc123def456789abcd"

  [[Plugin.Command]]
    Name = "run"
    Desc = "Execute a pipeline"
`

func TestParseManifest(t *testing.T) {
	pl, err := ParseManifest([]byte(sampleManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pl.SchemaVersion != 1 {
		t.Errorf("SchemaVersion got %d", pl.SchemaVersion)
	}
	if len(pl.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pl.Plugins))
	}
	p := pl.Plugins[0]
	if p.Shortname != "pipeline" {
		t.Errorf("Shortname got %q", p.Shortname)
	}
	if p.Binary != "hint-pipeline" {
		t.Errorf("Binary got %q", p.Binary)
	}
	if len(p.Releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(p.Releases))
	}
	if len(p.Commands) != 1 || p.Commands[0].Name != "run" {
		t.Errorf("Commands wrong: %+v", p.Commands)
	}
}

func TestParseManifestRejectsBadTOML(t *testing.T) {
	_, err := ParseManifest([]byte("not [valid toml"))
	if err == nil {
		t.Fatal("expected error on malformed TOML")
	}
}

func TestValidateRejectsUnknownSchemaVersion(t *testing.T) {
	pl := &PluginList{SchemaVersion: 9999}
	if err := ValidateManifest(pl); err == nil {
		t.Fatal("expected schema version error")
	}
}

func TestValidateRejectsBadShortname(t *testing.T) {
	pl := &PluginList{SchemaVersion: 1, Plugins: []Plugin{
		{Shortname: "Bad-Name", Binary: "hint-x", MagicCookieValue: "x", Releases: []Release{
			{Version: "0.1.0", OS: "darwin", Arch: "arm64", Sum: "deadbeef"},
		}},
	}}
	if err := ValidateManifest(pl); err == nil {
		t.Fatal("expected shortname rejection")
	}
}

func TestValidateRejectsEmptyMagicCookie(t *testing.T) {
	pl := &PluginList{SchemaVersion: 1, Plugins: []Plugin{
		{Shortname: "ok", Binary: "hint-ok", MagicCookieValue: "", Releases: []Release{
			{Version: "0.1.0", OS: "darwin", Arch: "arm64", Sum: "deadbeef"},
		}},
	}}
	if err := ValidateManifest(pl); err == nil {
		t.Fatal("expected magic cookie rejection")
	}
}

func TestValidateRejectsMissingReleases(t *testing.T) {
	pl := &PluginList{SchemaVersion: 1, Plugins: []Plugin{
		{Shortname: "ok", Binary: "hint-ok", MagicCookieValue: "x"},
	}}
	if err := ValidateManifest(pl); err == nil {
		t.Fatal("expected missing-releases rejection")
	}
}

func TestValidateRejectsBadSum(t *testing.T) {
	pl := &PluginList{SchemaVersion: 1, Plugins: []Plugin{
		{Shortname: "ok", Binary: "hint-ok", MagicCookieValue: "x", Releases: []Release{
			{Version: "0.1.0", OS: "darwin", Arch: "arm64", Sum: "not-hex!!"},
		}},
	}}
	if err := ValidateManifest(pl); err == nil {
		t.Fatal("expected bad sum rejection")
	}
}

func TestValidateAcceptsGoodManifest(t *testing.T) {
	pl, err := ParseManifest([]byte(sampleManifest))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateManifest(pl); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestVerifyManifestSignatureOK(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	sig, err := signer.Sign(priv, body)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyManifestSignature(&priv.PublicKey, body, sig); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestVerifyManifestSignatureTampered(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	sig, err := signer.Sign(priv, body)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	body[0] ^= 0xff

	if err := VerifyManifestSignature(&priv.PublicKey, body, sig); err == nil {
		t.Fatal("expected verify to fail on tampered body")
	}
}
