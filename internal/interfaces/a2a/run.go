package a2a

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
)

var ErrNotImplemented = errors.New("nano-code A2A server is not implemented yet")

type Env map[string]string

func EnvFromOS() Env {
	env := make(Env)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func Run(ctx context.Context, stdout, stderr io.Writer, env Env) error {
	_ = stdout
	_ = stderr
	_ = env

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrNotImplemented
	}
}
