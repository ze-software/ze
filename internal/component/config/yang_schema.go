// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: schema.go — schema types and validation
// Related: prune.go — inactive node pruning
//
// Package config provides YANG-to-schema conversion.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// PluginOnlySchema answers the schema of the first pass (FirstPass): the roots
// Ze reads to RUN a declared plugin, before that plugin's own module is known.
// They are `plugin`, which holds the declarations and the hub listener a
// forked plugin connects back to, and `environment`. Every other root is
// absent, so a strict parse over this schema refuses it and a parser in
// SkipUnknownRoots mode skips it.
//
// The schema is cut from YANGSchema, never resolved again: each root is the
// full schema's own node, with what every compiled-in module merged or
// augmented into it. The first pass runs on every start, so a loader of its
// own would cost each start a second resolve of the whole module set. The
// nodes are shared with YANGSchema and MUST NOT be written through this
// schema: a Define of a root it already holds would merge into the shared node.
//
// A root this build's schema does not hold is left out. No config of this
// build can hold it either: the full parse refuses it.
func PluginOnlySchema() (*Schema, error) {
	full, err := YANGSchema()
	if err != nil {
		return nil, err
	}
	schema := NewSchema()
	for _, root := range firstPassRoots {
		node := full.Get(root)
		if node == nil {
			continue
		}
		schema.Define(root, node)
	}
	return schema, nil
}

// YANGSchema returns the schema built from the embedded modules and the
// modules every init() registered (LoadRegistered), joined to the modules of
// external plugins once the process published them (PublishExternalModules).
// That input set changes at most once after the process has started, at the
// publish, and no product code writes to the schema after it is built
// (ExtendCapability has only a test caller), so the build runs once per module
// set (yangSchemaCache) and every later call shares it.
//
// It was rebuilt on every call until 2026-09-17, and a build resolves every
// YANG module: about 200 ms. One config commit called it fifteen times, three
// for each of the five parses a reload performs (LoadConfig, the BGP reload
// function, peersAndDynamicGroups), so a commit of one changed leaf cost
// three seconds of schema builds and the REST commit missed a 5 s client
// deadline. A schema built over module texts no init() registered is not
// cached here: YANGSchemaWithPlugins takes those as an argument and joins them
// on each call.
func YANGSchema() (*Schema, error) {
	return yangSchemaCache.get(schemaFromResolved)
}

// yangSchemaCache holds the plugin-free schema, built once per module set
// DefaultLoader answers. A failed build is cached too: the same module set
// cannot fail differently on a retry.
var yangSchemaCache resolvedCache[*Schema]

// resolvedCache memoizes one value built from yang.DefaultLoader's module set,
// keyed by the *yang.Resolved it was built from. DefaultLoader answers the
// same Resolved for as long as the registered set is unchanged and a new one
// when the process publishes the modules of its external plugins
// (PublishExternalModules), or when a test registers a module
// (yang.RegisterModuleForTest) or restores the set, so the schema, the
// validator and the completer all see one module set.
// A plain sync.Once froze whichever set the first caller saw. The key keeps
// the Resolved it names reachable, so a later Resolved never reuses its
// address. Safe for concurrent use; concurrent callers wait for one build.
type resolvedCache[T any] struct {
	mu    sync.Mutex
	built bool
	key   *yang.Resolved
	value T
	err   error
}

// reset forgets the cached value, so the next get builds it again.
func (c *resolvedCache[T]) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	var none T
	c.built, c.key, c.value, c.err = false, nil, none, nil
}

// get answers the value built from DefaultLoader's current module set,
// building it with build the first time that set is asked for.
func (c *resolvedCache[T]) get(build func(*yang.Resolved) (T, error)) (T, error) {
	resolved, err := loadYANGModules(nil)
	if err != nil {
		var none T
		return none, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.built && c.key == resolved {
		return c.value, c.err
	}
	c.value, c.err = build(resolved)
	c.key, c.built = resolved, true
	return c.value, c.err
}

// loadYANGModules answers the module set DefaultLoader resolved, shared by the
// whole process, or, given module texts, that set joined to them
// (JoinExternalModules). The texts are a Go caller's: each has no plugin
// declaration beside it, so its name and namespace are read from the text and
// its file name takes the plugin's place in a refusal.
//
// A text whose module the compiled-in set already holds with the same content,
// byte for byte, is that module: it adds nothing and is left out before the
// join (Resolved.HoldsModuleText). A text with a held name and different
// content goes to the join and is refused as a name clash. When every text is
// left out the answer is the shared set, with no resolve. This rule is for
// this road alone: a module an external plugin declares goes through
// JoinExternalModules, which refuses a compiled-in name even for an identical
// text. Shared by YANGSchemaWithPlugins and YANGValidatorWithPlugins.
func loadYANGModules(moduleTexts map[string]string) (*yang.Resolved, error) {
	resolved, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("resolve YANG modules: %w", err)
	}
	files := slices.Sorted(maps.Keys(moduleTexts))
	externals := make([]yang.ExternalModule, 0, len(files))
	for _, file := range files {
		external, textErr := yang.ExternalModuleFromText(file, moduleTexts[file])
		if textErr != nil {
			return nil, textErr
		}
		if resolved.HoldsModuleText(external.Module, external.Text) {
			continue
		}
		externals = append(externals, external)
	}
	if len(externals) == 0 {
		return resolved, nil
	}
	return JoinExternalModules(externals)
}

