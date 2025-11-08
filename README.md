# golangcix

A wrapper for `golangci-lint` that merges remote base configurations with local overrides, enabling centralized linting standards across multiple projects while allowing project-specific customizations.

## Why?

Managing consistent `golangci-lint` configurations across multiple projects is tedious—you end up copying configs, dealing with drift, and struggling to keep everything in sync. This wrapper lets you store a shared base configuration remotely (e.g., in a company repo) and override it per-project. Local values always win, and everything is cached for offline use.

## Install & Quick Start

```bash
go install github.com/truewebber/golangcix/cmd/golangcix@latest
```

Create `.golangci.local.yml` with a remote config directive:

```yaml
# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/common/.golangci.base.yml

linters:
  enable:
    - gosec
```

Then use it exactly like `golangci-lint`:

```bash
golangcix run ./...
golangcix run --fix
```

The wrapper automatically downloads the remote config, merges it with your local file, and passes the result to `golangci-lint`. 

**Requirements:** Go ≥ 1.25 and `golangci-lint` v2 (must be either in the `tool` section of your project's `go.mod` or available in PATH).

## Configuration

The wrapper searches for config files in this order: `.golangci.local.yml`, `.golangci.local.yaml`, `.golangci.yml`, `.golangci.yaml`. If the remote directive is missing or download fails, it falls back to local-only. Remote configs are cached in `~/.cache/golangcix` with ETag support.

### Using via `go tool`

To use both the wrapper and `golangci-lint` via `go tool`, add them to the `tool` section in your `go.mod`:

```go
tool (
	github.com/golangci/golangci-lint/v2/cmd/golangci-lint
	github.com/truewebber/golangcix/cmd/golangcix
)
```

Then run `go mod tidy` and use:

```bash
go tool github.com/truewebber/golangcix/cmd/golangcix run ./...
```

The wrapper will automatically detect and use `golangci-lint` via `go tool` as well.

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

Powered by [truewebber](https://truewebber.com)
