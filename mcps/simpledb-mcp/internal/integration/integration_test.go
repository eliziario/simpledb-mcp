package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
	"github.com/eliziario/simpledb-mcp/internal/database"
	"github.com/eliziario/simpledb-mcp/internal/testutil"
	"github.com/eliziario/simpledb-mcp/pkg/api"
)

// createTestConfig creates a test configuration to avoid import cycles
func createTestConfig() *config.Config {
	return &config.Config{
		Connections: map[string]config.Connection{
			"test-mysql": {
				Type:     "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
				Username: "testuser",
			},
			"test-postgres": {
				Type:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
				SSLMode:  "disable",
			},
		},
		Settings: config.Settings{
			QueryTimeout:     30 * time.Second,
			MaxRows:          1000,
			CacheCredentials: 5 * time.Minute,
			RequireBiometric: false, // Disable for tests
			ConnectionPool: config.ConnectionPoolSettings{
				PingInterval:    10 * time.Second, // Faster for tests
				MaxIdleTime:     1 * time.Minute,  // Shorter for tests
				MaxErrorCount:   2,                // Lower threshold for tests
				ReconnectDelay:  1 * time.Second,  // Faster reconnect for tests
				EnableKeepalive: true,
			},
		},
	}
}

func TestConfigurationFlow(t *testing.T) {
	// Test complete configuration lifecycle
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// Test loading non-existent config (should return defaults)
	cfg, err := config.Load()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, 0, len(cfg.Connections))
	
	// Test adding connections
	mysqlConn := config.Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "root",
	}
	
	postgresConn := config.Connection{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "postgres",
		SSLMode:  "disable",
	}
	
	cfg.Connections["mysql-test"] = mysqlConn
	cfg.Connections["postgres-test"] = postgresConn
	
	// Test saving and reloading
	err = cfg.Save()
	testutil.AssertNoError(t, err)
	
	reloadedCfg, err := config.Load()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, 2, len(reloadedCfg.Connections))
	
	mysql, exists := reloadedCfg.GetConnection("mysql-test")
	testutil.AssertEqual(t, true, exists)
	testutil.AssertEqual(t, "mysql", mysql.Type)
	testutil.AssertEqual(t, "localhost", mysql.Host)
	
	postgres, exists := reloadedCfg.GetConnection("postgres-test")
	testutil.AssertEqual(t, true, exists)
	testutil.AssertEqual(t, "postgres", postgres.Type)
	testutil.AssertEqual(t, "disable", postgres.SSLMode)
}

func TestCredentialManagerFlow(t *testing.T) {
	manager := credentials.NewManager(5 * time.Minute)
	
	// Test caching behavior
	connectionName := "test-db"
	username := "testuser"
	password := "testpass"
	
	// Clear cache before testing
	manager.ClearCache()
	
	// Add to cache
	manager.Store(connectionName, username, password)
	
	// Test cache retrieval (will likely fail without actual keychain, but tests the flow)
	_, err := manager.Get(connectionName, username)
	if err != nil {
		// Expected without keychain access
		t.Logf("Credential retrieval failed as expected without keychain: %v", err)
	}
	
	// Test cache clearing
	manager.ClearCache()
	
	// Test error handling
	err = manager.TestConnection("non-existent", "user")
	testutil.AssertError(t, err)
}

func TestDatabaseManagerFlow(t *testing.T) {
	cfg := createTestConfig()
	credManager := testutil.NewMockCredentialManager()
	
	// Set up mock credentials
	credManager.SetCredential("test-mysql", "testuser", "testpass")
	credManager.SetCredential("test-postgres", "testuser", "testpass")
	
	manager := database.NewManager(cfg, credManager)
	defer manager.Close()
	
	// Test connection attempts (will fail without real DB but tests the flow)
	_, err := manager.GetConnection("test-mysql")
	testutil.AssertError(t, err) // Expected - no real MySQL
	
	_, err = manager.GetConnection("test-postgres")
	testutil.AssertError(t, err) // Expected - no real PostgreSQL
	
	// Test non-existent connection
	_, err = manager.GetConnection("non-existent")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
	
	// Test connection pool status
	status := manager.GetConnectionStatus("test-mysql")
	testutil.AssertEqual(t, "test-mysql", status.Name)
	// State could be either disconnected or error since we tried to connect to non-existent DB
	if status.State != database.StateDisconnected && status.State != database.StateError {
		t.Errorf("Expected state to be disconnected or error, got %v", status.State)
	}
	
	// Test pool metrics
	metrics := manager.GetPoolMetrics()
	if metrics == nil {
		t.Error("Expected non-nil pool metrics")
	}
}

func TestConnectionPoolIntegration(t *testing.T) {
	cfg := createTestConfig()
	// Use faster settings for testing
	cfg.Settings.ConnectionPool.PingInterval = 100 * time.Millisecond
	cfg.Settings.ConnectionPool.MaxIdleTime = 500 * time.Millisecond
	cfg.Settings.ConnectionPool.MaxErrorCount = 2
	
	credManager := testutil.NewMockCredentialManager()
	credManager.SetCredential("test-mysql", "testuser", "testpass")
	
	manager := database.NewManager(cfg, credManager)
	defer manager.Close()
	
	// Test that pool is created and monitoring starts
	metrics := manager.GetPoolMetrics()
	testutil.AssertEqual(t, int64(0), metrics.TotalConnections)
	testutil.AssertEqual(t, int64(0), metrics.ActiveConnections)
	
	// Attempt connection (will fail but should create pool entry)
	_, err := manager.GetConnection("test-mysql")
	testutil.AssertError(t, err) // Expected - no real database
	
	// Check that pool tracked the attempt
	status := manager.GetConnectionStatus("test-mysql")
	testutil.AssertEqual(t, "test-mysql", status.Name)
	
	// Test cleanup
	err = manager.Close()
	testutil.AssertNoError(t, err)
}