// JoinExternalModules answers the compiled-in module set joined to the modules
// external plugins declared, or the operator error that names the plugin and
// the module the join refuses (yang.Join states each refusal). yang.Join is
// the one producer of a set that holds a module no init() registered. This
// entry answers that set to its caller alone and changes nothing for any other
// reader; PublishExternalModules makes it the set of the whole process.
//
// The set joined to is the one DefaultLoader answers. After a publish that is
// the published set, so a module the process already published is refused
// here as a held name: a caller that holds the declarations of the process's
// own plugins publishes them, and does not join them a second time.
//
// With no external module it answers the Resolved DefaultLoader shares, and
// resolves nothing: YANGSchema and YANGValidator are cached against that same
// Resolved, so a start that declares no external plugin builds nothing twice.
// With external modules the answer is a new Resolved, which schemaFromResolved
// and validatorFromResolved turn into the schema and the validator the same
// way they do for the compiled-in set, so a range, a pattern or a mandatory
// statement of an external module refuses a value as a compiled-in one does.
func JoinExternalModules(externals []yang.ExternalModule) (*yang.Resolved, error) {
	compiled, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("resolve YANG modules: %w", err)
	}
	return yang.Join(compiled, externals, newValidatorRegistry())
}

// PublishExternalModules makes the compiled-in module set joined to externals
// the set every schema reader of this process is answered from, for the rest
// of the process: yang.DefaultLoader, and with it YANGSchema and YANGValidator,
// which are cached against the Resolved DefaultLoader answers and so build
// once more for the joined set. externals is the module each external plugin
// declared, in config order.
//
// It is yang.PublishExternalModules with the validator registry the daemon
// holds, and that function states the contract: a refusal of the join
// publishes nothing and comes back unchanged; the set is published at most
// once per process, a second call with the same modules does nothing and a
// second call with other modules is refused, naming the restart; with no
// external module nothing is joined or resolved.
//
// The caller MUST publish before the full config is parsed and before the
// process builds anything it keeps from the schema for its whole life (the
// plugin server, the gNMI server): a publish that comes after one of those is
// refused. Safe for concurrent use.
func PublishExternalModules(externals []yang.ExternalModule) error {
	return yang.PublishExternalModules(externals, newValidatorRegistry())
}

// newValidatorRegistry answers the registry of every custom validator the
// daemon holds: the ones RegisterValidators lists, with the completions and
// suggestions components registered from init(). The value validator is built
// with it, and the join judges an external module's ze:validate against it.
func newValidatorRegistry() *yang.ValidatorRegistry {
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompletions()
	return reg
}

// RefuseUnheldRegistrations refuses set, the module set DefaultLoader
// answers, when a module compiled into ze names a ze:validate function or a
// ze:reference source this process does not hold, with the refusal
// yang.RefuseUnheldCompiledRegistrations states: a defect of the build, judged
// by the rule the join applies to an external module, against the registry the
// value validator is built with. Every error it answers is that refusal.
//
// Only a binary that links every package registering a name its modules use
// can pass it: the ze binary, through the composition root plugin/all. A test
// binary that links a module without the package registering its names holds
// a smaller set and is refused. `ze start` calls it before it parses the
// config (cmd/ze/hub refuseWronglyBuilt), so a daemon never runs such a build.
func RefuseUnheldRegistrations(set *yang.Resolved) error {
	return yang.RefuseUnheldCompiledRegistrations(set, newValidatorRegistry())
}

// YANGValidator answers the plugin-free YANG value validator, built once per
// module set (yangValidatorCache) and shared by every later call, for the
// reason YANGSchema is: its input is embedded or registered at init, and
// nothing changes the validator after it is built.
//
// ValidateCustomSections rebuilt it on every call, and it runs on every config
// load and every editor commit: each build resolves every YANG module, so a
// commit spent its time parsing YANG, and under the race detector a one-leaf
// editor commit outran its 30 s .et budget. A failed build is cached too, for
// the same reason as yangSchemaCache. Safe for concurrent use: the validator is
// read-only once built.
func YANGValidator() (*yang.Validator, error) {
	return yangValidatorCache.get(validatorFromResolved)
}

// yangValidatorCache holds the plugin-free validator, once per module set, as
// yangSchemaCache holds the schema.
var yangValidatorCache resolvedCache[*yang.Validator]

