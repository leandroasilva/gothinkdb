package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Config holds all configuration for the GoThinkDB server
type Config struct {
	// DataDir is the directory where database files are stored
	DataDir string `json:"data_dir"`

	// DriverAddress is the TCP address for the driver protocol (default: :28015)
	DriverAddress string `json:"driver_address"`

	// HTTPAddress is the address for the HTTP admin interface (default: :8080)
	HTTPAddress string `json:"http_address"`

	// ClusterAddress is the address for cluster communication (default: :29015)
	ClusterAddress string `json:"cluster_address"`

	// JoinAddress is the address of an existing cluster node to join
	JoinAddress string `json:"join_address,omitempty"`

	// ServerName is the unique name of this server in the cluster
	ServerName string `json:"server_name"`

	// ServerID is the unique identifier for this server
	ServerID string `json:"server_id"`

	// CacheSizeMB is the size of the page cache in megabytes
	CacheSizeMB int `json:"cache_size_mb"`

	// MaxConnections is the maximum number of concurrent client connections
	MaxConnections int `json:"max_connections"`

	// LogLevel is the logging level (debug, info, warn, error)
	LogLevel string `json:"log_level"`

	// AuthKey is the legacy authentication key (deprecated, use users)
	AuthKey string `json:"auth_key,omitempty"`

	// JWTSecret signs the HTTP admin API tokens. Required in production; set
	// via GOTHINKDB_JWT_SECRET.
	JWTSecret string `json:"jwt_secret,omitempty"`

	// ClusterSecret authenticates cluster (RPC) traffic. Set via
	// GOTHINKDB_CLUSTER_SECRET.
	ClusterSecret string `json:"cluster_secret,omitempty"`

	// AdminUser/AdminPassword bootstrap the initial admin account. Set via
	// GOTHINKDB_ADMIN_USER / GOTHINKDB_ADMIN_PASSWORD.
	AdminUser     string `json:"admin_user,omitempty"`
	AdminPassword string `json:"admin_password,omitempty"`

	// AllowDefaultAdmin permits the insecure admin/admin bootstrap (dev only),
	// enabled via GOTHINKDB_ALLOW_DEFAULT_ADMIN=1.
	AllowDefaultAdmin bool `json:"allow_default_admin,omitempty"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		DataDir:        "/data/gothinkdb",
		DriverAddress:  ":28015",
		HTTPAddress:    ":8080",
		ClusterAddress: ":29015",
		ServerName:     "gothinkdb_" + uuid.New().String()[:8],
		ServerID:       uuid.New().String(),
		CacheSizeMB:    1024,
		MaxConnections: 10000,
		LogLevel:       "info",
	}
}

// Load loads configuration from a file, falling back to defaults
func Load(configFile string) *Config {
	cfg := DefaultConfig()

	if configFile == "" {
		// Try default config file locations
		for _, path := range []string{
			"/etc/gothinkdb/config.json",
			filepath.Join(cfg.DataDir, "config.json"),
		} {
			if _, err := os.Stat(path); err == nil {
				configFile = path
				break
			}
		}
	}

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			slog.Warn("failed to read config file, using defaults", "file", configFile, "error", err)
			return cfg
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			slog.Warn("failed to parse config file, using defaults", "file", configFile, "error", err)
			return cfg
		}

		slog.Info("loaded configuration from file", "file", configFile)
	}

	applyEnvOverrides(cfg)

	return cfg
}

// applyEnvOverrides lets deployment (Docker/systemd) inject secrets and the
// bootstrap admin without writing them to a config file.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GOTHINKDB_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("GOTHINKDB_SERVER_NAME"); v != "" {
		cfg.ServerName = v
	}
	if v := os.Getenv("GOTHINKDB_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("GOTHINKDB_CLUSTER_SECRET"); v != "" {
		cfg.ClusterSecret = v
	}
	if v := os.Getenv("GOTHINKDB_ADMIN_USER"); v != "" {
		cfg.AdminUser = v
	}
	if v := os.Getenv("GOTHINKDB_ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("GOTHINKDB_ALLOW_DEFAULT_ADMIN"); v == "1" || v == "true" {
		cfg.AllowDefaultAdmin = true
	}
}

// Save saves the configuration to a file
func (c *Config) Save(configFile string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}
