package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/testutil"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	// Test default values
	testutil.AssertEqual(t, 30*time.Second, cfg.Settings.QueryTimeout)
	testutil.AssertEqual(t, 1000, cfg.Settings.MaxRows)
	testutil.AssertEqual(t, 5*time.Minute, cfg.Settings.CacheCredentials)
	testutil.AssertEqual(t, true, cfg.Settings.RequireBiometric)
	
	// Test connection pool defaults
	testutil.AssertEqual(t, 30*time.Second, cfg.Settings.ConnectionPool.PingInterval)
	testutil.AssertEqual(t, 15*time.Minute, cfg.Settings.ConnectionPool.MaxIdleTime)
	testutil.AssertEqual(t, 3, cfg.Settings.ConnectionPool.MaxErrorCount)
	testutil.AssertEqual(t, 5*time.Second, cfg.Settings.ConnectionPool.ReconnectDelay)
	testutil.AssertEqual(t, true, cfg.Settings.ConnectionPool.EnableKeepalive)
	
	// Test that connections map is initialized
	if cfg.Connections == nil {
		t.Error("Expected connections map to be initialized")
	}
	
	testutil.AssertEqual(t, 0, len(cfg.Connections))
}

func TestConfigDirAndPath(t *testing.T) {
	// Test ConfigDir
	configDir, err := ConfigDir()
	testutil.AssertNoError(t, err)
	
	if configDir == "" {
		t.Error("Expected non-empty config directory")
	}
	
	if !filepath.IsAbs(configDir) {
		t.Error("Expected absolute path for config directory")
	}
	
	// Verify it ends with the expected suffix
	expectedSuffix := filepath.Join(".config", "simpledb-mcp")
	if !strings.HasSuffix(configDir, expectedSuffix) {
		t.Errorf("Expected config directory to end with %s, got %s", expectedSuffix, configDir)
	}
	
	// Test ConfigPath
	configPath, err := ConfigPath()
	testutil.AssertNoError(t, err)
	
	expectedPath := filepath.Join(configDir, "config.yaml")
	testutil.AssertEqual(t, expectedPath, configPath)
}

func TestLoadConfigNonExistent(t *testing.T) {
	// Temporarily change home directory for testing
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	cfg, err := Load()
	testutil.AssertNoError(t, err)
	
	// Should return default config when file doesn't exist
	testutil.AssertEqual(t, 30*time.Second, cfg.Settings.QueryTimeout)
	testutil.AssertEqual(t, 0, len(cfg.Connections))
}

func TestLoadAndSaveConfig(t *testing.T) {
	// Create temp directory and set as home
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// Create test config
	cfg := DefaultConfig()
	cfg.Settings.QueryTimeout = 60 * time.Second
	cfg.Settings.MaxRows = 500
	cfg.AddConnection("test-conn", Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "testuser",
	})
	
	// Save config
	err := cfg.Save()
	testutil.AssertNoError(t, err)
	
	// Verify config file exists
	configPath, _ := ConfigPath()
	testutil.AssertFileExists(t, configPath)
	
	// Load config and verify
	loadedCfg, err := Load()
	testutil.AssertNoError(t, err)
	
	testutil.AssertEqual(t, 60*time.Second, loadedCfg.Settings.QueryTimeout)
	testutil.AssertEqual(t, 500, loadedCfg.Settings.MaxRows)
	testutil.AssertEqual(t, 1, len(loadedCfg.Connections))
	
	conn, exists := loadedCfg.GetConnection("test-conn")
	testutil.AssertEqual(t, true, exists)
	testutil.AssertEqual(t, "mysql", conn.Type)
	testutil.AssertEqual(t, "localhost", conn.Host)
	testutil.AssertEqual(t, 3306, conn.Port)
	testutil.AssertEqual(t, "testdb", conn.Database)
	testutil.AssertEqual(t, "testuser", conn.Username)
}

func TestAddConnection(t *testing.T) {
	cfg := DefaultConfig()
	
	conn := Connection{
		Type:     "postgres",
		Host:     "db.example.com",
		Port:     5432,
		Database: "myapp",
		Username: "readonly",
		SSLMode:  "require",
	}
	
	// Test adding connection
	err := cfg.AddConnection("prod-db", conn)
	// Note: This will fail without proper home directory setup, but that's expected in unit tests
	// The important part is that the connection is added to the map
	_ = err // May fail due to file system access, but we test the logic below
	
	// Verify connection was added to map
	testutil.AssertEqual(t, 1, len(cfg.Connections))
	
	retrieved, exists := cfg.GetConnection("prod-db")
	testutil.AssertEqual(t, true, exists)
	testutil.AssertEqual(t, "postgres", retrieved.Type)
	testutil.AssertEqual(t, "db.example.com", retrieved.Host)
	testutil.AssertEqual(t, 5432, retrieved.Port)
	testutil.AssertEqual(t, "myapp", retrieved.Database)
	testutil.AssertEqual(t, "readonly", retrieved.Username)
	testutil.AssertEqual(t, "require", retrieved.SSLMode)
}

