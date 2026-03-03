// Design: docs/architecture/config/syntax.md — config parsing and loading
//
// Package config provides YANG-to-schema conversion.
package config

import (
	"log/slog"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	bgpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/yang"
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
// This loads only the ze-plugin-conf.yang module.
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
	pluginEntry := loader.GetEntry("ze-plugin-conf")
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
// Includes all internal plugin YANG schemas (GR, hostname, etc.).
// Returns nil if YANG loading fails.
func YANGSchema() *Schema {
	return YANGSchemaWithPlugins(plugin.GetAllInternalPluginYANG())
}

// loadYANGModules creates a resolved YANG loader with all modules.
// Shared by YANGSchemaWithPlugins and YANGValidatorWithPlugins.
func loadYANGModules(pluginYANG map[string]string) *yang.Loader {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil
	}
	// Load module-specific YANG from their packages (module-specific YANG lives with its code)
	if err := loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG); err != nil {
		return nil
	}
	// Load plugin YANG modules
	for name, content := range pluginYANG {
		if err := loader.AddModuleFromText(name, content); err != nil {
			return nil
		}
	}
	if err := loader.Resolve(); err != nil {
		return nil
	}
	return loader
}

// YANGValidatorWithPlugins creates a YANG value validator with all modules loaded.
// Registers custom validators, attaches registry, and checks integrity.
// Used for runtime attribute validation (origin enum, med/local-pref uint32 ranges).
func YANGValidatorWithPlugins(pluginYANG map[string]string) *yang.Validator {
	loader := loadYANGModules(pluginYANG)
	if loader == nil {
		return nil
	}

	v := yang.NewValidator(loader)

	// Register custom validators and attach to validator.
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	v.SetRegistry(reg)

	// Startup integrity check: verify every ze:validate in YANG has a registered function.
	if err := yang.CheckAllValidatorsRegistered(loader, reg); err != nil {
		slog.Error("YANG validator integrity check failed", "error", err)
	}

	return v
}

// YANGSchemaWithPlugins loads YANG with additional plugin modules.
// pluginYANG maps module filename to YANG content.
// Returns nil if YANG loading fails.
func YANGSchemaWithPlugins(pluginYANG map[string]string) *Schema {
	loader := loadYANGModules(pluginYANG)
	if loader == nil {
		return nil
	}

	schema := NewSchema()

	// Load ze-hub module (environment)
	hubEntry := loader.GetEntry("ze-hub-conf")
	if hubEntry != nil {
		for _, name := range sortedKeys(hubEntry.Dir) {
			child := hubEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-plugin-conf module
	pluginEntry := loader.GetEntry("ze-plugin-conf")
	if pluginEntry != nil {
		for _, name := range sortedKeys(pluginEntry.Dir) {
			child := pluginEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-bgp-conf module (bgp, template)
	bgpEntry := loader.GetEntry("ze-bgp-conf")
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
		// leaf-list without ze:syntax extension — accepts single value or bracket list
		if entry.IsLeafList() {
			if entry.Type != nil && entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
				return ValueOrArrayEnum(entry.Type.Enum.Names())
			}
			return ValueOrArray(yangTypeToValueType(entry.Type))
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
	container := Container(fields...)

	// Check for ze:allow-unknown-fields extension
	container.AllowUnknown = hasAllowUnknownExtension(entry)

	// Check for YANG presence statement — enables flag/value/block modes
	container.Presence = hasPresenceStatement(entry)

	return container
}

// hasPresenceStatement checks if a YANG entry has a presence statement.
// goyang stores the presence statement in entry.Extra["presence"] as []any.
func hasPresenceStatement(entry *gyang.Entry) bool {
	if entry.Extra == nil {
		return false
	}
	vals, ok := entry.Extra["presence"]
	return ok && len(vals) > 0
}

// hasAllowUnknownExtension checks if a YANG entry has the ze:allow-unknown-fields extension.
func hasAllowUnknownExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:allow-unknown-fields" || strings.HasSuffix(ext.Keyword, ":allow-unknown-fields") {
			return true
		}
	}
	return false
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
	// Use YANG definition order for list children so inline positional
	// assignment matches the schema author's intent. Fall back to
	// alphabetical when the AST is unavailable (generated entries).
	names := yangChildOrder(entry)
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
	l := List(keyType, fields...)
	l.KeyName = entry.Key
	return l
}

// yangChildOrder returns child names in YANG definition order by
// inspecting the AST sub-statements. Falls back to alphabetical
// order when the AST is unavailable (e.g. programmatically created entries).
// Definition order matters for inline list syntax where values are
// assigned positionally to children fields.
func yangChildOrder(entry *gyang.Entry) []string {
	if entry.Node != nil {
		var names []string
		seen := make(map[string]bool)
		dataKeywords := map[string]bool{
			"leaf": true, "leaf-list": true,
			"container": true, "list": true,
			"choice": true, "anyxml": true,
		}
		for _, sub := range entry.Node.Statement().SubStatements() {
			if !dataKeywords[sub.Keyword] {
				continue
			}
			name := sub.Argument
			if !seen[name] && entry.Dir[name] != nil {
				seen[name] = true
				names = append(names, name)
			}
		}
		// Include any Dir entries not in the AST (from uses/augment).
		for _, name := range sortedKeys(entry.Dir) {
			if !seen[name] {
				names = append(names, name)
			}
		}
		return names
	}
	return sortedKeys(entry.Dir)
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

// LoadYANGModule loads a YANG module entry into a schema.
// This is used by external packages to create schemas from their own YANG modules.
func LoadYANGModule(schema *Schema, entry *gyang.Entry) {
	if entry == nil {
		return
	}
	for _, name := range sortedKeys(entry.Dir) {
		child := entry.Dir[name]
		node := yangToNode(child, name)
		if node != nil {
			schema.Define(name, node)
		}
	}
}
