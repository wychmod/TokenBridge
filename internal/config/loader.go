package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("LG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 18743)
	v.SetDefault("server.admin_path", "/admin")
	v.SetDefault("server.auto_open_admin", true)
	v.SetDefault("server.prefer_browser", "chrome")
	v.SetDefault("server.read_timeout", 15)
	v.SetDefault("server.write_timeout", 120)
	v.SetDefault("server.idle_timeout", 120)
	v.SetDefault("proxy.request_timeout", 120)
	v.SetDefault("proxy.stream_timeout", 300)
	v.SetDefault("proxy.max_retries", 2)
	v.SetDefault("proxy.retry_delay_ms", 500)
	v.SetDefault("logging.level", "standard")
	v.SetDefault("database.path", "./data/tokenbridge.db")
	v.SetDefault("routing.default_strategy", "priority")
}