func TestServerCreationAndShutdown(t *testing.T) {
	// Create server (will use default config since we're not setting up temp home)
	server, err := api.NewServer()
	if err != nil {
		// May fail due to configuration issues, but should not panic
		t.Logf("Server creation failed (expected): %v", err)
		return
	}
	
	if server == nil {
		t.Error("Expected non-nil server")
		return
	}
	
	// Test server info
	info := server.GetInfo()
	if info == nil {
		t.Error("Expected non-nil server info")
	}
	
	// Test graceful shutdown
	err = server.Close()
	testutil.AssertNoError(t, err)
}

func TestServerWithMockConfig(t *testing.T) {
	// Set up temporary config for testing
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// Create test configuration
	cfg := createTestConfig()
	err := cfg.Save()
	testutil.AssertNoError(t, err)
	
	// Create server
	server, err := api.NewServer()
	testutil.AssertNoError(t, err)
	defer server.Close()
	
	// Test server info with our test config
	info := server.GetInfo()
	serverInfo, ok := info["server"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	testutil.AssertEqual(t, "simpledb-mcp", serverInfo["name"])
	
	connections, ok := info["connections"].([]map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	testutil.AssertEqual(t, 2, len(connections)) // test-mysql and test-postgres
	
	settings, ok := info["settings"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	testutil.AssertEqual(t, 1000, settings["max_rows"])
}

func TestFullStackWithoutRealDatabase(t *testing.T) {
	// Test the complete stack without requiring real database connections
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// 1. Create and save configuration
	cfg := createTestConfig()
	err := cfg.Save()
	testutil.AssertNoError(t, err)
	
	// 2. Create server
	server, err := api.NewServer()
	testutil.AssertNoError(t, err)
	defer server.Close()
	
	// 3. Test that server loads configuration correctly
	info := server.GetInfo()
	connections, ok := info["connections"].([]map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	
	// Should have test connections from config
	foundMySQL := false
	foundPostgres := false
	for _, conn := range connections {
		name := conn["name"].(string)
		if name == "test-mysql" {
			foundMySQL = true
			testutil.AssertEqual(t, "mysql", conn["type"])
		}
		if name == "test-postgres" {
			foundPostgres = true
			testutil.AssertEqual(t, "postgres", conn["type"])
		}
	}
	testutil.AssertEqual(t, true, foundMySQL)
	testutil.AssertEqual(t, true, foundPostgres)
	
	// 4. Test graceful shutdown
	err = server.Close()
	testutil.AssertNoError(t, err)
}

func TestErrorPropagation(t *testing.T) {
	cfg := createTestConfig()
	credManager := testutil.NewMockCredentialManager()
	
	// Set up credential manager to return errors
	credManager.SetError("test-mysql", "testuser", fmt.Errorf("mock credential error"))
	
	manager := database.NewManager(cfg, credManager)
	defer manager.Close()
	
	// Test that credential errors propagate correctly
	_, err := manager.GetConnection("test-mysql")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "mock credential error")
}

func TestConcurrentOperations(t *testing.T) {
	cfg := createTestConfig()
	credManager := testutil.NewMockCredentialManager()
	credManager.SetCredential("test-mysql", "testuser", "testpass")
	
	manager := database.NewManager(cfg, credManager)
	defer manager.Close()
	
	// Test concurrent access to connection pool
	done := make(chan bool, 3)
	
	// Goroutine 1: Try to get connection
	go func() {
		_, err := manager.GetConnection("test-mysql")
		// Error expected (no real DB) but should not panic
		_ = err
		done <- true
	}()
	
	// Goroutine 2: Get pool metrics
	go func() {
		_ = manager.GetPoolMetrics()
		done <- true
	}()
	
	// Goroutine 3: Get connection status
	go func() {
		_ = manager.GetConnectionStatus("test-mysql")
		done <- true
	}()
	
	// Wait for all operations to complete
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(2 * time.Second):
			t.Error("Concurrent operation timed out")
		}
	}
}

func TestConfigurationPersistence(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tempDir := testutil.TempDir(t)
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// Create and save configuration
	cfg1 := config.DefaultConfig()
	cfg1.Settings.MaxRows = 2000
	cfg1.Settings.QueryTimeout = 60 * time.Second
	cfg1.AddConnection("persistent-test", config.Connection{
		Type:     "mysql",
		Host:     "persistent.example.com",
		Port:     3306,
		Database: "persistent_db",
		Username: "persistent_user",
	})
	
	// Save first time
	err := cfg1.Save()
	testutil.AssertNoError(t, err)
	
	// Load in new instance
	cfg2, err := config.Load()
	testutil.AssertNoError(t, err)
	
	// Verify persistence
	testutil.AssertEqual(t, 2000, cfg2.Settings.MaxRows)
	testutil.AssertEqual(t, 60*time.Second, cfg2.Settings.QueryTimeout)
	testutil.AssertEqual(t, 1, len(cfg2.Connections))
	
	conn, exists := cfg2.GetConnection("persistent-test")
	testutil.AssertEqual(t, true, exists)
	testutil.AssertEqual(t, "persistent.example.com", conn.Host)
	testutil.AssertEqual(t, "persistent_db", conn.Database)
}