// YANGValidatorWithPlugins creates a YANG value validator over the compiled-in
// modules joined to the module texts given (loadYANGModules), under the rule
// YANGSchemaWithPlugins states for them.
// Registers custom validators, attaches registry, and checks integrity.
// Used for runtime attribute validation (origin enum, med/local-pref uint32 ranges).
func YANGValidatorWithPlugins(pluginYANG map[string]string) (*yang.Validator, error) {
	loader, err := loadYANGModules(pluginYANG)
	if err != nil {
		return nil, err
	}
	return validatorFromResolved(loader)
}

// validatorFromResolved builds the value validator over one resolved module set.
func validatorFromResolved(loader *yang.Resolved) (*yang.Validator, error) {
	v := yang.NewValidator(loader)

	// Register custom validators and attach to validator.
	reg := newValidatorRegistry()
	v.SetRegistry(reg)

	// `ze start` refuses a build whose compiled-in modules name a registration
	// it does not hold (RefuseUnheldRegistrations, before the config is
	// parsed), so these two lines are reached only outside a daemon: a local
	// command of a wrongly built ze, or a test binary that links a module
	// without the package registering its names. They are not a refusal: such a
	// test binary judges every other leaf as ze does, and refusing would end it.
	if err := yang.CheckAllValidatorsRegistered(loader, reg); err != nil {
		slog.Error("a YANG leaf names a ze:validate function this process does not hold, "+
			"so the leaf is not validated here; ze start refuses such a build",
			"error", err, "code", diagnostic.CodeDoctorBuildRegistrationUnheld)
	}
	// A leaf whose source no component registered refuses every value at
	// commit (yang.ValidateReferences).
	if err := yang.CheckAllReferenceSourcesRegistered(loader); err != nil {
		slog.Error("a YANG leaf names a ze:reference source this process does not hold, "+
			"so the leaf refuses every value here; ze start refuses such a build",
			"error", err, "code", diagnostic.CodeDoctorBuildRegistrationUnheld)
	}

	return v, nil
}

// YANGSchemaWithPlugins answers the schema of the compiled-in modules joined to
// the module texts given. pluginYANG maps a module file name to its YANG text,
// and loadYANGModules states what happens to each: a text identical to a
// module the compiled-in set holds is left out, a held name with another
// content is refused, and every other text passes the join.
func YANGSchemaWithPlugins(pluginYANG map[string]string) (*Schema, error) {
	loader, err := loadYANGModules(pluginYANG)
	if err != nil {
		return nil, err
	}
	return schemaFromResolved(loader)
}

// SchemaOf answers the config schema built over resolved, so a caller that
// holds its own module set (the editor's completer) judges values with the
// nodes of that set. The set DefaultLoader answers shares YANGSchema's cached
// build; any other set is built for this call, as YANGSchemaWithPlugins does.
func SchemaOf(resolved *yang.Resolved) (*Schema, error) {
	shipped, err := yang.DefaultLoader()
	if err != nil {
		return nil, err
	}
	if shipped == resolved {
		return YANGSchema()
	}
	return schemaFromResolved(resolved)
}

// schemaFromResolved builds the config schema over one resolved module set.
func schemaFromResolved(loader *yang.Resolved) (*Schema, error) {
	resetSchemaBuildErrors()
	resetOtherOSNodes()
	schema := NewSchema()

	// Walk every loaded YANG module whose name ends with "-conf"
	// and load its top-level data nodes into the schema. Config
	// modules follow the ze naming convention: ze-<component>-conf.
	// Augment-only modules (ze-graceful-restart, ze-role, ze-rib,
	// etc.) lack the suffix and produce no top-level nodes because
	// their augments attach to the target module's entry tree.
	//
	// Schema.Define merges same-named top-level containers, appending
	// the children a later module adds after the earlier module's.
	// Modules are visited in name order and each module's top level in
	// definition order, so the root and every shared container follow
	// the child order of docs/architecture/config/syntax.md whatever
	// order the modules were registered or joined in.
	modNames := loader.ModuleNames()
	slices.Sort(modNames)
	for _, modName := range modNames {
		if !strings.HasSuffix(modName, "-conf") {
			continue
		}
		entry := loader.GetEntry(modName)
		if entry == nil {
			continue
		}
		for _, child := range definitionOrder(entry, "") {
			node := yangToNode(child.entry, child.name)
			if node != nil {
				schema.Define(child.name, node)
			}
		}
	}

	if buildErr := flushSchemaBuildErrors(); buildErr != nil {
		return nil, fmt.Errorf("schema build errors: %w", buildErr)
	}

	markOtherOSNodes(schema)
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
	// The parent records the name, so the parser refuses it as a node of
	// another OS rather than as an unknown field (os_restricted.go). It also
	// keeps the node built, so LookupDeclared can answer what the schema
	// declares about it, such as its defaults.
	if requiredOS := getOSExtension(entry); requiredOS != "" && requiredOS != schemaOS {
		recordOtherOSNode(path, requiredOS, yangToNodeOfAnyOS(entry, path))
		return nil
	}
	return yangToNodeOfAnyOS(entry, path)
}

