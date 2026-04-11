// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: schema.go — schema types and validation
// Related: prune.go — inactive node pruning (uses InactiveLeafName)
//
// Package config provides YANG-to-schema conversion.
package config

import (
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
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

// namedChild pairs a child entry with its effective name as it should appear
// under the parent container. Used by flattenChildren to expand choice/case
// subtrees into their data nodes.
type namedChild struct {
	name  string
	entry *gyang.Entry
}

// flattenChildren returns the effective children of a YANG entry, descending
// through ChoiceEntry/CaseEntry layers. The YANG choice/case construct is
// "transparent" at the data layer: the inner data nodes (leaves, containers,
// lists) appear as if they were direct children of the parent. The base
// yang_schema.go walker only handles LeafEntry and DirectoryEntry, so without
// this helper any choice subtree would be silently dropped from the schema.
//
// The case wrapper layer is bypassed entirely: the inner data node's name
// is used directly, matching the YANG source. Recursion handles nested
// choices (e.g. a `choice local` inside a per-encap case container).
func flattenChildren(entry *gyang.Entry) []namedChild {
	out := make([]namedChild, 0, len(entry.Dir))
	for _, name := range sortedKeys(entry.Dir) {
		child := entry.Dir[name]
		if child == nil {
			continue
		}
		if child.IsChoice() {
			out = append(out, flattenChoiceCases(child)...)
			continue
		}
		out = append(out, namedChild{name: name, entry: child})
	}
	return out
}

// flattenChoiceCases walks a ChoiceEntry's case branches and yields the data
// nodes contained within. Direct children of a case are yielded as-is.
// Nested choices inside a case are recursively flattened.
func flattenChoiceCases(choice *gyang.Entry) []namedChild {
	out := make([]namedChild, 0, len(choice.Dir))
	for _, caseName := range sortedKeys(choice.Dir) {
		caseEntry := choice.Dir[caseName]
		if caseEntry == nil {
			continue
		}
		if !caseEntry.IsCase() {
			out = append(out, namedChild{name: caseName, entry: caseEntry})
			continue
		}
		for _, name := range sortedKeys(caseEntry.Dir) {
			inner := caseEntry.Dir[name]
			if inner == nil {
				continue
			}
			if inner.IsChoice() {
				out = append(out, flattenChoiceCases(inner)...)
				continue
			}
			out = append(out, namedChild{name: name, entry: inner})
		}
	}
	return out
}

// PluginOnlySchema returns a schema that only accepts plugin blocks.
// Used for two-phase config parsing: first extract plugins, then parse full config.
// This loads only the ze-plugin-conf.yang module.
func PluginOnlySchema() (*Schema, error) {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("load embedded YANG: %w", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		return nil, fmt.Errorf("load registered YANG: %w", err)
	}
	if err := loader.Resolve(); err != nil {
		return nil, fmt.Errorf("resolve YANG modules: %w", err)
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

	return schema, nil
}

// YANGSchema loads YANG and creates a schema from it.
// Internal plugin schemas are loaded via init()-based registration (LoadRegistered).
func YANGSchema() (*Schema, error) {
	return YANGSchemaWithPlugins(nil)
}

// loadYANGModules creates a resolved YANG loader with all modules.
// Shared by YANGSchemaWithPlugins and YANGValidatorWithPlugins.
func loadYANGModules(pluginYANG map[string]string) (*yang.Loader, error) {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("load embedded YANG: %w", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		return nil, fmt.Errorf("load registered YANG: %w", err)
	}
	// Load external plugin YANG modules not covered by init()-based registration.
	// Skip duplicate modules — they may already be loaded via LoadRegistered().
	for name, content := range pluginYANG {
		if err := loader.AddModuleFromText(name, content); err != nil {
			if !strings.Contains(err.Error(), "duplicate module") {
				return nil, fmt.Errorf("add plugin YANG module %q: %w", name, err)
			}
		}
	}
	if err := loader.Resolve(); err != nil {
		return nil, fmt.Errorf("resolve YANG modules: %w", err)
	}
	return loader, nil
}

// YANGValidatorWithPlugins creates a YANG value validator with all modules loaded.
// Registers custom validators, attaches registry, and checks integrity.
// Used for runtime attribute validation (origin enum, med/local-pref uint32 ranges).
func YANGValidatorWithPlugins(pluginYANG map[string]string) (*yang.Validator, error) {
	loader, err := loadYANGModules(pluginYANG)
	if err != nil {
		return nil, err
	}

	v := yang.NewValidator(loader)

	// Register custom validators and attach to validator.
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompleteFns()
	v.SetRegistry(reg)

	// Startup integrity check: verify every ze:validate in YANG has a registered function.
	if err := yang.CheckAllValidatorsRegistered(loader, reg); err != nil {
		slog.Error("YANG validator integrity check failed", "error", err)
	}

	return v, nil
}

// YANGSchemaWithPlugins loads YANG with additional plugin modules.
// pluginYANG maps module filename to YANG content.
func YANGSchemaWithPlugins(pluginYANG map[string]string) (*Schema, error) {
	loader, err := loadYANGModules(pluginYANG)
	if err != nil {
		return nil, err
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

	// Load ze-system-conf module (system identity and archive)
	systemEntry := loader.GetEntry("ze-system-conf")
	if systemEntry != nil {
		for _, name := range sortedKeys(systemEntry.Dir) {
			child := systemEntry.Dir[name]
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

	// Load ze-authz-conf module (system > authorization)
	authzEntry := loader.GetEntry("ze-authz-conf")
	if authzEntry != nil {
		for _, name := range sortedKeys(authzEntry.Dir) {
			child := authzEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-ssh-conf module (system > ssh)
	sshEntry := loader.GetEntry("ze-ssh-conf")
	if sshEntry != nil {
		for _, name := range sortedKeys(sshEntry.Dir) {
			child := sshEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-web-conf module (environment > web)
	webEntry := loader.GetEntry("ze-web-conf")
	if webEntry != nil {
		for _, name := range sortedKeys(webEntry.Dir) {
			child := webEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-mcp-conf module (environment > mcp)
	mcpEntry := loader.GetEntry("ze-mcp-conf")
	if mcpEntry != nil {
		for _, name := range sortedKeys(mcpEntry.Dir) {
			child := mcpEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-api-conf module (environment > api-server)
	apiEntry := loader.GetEntry("ze-api-conf")
	if apiEntry != nil {
		for _, name := range sortedKeys(apiEntry.Dir) {
			child := apiEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-dns-conf module (environment > dns)
	dnsEntry := loader.GetEntry("ze-dns-conf")
	if dnsEntry != nil {
		for _, name := range sortedKeys(dnsEntry.Dir) {
			child := dnsEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-lg-conf module (environment > looking-glass)
	lgEntry := loader.GetEntry("ze-lg-conf")
	if lgEntry != nil {
		for _, name := range sortedKeys(lgEntry.Dir) {
			child := lgEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-telemetry-conf module (telemetry)
	telemetryEntry := loader.GetEntry("ze-telemetry-conf")
	if telemetryEntry != nil {
		for _, name := range sortedKeys(telemetryEntry.Dir) {
			child := telemetryEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-iface-conf module (interface)
	ifaceEntry := loader.GetEntry("ze-iface-conf")
	if ifaceEntry != nil {
		for _, name := range sortedKeys(ifaceEntry.Dir) {
			child := ifaceEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	// Load ze-bfd-conf module (bfd)
	bfdEntry := loader.GetEntry("ze-bfd-conf")
	if bfdEntry != nil {
		for _, name := range sortedKeys(bfdEntry.Dir) {
			child := bfdEntry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	return schema, nil
}

// yangToNode converts a YANG entry to a schema node.
func yangToNode(entry *gyang.Entry, path string) Node {
	if entry == nil {
		return nil
	}

	// Skip nodes restricted to a different OS via ze:os extension.
	if requiredOS := getOSExtension(entry); requiredOS != "" && requiredOS != runtime.GOOS {
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

// hasDisplayKeyExtension checks if a YANG entry has the ze:display-key extension.
func hasDisplayKeyExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:display-key" || strings.HasSuffix(ext.Keyword, ":display-key") {
			return true
		}
	}
	return false
}

// getDecorateExtension reads the ze:decorate extension argument from a YANG entry.
// Returns empty string if no decorate extension is present.
func getDecorateExtension(entry *gyang.Entry) string {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:decorate" || strings.HasSuffix(ext.Keyword, ":decorate") {
			return ext.Argument
		}
	}
	return ""
}

// getOSExtension reads the ze:os extension from a YANG entry.
// Returns the required GOOS value, or empty string if no OS restriction.
func getOSExtension(entry *gyang.Entry) string {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:os" || strings.HasSuffix(ext.Keyword, ":os") {
			return ext.Argument
		}
	}
	return ""
}

// hasSensitiveExtension checks if a YANG entry has the ze:sensitive extension.
func hasSensitiveExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:sensitive" || strings.HasSuffix(ext.Keyword, ":sensitive") {
			return true
		}
	}
	return false
}

// hasHiddenExtension checks if a YANG entry has the ze:hidden extension with argument "true".
func hasHiddenExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:hidden" || strings.HasSuffix(ext.Keyword, ":hidden") {
			return ext.Argument == "true"
		}
	}
	return false
}

// hasEphemeralExtension checks if a YANG entry has the ze:ephemeral extension.
func hasEphemeralExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:ephemeral" || strings.HasSuffix(ext.Keyword, ":ephemeral") {
			return true
		}
	}
	return false
}

// yangToLeaf converts YANG leaf to LeafNode.
func yangToLeaf(entry *gyang.Entry) *LeafNode {
	typ := yangTypeToValueType(entry.Type)
	node := Leaf(typ)
	if len(entry.Default) > 0 {
		node.Default = entry.Default[0]
	}
	node.Sensitive = hasSensitiveExtension(entry)
	node.Hidden = hasHiddenExtension(entry)
	node.Ephemeral = hasEphemeralExtension(entry)
	node.Decorate = getDecorateExtension(entry)
	node.Description = entry.Description
	if entry.Type != nil && entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
		node.Enums = entry.Type.Enum.Names()
	}
	return node
}

// InactiveLeafName is the name of the auto-injected inactive boolean leaf.
// Present on every container and list node in the schema.
const InactiveLeafName = "inactive"

// inactiveLeaf creates the auto-injected inactive boolean leaf.
func inactiveLeaf() *LeafNode {
	return &LeafNode{Type: TypeBool, Default: "false", Description: "Deactivate this configuration block"}
}

// hasStructuralChildren reports whether a ListNode has at least one
// non-leaf child (container or list). Positional lists with only leaf
// children are compact data entries that don't benefit from inactive.
func hasStructuralChildren(l *ListNode) bool {
	for _, name := range l.Children() {
		switch l.Get(name).(type) {
		case *ContainerNode, *ListNode:
			return true
		}
	}
	return false
}

// yangToContainer converts YANG container to ContainerNode.
func yangToContainer(entry *gyang.Entry, path string) *ContainerNode {
	children := flattenChildren(entry)
	fields := make([]FieldDef, 0, len(children)+1)
	for _, c := range children {
		childPath := AppendPath(path, c.name)
		node := yangToNode(c.entry, childPath)
		if node != nil {
			fields = append(fields, Field(c.name, node))
		}
	}
	container := Container(fields...)

	// Auto-inject inactive leaf if not already present (from YANG).
	if !container.Has(InactiveLeafName) {
		container.children[InactiveLeafName] = inactiveLeaf()
		container.order = append(container.order, InactiveLeafName)
	}

	// Check for ze:allow-unknown-fields extension
	container.AllowUnknown = hasAllowUnknownExtension(entry)

	// Check for ze:hidden and ze:ephemeral extensions
	container.Hidden = hasHiddenExtension(entry)
	container.Ephemeral = hasEphemeralExtension(entry)

	// Check for YANG presence statement — enables flag/value/block modes
	container.Presence = hasPresenceStatement(entry)
	container.Description = entry.Description

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

	fields := make([]FieldDef, 0, len(entry.Dir)+1)
	// Use YANG definition order for list children so inline positional
	// assignment matches the schema author's intent. Fall back to
	// alphabetical when the AST is unavailable (generated entries).
	names := yangChildOrder(entry)
	for _, name := range names {
		if name == entry.Key {
			continue // Key is not a child field
		}
		child := entry.Dir[name]
		childPath := AppendPath(path, name)
		node := yangToNode(child, childPath)
		if node != nil {
			fields = append(fields, Field(name, node))
		}
	}
	l := List(keyType, fields...)

	// Auto-inject inactive leaf into structural lists (those with container/list children).
	// Skip positional lists where all children are leaves (nlri, nexthop, add-path).
	// These are compact data entries; deactivate the parent instead.
	if !l.Has(InactiveLeafName) && hasStructuralChildren(l) {
		l.children[InactiveLeafName] = inactiveLeaf()
		l.order = append(l.order, InactiveLeafName)
	}
	l.KeyName = entry.Key
	l.Hidden = hasHiddenExtension(entry)
	l.Ephemeral = hasEphemeralExtension(entry)
	l.Description = entry.Description

	// Scan children for ze:display-key extension (keyless lists only).
	if entry.Key == "" {
		for _, name := range names {
			if child, ok := entry.Dir[name]; ok && hasDisplayKeyExtension(child) {
				l.DisplayKey = name
				break
			}
		}
	}

	// Extract YANG unique constraints from Entry.Extra.
	if vals, ok := entry.Extra["unique"]; ok {
		for _, v := range vals {
			if uv, ok := v.(*gyang.Value); ok {
				fields := strings.Fields(uv.Name)
				if len(fields) > 0 {
					l.Unique = append(l.Unique, fields)
				}
			}
		}
	}

	// Extract ze:required and ze:suggest extensions from Entry.Exts.
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:required" || strings.HasSuffix(ext.Keyword, ":required") {
			if fields := strings.Split(ext.Argument, "/"); len(fields) > 0 && fields[0] != "" {
				l.Required = append(l.Required, fields)
			}
		}
		if ext.Keyword == "ze:suggest" || strings.HasSuffix(ext.Keyword, ":suggest") {
			if fields := strings.Split(ext.Argument, "/"); len(fields) > 0 && fields[0] != "" {
				l.Suggest = append(l.Suggest, fields)
			}
		}
	}

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
		childPath := AppendPath(path, name)
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
		childPath := AppendPath(path, name)
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
