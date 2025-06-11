package database

import (
	"testing"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/testutil"
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

func TestNewManager(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	
	manager := NewManager(cfg, credManager)
	
	if manager == nil {
		t.Error("Expected non-nil manager")
	}
	
	if manager.config != cfg {
		t.Error("Expected config to be set")
	}
	
	if manager.credManager != credManager {
		t.Error("Expected credential manager to be set")
	}
	
	if manager.pool == nil {
		t.Error("Expected connection pool to be initialized")
	}
}

func TestBuildDSN(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	tests := []struct {
		name     string
		conn     config.Connection
		username string
		password string
		expected string
	}{
		{
			name: "MySQL with credentials",
			conn: config.Connection{
				Type:     "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
			},
			username: "user",
			password: "pass",
			expected: "user:pass@tcp(localhost:3306)/testdb",
		},
		{
			name: "MySQL without credentials",
			conn: config.Connection{
				Type:     "mysql",
				Host:     "db.example.com",
				Port:     3306,
				Database: "myapp",
			},
			username: "",
			password: "",
			expected: "tcp(db.example.com:3306)/myapp",
		},
		{
			name: "Postgres with credentials",
			conn: config.Connection{
				Type:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLMode:  "disable",
			},
			username: "user",
			password: "pass",
			expected: "host=localhost port=5432 dbname=testdb user=user password=pass sslmode=disable",
		},
		{
			name: "Postgres without credentials",
			conn: config.Connection{
				Type:     "postgres",
				Host:     "pg.example.com",
				Port:     5432,
				Database: "myapp",
			},
			username: "",
			password: "",
			expected: "host=pg.example.com port=5432 dbname=myapp sslmode=prefer",
		},
		{
			name: "Postgres with SSL mode",
			conn: config.Connection{
				Type:     "postgres",
				Host:     "secure-db.com",
				Port:     5432,
				Database: "prod",
				SSLMode:  "require",
			},
			username: "admin",
			password: "secret",
			expected: "host=secure-db.com port=5432 dbname=prod user=admin password=secret sslmode=require",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, err := manager.buildDSN(tt.conn, tt.username, tt.password)
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, tt.expected, dsn)
		})
	}
}

func TestBuildDSNUnsupportedType(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	conn := config.Connection{
		Type:     "unsupported",
		Host:     "localhost",
		Port:     1234,
		Database: "test",
	}
	
	_, err := manager.buildDSN(conn, "", "")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "unsupported database type")
}

func TestCreateRawConnection(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	// Set up mock credentials
	credManager.SetCredential("test-conn", "testuser", "testpass")
	
	conn := config.Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "testuser",
	}
	
	// This will fail because we don't have a real MySQL connection,
	// but we can test that the DSN building and credential retrieval works
	_, err := manager.createRawConnection(conn, "test-conn")
	
	// We expect this to fail with a connection error, not a credential error
	if err != nil {
		// Should contain connection-related error, not credential error
		errorStr := err.Error()
		if errorStr == "failed to get credentials for connection 'test-conn': credential not found" {
			t.Error("Unexpected credential error - mock should have provided credentials")
		}
		// Other errors are expected (no real database to connect to)
	}
}

func TestCreateRawConnectionMissingCredentials(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	conn := config.Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "testuser", // Username specified but no credentials in mock
	}
	
	_, err := manager.createRawConnection(conn, "missing-conn")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "failed to get credentials")
}

func TestCreateRawConnectionNoUsername(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	conn := config.Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		// No username - should not try to get credentials
	}
	
	_, err := manager.createRawConnection(conn, "no-auth-conn")
	
	// Should fail with connection error, not credential error
	if err != nil && err.Error() == "failed to get credentials for connection 'no-auth-conn': credential not found" {
		t.Error("Should not try to get credentials when no username specified")
	}
}

func TestGetConnection(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	// Test getting connection for configured connection
	_, err := manager.GetConnection("test-mysql")
	
	// Expected to fail since we don't have real database, but should not panic
	// and should go through the proper flow
	if err == nil {
		t.Error("Expected error when connecting to non-existent database")
	}
}

func TestGetConnectionNonExistent(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	_, err := manager.GetConnection("non-existent-connection")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found in configuration")
}

func TestClose(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	// Should not panic
	err := manager.Close()
	testutil.AssertNoError(t, err)
}

func TestTestConnection(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	// Test with non-existent connection
	err := manager.TestConnection("non-existent")
	testutil.AssertError(t, err)
	
	// Test with configured connection (will fail due to no real DB)
	err = manager.TestConnection("test-mysql")
	testutil.AssertError(t, err) // Expected - no real database
}

func TestTableInfo(t *testing.T) {
	info := TableInfo{
		Name:     "users",
		Type:     "table",
		RowCount: func() *int64 { count := int64(1000); return &count }(),
	}
	
	testutil.AssertEqual(t, "users", info.Name)
	testutil.AssertEqual(t, "table", info.Type)
	testutil.AssertEqual(t, int64(1000), *info.RowCount)
}

func TestColumnInfo(t *testing.T) {
	defaultVal := "NULL"
	col := ColumnInfo{
		Name:         "id",
		Type:         "int",
		Nullable:     false,
		DefaultValue: &defaultVal,
		IsPrimaryKey: true,
	}
	
	testutil.AssertEqual(t, "id", col.Name)
	testutil.AssertEqual(t, "int", col.Type)
	testutil.AssertEqual(t, false, col.Nullable)
	testutil.AssertEqual(t, "NULL", *col.DefaultValue)
	testutil.AssertEqual(t, true, col.IsPrimaryKey)
}

func TestIndexInfo(t *testing.T) {
	idx := IndexInfo{
		Name:    "idx_email",
		Columns: []string{"email"},
		Type:    "btree",
		Unique:  true,
	}
	
	testutil.AssertEqual(t, "idx_email", idx.Name)
	testutil.AssertEqual(t, 1, len(idx.Columns))
	testutil.AssertEqual(t, "email", idx.Columns[0])
	testutil.AssertEqual(t, "btree", idx.Type)
	testutil.AssertEqual(t, true, idx.Unique)
}

func TestForeignKeyInfo(t *testing.T) {
	fk := ForeignKeyInfo{
		Name:               "fk_user_id",
		Columns:            []string{"user_id"},
		ReferencedTable:    "users",
		ReferencedColumns:  []string{"id"},
	}
	
	testutil.AssertEqual(t, "fk_user_id", fk.Name)
	testutil.AssertEqual(t, 1, len(fk.Columns))
	testutil.AssertEqual(t, "user_id", fk.Columns[0])
	testutil.AssertEqual(t, "users", fk.ReferencedTable)
	testutil.AssertEqual(t, 1, len(fk.ReferencedColumns))
	testutil.AssertEqual(t, "id", fk.ReferencedColumns[0])
}