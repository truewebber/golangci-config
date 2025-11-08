package lint

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const (
	golangciLintToolPath = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	golangciLintBinary   = "golangci-lint"
)

type ToolRunner struct {
	useGoTool bool
}

func NewToolRunner() *ToolRunner {
	return &ToolRunner{}
}

func (t *ToolRunner) EnsureAvailable(ctx context.Context) error {
	if t.checkGoToolAvailable(ctx) {
		t.useGoTool = true

		return nil
	}

	if err := t.checkBinaryInPath(ctx); err != nil {
		return fmt.Errorf("golangci-lint not found: neither via 'go tool' nor in PATH: %w", err)
	}

	t.useGoTool = false

	return nil
}

func (t *ToolRunner) Run(ctx context.Context, args []string) error {
	cmd, err := t.buildCommand(ctx, args)
	if err != nil {
		return err
	}

	return t.executeCommand(cmd)
}

func (t *ToolRunner) checkGoToolAvailable(ctx context.Context) bool {
	// First check if the tool path exists
	if !t.checkGoToolPathExists(ctx) {
		return false
	}

	// Then try to actually run the tool to verify it's compilable and executable
	return t.checkGoToolRunnable(ctx)
}

func (t *ToolRunner) checkGoToolPathExists(ctx context.Context) bool {
	cmd := exec.CommandContext(
		ctx,
		"go",
		"tool",
		"-n",
		golangciLintToolPath,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	return cmd.Run() == nil
}

func (t *ToolRunner) checkGoToolRunnable(ctx context.Context) bool {
	cmd := exec.CommandContext(
		ctx,
		"go",
		"tool",
		golangciLintToolPath,
		"--version",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	return cmd.Run() == nil
}

func (t *ToolRunner) checkBinaryInPath(ctx context.Context) error {
	path, err := exec.LookPath(golangciLintBinary)
	if err != nil {
		return fmt.Errorf("lookup failed: %w", err)
	}

	if verifyErr := t.verifyBinaryExecutable(ctx, path); verifyErr != nil {
		return fmt.Errorf("verification failed: %w", verifyErr)
	}

	return nil
}

func (t *ToolRunner) verifyBinaryExecutable(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("golangci-lint found at %s but not executable: %w", path, err)
	}

	return nil
}

func (t *ToolRunner) buildCommand(ctx context.Context, args []string) (*exec.Cmd, error) {
	if t.useGoTool {
		return t.buildGoToolCommand(ctx, args), nil
	}

	return t.buildBinaryCommand(ctx, args)
}

func (t *ToolRunner) buildGoToolCommand(ctx context.Context, args []string) *exec.Cmd {
	commandArgs := append([]string{
		"tool",
		golangciLintToolPath,
	}, args...)

	//nolint:gosec // G204: commandArgs are controlled by the caller
	cmd := exec.CommandContext(ctx, "go", commandArgs...)

	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	return cmd
}

func (t *ToolRunner) buildBinaryCommand(ctx context.Context, args []string) (*exec.Cmd, error) {
	path, err := exec.LookPath(golangciLintBinary)
	if err != nil {
		return nil, fmt.Errorf("golangci-lint not found in PATH: %w", err)
	}

	//nolint:gosec // G204: args are controlled by the caller
	return exec.CommandContext(ctx, path, args...), nil
}

func (t *ToolRunner) executeCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec run: %w", err)
	}

	return nil
}
