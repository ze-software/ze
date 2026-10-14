// Design: docs/architecture/config/syntax.md — multi-leaf list keys
// Related: parser_list.go — the config file parser stores entries under the canonical key
// Related: tree.go — Tree.AddListEntry, the map the canonical key indexes
// Related: serialize.go — brace-form printers write the first key value only

package config

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// This file is the one codec for list entry ADDRESSES. A YANG list whose key
// names several leaves (`key "peer vrf interface"`) stores each entry under
// its canonical address: the first key leaf's value, then, in YANG key order,
// `name value` for each other key leaf whose value is not its YANG default.
// A key leaf with no default is omitted only when absent. So `fe80::b` is the
// entry with no `interface` and `vrf` absent or `default`, and
// `fe80::b interface zbl1` is the one on the second link. A list whose key
// names one leaf keeps its key exactly as written: every function here
// returns it unchanged.
//
// The address is a sequence of tokens, never one string written raw: a key
// value of a multi-leaf-key list holding whitespace, or empty, is refused
// here, so splitting a stored key on spaces is exact.

// ListKeyPair is one non-first key leaf of an address, as written.
type ListKeyPair struct {
	Leaf  string
	Value string
}

// ListAddress is an address read from path tokens by ParseListAddress.
type ListAddress struct {
	Key    string        // canonical map key the entry is stored under
	First  string        // first key leaf value
	Pairs  []ListKeyPair // non-first key leaves the tokens named, as written
	Tokens int           // path tokens the address consumed
}

// KeyLeafNames returns the list's key leaf names in YANG key order, nil for
// a keyless list.
func (n *ListNode) KeyLeafNames() []string {
	return strings.Fields(n.KeyName)
}

// HasMultiLeafKey reports whether the list's YANG key names more than one leaf.
func (n *ListNode) HasMultiLeafKey() bool {
	return len(n.KeyLeafNames()) > 1
}

