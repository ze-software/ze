// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: reference_path.go — schema navigation of a leafref path
// Related: validator.go — validateLeafValue dispatches the three reference kinds here
// RFC: rfc/short/rfc7950.md -- Sections 9.9, 9.10 and 9.13

package yang

import (
	"slices"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// RFC 7950 Section 9.9: "The value space of the referring node is the value
// space of the referred node."
// RFC 7950 Section 9.9.4: "A leafref value is lexically represented the same
// way as the leaf it references represents its value."
//
// validateLeafref checks a leafref value against the type of the leaf it
// refers to. Whether the referred instance exists is a property of the whole
// data tree, which ValidateReferences checks.
func (v *Validator) validateLeafref(path string, leaf *yang.Entry, yangType *yang.YangType, value any) error {
	if leaf == nil {
		return referenceValidationError(path, yangType, value, "a leafref value is checked against the leaf it refers to, and no leaf was given: this is a Ze defect, report it")
	}
	target, _, err := v.leafrefTarget(leaf, yangType)
	if err != nil {
		return referenceValidationError(path, yangType, value, err.Error())
	}
	return v.validateLeafValue(path, target, target.Type, value)
}

// validateIdentityref checks that an identityref value names an identity
// derived from the type's base.
//
// RFC 7950 Section 9.10.2: "Valid values for an identityref are any
// identities derived from all the identityref's base identities."
// RFC 7950 Section 9.10.3: "An identityref is lexically represented as the
// referred identity's qualified name as defined in [XML-NAMES].  If the
// prefix is not present, the namespace of the identityref is the default
// namespace in effect on the element that contains the identityref value."
//
// Config text carries no XML namespace, so the prefix, when written, is the
// prefix or the name of the module defining the identity, and an unprefixed
// name is accepted when exactly one derived identity bears it.
func (v *Validator) validateIdentityref(path string, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return referenceValidationError(path, yangType, value, "expected an identity name")
	}
	base := yangType.IdentityBase
	if base == nil {
		return referenceValidationError(path, yangType, value, "the identityref type has no resolved base: this is a Ze defect, report it")
	}
	if identifierRefLength(str) != len(str) {
		return referenceValidationError(path, yangType, value, "an identity is named as name or prefix:name: write one of "+identityChoices(base))
	}
	prefix, name := nodePrefix(str), localName(str)
	var matched []*yang.Identity
	for _, identity := range base.Values {
		if identity.Name != name {
			continue
		}
		if prefix != "" && !identityPrefixed(identity, prefix) {
			continue
		}
		matched = append(matched, identity)
	}
	if len(matched) == 1 {
		return nil
	}
	if len(matched) > 1 {
		return referenceValidationError(path, yangType, value, "several modules define an identity "+name+": write it with its module prefix, one of "+identityChoices(base))
	}
	return referenceValidationError(path, yangType, value, "no identity derived from "+base.Name+" is called "+str+": write one of "+identityChoices(base))
}

// identityPrefixed reports whether prefix names the module that defines
// identity, by its prefix or by its module name.
func identityPrefixed(identity *yang.Identity, prefix string) bool {
	mod := yang.RootNode(identity)
	if mod == nil {
		return false
	}
	if mod.GetPrefix() == prefix {
		return true
	}
	return mod.Name == prefix
}

// identityChoices lists the qualified names of the identities derived from
// base, the values an identityref of that base accepts.
func identityChoices(base *yang.Identity) string {
	names := make([]string, 0, len(base.Values))
	for _, identity := range base.Values {
		names = append(names, identity.PrefixedName())
	}
	if len(names) == 0 {
		return "(none: no identity is derived from " + base.Name + ")"
	}
	return strings.Join(names, ", ")
}

// instanceStep is one step of an instance-identifier value.
type instanceStep struct {
	node  string // "prefix:name"
	keys  []instanceKey
	value string // the leaf-list-predicate value, when hasValue
	pos   int    // the positional predicate, zero when absent
	// hasValue marks a leaf-list-predicate: its value may be "".
	hasValue bool
}

// instanceKey is one key-predicate of an instance-identifier step.
type instanceKey struct {
	node  string
	value string
}

// parseInstanceIdentifier reads an instance-identifier value.
//
// RFC 7950 Section 14: "instance-identifier = 1*("/" (node-identifier
// [1*key-predicate / leaf-list-predicate / pos]))". The reader is
// iterative and every loop consumes at least one byte, so its work is
// bounded by the value's length.
func parseInstanceIdentifier(text string) ([]instanceStep, bool) {
	var steps []instanceStep
	at := 0
	for at < len(text) {
		if text[at] != '/' {
			return nil, false
		}
		at++
		n := identifierRefLength(text[at:])
		if n == 0 {
			return nil, false
		}
		step := instanceStep{node: text[at : at+n]}
		at += n
		var ok bool
		if at, ok = readInstancePredicates(text, at, &step); !ok {
			return nil, false
		}
		steps = append(steps, step)
	}
	return steps, len(steps) > 0
}

