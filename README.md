# Golangci Wrapper

Go-based helper that generates a merged configuration for `golangci-lint` before running `golangci-lint run`. The wrapper expects you to keep the project-specific configuration locally and store the remote base configuration URL inside that file.

## Installation

1. Install the wrapper (pick any version tag you maintain):
   ```bash
   go install github.com/truewebber/golangci-config/cmd/golangci-wrapper@latest
   ```
2. Install the actual linter using the version you want to standardise on:
   ```bash
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
   ```

## Local configuration layout

Create a local config file (default search order: `.golangci.local.yml`, `.golangci.local.yaml`, `.golangci.yml`, `.golangci.yaml`) and add a comment with the remote base configuration directive:

```yaml
// GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/common/.golangci.base.yml

run:
  tests: false

linters:
  enable:
    - gosec
```

During execution the wrapper downloads the remote configuration, merges it with the local file (local values win), writes `.golangci.generated.yml` next to the local config, and passes that file to `golangci-lint`.

If the directive is missing or the download fails, the wrapper warns and proceeds with the local configuration only. Remote files are cached in `~/.cache/golangci-wrapper` and reused when the network is unavailable.

## Usage

```bash
golangci-wrapper run ./...
golangci-wrapper run --fix
golangci-wrapper run -c internal/config/.golangci.local.yml ./...
```

The wrapper only strips `-c/--config` flags to inject the generated file; all other arguments are forwarded to the underlying `golangci-lint`.

## Toolchain management

To pin tool versions in `go.mod`, add a `tools/tools.go` file:

```go
//go:build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
```

Running `go mod tidy` then locks in the linter version. Developers install or update both tools with:

```bash
go install github.com/truewebber/golangci-config/cmd/golangci-wrapper@<tag>
go install github.com/golangci/golangci-lint/cmd/golangci-lint@<version>
```