// FirstKeyLeafName returns the name of the leaf an entry is written under,
// empty for a keyless list.
func (n *ListNode) FirstKeyLeafName() string {
	names := n.KeyLeafNames()
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// isNonFirstKeyLeaf reports whether name is a key leaf of the list other
// than the first.
func (n *ListNode) isNonFirstKeyLeaf(name string) bool {
	names := n.KeyLeafNames()
	return slices.Contains(names[min(1, len(names)):], name)
}

// keyLeafDefault returns the YANG default of a non-first key leaf.
func (n *ListNode) keyLeafDefault(name string) (string, bool) {
	leaf, ok := n.Get(name).(*LeafNode)
	if !ok {
		return "", false
	}
	if leaf.Default == "" {
		return "", false
	}
	return leaf.Default, true
}

// errListKeyValue is the refusal of an empty or space-holding key value.
var errListKeyValue = errors.New("a key value cannot be empty or hold a space")

// checkListKeyValue refuses a key value of a multi-leaf-key list that is
// empty or holds whitespace: either would make the address ambiguous.
func checkListKeyValue(listName, leaf, value string) error {
	if value != "" && !strings.ContainsAny(value, " \t\n\r") {
		return nil
	}
	var b textbuf.Buffer
	b.Str("invalid key for ").Str(listName).Str(": ").Str(leaf).
		Str(" is part of the key, and ").Str(errListKeyValue.Error()).
		Str(": remove the ").Str(leaf).Str(" leaf or give it a value")
	return errors.New(b.String())
}

// BuildListKey returns the canonical address of the entry whose first key
// value is first and whose other key leaves valueOf answers. For a list
// whose key names one leaf it returns first unchanged.
func BuildListKey(listName string, node *ListNode, first string, valueOf func(leaf string) (string, bool)) (string, error) {
	if !node.HasMultiLeafKey() {
		return first, nil
	}
	names := node.KeyLeafNames()
	if err := checkListKeyValue(listName, names[0], first); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	b.Str(first)
	for _, leaf := range names[1:] {
		value, ok := valueOf(leaf)
		if !ok {
			continue
		}
		if err := checkListKeyValue(listName, leaf, value); err != nil {
			return "", err
		}
		if def, hasDefault := node.keyLeafDefault(leaf); hasDefault && value == def {
			continue // A key leaf at its YANG default is omitted from the address.
		}
		b.Byte(' ').Str(leaf).Byte(' ').Str(value)
	}
	return b.String(), nil
}

// ParseListAddress reads an entry address from the front of path tokens.
// The first token is the first key value. A `name value` pair is then
// consumed while the next token names a non-first key leaf of the list AND
// a token follows it; otherwise the address ends and the next token is read
// as it is today (a child name, a leaf, or a value). The rule runs left to
// right with no choice point, so no path reads two ways. A key leaf named
// twice is refused, and each value is validated against its own leaf.
func ParseListAddress(listName string, node *ListNode, tokens []string) (ListAddress, error) {
	if len(tokens) == 0 {
		return ListAddress{}, errors.New("missing key for " + listName)
	}
	addr := ListAddress{First: tokens[0], Tokens: 1}
	if !node.HasMultiLeafKey() {
		addr.Key = tokens[0]
		return addr, nil
	}
	for addr.Tokens+1 < len(tokens) && node.isNonFirstKeyLeaf(tokens[addr.Tokens]) {
		leafName := tokens[addr.Tokens]
		value := tokens[addr.Tokens+1]
		for _, pair := range addr.Pairs {
			if pair.Leaf == leafName {
				return ListAddress{}, errors.New("invalid key for " + listName + ": " + leafName +
					" is given twice; a key leaf is set once, when the entry is created")
			}
		}
		if leaf, ok := node.Get(leafName).(*LeafNode); ok {
			if err := ValidateLeafValue(leaf, value); err != nil {
				return ListAddress{}, errors.New("invalid " + leafName + " for " + listName + ": " + err.Error())
			}
		}
		addr.Pairs = append(addr.Pairs, ListKeyPair{Leaf: leafName, Value: value})
		addr.Tokens += 2
	}
	key, err := BuildListKey(listName, node, addr.First, addr.pairValue)
	if err != nil {
		return ListAddress{}, err
	}
	addr.Key = key
	return addr, nil
}

// pairValue answers a non-first key leaf the address named.
func (a ListAddress) pairValue(leaf string) (string, bool) {
	for _, pair := range a.Pairs {
		if pair.Leaf == leaf {
			return pair.Value, true
		}
	}
	return "", false
}

// SplitListKey splits a stored map key into its first value and its pairs.
// A positional `#N` suffix is removed first. For a list whose key names one
// leaf the key is returned whole, because a single key may hold a space.
func SplitListKey(node *ListNode, key string) (string, []ListKeyPair) {
	key = StripListKeySuffix(key)
	if !node.HasMultiLeafKey() {
		return key, nil
	}
	fields := strings.Fields(key)
	if len(fields) == 0 {
		return key, nil
	}
	var pairs []ListKeyPair
	for i := 1; i+1 < len(fields); i += 2 {
		pairs = append(pairs, ListKeyPair{Leaf: fields[i], Value: fields[i+1]})
	}
	return fields[0], pairs
}

// ListKeyTokens returns the address of a stored map key as path tokens:
// the first value, then each pair's name and value. Each token is written
// to text on its own, through quoteIfNeeded.
func ListKeyTokens(node *ListNode, key string) []string {
	first, pairs := SplitListKey(node, key)
	tokens := make([]string, 0, 1+2*len(pairs))
	tokens = append(tokens, first)
	for _, pair := range pairs {
		tokens = append(tokens, pair.Leaf, pair.Value)
	}
	return tokens
}

// EnsureListEntry returns the entry stored under the address, creating it
// under the canonical key when absent, and writes the address's pairs as
// its key leaves. It is the one creator every addressing surface calls, so
// an entry reached from an address always holds the key leaves its key
// names. Writing onto an existing entry changes no key leaf: an equal key
// means equal pairs, and a pair at its default (an explicit `vrf default`)
// only keeps the leaf explicit, as written.
func EnsureListEntry(tree *Tree, listName string, addr ListAddress) *Tree {
	entry := tree.GetList(listName)[addr.Key]
	if entry == nil {
		entry = NewTree()
		tree.AddListEntry(listName, addr.Key, entry)
	}
	for _, pair := range addr.Pairs {
		entry.Set(pair.Leaf, pair.Value)
	}
	return entry
}

// setFormEntry returns the address tokens a set-form line writes for a stored
// key, and the schema view the emitter walks for the entry's body. For a
// multi-leaf-key list the view answers nil for each key leaf the address
// already carries, so no line sets it on its own: replayed, such a line would
// name the key leaf twice. Every other list gets its own node back.
func setFormEntry(node *ListNode, key string) ([]string, ChildProvider) {
	tokens := ListKeyTokens(node, key)
	if !node.HasMultiLeafKey() {
		return tokens, node
	}
	_, pairs := SplitListKey(node, key)
	inAddress := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		inAddress = append(inAddress, pair.Leaf)
	}
	return tokens, addressedEntry{list: node, inAddress: inAddress}
}

