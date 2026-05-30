package logger_test

import (
	"bytes"
	"strings"
	"testing"

	"nano-code-go/internal/infrastructure/logger"
)

func TestLoggerSeparatesOutputAndDiagnostics(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	log := logger.New(&stdout, &stderr)

	log.Output("answer")
	log.Info("starting")
	log.Warn("careful")
	log.Error("failed")

	if got := stdout.String(); got != "answer\n" {
		t.Fatalf("stdout = %q, want answer newline", got)
	}
	for _, want := range []string{"[info] starting", "[warn] careful", "[error] failed"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q in %q", want, stderr.String())
		}
	}
}

func TestLoggerDebugLevel(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	log := logger.New(nil, &stderr)

	log.Debug("hidden")
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty before debug enabled", stderr.String())
	}

	log.SetDebug(true)
	log.Debug("visible")
	if !strings.Contains(stderr.String(), "[debug] visible") {
		t.Fatalf("stderr = %q, want debug message", stderr.String())
	}
}
