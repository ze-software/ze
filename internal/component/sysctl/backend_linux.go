// Design: docs/architecture/core-design.md -- sysctl Linux backend
// Overview: backend.go -- backend interface

//go:build linux

package sysctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// procSysRoot is the base path for sysctl reads/writes. Tests override this
// to a temporary directory so that no real kernel tunables are modified.
var procSysRoot = "/proc/sys"

type linuxBackend struct{}

func newBackend() backend {
	return &linuxBackend{}
}

func (b *linuxBackend) read(key string) (string, error) {
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("sysctl: invalid key %q (contains ..)", key)
	}
	rel := keyToPath(key)
	full := filepath.Join(procSysRoot, rel)
	data, err := os.ReadFile(full) //nolint:gosec // key validated (no ".."), rooted at procSysRoot
	if err != nil {
		return "", fmt.Errorf("sysctl read %s: %w", key, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (b *linuxBackend) write(key, value string) error {
	if strings.Contains(key, "..") {
		return fmt.Errorf("sysctl: invalid key %q (contains ..)", key)
	}
	rel := keyToPath(key)
	full := filepath.Join(procSysRoot, rel)
	if err := os.WriteFile(full, []byte(value), 0o600); err != nil {
		return fmt.Errorf("sysctl write %s=%s: %w", key, value, err)
	}
	return nil
}
