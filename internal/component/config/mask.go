// Design: plan/learned/1181-fixit-bcrypt-hash-credential.md -- mask ze:bcrypt leaves on display
// Related: password_hash.go -- CheckBcryptLeaves walk pattern; joinDotPath
// Related: schema.go -- SecretDataPlaceholder, LeafNode.Bcrypt, SensitiveKeys

package config

import (
	"fmt"
	"strings"
)

// BcryptKeys returns the set of leaf names marked ze:bcrypt in the schema. It
// mirrors SensitiveKeys and is used by per-leaf display paths (CLI search, web
// leaf/form rendering, config dump) that mask by leaf name rather than by
// cloning the whole tree.
func BcryptKeys(schema *Schema) map[string]bool {
	if schema == nil {
		return nil
	}
	keys := make(map[string]bool)
	collectBcryptKeys(schema.root, keys)
	return keys
}

func collectBcryptKeys(node Node, keys map[string]bool) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, name := range cp.Children() {
		child := cp.Get(name)
		if leaf, ok := child.(*LeafNode); ok && leaf.Bcrypt {
			keys[name] = true
		}
		collectBcryptKeys(child, keys)
	}
}

// MaskBcrypt returns a deep clone of tree in which every ze:bcrypt leaf holding
// a non-empty value is replaced with SecretDataPlaceholder. The input tree is
// never modified, so the caller's live/committed tree keeps the real hash.
//
// Use it on display and display-only serialization paths (CLI show, annotated
// view, diff, the web terminal); the persistence serializers must run against
// the original unmasked tree so the on-disk config keeps the real hash. Unlike
// ze:sensitive leaves, bcrypt hashes are never $9$-encoded: $9$ is reversible
// and the parser refuses it on bcrypt leaves, so the placeholder is the only
// correct display form.
//
// Nil-safe: returns tree unchanged when tree or schema is nil.
func MaskBcrypt(tree *Tree, schema *Schema) *Tree {
	if tree == nil || schema == nil {
		return tree
	}
	clone := tree.Clone()
	maskBcryptWalk(clone, schema.root)
	return clone
}

// MaskBcryptInPlace masks every non-empty ze:bcrypt leaf value in tree directly,
// without cloning. Use it only when the caller already owns a private (cloned)
// tree that will feed a display serializer; callers that hold a shared/live tree
// must use MaskBcrypt instead so the live tree keeps the real hash. Nil-safe.
func MaskBcryptInPlace(tree *Tree, schema *Schema) {
	if tree == nil || schema == nil {
		return
	}
	maskBcryptWalk(tree, schema.root)
}

func maskBcryptWalk(tree *Tree, node Node) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		if leaf, ok := child.(*LeafNode); ok && leaf.Bcrypt {
			if val, present := tree.Get(childName); present && val != "" {
				tree.Set(childName, SecretDataPlaceholder)
			}
			continue
		}
		switch n := child.(type) {
		case *ContainerNode:
			if sub := tree.GetContainer(childName); sub != nil {
				maskBcryptWalk(sub, n)
			}
		case *ListNode:
			for _, entry := range tree.GetList(childName) {
				maskBcryptWalk(entry, n)
			}
		}
	}
}

// RejectMaskedBcryptLeaves returns an error if any ze:bcrypt leaf in tree holds
// exactly SecretDataPlaceholder — the value the display mask emits. It is a
// fail-closed guard for the commit and upload/validate entry points: a masked
// `show config` (or a downloaded-then-hand-edited artifact that lost the raw
// hash) pasted back into a commit would otherwise clobber the stored password
// with the placeholder. Rejecting (rather than silently resolving the
// placeholder to the committed value) keeps the behavior explicit and the error
// recoverable via the edit-authorized raw download or plaintext-<name>.
//
// Nil-safe: returns nil when tree or schema is nil.
func RejectMaskedBcryptLeaves(tree *Tree, schema *Schema) error {
	if tree == nil || schema == nil {
		return nil
	}
	var masked []string
	rejectMaskedBcryptWalk(tree, schema.root, "", &masked)
	if len(masked) == 0 {
		return nil
	}
	return fmt.Errorf("config leaf %s holds the display placeholder %q: the bcrypt hash was masked for display and cannot be committed. Restore the real hash from the edit-authorized raw config download, or set the password via plaintext-<name> (e.g. plaintext-password) or 'ze passwd'",
		strings.Join(masked, ", "), SecretDataPlaceholder)
}

func rejectMaskedBcryptWalk(tree *Tree, node Node, prefix string, masked *[]string) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		path := joinDotPath(prefix, childName)
		if leaf, ok := child.(*LeafNode); ok && leaf.Bcrypt {
			if val, present := tree.Get(childName); present && val == SecretDataPlaceholder {
				*masked = append(*masked, path)
			}
			continue
		}
		switch n := child.(type) {
		case *ContainerNode:
			if sub := tree.GetContainer(childName); sub != nil {
				rejectMaskedBcryptWalk(sub, n, path, masked)
			}
		case *ListNode:
			for key, entry := range tree.GetList(childName) {
				rejectMaskedBcryptWalk(entry, n, joinDotPath(path, key), masked)
			}
		}
	}
}
