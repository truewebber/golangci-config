package lint

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type ToolRunner struct{}

func NewToolRunner() *ToolRunner {
	return &ToolRunner{}
}

func (t *ToolRunner) EnsureAvailable() error {
	cmd := exec.Command("go", "tool", "-n", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("golangci-lint via go tool is unavailable: %w", err)
	}
	return nil
}

func (t *ToolRunner) Run(args []string) error {
	commandArgs := append([]string{
		"tool",
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
	}, args...)

	cmd := exec.Command("go", commandArgs...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
