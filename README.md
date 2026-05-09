# hint

The Hintoric command-line tool. A small static host binary with an extensible plugin system.

## Install

Download the archive for your platform from the [latest release](https://github.com/hintoric/cli/releases/latest), extract, and put `hint` on your `$PATH`:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/hintoric/cli/releases/latest/download/hint_0.1.0_macos_arm64.tar.gz | tar xz
sudo mv hint /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/hintoric/cli/releases/latest/download/hint_0.1.0_linux_x86_64.tar.gz | tar xz
sudo mv hint /usr/local/bin/
```

A Homebrew tap is on the roadmap.

## Usage

```bash
hint version              # version, Go runtime, platform
hint --help               # top-level help
hint plugin list          # list installed and available plugins
hint plugin install <name>
hint plugin update --all
hint plugin uninstall <name>
hint <plugin> [args...]   # invoke a plugin (lazy-installs on first use)
```

## How plugins work

`hint` itself is small. Functionality lives in plugins — separate binaries discovered through a signed manifest at `cli.hintoric.com`. Each plugin:

- declares its name, version, and platform binaries in a TOML manifest
- ships a sha256 hash per build for tamper detection
- speaks a gRPC dispatch protocol over stdio (HashiCorp `go-plugin`) so the host can invoke it with arguments and get an exit code back

The manifest is signed with an ECDSA P-256 key whose public half is embedded in the host binary. Every fetch is verified before any plugin is downloaded, and every download is sha256-checked before execution.

## Authoring a plugin

A plugin is a normal Go binary that calls `hcplugin.Serve` with the `hint` plugin contract. See the test fixture at [`testdata/hint-echo`](testdata/hint-echo) for a minimal example. Plugins for other languages need an hcplugin-compatible SDK (or a thin Go wrapper that shells out to your real binary).

## Building from source

Requires Go 1.22+.

```bash
git clone https://github.com/hintoric/cli && cd cli
make build
./bin/hint version
```

`make test` runs the full test suite. `goreleaser release --snapshot --clean` produces a local cross-platform release matching what CI ships.

## License

[MIT](LICENSE).
