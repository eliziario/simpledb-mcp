//go:build darwin
// +build darwin

package credentials

import (
	"fmt"

	"github.com/ansxuman/go-touchid"
	"github.com/zalando/go-keyring"
)

func (m *Manager) getMacOSWithBiometric(key string) (string, error) {
	// First try to authenticate with TouchID/FaceID
	authenticated, err := touchid.Auth(touchid.DeviceTypeBiometrics, "SimpleDB MCP needs to access your database credentials")
	if err != nil {
		return "", fmt.Errorf("biometric authentication failed: %w", err)
	}

	if !authenticated {
		return "", fmt.Errorf("biometric authentication was cancelled or failed")
	}

	// If authenticated, get the password from keychain
	password, err := keyring.Get(ServiceName, key)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve password from keychain: %w", err)
	}

	return password, nil
}

func (m *Manager) getWindowsWithBiometric(key string) (string, error) {
	// Not supported on macOS
	return keyring.Get(ServiceName, key)
}
