package credentials

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	ServiceName = "simpledb-mcp"
)

type Manager struct {
	cache      map[string]cachedCredential
	cacheMutex sync.RWMutex
	cacheTime  time.Duration
}

type cachedCredential struct {
	password  string
	timestamp time.Time
}

type Credential struct {
	Username string
	Password string
}

func NewManager(cacheTime time.Duration) *Manager {
	return &Manager{
		cache:     make(map[string]cachedCredential),
		cacheTime: cacheTime,
	}
}

func (m *Manager) Store(connectionName, username, password string) error {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	
	if err := keyring.Set(ServiceName, key, password); err != nil {
		return fmt.Errorf("failed to store credential in keychain: %w", err)
	}

	// Update cache
	m.cacheMutex.Lock()
	m.cache[key] = cachedCredential{
		password:  password,
		timestamp: time.Now(),
	}
	m.cacheMutex.Unlock()

	return nil
}

func (m *Manager) Get(connectionName, username string) (*Credential, error) {
	key := fmt.Sprintf("%s:%s", connectionName, username)

	// Check cache first
	m.cacheMutex.RLock()
	if cached, exists := m.cache[key]; exists {
		if time.Since(cached.timestamp) < m.cacheTime {
			m.cacheMutex.RUnlock()
			return &Credential{
				Username: username,
				Password: cached.password,
			}, nil
		}
	}
	m.cacheMutex.RUnlock()

	// Get from keychain with biometric prompt if supported
	password, err := m.getWithBiometric(key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}

	// Update cache
	m.cacheMutex.Lock()
	m.cache[key] = cachedCredential{
		password:  password,
		timestamp: time.Now(),
	}
	m.cacheMutex.Unlock()

	return &Credential{
		Username: username,
		Password: password,
	}, nil
}

func (m *Manager) Delete(connectionName, username string) error {
	key := fmt.Sprintf("%s:%s", connectionName, username)
	
	if err := keyring.Delete(ServiceName, key); err != nil {
		return fmt.Errorf("failed to delete credential from keychain: %w", err)
	}

	// Remove from cache
	m.cacheMutex.Lock()
	delete(m.cache, key)
	m.cacheMutex.Unlock()

	return nil
}

func (m *Manager) getWithBiometric(key string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return m.getMacOSWithBiometric(key)
	case "windows":
		return m.getWindowsWithBiometric(key)
	default:
		// Fallback to regular keyring for Linux/other systems
		return keyring.Get(ServiceName, key)
	}
}

func (m *Manager) ClearCache() {
	m.cacheMutex.Lock()
	m.cache = make(map[string]cachedCredential)
	m.cacheMutex.Unlock()
}

func (m *Manager) TestConnection(connectionName, username string) error {
	// This will trigger biometric auth if needed
	_, err := m.Get(connectionName, username)
	return err
}