// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: yang_schema.go — YANG-to-schema conversion
// Related: validators.go — custom schema validators
//
// Package config implements schema-driven configuration parsing.
//
// It supports multiple input formats (ExaBGP, set-style, KDL) that all
// validate against the same schema and produce the same Config output.
package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// ValueType represents the type of a leaf value.
type ValueType int

const (
	TypeString ValueType = iota
	TypeBool
	TypeUint16
	TypeUint32
	TypeIPv4
	TypeIPv6
	TypeIP       // IPv4 or IPv6
	TypePrefix   // CIDR prefix
	TypeDuration // time.Duration (e.g., "100ms", "5s")
	TypeInt      // signed integer
)

func (t ValueType) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	case TypeUint16:
		return "uint16"
	case TypeUint32:
		return "uint32"
	case TypeIPv4:
		return "ipv4"
	case TypeIPv6:
		return "ipv6"
	case TypeIP:
		return "ip"
	case TypePrefix:
		return "prefix"
	case TypeDuration:
		return "duration"
	case TypeInt:
		return "int"
	default:
		return "unknown"
	}
}

// DisplayMode controls how sensitive values are shown in output.
type DisplayMode int

const (
	// DisplayEncode shows sensitive values as $9$-encoded (default).
	DisplayEncode DisplayMode = iota
	// DisplayStrip replaces sensitive values with /* SECRET-DATA */.
	DisplayStrip
	// DisplayPlain shows sensitive values as plaintext (internal use).
	DisplayPlain
)

// SecretDataPlaceholder is the replacement text for stripped sensitive values.
const SecretDataPlaceholder = "/* SECRET-DATA */" //nolint:gosec // not a credential

// SensitiveKeys walks a schema tree and returns all leaf names marked ze:sensitive.
func SensitiveKeys(schema *Schema) map[string]bool {
	keys := make(map[string]bool)
	collectSensitiveKeys(schema.root, keys)
	return keys
}

func collectSensitiveKeys(node Node, keys map[string]bool) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}
	for _, name := range cp.Children() {
		child := cp.Get(name)
		if leaf, ok := child.(*LeafNode); ok && leaf.Sensitive {
			keys[name] = true
		}
		collectSensitiveKeys(child, keys)
	}
}

// NodeKind represents the kind of schema node.
type NodeKind int

const (
	NodeLeaf NodeKind = iota
	NodeContainer
	NodeList
	NodeFreeform   // accepts "word word;" entries as key->true
	NodeFlex       // can be flag (;), value (word;), or block ({})
	NodeInlineList // list that supports inline: "route PREFIX attr val;"
)

// Node is the interface for all schema nodes.
type Node interface {
	Kind() NodeKind
}

// LeafNode represents a terminal value.
type LeafNode struct {
	Type      ValueType
	Default   string
	Sensitive bool // ze:sensitive — value is a password/key, masked in display
}

func (n *LeafNode) Kind() NodeKind { return NodeLeaf }

// ContainerNode represents a group of child nodes.
type ContainerNode struct {
	children     map[string]Node
	order        []string // preserve definition order
	AllowUnknown bool     // accept arbitrary key-value pairs (ze:allow-unknown-fields)
	Presence     bool     // YANG presence container: accepts flag (;), value (word;), or block ({})
}

func (n *ContainerNode) Kind() NodeKind { return NodeContainer }

// Has returns true if the container has a child with the given name.
func (n *ContainerNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *ContainerNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *ContainerNode) Children() []string {
	return n.order
}

// ListNode represents a keyed collection of containers.
type ListNode struct {
	KeyType  ValueType
	KeyName  string // YANG key name (empty = keyless list like update)
	children map[string]Node
	order    []string
}

func (n *ListNode) Kind() NodeKind { return NodeList }

// Has returns true if the list entry has a child with the given name.
func (n *ListNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *ListNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *ListNode) Children() []string {
	return n.order
}

// Schema holds the configuration schema definition.
type Schema struct {
	root *ContainerNode
}

// NewSchema creates a new empty schema.
func NewSchema() *Schema {
	return &Schema{
		root: &ContainerNode{
			children: make(map[string]Node),
		},
	}
}

