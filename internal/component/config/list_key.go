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

// KeyLeafDefault returns the YANG default of a non-first key leaf, false when
// it has none.
func (n *ListNode) KeyLeafDefault(name string) (string, bool) {
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
		if def, hasDefault := node.KeyLeafDefault(leaf); hasDefault && value == def {
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

// ListKeyFirstValue returns the first key value of a stored map key, for a
// list whose YANG key statement names keyLeaves. The completer holds YANG
// entries rather than list nodes, so it reads the key leaves itself. A list
// whose key names one leaf returns the key whole, as SplitListKey does.
func ListKeyFirstValue(keyLeaves []string, key string) string {
	key = StripListKeySuffix(key)
	if len(keyLeaves) < 2 {
		return key
	}
	first, _, _ := strings.Cut(key, " ")
	return first
}

// ListKeyTokens returns the address of a stored map key as path tokens:
// the first value, then each pair's name and value. Each token is written
// to text on its own, through QuoteIfNeeded.
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
		b.Str(QuoteIfNeeded(token))
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

// QuoteIfNeeded writes s as ONE token that the config tokenizer reads back as
// s: an address reaches text through it a token at a time. s is written bare
// only when bareWordReadsBack says the tokenizer returns it whole as one word;
// every other value is quoted, its `"`, `\`, newline and tab escaped the way
// readString and readQuotedToken decode them.
func QuoteIfNeeded(s string) string {
	if bareWordReadsBack(s) {
		return s
	}

	// Byte by byte, never rune by rune: a range over runes turns each byte of
	// invalid UTF-8 into U+FFFD, a different value than the one stored.
	var b textbuf.Buffer
	b.Byte('"')
	for i := range len(s) {
		c := s[i]
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
			b.Byte(c)
		}
	}
	b.Byte('"')
	return b.String()
}

// splitQuotedTokens splits a line into the tokens QuoteIfNeeded wrote: a
// token is a run of non-space characters, or a `"`-quoted string whose
// escapes (\", \\, \n, \t) QuoteIfNeeded wrote, or a `'`-quoted string
// with no escapes (a record written by hand or before keys were quoted). It
// is the inverse of QuoteIfNeeded: splitQuotedTokens(QuoteIfNeeded(s)) is
// [s]. An unterminated quote is refused.
func splitQuotedTokens(line string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(line); {
		c := line[i]
		if isTokenSpace(c) {
			i++
			continue
		}
		if c != '"' && c != '\'' {
			end := i
			for end < len(line) && !isTokenSpace(line[end]) {
				end++
			}
			tokens = append(tokens, line[i:end])
			i = end
			continue
		}
		token, next, err := readQuotedToken(line, i)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		i = next
	}
	return tokens, nil
}

// isTokenSpace reports whether c separates two tokens of a record.
func isTokenSpace(c byte) bool { return c == ' ' || c == '\t' }

// trimTokenSpace removes the separators isTokenSpace names, and the carriage
// return of a CRLF line, from both ends of a record line. It is not
// strings.TrimSpace: that also strips a vertical tab, a form feed and Unicode
// spaces, which a bare value QuoteIfNeeded writes may end with, so a set line
// would read back a shorter value than the one stored.
func trimTokenSpace(line string) string { return strings.Trim(line, " \t\r") }

// readQuotedToken reads the quoted token opening at line[start] and returns
// it and the index after its closing quote.
func readQuotedToken(line string, start int) (string, int, error) {
	quote := line[start]
	var b textbuf.Buffer
	for i := start + 1; i < len(line); i++ {
		c := line[i]
		if c == quote {
			return b.String(), i + 1, nil
		}
		if quote == '"' && c == '\\' && i+1 < len(line) {
			i++
			switch line[i] {
			case 'n':
				b.Byte('\n')
			case 't':
				b.Byte('\t')
			default:
				b.Byte(line[i])
			}
			continue
		}
		b.Byte(c)
	}
	return "", 0, errors.New("unterminated " + string(quote) + " quote in " + line)
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

// ListPathStep reads the entry a path addresses after a list name; rest is
// the path after the name. A path that ends at the list, or whose next token
// names a child of the list, addresses the anonymous entry (KeyDefault) and
// consumes no token. Otherwise the address is read by ParseListAddress. It is
// the one step every list path walker takes, so no walker reads an address
// its own way.
func ListPathStep(listName string, node *ListNode, rest []string) (ListAddress, error) {
	if len(rest) == 0 {
		return ListAddress{Key: KeyDefault, First: KeyDefault}, nil
	}
	if node.Get(rest[0]) != nil {
		return ListAddress{Key: KeyDefault, First: KeyDefault}, nil
	}
	return ParseListAddress(listName, node, rest)
}

// ListAddressLen returns how many tokens of rest the entry address after a
// list name spans, by the ListPathStep rule, without validating the values.
// It serves the walkers that resolve the schema only.
func ListAddressLen(node *ListNode, rest []string) int {
	return ListAddressSpan(node.KeyLeafNames(), rest, func(name string) bool { return node.Get(name) != nil })
}

// ListAddressSpan is the ListPathStep rule over any schema representation:
// keyLeaves are the list's key leaf names in YANG key order and isChild
// says whether a name is a child of the list. A path that ends, or whose
// next token is a child, spans no token (an anonymous entry); otherwise the
// first token, then a `name value` pair while the next token names a
// non-first key leaf and a token follows it. The CLI completer, which walks
// the YANG entries rather than the config schema, calls it so both read one
// address the same way.
func ListAddressSpan(keyLeaves, rest []string, isChild func(name string) bool) int {
	if len(rest) == 0 {
		return 0
	}
	if isChild(rest[0]) {
		return 0
	}
	n := 1
	if len(keyLeaves) < 2 {
		return n
	}
	for n+1 < len(rest) && slices.Contains(keyLeaves[1:], rest[n]) {
		n += 2
	}
	return n
}

// ListEntryTarget is the list entry a token path ends at.
type ListEntryTarget struct {
	ParentPath []string    // tokens before the list name
	ListName   string      // the list holding the entry
	Node       *ListNode   // the list's schema
	Address    ListAddress // the entry's address
	End        int         // tokens consumed, through the end of the address
	Keyless    string      // first keyless list the path crosses, empty when none
}

// errPathNotAtListEntry is the refusal of a path that does not name an entry.
var errPathNotAtListEntry = errors.New("path does not end at a list entry")

// WalkListEntryPath walks tokens from the schema root and reports the list
// entry they end at. The walk stops after a list entry's address when the
// next token is not a child of that list, so a caller may read more tokens
// (`to`, a destination) from End. ok is false when the walk ends anywhere
// but right after a keyed entry's address, and err is set only when an
// address the walk crosses is refused. It is the one target walk of the
// CLI editor, the web editor and the web terminal.
func WalkListEntryPath(schema *Schema, tokens []string) (target ListEntryTarget, ok bool, err error) {
	if schema == nil {
		return target, false, nil
	}
	var current ChildProvider = schema
	afterEntry := false
	for i := 0; i < len(tokens); {
		name := tokens[i]
		node := current.Get(name)
		if node == nil {
			if afterEntry {
				target.End = i
				return target, true, nil
			}
			return target, false, nil // Not a schema path: no entry, and the caller's own check names the element.
		}
		afterEntry = false
		switch n := node.(type) {
		case *ContainerNode:
			current = n
			i++
		case *FlexNode:
			current = n
			i++
		case *ListNode:
			if n.KeyName == "" && target.Keyless == "" {
				target.Keyless = name
			}
			addr, stepErr := ListPathStep(name, n, tokens[i+1:])
			if stepErr != nil {
				return target, false, stepErr
			}
			current = n
			if addr.Tokens == 0 {
				i++
				continue
			}
			target.ParentPath = tokens[:i]
			target.ListName = name
			target.Node = n
			target.Address = addr
			i += 1 + addr.Tokens
			afterEntry = true
		default:
			return target, false, nil
		}
	}
	target.End = len(tokens)
	return target, afterEntry, nil
}

// ResolveListEntryTarget returns the list entry a rename or copy names; verb
// is the operation, for the refusal. An entry of a keyless list, or under
// one, is refused before anything changes: such an entry has no key a
// destination could name.
func ResolveListEntryTarget(schema *Schema, verb string, tokens []string) (ListEntryTarget, error) {
	target, ok, err := WalkListEntryPath(schema, tokens)
	if err != nil {
		return target, err
	}
	if target.Keyless != "" {
		return target, errors.New(verb + " refused: " + target.Keyless +
			" has no key, so its entries cannot be renamed or copied.")
	}
	if !ok {
		return target, errPathNotAtListEntry
	}
	return target, nil
}

// errSameListKey is the refusal of a rename or copy onto its own source.
var errSameListKey = errors.New("new key must differ from current key")

// ListEntryDestination returns the key a rename or copy of the entry stored
// under oldKey moves it to; dst is the destination address as tokens and
// parentPath the tokens before the list, for the refusal's fix lines. A
// rename or copy changes only the first key value: every other key leaf
// carries over from the source, and a destination that gives one another
// value is refused, naming the leaf and saying to delete the entry and
// create it again. A destination equal to the source is refused too.
func ListEntryDestination(verb string, parentPath []string, listName string, node *ListNode, oldKey string, dst []string) (string, error) {
	addr, err := ParseListAddress(listName, node, dst)
	if err != nil {
		return "", err
	}
	if addr.Tokens != len(dst) {
		return "", errors.New("invalid destination for " + listName + ": " + dst[addr.Tokens] + " is not a key leaf of " + listName)
	}
	_, srcPairs := SplitListKey(node, oldKey)
	source := ListAddress{Pairs: srcPairs}
	sourceValue := func(leaf string) (string, bool) {
		if value, ok := source.pairValue(leaf); ok {
			return value, true
		}
		return node.KeyLeafDefault(leaf)
	}
	for _, pair := range addr.Pairs {
		if value, ok := sourceValue(pair.Leaf); ok && value == pair.Value {
			continue
		}
		return "", keyLeafMoveRefusal(verb, parentPath, listName, node, oldKey, addr, pair)
	}
	newKey, err := BuildListKey(listName, node, addr.First, source.pairValue)
	if err != nil {
		return "", err
	}
	if newKey == StripListKeySuffix(oldKey) {
		return "", errSameListKey
	}
	return newKey, nil
}

// keyLeafMoveRefusal writes the refusal of a rename or copy that changes a
// non-first key leaf, with the delete and set lines that move the entry.
func keyLeafMoveRefusal(verb string, parentPath []string, listName string, node *ListNode, oldKey string, dst ListAddress, changed ListKeyPair) error {
	_, srcPairs := SplitListKey(node, oldKey)
	source := ListAddress{Pairs: srcPairs}
	movedValue := func(leaf string) (string, bool) {
		if value, ok := dst.pairValue(leaf); ok {
			return value, true
		}
		return source.pairValue(leaf)
	}
	movedKey, err := BuildListKey(listName, node, dst.First, movedValue)
	if err != nil {
		return err
	}
	var b textbuf.Buffer
	b.Str(verb).Str(" refused: ").Str(changed.Leaf).Str(" is part of the key of ").Str(listName).
		Str(", and ").Str(verb).Str(" changes only ").Str(node.FirstKeyLeafName()).Str(".\n")
	b.Str("To move the entry to ").Str(changed.Value).Str(", delete it and create it again:\n")
	b.Str("  delete ")
	writeListAddress(&b, append(append(append([]string{}, parentPath...), listName), ListKeyTokens(node, oldKey)...))
	b.Str("\n  set ")
	newTokens := append(append(append([]string{}, parentPath...), listName), ListKeyTokens(node, movedKey)...)
	writeListAddress(&b, newTokens)
	return errors.New(b.String())
}
