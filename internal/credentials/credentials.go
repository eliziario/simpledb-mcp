package credentials

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
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

type SalesforceCredential struct {
	Username      string
	Password      string
	SecurityToken string
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

func (m *Manager) StoreSalesforce(connectionName, username, password, securityToken string) error {
	// Store Salesforce credentials as JSON in keychain
	sfCred := SalesforceCredential{
		Username:      username,
		Password:      password,
		SecurityToken: securityToken,
	}
	
	credJSON, err := json.Marshal(sfCred)
	if err != nil {
		return fmt.Errorf("failed to marshal Salesforce credentials: %w", err)
	}
	
	key := fmt.Sprintf("%s:salesforce", connectionName)
	if err := keyring.Set(ServiceName, key, string(credJSON)); err != nil {
		return fmt.Errorf("failed to store Salesforce credential in keychain: %w", err)
	}

	return nil
}

func (m *Manager) GetSalesforce(connectionName string) (*SalesforceCredential, error) {
	key := fmt.Sprintf("%s:salesforce", connectionName)
	
	// Get from keychain with biometric prompt if supported
	credJSON, err := m.getWithBiometric(key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Salesforce credential: %w", err)
	}

	// Decode password if it's base64-encoded by go-keyring
	decodedJSON, err := m.decodePassword(credJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Salesforce credential: %w", err)
	}

	// Parse JSON credential
	var sfCred SalesforceCredential
	if err := json.Unmarshal([]byte(decodedJSON), &sfCred); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Salesforce credential: %w", err)
	}

	return &sfCred, nil
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

	// Decode password if it's base64-encoded by go-keyring
	decodedPassword, err := m.decodePassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode password: %w", err)
	}

	// Update cache
	m.cacheMutex.Lock()
	m.cache[key] = cachedCredential{
		password:  decodedPassword,
		timestamp: time.Now(),
	}
	m.cacheMutex.Unlock()

	return &Credential{
		Username: username,
		Password: decodedPassword,
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

// decodePassword handles base64-encoded passwords from go-keyring
func (m *Manager) decodePassword(password string) (string, error) {
	// Check if password has the go-keyring base64 prefix
	if strings.HasPrefix(password, "go-keyring-base64:") {
		encoded := strings.TrimPrefix(password, "go-keyring-base64:")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 password: %w", err)
		}
		password = string(decoded)
	}
	// Remove surrounding quotes and brackets if present
	if strings.HasPrefix(password, "[") && strings.HasSuffix(password, "]") {
		password = strings.TrimPrefix(password, "[")
		password = strings.TrimSuffix(password, "]")
	}

	// Return as-is if not base64-encoded
	return password, nil
}