// yangToNodeOfAnyOS converts a non-nil YANG entry to a schema node whatever
// its own ze:os names. A child's ze:os still prunes that child, because the
// child goes through yangToNode.
func yangToNodeOfAnyOS(entry *gyang.Entry, path string) Node {
	// Check for ze:syntax extension in YANG entry
	syntax := getSyntaxExtension(entry)

	// ze:decorate names a display-time decorator, and Decorate is a field on
	// LeafNode ALONE. Every leaf-list syntax below builds MultiLeafNode,
	// BracketLeafListNode or ValueOrArrayNode, none of which has that field, and
	// only yangToLeaf ever calls getDecorateExtension. So a decorator written on
	// a leaf-list parses, validates, and is then dropped without a word: the web
	// render reads Decorate off a *config.LeafNode and never sees the entry.
	//
	// Refuse it rather than discard it. A silently ignored decorator is a guard
	// that fails open, and the author has no way to tell "not supported" from
	// "supported and broken" (ai/rules/evidence.md). No leaf-list carries
	// ze:decorate today, so this refuses nothing that works.
	//
	// Supporting it means teaching the render path to decorate each element and
	// giving the leaf-list node types the field. That work is
	// plan/spec-bgp-deferred-community-name-leaf.md, which wants it for the BGP
	// community leaf-list. Delete this guard when that lands.
	if decorator := getDecorateExtension(entry); decorator != "" && isLeafListNode(syntax, entry) {
		recordSchemaBuildError(fmt.Errorf(
			"%s: ze:decorate %q on a leaf-list is not supported and would be silently dropped; "+
				"only a single-valued leaf can carry a decorator", path, decorator))
	}

	switch syntax {
	case "flex":
		return yangToFlex(entry, path)
	case "freeform":
		return Freeform()
	case "inline-list":
		keyType := getKeyTypeExtension(entry)
		return yangToInlineListWithKey(entry, path, keyType)
	case "multi-leaf":
		return yangToMultiLeaf(entry, path)
	case "bracket":
		return yangToBracketLeafList(entry, path)
	case "value-or-array":
		return yangToValueOrArray(entry, path)
	}

	// Standard YANG node types
	// Only leaf and directory data entries project into config schema nodes.
	//exhaustive:ignore
	switch entry.Kind {
	case gyang.LeafEntry:
		// leaf-list without ze:syntax extension — accepts single value or bracket list
		if entry.IsLeafList() {
			validateLeafDefaults(entry, path)
			return yangToValueOrArray(entry, path)
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

// isLeafListNode reports whether yangToNode will build a multi-valued leaf node
// for this entry, from either an explicit ze:syntax or plain YANG leaf-list.
//
// It mirrors the branches in yangToNode that return MultiLeafNode,
// BracketLeafListNode or ValueOrArrayNode. Keep the two in step: a new
// multi-valued syntax added there and not named here goes back to dropping a
// ze:decorate in silence, which is the failure the caller exists to stop.
func isLeafListNode(syntax string, entry *gyang.Entry) bool {
	switch syntax {
	case "multi-leaf", "bracket", "value-or-array":
		return true
	}
	// A plain leaf-list with no ze:syntax reaches the same node types.
	return entry.Kind == gyang.LeafEntry && entry.IsLeafList()
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

// getOrderedExtension reports whether the ze:ordered extension is present on a
// leaf-list entry. It marks the leaf-list as an ordered SEQUENCE whose duplicate
// values are meaningful (AS_PATH prepends, MPLS label stacks) rather than a set,
// so the parser preserves duplicates instead of deduplicating.
func getOrderedExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:ordered" || strings.HasSuffix(ext.Keyword, ":ordered") {
			return true
		}
	}
	return false
}

// getKeyTypeExtension reads the ze:key-type extension from a YANG entry.
// Returns "prefix" as default for inline-list nodes.
func getKeyTypeExtension(entry *gyang.Entry) ValueType {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:key-type" || strings.HasSuffix(ext.Keyword, ":key-type") {
			switch ext.Argument {
			case env.TypeString:
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
	return yang.GetBackendExtension(entry)
}

// hasHiddenExtension checks if a YANG entry has the ze:hidden extension with argument "true".
func hasHiddenExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:hidden" || strings.HasSuffix(ext.Keyword, ":hidden") {
			return ext.Argument == configTrue
		}
	}
	return false
}

