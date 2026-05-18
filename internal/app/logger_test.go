package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRuntimeLoggerWritesApplicationLogsToLogDir(t *testing.T) {
	logDir := t.TempDir()
	var console bytes.Buffer

	logger, closeLogs, err := newRuntimeLogger(logDir, &console)
	if err != nil {
		t.Fatalf("newRuntimeLogger() error = %v", err)
	}

	logger.Info().Msg("startup complete")
	logger.Error().Msg("shutdown failed")
	if err := closeLogs(); err != nil {
		t.Fatalf("closeLogs() error = %v", err)
	}

	appLog := readTestLog(t, filepath.Join(logDir, "tokenbridge.log"))
	if !strings.Contains(appLog, "startup complete") || !strings.Contains(appLog, "shutdown failed") {
		t.Fatalf("tokenbridge.log should contain info and error logs, got: %s", appLog)
	}

	errLog := readTestLog(t, filepath.Join(logDir, "tokenbridge.err.log"))
	if strings.Contains(errLog, "startup complete") {
		t.Fatalf("tokenbridge.err.log should not contain info logs, got: %s", errLog)
	}
	if !strings.Contains(errLog, "shutdown failed") {
		t.Fatalf("tokenbridge.err.log should contain error logs, got: %s", errLog)
	}

	consoleLog := console.String()
	if !strings.Contains(consoleLog, "startup complete") || !strings.Contains(consoleLog, "shutdown failed") {
		t.Fatalf("console should still receive runtime logs, got: %s", consoleLog)
	}
}

func readTestLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
