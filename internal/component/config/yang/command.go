// Design: docs/architecture/config/yang-config-design.md -- YANG command tree extensions
// Related: validator_registry.go -- ze:validate extension (same pattern)

package yang

import (
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/component/command"
)

// cmdModuleSuffix identifies YANG command tree modules by naming convention.
const cmdModuleSuffix = "-cmd"

// WireMethodToPaths walks all -cmd YANG modules and builds a map from
// WireMethod (ze:command argument) to all CLI paths (space-joined tree paths).
// Multiple paths per wire method represent command aliases.
func WireMethodToPaths(loader *Loader) map[string][]string {
	result := make(map[string][]string)
	if loader == nil {
		return result
	}
	tree := BuildCommandTree(loader)
	collectPaths(tree, "", result)
	return result
}

// WireMethodToPath returns the shortest CLI path for each wire method.
// Deterministic: when multiple aliases exist, the lexicographically smallest
// path is chosen so restarts produce consistent authz context.
// Callers that need all aliases should use WireMethodToPaths.
func WireMethodToPath(loader *Loader) map[string]string {
	paths := WireMethodToPaths(loader)
	result := make(map[string]string, len(paths))
	for method, ps := range paths {
		if len(ps) == 0 {
			continue
		}
		best := ps[0]
		for _, p := range ps[1:] {
			if p < best {
				best = p
			}
		}
		result[method] = best
	}
	return result
}

// collectPaths recursively walks the command tree and collects WireMethod -> path mappings.
func collectPaths(node *command.Node, prefix string, result map[string][]string) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if child.WireMethod != "" {
			result[child.WireMethod] = append(result[child.WireMethod], path)
		}
		collectPaths(child, path, result)
	}
}

// PathToDescription walks all -cmd YANG modules and builds a map from
// CLI path (space-joined) to description. Used to populate help text
// when registering commands in the dispatcher.
func PathToDescription(loader *Loader) map[string]string {
	result := make(map[string]string)
	if loader == nil {
		return result
	}
	tree := BuildCommandTree(loader)
	collectDescriptions(tree, "", result)
	return result
}

// collectDescriptions recursively walks the command tree and collects path -> description.
func collectDescriptions(node *command.Node, prefix string, result map[string]string) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if child.Description != "" {
			result[path] = child.Description
		}
		collectDescriptions(child, path, result)
	}
}

// PathToTaskSupport walks all -cmd YANG modules and builds a map from
// CLI path to ze:task-support value. Paths without the extension are absent.
func PathToTaskSupport(loader *Loader) map[string]string {
	result := make(map[string]string)
	if loader == nil {
		return result
	}
	tree := BuildCommandTree(loader)
	collectTaskSupport(tree, "", result)
	return result
}

func collectTaskSupport(node *command.Node, prefix string, result map[string]string) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if child.TaskSupport != "" {
			result[path] = child.TaskSupport
		}
		collectTaskSupport(child, path, result)
	}
}

// PathToArgDefs walks all -cmd YANG modules and builds a map from CLI path
// to ArgDef slices. Only paths with at least one ArgDef are included.
func PathToArgDefs(loader *Loader) map[string][]command.ArgDef {
	result := make(map[string][]command.ArgDef)
	if loader == nil {
		return result
	}
	tree := BuildCommandTree(loader)
	collectArgDefs(tree, "", result)
	return result
}

func collectArgDefs(node *command.Node, prefix string, result map[string][]command.ArgDef) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if len(child.ArgDefs) > 0 {
			result[path] = child.ArgDefs
		}
		collectArgDefs(child, path, result)
	}
}

