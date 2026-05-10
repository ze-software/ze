// Design: plan/spec-appliance-4-device-config.md — pushed config loading priority at boot

package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const (
	pushedConfigPath = "/perm/ze/config-pushed.conf"
	configActiveHash = "/perm/ze/config-active-hash"
)

var (
	readPushedConfig   = defaultReadPushedConfig
	removePushedConfig = defaultRemovePushedConfig
	writeActiveHash    = defaultWriteActiveHash
)

func defaultReadPushedConfig() ([]byte, error) {
	return os.ReadFile(pushedConfigPath) //nolint:gosec // appliance persistent path
}

func defaultRemovePushedConfig() error {
	return os.Remove(pushedConfigPath)
}

func defaultWriteActiveHash(hash string) error {
	return os.WriteFile(configActiveHash, []byte(hash), 0o644) //nolint:gosec // informational
}

func checkPushedConfig(store storage.Storage, configName string) {
	data, err := readPushedConfig()
	if err != nil {
		return
	}

	_, parseErr := config.LoadConfig(string(data), "", nil)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "warning: pushed config invalid, ignoring: %v\n", parseErr)
		if rmErr := removePushedConfig(); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: remove invalid pushed config: %v\n", rmErr)
		}
		return
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	if writeErr := store.WriteFile(activeKey, data, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: apply pushed config: %v\n", writeErr)
		return
	}
	fmt.Fprintf(os.Stderr, "config: using pushed config from %s\n", pushedConfigPath)
}

func writeConfigActiveHash(store storage.Storage, configName string) {
	data, err := store.ReadFile(zefs.KeyFileActive.Key(configName))
	if err != nil {
		return
	}
	h := sha256.Sum256(data)
	hash := fmt.Sprintf("sha256:%x", h)
	if writeErr := writeActiveHash(hash); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: write config-active-hash: %v\n", writeErr)
	}
}
