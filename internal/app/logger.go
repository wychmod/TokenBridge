package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

const (
	runtimeLogFileName = "tokenbridge.log"
	errorLogFileName   = "tokenbridge.err.log"
)

type runtimeLogWriter struct {
	all io.Writer
	err io.Writer
}

func (w runtimeLogWriter) Write(p []byte) (int, error) {
	return w.all.Write(p)
}

func (w runtimeLogWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	n, writeErr := w.all.Write(p)
	if level >= zerolog.ErrorLevel {
		if _, err := w.err.Write(p); writeErr == nil {
			writeErr = err
		}
	}
	return n, writeErr
}

func newRuntimeLogger(logDir string, console io.Writer) (zerolog.Logger, func() error, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("create log dir: %w", err)
	}

	appLog, err := os.OpenFile(filepath.Join(logDir, runtimeLogFileName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("open runtime log: %w", err)
	}

	errLog, err := os.OpenFile(filepath.Join(logDir, errorLogFileName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		_ = appLog.Close()
		return zerolog.Logger{}, nil, fmt.Errorf("open error log: %w", err)
	}

	fileWriter := runtimeLogWriter{all: appLog, err: errLog}
	writer := zerolog.MultiLevelWriter(console, fileWriter)
	logger := zerolog.New(writer).With().Timestamp().Str("service", "tokenbridge").Logger()

	closeLogs := func() error {
		return errors.Join(appLog.Close(), errLog.Close())
	}
	return logger, closeLogs, nil
}
