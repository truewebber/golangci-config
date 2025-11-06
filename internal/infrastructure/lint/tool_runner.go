package lint

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type ToolRunner struct{}

func NewToolRunner() *ToolRunner {
	return &ToolRunner{}
}

func (t *ToolRunner) EnsureAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"go",
		"tool",
		"-n",
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec run: %w", err)
	}

	return nil
}

func (t *ToolRunner) Run(ctx context.Context, args []string) error {
	commandArgs := append([]string{
		"tool",
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
	}, args...)

	cmd := exec.CommandContext(ctx, "go", commandArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec run: %w", err)
	}

	return nil
}
