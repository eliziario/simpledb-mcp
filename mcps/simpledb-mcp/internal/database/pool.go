package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/config"
)

// ConnectionState represents the current state of a database connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateError
	StateIdle
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateError:
		return "error"
	case StateIdle:
		return "idle"
	default:
		return "unknown"
	}
}

// PooledConnection represents a database connection with metadata
type PooledConnection struct {
	Name         string
	DB           *sql.DB
	Config       config.Connection
	State        ConnectionState
	LastUsed     time.Time
	LastPing     time.Time
	ErrorCount   int
	CreatedAt    time.Time
	mutex        sync.RWMutex
}

// ConnectionPool manages database connections with keep-alive functionality
type ConnectionPool struct {
	connections map[string]*PooledConnection
	manager     *Manager
	ctx         context.Context
	cancel      context.CancelFunc
	mutex       sync.RWMutex
	
	// Configuration
	pingInterval    time.Duration
	maxIdleTime     time.Duration
	maxErrorCount   int
	reconnectDelay  time.Duration
	
	// Metrics
	totalConnections int64
	successfulPings  int64
	failedPings      int64
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(manager *Manager) *ConnectionPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	poolConfig := manager.config.Settings.ConnectionPool
	
	pool := &ConnectionPool{
		connections:     make(map[string]*PooledConnection),
		manager:         manager,
		ctx:             ctx,
		cancel:          cancel,
		pingInterval:    poolConfig.PingInterval,
		maxIdleTime:     poolConfig.MaxIdleTime,
		maxErrorCount:   poolConfig.MaxErrorCount,
		reconnectDelay:  poolConfig.ReconnectDelay,
	}
	
	// Start background monitoring if enabled
	if poolConfig.EnableKeepalive {
		go pool.backgroundMonitor()
	}
	
	return pool
}

// GetConnection gets or creates a pooled connection
func (p *ConnectionPool) GetConnection(connectionName string) (*sql.DB, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Check if connection exists and is healthy
	if conn, exists := p.connections[connectionName]; exists {
		conn.mutex.Lock()
		conn.LastUsed = time.Now()
		
		// If connection is healthy, return it
		if conn.State == StateConnected && conn.DB != nil {
			conn.mutex.Unlock()
			return conn.DB, nil
		}
		conn.mutex.Unlock()
	}
	
	// Create or recreate connection
	return p.createConnection(connectionName)
}

// createConnection creates a new database connection
func (p *ConnectionPool) createConnection(connectionName string) (*sql.DB, error) {
	// Get connection config
	connConfig, exists := p.manager.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found in configuration", connectionName)
	}
	
	// Create new pooled connection
	pooledConn := &PooledConnection{
		Name:      connectionName,
		Config:    connConfig,
		State:     StateConnecting,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	
	// Store in pool
	p.connections[connectionName] = pooledConn
	
	// Create actual database connection
	db, err := p.manager.createRawConnection(connConfig, connectionName)
	if err != nil {
		pooledConn.mutex.Lock()
		pooledConn.State = StateError
		pooledConn.ErrorCount++
		pooledConn.mutex.Unlock()
		return nil, err
	}
	
	// Configure connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(p.maxIdleTime)
	
	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		pooledConn.mutex.Lock()
		pooledConn.State = StateError
		pooledConn.ErrorCount++
		pooledConn.mutex.Unlock()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	// Update pooled connection
	pooledConn.mutex.Lock()
	pooledConn.DB = db
	pooledConn.State = StateConnected
	pooledConn.LastPing = time.Now()
	pooledConn.ErrorCount = 0
	pooledConn.mutex.Unlock()
	
	p.totalConnections++
	log.Printf("Created new database connection for '%s'", connectionName)
	
	return db, nil
}

// backgroundMonitor runs the background connection monitoring
func (p *ConnectionPool) backgroundMonitor() {
	ticker := time.NewTicker(p.pingInterval)
	defer ticker.Stop()
	
	log.Printf("Starting connection pool monitor (ping interval: %s)", p.pingInterval)
	
	for {
		select {
		case <-p.ctx.Done():
			log.Printf("Connection pool monitor stopping")
			return
		case <-ticker.C:
			p.healthCheck()
		}
	}
}

// healthCheck performs health checks on all connections
func (p *ConnectionPool) healthCheck() {
	p.mutex.RLock()
	connections := make([]*PooledConnection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.mutex.RUnlock()
	
	for _, conn := range connections {
		p.checkConnection(conn)
	}
	
	// Clean up idle connections
	p.cleanupIdleConnections()
}

// checkConnection performs a health check on a single connection
func (p *ConnectionPool) checkConnection(conn *PooledConnection) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	
	if conn.DB == nil || conn.State == StateError {
		return
	}
	
	// Ping the database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := conn.DB.PingContext(ctx); err != nil {
		conn.State = StateError
		conn.ErrorCount++
		p.failedPings++
		
		log.Printf("Connection '%s' ping failed (errors: %d): %v", 
			conn.Name, conn.ErrorCount, err)
		
		// If too many errors, close and mark for recreation
		if conn.ErrorCount >= p.maxErrorCount {
			log.Printf("Connection '%s' exceeded max errors, closing", conn.Name)
			conn.DB.Close()
			conn.DB = nil
			conn.State = StateDisconnected
		}
	} else {
		// Successful ping
		conn.LastPing = time.Now()
		if conn.State == StateError {
			conn.State = StateConnected
			log.Printf("Connection '%s' recovered", conn.Name)
		}
		conn.ErrorCount = 0
		p.successfulPings++
	}
}

