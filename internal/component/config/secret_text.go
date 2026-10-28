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
	decoded, wasEncoded, err := DecodeSecretText(name, leaf, value)
	if err != nil {
		return "", err
	}
	if wasEncoded {
		tree.SetStoredSecret(name, value)
	}
	return decoded, nil
}

// DecodeSecretText answers the key text spells for the leaf name: a $9$
// string on a ze:sensitive leaf is decoded, and wasEncoded says so; every
// other text is answered unchanged. It serves a leaf value (decodeSecretLeaf),
// a session entry's ^previous field, which a meta writer prints in the $9$
// form too (encodeSecretMeta), and a value a client sets through the editor
// (a gNMI Set, a REST or gRPC config session), which keeps text as the
// leaf's stored text when wasEncoded (Tree.SetStoredSecret). A nil leaf
// holds no secret.
func DecodeSecretText(name string, leaf *LeafNode, text string) (key string, wasEncoded bool, err error) {
	if !reversibleSecret(leaf) {
		return text, false, nil
	}
	if !secret.IsEncoded(text) {
		return text, false, nil
	}
	decoded, err := secret.Decode(text)
	if err != nil {
		return "", false, fmt.Errorf("invalid $9$ encoding for %s: %w", name, err)
	}
	return decoded, true, nil
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

// EncodeSecrets is encodeSecrets for a writer whose signature carries no
// error: the printers here, and every surface outside this package that
// publishes values read from a tree holding decoded keys (gNMI Get and
// Subscribe read the running tree). Such a surface reads the copy this
// answers, never tree itself. secret.Encode fails only when crypto/rand.Read
// does, and since Go 1.24 crypto/rand.Read never returns an error: it ends
// the program instead. Safe for concurrent use: Tree locks every read, and
// the one write to tree, the memoized stored text, goes through
// Tree.SetStoredSecret.
func EncodeSecrets(tree *Tree, schema *Schema) *Tree {
	printed, err := encodeSecrets(tree, schema)
	if err != nil {
		panic("impossible: " + err.Error())
	}
	return printed
}

// SecretValueAtPath answers the text a surface may publish for value, written
// at the leaf that path, a list of CLI tokens, names: a ze:sensitive key in
// its $9$ form (secretSpelling), every other value as DisplayValueAtPath
// answers it. So a one-way hash reads as the placeholder, and a path that
// resolves to no leaf reads as the placeholder too: that path cannot be told
// from one that names a credential. It serves a surface that echoes a value
// a client wrote, with no tree to encode (a gNMI Set notification).
func SecretValueAtPath(schema *Schema, path []string, value string) string {
	leaf, isLeaf := schema.LookupTokenPath(path).(*LeafNode)
	if isLeaf && reversibleSecret(leaf) {
		return secretSpelling("", value)
	}
	return DisplayValueAtPath(schema, path, value)
}

// encodeSecretLeaves rewrites each ze:sensitive leaf of printed, under the
// schema node cp, into its $9$ form, recording a fresh encoding in source,
// the tree printed was cloned from; it walks both side by side. It follows
// the tree's storage, as maskWalk does: a ContainerNode's and a FlexNode's
// children live in the container of that name, a ListNode's and an
// InlineListNode's in the entries of the list of that name. The other node
// kinds hold no LeafNode. The recursion is bounded by the schema's depth.
func encodeSecretLeaves(source, printed *Tree, cp ChildProvider) error {
	for _, name := range cp.Children() {
		var err error
		switch child := cp.Get(name).(type) {
		case *LeafNode:
			if reversibleSecret(child) {
				err = encodeSecretLeaf(source, printed, name)
			}
		case *ContainerNode:
			err = encodeSecretContainer(source, printed, name, child)
		case *FlexNode:
			err = encodeSecretContainer(source, printed, name, child)
		case *ListNode:
			err = encodeSecretList(source, printed, name, child)
		case *InlineListNode:
			err = encodeSecretList(source, printed, name, child)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// encodeSecretContainer is encodeSecretLeaves over the container name, whose
// schema node is cp, of source and printed. A source that lost the container
// gets no stored text, which only costs a fresh encoding on the next print.
func encodeSecretContainer(source, printed *Tree, name string, cp ChildProvider) error {
	printedSub := printed.GetContainer(name)
	// refuse-by-default: guards encodeSecretLeaves -- a container the tree does not hold holds no key to print
	if printedSub == nil {
		return nil
	}
	var sourceSub *Tree
	if source != nil {
		sourceSub = source.GetContainer(name)
	}
	return encodeSecretLeaves(sourceSub, printedSub, cp)
}

// encodeSecretList is encodeSecretLeaves over every entry of the list name,
// whose schema node is cp, of printed, beside the same entry of source.
func encodeSecretList(source, printed *Tree, name string, cp ChildProvider) error {
	var sourceEntries map[string]*Tree
	if source != nil {
		sourceEntries = source.GetList(name)
	}
	for key, entry := range printed.GetList(name) {
		if err := encodeSecretLeaves(sourceEntries[key], entry, cp); err != nil {
			return err
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

// encodeSecretMeta answers a copy of meta in which every session entry of a
// ze:sensitive leaf holds its value and its ^previous key in the $9$ form,
// for SerializeSetWithMeta: printed is the tree that printer writes, already
// encoded (encodeSecrets). So no meta line prints a key in clear, whichever
// writer emits it: a contested entry (writeMetaLeafLineFormat), a value a
// session set that another session deleted (writeDeleteMetaLines), and the
// ^previous field of either (writeMetaPrefix). The parsers decode both
// fields back (DecodeSecretText), so the editor compares keys, never
// encodings. A meta that holds no entry under a ze:sensitive leaf name is
// answered as it is, as encodeSecrets does for its tree.
func encodeSecretMeta(printed *Tree, meta *MetaTree, schema *Schema) *MetaTree {
	if meta == nil {
		return nil
	}
	if !metaHoldsEntryNamed(meta, schema.sensitiveKeyNames()) {
		return meta
	}
	encoded := meta.Clone()
	encodeSecretMetaEntries(printed, encoded, schema.root)
	return encoded
}

// metaHoldsEntryNamed reports whether meta, or any meta under it, holds an
// entry under one of names; like holdsValueNamed it may answer true for a
// name that is not sensitive there, and never false for one that is. The
// recursion is bounded by the meta tree's depth, which the schema bounds.
func metaHoldsEntryNamed(meta *MetaTree, names map[string]bool) bool {
	meta.mu.RLock()
	defer meta.mu.RUnlock()
	for name := range meta.entries {
		if names[name] {
			return true
		}
	}
	for _, sub := range meta.containers {
		if metaHoldsEntryNamed(sub, names) {
			return true
		}
	}
	for _, sub := range meta.lists {
		if metaHoldsEntryNamed(sub, names) {
			return true
		}
	}
	return false
}

// encodeSecretMetaEntries rewrites, in meta (a private copy), each session
// entry of a ze:sensitive leaf under the schema node cp into its $9$
// spelling, walking printed beside it: a container's meta is
// meta.containers[name], a list entry's is meta.containers[name].lists[key]
// (metaListEntry). printed is nil where the tree holds nothing at that place,
// which is where a deleted value's entries live. A FlexNode is stored as a
// container and an InlineListNode as a list, in the tree and in the meta
// (cli walkOrCreateMeta), as encodeSecretLeaves follows them. The recursion
// is bounded by the schema's depth.
func encodeSecretMetaEntries(printed *Tree, meta *MetaTree, cp ChildProvider) {
	for _, name := range cp.Children() {
		switch child := cp.Get(name).(type) {
		case *LeafNode:
			if reversibleSecret(child) {
				encodeSecretMetaLeaf(printed, meta, name)
			}
		case *ContainerNode:
			encodeSecretMetaContainer(printed, meta, name, child)
		case *FlexNode:
			encodeSecretMetaContainer(printed, meta, name, child)
		case *ListNode:
			encodeSecretMetaList(printed, meta, name, child)
		case *InlineListNode:
			encodeSecretMetaList(printed, meta, name, child)
		}
	}
}

// encodeSecretMetaContainer is encodeSecretMetaEntries over the container
// name, whose schema node is cp: its meta is meta.containers[name].
func encodeSecretMetaContainer(printed *Tree, meta *MetaTree, name string, cp ChildProvider) {
	if sub := meta.containers[name]; sub != nil {
		encodeSecretMetaEntries(printedContainer(printed, name), sub, cp)
	}
}

// encodeSecretMetaList is encodeSecretMetaEntries over every entry of the
// list name, whose schema node is cp: an entry's meta is
// meta.containers[name].lists[key].
func encodeSecretMetaList(printed *Tree, meta *MetaTree, name string, cp ChildProvider) {
	listMeta := meta.containers[name]
	if listMeta == nil {
		return
	}
	for key, entryMeta := range listMeta.lists {
		encodeSecretMetaEntries(printedListEntry(printed, name, key), entryMeta, cp)
	}
}

// printedContainer answers the container name of printed, or nil when
// printed is nil or holds none.
func printedContainer(printed *Tree, name string) *Tree {
	if printed == nil {
		return nil
	}
	return printed.GetContainer(name)
}

// printedListEntry answers the entry key of the list name of printed, or nil
// when printed is nil or holds no such entry.
func printedListEntry(printed *Tree, name, key string) *Tree {
	if printed == nil {
		return nil
	}
	return printed.GetList(name)[key]
}

// encodeSecretMetaLeaf spells the value and ^previous key of every entry of
// the leaf name in meta in the $9$ form (secretSpelling), against the text
// printed holds for the leaf.
func encodeSecretMetaLeaf(printed *Tree, meta *MetaTree, name string) {
	var printedValue string
	if printed != nil {
		printedValue, _ = printed.Get(name)
	}
	entries := meta.entries[name]
	for i := range entries {
		entries[i].Value = secretSpelling(printedValue, entries[i].Value)
		entries[i].Previous = secretSpelling(printedValue, entries[i].Previous)
	}
}

// secretSpelling answers the $9$ text a meta writer prints for key: the
// printed tree's text when it decodes to key, so an entry that holds the
// tree's own key repeats its stored encoding byte for byte; else a fresh
// encoding. An empty key (a delete, or no previous value), a key already in
// the $9$ form and the mask placeholder are printed as they are.
func secretSpelling(printedValue, key string) string {
	// refuse-by-default: guards writeDeleteMetaLines -- an empty Value is a session's delete, never a key
	if key == "" {
		return key
	}
	if secret.IsEncoded(key) {
		return key
	}
	if key == SecretDataPlaceholder {
		return key
	}
	if decoded, err := secret.Decode(printedValue); err == nil && decoded == key {
		return printedValue
	}
	encoded, err := secret.Encode(key)
	if err != nil {
		panic("impossible: " + err.Error())
	}
	return encoded
}