// hasFilterExtension reports whether a YANG list carries the ze:filter marker,
// which makes each of its entries a named route policy filter instance. The
// policy registry (internal/component/policy) discovers filter types by this
// flag alone, so a list beside them under the policy root (policy routing's
// route list) is never taken for one.
func hasFilterExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:filter" || strings.HasSuffix(ext.Keyword, ":filter") {
			return true
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
	node := yangValueLeaf(entry, path)
	// RFC 7950 Section 7.3.4: "If the base type has a default value and the
	// new derived type does not specify a new default value, the base type's
	// default value is also the default value of the new derived type."
	// DefaultValues returns the leaf's own default, else its type's default,
	// and none on a mandatory leaf.
	if defaults := entry.DefaultValues(); len(defaults) > 0 {
		node.Default = defaults[0]
		// RFC 7950 Section 9.1: a module may write an integer default in
		// hexadecimal or octal, and the config tree takes the decimal value.
		// A union default takes the form of the first member type that
		// accepts it (Section 9.12).
		// A default whose notation is refused keeps its text, and
		// validateLeafDefaults records the build error for it.
		if canonical, err := yang.CanonicalModuleDefault(entry, entry.Type, defaults[0]); err == nil {
			node.Default = canonical
		}
	}
	validateLeafDefaults(entry, path)
	node.Sensitive = yang.HasSensitiveExtension(entry)
	node.Bcrypt = yang.HasBcryptExtension(entry)
	node.Hidden = hasHiddenExtension(entry)
	node.Ephemeral = hasEphemeralExtension(entry)
	node.Decorate = getDecorateExtension(entry)
	node.ShortHelp = yang.GetHelpExtension(entry.Exts) // the ze:help summary
	node.Description = entry.Description               // the long explanation
	node.Backend = getBackendExtension(entry)
	node.Related = extractRelatedTools(entry, path)
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:required" || strings.HasSuffix(ext.Keyword, ":required") {
			recordSchemaBuildError(fmt.Errorf("at %s: ze:required on a leaf is invalid (use mandatory true instead)", path))
		}
	}
	return node
}

// enumFactsFromEntry answers the values of entry's enumeration in YANG
// declaration order and the ze:help each declares, by name, or nil and nil
// for an entry whose type is no enumeration. A refusal lists the values in
// this order with this help; the order is the module author's, so the default
// comes first and related values stay together.
func enumFactsFromEntry(entry *gyang.Entry, path string) ([]string, map[string]string) {
	if entry.Type == nil || entry.Type.Kind != gyang.Yenum || entry.Type.Enum == nil {
		return nil, nil
	}
	enums, err := yang.EnumNamesDeclared(entry)
	if err != nil {
		recordSchemaBuildError(fmt.Errorf("at %s: %w", path, err))
	}
	return enums, yang.EnumValueSummaries(entry)
}

// defaultValidator checks every default statement against its type at
// schema build. It holds no loader: a type check reads the YangType alone.
var defaultValidator = yang.NewValidator(nil)

// checkModuleDefault checks one default statement against its type. The default
// is read in the notation a module may use, then its canonical value is checked,
// so a hexadecimal or octal integer default is held to the type's range.
//
// RFC 7950 Section 9.1: "Implementations MUST support all lexical representations
// specified in this document." Section 9.2.1 adds the hexadecimal and octal
// notations for an integer default in a YANG module.
func checkModuleDefault(path string, leaf *gyang.Entry, yangType *gyang.YangType, def string) error {
	canonical, err := yang.CanonicalModuleDefault(leaf, yangType, def)
	if err != nil {
		return err
	}
	return defaultValidator.ValidateLeafType(path, leaf, yangType, canonical)
}

// validateLeafDefaults records a schema build error for a default statement
// its own type refuses, and for a default placed where RFC 7950 forbids one.
// goyang copies a leaf's own default onto entry.Default and a typedef's
// default onto entry.Type.Default, so both are read: the leaf's own default
// here, the typedef's in validateTypedefDefault.
func validateLeafDefaults(entry *gyang.Entry, path string) {
	if entry.Type == nil {
		return
	}
	// RFC 7950 Section 7.6.4: "The value of the "default" statement MUST be
	// valid according to the type specified in the leaf's "type" statement."
	// RFC 7950 Section 7.7.4 states the same for a leaf-list, and Section
	// 7.3.4: "The value of the "default" statement MUST be valid according
	// to the type specified in the "type" statement."
	for _, def := range entry.Default {
		if err := checkModuleDefault(path, entry, entry.Type, def); err != nil {
			recordSchemaBuildError(fmt.Errorf("at %s: default %q is not valid for type %s: %w", path, def, entry.Type.Name, err))
		}
		validateDefaultNotIfFeature(entry, path, def)
	}
	if entry.Type.HasDefault {
		validateTypedefDefault(entry, path)
	}
	if len(entry.Default) == 0 {
		return
	}
	// RFC 7950 Section 7.6.4: "The "default" statement MUST NOT be present
	// on nodes where "mandatory" is "true"."
	if entry.Mandatory == gyang.TSTrue {
		recordSchemaBuildError(fmt.Errorf("at %s: default %q on a mandatory leaf is invalid (RFC 7950 Section 7.6.4)", path, entry.Default[0]))
	}
	// RFC 7950 Section 7.7.4: "The "default" statement MUST NOT be present
	// on nodes where "min-elements" has a value greater than or equal to
	// one."
	if entry.ListAttr != nil && entry.ListAttr.MinElements >= 1 {
		recordSchemaBuildError(fmt.Errorf("at %s: default %q on a leaf-list with min-elements %d is invalid (RFC 7950 Section 7.7.4)", path, entry.Default[0], entry.ListAttr.MinElements))
	}
}

