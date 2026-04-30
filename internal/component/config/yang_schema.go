// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: schema.go — schema types and validation
// Related: prune.go — inactive node pruning (uses InactiveLeafName)
//
// Package config provides YANG-to-schema conversion.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/command"

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
	resetSchemaBuildErrors()

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

	if buildErr := flushSchemaBuildErrors(); buildErr != nil {
		return nil, fmt.Errorf("schema build errors: %w", buildErr)
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
	resetSchemaBuildErrors()

	loader, err := loadYANGModules(pluginYANG)
	if err != nil {
		return nil, err
	}

	schema := NewSchema()

	// Walk every loaded YANG module whose name ends with "-conf"
	// and load its top-level data nodes into the schema. Config
	// modules follow the ze naming convention: ze-<component>-conf.
	// Augment-only modules (ze-graceful-restart, ze-role, ze-rib,
	// etc.) lack the suffix and produce no top-level nodes because
	// their augments attach to the target module's entry tree.
	//
	// Schema.Define merges same-named top-level containers so
	// modules that define children under a shared parent (e.g.,
	// multiple -conf modules contributing to "environment")
	// compose correctly regardless of iteration order.
	modNames := loader.ModuleNames()
	sort.Strings(modNames)
	for _, modName := range modNames {
		if !strings.HasSuffix(modName, "-conf") {
			continue
		}
		entry := loader.GetEntry(modName)
		if entry == nil {
			continue
		}
		for _, name := range sortedKeys(entry.Dir) {
			child := entry.Dir[name]
			node := yangToNode(child, name)
			if node != nil {
				schema.Define(name, node)
			}
		}
	}

	if buildErr := flushSchemaBuildErrors(); buildErr != nil {
		return nil, fmt.Errorf("schema build errors: %w", buildErr)
	}

	return schema, nil
}

// ValidateSchemaAgainstCommandTree runs the strict ze:related command-tree
// check that would otherwise fail the schema build. Callers that hold both
// the resolved schema and the full operational command tree -- typically
// the hub's main process where every -cmd YANG module has been registered
// -- invoke this after schema build to surface typo and rename errors at
// startup. Unit tests that build a schema with only a subset of -cmd
// modules deliberately skip this step; otherwise every descriptor whose
// command lives in an unloaded subtree would falsely fail.
//
// Returns nil when every descriptor's static prefix matches a path in the
// supplied tree.
func ValidateSchemaAgainstCommandTree(s *Schema, tree *command.Node) error {
	if s == nil || tree == nil || len(tree.Children) == 0 {
		return nil
	}
	resetSchemaBuildErrors()
	walkSchemaForRelatedValidation(s, tree)
	return flushSchemaBuildErrors()
}

// walkSchemaForRelatedValidation walks every node in the schema, collecting
// RelatedTool descriptors and validating them against the operational
// command tree. Errors are recorded in the build accumulator.
func walkSchemaForRelatedValidation(s *Schema, tree *command.Node) {
	if s == nil || s.root == nil {
		return
	}
	walkNodeRelated(s.root, tree, "")
}

func walkNodeRelated(n Node, tree *command.Node, path string) {
	switch v := n.(type) {
	case *ContainerNode:
		recordRelatedErrors(ValidateRelatedAgainstCommandTree(v.Related, tree), path)
		for _, name := range v.order {
			walkNodeRelated(v.Get(name), tree, AppendPath(path, name))
		}
	case *ListNode:
		recordRelatedErrors(ValidateRelatedAgainstCommandTree(v.Related, tree), path)
		for _, name := range v.order {
			walkNodeRelated(v.Get(name), tree, AppendPath(path, name))
		}
	case *LeafNode:
		recordRelatedErrors(ValidateRelatedAgainstCommandTree(v.Related, tree), path)
	}
}

