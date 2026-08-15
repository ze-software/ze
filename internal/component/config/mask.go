// Design: docs/architecture/ssh/fixit-bcrypt-hash-credential.md -- mask secret leaves on display
// Related: password_hash.go -- CheckBcryptLeaves walk pattern; joinDotPath
// Related: schema.go -- SecretDataPlaceholder, LeafNode.Bcrypt, SensitiveKeys

package config

import (
	"fmt"
	"sort"
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

// LeafHoldsSecret reports whether the schema marks this leaf as holding a
// secret. It is the ONE answer to that question. Every display mask and every
// write guard reads it, so the two halves of the placeholder pair cannot
// disagree: a path that renders SecretDataPlaceholder is always paired with a
// path that refuses to store it.
//
// YANG marks a leaf ze:sensitive (a password or a key, $9$-encoded on disk and
// decoded into the tree by the parser) or ze:bcrypt (a one-way hash). Both hold
// a secret. A nil node holds none, because nothing was resolved.
//
// ze:ephemeral is NOT read here. That extension answers "is this value
// persisted", not "is this value a secret". A leaf that holds a cleartext
// secret says so with ze:sensitive, plaintext-password included.
func LeafHoldsSecret(leaf *LeafNode) bool {
	return leaf != nil && (leaf.Sensitive || leaf.Bcrypt)
}

// leafIsBcrypt reports whether the leaf holds a one-way hash. It is the narrow
// predicate MaskBcrypt keeps, and the comment on that function says why.
func leafIsBcrypt(leaf *LeafNode) bool {
	return leaf != nil && leaf.Bcrypt
}

// MaskBcrypt returns a deep clone of tree in which every ze:bcrypt leaf holding
// a non-empty value is replaced with SecretDataPlaceholder. The input tree is
// never modified, so the caller's live/committed tree keeps the real hash.
//
// It masks the bcrypt half alone, and MaskSecrets is what a display path
// usually wants. A bcrypt hash has no other display form: $9$ is reversible
// and the parser refuses it on a bcrypt leaf. A ze:sensitive value does have
// one, and `ze config dump` writes it as $9$ so the dump reloads. That dump
// calls this function first, so widening it here would replace an encoded
// secret with the placeholder and break the round trip.
//
// The persistence serializers must run against the original unmasked tree, so
// the on-disk config keeps the real hash.
//
// Nil-safe: returns tree unchanged when tree or schema is nil.
func MaskBcrypt(tree *Tree, schema *Schema) *Tree {
	return maskClone(tree, schema, leafIsBcrypt)
}

// MaskBcryptInPlace masks every non-empty ze:bcrypt leaf value in tree directly,
// without cloning. Use it only when the caller already owns a private (cloned)
// tree that will feed a display serializer; callers that hold a shared/live tree
// must use MaskBcrypt instead so the live tree keeps the real hash. Nil-safe.
func MaskBcryptInPlace(tree *Tree, schema *Schema) {
	if tree == nil || schema == nil {
		return
	}
	maskWalk(tree, schema.root, leafIsBcrypt)
}

// MaskSecrets returns a deep clone of tree in which every secret leaf holding a
// non-empty value reads as SecretDataPlaceholder. The input tree is never
// modified, so the caller's working tree keeps the real value.
//
// Use it on any path that renders a whole tree or serializes one for display.
// An empty value stays empty, so an unset secret renders no placeholder and the
// field reads as unconfigured.
//
// Nil-safe: returns tree unchanged when tree or schema is nil.
func MaskSecrets(tree *Tree, schema *Schema) *Tree {
	return maskClone(tree, schema, LeafHoldsSecret)
}

func maskClone(tree *Tree, schema *Schema, holdsSecret func(*LeafNode) bool) *Tree {
	if tree == nil || schema == nil {
		return tree
	}
	clone := tree.Clone()
	maskWalk(clone, schema.root, holdsSecret)
	return clone
}

// maskWalk replaces the value of every leaf holdsSecret accepts. It descends
// each node kind that can carry an addressable leaf value: a container, a flex
// node, a list and an inline list. A kind missing from that set leaves the
// secrets under it in the clear, which is what the ze:bcrypt walk did to a flex
// node before this function was one walk.
func maskWalk(tree *Tree, node Node, holdsSecret func(*LeafNode) bool) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		if leaf, ok := child.(*LeafNode); ok {
			if !holdsSecret(leaf) {
				continue
			}
			if val, present := tree.Get(childName); present && val != "" {
				tree.Set(childName, SecretDataPlaceholder)
			}
			continue
		}
		switch n := child.(type) {
		case *ContainerNode:
			if sub := tree.GetContainer(childName); sub != nil {
				maskWalk(sub, n, holdsSecret)
			}
		case *FlexNode:
			if sub := tree.GetContainer(childName); sub != nil {
				maskWalk(sub, n, holdsSecret)
			}
		case *ListNode:
			for _, entry := range tree.GetList(childName) {
				maskWalk(entry, n, holdsSecret)
			}
		case *InlineListNode:
			for _, entry := range tree.GetList(childName) {
				maskWalk(entry, n, holdsSecret)
			}
		}
	}
}

