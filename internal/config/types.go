package config

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Proxy    ProxyConfig    `mapstructure:"proxy"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Database DatabaseConfig `mapstructure:"database"`
	Routing  RoutingConfig  `mapstructure:"routing"`
}

type ServerConfig struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	AdminPath     string `mapstructure:"admin_path"`
	AutoOpenAdmin bool   `mapstructure:"auto_open_admin"`
	PreferBrowser string `mapstructure:"prefer_browser"`
	ReadTimeout   int    `mapstructure:"read_timeout"`
	WriteTimeout  int    `mapstructure:"write_timeout"`
	IdleTimeout   int    `mapstructure:"idle_timeout"`
}

type ProxyConfig struct {
	RequestTimeout int `mapstructure:"request_timeout"`
	StreamTimeout  int `mapstructure:"stream_timeout"`
	MaxRetries     int `mapstructure:"max_retries"`
	RetryDelayMS   int `mapstructure:"retry_delay_ms"`
}

type SecurityConfig struct {
	APIKeyEncryption  string   `mapstructure:"api_key_encryption"`
	EncryptionKeyFile string   `mapstructure:"encryption_key_file"`
	CORSEnabled       bool     `mapstructure:"cors_enabled"`
	AllowedOrigins    []string `mapstructure:"allowed_origins"`
}

type LoggingConfig struct {
	Level         string `mapstructure:"level"`
	RetentionDays int    `mapstructure:"retention_days"`
	MaxLogSizeMB  int    `mapstructure:"max_log_size_mb"`
	LogPrompts    bool   `mapstructure:"log_prompts"`
}

type DatabaseConfig struct {
	Path           string `mapstructure:"path"`
	WALMode        bool   `mapstructure:"wal_mode"`
	AutoVacuum     bool   `mapstructure:"auto_vacuum"`
	BackupInterval string `mapstructure:"backup_interval"`
}

type RoutingConfig struct {
	DefaultStrategy string         `mapstructure:"default_strategy"`
	Fallback        FallbackConfig `mapstructure:"fallback"`
}

type FallbackConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	MaxRetries      int      `mapstructure:"max_retries"`
	CooldownSeconds int      `mapstructure:"cooldown_seconds"`
	RetryOn         []string `mapstructure:"retry_on"`
}