// validateTypedefDefault checks the default a leaf's type carries from a
// typedef. goyang sets entry.Type.Base to the type statement of the typedef
// the leaf's type derives from, so its YangType holds the typedef's own
// restrictions, while entry.Type also holds the leaf's.
func validateTypedefDefault(entry *gyang.Entry, path string) {
	def := entry.Type.Default
	// RFC 7950 Section 7.3.4: "The value of the "default" statement MUST be
	// valid according to the type specified in the "type" statement."
	if base := entry.Type.Base; base != nil && base.YangType != nil {
		if err := checkModuleDefault(path, entry, base.YangType, def); err != nil {
			recordSchemaBuildError(fmt.Errorf("at %s: typedef default %q is not valid for type %s: %w", path, def, base.YangType.Name, err))
			return
		}
	}
	if len(entry.Default) > 0 {
		return
	}
	// RFC 7950 Section 7.3.4: "If the type's default value is not valid
	// according to the new restrictions specified in a derived type or leaf
	// definition, the derived type or leaf definition MUST specify a new
	// default value compatible with the restrictions."
	if err := checkModuleDefault(path, entry, entry.Type, def); err != nil {
		recordSchemaBuildError(fmt.Errorf("at %s: inherited default %q is not valid for the restricted type %s, which must specify a new default: %w", path, def, entry.Type.Name, err))
		return
	}
	validateDefaultNotIfFeature(entry, path, def)
}

// typedefChainMax bounds the walk from a leaf's type statement through the
// typedefs it derives from, and into the union member that holds the default.
const typedefChainMax = 64

// validateDefaultNotIfFeature records a schema build error when an enum or a
// bit that the default names carries an if-feature statement, in the leaf's
// own type statement, in any typedef the type derives from, or in the union
// member type that the default's value belongs to.
func validateDefaultNotIfFeature(entry *gyang.Entry, path, def string) {
	var stmt *gyang.Type
	switch n := entry.Node.(type) {
	case *gyang.Leaf:
		stmt = n.Type
	case *gyang.LeafList:
		stmt = n.Type
	}
	names := strings.Fields(def)
	for range typedefChainMax {
		if stmt == nil {
			return
		}
		// RFC 7950 Section 7.6.4: "The definition of the default value MUST
		// NOT be marked with an "if-feature" statement."
		for _, enum := range stmt.Enum {
			if enum.Name == def && len(enum.IfFeature) > 0 {
				recordSchemaBuildError(fmt.Errorf("at %s: default %q names enum %q, whose definition carries an if-feature (RFC 7950 Section 7.6.4)", path, def, enum.Name))
				return
			}
		}
		for _, bit := range stmt.Bit {
			if slices.Contains(names, bit.Name) && len(bit.IfFeature) > 0 {
				recordSchemaBuildError(fmt.Errorf("at %s: default %q names bit %q, whose definition carries an if-feature (RFC 7950 Section 7.6.4)", path, def, bit.Name))
				return
			}
		}
		if len(stmt.Type) > 0 {
			// RFC 7950 Section 9.12: "a value is validated consecutively
			// against each member type, in the order they are specified in
			// the "type" statement, until a match is found."
			stmt = unionMemberHolding(stmt.Type, path, def)
			continue
		}
		if stmt.YangType == nil || stmt.YangType.Base == stmt {
			return
		}
		stmt = stmt.YangType.Base
	}
}

// unionMemberHolding returns the first union member type whose resolved type
// accepts def, which is the type the default's value belongs to. It returns
// nil when no member accepts def: that default is not valid for its type, and
// the type check in validateLeafDefaults records that error, so there is no
// definition left to inspect for an if-feature.
func unionMemberHolding(members []*gyang.Type, path, def string) *gyang.Type {
	for _, member := range members {
		if member.YangType == nil {
			continue
		}
		if defaultValidator.ValidateType(path, member.YangType, def) == nil {
			return member
		}
	}
	return nil
}

func patternsFromType(typ *gyang.YangType) []string {
	if typ == nil || len(typ.Pattern) == 0 {
		return nil
	}
	patterns := make([]string, len(typ.Pattern))
	copy(patterns, typ.Pattern)
	return patterns
}