// BuildCommandTree walks all -cmd YANG modules in the loader and builds
// a merged command.Node tree. Multiple modules contributing to the same
// container path (e.g., 4 modules defining peer > ...) are merged.
// Only nodes with ze:command get a Description (from the YANG description).
// Grouping containers (no ze:command) become navigation-only branches.
func BuildCommandTree(loader *Loader) *command.Node {
	root := &command.Node{Children: make(map[string]*command.Node)}

	// Collect and sort -cmd module names for deterministic merge order.
	var cmdModules []string
	for _, name := range loader.ModuleNames() {
		if strings.HasSuffix(name, cmdModuleSuffix) {
			cmdModules = append(cmdModules, name)
		}
	}
	sort.Strings(cmdModules)

	for _, name := range cmdModules {
		entry := loader.GetEntry(name)
		if entry == nil || entry.Dir == nil {
			continue
		}
		mergeYANGEntry(root, entry)
	}

	validateOnce.Do(func() { validateNode(root, "") })

	return root
}

var validateOnce sync.Once

// validateCommandTree walks the merged command tree and warns about nodes
// that have no description after all modules have been merged. Every node
// should inherit a description from at least one contributing module.
// Called automatically by BuildCommandTree (once per process).
func validateCommandTree(root *command.Node) {
	validateNode(root, "")
}

func validateNode(node *command.Node, prefix string) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if child.Description == "" {
			slog.Warn("YANG command node missing description", "path", path)
		}
		validateNode(child, path)
	}
}

// mergeYANGEntry recursively walks a YANG entry's children and merges them
// into the command.Node tree. config false containers become tree nodes.
// Nodes with ze:command get their YANG description as the node Description.
func mergeYANGEntry(node *command.Node, entry *gyang.Entry) {
	if entry == nil || entry.Dir == nil {
		return
	}
	for name, child := range entry.Dir {
		// Only walk config false containers (command tree nodes).
		// Note: -cmd.yang files must explicitly mark every container as config false.
		// goyang may not propagate inherited config false to all descendants.
		if child.Config != gyang.TSFalse {
			continue
		}

		if node.Children == nil {
			node.Children = make(map[string]*command.Node)
		}

		target, exists := node.Children[name]
		if !exists {
			target = &command.Node{Name: name}
			node.Children[name] = target
		}

		// ze:command nodes get their WireMethod and description (executable commands).
		// Grouping containers also get their YANG description for help text.
		wm := GetCommandExtension(child)
		switch {
		case wm != "" && target.WireMethod == "":
			target.WireMethod = wm
			target.Description = child.Description
			if ts := getTaskSupportExtension(child); ts != "" {
				target.TaskSupport = ts
			}
		case target.Description == "" && child.Description != "":
			target.Description = child.Description
		case child.Description != "" && target.Description != child.Description:
			slog.Warn("YANG command description mismatch", "node", name, "existing", target.Description, "incoming", child.Description)
		}

		if be := GetBackendExtension(child); be != nil && target.Backend == nil {
			target.Backend = be
		}

		if ee := getEnsureExistsExtension(child); ee != "" && target.EnsureExists == "" {
			target.EnsureExists = ee
		}

		// Extract typed argument definitions from leaf children of ze:command nodes.
		if wm != "" && len(target.ArgDefs) == 0 {
			target.ArgDefs = extractArgDefs(child)
		}

		// A modifier group is not a command, so its leaves are extracted here
		// rather than by the branch above. A container carrying ze:command is a
		// subcommand whatever else it says, which is why the extension is read
		// only when there is no wire method.
		if wm == "" && target.Modifier == command.ModifierNone {
			if modifier, ok := getModifierExtension(child); ok {
				target.Modifier = modifier
				target.ModifierOrder = declaredContainerOrder(entry, name)
				target.ArgDefs = extractArgDefs(child)
			}
		}

		// Recurse into children (merge overlapping branches from multiple modules).
		mergeYANGEntry(target, child)
	}
}