// addressedEntry is the schema view of one multi-leaf-key entry in set form.
// Children keeps every name, so a key leaf is never mistaken for a value the
// schema does not know; Get answers nil for the key leaves in the address,
// which the set-form dispatchers write nothing for.
type addressedEntry struct {
	list      *ListNode
	inAddress []string
}

// Children returns the list's children, key leaves included.
func (e addressedEntry) Children() []string { return e.list.Children() }

// Get returns the list's child, or nil for a key leaf the address carries.
func (e addressedEntry) Get(name string) Node {
	if slices.Contains(e.inAddress, name) {
		return nil
	}
	return e.list.Get(name)
}

// writeListAddress appends the address of a stored key to b, each token
// quoted on its own.
func writeListAddress(b *textbuf.Buffer, tokens []string) {
	for i, token := range tokens {
		if i > 0 {
			b.Byte(' ')
		}
		b.Str(quoteIfNeeded(token))
	}
}

// canonicalParsedListKey returns the canonical address of an entry the
// config file parser read under its first word. The body's own key leaves
// supply the pairs; a body that names the first key leaf with another value
// than the first word is refused, because the entry would then name two
// first values.
func canonicalParsedListKey(listName string, node *ListNode, first string, entry *Tree) (string, error) {
	if !node.HasMultiLeafKey() {
		return first, nil
	}
	if value, ok := entry.Get(node.FirstKeyLeafName()); ok && value != first {
		return "", errors.New("invalid key for " + listName + ": the entry is written under " + first +
			" and its " + node.FirstKeyLeafName() + " leaf says " + value + ": remove the leaf")
	}
	return BuildListKey(listName, node, first, entry.Get)
}

// quoteIfNeeded quotes a string if it contains spaces or special characters.
// It writes ONE token: an address reaches text through it a token at a time.
func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}

	needsQuote := false
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' || c == '\'' || c == '{' || c == '}' || c == ';' || c == '#' {
			needsQuote = true
			break
		}
	}

	if !needsQuote {
		return s
	}

	// Escape quotes and backslashes
	var b textbuf.Buffer
	b.Byte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.Str(`\"`)
		case '\\':
			b.Str(`\\`)
		case '\n':
			b.Str(`\n`)
		case '\t':
			b.Str(`\t`)
		default:
			b.WriteRune(c)
		}
	}
	b.Byte('"')
	return b.String()
}

// ListKeyOpeningValue returns what a brace-form opening line writes for a
// stored key. A multi-leaf-key list writes the first value only, because the
// entry's body already carries the other key leaves: writing the whole
// address would quote it into one first value holding spaces, which reads
// back refused. Every other list writes the key unchanged.
func ListKeyOpeningValue(node *ListNode, key string) string {
	if node == nil {
		return key
	}
	if !node.HasMultiLeafKey() {
		return key
	}
	first, _ := SplitListKey(node, key)
	return first
}

// emitAddressLine writes `set <address>` for an active entry whose address
// carries key leaf pairs. Its key leaves get no line of their own (the
// address carries them), so an entry holding nothing else would otherwise
// print no line at all and be lost on replay. Replayed, the line reaches or
// creates the entry with its key leaves written (EnsureListEntry).
func emitAddressLine(b *textbuf.Buffer, entry *Tree, addressTokens []string, entryPath string) {
	if len(addressTokens) < 2 {
		return
	}
	if entry.IsInactive() {
		return // The nop line emitStructuralNop wrote addresses the entry.
	}
	b.Str("set ").Str(entryPath).Byte('\n')
}
