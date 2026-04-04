// Design: plan/spec-iface-0-umbrella.md — Interface name validation
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"
	"strings"
)

// Interface name length limits (Linux kernel IFNAMSIZ = 16, including NUL).
const (
	minIfaceNameLen = 1
	maxIfaceNameLen = 15
)

// validateIfaceName checks that name is a valid Linux interface name.
// Linux kernel forbids '/' and NUL in interface names (IFNAMSIZ).
// We also reject ".." sequences to prevent path traversal in sysctl writes.
func validateIfaceName(name string) error {
	n := len(name)
	if n < minIfaceNameLen || n > maxIfaceNameLen {
		return fmt.Errorf("iface: name %q length %d not in [%d, %d]",
			name, n, minIfaceNameLen, maxIfaceNameLen)
	}
	for i := range n {
		c := name[i]
		if c == '/' || c == 0 || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return fmt.Errorf("iface: name %q contains forbidden character", name)
		}
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("iface: name %q contains path traversal sequence", name)
	}
	return nil
}