// unionMembersFromType lifts the member types of a YANG union into the schema,
// each with its own restrictions, so ValidateLeafValue can judge a value against
// them. Without it a union maps to TypeString and its members are never read.
// A union a type name already maps (zt:ip-address is TypeIP) is judged by that
// value type and answers nil here.
//
// RFC 7950 Section 9.12: "A member type can be of any built-in or derived
// type." A member that is itself a union recurses; the depth is bounded by the
// type nesting the loaded YANG modules declare.
//
// leaf is the leaf whose type statements declare each member's error-message
// strings (yang.LeafConstraintMessages).
func unionMembersFromType(leaf *gyang.Entry, typ *gyang.YangType) []UnionMember {
	if typ == nil {
		return nil
	}
	if typ.Kind != gyang.Yunion {
		return nil
	}
	if yangTypeToValueType(typ) != TypeString {
		return nil
	}
	members := make([]UnionMember, 0, len(typ.Type))
	for _, member := range typ.Type {
		node := Leaf(yangTypeToValueType(member))
		markDecimal64(node, member)
		node.Ranges = numericRangesFromType(member, "")
		node.Patterns = patternsFromType(member)
		node.Lengths = lengthRangesFromType(member)
		node.Constraint = yang.LeafConstraintMessages(leaf, member)
		node.Format = yang.LeafValueFormat(leaf, member)
		if member.Kind == gyang.Yenum && member.Enum != nil {
			// The member's values in the module's order, with the help the
			// leaf's enumerations declare, as a refusal lists them.
			enums, err := yang.EnumNamesDeclaredOf(leaf, member)
			if err != nil {
				recordSchemaBuildError(fmt.Errorf("at %s: %w", leaf.Path(), err))
			}
			node.Enums = enums
			node.EnumHelp = yang.EnumValueSummaries(leaf)
		}
		node.Union = unionMembersFromType(leaf, member)
		members = append(members, UnionMember{TypeName: member.Name, Leaf: node})
	}
	return members
}