func recordRelatedErrors(errs []error, path string) {
	for _, err := range errs {
		recordSchemaBuildError(fmt.Errorf("at %s: %w", path, err))
	}
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
		node := MultiLeaf(yangTypeToValueType(entry.Type))
		node.Patterns = patternsFromType(entry.Type)
		return node
	case "bracket":
		node := BracketLeafList(yangTypeToValueType(entry.Type))
		node.Patterns = patternsFromType(entry.Type)
		return node
	case "value-or-array":
		node := ValueOrArray(yangTypeToValueType(entry.Type))
		node.Patterns = patternsFromType(entry.Type)
		return node
	}

	// Standard YANG node types
	//nolint:exhaustive // Only handle types relevant to config schema
	switch entry.Kind {
	case gyang.LeafEntry:
		// leaf-list without ze:syntax extension — accepts single value or bracket list
		if entry.IsLeafList() {
			if entry.Type != nil && entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
				node := ValueOrArrayEnum(entry.Type.Enum.Names())
				node.Patterns = patternsFromType(entry.Type)
				return node
			}
			node := ValueOrArray(yangTypeToValueType(entry.Type))
			node.Patterns = patternsFromType(entry.Type)
			return node
		}
		return yangToLeaf(entry, path)
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

// getBackendExtension reads every ze:backend statement on a YANG entry and
// returns the union of backend names across them, de-duplicated and in
// statement-then-argument order. Returns nil when no ze:backend is present
// (meaning unrestricted -- every registered backend is accepted).
//
// Whitespace tokenisation mirrors a YANG author writing
// `ze:backend "netlink vpp";`, where both names are valid. Tokens empty
// after trimming contribute nothing: `ze:backend "";` is identical to
// not writing the statement at all, and CANNOT reset or widen a sibling
// statement -- the union always grows (or is a no-op), never shrinks.
//
// Merging multiple statements (`ze:backend "netlink"; ze:backend "vpp";`)
// lets grouping authors add annotations without stomping a caller's existing
// one, and makes the merge rule explicit rather than "first statement wins"
// which surprises schema authors.
//
// Unlike ze:os (which prunes at schema-build time because GOOS is
// immutable), ze:backend is consulted post-parse: the active backend is
// chosen by the `backend` config leaf, not at schema-build time. The
// returned list is therefore stored on the schema Node for the walker
// in backend_gate.go.
func getBackendExtension(entry *gyang.Entry) []string {
	var (
		out  []string
		seen map[string]bool
	)
	for _, ext := range entry.Exts {
		if ext.Keyword != "ze:backend" && !strings.HasSuffix(ext.Keyword, ":backend") {
			continue
		}
		for f := range strings.FieldsSeq(ext.Argument) {
			if seen == nil {
				seen = make(map[string]bool)
			}
			if seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
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

// hasBcryptExtension checks if a YANG entry has the ze:bcrypt extension.
func hasBcryptExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:bcrypt" || strings.HasSuffix(ext.Keyword, ":bcrypt") {
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

// hasListenerExtension checks if a YANG entry has the ze:listener extension.
func hasListenerExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:listener" || strings.HasSuffix(ext.Keyword, ":listener") {
			return true
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

// schemaBuildErrors accumulates errors produced by extension extractors
// that cannot easily return errors through the existing yangTo* signature
// chain (yangToNode, yangToContainer, yangToList, yangToLeaf all return
// concrete types without errors). YANGSchema/YANGSchemaWithPlugins reset
// this slice at entry and join the accumulated errors at exit, so a
// malformed `ze:related` descriptor fails the schema build per spec.
var (
	schemaBuildErrorsMu sync.Mutex
	schemaBuildErrors   []error
)

// resetSchemaBuildErrors clears the build-time error accumulator. Called at
// the top of every schema build entry point.
func resetSchemaBuildErrors() {
	schemaBuildErrorsMu.Lock()
	schemaBuildErrors = nil
	schemaBuildErrorsMu.Unlock()
}

// recordSchemaBuildError appends an error to the build-time accumulator.
func recordSchemaBuildError(err error) {
	if err == nil {
		return
	}
	schemaBuildErrorsMu.Lock()
	schemaBuildErrors = append(schemaBuildErrors, err)
	schemaBuildErrorsMu.Unlock()
}

// flushSchemaBuildErrors returns and clears the accumulated errors. Returns
// nil when no errors were recorded.
func flushSchemaBuildErrors() error {
	schemaBuildErrorsMu.Lock()
	defer schemaBuildErrorsMu.Unlock()
	if len(schemaBuildErrors) == 0 {
		return nil
	}
	err := errors.Join(schemaBuildErrors...)
	schemaBuildErrors = nil
	return err
}

// extractRelatedTools parses every ze:related extension on the entry into
// RelatedTool descriptors. Records errors in the build-time accumulator
// when descriptors are malformed or share an id within the node; returns
// only the descriptors that parsed successfully so the rest of the schema
// build proceeds and surfaces every problem at once.
func extractRelatedTools(entry *gyang.Entry, path string) []*RelatedTool {
	var tools []*RelatedTool
	seenIDs := make(map[string]bool)

	for _, ext := range entry.Exts {
		if ext.Keyword != "ze:related" && !strings.HasSuffix(ext.Keyword, ":related") {
			continue
		}
		tool, err := ParseRelatedDescriptor(ext.Argument)
		if err != nil {
			recordSchemaBuildError(fmt.Errorf("ze:related on %s: %w", path, err))
			continue
		}
		if seenIDs[tool.ID] {
			recordSchemaBuildError(fmt.Errorf("ze:related on %s: duplicate id %q", path, tool.ID))
			continue
		}
		seenIDs[tool.ID] = true
		tools = append(tools, tool)
	}
	return tools
}

// yangToLeaf converts YANG leaf to LeafNode.
func yangToLeaf(entry *gyang.Entry, path string) *LeafNode {
	typ := yangTypeToValueType(entry.Type)
	node := Leaf(typ)
	if len(entry.Default) > 0 {
		node.Default = entry.Default[0]
	}
	node.Sensitive = hasSensitiveExtension(entry)
	node.Bcrypt = hasBcryptExtension(entry)
	node.Hidden = hasHiddenExtension(entry)
	node.Ephemeral = hasEphemeralExtension(entry)
	node.Decorate = getDecorateExtension(entry)
	node.Description = entry.Description
	node.Backend = getBackendExtension(entry)
	node.Related = extractRelatedTools(entry, path)
	if entry.Type != nil && entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
		node.Enums = entry.Type.Enum.Names()
	}
	node.Ranges = numericRangesFromType(entry.Type, path)
	node.Patterns = patternsFromType(entry.Type)
	return node
}

func patternsFromType(typ *gyang.YangType) []string {
	if typ == nil || len(typ.Pattern) == 0 {
		return nil
	}
	patterns := make([]string, len(typ.Pattern))
	copy(patterns, typ.Pattern)
	return patterns
}

func numericRangesFromType(typ *gyang.YangType, _ string) []NumericRange {
	if typ == nil || len(typ.Range) == 0 {
		return nil
	}
	ranges := make([]NumericRange, 0, len(typ.Range))
	for _, r := range typ.Range {
		ranges = append(ranges, NumericRange{Min: r.Min.String(), Max: r.Max.String()})
	}
	return ranges
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
	container.Backend = getBackendExtension(entry)
	container.Related = extractRelatedTools(entry, path)

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
	if entry.Key != "" && entry.Dir != nil {
		if keyEntry, ok := entry.Dir[entry.Key]; ok {
			l.KeyLeaf = yangToLeaf(keyEntry, AppendPath(path, entry.Key))
		}
	}

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
	l.Listener = hasListenerExtension(entry)
	l.Description = entry.Description
	l.Backend = getBackendExtension(entry)

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

	l.Related = extractRelatedTools(entry, path)

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
		f := Flex()
		f.Backend = getBackendExtension(entry)
		return f
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
	f := Flex(fields...)
	f.Backend = getBackendExtension(entry)
	return f
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

	l := InlineList(keyType, fields...)
	l.Backend = getBackendExtension(entry)
	return l
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
