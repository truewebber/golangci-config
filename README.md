# Golangci Wrapper

Go-based helper that generates a merged configuration for `golangci-lint` before running `golangci-lint run`. The wrapper expects you to keep the project-specific configuration locally and store the remote base configuration URL inside that file.

## Installation

1. Install the wrapper (pick any version tag you maintain):
   ```bash
   go install github.com/truewebber/golangci-config/cmd/golangci-wrapper@latest
   ```
2. Ensure you have Go ≥ 1.25 installed. The wrapper uses `go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint`, so the linter is built on demand from source—no separate binary is required.

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

The wrapper only strips `-c/--config` flags to inject the generated file; all other arguments are forwarded to `go tool … golangci-lint`.

## Toolchain management

To pin the linter version in your repository, add a `tools/tools.go` file and keep it in sync with the wrapper:

```go
//go:build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
```

Running `go mod tidy` then locks in the linter version the wrapper will compile when invoking `go tool`.