// extractArgDefs reads leaf children of a ze:command YANG entry and converts
// their type metadata into ArgDef entries. Leaves have Config == TSUnset
// (inherited, not explicit), so the main mergeYANGEntry filter skips them.
//
// The order is the one the MODULE declares, because it is the order an operator
// reads in a usage line and a machine reads in the published grammar. goyang
// keeps each entry's parsed statement, and the parser appends substatements in
// lexer order, so the declaration order survives the parse.
//
// A leaf that reaches this entry from a grouping or an augment is named by no
// substatement of this container. Those follow, in name order, so the result is
// deterministic whatever order the entry directory's map yields.
//
// Nothing binds a value BY POSITION in this slice: a positional token goes to
// the definition whose type constrains it most (internal/component/plugin/server,
// positionalDef). That is what makes the order safe to change.
func extractArgDefs(entry *gyang.Entry) []command.ArgDef {
	if entry == nil || entry.Dir == nil {
		return nil
	}

	defs := make([]command.ArgDef, 0, len(entry.Dir))
	taken := make(map[string]bool, len(entry.Dir))

	for _, name := range declaredLeafNames(entry) {
		if taken[name] {
			continue
		}
		def, ok := argDefFor(entry.Dir[name], name)
		if !ok {
			continue
		}
		taken[name] = true
		defs = append(defs, def)
	}

	undeclared := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
		if !taken[name] {
			undeclared = append(undeclared, name)
		}
	}
	sort.Strings(undeclared)

	for _, name := range undeclared {
		if def, ok := argDefFor(entry.Dir[name], name); ok {
			defs = append(defs, def)
		}
	}

	if len(defs) == 0 {
		return nil
	}
	return defs
}

// getModifierExtension reads the ze:modifier extension from a YANG entry and
// answers the occurrence it names.
//
// It answers false for an absent extension and for an argument the vocabulary
// does not hold, so a typo leaves a plain grouping node rather than a group
// nobody declared (ai/rules/evidence.md). The warning names the container,
// because a silent no is what makes a typo cost a reader an afternoon.
func getModifierExtension(entry *gyang.Entry) (command.Modifier, bool) {
	if entry == nil {
		return command.ModifierNone, false
	}
	for _, ext := range entry.Exts {
		if ext.Keyword != "ze:modifier" && !strings.HasSuffix(ext.Keyword, ":modifier") {
			continue
		}
		modifier, ok := command.ParseModifier(ext.Argument)
		if !ok {
			slog.Warn("YANG ze:modifier argument is not a declared occurrence", "node", entry.Name, "argument", ext.Argument)
		}
		return modifier, ok
	}
	return command.ModifierNone, false
}

// declaredContainerOrder answers the position the module declares the named
// container in, counting from 1 among its parent's substatements, and 0 for a
// container the parent's statement does not name.
//
// Node.Children is a map, so a modifier group's declared order is carried on
// the node or it is lost, and two groups would then render in whichever order
// the map yielded.
//
// The loop is bounded by the substatement count of one container in a module
// this binary carries.
func declaredContainerOrder(parent *gyang.Entry, name string) int {
	if parent == nil || parent.Node == nil {
		return 0
	}
	statement := parent.Node.Statement()
	if statement == nil {
		return 0
	}
	position := 0
	for _, sub := range statement.SubStatements() {
		if sub.Keyword != "container" {
			continue
		}
		position++
		if sub.Argument == name {
			return position
		}
	}
	return 0
}

// declaredLeafNames lists the leaf names this container's own statement holds,
// in the order the module declares them. It answers nil for an entry that
// carries no parsed statement, and the caller then falls back to name order.
//
// The loop is bounded by the substatement count of one container in a module
// this binary carries.
func declaredLeafNames(entry *gyang.Entry) []string {
	if entry.Node == nil {
		return nil
	}
	statement := entry.Node.Statement()
	if statement == nil {
		return nil
	}
	subs := statement.SubStatements()
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		if sub.Keyword != "leaf" && sub.Keyword != "leaf-list" {
			continue
		}
		names = append(names, sub.Argument)
	}
	return names
}

// argDefFor converts one leaf entry into an argument definition. It answers
// false for a leaf that declares no type, for a type no command argument can
// carry, and for a name the statement holds while the directory does not.
func argDefFor(leaf *gyang.Entry, name string) (command.ArgDef, bool) {
	if leaf == nil || leaf.Type == nil {
		return command.ArgDef{}, false
	}
	def, ok := yangTypeToArgDef(name, leaf.Type)
	if !ok {
		return command.ArgDef{}, false
	}
	if leaf.Mandatory == gyang.TSTrue {
		def.Mandatory = true
	}
	return def, true
}

