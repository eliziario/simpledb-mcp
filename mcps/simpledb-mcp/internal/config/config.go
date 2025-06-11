package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Connections map[string]Connection `yaml:"connections"`
	Settings    Settings              `yaml:"settings"`
}

type Connection struct {
	Type     string `yaml:"type"`     // mysql, postgres
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode,omitempty"` // for postgres
	Username string `yaml:"username,omitempty"` // optional, can be stored in keychain
}

type Settings struct {
	QueryTimeout     time.Duration `yaml:"query_timeout"`
	MaxRows          int           `yaml:"max_rows"`
	CacheCredentials time.Duration `yaml:"cache_credentials"`
	RequireBiometric bool          `yaml:"require_biometric"`
	
	// Connection pool settings
	ConnectionPool ConnectionPoolSettings `yaml:"connection_pool"`
	
	// Server settings
	Server ServerSettings `yaml:"server"`
}

type ConnectionPoolSettings struct {
	PingInterval    time.Duration `yaml:"ping_interval"`
	MaxIdleTime     time.Duration `yaml:"max_idle_time"`
	MaxErrorCount   int           `yaml:"max_error_count"`
	ReconnectDelay  time.Duration `yaml:"reconnect_delay"`
	EnableKeepalive bool          `yaml:"enable_keepalive"`
}

type ServerSettings struct {
	Transport string `yaml:"transport"` // stdio, http, gin
	Address   string `yaml:"address"`   // for http/gin transport (e.g., ":8080")
	Path      string `yaml:"path"`      // endpoint path for http/gin (e.g., "/mcp")
}

func DefaultConfig() *Config {
	return &Config{
		Connections: make(map[string]Connection),
		Settings: Settings{
			QueryTimeout:     30 * time.Second,
			MaxRows:          1000,
			CacheCredentials: 5 * time.Minute,
			RequireBiometric: true,
			ConnectionPool: ConnectionPoolSettings{
				PingInterval:    30 * time.Second,
				MaxIdleTime:     15 * time.Minute,
				MaxErrorCount:   3,
				ReconnectDelay:  5 * time.Second,
				EnableKeepalive: true,
			},
			Server: ServerSettings{
				Transport: "stdio",
				Address:   ":8080",
				Path:      "/mcp",
			},
		},
	}
}

func ConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "simpledb-mcp")
	return configDir, nil
}

func ConfigPath() (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

func Load() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// If config doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func (c *Config) Save() error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) AddConnection(name string, conn Connection) error {
	if c.Connections == nil {
		c.Connections = make(map[string]Connection)
	}
	c.Connections[name] = conn
	return c.Save()
}

func (c *Config) RemoveConnection(name string) error {
	delete(c.Connections, name)
	return c.Save()
}

func (c *Config) GetConnection(name string) (Connection, bool) {
	conn, exists := c.Connections[name]
	return conn, exists
}

func (c *Config) ListConnections() []string {
	names := make([]string, 0, len(c.Connections))
	for name := range c.Connections {
		names = append(names, name)
	}
	return names
}