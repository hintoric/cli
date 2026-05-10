# @hintoric/cli

The global command-line tool for [Hintoric](https://hintoric.com), distributed via npm.

```bash
npm install -g @hintoric/cli
hint --help

# or one-shot via npx:
npx @hintoric/cli version
```

This package is a thin wrapper around a native `hint` binary. The actual binary
is shipped via per-platform optional dependencies:

| Platform           | Package                          |
| ------------------ | -------------------------------- |
| macOS arm64        | `@hintoric/cli-darwin-arm64`     |
| macOS x64          | `@hintoric/cli-darwin-x64`       |
| Linux arm64        | `@hintoric/cli-linux-arm64`      |
| Linux x64          | `@hintoric/cli-linux-x64`        |
| Windows x64        | `@hintoric/cli-windows-x64`      |

npm installs only the package matching your `os` / `cpu` at install time. No
postinstall download, no shell scripts.

## Source & docs

Repo: <https://github.com/hintoric/cli>
License: [MIT](https://github.com/hintoric/cli/blob/main/LICENSE)