// cleanupIdleConnections removes connections that have been idle too long
func (p *ConnectionPool) cleanupIdleConnections() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	now := time.Now()
	toRemove := make([]string, 0)
	
	for name, conn := range p.connections {
		conn.mutex.RLock()
		idleTime := now.Sub(conn.LastUsed)
		shouldRemove := idleTime > p.maxIdleTime && conn.State != StateConnected
		conn.mutex.RUnlock()
		
		if shouldRemove {
			toRemove = append(toRemove, name)
		}
	}
	
	for _, name := range toRemove {
		conn := p.connections[name]
		conn.mutex.Lock()
		if conn.DB != nil {
			conn.DB.Close()
		}
		conn.mutex.Unlock()
		delete(p.connections, name)
		log.Printf("Removed idle connection '%s'", name)
	}
}

// GetConnectionStatus returns the status of a specific connection
func (p *ConnectionPool) GetConnectionStatus(connectionName string) *ConnectionStatus {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	conn, exists := p.connections[connectionName]
	if !exists {
		return &ConnectionStatus{
			Name:  connectionName,
			State: StateDisconnected,
		}
	}
	
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	
	return &ConnectionStatus{
		Name:        conn.Name,
		State:       conn.State,
		LastUsed:    conn.LastUsed,
		LastPing:    conn.LastPing,
		ErrorCount:  conn.ErrorCount,
		CreatedAt:   conn.CreatedAt,
		IdleTime:    time.Since(conn.LastUsed),
		ConnectedFor: time.Since(conn.CreatedAt),
	}
}

// GetAllConnectionStatus returns status for all connections
func (p *ConnectionPool) GetAllConnectionStatus() []*ConnectionStatus {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	statuses := make([]*ConnectionStatus, 0, len(p.connections))
	for _, conn := range p.connections {
		conn.mutex.RLock()
		statuses = append(statuses, &ConnectionStatus{
			Name:         conn.Name,
			State:        conn.State,
			LastUsed:     conn.LastUsed,
			LastPing:     conn.LastPing,
			ErrorCount:   conn.ErrorCount,
			CreatedAt:    conn.CreatedAt,
			IdleTime:     time.Since(conn.LastUsed),
			ConnectedFor: time.Since(conn.CreatedAt),
		})
		conn.mutex.RUnlock()
	}
	
	return statuses
}

// GetPoolMetrics returns overall pool metrics
func (p *ConnectionPool) GetPoolMetrics() *PoolMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	connected := 0
	errors := 0
	
	for _, conn := range p.connections {
		conn.mutex.RLock()
		if conn.State == StateConnected {
			connected++
		} else if conn.State == StateError {
			errors++
		}
		conn.mutex.RUnlock()
	}
	
	return &PoolMetrics{
		TotalConnections:   p.totalConnections,
		ActiveConnections:  int64(len(p.connections)),
		ConnectedCount:     int64(connected),
		ErrorCount:         int64(errors),
		SuccessfulPings:    p.successfulPings,
		FailedPings:        p.failedPings,
		PingInterval:       p.pingInterval,
		MaxIdleTime:        p.maxIdleTime,
	}
}

// Close gracefully shuts down the connection pool
func (p *ConnectionPool) Close() error {
	log.Printf("Shutting down connection pool...")
	
	// Stop background monitor
	p.cancel()
	
	// Close all connections
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for name, conn := range p.connections {
		conn.mutex.Lock()
		if conn.DB != nil {
			if err := conn.DB.Close(); err != nil {
				log.Printf("Error closing connection '%s': %v", name, err)
			} else {
				log.Printf("Closed connection '%s'", name)
			}
		}
		conn.mutex.Unlock()
	}
	
	p.connections = make(map[string]*PooledConnection)
	log.Printf("Connection pool shutdown complete")
	
	return nil
}

// ConnectionStatus represents the status of a database connection
type ConnectionStatus struct {
	Name         string            `json:"name"`
	State        ConnectionState   `json:"state"`
	LastUsed     time.Time         `json:"last_used"`
	LastPing     time.Time         `json:"last_ping"`
	ErrorCount   int               `json:"error_count"`
	CreatedAt    time.Time         `json:"created_at"`
	IdleTime     time.Duration     `json:"idle_time"`
	ConnectedFor time.Duration     `json:"connected_for"`
}

// PoolMetrics represents overall connection pool metrics
type PoolMetrics struct {
	TotalConnections   int64         `json:"total_connections"`
	ActiveConnections  int64         `json:"active_connections"`
	ConnectedCount     int64         `json:"connected_count"`
	ErrorCount         int64         `json:"error_count"`
	SuccessfulPings    int64         `json:"successful_pings"`
	FailedPings        int64         `json:"failed_pings"`
	PingInterval       time.Duration `json:"ping_interval"`
	MaxIdleTime        time.Duration `json:"max_idle_time"`
}