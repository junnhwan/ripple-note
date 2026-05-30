package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultAppName = "ripple-note"
	defaultEnv     = "local"
	defaultPort    = 8080
)

type Config struct {
	App    AppConfig    `yaml:"app"`
	HTTP   HTTPConfig   `yaml:"http"`
	Log    LogConfig    `yaml:"log"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	Auth   AuthConfig   `yaml:"auth"`
	Review ReviewConfig `yaml:"review"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type HTTPConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type MySQLConfig struct {
	Enabled               bool          `yaml:"enabled"`
	DSN                   string        `yaml:"dsn"`
	MaxOpenConnections    int           `yaml:"max_open_connections"`
	MaxIdleConnections    int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

type AuthConfig struct {
	JWTSecret string        `yaml:"jwt_secret"`
	JWTIssuer string        `yaml:"jwt_issuer"`
	JWTTTL    time.Duration `yaml:"jwt_ttl"`
}

type ReviewConfig struct {
	InternalToken string `yaml:"internal_token"`
}

func Load(path string) (Config, error) {
	cfg := defaultConfig()
	if path == "" {
		return cfg, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}
	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535, got %d", c.HTTP.Port)
	}
	if c.MySQL.Enabled && c.MySQL.DSN == "" {
		return errors.New("mysql.dsn is required when mysql.enabled is true")
	}
	if c.Auth.JWTSecret == "" {
		return errors.New("auth.jwt_secret is required")
	}
	return nil
}

func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.HTTP.Port)
}

func defaultConfig() Config {
	cfg := Config{}
	applyDefaults(&cfg)
	return cfg
}

func applyDefaults(cfg *Config) {
	if cfg.App.Name == "" {
		cfg.App.Name = defaultAppName
	}
	if cfg.App.Env == "" {
		cfg.App.Env = defaultEnv
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = defaultPort
	}
	if cfg.HTTP.ReadTimeout == 0 {
		cfg.HTTP.ReadTimeout = 5 * time.Second
	}
	if cfg.HTTP.WriteTimeout == 0 {
		cfg.HTTP.WriteTimeout = 10 * time.Second
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.MySQL.MaxOpenConnections == 0 {
		cfg.MySQL.MaxOpenConnections = 20
	}
	if cfg.MySQL.MaxIdleConnections == 0 {
		cfg.MySQL.MaxIdleConnections = 10
	}
	if cfg.MySQL.ConnectionMaxLifetime == 0 {
		cfg.MySQL.ConnectionMaxLifetime = time.Hour
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "local-dev-jwt-secret-change-me"
	}
	if cfg.Auth.JWTIssuer == "" {
		cfg.Auth.JWTIssuer = defaultAppName
	}
	if cfg.Auth.JWTTTL == 0 {
		cfg.Auth.JWTTTL = 24 * time.Hour
	}
}
