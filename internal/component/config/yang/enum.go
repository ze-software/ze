// Design: docs/architecture/config/yang-config-design.md -- the model is the one
// declaration of an enumeration's value set.
// Related: validator.go -- the entry walk this file reads an enumeration through.

package yang

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// modelLoader answers the default loader once for the life of the process.
//
// The model is embedded in the binary and registered by init(), so it cannot
// change while the process runs, and parsing every module costs enough that a
// guard calling EnumValues on each config apply would pay it again each time.
var modelLoader = sync.OnceValues(DefaultLoader)

// EnumValues answers every value of the enumeration the model declares at path,
// sorted, or an error naming why the model could not answer.
//
// path names the schema node: the top-level section, then each container, then
// the leaf, as ValidateTree takes it, for example "system/conntrack/module". It
// also reaches a command module, whose first part is the RPC root rather than a
// config section. Every loaded module is tried, in name order, because several
// modules contribute to one section: "environment" is declared by the web, the
// gNMI and the MCP modules, and only one of them holds any given leaf.
//
// This is the route for Go code that needs the SET rather than a mapping. A
// caller that holds its own copy of the values holds a second declaration of
// one fact, and the two drift: an operator then writes a value the model
// accepts and the Go side refuses (`ai/rules/principles.md`).
//
// The error is never traded for an empty slice. A guard that reads its allowed
// set from here refuses everything when the model cannot answer, which is the
// closed direction, and the caller states that in its own error.
//
// MUST NOT be called from an init function: the modules load through init(), so
// a caller that runs during init can see a model that is still being built.
func EnumValues(path string) ([]string, error) {
	loader, err := modelLoader()
	if err != nil {
		return nil, fmt.Errorf("load the YANG model: %w", err)
	}

	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("resolve %q in the YANG model: the path names no section", path)
	}

	// findInEntry is the walk the validator uses for the same paths, so a path
	// this answers for is a path the validator also validates against.
	walk := NewValidator(loader)
	resolved := false
	for _, module := range loader.moduleNames() {
		entry := loader.GetEntry(module)
		if entry == nil || entry.Dir == nil {
			continue
		}
		if _, declares := entry.Dir[parts[0]]; !declares {
			continue
		}
		leaf, err := walk.findInEntry(entry, parts)
		if err != nil {
			continue
		}
		resolved = true
		if leaf.Type == nil || leaf.Type.Kind != gyang.Yenum || leaf.Type.Enum == nil {
			continue
		}
		// Names allocates and sorts a fresh slice on each call, so the caller
		// owns what it gets back.
		names := leaf.Type.Enum.Names()
		if len(names) == 0 {
			return nil, fmt.Errorf("%s declares no enumeration value in the YANG model", path)
		}
		return names, nil
	}

	if resolved {
		return nil, fmt.Errorf("%s is not an enumeration in the YANG model", path)
	}
	return nil, fmt.Errorf("resolve %s in the YANG model: no loaded module declares that path", path)
}

// EnumNamesDeclared answers the names of e in the order the module declares
// them, which is the order of their values.
//
// goyang's Names sorts alphabetically and loses the declaration order, and a
// surface that offers the values to an operator, such as a form dropdown, keeps
// the module's order: the module puts the default first and groups what belongs
// together. A surface that only asks whether a value is a member reads Names.
func EnumNamesDeclared(e *gyang.EnumType) []string {
	values := e.Values()
	slices.Sort(values)
	names := make([]string, len(values))
	for i, value := range values {
		names[i] = e.Name(value)
	}
	return names
}

// moduleNames answers every loaded module once, in name order, so two runs over
// one model read the modules in one order.
func (l *Loader) moduleNames() []string {
	names := l.ModuleNames()
	slices.Sort(names)
	return names
}

// EnumValueSummaries answers the ze:help summary each value of entry's
// enumeration declares, keyed by value name, and nil when entry declares none.
//
// The resolved EnumType keeps only the name and the value, so the summary is
// read off the parse-tree type statements, which still carry every enum
// statement with its extensions. A union is read member by member, because
// value completion offers every enumeration member's values. A typedef
// reference is followed to the typedef's own type statement, whichever scope
// declares it and however many typedefs sit between: goyang resolves the
// reference once at load and leaves that statement at YangType.Base, so this
// reader walks what the resolver found rather than resolving a name again.
// A value declared with no ze:help stays absent: nothing derives a summary
// from any other text.
//
// This is the ONE reader of an enum value's ze:help. The completion row, the
// analysis tree the site reads and the help-shape gate all read through it,
// so the three surfaces cannot disagree about which value carries which text.
func EnumValueSummaries(entry *gyang.Entry) map[string]string {
	if entry == nil {
		return nil
	}
	var declared *gyang.Type
	switch node := entry.Node.(type) {
	case *gyang.Leaf:
		declared = node.Type
	case *gyang.LeafList:
		declared = node.Type
	}
	if declared == nil {
		return nil
	}

	summaries := make(map[string]string)
	collectEnumSummaries(declared, summaries, 0)
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

// EnumValueNames lists the enumeration values a leaf renders: the values of
// its own enumeration type and of every enumeration member of a union, sorted
// and deduplicated, which is the order goyang's EnumType.Names answers in. It
// answers nil for a leaf that is no enumeration and declares no enumeration
// member.
//
// This is the ONE reader of which values a leaf renders. Value completion
// (internal/component/cli, valueCompletions), the analysis tree
// (internal/component/config/yang/cli, yangEnumValues) and the help-shape
// gate (internal/le/docvalid, schemaEnums) all read through it, so the
// population the gate judges is the population the two surfaces render.
func EnumValueNames(entry *gyang.Entry) []string {
	if entry == nil || entry.Type == nil {
		return nil
	}
	var names []string
	for _, member := range append([]*gyang.YangType{entry.Type}, entry.Type.Type...) {
		if member == nil || member.Kind != gyang.Yenum || member.Enum == nil {
			continue
		}
		names = append(names, member.Enum.Names()...)
	}
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// typeDepthMax bounds collectEnumSummaries. The walk follows the model's own
// structure, which goyang resolved before this reader runs, and a typedef
// chain that loops never resolves, so a model that reaches the bound is a
// goyang defect rather than a shape the model can declare.
const typeDepthMax = 16

// collectEnumSummaries puts into summaries the ze:help of every enum statement
// reachable from declared: its own, then each union member's, then those of
// the typedef its name references, which goyang's resolver left at
// YangType.Base as the typedef's type statement
// (vendor/github.com/openconfig/goyang/pkg/yang/types.go, Type.resolve). A
// typedef whose type is a builtin ends the chain with a Base of nil.
//
// The recursion is over the model, an internal structure, and typeDepthMax
// bounds it. A name declared twice keeps the first declaration, which is the
// statement nearest the leaf, as goyang's own resolution does.
func collectEnumSummaries(declared *gyang.Type, summaries map[string]string, depth int) {
	if declared == nil {
		return
	}
	if depth > typeDepthMax {
		return
	}
	for _, value := range declared.Enum {
		if value == nil {
			continue
		}
		summary := GetHelpExtension(value.Extensions)
		if summary == "" {
			continue
		}
		if _, held := summaries[value.Name]; held {
			continue
		}
		summaries[value.Name] = strings.Join(strings.Fields(summary), " ")
	}
	for _, member := range declared.Type {
		collectEnumSummaries(member, summaries, depth+1)
	}
	if declared.YangType == nil {
		return
	}
	collectEnumSummaries(declared.YangType.Base, summaries, depth+1)
}
