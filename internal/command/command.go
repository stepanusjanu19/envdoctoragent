package command

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultTimeout = 5 * time.Second

type Options struct {
	Dir     string
	Timeout time.Duration
}

func Output(name string, args ...string) ([]byte, error) {
	return OutputWithOptions(Options{}, name, args...)
}

func OutputInDir(dir, name string, args ...string) ([]byte, error) {
	return OutputWithOptions(Options{Dir: dir}, name, args...)
}

func OutputWithOptions(options Options, name string, args ...string) ([]byte, error) {
	cmd, ctx, cancel := newCommand(options, name, args...)
	defer cancel()

	out, err := cmd.Output()
	return out, commandError(ctx, err, name, args)
}

func CombinedOutput(name string, args ...string) ([]byte, error) {
	return CombinedOutputWithOptions(Options{}, name, args...)
}

func CombinedOutputInDir(dir, name string, args ...string) ([]byte, error) {
	return CombinedOutputWithOptions(Options{Dir: dir}, name, args...)
}

func CombinedOutputWithOptions(options Options, name string, args ...string) ([]byte, error) {
	cmd, ctx, cancel := newCommand(options, name, args...)
	defer cancel()

	out, err := cmd.CombinedOutput()
	return out, commandError(ctx, err, name, args)
}

func newCommand(options Options, name string, args ...string) (*exec.Cmd, context.Context, context.CancelFunc) {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, name, args...)
	if options.Dir != "" {
		cmd.Dir = options.Dir
	}
	return cmd, ctx, cancel
}

func commandError(ctx context.Context, err error, name string, args []string) error {
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timed out: %s", commandString(name, args))
	}
	return err
}

func commandString(name string, args []string) string {
	parts := append([]string{name}, args...)
	return strings.Join(parts, " ")
}