// ChangedSecretPaths reports the dotted path of every secret leaf whose value
// differs between base and work, when both sides hold a value. A display path
// uses it to say that a secret changed. It publishes neither side.
//
// A mask gives both sides the same SecretDataPlaceholder, so a diff of two
// masked texts reads as no change at all. That is the hole this closes: the
// operator must learn that the leaf moved, and neither value.
//
// A leaf that holds a value on one side alone is NOT reported. The masked text
// already carries that line as an addition or a removal.
//
// The answer is sorted, because a list projects to a map.
//
// Nil-safe: returns nil when a tree or the schema is nil.
func ChangedSecretPaths(base, work *Tree, schema *Schema) []string {
	if schema == nil {
		return nil
	}
	return changedSecretPaths(base, work, schema.root)
}

// ChangedSecretPathsSubtree is ChangedSecretPaths under one schema node, for a
// caller that already walked to a subtree. The paths it returns are relative to
// node. Serialize and SerializeSubtree are the same pair, for the same reason:
// the schema root is not a Node, so the two cases need two doors.
func ChangedSecretPathsSubtree(base, work *Tree, node Node) []string {
	return changedSecretPaths(base, work, node)
}

func changedSecretPaths(base, work *Tree, node Node) []string {
	if base == nil || work == nil || node == nil {
		return nil
	}
	var changed []string
	changedSecretWalk(base, work, node, "", &changed)
	sort.Strings(changed)
	return changed
}

// changedSecretWalk descends the two trees together and keeps the paths where
// they disagree. It walks the same node kinds as maskWalk, so a secret the mask
// hides cannot fall outside the report that names it.
func changedSecretWalk(base, work *Tree, node Node, prefix string, changed *[]string) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		path := joinDotPath(prefix, childName)
		if leaf, ok := child.(*LeafNode); ok {
			if !LeafHoldsSecret(leaf) {
				continue
			}
			baseValue, baseSet := base.Get(childName)
			workValue, workSet := work.Get(childName)
			if baseSet && workSet && baseValue != "" && workValue != "" && baseValue != workValue {
				*changed = append(*changed, path)
			}
			continue
		}
		switch child.(type) {
		case *ContainerNode, *FlexNode:
			baseSub, workSub := base.GetContainer(childName), work.GetContainer(childName)
			if baseSub != nil && workSub != nil {
				changedSecretWalk(baseSub, workSub, child, path, changed)
			}
		case *ListNode, *InlineListNode:
			workEntries := work.GetList(childName)
			for key, baseEntry := range base.GetList(childName) {
				if workEntry := workEntries[key]; workEntry != nil {
					changedSecretWalk(baseEntry, workEntry, child, joinDotPath(path, key), changed)
				}
			}
		}
	}
}

// RejectMaskedSecretLeaves returns an error if any secret leaf in tree holds
// exactly SecretDataPlaceholder, the value a display mask emits. It is a
// fail-closed guard for the commit, load and upload/validate entry points: a
// masked `show config` (or a `ze config dump --strip` artifact) pasted back into
// a commit would otherwise clobber the stored secret with the placeholder.
// Rejecting (rather than silently resolving the placeholder to the committed
// value) keeps the behavior explicit and the error recoverable via the
// edit-authorized raw download or plaintext-<name>.
//
// It reads LeafHoldsSecret, so it refuses the placeholder wherever a display
// path can emit one. The read half and the write half MUST answer one
// predicate: a guard narrower than the mask beside it turns the mask into a way
// to destroy what it protects.
//
// Nil-safe: returns nil when tree or schema is nil.
func RejectMaskedSecretLeaves(tree *Tree, schema *Schema) error {
	if tree == nil || schema == nil {
		return nil
	}
	var masked []string
	rejectMaskedSecretWalk(tree, schema.root, "", &masked)
	if len(masked) == 0 {
		return nil
	}
	return fmt.Errorf("config leaf %s holds the display placeholder %q: the value was masked for display and cannot be used. Restore the real value from the edit-authorized raw config download, or set the password via plaintext-<name> (e.g. plaintext-password) or 'ze passwd'",
		strings.Join(masked, ", "), SecretDataPlaceholder)
}

func rejectMaskedSecretWalk(tree *Tree, node Node, prefix string, masked *[]string) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, childName := range cp.Children() {
		child := cp.Get(childName)
		path := joinDotPath(prefix, childName)
		if leaf, ok := child.(*LeafNode); ok {
			if !LeafHoldsSecret(leaf) {
				continue
			}
			if val, present := tree.Get(childName); present && val == SecretDataPlaceholder {
				*masked = append(*masked, path)
			}
			continue
		}
		switch n := child.(type) {
		case *ContainerNode:
			if sub := tree.GetContainer(childName); sub != nil {
				rejectMaskedSecretWalk(sub, n, path, masked)
			}
		case *FlexNode:
			if sub := tree.GetContainer(childName); sub != nil {
				rejectMaskedSecretWalk(sub, n, path, masked)
			}
		case *ListNode:
			for key, entry := range tree.GetList(childName) {
				rejectMaskedSecretWalk(entry, n, joinDotPath(path, key), masked)
			}
		case *InlineListNode:
			for key, entry := range tree.GetList(childName) {
				rejectMaskedSecretWalk(entry, n, joinDotPath(path, key), masked)
			}
		}
	}
}
