package logger

import (
	"fmt"
	"io"
	"strings"
)

type Level int

const (
	LevelInfo Level = iota
	LevelDebug
)

type Logger struct {
	Out   io.Writer
	Err   io.Writer
	Level Level
}

func New(out, err io.Writer) *Logger {
	if out == nil {
		out = io.Discard
	}
	if err == nil {
		err = io.Discard
	}
	return &Logger{Out: out, Err: err, Level: LevelInfo}
}

func (l *Logger) SetDebug(enabled bool) {
	if enabled {
		l.Level = LevelDebug
		return
	}
	l.Level = LevelInfo
}

func (l *Logger) Output(args ...any) {
	_, _ = fmt.Fprintln(l.Out, args...)
}

func (l *Logger) Info(args ...any) {
	l.writeDiagnostic("info", args...)
}

func (l *Logger) Warn(args ...any) {
	l.writeDiagnostic("warn", args...)
}

func (l *Logger) Error(args ...any) {
	l.writeDiagnostic("error", args...)
}

func (l *Logger) Debug(args ...any) {
	if l.Level < LevelDebug {
		return
	}
	l.writeDiagnostic("debug", args...)
}

func (l *Logger) writeDiagnostic(level string, args ...any) {
	if l == nil || l.Err == nil {
		return
	}
	message := strings.TrimSuffix(fmt.Sprintln(args...), "\n")
	_, _ = fmt.Fprintf(l.Err, "[%s] %s\n", level, message)
}
