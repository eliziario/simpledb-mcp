// +build !darwin,!windows

package credentials

import (
	"github.com/zalando/go-keyring"
)

func (m *Manager) getMacOSWithBiometric(key string) (string, error) {
	// Not supported on this platform
	return keyring.Get(ServiceName, key)
}

func (m *Manager) getWindowsWithBiometric(key string) (string, error) {
	// Not supported on this platform
	return keyring.Get(ServiceName, key)
}