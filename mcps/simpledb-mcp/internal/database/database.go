package database

import (
	"database/sql"
	"fmt"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
	
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Manager struct {
	pool        *ConnectionPool
	config      *config.Config
	credManager credentials.CredentialManager
}

type TableInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`     // table, view, etc.
	RowCount *int64 `json:"row_count,omitempty"`
}

type ColumnInfo struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"default_value"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Type    string   `json:"type"`
	Unique  bool     `json:"unique"`
}

type ForeignKeyInfo struct {
	Name               string   `json:"name"`
	Columns            []string `json:"columns"`
	ReferencedTable    string   `json:"referenced_table"`
	ReferencedColumns  []string `json:"referenced_columns"`
}

func NewManager(config *config.Config, credManager credentials.CredentialManager) *Manager {
	manager := &Manager{
		config:      config,
		credManager: credManager,
	}
	manager.pool = NewConnectionPool(manager)
	return manager
}

func (m *Manager) GetConnection(connectionName string) (*sql.DB, error) {
	return m.pool.GetConnection(connectionName)
}

func (m *Manager) createRawConnection(connConfig config.Connection, connectionName string) (*sql.DB, error) {
	// Get credentials
	var username, password string
	if connConfig.Username != "" {
		cred, err := m.credManager.Get(connectionName, connConfig.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to get credentials for connection '%s': %w", connectionName, err)
		}
		username = cred.Username
		password = cred.Password
	}

	// Build connection string
	dsn, err := m.buildDSN(connConfig, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to build connection string: %w", err)
	}

	// Open connection
	db, err := sql.Open(connConfig.Type, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	return db, nil
}

func (m *Manager) buildDSN(conn config.Connection, username, password string) (string, error) {
	switch conn.Type {
	case "mysql":
		if username == "" {
			return fmt.Sprintf("tcp(%s:%d)/%s", conn.Host, conn.Port, conn.Database), nil
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, conn.Host, conn.Port, conn.Database), nil
	
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d dbname=%s", conn.Host, conn.Port, conn.Database)
		if username != "" {
			dsn += fmt.Sprintf(" user=%s password=%s", username, password)
		}
		if conn.SSLMode != "" {
			dsn += fmt.Sprintf(" sslmode=%s", conn.SSLMode)
		} else {
			dsn += " sslmode=prefer"
		}
		return dsn, nil
	
	default:
		return "", fmt.Errorf("unsupported database type: %s", conn.Type)
	}
}

func (m *Manager) Close() error {
	return m.pool.Close()
}

func (m *Manager) TestConnection(connectionName string) error {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return err
	}
	return db.Ping()
}

// GetConnectionStatus returns the status of a specific connection
func (m *Manager) GetConnectionStatus(connectionName string) *ConnectionStatus {
	return m.pool.GetConnectionStatus(connectionName)
}

// GetAllConnectionStatus returns status for all connections
func (m *Manager) GetAllConnectionStatus() []*ConnectionStatus {
	return m.pool.GetAllConnectionStatus()
}

// GetPoolMetrics returns overall pool metrics
func (m *Manager) GetPoolMetrics() *PoolMetrics {
	return m.pool.GetPoolMetrics()
}