// readInstancePredicates reads the predicates after a node-identifier:
// one or more key-predicates, or one leaf-list-predicate, or one pos.
//
// RFC 7950 Section 14: "key-predicate-expr = node-identifier *WSP "=" *WSP
// quoted-string", "leaf-list-predicate-expr = "." *WSP "=" *WSP
// quoted-string", "pos = "[" *WSP positive-integer-value *WSP "]"".
func readInstancePredicates(text string, at int, step *instanceStep) (int, bool) {
	for at < len(text) && text[at] == '[' {
		if step.hasValue || step.pos > 0 {
			return at, false
		}
		at = skipWSP(text, at+1)
		if at >= len(text) {
			return at, false
		}
		switch c := text[at]; {
		case c == '.':
			if len(step.keys) > 0 {
				return at, false
			}
			value, next, ok := readEqualsQuoted(text, at+1)
			if !ok {
				return at, false
			}
			step.value, step.hasValue, at = value, true, next
		case c >= '1' && c <= '9':
			if len(step.keys) > 0 {
				return at, false
			}
			end := at
			for end < len(text) && text[end] >= '0' && text[end] <= '9' {
				end++
			}
			pos, err := strconv.Atoi(text[at:end])
			if err != nil {
				return at, false
			}
			step.pos, at = pos, end
		default:
			n := identifierRefLength(text[at:])
			if n == 0 {
				return at, false
			}
			key := instanceKey{node: text[at : at+n]}
			value, next, ok := readEqualsQuoted(text, at+n)
			if !ok {
				return at, false
			}
			key.value, at = value, next
			step.keys = append(step.keys, key)
		}
		at = skipWSP(text, at)
		if at >= len(text) || text[at] != ']' {
			return at, false
		}
		at++
	}
	return at, true
}

// readEqualsQuoted reads `*WSP "=" *WSP quoted-string` from at.
//
// RFC 7950 Section 14: "quoted-string = (DQUOTE string DQUOTE) / (SQUOTE
// string SQUOTE)".
func readEqualsQuoted(text string, at int) (string, int, bool) {
	at = skipWSP(text, at)
	if at >= len(text) || text[at] != '=' {
		return "", at, false
	}
	at = skipWSP(text, at+1)
	if at >= len(text) {
		return "", at, false
	}
	quote := text[at]
	if quote != '"' && quote != '\'' {
		return "", at, false
	}
	end := strings.IndexByte(text[at+1:], quote)
	if end < 0 {
		return "", at, false
	}
	return text[at+1 : at+1+end], at + 2 + end, true
}

// skipWSP skips "*WSP", spaces and tabs.
func skipWSP(text string, at int) int {
	for at < len(text) && (text[at] == ' ' || text[at] == '\t') {
		at++
	}
	return at
}

// validateInstanceIdentifier checks an instance-identifier value: its
// syntax, its prefixes, and that each step names a data node with the
// predicates that node takes. Whether the instance exists is checked by
// ValidateReferences.
//
// RFC 7950 Section 9.13: "For identifying list entries with keys, each
// predicate consists of one equality test per key, and each key MUST have a
// corresponding predicate."
// RFC 7950 Section 9.13.2: "All node names in an instance-identifier value
// MUST be qualified with explicit namespace prefixes".
func (v *Validator) validateInstanceIdentifier(path string, leaf *yang.Entry, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return referenceValidationError(path, yangType, value, "expected an instance-identifier path")
	}
	steps, ok := parseInstanceIdentifier(str)
	if !ok {
		return referenceValidationError(path, yangType, value, "not an instance-identifier: write /prefix:node[prefix:key='value']/prefix:leaf (RFC 7950 Section 14)")
	}
	if _, err := v.instanceSchema(leaf, steps); err != nil {
		return referenceValidationError(path, yangType, value, err.Error())
	}
	return nil
}

// instanceSchema resolves the schema nodes an instance-identifier's steps
// name, checking each step's prefix and predicates.
func (v *Validator) instanceSchema(leaf *yang.Entry, steps []instanceStep) ([]*yang.Entry, error) {
	var tb textbuf.Buffer
	nodes := make([]*yang.Entry, 0, len(steps))
	parents := v.topLevelParents(leaf, localName(steps[0].node))
	for _, step := range steps {
		prefix := nodePrefix(step.node)
		if prefix == "" {
			return nil, refError(tb.Str("node ").Quoted(step.node).Str(" has no module prefix: write every node as prefix:name (RFC 7950 Section 9.13.2)"))
		}
		node := childOf(parents, localName(step.node))
		if node == nil {
			return nil, refError(tb.Str("names no node ").Quoted(step.node).Str(": correct the path to an existing node"))
		}
		if !entryPrefixed(node, prefix) {
			return nil, refError(tb.Str("prefix ").Quoted(prefix).Str(" is not the module of ").Quoted(localName(step.node)).Str(": write ").Str(entryPrefix(node)).Byte(':').Str(localName(step.node)))
		}
		if err := checkInstancePredicates(node, step); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
		parents = []*yang.Entry{node}
	}
	return nodes, nil
}

