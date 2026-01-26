// Package config provides YANG-to-schema conversion.
package config

import (
	"sort"
	"strings"

	hubschema "codeberg.org/thomas-mangin/ze/internal/hub/schema"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/yang"
	gyang "github.com/openconfig/goyang/pkg/yang"
)

// sortedKeys returns the keys of a map[string]*gyang.Entry in sorted order.
func sortedKeys(m map[string]*gyang.Entry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// PluginOnlySchema returns a schema that only accepts plugin blocks.
// Used for two-phase config parsing: first extract plugins, then parse full config.
// This loads only the ze-plugin.yang module.
func PluginOnlySchema() *Schema {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil
	}
	if err := loader.Resolve(); err != nil {
		return nil
	}

	schema := NewSchema()

	// Load only ze-plugin module
	pluginEntry := loader.GetEntry("ze-plugin")
	if pluginEntry != nil {
		for _, name := range sortedKeys(pluginEntry.Dir) {
			child := pluginEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	return schema
}

// YANGSchema loads YANG and creates a schema from it.
// Returns nil if YANG loading fails.
func YANGSchema() *Schema {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil
	}
	// Load module-specific YANG from their packages (module-specific YANG lives with its code)
	if err := loader.AddModuleFromText("ze-hub.yang", hubschema.ZeHubYANG); err != nil {
		return nil
	}
	if err := loader.AddModuleFromText("ze-bgp.yang", bgpschema.ZeBGPYANG); err != nil {
		return nil
	}
	if err := loader.Resolve(); err != nil {
		return nil
	}

	schema := NewSchema()

	// Load ze-hub module (environment)
	hubEntry := loader.GetEntry("ze-hub")
	if hubEntry != nil {
		for _, name := range sortedKeys(hubEntry.Dir) {
			child := hubEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-plugin module
	pluginEntry := loader.GetEntry("ze-plugin")
	if pluginEntry != nil {
		for _, name := range sortedKeys(pluginEntry.Dir) {
			child := pluginEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-bgp module (bgp, template)
	bgpEntry := loader.GetEntry("ze-bgp")
	if bgpEntry != nil {
		for _, name := range sortedKeys(bgpEntry.Dir) {
			child := bgpEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	return schema
}

// yangToNode converts a YANG entry to a schema node.
func yangToNode(entry *gyang.Entry, path string) Node {
	if entry == nil {
		return nil
	}

	// Check for ze:syntax extension in YANG entry
	syntax := getSyntaxExtension(entry)

	switch syntax {
	case "flex":
		return yangToFlex(entry, path)
	case "freeform":
		return Freeform()
	case "family-block":
		return FamilyBlock()
	case "inline-list":
		keyType := getKeyTypeExtension(entry)
		return yangToInlineListWithKey(entry, path, keyType)
	case "multi-leaf":
		return MultiLeaf(yangTypeToValueType(entry.Type))
	case "bracket":
		return BracketLeafList(yangTypeToValueType(entry.Type))
	case "value-or-array":
		return ValueOrArray(yangTypeToValueType(entry.Type))
	}

	// Standard YANG node types
	//nolint:exhaustive // Only handle types relevant to config schema
	switch entry.Kind {
	case gyang.LeafEntry:
		// leaf-list without ze:syntax extension - treat as multi-leaf (space-separated)
		if entry.IsLeafList() {
			return MultiLeaf(yangTypeToValueType(entry.Type))
		}
		return yangToLeaf(entry)
	case gyang.DirectoryEntry:
		if entry.IsList() {
			return yangToList(entry, path)
		}
		return yangToContainer(entry, path)
	default:
		return nil
	}
}

// getSyntaxExtension reads the ze:syntax extension from a YANG entry.
// Returns empty string if no syntax extension is present.
func getSyntaxExtension(entry *gyang.Entry) string {
	for _, ext := range entry.Exts {
		// Extension keyword is "ze:syntax" (or just "syntax" with ze prefix)
		if ext.Keyword == "ze:syntax" || strings.HasSuffix(ext.Keyword, ":syntax") {
			return ext.Argument
		}
	}
	return ""
}

// getKeyTypeExtension reads the ze:key-type extension from a YANG entry.
// Returns "prefix" as default for inline-list nodes.
func getKeyTypeExtension(entry *gyang.Entry) ValueType {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:key-type" || strings.HasSuffix(ext.Keyword, ":key-type") {
			switch ext.Argument {
			case "string":
				return TypeString
			case "prefix":
				return TypePrefix
			case "ip":
				return TypeIP
			case "uint32":
				return TypeUint32
			}
		}
	}
	return TypePrefix // Default for route-like lists
}

// yangToLeaf converts YANG leaf to LeafNode.
func yangToLeaf(entry *gyang.Entry) *LeafNode {
	typ := yangTypeToValueType(entry.Type)
	if len(entry.Default) > 0 {
		return LeafWithDefault(typ, entry.Default[0])
	}
	return Leaf(typ)
}

// yangToContainer converts YANG container to ContainerNode.
func yangToContainer(entry *gyang.Entry, path string) *ContainerNode {
	fields := make([]FieldDef, 0, len(entry.Dir))
	// Sort keys for deterministic field order
	names := sortedKeys(entry.Dir)
	for _, name := range names {
		child := entry.Dir[name]
		childPath := path + "." + name
		node := yangToNode(child, childPath)
		if node != nil {
			fields = append(fields, Field(name, node))
		}
	}
	return Container(fields...)
}

// yangToList converts YANG list to ListNode.
func yangToList(entry *gyang.Entry, path string) *ListNode {
	// Determine key type from first key leaf
	keyType := TypeString
	if entry.Key != "" && entry.Dir != nil {
		if keyEntry, ok := entry.Dir[entry.Key]; ok {
			keyType = yangTypeToValueType(keyEntry.Type)
		}
	}

	fields := make([]FieldDef, 0, len(entry.Dir))
	// Sort keys for deterministic field order
	names := sortedKeys(entry.Dir)
	for _, name := range names {
		if name == entry.Key {
			continue // Key is not a child field
		}
		child := entry.Dir[name]
		childPath := path + "." + name
		node := yangToNode(child, childPath)
		if node != nil {
			fields = append(fields, Field(name, node))
		}
	}
	return List(keyType, fields...)
}

// yangToFlex converts YANG entry to FlexNode.
func yangToFlex(entry *gyang.Entry, path string) *FlexNode {
	if entry.Dir == nil {
		return Flex()
	}

	fields := make([]FieldDef, 0, len(entry.Dir))
	// Sort keys for deterministic field order
	names := sortedKeys(entry.Dir)
	for _, name := range names {
		child := entry.Dir[name]
		childPath := path + "." + name
		node := yangToNode(child, childPath)
		if node != nil {
			fields = append(fields, Field(name, node))
		}
	}
	return Flex(fields...)
}

// yangToInlineListWithKey converts YANG entry to InlineListNode with specified key type.
func yangToInlineListWithKey(entry *gyang.Entry, path string, keyType ValueType) *InlineListNode {
	fields := make([]FieldDef, 0, len(entry.Dir))
	// Sort keys for deterministic field order
	names := sortedKeys(entry.Dir)
	for _, name := range names {
		if name == entry.Key {
			continue
		}
		child := entry.Dir[name]
		childPath := path + "." + name
		node := yangToNode(child, childPath)
		if node != nil {
			fields = append(fields, Field(name, node))
		}
	}

	return InlineList(keyType, fields...)
}

// yangTypeToValueType converts YANG type to config ValueType.
func yangTypeToValueType(t *gyang.YangType) ValueType {
	if t == nil {
		return TypeString
	}

	// First check type name for custom types (before checking Kind)
	// This handles typedefs like zt:ipv4-address
	name := t.Name
	if isIPv4Type(name) {
		return TypeIPv4
	}
	if isIPv6Type(name) {
		return TypeIPv6
	}
	if isIPType(name) {
		return TypeIP
	}
	if isASNType(name) {
		return TypeUint32
	}

	//nolint:exhaustive // default handles all other types
	switch t.Kind {
	case gyang.Ystring:
		return TypeString
	case gyang.Ybool:
		return TypeBool
	case gyang.Yuint8, gyang.Yuint16:
		return TypeUint16
	case gyang.Yuint32, gyang.Yuint64:
		return TypeUint32
	case gyang.Yint8, gyang.Yint16, gyang.Yint32, gyang.Yint64:
		return TypeInt
	case gyang.Yenum:
		return TypeString // Enums stored as strings
	default:
		return TypeString
	}
}

func isIPv4Type(name string) bool {
	return name == "ipv4-address" || name == "zt:ipv4-address"
}

func isIPv6Type(name string) bool {
	return name == "ipv6-address" || name == "zt:ipv6-address"
}

func isIPType(name string) bool {
	return name == "ip-address" || name == "zt:ip-address"
}

func isASNType(name string) bool {
	return name == "asn" || name == "zt:asn"
}