// Define adds a top-level node to the schema.
// If a node with the same name already exists, it is replaced
// without adding a duplicate to the order.
func (s *Schema) Define(name string, node Node) {
	existing, exists := s.root.children[name]
	if exists {
		// Merge containers: add new children to existing container.
		// Multiple YANG modules may define children under the same top-level container
		// (e.g., ze-system-conf and ze-ssh-conf both contribute to "system").
		if ec, ok := existing.(*ContainerNode); ok {
			if nc, ok := node.(*ContainerNode); ok {
				for _, childName := range nc.order {
					if _, dup := ec.children[childName]; !dup {
						ec.children[childName] = nc.children[childName]
						ec.order = append(ec.order, childName)
					}
				}
				return
			}
		}
	}
	s.root.children[name] = node
	if !exists {
		s.root.order = append(s.root.order, name)
	}
}

// Has returns true if the schema has a top-level node with the given name.
func (s *Schema) Has(name string) bool {
	return s.root.Has(name)
}

// Get returns a top-level node by name.
func (s *Schema) Get(name string) Node {
	return s.root.Get(name)
}

// Children returns all top-level node names in definition order.
func (s *Schema) Children() []string {
	return s.root.Children()
}

// Lookup finds a node by dot-separated path.
func (s *Schema) Lookup(path string) (Node, error) {
	parts := strings.Split(path, ".")
	var current Node = s.root

	for _, part := range parts {
		switch n := current.(type) {
		case *ContainerNode:
			child := n.Get(part)
			if child == nil {
				return nil, fmt.Errorf("unknown path element: %s", part)
			}
			current = child
		case *ListNode:
			child := n.Get(part)
			if child == nil {
				return nil, fmt.Errorf("unknown path element: %s", part)
			}
			current = child
		case *LeafNode:
			return nil, fmt.Errorf("cannot traverse into leaf at: %s", part)
		default:
			return nil, fmt.Errorf("unknown node type at: %s", part)
		}
	}

	return current, nil
}

// ExtendCapability adds a capability sub-block to the schema at runtime.
// Used by plugins to declare their config schema extensions.
// The path is "peer.capability" by default - the name is the capability name.
func (s *Schema) ExtendCapability(name string, fields ...FieldDef) error {
	// Navigate to bgp.peer.capability container
	bgpNode := s.Get("bgp")
	if bgpNode == nil {
		return fmt.Errorf("bgp node not found in schema")
	}

	bgpContainer, ok := bgpNode.(*ContainerNode)
	if !ok {
		return fmt.Errorf("bgp is not a ContainerNode")
	}

	peerNode := bgpContainer.Get("peer")
	if peerNode == nil {
		return fmt.Errorf("peer node not found in bgp schema")
	}

	listNode, ok := peerNode.(*ListNode)
	if !ok {
		return fmt.Errorf("peer is not a ListNode")
	}

	capNode := listNode.Get("capability")
	if capNode == nil {
		return fmt.Errorf("capability node not found in peer schema")
	}

	container, ok := capNode.(*ContainerNode)
	if !ok {
		return fmt.Errorf("capability is not a ContainerNode")
	}

	// Add the new capability as a Flex node
	container.children[name] = Flex(fields...)
	container.order = append(container.order, name)

	// Also add to template.group and template.match if they exist
	s.extendTemplateCapability(name, fields)

	return nil
}

// extendTemplateCapability extends capability schema in templates.
func (s *Schema) extendTemplateCapability(name string, fields []FieldDef) {
	templateNode := s.Get("template")
	if templateNode == nil {
		return
	}

	templateContainer, ok := templateNode.(*ContainerNode)
	if !ok {
		return
	}

	// Extend template.bgp.peer
	bgpNode := templateContainer.Get("bgp")
	if bgpNode == nil {
		return
	}

	bgpContainer, ok := bgpNode.(*ContainerNode)
	if !ok {
		return
	}

	peerNode := bgpContainer.Get("peer")
	if peerNode == nil {
		return
	}

	if peerList, ok := peerNode.(*ListNode); ok {
		if capNode := peerList.Get("capability"); capNode != nil {
			if capContainer, ok := capNode.(*ContainerNode); ok {
				capContainer.children[name] = Flex(fields...)
				capContainer.order = append(capContainer.order, name)
			}
		}
	}

	// Also extend legacy template.group and template.match if they exist
	if groupNode := templateContainer.Get("group"); groupNode != nil {
		if groupList, ok := groupNode.(*ListNode); ok {
			if capNode := groupList.Get("capability"); capNode != nil {
				if capContainer, ok := capNode.(*ContainerNode); ok {
					capContainer.children[name] = Flex(fields...)
					capContainer.order = append(capContainer.order, name)
				}
			}
		}
	}

	if matchNode := templateContainer.Get("match"); matchNode != nil {
		if matchList, ok := matchNode.(*ListNode); ok {
			if capNode := matchList.Get("capability"); capNode != nil {
				if capContainer, ok := capNode.(*ContainerNode); ok {
					capContainer.children[name] = Flex(fields...)
					capContainer.order = append(capContainer.order, name)
				}
			}
		}
	}
}

