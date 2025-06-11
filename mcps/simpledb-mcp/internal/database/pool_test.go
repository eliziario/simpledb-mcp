package database

import (
	"testing"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/testutil"
)

func TestNewConnectionPool(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	pool := NewConnectionPool(manager)
	
	if pool == nil {
		t.Error("Expected non-nil connection pool")
	}
	
	testutil.AssertEqual(t, manager, pool.manager)
	testutil.AssertEqual(t, cfg.Settings.ConnectionPool.PingInterval, pool.pingInterval)
	testutil.AssertEqual(t, cfg.Settings.ConnectionPool.MaxIdleTime, pool.maxIdleTime)
	testutil.AssertEqual(t, cfg.Settings.ConnectionPool.MaxErrorCount, pool.maxErrorCount)
	testutil.AssertEqual(t, cfg.Settings.ConnectionPool.ReconnectDelay, pool.reconnectDelay)
	
	if pool.connections == nil {
		t.Error("Expected connections map to be initialized")
	}
	
	testutil.AssertEqual(t, 0, len(pool.connections))
	
	// Clean up
	pool.Close()
}

func TestConnectionPoolDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.ConnectionPool.EnableKeepalive = false
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	
	pool := NewConnectionPool(manager)
	
	// Pool should be created but monitoring disabled
	if pool == nil {
		t.Error("Expected non-nil connection pool even when disabled")
	}
	
	pool.Close()
}

func TestConnectionState(t *testing.T) {
	states := []ConnectionState{
		StateDisconnected,
		StateConnecting,
		StateConnected,
		StateError,
		StateIdle,
	}
	
	expectedStrings := []string{
		"disconnected",
		"connecting", 
		"connected",
		"error",
		"idle",
	}
	
	for i, state := range states {
		testutil.AssertEqual(t, expectedStrings[i], state.String())
	}
	
	// Test unknown state
	var unknownState ConnectionState = 999
	testutil.AssertEqual(t, "unknown", unknownState.String())
}

func TestPooledConnection(t *testing.T) {
	now := time.Now()
	conn := &PooledConnection{
		Name:       "test-conn",
		State:      StateConnected,
		LastUsed:   now,
		LastPing:   now,
		ErrorCount: 0,
		CreatedAt:  now,
	}
	
	testutil.AssertEqual(t, "test-conn", conn.Name)
	testutil.AssertEqual(t, StateConnected, conn.State)
	testutil.AssertEqual(t, 0, conn.ErrorCount)
}

func TestPoolGetConnectionNonExistent(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	_, err := pool.GetConnection("non-existent")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "not found in configuration")
}

func TestGetConnectionStatus(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	// Test non-existent connection
	status := pool.GetConnectionStatus("non-existent")
	testutil.AssertEqual(t, "non-existent", status.Name)
	testutil.AssertEqual(t, StateDisconnected, status.State)
	
	// Add a mock connection
	now := time.Now()
	pool.connections["test-conn"] = &PooledConnection{
		Name:       "test-conn",
		State:      StateConnected,
		LastUsed:   now,
		LastPing:   now,
		ErrorCount: 1,
		CreatedAt:  now.Add(-time.Hour),
	}
	
	status = pool.GetConnectionStatus("test-conn")
	testutil.AssertEqual(t, "test-conn", status.Name)
	testutil.AssertEqual(t, StateConnected, status.State)
	testutil.AssertEqual(t, 1, status.ErrorCount)
}

func TestGetAllConnectionStatus(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	// Test empty pool
	statuses := pool.GetAllConnectionStatus()
	testutil.AssertEqual(t, 0, len(statuses))
	
	// Add mock connections
	now := time.Now()
	pool.connections["conn1"] = &PooledConnection{
		Name:      "conn1",
		State:     StateConnected,
		LastUsed:  now,
		CreatedAt: now,
	}
	pool.connections["conn2"] = &PooledConnection{
		Name:      "conn2", 
		State:     StateError,
		LastUsed:  now,
		CreatedAt: now,
	}
	
	statuses = pool.GetAllConnectionStatus()
	testutil.AssertEqual(t, 2, len(statuses))
	
	// Verify we got both connections
	names := make(map[string]bool)
	for _, status := range statuses {
		names[status.Name] = true
	}
	testutil.AssertEqual(t, true, names["conn1"])
	testutil.AssertEqual(t, true, names["conn2"])
}

func TestGetPoolMetrics(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	// Set some test values
	pool.totalConnections = 5
	pool.successfulPings = 100
	pool.failedPings = 10
	
	// Add mock connections
	pool.connections["conn1"] = &PooledConnection{State: StateConnected}
	pool.connections["conn2"] = &PooledConnection{State: StateError}
	pool.connections["conn3"] = &PooledConnection{State: StateConnected}
	
	metrics := pool.GetPoolMetrics()
	
	testutil.AssertEqual(t, int64(5), metrics.TotalConnections)
	testutil.AssertEqual(t, int64(3), metrics.ActiveConnections) // 3 in pool
	testutil.AssertEqual(t, int64(2), metrics.ConnectedCount)   // 2 connected
	testutil.AssertEqual(t, int64(1), metrics.ErrorCount)      // 1 error
	testutil.AssertEqual(t, int64(100), metrics.SuccessfulPings)
	testutil.AssertEqual(t, int64(10), metrics.FailedPings)
	testutil.AssertEqual(t, cfg.Settings.ConnectionPool.PingInterval, metrics.PingInterval)
	testutil.AssertEqual(t, cfg.Settings.ConnectionPool.MaxIdleTime, metrics.MaxIdleTime)
}

