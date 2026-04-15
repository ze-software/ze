// Design: docs/architecture/config/syntax.md -- password hashing on commit
// Related: schema.go -- LeafNode.Bcrypt flag
// Related: parser.go -- parseLeaf skips $9$ decode for Bcrypt leaves

package config

import (
	"errors"
	"fmt"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

// bcryptFormat matches a canonical bcrypt hash:
// $2[aby]$<cost>$<22-char salt + 31-char hash, base64url> (60 chars total).
var bcryptFormat = regexp.MustCompile(`^\$2[aby]\$\d{2}\$[./A-Za-z0-9]{53}$`)

// IsBcryptHash reports whether s is a syntactically valid bcrypt hash.
// Used by the validator to warn when a ze:bcrypt canonical leaf holds
// a non-hash value (typically literal plaintext from hand-edited config).
func IsBcryptHash(s string) bool {
	return bcryptFormat.MatchString(s)
}

// CheckBcryptLeaves walks the tree and returns a warning string for each
// ze:bcrypt leaf whose value is non-empty and not a valid bcrypt hash.
// Path is dot-separated (e.g., "system.authentication.user.alice.password").
// Empty-value leaves are skipped (a missing password is a separate concern).
func CheckBcryptLeaves(tree *Tree, schema *Schema) []string {
	if tree == nil || schema == nil {
		return nil
	}
	var warnings []string
	checkBcryptLeavesWalk(tree, schema.root, "", &warnings)
	return warnings
}

func checkBcryptLeavesWalk(tree *Tree, node Node, prefix string, warnings *[]string) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		path := joinDotPath(prefix, childName)
		if leaf, ok := child.(*LeafNode); ok && leaf.Bcrypt {
			if val, present := tree.Get(childName); present && val != "" && !IsBcryptHash(val) {
				*warnings = append(*warnings,
					fmt.Sprintf("%s: not a valid bcrypt hash; use plaintext-%s or 'ze passwd' to set",
						path, childName))
			}
			continue
		}
		switch n := child.(type) {
		case *ContainerNode:
			if sub := tree.GetContainer(childName); sub != nil {
				checkBcryptLeavesWalk(sub, n, path, warnings)
			}
		case *ListNode:
			for key, entry := range tree.GetList(childName) {
				checkBcryptLeavesWalk(entry, n, joinDotPath(path, key), warnings)
			}
		}
	}
}

func joinDotPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// plaintextPrefix is the name prefix for the ephemeral write-only companion
// leaf of a ze:bcrypt canonical leaf. The Junos convention uses hyphenated
// names like "plain-text-password"; ze uses "plaintext-<canonical>" so the
// canonical leaf name (e.g., "password") determines the companion.
const plaintextPrefix = "plaintext-"

// ApplyPasswordHashing walks the tree and, for every schema leaf marked
// ze:bcrypt (LeafNode.Bcrypt), bcrypt-hashes the value of the sibling
// "plaintext-<name>" leaf into the canonical leaf. The plaintext sibling
// is removed after hashing. No-op if the plaintext sibling is absent or
// empty. Idempotent: an already-hashed canonical leaf with no plaintext
// sibling is left untouched.
//
// Invoke this before persisting the tree (editor commit, cmd_set, cmd_import).
func ApplyPasswordHashing(tree *Tree, schema *Schema) error {
	if tree == nil || schema == nil {
		return nil
	}
	return walkHashNodes(tree, schema.root)
}

// walkHashNodes recursively applies the bcrypt transform at the current
// tree/schema level and descends into every child-bearing node.
func walkHashNodes(tree *Tree, node Node) error {
	cp, ok := node.(childProvider)
	if !ok {
		return nil
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		if leaf, ok := child.(*LeafNode); ok && leaf.Bcrypt {
			if err := hashPlaintextSibling(tree, childName); err != nil {
				return err
			}
			continue
		}
		if err := descend(tree, childName, child); err != nil {
			return err
		}
	}
	return nil
}

// descend walks into a child-bearing node using the schema. Containers and
// lists project to sub-Trees; other childProviders (e.g., FlexNode) keep the
// surrounding tree because they do not introduce a sub-Tree, so ze:bcrypt
// leaves nested under them are still discoverable from the parent walk.
//
// Assumption: the childProvider graph is acyclic. ze schema today (Container,
// List, Flex) does not produce a node whose Children() includes itself; if
// a future schema feature introduces recursion, the fallback walk would
// loop. Bound by node-graph traversal depth in practice.
func descend(tree *Tree, name string, node Node) error {
	if c, ok := node.(*ContainerNode); ok {
		if sub := tree.GetContainer(name); sub != nil {
			return walkHashNodes(sub, c)
		}
		return nil
	}
	if l, ok := node.(*ListNode); ok {
		for _, entry := range tree.GetList(name) {
			if err := walkHashNodes(entry, l); err != nil {
				return err
			}
		}
		return nil
	}
	if _, ok := node.(childProvider); ok {
		return walkHashNodes(tree, node)
	}
	return nil
}

// hashPlaintextSibling hashes the plaintext-<canonical> leaf into <canonical>
// and deletes the plaintext sibling. No-op if plaintext is absent or empty.
// Returns an error if the plaintext exceeds bcrypt's 72-byte limit (vendored
// bcrypt rejects with ErrPasswordTooLong) so the commit fails fast instead of
// silently storing a hash that only validates a prefix of the user's input.
// The caller surfaces this as a commit failure with a clear message.
func hashPlaintextSibling(tree *Tree, canonical string) error {
	plaintextKey := plaintextPrefix + canonical
	plaintext, ok := tree.Get(plaintextKey)
	if !ok || plaintext == "" {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return fmt.Errorf("%s: password too long (%d bytes; bcrypt limit is 72)",
				canonical, len(plaintext))
		}
		return fmt.Errorf("bcrypt %s: %w", canonical, err)
	}
	tree.Set(canonical, string(hash))
	tree.Delete(plaintextKey)
	return nil
}
