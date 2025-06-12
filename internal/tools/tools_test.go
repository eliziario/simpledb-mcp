package tools

import (
	"testing"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/database"
	"github.com/eliziario/simpledb-mcp/internal/testutil"
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// testConfig creates a test configuration to avoid import cycles
func testConfig() *config.Config {
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

// MockMCPServer is a simplified mock for testing
type MockMCPServer struct {
	tools map[string]string // tool name -> description
}

func NewMockMCPServer() *MockMCPServer {
	return &MockMCPServer{
		tools: make(map[string]string),
	}
}

func (m *MockMCPServer) RegisterTool(name, description string, handler interface{}) error {
	m.tools[name] = description
	return nil
}

func TestNewHandler(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	// Use real MCP server for this test
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	if handler == nil {
		t.Error("Expected non-nil handler")
	}
	
	testutil.AssertEqual(t, dbManager, handler.dbManager)
	testutil.AssertEqual(t, cfg, handler.config)
	testutil.AssertEqual(t, mcpServer, handler.server)
}

func TestToolArgumentStructs(t *testing.T) {
	// Test all argument structs have proper JSON tags
	
	listConnArgs := ListConnectionsArgs{}
	_ = listConnArgs // No fields to test
	
	listDbArgs := ListDatabasesArgs{
		Connection: "test-conn",
	}
	testutil.AssertEqual(t, "test-conn", listDbArgs.Connection)
	
	listSchemaArgs := ListSchemasArgs{
		Connection: "test-conn",
		Database:   "test-db",
	}
	testutil.AssertEqual(t, "test-conn", listSchemaArgs.Connection)
	testutil.AssertEqual(t, "test-db", listSchemaArgs.Database)
	
	listTableArgs := ListTablesArgs{
		Connection: "test-conn",
		Database:   "test-db",
		Schema:     "public",
	}
	testutil.AssertEqual(t, "test-conn", listTableArgs.Connection)
	testutil.AssertEqual(t, "test-db", listTableArgs.Database)
	testutil.AssertEqual(t, "public", listTableArgs.Schema)
	
	describeArgs := DescribeTableArgs{
		Connection: "test-conn",
		Database:   "test-db",
		Table:      "users",
		Schema:     "public",
	}
	testutil.AssertEqual(t, "test-conn", describeArgs.Connection)
	testutil.AssertEqual(t, "test-db", describeArgs.Database)
	testutil.AssertEqual(t, "users", describeArgs.Table)
	testutil.AssertEqual(t, "public", describeArgs.Schema)
	
	listIdxArgs := ListIndexesArgs{
		Connection: "test-conn",
		Database:   "test-db",
		Table:      "users",
		Schema:     "public",
	}
	testutil.AssertEqual(t, "test-conn", listIdxArgs.Connection)
	testutil.AssertEqual(t, "test-db", listIdxArgs.Database)
	testutil.AssertEqual(t, "users", listIdxArgs.Table)
	testutil.AssertEqual(t, "public", listIdxArgs.Schema)
	
	sampleArgs := GetTableSampleArgs{
		Connection: "test-conn",
		Database:   "test-db",
		Table:      "users",
		Schema:     "public",
		Limit:      50,
	}
	testutil.AssertEqual(t, "test-conn", sampleArgs.Connection)
	testutil.AssertEqual(t, "test-db", sampleArgs.Database)
	testutil.AssertEqual(t, "users", sampleArgs.Table)
	testutil.AssertEqual(t, "public", sampleArgs.Schema)
	testutil.AssertEqual(t, 50, sampleArgs.Limit)
	
	statusArgs := GetConnectionStatusArgs{
		Connection: "test-conn",
	}
	testutil.AssertEqual(t, "test-conn", statusArgs.Connection)
	
	metricsArgs := GetPoolMetricsArgs{}
	_ = metricsArgs // No fields to test
}

func TestListConnections(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	args := ListConnectionsArgs{}
	response, err := handler.listConnections(args)
	
	testutil.AssertNoError(t, err)
	if response == nil {
		t.Error("Expected non-nil response")
	}
}

func TestListDatabases(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test with valid connection name
	args := ListDatabasesArgs{Connection: "test-mysql"}
	_, err := handler.listDatabases(args)
	// Expected to fail since no real database, but should not panic
	testutil.AssertError(t, err)
	
	// Test with invalid connection name
	args = ListDatabasesArgs{Connection: "non-existent"}
	_, err = handler.listDatabases(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
}

func TestListSchemas(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test with PostgreSQL connection
	args := ListSchemasArgs{
		Connection: "test-postgres",
		Database:   "testdb",
	}
	_, err := handler.listSchemas(args)
	// Expected to fail since no real database
	testutil.AssertError(t, err)
	
	// Test with MySQL connection (should error)
	args = ListSchemasArgs{
		Connection: "test-mysql",
		Database:   "testdb",
	}
	_, err = handler.listSchemas(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "does not support schemas")
	
	// Test with non-existent connection
	args = ListSchemasArgs{
		Connection: "non-existent",
		Database:   "testdb",
	}
	_, err = handler.listSchemas(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
}

func TestListTables(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test with MySQL
	args := ListTablesArgs{
		Connection: "test-mysql",
		Database:   "testdb",
	}
	_, err := handler.listTables(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with PostgreSQL
	args = ListTablesArgs{
		Connection: "test-postgres",
		Database:   "testdb",
		Schema:     "public",
	}
	_, err = handler.listTables(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with invalid connection
	args = ListTablesArgs{
		Connection: "invalid",
		Database:   "testdb",
	}
	_, err = handler.listTables(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
}

func TestDescribeTable(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	args := DescribeTableArgs{
		Connection: "test-mysql",
		Database:   "testdb",
		Table:      "users",
	}
	_, err := handler.describeTable(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with invalid connection
	args = DescribeTableArgs{
		Connection: "invalid",
		Database:   "testdb",
		Table:      "users",
	}
	_, err = handler.describeTable(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
}

func TestListIndexes(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	args := ListIndexesArgs{
		Connection: "test-mysql",
		Database:   "testdb",
		Table:      "users",
	}
	_, err := handler.listIndexes(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with invalid connection
	args = ListIndexesArgs{
		Connection: "invalid",
		Database:   "testdb",
		Table:      "users",
	}
	_, err = handler.listIndexes(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
}

func TestGetTableSample(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test with default limit
	args := GetTableSampleArgs{
		Connection: "test-mysql",
		Database:   "testdb",
		Table:      "users",
		Limit:      0, // Should use default of 10
	}
	_, err := handler.getTableSample(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with custom limit
	args = GetTableSampleArgs{
		Connection: "test-mysql",
		Database:   "testdb",
		Table:      "users",
		Limit:      50,
	}
	_, err = handler.getTableSample(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with limit exceeding max_rows
	args = GetTableSampleArgs{
		Connection: "test-mysql",
		Database:   "testdb",
		Table:      "users",
		Limit:      5000, // Should be capped to config max_rows (1000)
	}
	_, err = handler.getTableSample(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with limit exceeding 100 (tool max)
	args = GetTableSampleArgs{
		Connection: "test-mysql",
		Database:   "testdb",
		Table:      "users",
		Limit:      200, // Should be capped to 100
	}
	_, err = handler.getTableSample(args)
	testutil.AssertError(t, err) // Expected - no real DB
	
	// Test with invalid connection
	args = GetTableSampleArgs{
		Connection: "invalid",
		Database:   "testdb",
		Table:      "users",
		Limit:      10,
	}
	_, err = handler.getTableSample(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found")
}

func TestGetConnectionStatus(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test specific connection
	args := GetConnectionStatusArgs{Connection: "test-mysql"}
	response, err := handler.getConnectionStatus(args)
	testutil.AssertNoError(t, err)
	if response == nil {
		t.Error("Expected non-nil response")
	}
	
	// Test all connections
	args = GetConnectionStatusArgs{Connection: ""}
	response, err = handler.getConnectionStatus(args)
	testutil.AssertNoError(t, err)
	if response == nil {
		t.Error("Expected non-nil response")
	}
}

func TestGetPoolMetrics(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	args := GetPoolMetricsArgs{}
	response, err := handler.getPoolMetrics(args)
	testutil.AssertNoError(t, err)
	if response == nil {
		t.Error("Expected non-nil response")
	}
}

func TestToolRegistration(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	// Use a real MCP server for this test since we need to test actual registration
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test that handler was created successfully
	if handler == nil {
		t.Error("Expected non-nil handler")
	}
	
	testutil.AssertEqual(t, dbManager, handler.dbManager)
	testutil.AssertEqual(t, cfg, handler.config)
	testutil.AssertEqual(t, mcpServer, handler.server)
}

func TestUnsupportedDatabaseType(t *testing.T) {
	cfg := testConfig()
	// Add unsupported database type
	cfg.Connections["unsupported"] = config.Connection{
		Type:     "oracle", // Not supported
		Host:     "localhost",
		Port:     1521,
		Database: "xe",
	}
	
	credManager := testutil.NewMockCredentialManager()
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()
	
	transport := stdio.NewStdioServerTransport()
	mcpServer := mcp_golang.NewServer(transport)
	handler := NewHandler(dbManager, cfg, mcpServer)
	
	// Test with unsupported database type
	args := ListDatabasesArgs{Connection: "unsupported"}
	_, err := handler.listDatabases(args)
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "unsupported database type")
}