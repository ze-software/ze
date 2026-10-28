// Design: docs/architecture/config/yang-config-design.md — ze:sensitive and the $9$ form
// Related: parser.go — parseLeaf decodes a $9$ leaf through decodeSecretLeaf
// Related: setparser.go — the set and set-meta parsers decode through the same helper
// Related: serialize.go, serialize_set.go — every printer prints through encodeSecrets

package config

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config/secret"
)

// reversibleSecret reports whether leaf holds a ze:sensitive value that the
// $9$ form carries. A ze:bcrypt leaf holds a one-way hash, which cannot share
// the reversible $9$ path, so a $9$-prefixed string there stays verbatim (and
// fails bcrypt format validation later).
func reversibleSecret(leaf *LeafNode) bool {
	return leaf != nil && leaf.Sensitive && !leaf.Bcrypt
}

// decodeSecretLeaf answers the value every config parser stores for the leaf
// name of tree, from the text it read: a $9$ string on a ze:sensitive leaf is
// decoded, so the tree holds the key whichever form the file was written in,
// and the text read is recorded (Tree.SetStoredSecret) so a printer writes the
// same encoding back. Every other value is answered unchanged. A nil leaf is
// a child that is not a LeafNode, which holds no secret.
func decodeSecretLeaf(tree *Tree, name string, leaf *LeafNode, value string) (string, error) {
	if reversibleSecret(leaf) && secret.IsEncoded(value) {
		decoded, err := secret.Decode(value)
		if err != nil {
			return "", fmt.Errorf("invalid $9$ encoding for %s: %w", name, err)
		}
		tree.SetStoredSecret(name, value)
		return decoded, nil
	}
	return value, nil
}

// encodeSecrets answers a copy of tree in which every ze:sensitive leaf holds
// its $9$ form, for the printers (Serialize, SerializeSet,
// SerializeSetWithMeta): no writer prints a key in clear. The Junos model
// decides the spelling. A leaf a parser read as $9$ prints the text it read,
// while that text still decodes to the value held, so a second print changes
// no byte; a value given in clear is encoded once with secret.Encode, and that
// encoding is recorded in tree as its stored text, so every later print of
// tree repeats it. tree keeps the decoded values, which is the meaning the
// printed text reads back as. A tree that holds no value under a ze:sensitive
// leaf name is answered as it is: the editor prints on every change, and a
// whole-tree copy per print made loading a large config many times slower.
func encodeSecrets(tree *Tree, schema *Schema) (*Tree, error) {
	if !holdsValueNamed(tree, schema.sensitiveKeyNames()) {
		return tree, nil
	}
	printed := tree.Clone()
	if err := encodeSecretLeaves(tree, printed, schema.root); err != nil {
		return nil, err
	}
	return printed, nil
}

// holdsValueNamed reports whether tree, or any container or list entry under
// it, holds a value under one of names. It matches by name alone, so it can
// answer true for a leaf of that name that is not sensitive, which costs only
// the full walk; it never answers false for a tree that holds a secret. The
// recursion is bounded by the tree's depth, which the schema bounds.
func holdsValueNamed(tree *Tree, names map[string]bool) bool {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for name := range tree.values {
		if names[name] {
			return true
		}
	}
	for _, sub := range tree.containers {
		if holdsValueNamed(sub, names) {
			return true
		}
	}
	for _, entries := range tree.lists {
		for _, entry := range entries {
			if holdsValueNamed(entry, names) {
				return true
			}
		}
	}
	return false
}

// mustEncodeSecrets is encodeSecrets for a printer whose signature carries no
// error. secret.Encode fails only when crypto/rand.Read does, and since Go
// 1.24 crypto/rand.Read never returns an error: it ends the program instead.
func mustEncodeSecrets(tree *Tree, schema *Schema) *Tree {
	printed, err := encodeSecrets(tree, schema)
	if err != nil {
		panic("impossible: " + err.Error())
	}
	return printed
}

// encodeSecretLeaves rewrites each ze:sensitive leaf of printed, under the
// schema node cp, into its $9$ form, recording a fresh encoding in source,
// the tree printed was cloned from; it walks both side by side. A FlexNode
// keeps the surrounding tree, as the password hashing walk does. The
// recursion is bounded by the schema's depth.
func encodeSecretLeaves(source, printed *Tree, cp ChildProvider) error {
	for _, name := range cp.Children() {
		switch child := cp.Get(name).(type) {
		case *LeafNode:
			if reversibleSecret(child) {
				if err := encodeSecretLeaf(source, printed, name); err != nil {
					return err
				}
			}
		case *ContainerNode:
			sourceSub := source.GetContainer(name)
			printedSub := printed.GetContainer(name)
			if sourceSub != nil && printedSub != nil {
				if err := encodeSecretLeaves(sourceSub, printedSub, child); err != nil {
					return err
				}
			}
		case *ListNode:
			sourceEntries := source.GetList(name)
			for key, entry := range printed.GetList(name) {
				if err := encodeSecretLeaves(sourceEntries[key], entry, child); err != nil {
					return err
				}
			}
		case ChildProvider:
			if err := encodeSecretLeaves(source, printed, child); err != nil {
				return err
			}
		}
	}
	return nil
}

// encodeSecretLeaf sets the leaf name of printed to its $9$ spelling: the
// stored text when it still decodes to the value held, else a fresh encoding,
// which source then keeps as its stored text.
func encodeSecretLeaf(source, printed *Tree, name string) error {
	value, ok := printed.Get(name)
	// refuse-by-default: guards encodeSecretLeaves -- a sensitive leaf the config does not set has no key to print
	if !ok {
		return nil
	}
	// A value already in the $9$ form is printed as it is: every parser
	// decodes it on read, so encoding it again would change its meaning.
	if secret.IsEncoded(value) {
		return nil
	}
	// The mask placeholder is display text, never a key.
	if value == SecretDataPlaceholder {
		return nil
	}
	if stored, ok := printed.StoredSecret(name); ok {
		if decoded, err := secret.Decode(stored); err == nil && decoded == value {
			printed.Set(name, stored)
			return nil
		}
	}
	encoded, err := secret.Encode(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	printed.Set(name, encoded)
	if source != nil {
		source.SetStoredSecret(name, encoded)
	}
	return nil
}

// secretEntrySpelling answers how a set-meta writer spells a session entry's
// value for a ze:sensitive leaf whose printed text is printedValue: an entry
// holding the key printedValue decodes to repeats that text, and any other
// key is encoded, so a contested secret never prints in clear either.
func secretEntrySpelling(printedValue string) func(string) string {
	return func(value string) string {
		if decoded, err := secret.Decode(printedValue); err == nil && decoded == value {
			return quoteIfNeeded(printedValue)
		}
		encoded, err := secret.Encode(value)
		if err != nil {
			panic("impossible: " + err.Error())
		}
		return quoteIfNeeded(encoded)
	}
}