// lengthRangesFromType lifts YANG `length` restrictions into the schema. Without
// it the statement parses and is then discarded, which is what made every
// `length "1..255"` in the tree decorative.
func lengthRangesFromType(typ *gyang.YangType) []NumericRange {
	if typ == nil || len(typ.Length) == 0 {
		return nil
	}
	lengths := make([]NumericRange, 0, len(typ.Length))
	for _, r := range typ.Length {
		lengths = append(lengths, NumericRange{Min: r.Min.String(), Max: r.Max.String()})
	}
	return lengths
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

// InactiveLeafName is the token prefix used by the parser to recognize
// "inactive:" annotations in config files.
const InactiveLeafName = "inactive"

// HasStructuralChildren reports whether a ListNode has at least one
// non-leaf child (container or list). Positional lists with only leaf
// children are compact data entries that don't benefit from inactive.
func (l *ListNode) HasStructuralChildren() bool {
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
	children := definitionOrder(entry, path)
	fields := make([]FieldDef, 0, len(children)+1)
	var cases choiceMembers
	for _, c := range children {
		childPath := AppendPath(path, c.name)
		node := yangToNode(c.entry, childPath)
		if node != nil {
			fields = append(fields, Field(c.name, node))
			cases.record(c.name, c.entry)
		}
	}
	container := Container(fields...)
	container.cases = cases

	// Check for ze:allow-unknown-fields extension
	container.AllowUnknown = hasAllowUnknownExtension(entry)

	// Check for ze:hidden and ze:ephemeral extensions
	container.Hidden = hasHiddenExtension(entry)
	container.Ephemeral = hasEphemeralExtension(entry)

	// Check for YANG presence statement — enables flag/value/block modes
	container.Presence = hasPresenceStatement(entry)
	container.Flatten = hasFlattenExtension(entry)
	container.ShortHelp = yang.GetHelpExtension(entry.Exts) // the ze:help summary
	container.Description = entry.Description               // the long explanation
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
	// The key type and the key leaf come from the FIRST key leaf. A YANG
	// key names one leaf or several ("peer vrf interface"); the first is the
	// value an entry is written under, and the others are addressed by name
	// (list_key.go).
	firstKey := ""
	if keys := strings.Fields(entry.Key); len(keys) > 0 {
		firstKey = keys[0]
	}
	keyType := TypeString
	if firstKey != "" && entry.Dir != nil {
		if keyEntry, ok := entry.Dir[firstKey]; ok {
			keyType = yangTypeToValueType(keyEntry.Type)
		}
	}

	// The children in definition order, which is also the order a positional
	// one-line entry assigns its values in (parseListInlineEntry). Choice and
	// case nodes are flattened, as in yangToContainer.
	children := definitionOrder(entry, path)
	fields := make([]FieldDef, 0, len(children)+1)
	var cases choiceMembers
	for _, c := range children {
		if c.name == entry.Key {
			continue // Key is not a child field
		}
		node := yangToNode(c.entry, AppendPath(path, c.name))
		if node != nil {
			fields = append(fields, Field(c.name, node))
			cases.record(c.name, c.entry)
		}
	}
	l := List(keyType, fields...)
	if firstKey != "" && entry.Dir != nil {
		if keyEntry, ok := entry.Dir[firstKey]; ok {
			l.KeyLeaf = yangToLeaf(keyEntry, AppendPath(path, firstKey))
		}
	}

	l.KeyName = entry.Key
	l.cases = cases
	l.Hidden = hasHiddenExtension(entry)
	l.Ephemeral = hasEphemeralExtension(entry)
	l.UserOrdered = entry.ListAttr != nil && entry.ListAttr.OrderedByUser
	l.Listener = hasListenerExtension(entry)
	l.Filter = hasFilterExtension(entry)
	l.ShortHelp = yang.GetHelpExtension(entry.Exts) // the ze:help summary
	l.Description = entry.Description               // the long explanation
	l.Backend = getBackendExtension(entry)

	// Scan children for ze:display-key extension (keyless lists only).
	if entry.Key == "" {
		for _, c := range children {
			if hasDisplayKeyExtension(c.entry) {
				l.DisplayKey = c.name
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
			if ext.Argument == "" {
				recordSchemaBuildError(fmt.Errorf("at %s: bare ze:required; is invalid (use mandatory true for leaf-level required, or ze:required \"path\" for list-level)", path))
				continue
			}
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

// yangToFlex converts YANG entry to FlexNode.
func yangToFlex(entry *gyang.Entry, path string) *FlexNode {
	if entry.Dir == nil {
		f := Flex()
		f.Backend = getBackendExtension(entry)
		return f
	}

	children := definitionOrder(entry, path)
	fields := make([]FieldDef, 0, len(children))
	for _, c := range children {
		node := yangToNode(c.entry, AppendPath(path, c.name))
		if node != nil {
			fields = append(fields, Field(c.name, node))
		}
	}
	f := Flex(fields...)
	f.Backend = getBackendExtension(entry)
	return f
}

// yangToInlineListWithKey converts YANG entry to InlineListNode with specified key type.
func yangToInlineListWithKey(entry *gyang.Entry, path string, keyType ValueType) *InlineListNode {
	children := definitionOrder(entry, path)
	fields := make([]FieldDef, 0, len(children))
	for _, c := range children {
		if c.name == entry.Key {
			continue
		}
		node := yangToNode(c.entry, AppendPath(path, c.name))
		if node != nil {
			fields = append(fields, Field(c.name, node))
		}
	}

	l := InlineList(keyType, fields...)
	l.Backend = getBackendExtension(entry)
	return l
}

// markDecimal64 makes node, a leaf or a union member of type typ, a decimal64
// number when typ is decimal64, so its range is compared as a decimal of the
// type's fraction digits rather than refused as text (ValidateLeafValue). A
// list key keeps the string type yangTypeToValueType answers. A multi-valued
// node takes this type and its fraction digits through yangValueLeaf too.
//
// RFC 7950 Section 9.3.4: "The "fraction-digits" statement, which is a
// substatement to the "type" statement, MUST be present if the type is
// "decimal64"."
func markDecimal64(node *LeafNode, typ *gyang.YangType) {
	if typ == nil {
		return
	}
	if typ.Kind != gyang.Ydecimal64 {
		return
	}
	node.Type = TypeDecimal64
	node.FractionDigits = typ.FractionDigits
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
		return TypeASN
	}

	switch t.Kind {
	case gyang.Ystring:
		return TypeString
	case gyang.Ybool:
		return TypeBool
	case gyang.Yempty:
		return TypeEmpty
	case gyang.Yuint8, gyang.Yuint16:
		return TypeUint16
	case gyang.Yuint32, gyang.Yuint64:
		return TypeUint32
	case gyang.Yint8, gyang.Yint16, gyang.Yint32, gyang.Yint64:
		return TypeInt
	case gyang.Yenum:
		return TypeString // Enums stored as strings
	case gyang.Ynone, gyang.Ybinary, gyang.Ybits, gyang.Ydecimal64,
		gyang.Yidentityref, gyang.YinstanceIdentifier, gyang.Yleafref, gyang.Yunion:
		return TypeString
	default:
		// Plugin-supplied YANG kinds are open; retain string storage for unknown kinds.
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
	return asn.IsTypedef(name)
}

// LoadYANGModule loads a YANG module entry into a schema, its top level in
// definition order, and answers the schema build errors the conversion
// recorded, as schemaFromResolved does for the config schema.
// This is used by external packages to create schemas from their own YANG modules.
func LoadYANGModule(schema *Schema, entry *gyang.Entry) error {
	if entry == nil {
		return nil
	}
	resetSchemaBuildErrors()
	for _, child := range definitionOrder(entry, "") {
		node := yangToNode(child.entry, child.name)
		if node != nil {
			schema.Define(child.name, node)
		}
	}
	return flushSchemaBuildErrors()
}
