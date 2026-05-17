// Design: docs/architecture/config/yang-config-design.md -- Shared node name validation

package naming

import "fmt"

// ValidateNodeName checks that a configuration node name (unit, peer,
// group, firewall table/chain, sysctl profile, VPP interface, etc.)
// follows the ze-types.yang node-name pattern: ASCII alphanumeric,
// hyphens, underscores, and dots. Must start with a letter, digit, or
// underscore. Length bounded by maxLen.
//
// This is the single source of truth for config node name validation.
// YANG enforces the pattern at parse time; this function is called at
// verify time as defense in depth.
func ValidateNodeName(kind, name string, maxLen int) error {
	n := len(name)
	if n == 0 || n > maxLen {
		return fmt.Errorf("%s: name %q length %d not in [1, %d]", kind, name, n, maxLen)
	}
	for i := range n {
		c := name[i]
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '_' {
			continue
		}
		if (c == '-' || c == '.') && i > 0 {
			continue
		}
		return fmt.Errorf("%s: name %q contains invalid character %q (allowed: alphanumeric, hyphens, underscores, dots)", kind, name, string(c))
	}
	return nil
}
