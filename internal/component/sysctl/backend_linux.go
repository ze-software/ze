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

// keyToPath converts a dotted sysctl key to a /proc/sys relative path.
// Per-interface keys (net.ipv4.conf.<iface>.leaf) need special handling
// because VLAN interface names contain dots (e.g., eth0.100) that must
// NOT be converted to slashes. The leaf name (last segment after the
// interface name) never contains dots, so we split on the last dot
// for keys under net.ipv{4,6}.conf.
// If future per-interface sysctl prefixes are added (e.g., net.ipv4.neigh.),
// they must be added to the prefix list here.
func keyToPath(key string) string {
	for _, prefix := range []string{"net.ipv4.conf.", "net.ipv6.conf."} {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := key[len(prefix):] // e.g., "eth0.100.forwarding"
		lastDot := strings.LastIndex(rest, ".")
		if lastDot < 0 {
			break // malformed, fall through to naive conversion
		}
		ifaceName := rest[:lastDot] // "eth0.100"
		if ifaceName == "" {
			break // malformed: empty interface name, fall through
		}
		leaf := rest[lastDot+1:]                                           // "forwarding"
		pathPrefix := strings.ReplaceAll(prefix[:len(prefix)-1], ".", "/") // "net/ipv4/conf"
		return pathPrefix + "/" + ifaceName + "/" + leaf
	}
	return strings.ReplaceAll(key, ".", "/")
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
