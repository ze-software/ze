// Design: docs/architecture/config/yang-config-design.md — YANG value validation
// Overview: completer.go — YANG-driven completion engine

package cli

import (
	"errors"
	"fmt"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoSchemaLoaded = errors.New("no schema loaded")

// isConfigFalse checks if any node in the path has config false set.
// YANG config false is inherited: if a container is config false, all children are too.
func (c *Completer) isConfigFalse(path []string) bool {
	if c.schema == nil {
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
// Returns an error naming the first invalid token; nil when the whole path is valid.
func (c *Completer) validateTokenPath(tokens []string) error {
	if c.schema == nil {
		return errNoSchemaLoaded
	}
	if len(tokens) == 0 {
		return errEmptyPath
	}

	entry := c.findModuleEntry(tokens[0])
	if entry == nil {
		return fmt.Errorf("unknown path: %s", tokens[0])
	}

	for i := 1; i < len(tokens); i++ {
		part := tokens[i]
		children := effectiveChildren(entry)
		if len(children) == 0 {
			return fmt.Errorf("unknown path: %s", textbuf.Join(tokens[:i+1], " "))
		}
		child, ok := children[part]
		if !ok {
			return fmt.Errorf("unknown path: %s", textbuf.Join(tokens[:i+1], " "))
		}
		entry = child

		// If this is a list, the next token MUST be a key value (not a schema child name).
		// If the next token is a known child (and NOT the key leaf), the user forgot the list key.
		// The key leaf (e.g., "name" for peer list) is always a schema child, so we must
		// exclude it from the "missing key" check -- it IS the key value.
		if entry.IsList() {
			if i+1 >= len(tokens) {
				return fmt.Errorf("%s is a list — requires a key (e.g., %s <key> ...)", part, part)
			}
			nextToken := tokens[i+1]
			children := effectiveChildren(entry)
			_, isChild := children[nextToken]
			isKeyLeaf := entry.Key == nextToken
			if isChild && !isKeyLeaf {
				return fmt.Errorf("%s is a list — requires a key (e.g., %s <key> %s ...)", part, part, nextToken)
			}
			// Next token opens the entry address: skip it, and a multi-leaf
			// key's `name value` pairs after it.
			i += config.ListAddressSpan(strings.Fields(entry.Key), tokens[i+1:], func(string) bool { return false })
		}
	}

	return nil
}

// getListKeyEntry returns the YANG entry for a list's first key leaf, the
// one the first value of an entry address is typed for.
// For example, peer list with key "address" returns the address leaf entry,
// and single-hop-session with key "peer vrf interface" returns peer.
func (c *Completer) getListKeyEntry(listPath []string) *gyang.Entry {
	listEntry := c.getEntry(listPath)
	if listEntry == nil || !listEntry.IsList() || listEntry.Key == "" {
		return nil
	}
	if listEntry.Dir == nil {
		return nil
	}
	keyLeaf, ok := listEntry.Dir[strings.Fields(listEntry.Key)[0]]
	if !ok {
		return nil
	}
	return keyLeaf
}

// valueSchema answers the config schema over the completer's own module set
// (config.SchemaOf), built once. The value and key checks judge with it, so a
// completer built over another set (a test's probe module) finds that set's
// nodes rather than the shipped schema's.
func (c *Completer) valueSchema() (*config.Schema, error) {
	c.valuesOnce.Do(func() {
		c.values, c.valuesErr = config.SchemaOf(c.schema)
	})
	return c.values, c.valuesErr
}

// keyRefusal answers the refusal of key as the first key of a new entry of
// the list at listPath, by the check the creator applies (EnsureListEntry,
// config.ValidateListKey), or nil when the list takes it. Completion offers a
// typed key as a new entry only when the creator would accept it.
func (c *Completer) keyRefusal(listPath []string, key string) error {
	schema, err := c.valueSchema()
	if err != nil {
		return err
	}
	list, isList := schema.LookupTokenPath(listPath).(*config.ListNode)
	if !isList {
		return fmt.Errorf("unknown list: %s", textbuf.Join(listPath, " "))
	}
	return config.ValidateListKey(list, listPath, key)
}
