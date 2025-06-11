package testutil

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"testing"

	"github.com/eliziario/simpledb-mcp/internal/credentials"
)

// TempDir creates a temporary directory for tests
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "simpledb-mcp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// MockCredentialManager is a mock implementation of credentials.Manager for testing
type MockCredentialManager struct {
	credentials map[string]string
	errors      map[string]error
}

func NewMockCredentialManager() *MockCredentialManager {
	return &MockCredentialManager{
		credentials: make(map[string]string),
		errors:      make(map[string]error),
	}
}

func (m *MockCredentialManager) Store(connectionName, username, password string) error {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	if err, exists := m.errors[key]; exists {
		return err
	}
	m.credentials[key] = password
	return nil
}

func (m *MockCredentialManager) Get(connectionName, username string) (*credentials.Credential, error) {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	if err, exists := m.errors[key]; exists {
		return nil, err
	}
	if password, exists := m.credentials[key]; exists {
		return &credentials.Credential{
			Username: username,
			Password: password,
		}, nil
	}
	return nil, fmt.Errorf("credential not found")
}

func (m *MockCredentialManager) Delete(connectionName, username string) error {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	if err, exists := m.errors[key]; exists {
		return err
	}
	delete(m.credentials, key)
	return nil
}

func (m *MockCredentialManager) ClearCache() {
	// No-op for mock
}

func (m *MockCredentialManager) TestConnection(connectionName, username string) error {
	_, err := m.Get(connectionName, username)
	return err
}

// SetError sets an error to be returned for a specific credential key
func (m *MockCredentialManager) SetError(connectionName, username string, err error) {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	m.errors[key] = err
}

// SetCredential manually sets a credential for testing
func (m *MockCredentialManager) SetCredential(connectionName, username, password string) {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	m.credentials[key] = password
}


// MockDriver is a mock SQL driver for testing
type MockDriver struct {
	connections map[string]*MockDB
	shouldFail  bool
	failError   error
}

type MockDB struct {
	driver     *MockDriver
	dsn        string
	closed     bool
	pingFails  bool
	pingError  error
	queryFails bool
	queryError error
	results    map[string]*MockRows
}

type MockRows struct {
	columns []string
	data    [][]interface{}
	index   int
	closed  bool
}

func NewMockDriver() *MockDriver {
	return &MockDriver{
		connections: make(map[string]*MockDB),
	}
}

func (d *MockDriver) Open(dsn string) (driver.Conn, error) {
	if d.shouldFail {
		return nil, d.failError
	}
	
	db := &MockDB{
		driver:  d,
		dsn:     dsn,
		results: make(map[string]*MockRows),
	}
	d.connections[dsn] = db
	return db, nil
}

func (d *MockDriver) SetShouldFail(fail bool, err error) {
	d.shouldFail = fail
	d.failError = err
}

func (d *MockDriver) GetConnection(dsn string) *MockDB {
	return d.connections[dsn]
}

// MockDB methods
func (db *MockDB) Prepare(query string) (driver.Stmt, error) {
	return &MockStmt{db: db, query: query}, nil
}

func (db *MockDB) Close() error {
	db.closed = true
	return nil
}

func (db *MockDB) Begin() (driver.Tx, error) {
	return &MockTx{}, nil
}

func (db *MockDB) Ping() error {
	if db.pingFails {
		return db.pingError
	}
	return nil
}

func (db *MockDB) SetPingFails(fails bool, err error) {
	db.pingFails = fails
	db.pingError = err
}

func (db *MockDB) SetQueryResult(query string, columns []string, data [][]interface{}) {
	db.results[query] = &MockRows{
		columns: columns,
		data:    data,
		index:   0,
	}
}

func (db *MockDB) SetQueryFails(fails bool, err error) {
	db.queryFails = fails
	db.queryError = err
}

// MockStmt methods
type MockStmt struct {
	db    *MockDB
	query string
}

func (stmt *MockStmt) Close() error {
	return nil
}

func (stmt *MockStmt) NumInput() int {
	return 0
}

func (stmt *MockStmt) Exec(args []driver.Value) (driver.Result, error) {
	if stmt.db.queryFails {
		return nil, stmt.db.queryError
	}
	return &MockResult{}, nil
}

func (stmt *MockStmt) Query(args []driver.Value) (driver.Rows, error) {
	if stmt.db.queryFails {
		return nil, stmt.db.queryError
	}
	
	if rows, exists := stmt.db.results[stmt.query]; exists {
		return &MockRows{
			columns: rows.columns,
			data:    rows.data,
			index:   0,
		}, nil
	}
	
	// Default empty result
	return &MockRows{
		columns: []string{},
		data:    [][]interface{}{},
		index:   0,
	}, nil
}

// MockRows methods
func (r *MockRows) Columns() []string {
	return r.columns
}

func (r *MockRows) Close() error {
	r.closed = true
	return nil
}

func (r *MockRows) Next(dest []driver.Value) error {
	if r.index >= len(r.data) {
		return sql.ErrNoRows
	}
	
	row := r.data[r.index]
	for i, val := range row {
		if i < len(dest) {
			dest[i] = val
		}
	}
	r.index++
	return nil
}

// MockTx methods
type MockTx struct{}

func (tx *MockTx) Commit() error   { return nil }
func (tx *MockTx) Rollback() error { return nil }

// MockResult methods
type MockResult struct{}

func (r *MockResult) LastInsertId() (int64, error) { return 0, nil }
func (r *MockResult) RowsAffected() (int64, error) { return 0, nil }

// AssertFileExists checks if a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist: %s", path)
	}
}

// AssertFileNotExists checks if a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("Expected file to not exist: %s", path)
	}
}

// AssertNoError checks that an error is nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// AssertError checks that an error is not nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertContains checks if a string contains a substring
func AssertContains(t *testing.T, str, substr string) {
	t.Helper()
	if str == "" {
		t.Error("String is empty")
		return
	}
	// Simple contains check
	found := false
	if len(substr) <= len(str) {
		for i := 0; i <= len(str)-len(substr); i++ {
			if str[i:i+len(substr)] == substr {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("Expected string to contain %q, got %q", substr, str)
	}
}