// yangTypeToArgDef converts a goyang YangType into an ArgDef.
func yangTypeToArgDef(name string, yt *gyang.YangType) (command.ArgDef, bool) {
	def := command.ArgDef{Name: name}

	//nolint:exhaustive // only handle types relevant to command arguments
	switch yt.Kind {
	case gyang.Yenum:
		def.Kind = command.ArgEnum
		if yt.Enum != nil {
			def.EnumValues = enumNames(yt.Enum)
		}

	case gyang.Yuint8:
		def.Kind = command.ArgUint
		def.UintBits = 8
		applyRange(&def, yt.Range)
	case gyang.Yuint16:
		def.Kind = command.ArgUint
		def.UintBits = 16
		applyRange(&def, yt.Range)
	case gyang.Yuint32:
		def.Kind = command.ArgUint
		def.UintBits = 32
		applyRange(&def, yt.Range)
	case gyang.Yuint64:
		def.Kind = command.ArgUint
		def.UintBits = 64
		applyRange(&def, yt.Range)

	case gyang.Ystring:
		def.Kind = command.ArgString
		if len(yt.Pattern) > 0 {
			compiled, err := compileYANGPattern(yt.Pattern[0])
			if err == nil {
				def.Pattern = compiled
			}
		}

	case gyang.Yunion:
		def.Kind = command.ArgUnion
		for _, member := range yt.Type {
			sub, ok := yangTypeToArgDef(name, member)
			if ok {
				def.UnionDefs = append(def.UnionDefs, sub)
				if sub.Kind == command.ArgEnum {
					def.EnumValues = append(def.EnumValues, sub.EnumValues...)
				}
			}
		}

	default:
		return def, false
	}

	return def, true
}

// enumNames lists a goyang EnumType's value names in the order the module
// declares them.
//
// The order is the enum's own assigned integers, which YANG hands out as
// last+1 when a module states no `value`. So the sort below is declaration
// order for every enum in this repository, and the module's stated order for
// any that numbers its values by hand.
//
// It is the order an operator reads in a generated usage line, and it is what
// makes `[import|export]` come out the way handleShowPolicyChain
// (internal/component/bgp/plugins/cmd/policy/handler.go) documents it. Sorting
// on the NAME renders a set no module chose, which is the same defect
// extractArgDefs above already stopped making for leaves.
func enumNames(enum *gyang.EnumType) []string {
	if enum == nil || len(enum.ToString) == 0 {
		return nil
	}
	values := make([]int64, 0, len(enum.ToString))
	for value := range enum.ToString {
		values = append(values, value)
	}
	slices.Sort(values)
	names := make([]string, 0, len(values))
	for _, value := range values {
		names = append(names, enum.ToString[value])
	}
	return names
}

// applyRange converts each YangRange segment into a UintRange on the ArgDef.
// Supports disjoint ranges (e.g., "1..100 | 200..300").
func applyRange(def *command.ArgDef, r gyang.YangRange) {
	if len(r) == 0 {
		return
	}
	def.Ranges = make([]command.UintRange, len(r))
	for i, seg := range r {
		def.Ranges[i] = command.UintRange{Min: seg.Min.Value, Max: seg.Max.Value}
	}
}

// compileYANGPattern compiles an XSD-style pattern into a Go regexp, anchoring it.
func compileYANGPattern(pattern string) (*regexp.Regexp, error) {
	var tb textbuf.Buffer
	if !strings.HasPrefix(pattern, "^") {
		tb.Byte('^')
	}
	tb.Str(pattern)
	if !strings.HasSuffix(pattern, "$") {
		tb.Byte('$')
	}
	return regexp.Compile(tb.String())
}

// GetCommandExtension reads the ze:command extension from a YANG entry.
// Returns the WireMethod handler string (e.g., "ze-bgp:peer-list"), or empty
// string if the entry has no ze:command extension.
func GetCommandExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:command" || strings.HasSuffix(ext.Keyword, ":command") {
			return ext.Argument
		}
	}
	return ""
}