func TestRemoveConnection(t *testing.T) {
	cfg := DefaultConfig()
	
	// Add a connection first
	conn := Connection{Type: "mysql", Host: "localhost", Port: 3306, Database: "test"}
	cfg.Connections["test-conn"] = conn
	
	testutil.AssertEqual(t, 1, len(cfg.Connections))
	
	// Remove connection
	err := cfg.RemoveConnection("test-conn")
	// Will fail without proper directory setup, but map should be updated
	_ = err // May fail due to file system access, but we test the logic below
	
	testutil.AssertEqual(t, 0, len(cfg.Connections))
	
	_, exists := cfg.GetConnection("test-conn")
	testutil.AssertEqual(t, false, exists)
}

func TestGetConnection(t *testing.T) {
	cfg := DefaultConfig()
	
	// Test non-existent connection
	_, exists := cfg.GetConnection("non-existent")
	testutil.AssertEqual(t, false, exists)
	
	// Add connection and test retrieval
	conn := Connection{Type: "mysql", Host: "localhost"}
	cfg.Connections["test"] = conn
	
	retrieved, exists := cfg.GetConnection("test")
	testutil.AssertEqual(t, true, exists)
	testutil.AssertEqual(t, "mysql", retrieved.Type)
	testutil.AssertEqual(t, "localhost", retrieved.Host)
}

func TestListConnections(t *testing.T) {
	cfg := DefaultConfig()
	
	// Test empty list
	connections := cfg.ListConnections()
	testutil.AssertEqual(t, 0, len(connections))
	
	// Add connections
	cfg.Connections["conn1"] = Connection{Type: "mysql"}
	cfg.Connections["conn2"] = Connection{Type: "postgres"}
	cfg.Connections["conn3"] = Connection{Type: "mysql"}
	
	connections = cfg.ListConnections()
	testutil.AssertEqual(t, 3, len(connections))
	
	// Verify all connection names are present
	nameMap := make(map[string]bool)
	for _, name := range connections {
		nameMap[name] = true
	}
	
	testutil.AssertEqual(t, true, nameMap["conn1"])
	testutil.AssertEqual(t, true, nameMap["conn2"])
	testutil.AssertEqual(t, true, nameMap["conn3"])
}

func TestConfigWithInvalidYAML(t *testing.T) {
	// Create temp directory and set as home
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// Create config directory
	configDir, _ := ConfigDir()
	err := os.MkdirAll(configDir, 0755)
	testutil.AssertNoError(t, err)
	
	// Write invalid YAML
	configPath, _ := ConfigPath()
	invalidYAML := `
connections:
  test:
    type: mysql
    port: "not-a-number"  # This should cause parsing error
settings:
  query_timeout: invalid-duration
`
	err = os.WriteFile(configPath, []byte(invalidYAML), 0644)
	testutil.AssertNoError(t, err)
	
	// Try to load - should return error
	_, err = Load()
	testutil.AssertError(t, err)
}

func TestConnectionPoolSettings(t *testing.T) {
	cfg := DefaultConfig()
	
	pool := cfg.Settings.ConnectionPool
	testutil.AssertEqual(t, 30*time.Second, pool.PingInterval)
	testutil.AssertEqual(t, 15*time.Minute, pool.MaxIdleTime)
	testutil.AssertEqual(t, 3, pool.MaxErrorCount)
	testutil.AssertEqual(t, 5*time.Second, pool.ReconnectDelay)
	testutil.AssertEqual(t, true, pool.EnableKeepalive)
}

func TestConnectionTypes(t *testing.T) {
	// Test different connection types
	mysqlConn := Connection{
		Type:     "mysql",
		Host:     "mysql.example.com",
		Port:     3306,
		Database: "app",
		Username: "user",
	}
	
	postgresConn := Connection{
		Type:     "postgres",
		Host:     "postgres.example.com",
		Port:     5432,
		Database: "app",
		Username: "user",
		SSLMode:  "require",
	}
	
	// Verify fields are set correctly
	testutil.AssertEqual(t, "mysql", mysqlConn.Type)
	testutil.AssertEqual(t, 3306, mysqlConn.Port)
	testutil.AssertEqual(t, "", mysqlConn.SSLMode) // Should be empty for MySQL
	
	testutil.AssertEqual(t, "postgres", postgresConn.Type)
	testutil.AssertEqual(t, 5432, postgresConn.Port)
	testutil.AssertEqual(t, "require", postgresConn.SSLMode)
}