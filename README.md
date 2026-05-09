# hint

The global command-line tool for [Hintoric](https://hintoric.com).

## Install

### macOS (Homebrew)

```bash
brew tap hintoric/tap
brew install hint
```

### Linux

```bash
curl -L https://github.com/hintoric/cli/releases/latest/download/hint_0.1.0_linux_x86_64.tar.gz | tar xz
sudo mv hint /usr/local/bin/
```

(replace `linux_x86_64` with `linux_arm64` on ARM)

### Windows

Download the [latest release](https://github.com/hintoric/cli/releases/latest) `*_windows_x86_64.zip`, extract, and place `hint.exe` on your `PATH`.

## Usage

```bash
hint --help
hint version
```

Run `hint --help` to see what's available in your version.

## Building from source

Requires Go 1.22+.

```bash
git clone https://github.com/hintoric/cli && cd cli
make build
./bin/hint version
```

`make test` runs the test suite.

## License

[MIT](LICENSE).