func TestCleanupIdleConnections(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.ConnectionPool.MaxIdleTime = 100 * time.Millisecond // Very short for test
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	oldTime := time.Now().Add(-time.Hour)
	recentTime := time.Now()
	
	// Add connections with different states and usage times
	pool.connections["old-disconnected"] = &PooledConnection{
		Name:     "old-disconnected",
		State:    StateDisconnected,
		LastUsed: oldTime,
	}
	pool.connections["old-connected"] = &PooledConnection{
		Name:     "old-connected",
		State:    StateConnected,
		LastUsed: oldTime,
	}
	pool.connections["recent-disconnected"] = &PooledConnection{
		Name:     "recent-disconnected",
		State:    StateDisconnected,
		LastUsed: recentTime,
	}
	
	testutil.AssertEqual(t, 3, len(pool.connections))
	
	// Run cleanup
	pool.cleanupIdleConnections()
	
	// Should only remove old disconnected connections
	testutil.AssertEqual(t, 2, len(pool.connections))
	
	_, exists := pool.connections["old-disconnected"]
	testutil.AssertEqual(t, false, exists)
	
	_, exists = pool.connections["old-connected"]
	testutil.AssertEqual(t, true, exists) // Should keep connected even if old
	
	_, exists = pool.connections["recent-disconnected"] 
	testutil.AssertEqual(t, true, exists) // Should keep recent even if disconnected
}

func TestConnectionStatus(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-time.Hour)
	lastUsed := now.Add(-10 * time.Minute)
	
	status := &ConnectionStatus{
		Name:         "test",
		State:        StateConnected,
		LastUsed:     lastUsed,
		LastPing:     now,
		ErrorCount:   2,
		CreatedAt:    createdAt,
		IdleTime:     time.Since(lastUsed),
		ConnectedFor: time.Since(createdAt),
	}
	
	testutil.AssertEqual(t, "test", status.Name)
	testutil.AssertEqual(t, StateConnected, status.State)
	testutil.AssertEqual(t, 2, status.ErrorCount)
	
	// Verify time calculations are reasonable
	if status.IdleTime < 9*time.Minute || status.IdleTime > 11*time.Minute {
		t.Errorf("Expected idle time around 10 minutes, got %v", status.IdleTime)
	}
	
	if status.ConnectedFor < 50*time.Minute || status.ConnectedFor > 70*time.Minute {
		t.Errorf("Expected connected time around 1 hour, got %v", status.ConnectedFor)
	}
}

func TestPoolMetrics(t *testing.T) {
	metrics := &PoolMetrics{
		TotalConnections:  10,
		ActiveConnections: 5,
		ConnectedCount:    4,
		ErrorCount:        1,
		SuccessfulPings:   100,
		FailedPings:       5,
		PingInterval:      30 * time.Second,
		MaxIdleTime:       15 * time.Minute,
	}
	
	testutil.AssertEqual(t, int64(10), metrics.TotalConnections)
	testutil.AssertEqual(t, int64(5), metrics.ActiveConnections)
	testutil.AssertEqual(t, int64(4), metrics.ConnectedCount)
	testutil.AssertEqual(t, int64(1), metrics.ErrorCount)
	testutil.AssertEqual(t, int64(100), metrics.SuccessfulPings)
	testutil.AssertEqual(t, int64(5), metrics.FailedPings)
	testutil.AssertEqual(t, 30*time.Second, metrics.PingInterval)
	testutil.AssertEqual(t, 15*time.Minute, metrics.MaxIdleTime)
}

func TestPoolClose(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	
	// Add a mock connection
	pool.connections["test"] = &PooledConnection{
		Name:  "test",
		State: StateConnected,
	}
	
	testutil.AssertEqual(t, 1, len(pool.connections))
	
	// Close pool
	err := pool.Close()
	testutil.AssertNoError(t, err)
	
	// Verify cleanup
	testutil.AssertEqual(t, 0, len(pool.connections))
	
	// Verify context was cancelled
	select {
	case <-pool.ctx.Done():
		// Expected
	default:
		t.Error("Expected context to be cancelled")
	}
}

func TestPoolContextCancellation(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	
	// Verify context starts as not done
	select {
	case <-pool.ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// Expected
	}
	
	// Cancel context
	pool.cancel()
	
	// Verify context is now done
	select {
	case <-pool.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected context to be cancelled")
	}
	
	pool.Close()
}

func TestCheckConnectionWithMockDB(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	// Create a mock connection without real DB (will be nil)
	conn := &PooledConnection{
		Name:       "test",
		DB:         nil, // No real DB
		State:      StateError,
		ErrorCount: 0,
	}
	
	// Should not panic with nil DB
	pool.checkConnection(conn)
	
	// Error count should not change for nil DB
	testutil.AssertEqual(t, 0, conn.ErrorCount)
	testutil.AssertEqual(t, StateError, conn.State)
}

func TestPoolConcurrentAccess(t *testing.T) {
	cfg := testConfig()
	credManager := testutil.NewMockCredentialManager()
	manager := NewManager(cfg, credManager)
	pool := NewConnectionPool(manager)
	defer pool.Close()
	
	done := make(chan bool, 3)
	
	// Goroutine 1: Add connection
	go func() {
		pool.mutex.Lock()
		pool.connections["test1"] = &PooledConnection{Name: "test1"}
		pool.mutex.Unlock()
		done <- true
	}()
	
	// Goroutine 2: Get status
	go func() {
		_ = pool.GetConnectionStatus("test1")
		done <- true
	}()
	
	// Goroutine 3: Get metrics
	go func() {
		_ = pool.GetPoolMetrics()
		done <- true
	}()
	
	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(time.Second):
			t.Error("Goroutine timed out")
		}
	}
}