// entryPrefix answers the prefix of the module a schema node belongs to.
func entryPrefix(e *yang.Entry) string {
	if e.Prefix != nil {
		return e.Prefix.Name
	}
	return ""
}

// entryPrefixed reports whether prefix names e's module, by prefix or name.
func entryPrefixed(e *yang.Entry, prefix string) bool {
	if entryPrefix(e) == prefix {
		return true
	}
	mod, err := e.InstantiatingModule()
	if err != nil {
		return false
	}
	return mod == prefix
}

// RFC 7950 Section 9.13: "Predicates are used only for specifying the
// values for the key nodes for list entries, a value of a leaf-list entry,
// or a positional index for a list without keys."
//
// checkInstancePredicates checks the predicates of one step against the node
// it names.
func checkInstancePredicates(node *yang.Entry, step instanceStep) error {
	var tb textbuf.Buffer
	switch {
	case node.IsList():
		keys := listKeys(node)
		if len(keys) == 0 {
			if step.pos == 0 {
				return refError(tb.Str("list ").Quoted(node.Name).Str(" has no key: select an entry by position, [1] for the first"))
			}
			return nil
		}
		if step.pos > 0 {
			return refError(tb.Str("list ").Quoted(node.Name).Str(" has keys: select its entry by ").Quoted(node.Key).Str(", not by position"))
		}
		if step.hasValue {
			return refError(tb.Str("list ").Quoted(node.Name).Str(" is not a leaf-list: select its entry by ").Quoted(node.Key))
		}
		return checkInstanceKeys(node, keys, step)
	case node.IsLeafList():
		if len(step.keys) > 0 {
			return refError(tb.Str("leaf-list ").Quoted(node.Name).Str(" has no keys: select an entry with [.='value']"))
		}
		if step.pos > 0 {
			return refError(tb.Str("leaf-list ").Quoted(node.Name).Str(" is selected by value: write [.='value']"))
		}
		return nil
	default:
		if len(step.keys) > 0 || step.hasValue || step.pos > 0 {
			return refError(tb.Str("node ").Quoted(node.Name).Str(" is not a list or leaf-list and takes no predicate: remove it"))
		}
		return nil
	}
}

// checkInstanceKeys checks that a list step carries one predicate per key.
func checkInstanceKeys(node *yang.Entry, keys []string, step instanceStep) error {
	var tb textbuf.Buffer
	seen := make(map[string]bool, len(step.keys))
	for _, key := range step.keys {
		name := localName(key.node)
		if !slices.Contains(keys, name) {
			return refError(tb.Str("list ").Quoted(node.Name).Str(" has no key ").Quoted(name).Str(": its keys are ").Quoted(node.Key))
		}
		if seen[name] {
			return refError(tb.Str("key ").Quoted(name).Str(" of list ").Quoted(node.Name).Str(" is given twice: give it once"))
		}
		seen[name] = true
		if err := checkInstanceKeyPrefix(node, name, nodePrefix(key.node)); err != nil {
			return err
		}
	}
	for _, key := range keys {
		if !seen[key] {
			return refError(tb.Str("key ").Quoted(key).Str(" of list ").Quoted(node.Name).Str(" has no predicate: every key needs one, as [prefix:").Str(key).Str("='value']"))
		}
	}
	return nil
}

// RFC 7950 Section 9.13.2: "All node names in an instance-identifier value
// MUST be qualified with explicit namespace prefixes".
//
// checkInstanceKeyPrefix checks the prefix of a key-predicate's node name:
// present, and naming the module of the key leaf.
func checkInstanceKeyPrefix(list *yang.Entry, name, prefix string) error {
	var tb textbuf.Buffer
	keyLeaf := schemaChild(list, name)
	if keyLeaf == nil {
		return refError(tb.Str("list ").Quoted(list.Name).Str(" declares key ").Quoted(name).Str(" but holds no such leaf: this is a defect of the module, report it"))
	}
	if prefix == "" {
		return refError(tb.Str("key ").Quoted(name).Str(" has no module prefix: write it as ").Str(entryPrefix(keyLeaf)).Byte(':').Str(name).Str(" (RFC 7950 Section 9.13.2)"))
	}
	if !entryPrefixed(keyLeaf, prefix) {
		return refError(tb.Str("prefix ").Quoted(prefix).Str(" is not the module of ").Quoted(name).Str(": write ").Str(entryPrefix(keyLeaf)).Byte(':').Str(name))
	}
	return nil
}

// referenceValidationError is the ValidationError for a reference value.
func referenceValidationError(path string, yangType *yang.YangType, value any, message string) error {
	got := valueTypeName(value)
	if str, ok := value.(string); ok {
		got = str
	}
	return &ValidationError{
		Path:     path,
		Type:     ErrTypeType,
		Message:  message,
		Expected: yangType.Name,
		Got:      got,
	}
}
