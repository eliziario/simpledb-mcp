// +build windows

package credentials

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

func (m *Manager) getWindowsWithBiometric(key string) (string, error) {
	// TODO: Implement Windows Hello authentication
	// For now, fall back to regular keyring access
	// Windows Credential Manager will prompt for authentication if needed
	
	password, err := keyring.Get(ServiceName, key)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve password from Windows Credential Manager: %w", err)
	}

	return password, nil
}

func (m *Manager) getMacOSWithBiometric(key string) (string, error) {
	// Not supported on Windows
	return keyring.Get(ServiceName, key)
}

// Note: Windows Hello integration would require:
// - Using Windows Hello API through CGO or syscalls
// - Or using PowerShell commands to trigger Windows Hello
// - Or using Windows Runtime APIs (WinRT)
// This is a more complex implementation that would need platform-specific code