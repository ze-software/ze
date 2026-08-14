// Design: docs/architecture/config/syntax.md -- password hashing on commit
// Related: schema.go -- LeafNode.Bcrypt flag
// Related: parser.go -- parseLeaf skips $9$ decode for Bcrypt leaves

package config

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/redact"
)

// IsBcryptHash reports whether s is a syntactically valid bcrypt hash.
// Used by the validator to warn when a ze:bcrypt canonical leaf holds
// a non-hash value (typically literal plaintext from hand-edited config).
// It delegates to redact.IsBcryptHash so the bcrypt-shape regex has a single
// home shared with the log-redaction path.
func IsBcryptHash(s string) bool {
	return redact.IsBcryptHash(s)
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
					path+": not a valid bcrypt hash; use plaintext-"+childName+" or 'ze passwd' to set")
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
// "plaintext-<name>" leaf into the canonical leaf.
//
// The plaintext sibling is removed on every path that returns without an error,
// whether or not it was hashed. An EMPTY sibling hashes nothing and is still
// deleted, because the leaf is ze:ephemeral: it must reach neither a running tree
// nor a serialized file. An ABSENT sibling is the no-op, and it is what makes the
// call idempotent: an already-hashed canonical leaf is left untouched. A sibling
// over bcrypt's 72-byte limit is the exception and survives, because that path
// returns an error and every caller aborts on it.
//
// It returns the dot-path of every canonical leaf it HASHED, and nil when it
// hashed nothing. Order follows the schema walk, except across the entries of
// one list, which come from a Go map and are unordered. The caller that reads
// it is LoadConfig, which writes no file itself, so it must warn that the
// plaintext is still where the operator put it, and it can only know that from
// this walk. Making the walk report what it did is what keeps a second walk
// (and a second spelling of the plaintext- prefix rule) out of the tree.
//
// Invoke this before persisting the tree (editor commit, cmd_set, cmd_import),
// and on the load path before anything reads a credential.
func ApplyPasswordHashing(tree *Tree, schema *Schema) ([]string, error) {
	if tree == nil || schema == nil {
		return nil, nil
	}
	var hashed []string
	if err := walkHashNodes(tree, schema.root, "", &hashed); err != nil {
		return nil, err
	}
	return hashed, nil
}

// walkHashNodes recursively applies the bcrypt transform at the current
// tree/schema level and descends into every child-bearing node. prefix is the
// dot-path of the current level, and every canonical leaf hashed is appended to
// hashed.
func walkHashNodes(tree *Tree, node Node, prefix string, hashed *[]string) error {
	cp, ok := node.(childProvider)
	if !ok {
		return nil
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		if leaf, ok := child.(*LeafNode); ok && leaf.Bcrypt {
			did, err := hashPlaintextSibling(tree, childName)
			if err != nil {
				return err
			}
			if did {
				*hashed = append(*hashed, joinDotPath(prefix, childName))
			}
			continue
		}
		if err := descend(tree, childName, child, joinDotPath(prefix, childName), hashed); err != nil {
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
func descend(tree *Tree, name string, node Node, path string, hashed *[]string) error {
	if c, ok := node.(*ContainerNode); ok {
		if sub := tree.GetContainer(name); sub != nil {
			return walkHashNodes(sub, c, path, hashed)
		}
		return nil
	}
	if l, ok := node.(*ListNode); ok {
		for key, entry := range tree.GetList(name) {
			if err := walkHashNodes(entry, l, joinDotPath(path, key), hashed); err != nil {
				return err
			}
		}
		return nil
	}
	if _, ok := node.(childProvider); ok {
		return walkHashNodes(tree, node, path, hashed)
	}
	return nil
}

// hashPlaintextSibling hashes the plaintext-<canonical> leaf into <canonical>
// and deletes the plaintext sibling. An ABSENT sibling is the no-op. An EMPTY
// one hashes nothing and is deleted anyway, because the leaf is ze:ephemeral.
// The bool reports whether a hash was written, so the empty case returns false.
// Returns an error if the plaintext exceeds bcrypt's 72-byte limit (vendored
// bcrypt rejects with ErrPasswordTooLong) so the commit fails fast instead of
// silently storing a hash that only validates a prefix of the user's input.
// The caller surfaces this as a commit failure with a clear message.
func hashPlaintextSibling(tree *Tree, canonical string) (bool, error) {
	plaintextKey := plaintextPrefix + canonical
	plaintext, ok := tree.Get(plaintextKey)
	if !ok {
		return false, nil
	}
	// An empty plaintext leaf is not a password, so there is nothing to hash and
	// nothing to warn about. It is still an EPHEMERAL leaf, so it is dropped
	// rather than carried into the running tree: the leaf exists to be consumed
	// here, and `show config` must never display it. The canonical leaf keeps
	// whatever it held, which for an unset password is empty, and CheckPassword
	// (internal/component/authz/auth.go) refuses an empty hash.
	if plaintext == "" {
		tree.Delete(plaintextKey)
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return false, fmt.Errorf("%s: password too long (%d bytes; bcrypt limit is 72)",
				canonical, len(plaintext))
		}
		return false, fmt.Errorf("bcrypt %s: %w", canonical, err)
	}
	tree.Set(canonical, string(hash))
	tree.Delete(plaintextKey)
	return true, nil
}
