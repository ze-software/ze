// Design: docs/architecture/config/yang-config-design.md -- the config schema an operator reads
// Overview: helpshape.go -- the gate this file adds a fourth surface to
// Related: helpshape.go -- judgeCaps and judgePair, the judges this surface calls
//
// helpshape_schema.go reads the fourth surface a summary reaches an operator
// from: the CONFIG tree. A config node declares its summary as the ze:help
// extension and its long explanation as the YANG description statement, and
// `entryDescription` (internal/component/cli/completer.go) puts the summary on
// the one-line row under the completion menu, exactly as a command node's
// summary reaches the same row.
//
// The other three surfaces cannot see it. `BuildCommandTree` walks the
// `-cmd.yang` modules, `ExtractRPCs` walks the rpc statements, and the offline
// registry holds Go registrations. Some 2,000 config summaries are outside
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
//     and only the second renders here. A `-cmd` leaf is a command argument,
//     judged under the two caps by `arguments` (helpshape.go) off the command
//     tree, where `argDefFor` carries its texts; an `-api` leaf is an rpc or
//     notification leaf, judged by `leaves` off `ExtractRPCs` and
//     `ExtractNotifications`. Neither is judged twice.
//   - An enumeration leaf's values render on the value completion rows with
//     the ze:help each value declares (`valueCompletions`), so every one of
//     them is judged by `schemaEnums` under the two caps.
//   - A node another module AUGMENTS in is in the tree, so `ze-role` and the
//     other BGP plugin modules are judged under the module they augment,
//     without this file knowing they exist.

package docvalid

import (
	"slices"
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
	slices.Sort(names)

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
	slices.Sort(names)

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
		// A choice and a case are schema structure, not a node an operator
		// types. `effectiveChildren` (internal/component/cli/completer.go)
		// walks THROUGH both and never emits either as a completion row, so
		// neither text ever renders and a cap or a missing rule over one would
		// report a defect that does not exist. The walk mirrors that reader
		// statement for statement: descend, and judge what it emits.
		if child.IsChoice() || child.IsCase() {
			walkSchema(child, module, path, report, seen)
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

// schema judges one config node's two texts and counts it. The summary is the
// ze:help extension and the long explanation is the YANG description.
func (r *HelpShapeReport) schema(label string, entry *gyang.Entry) {
	summary := yang.GetHelpExtension(entry.Exts)
	long := entry.Description

	r.Schema++
	if strings.TrimSpace(long) != "" {
		r.SchemaWithHelp++
	}
	if strings.TrimSpace(summary) == "" {
		// A config node with no ze:help renders an empty row under the
		// completion menu, which tells an operator the name exists and nothing
		// about what it does. Counting it as coverage owed and saying nothing
		// is the silent answer this gate exists to remove
		// (ai/rules/principles.md).
		r.refuse(surfaceSchema, label, ruleMissingSummary,
			"the config node declares no ze:help summary", "")
		return
	}
	r.SchemaWithSummary++

	// The five shape rules a COMMAND summary is held to are not applied here. A
	// ze:help argument is written over as many lines as its author needed, and
	// `entryDescription` collapses the whitespace before it renders, so a
	// newline in one is the normal spelling rather than a defect. What this
	// spec brings the config tree under is the two caps and the pair
	// (plan/spec-command-help-and-description.md, D-4).
	r.judgeCaps(surfaceSchema, label, summary)
	r.judgePair(surfaceSchema, label, summary, long)
}

// schemaEnums judges the values of an enumeration leaf, which are the values
// value completion renders: `valueCompletions`
// (internal/component/cli/completer.go) puts every value of an enumeration
// leaf, and of a union's enumeration members, on a completion row with the
// ze:help summary the value declares, and `listKeyCompletions` does the same
// for a list key. Both read the values through `yang.EnumValueNames` and the
// summaries through `yang.EnumValueSummaries`, and so does this walk, so the
// population judged is the population rendered.
//
// Every rendered value is counted. A value that declares a summary takes the
// two caps, because the row is one line. A value that declares none is
// counted and not refused: the row then carries an empty summary, and the
// count is what tells an author how much of the corpus is still unwritten.
//
// An enum is never asked for a long text: nothing anywhere reads a
// description on one, so demanding it would demand a declaration no surface
// prints.
func (r *HelpShapeReport) schemaEnums(module string, path []string, entry *gyang.Entry) {
	names := yang.EnumValueNames(entry)
	if len(names) == 0 {
		return
	}
	summaries := yang.EnumValueSummaries(entry)
	for _, name := range names {
		r.SchemaEnumValues++
		summary := summaries[name]
		if strings.TrimSpace(summary) == "" {
			continue
		}
		r.SchemaEnumValuesWithSummary++
		below := append(append([]string(nil), path...), name)
		r.judgeCaps(surfaceSchema, schemaLabel(module, below), summary)
	}
}
