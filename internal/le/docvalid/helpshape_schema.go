// Design: docs/architecture/config/yang-config-design.md -- the config schema an operator reads
// Overview: helpshape.go -- the gate this file adds a fourth surface to
// Related: helpshape_baseline.go -- what HEAD already declared
//
// helpshape_schema.go reads the fourth surface a summary reaches an operator
// from: the CONFIG tree. A config node declares its summary as the YANG
// description statement, and `entryDescription`
// (internal/component/cli/completer.go) puts that text on the one-line row
// under the completion menu, exactly as a command node's summary reaches the
// same row.
//
// The other three surfaces cannot see it. `BuildCommandTree` walks the
// `-cmd.yang` modules, `ExtractRPCs` walks the rpc statements, and the offline
// registry holds Go registrations. Some 2,000 config descriptions are outside
// all three, which is why 640 of them were over the render bound with no gate
// saying so (plan/spec-command-help-and-description.md).
//
// The population is the RESOLVED entry tree of the `-conf` modules, which is
// the tree the completer itself walks (`Completer.confModuleNames` then
// `Completer.getEntry`). Deriving it rather than scanning the source text
// settles three questions that no rule over statement keywords answers
// correctly:
//
//   - A module, a submodule, a revision, an import, an include, a grouping, a
//     typedef, an identity, a feature and an extension declare a description
//     that never becomes an entry, so no cap can reach one. Given a brief with
//     no population rule, three agents shortened exactly these statements and
//     moved the prose into `//` comments, which is a downgrade: a YANG
//     description is schema that standard tooling reads and the schema output
//     publishes, and a comment is neither. All three passes were reverted.
//   - A leaf reaches an operator only where it lands in the config tree. One
//     `grouping` in `ze-types` supplies both an rpc payload and a config node,
//     and only the second renders. A `-cmd` or `-api` leaf becomes a
//     command.ArgDef, which holds no text field, so its description is dropped
//     at the tree boundary and reaches nobody.
//   - A node another module AUGMENTS in is in the tree, so `ze-role` and the
//     other BGP plugin modules are judged under the module they augment,
//     without this file knowing they exist.

package docvalid

import (
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// collectSchema judges the summary of every config node the loader holds.
//
// The modules come from the loader rather than from the checkout, for the
// reason every other surface takes them from there: the loader holds exactly
// what this binary carries, so a fixture module reaches the walk by being added
// to a loader, and a `.yang` file under a testdata directory never does.
func collectSchema(loader *yang.Loader, report *HelpShapeReport) {
	if loader == nil {
		return
	}
	names := loader.ConfModuleNames()
	sort.Strings(names)

	for _, name := range names {
		module := loader.GetEntry(name)
		if module == nil {
			continue
		}
		// The module entry itself carries the MODULE description, which no row
		// renders, so the walk starts at its children.
		walkSchema(module, name, nil, report, map[*gyang.Entry]bool{})
	}
}

// walkSchema judges every config node under one entry.
//
// The tree is finite and acyclic once goyang has expanded every `uses`, but the
// visited set is kept all the same: a recursive grouping that slipped past
// goyang would otherwise hang the gate rather than report on it
// (docs/contributing/ze-go-style.md, "A limit on everything").
func walkSchema(entry *gyang.Entry, module string, path []string,
	report *HelpShapeReport, seen map[*gyang.Entry]bool,
) {
	if entry == nil || seen[entry] {
		return
	}
	seen[entry] = true

	names := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		child := entry.Dir[name]
		if child == nil {
			continue
		}
		// An rpc, and everything under it, is judged by collectRPCs, which
		// walks EVERY module for it. Judging it here would refuse one
		// declaration twice.
		if child.RPC != nil {
			continue
		}
		below := append(append([]string(nil), path...), name)
		report.schema(schemaLabel(module, below), child)
		report.schemaEnums(module, below, child)
		walkSchema(child, module, below, report, seen)
	}
}

// schemaLabel names the declaration a refusal belongs to:
// `<module>:<node>/<node>/<name>`, which is the path an operator types and the
// path an author walks down the file to reach.
func schemaLabel(module string, path []string) string {
	var tb textbuf.Buffer
	tb.Str(module).Byte(':')
	for index, name := range path {
		if index > 0 {
			tb.Byte('/')
		}
		tb.Str(name)
	}
	return tb.String()
}

// schema judges one config node's two texts and counts it.
func (r *HelpShapeReport) schema(label string, entry *gyang.Entry) {
	long := yang.GetHelpExtension(entry.Exts)

	r.Schema++
	if strings.TrimSpace(long) != "" {
		r.SchemaWithHelp++
	}
	if strings.TrimSpace(entry.Description) == "" {
		return
	}
	r.SchemaWithSummary++

	// The five shape rules a COMMAND summary is held to are not applied here. A
	// YANG description is written over as many lines as its author needed, and
	// `entryDescription` collapses the whitespace before it renders, so a
	// newline in one is the normal spelling rather than a defect. What this
	// spec brings the config tree under is the two caps and the pair
	// (plan/spec-command-help-and-description.md, D-4).
	r.judgeCaps(surfaceSchema, label, entry.Description)
	r.judgePair(surfaceSchema, label, entry.Description, long)
}

// schemaEnums judges the values of a list whose key is an enumeration.
//
// This is the ONE shape in which an enum's description reaches an operator, and
// the walk mirrors the two producers statement for statement.
// `listKeyCompletions` is the only caller of `enumKeyVocabulary`, and the entry
// it hands over comes from `getListKeyEntry`, which answers
// `listEntry.Dir[listEntry.Key]` and nil for everything else
// (internal/component/cli/completer.go, completer_validate.go). So an
// enumeration on an ordinary leaf renders nowhere, and one reached through a
// typedef has no enum statement on the leaf's own node at all. A cap on either
// would report a defect that does not exist, which is the repair that cost
// three earlier passes.
//
// None of the 278 enum descriptions in the corpus keys a list today, so this
// rule refuses nothing over the checkout. It is written for the
// enumeration-keyed list a later spec adds, and it is written NARROW because a
// gate that judges the wrong population is the failure this scoping exists to
// prevent.
//
// An enum is never asked for a long text: nothing anywhere reads a ze:help on
// one, so demanding it would demand a declaration no surface prints.
func (r *HelpShapeReport) schemaEnums(module string, path []string, list *gyang.Entry) {
	if !list.IsList() || list.Key == "" {
		return
	}
	key, held := list.Dir[list.Key]
	if !held || key == nil || key.Type == nil || key.Type.Kind != gyang.Yenum {
		return
	}
	leaf, ok := key.Node.(*gyang.Leaf)
	if !ok || leaf.Type == nil {
		return
	}

	for _, declared := range leaf.Type.Enum {
		if declared == nil || declared.Description == nil {
			continue
		}
		summary := declared.Description.Name
		if strings.TrimSpace(summary) == "" {
			continue
		}
		below := append(append([]string(nil), path...), list.Key, declared.Name)
		r.Schema++
		r.SchemaWithSummary++
		r.judgeCaps(surfaceSchema, schemaLabel(module, below), summary)
	}
}