// Field is a helper for defining container/list children.
type FieldDef struct {
	Name string
	Node Node
}

// Field creates a field definition.
func Field(name string, node Node) FieldDef {
	return FieldDef{Name: name, Node: node}
}

// Leaf creates a leaf node with the given type.
func Leaf(typ ValueType) *LeafNode {
	return &LeafNode{Type: typ}
}

// LeafWithDefault creates a leaf node with a default value.
func LeafWithDefault(typ ValueType, def string) *LeafNode {
	return &LeafNode{Type: typ, Default: def}
}

// Container creates a container node with the given children.
func Container(fields ...FieldDef) *ContainerNode {
	c := &ContainerNode{
		children: make(map[string]Node),
	}
	for _, f := range fields {
		c.children[f.Name] = f.Node
		c.order = append(c.order, f.Name)
	}
	return c
}

// List creates a list node with the given key type and children.
func List(keyType ValueType, fields ...FieldDef) *ListNode {
	l := &ListNode{
		KeyType:  keyType,
		children: make(map[string]Node),
	}
	for _, f := range fields {
		l.children[f.Name] = f.Node
		l.order = append(l.order, f.Name)
	}
	return l
}

// FreeformNode accepts "word word ...;" entries, storing as "word word" -> true.
// Used for family { ipv4/unicast; ipv6/unicast; }.
type FreeformNode struct{}

func (n *FreeformNode) Kind() NodeKind { return NodeFreeform }

// Freeform creates a freeform node.
func Freeform() *FreeformNode {
	return &FreeformNode{}
}

// MultiLeafNode accepts multiple words until semicolon: "word word word;".
type MultiLeafNode struct {
	Type ValueType
}

func (n *MultiLeafNode) Kind() NodeKind { return NodeLeaf }

// MultiLeaf creates a multi-word leaf node.
func MultiLeaf(typ ValueType) *MultiLeafNode {
	return &MultiLeafNode{Type: typ}
}

// BracketLeafListNode accepts [ item item ... ] syntax for leaf-list.
// Maps to YANG leaf-list with ze:syntax "bracket".
type BracketLeafListNode struct {
	Type ValueType
}

func (n *BracketLeafListNode) Kind() NodeKind { return NodeLeaf }

// BracketLeafList creates a bracket-syntax leaf-list node.
func BracketLeafList(typ ValueType) *BracketLeafListNode {
	return &BracketLeafListNode{Type: typ}
}

// ArrayLeaf is an alias for BracketLeafList, kept for legacy schema compatibility.
// Use BracketLeafList for new code.
func ArrayLeaf(typ ValueType) *BracketLeafListNode {
	return BracketLeafList(typ)
}

// ValueOrArrayNode accepts either "value;" or "[ item item ... ];" syntax.
// Stores result as space-separated string in both cases.
type ValueOrArrayNode struct {
	Type        ValueType
	ValidValues []string // If non-nil, each item must be one of these values (YANG enum)
}

func (n *ValueOrArrayNode) Kind() NodeKind { return NodeLeaf }

// ValueOrArray creates a node that accepts either a single value or bracketed list.
func ValueOrArray(typ ValueType) *ValueOrArrayNode {
	return &ValueOrArrayNode{Type: typ}
}

// ValueOrArrayEnum creates a node that accepts either a single value or bracketed list,
// with validation against a fixed set of enum values.
func ValueOrArrayEnum(validValues []string) *ValueOrArrayNode {
	return &ValueOrArrayNode{Type: TypeString, ValidValues: validValues}
}

// FlexNode can be a flag (;), value (word;), or block ({}).
// Used for capabilities like graceful-restart which support all three forms.
type FlexNode struct {
	children map[string]Node
	order    []string
}

