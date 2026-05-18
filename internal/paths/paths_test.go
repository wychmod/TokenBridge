package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureUserDirsCreatesLogDir(t *testing.T) {
	tmp := t.TempDir()
	appPaths := AppPaths{
		ConfigDir: filepath.Join(tmp, "config"),
		DataDir:   filepath.Join(tmp, "data"),
		LogDir:    filepath.Join(tmp, "logs"),
	}

	if err := EnsureUserDirs(appPaths); err != nil {
		t.Fatalf("EnsureUserDirs() error = %v", err)
	}

	for _, dir := range []string{appPaths.ConfigDir, appPaths.DataDir, appPaths.LogDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected directory %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}
}
