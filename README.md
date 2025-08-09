# Golangci-lint Configuration Manager

A transparent proxy for golangci-lint that automatically manages configuration by merging base and local YAML files.

## Why Use This?

- **Consistent configuration** across all your Go projects
- **Automatic updates** of base linting rules without manual intervention
- **Local customization** while maintaining team standards
- **Zero configuration** for new projects - just run and it works
- **Transparent usage** - works exactly like regular golangci-lint

## Installation

1. Copy `golangci-lint.sh` to your desired location (e.g., `/usr/local/bin/`)
2. Make it executable:
   ```bash
   chmod +x golangci-lint.sh
   ```

## Prerequisites

Install the required dependencies:

```bash
# macOS
brew install golangci-lint yq curl

# Or via Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/mikefarah/yq/v4@latest
```

## Usage

Use it exactly like `golangci-lint`, but with automatic configuration management:

```bash
# Basic usage
./golangci-lint.sh run

# With custom local config
./golangci-lint.sh run -c my-config.yml

# With additional flags
./golangci-lint.sh run --fix --verbose
```

## Configuration

The tool automatically:
1. Downloads/updates base configuration from the remote repository
2. Looks for local configuration (`.golangci.local.yml` or `.golangci.local.yaml`)
3. Merges them into `.golangci.generated.yml`
4. Runs golangci-lint with the merged configuration

Create `.golangci.local.yml` in your project to customize settings:

```yaml
linters:
  enable:
    - gosec
  disable:
    - gofmt

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gosec
```

## Environment Variables

- `GOLANGCI_FORCE_UPDATE=1` - Force update base configuration
- `GOLANGCI_SKIP_UPDATE=1` - Skip configuration update

## Cleanup

Remove cache and generated files:

```bash
./golangci-lint.sh --cleanup
```

## Help

```bash
./golangci-lint.sh --help
```
