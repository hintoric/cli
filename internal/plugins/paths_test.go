package plugins

import (
	"runtime"
	"testing"
)

func TestBinaryExtension(t *testing.T) {
	got := BinaryExtension()
	if runtime.GOOS == "windows" && got != ".exe" {
		t.Errorf("windows: got %q, want .exe", got)
	}
	if runtime.GOOS != "windows" && got != "" {
		t.Errorf("non-windows: got %q, want empty", got)
	}
}

func TestInstallPath(t *testing.T) {
	got := InstallPath("/cfg", "pipeline", "0.1.0")
	want := "/cfg/plugins/pipeline/0.1.0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBinaryPath(t *testing.T) {
	got := BinaryPath("/cfg", "pipeline", "0.1.0", "hint-pipeline")
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	want := "/cfg/plugins/pipeline/0.1.0/hint-pipeline" + suffix
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSelectLatestRelease(t *testing.T) {
	p := &Plugin{Releases: []Release{
		{Version: "0.1.0", OS: "darwin", Arch: "arm64", Sum: "a"},
		{Version: "0.2.0", OS: "darwin", Arch: "arm64", Sum: "b"},
		{Version: "0.2.0", OS: "linux", Arch: "amd64", Sum: "c"},
	}}
	r := SelectLatestRelease(p, "darwin", "arm64")
	if r == nil || r.Version != "0.2.0" {
		t.Fatalf("got %+v, want 0.2.0/darwin/arm64", r)
	}
}

func TestSelectExactRelease(t *testing.T) {
	p := &Plugin{Releases: []Release{
		{Version: "0.1.0", OS: "darwin", Arch: "arm64", Sum: "a"},
		{Version: "0.2.0", OS: "darwin", Arch: "arm64", Sum: "b"},
	}}
	r := SelectExactRelease(p, "0.1.0", "darwin", "arm64")
	if r == nil || r.Sum != "a" {
		t.Fatalf("got %+v, want 0.1.0/sum=a", r)
	}
	if SelectExactRelease(p, "9.9.9", "darwin", "arm64") != nil {
		t.Fatal("expected nil for missing version")
	}
}
