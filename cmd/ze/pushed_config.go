// Design: docs/architecture/appliance/device-config.md -- pushed config loading priority at boot

//go:build ze_core

package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// pushedConfigPath is a loose-file inbox written by an EXTERNAL actor
// (cloud-init / operator), not by ze, so it stays a raw file. Only ze's own
// output state (the active-config hash) is persisted in the shared zefs store.
const pushedConfigPath = "/perm/ze/config-pushed.conf"

var (
	readPushedConfig   = defaultReadPushedConfig
	removePushedConfig = defaultRemovePushedConfig
)

func defaultReadPushedConfig() ([]byte, error) {
	return os.ReadFile(pushedConfigPath) //nolint:gosec // appliance persistent path
}

func defaultRemovePushedConfig() error {
	return os.Remove(pushedConfigPath)
}

func checkPushedConfig(store storage.Storage, configName string) (applied bool, preChange []byte) {
	data, err := readPushedConfig()
	if err != nil {
		return false, nil
	}

	_, parseErr := config.LoadConfig(string(data), "", nil)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "warning: pushed config invalid, ignoring: %v\n", parseErr)
		if rmErr := removePushedConfig(); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: remove invalid pushed config: %v\n", rmErr)
		}
		return false, nil
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	preChange, _ = store.ReadFile(activeKey)

	if writeErr := store.WriteFile(activeKey, data, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: apply pushed config: %v\n", writeErr)
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "config: using pushed config from %s\n", pushedConfigPath)
	return true, preChange
}

func writeConfigActiveHash(store storage.Storage, configName string) {
	data, err := store.ReadFile(zefs.KeyFileActive.Key(configName))
	if err != nil {
		return
	}
	h := sha256.Sum256(data)
	hash := fmt.Sprintf("sha256:%x", h)
	if writeErr := store.WriteFile(zefs.KeyConfigActiveHash.Pattern, []byte(hash), 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: write config-active-hash: %v\n", writeErr)
	}
}