// hasCommandExtension returns true if the YANG entry has the ze:command extension.
// This marks a config false container as an executable command.
func hasCommandExtension(entry *gyang.Entry) bool {
	return GetCommandExtension(entry) != ""
}

// validTaskSupportValues are the accepted ze:task-support arguments.
var validTaskSupportValues = map[string]bool{
	"optional":  true,
	"required":  true,
	"forbidden": true,
}

// getTaskSupportExtension reads the ze:task-support extension from a YANG entry.
// Returns the level string ("required", "optional", "forbidden"), or empty
// string if absent or if the value is not one of the three valid levels.
func getTaskSupportExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:task-support" || strings.HasSuffix(ext.Keyword, ":task-support") {
			if validTaskSupportValues[ext.Argument] {
				return ext.Argument
			}
			return ""
		}
	}
	return ""
}

// UIResourceEntry holds the MCP Apps ui-resource metadata from YANG extensions.
type UIResourceEntry struct {
	Path        string
	Permissions string
	CSP         string
}

// PathToUIResource walks all -cmd YANG modules and builds a map from
// CLI path to UIResourceEntry. The ze:ui-resource extension can appear on
// grouping containers (not just ze:command nodes).
func PathToUIResource(loader *Loader) map[string]UIResourceEntry {
	result := make(map[string]UIResourceEntry)
	if loader == nil {
		return result
	}

	var cmdModules []string
	for _, name := range loader.ModuleNames() {
		if strings.HasSuffix(name, cmdModuleSuffix) {
			cmdModules = append(cmdModules, name)
		}
	}
	sort.Strings(cmdModules)

	for _, name := range cmdModules {
		entry := loader.GetEntry(name)
		if entry == nil || entry.Dir == nil {
			continue
		}
		collectUIResource(entry, "", result)
	}
	return result
}

func collectUIResource(entry *gyang.Entry, prefix string, result map[string]UIResourceEntry) {
	if entry == nil || entry.Dir == nil {
		return
	}
	for name, child := range entry.Dir {
		if child.Config != gyang.TSFalse {
			continue
		}
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if info := getUIResourceExtensions(child); info.Path != "" {
			result[path] = info
		}
		collectUIResource(child, path, result)
	}
}

func getUIResourceExtensions(entry *gyang.Entry) UIResourceEntry {
	var info UIResourceEntry
	for _, ext := range entry.Exts {
		switch {
		case ext.Keyword == "ze:ui-resource" || strings.HasSuffix(ext.Keyword, ":ui-resource"):
			p := ext.Argument
			if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
				continue
			}
			info.Path = p
		case ext.Keyword == "ze:ui-permissions" || strings.HasSuffix(ext.Keyword, ":ui-permissions"):
			info.Permissions = ext.Argument
		case ext.Keyword == "ze:ui-csp" || strings.HasSuffix(ext.Keyword, ":ui-csp"):
			info.CSP = ext.Argument
		}
	}
	return info
}

// GetBackendExtension reads ze:backend extensions from a YANG entry.
// Returns a deduplicated slice of backend names, or nil if unrestricted.
func GetBackendExtension(entry *gyang.Entry) []string {
	if entry == nil {
		return nil
	}
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

// getEnsureExistsExtension reads the ze:ensure-exists extension from a YANG entry.
// Returns the rollback WireMethod string (e.g., "ze-iface:interface-delete"),
// or empty string if the entry has no ze:ensure-exists extension.
func getEnsureExistsExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:ensure-exists" || strings.HasSuffix(ext.Keyword, ":ensure-exists") {
			return ext.Argument
		}
	}
	return ""
}

// hasEditShortcutExtension returns true if the YANG entry has the ze:edit-shortcut extension.
// This marks a command as available in edit mode as a shortcut (e.g., commit, save).
func hasEditShortcutExtension(entry *gyang.Entry) bool {
	if entry == nil {
		return false
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:edit-shortcut" || strings.HasSuffix(ext.Keyword, ":edit-shortcut") {
			return true
		}
	}
	return false
}
