package paths

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const AppDirName = "TokenBridge"

type AppPaths struct {
	BaseDir   string
	ConfigDir string
	DataDir   string
	LogDir    string
	Config    string
	Database  string
}

func Resolve() (AppPaths, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return AppPaths{}, fmt.Errorf("resolve user config dir: %w", err)
	}

	configDir := filepath.Join(baseDir, AppDirName)
	dataDir := filepath.Join(configDir, "data")
	return AppPaths{
		BaseDir:   configDir,
		ConfigDir: configDir,
		DataDir:   dataDir,
		LogDir:    filepath.Join(configDir, "logs"),
		Config:    filepath.Join(configDir, "config.yaml"),
		Database:  filepath.Join(dataDir, "tokenbridge.db"),
	}, nil
}

func EnsureUserDirs(appPaths AppPaths) error {
	if err := os.MkdirAll(appPaths.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(appPaths.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(appPaths.LogDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	return nil
}

func EnsureUserConfig(appPaths AppPaths) (string, error) {
	if err := EnsureUserDirs(appPaths); err != nil {
		return "", err
	}
	if fileExists(appPaths.Config) {
		return appPaths.Config, nil
	}

	for _, candidate := range []string{"config.yaml", filepath.Join("configs", "config.example.yaml")} {
		if fileExists(candidate) {
			if err := copyFile(candidate, appPaths.Config, 0o644); err != nil {
				return "", fmt.Errorf("copy config from %s: %w", candidate, err)
			}
			return appPaths.Config, nil
		}
	}

	if err := os.WriteFile(appPaths.Config, []byte(defaultConfigYAML), 0o644); err != nil {
		return "", fmt.Errorf("write default config: %w", err)
	}
	return appPaths.Config, nil
}

func ResolveUserDataPath(appPaths AppPaths, configuredPath string, defaultPath string) string {
	path := strings.TrimSpace(configuredPath)
	if path == "" {
		path = defaultPath
	}
	if filepath.IsAbs(path) {
		return path
	}
	path = strings.TrimPrefix(path, "."+string(filepath.Separator))
	path = strings.TrimPrefix(path, "./")
	return filepath.Join(appPaths.BaseDir, filepath.Clean(path))
}

func MigrateLegacyDatabase(targetPath string) error {
	if fileExists(targetPath) {
		return nil
	}

	legacyPath := filepath.Join("data", "tokenbridge.db")
	if !fileExists(legacyPath) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create target database dir: %w", err)
	}
	if err := copyFile(legacyPath, targetPath, 0o644); err != nil {
		return fmt.Errorf("migrate legacy database: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src string, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

const defaultConfigYAML = `server:
  host: "127.0.0.1"
  port: 18743
  admin_path: "/admin"
  auto_open_admin: true
  prefer_browser: "chrome"
  read_timeout: 15
  write_timeout: 120
  idle_timeout: 120

proxy:
  request_timeout: 120
  stream_timeout: 300
  max_retries: 2
  retry_delay_ms: 500

security:
  api_key_encryption: "aes-256-gcm"
  encryption_key_file: ".secret"
  cors_enabled: false
  allowed_origins: []

logging:
  level: "standard"
  retention_days: 30
  max_log_size_mb: 500
  log_prompts: false

database:
  path: "./data/tokenbridge.db"
  wal_mode: true
  auto_vacuum: true
  backup_interval: "24h"

routing:
  default_strategy: priority
  fallback:
    enabled: true
    max_retries: 2
    cooldown_seconds: 60
    retry_on:
      - rate_limit
      - server_error
      - timeout

providers: []
local_keys: []
`