func (n *FlexNode) Kind() NodeKind { return NodeFlex }

// Has returns true if the flex node has a child with the given name.
func (n *FlexNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *FlexNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *FlexNode) Children() []string {
	return n.order
}

// Flex creates a flex node with optional children for block mode.
func Flex(fields ...FieldDef) *FlexNode {
	f := &FlexNode{
		children: make(map[string]Node),
	}
	for _, field := range fields {
		f.children[field.Name] = field.Node
		f.order = append(f.order, field.Name)
	}
	return f
}

// InlineListNode is a list that supports inline attribute syntax.
// e.g., "route 10.0.0.0/8 next-hop 1.1.1.1;" or "route 10.0.0.0/8 { next-hop 1.1.1.1; }".
type InlineListNode struct {
	KeyType  ValueType
	children map[string]Node
	order    []string
}

func (n *InlineListNode) Kind() NodeKind { return NodeInlineList }

// Has returns true if the list entry has a child with the given name.
func (n *InlineListNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *InlineListNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *InlineListNode) Children() []string {
	return n.order
}

// InlineList creates an inline list node.
func InlineList(keyType ValueType, fields ...FieldDef) *InlineListNode {
	l := &InlineListNode{
		KeyType:  keyType,
		children: make(map[string]Node),
	}
	for _, f := range fields {
		l.children[f.Name] = f.Node
		l.order = append(l.order, f.Name)
	}
	return l
}

// ValidateValue validates a string value against a type.
func ValidateValue(typ ValueType, value string) error {
	switch typ {
	case TypeString:
		return nil

	case TypeBool:
		if value != configTrue && value != "false" && value != "enable" && value != "disable" && //nolint:goconst // String literals for validation
			value != "require" && value != "refuse" {
			return fmt.Errorf("invalid bool: %q (expected true/false/enable/disable/require/refuse)", value)
		}
		return nil

	case TypeUint16:
		n, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("invalid uint16: %q", value)
		}
		if n > 65535 {
			return fmt.Errorf("uint16 overflow: %s", value)
		}
		return nil

	case TypeUint32:
		_, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid uint32: %q", value)
		}
		return nil

	case TypeIPv4:
		addr, err := netip.ParseAddr(value)
		if err != nil || !addr.Is4() {
			return fmt.Errorf("invalid IPv4: %q", value)
		}
		return nil

	case TypeIPv6:
		addr, err := netip.ParseAddr(value)
		if err != nil || !addr.Is6() {
			return fmt.Errorf("invalid IPv6: %q", value)
		}
		return nil

	case TypeIP:
		_, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("invalid IP: %q", value)
		}
		return nil

	case TypePrefix:
		// Try as prefix first
		_, err := netip.ParsePrefix(value)
		if err != nil {
			// Try as plain IP (host route /32 or /128)
			_, err2 := netip.ParseAddr(value)
			if err2 != nil {
				return fmt.Errorf("invalid prefix: %q", value)
			}
		}
		return nil

	case TypeDuration:
		// Accept "0" as valid (means 0 duration)
		if value == "0" {
			return nil
		}
		// Validate Go duration format (e.g., "100ms", "5s", "1m30s")
		_, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration: %q (expected format like 100ms, 5s)", value)
		}
		return nil

	case TypeInt:
		_, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int: %q", value)
		}
		return nil

	default:
		return fmt.Errorf("unknown type: %v", typ)
	}
}

// parseDuration parses a duration string like "100ms", "5s", "0.5s".
// Returns the duration in nanoseconds for validation purposes.
func parseDuration(s string) (int64, error) {
	// Handle simple cases manually to avoid importing time in schema
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Find where the number ends and unit begins
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("no number in duration")
	}

	numStr := s[:i]
	unit := s[i:]

	// Parse number (can be float)
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert based on unit
	var multiplier float64
	switch unit {
	case "", "ns":
		multiplier = 1
	case "us", "µs":
		multiplier = 1000
	case "ms":
		multiplier = 1000000
	case "s":
		multiplier = 1000000000
	case "m":
		multiplier = 60 * 1000000000
	case "h":
		multiplier = 3600 * 1000000000
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(num * multiplier), nil
}

// NormalizeBool converts enable/disable to true/false.
func NormalizeBool(value string) string {
	switch value {
	case configEnable:
		return configTrue
	case configDisable:
		return configFalse
	default:
		return value
	}
}
