// Design: docs/architecture/config/yang-config-design.md — YANG value validation
// Overview: completer.go — YANG-driven completion engine

package cli

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// isConfigFalse checks if any node in the path has config false set.
// YANG config false is inherited: if a container is config false, all children are too.
func (c *Completer) isConfigFalse(path []string) bool {
	if c.loader == nil {
		return false
	}
	// Check each prefix of the path for config false
	for i := 1; i <= len(path); i++ {
		entry := c.getEntry(path[:i])
		if entry != nil && entry.Config == gyang.TSFalse {
			return true
		}
	}
	return false
}

// validateTokenPath walks the full token path (including list key values) against the schema.
// Unlike getEntry (which skips list keys silently), this enforces that every list has a key value.
// Returns the leaf entry at the end of the path, or an error if the path is invalid.
func (c *Completer) validateTokenPath(tokens []string) (*gyang.Entry, error) {
	if c.loader == nil {
		return nil, fmt.Errorf("no schema loaded")
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	entry := c.findModuleEntry(tokens[0])
	if entry == nil {
		return nil, fmt.Errorf("unknown path: %s", tokens[0])
	}

	for i := 1; i < len(tokens); i++ {
		part := tokens[i]
		if entry.Dir == nil {
			return nil, fmt.Errorf("unknown path: %s", strings.Join(tokens[:i+1], " "))
		}
		child, ok := entry.Dir[part]
		if !ok {
			return nil, fmt.Errorf("unknown path: %s", strings.Join(tokens[:i+1], " "))
		}
		entry = child

		// If this is a list, the next token MUST be a key value (not a schema child name).
		// If the next token is a known child (and NOT the key leaf), the user forgot the list key.
		// The key leaf (e.g., "name" for peer list) is always a schema child, so we must
		// exclude it from the "missing key" check -- it IS the key value.
		if entry.IsList() {
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("%s is a list — requires a key (e.g., %s <key> ...)", part, part)
			}
			nextToken := tokens[i+1]
			_, isChild := entry.Dir[nextToken]
			isKeyLeaf := entry.Key == nextToken
			if isChild && !isKeyLeaf {
				// Next token is a non-key schema child — key is missing
				return nil, fmt.Errorf("%s is a list — requires a key (e.g., %s <key> %s ...)", part, part, nextToken)
			}
			// Next token is the key value — skip it
			i++
		}
	}

	return entry, nil
}

// getListKeyEntry returns the YANG entry for a list's key leaf.
// For example, peer list with key "address" returns the address leaf entry.
func (c *Completer) getListKeyEntry(listPath []string) *gyang.Entry {
	listEntry := c.getEntry(listPath)
	if listEntry == nil || !listEntry.IsList() || listEntry.Key == "" {
		return nil
	}
	if listEntry.Dir == nil {
		return nil
	}
	keyLeaf, ok := listEntry.Dir[listEntry.Key]
	if !ok {
		return nil
	}
	return keyLeaf
}

// validateLeafValue checks if a value is valid for a given YANG leaf type.
// Returns true if the value passes type validation, false if it's clearly invalid.
// Used to prevent Tab from accepting invalid list keys or set values.
func validateLeafValue(entry *gyang.Entry, value string) bool {
	if entry == nil || entry.Type == nil {
		return true // No type info — accept anything
	}
	return validateYangType(entry.Type, value)
}

// validateYangType checks a value against a resolved YANG type.
// Types not explicitly handled are accepted (completer assists, validator enforces).
func validateYangType(t *gyang.YangType, value string) bool {
	if t == nil {
		return true
	}

	// Recognized types with validation
	switch t.Kind { //nolint:exhaustive // unrecognized types accepted — completer assists, validator enforces
	case gyang.Yunion:
		// Union: valid if any member type accepts it
		for _, member := range t.Type {
			if validateYangType(member, value) {
				return true
			}
		}
		return false

	case gyang.Ystring:
		if len(t.Pattern) > 0 {
			return validateStringPatterns(t, value)
		}

	case gyang.Yuint8:
		return validateUintRange(value, 0, 255)
	case gyang.Yuint16:
		return validateUintRange(value, 0, 65535)
	case gyang.Yuint32:
		return validateUintRange(value, 0, 4294967295)

	case gyang.Ybool:
		return value == "true" || value == "false"

	case gyang.Yenum:
		if t.Enum == nil {
			return true
		}
		return slices.Contains(t.Enum.Names(), value)
	}

	return true // Unrecognized types accepted — completer is best-effort, validator is authoritative
}

// validateStringPatterns checks if a value could match any of the YANG patterns.
// For IP address types specifically, uses net.ParseIP for robust validation.
func validateStringPatterns(t *gyang.YangType, value string) bool {
	// Check if any pattern looks like an IP address pattern.
	// YANG ip-address typedef resolves to union of string-with-pattern types.
	// Rather than implementing full regex (YANG uses XSD patterns, not Go regex),
	// detect IP types by their pattern structure and use net.ParseIP.
	for _, p := range t.Pattern {
		if strings.Contains(p, "25[0-5]") || strings.Contains(p, "[0-9a-fA-F]") {
			// IPv4 or IPv6 pattern — validate with net.ParseIP
			return net.ParseIP(value) != nil
		}
	}
	// Non-IP string patterns: accept (full XSD regex validation is complex)
	return true
}

// validateUintRange validates a string as an unsigned integer within [min, max].
func validateUintRange(value string, minVal, maxVal uint64) bool {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return false
	}
	return n >= minVal && n <= maxVal
}
