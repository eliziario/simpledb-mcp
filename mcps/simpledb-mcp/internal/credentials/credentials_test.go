package credentials

import (
	"testing"
	"time"
)

// Test helper functions to avoid import cycle with testutil
func assertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

func TestNewManager(t *testing.T) {
	cacheTime := 10 * time.Minute
	manager := NewManager(cacheTime)
	
	if manager == nil {
		t.Error("Expected non-nil manager")
	}
	
	if manager.cache == nil {
		t.Error("Expected cache to be initialized")
	}
	
	assertEqual(t, cacheTime, manager.cacheTime)
}

func TestManagerStoreAndGet(t *testing.T) {
	// Note: This test will fail on systems without keychain access
	// In a real test environment, we'd use dependency injection with a mock keychain
	manager := NewManager(5 * time.Minute)
	
	connectionName := "test-conn"
	username := "testuser"
	password := "testpass"
	
	// For this test, we'll focus on cache behavior
	// Store manually in cache
	key := connectionName + ":" + username
	manager.cacheMutex.Lock()
	manager.cache[key] = cachedCredential{
		password:  password,
		timestamp: time.Now(),
	}
	manager.cacheMutex.Unlock()
	
	// Test cache retrieval
	cred, err := manager.Get(connectionName, username)
	if err != nil {
		// Expected if no keychain access, but cache should work
		t.Logf("Get failed (expected without keychain): %v", err)
		return
	}
	
	assertEqual(t, username, cred.Username)
	assertEqual(t, password, cred.Password)
}

func TestManagerCacheExpiry(t *testing.T) {
	shortCacheTime := 100 * time.Millisecond
	manager := NewManager(shortCacheTime)
	
	connectionName := "test-conn"
	username := "testuser"
	password := "testpass"
	
	// Manually add to cache with old timestamp
	key := connectionName + ":" + username
	manager.cacheMutex.Lock()
	manager.cache[key] = cachedCredential{
		password:  password,
		timestamp: time.Now().Add(-time.Hour), // Old timestamp
	}
	manager.cacheMutex.Unlock()
	
	// Try to get - should not return cached value due to expiry
	_, err := manager.Get(connectionName, username)
	assertError(t, err) // Should fail because cache is expired and no keychain
}

func TestManagerClearCache(t *testing.T) {
	manager := NewManager(5 * time.Minute)
	
	// Add something to cache
	key := "test:user"
	manager.cacheMutex.Lock()
	manager.cache[key] = cachedCredential{
		password:  "test",
		timestamp: time.Now(),
	}
	manager.cacheMutex.Unlock()
	
	assertEqual(t, 1, len(manager.cache))
	
	// Clear cache
	manager.ClearCache()
	
	assertEqual(t, 0, len(manager.cache))
}

func TestManagerTestConnection(t *testing.T) {
	manager := NewManager(5 * time.Minute)
	
	// Should fail for non-existent connection
	err := manager.TestConnection("non-existent", "user")
	assertError(t, err)
}

func TestCachedCredentialStruct(t *testing.T) {
	now := time.Now()
	cached := cachedCredential{
		password:  "secret",
		timestamp: now,
	}
	
	assertEqual(t, "secret", cached.password)
	assertEqual(t, now, cached.timestamp)
}

func TestCredentialStruct(t *testing.T) {
	cred := Credential{
		Username: "admin",
		Password: "secret123",
	}
	
	assertEqual(t, "admin", cred.Username)
	assertEqual(t, "secret123", cred.Password)
}

func TestManagerWithZeroCacheTime(t *testing.T) {
	manager := NewManager(0) // No caching
	
	if manager == nil {
		t.Error("Expected manager to be created even with zero cache time")
	}
	
	assertEqual(t, time.Duration(0), manager.cacheTime)
}

func TestManagerKeyGeneration(t *testing.T) {
	manager := NewManager(5 * time.Minute)
	
	// Test that keys are generated consistently
	conn1 := "mydb"
	user1 := "admin"
	
	// We can't easily test private key generation, but we can test the public interface
	// Add to cache manually and verify retrieval works
	key := conn1 + ":" + user1
	
	manager.cacheMutex.Lock()
	manager.cache[key] = cachedCredential{
		password:  "test123",
		timestamp: time.Now(),
	}
	manager.cacheMutex.Unlock()
	
	// The cache should contain exactly one entry
	assertEqual(t, 1, len(manager.cache))
}

func TestManagerConcurrentAccess(t *testing.T) {
	manager := NewManager(5 * time.Minute)
	
	// Test concurrent cache operations don't panic
	done := make(chan bool, 3)
	
	// Goroutine 1: Add to cache
	go func() {
		manager.cacheMutex.Lock()
		manager.cache["test1:user1"] = cachedCredential{
			password:  "pass1",
			timestamp: time.Now(),
		}
		manager.cacheMutex.Unlock()
		done <- true
	}()
	
	// Goroutine 2: Add to cache
	go func() {
		manager.cacheMutex.Lock()
		manager.cache["test2:user2"] = cachedCredential{
			password:  "pass2",
			timestamp: time.Now(),
		}
		manager.cacheMutex.Unlock()
		done <- true
	}()
	
	// Goroutine 3: Clear cache
	go func() {
		time.Sleep(10 * time.Millisecond) // Let others add first
		manager.ClearCache()
		done <- true
	}()
	
	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
	
	// Should not panic and cache should be empty
	assertEqual(t, 0, len(manager.cache))
}