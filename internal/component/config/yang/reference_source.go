// Design: docs/architecture/config/yang-config-design.md — the ze:reference extension
// Related: validator_instance.go — the reference walk that checks and completes a ze:reference leaf

package yang

import (
	"fmt"
	"slices"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ReferenceSource answers the names a ze:reference leaf may hold at one
// position of the config data tree. root is the whole data tree as
// Tree.ToPluginMap lowers it, and at is the data path of the referring leaf
// (referenceDataPath): section, container and list names, each list entry's
// map key after the name of its list, and the leaf's own name last.
//
// A source MUST NOT modify root, and MUST be safe for concurrent use: editor
// completion of several sessions and a commit can ask it at once. The order of
// the names it answers does not matter, and duplicates are dropped.
type ReferenceSource func(root map[string]any, at []string) []string

// referenceSources holds the sources RegisterReferenceSource added, by name.
// Written during init(), read-only after -- no mutex needed.
var referenceSources = map[string]ReferenceSource{}

// RegisterReferenceSource adds the source a `ze:reference "<name>"` leaf
// reads. Called during init() by the component that owns the names, so the
// config package imports no component. Completion offers exactly the names the
// source answers, and commit refuses any other value
// (Validator.ValidateReferences), so the source is both lists.
func RegisterReferenceSource(name string, source ReferenceSource) {
	if name == "" {
		panic("BUG: a ze:reference source was registered with an empty name")
	}
	if source == nil {
		panic("BUG: ze:reference source " + name + " was registered as nil")
	}
	if _, taken := referenceSources[name]; taken {
		panic("BUG: ze:reference source " + name + " was registered twice")
	}
	referenceSources[name] = source
}

// GetReferenceExtension reads the ze:reference argument of a YANG entry, the
// name of the source its value must be one of. It answers the empty string
// for an entry with no ze:reference.
func GetReferenceExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	for _, ext := range entry.Exts {
		if strings.HasSuffix(ext.Keyword, ":reference") {
			return ext.Argument
		}
	}
	return ""
}

// CheckAllReferenceSourcesRegistered reports every ze:reference argument in
// the schema that names no registered source. Such a leaf refuses every value
// at commit, because no list exists to accept one.
func CheckAllReferenceSourcesRegistered(schema *Resolved) error {
	missing := make(map[string]bool)
	for _, moduleName := range schema.ModuleNames() {
		if entry := schema.GetEntry(moduleName); entry != nil {
			collectMissingReferenceSources(entry, missing)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	slices.Sort(names)
	return fmt.Errorf("ze:reference names sources no component registered: %s", textbuf.Join(names, ", "))
}

// collectMissingReferenceSources records the unregistered ze:reference
// sources under entry. The recursion is bounded by the schema depth, which Ze
// builds from its own modules.
func collectMissingReferenceSources(entry *gyang.Entry, missing map[string]bool) {
	if name := GetReferenceExtension(entry); name != "" {
		if _, ok := referenceSources[name]; !ok {
			missing[name] = true
		}
	}
	for _, child := range entry.Dir {
		collectMissingReferenceSources(child, missing)
	}
}

// sourceKey names the names one source answers for one leaf read under one
// data node, the parent of the leaf, by its number in the walk.
type sourceKey struct {
	parent int
	leaf   *gyang.Entry
}

// sourceInstances answers the sorted names the ze:reference source called
// name lists for leaf, whose parent closes frames, asked once per parent.
func (w *referenceWalk) sourceInstances(leaf *gyang.Entry, name string, frames []dataNode) ([]string, error) {
	key := sourceKey{parent: frames[len(frames)-1].id, leaf: leaf}
	if names, ok := w.sources[key]; ok {
		return names, nil
	}
	source, ok := referenceSources[name]
	if !ok {
		return nil, fmt.Errorf("%s names the reference source %q, which no component in this build registers, "+
			"so no value can be accepted: build ze with the component that owns those names, or remove the leaf",
			leaf.Name, name)
	}
	names := slices.Compact(slices.Sorted(slices.Values(source(frames[0].data, referenceDataPath(frames, leaf)))))
	w.sources[key] = names
	return names, nil
}

// checkSourceInstance checks that value is one of the names the ze:reference
// source of leaf lists.
func (w *referenceWalk) checkSourceInstance(path string, leaf *gyang.Entry, name, value string, frames []dataNode) error {
	names, err := w.sourceInstances(leaf, name, frames)
	if err != nil {
		return &ValidationError{Path: path, Type: ErrTypeReference, Message: err.Error(), Expected: name, Got: value}
	}
	if slices.Contains(names, value) {
		return nil
	}
	return &ValidationError{Path: path, Type: ErrTypeReference, Message: absentSourceMessage(leaf, value, names), Expected: name, Got: value}
}

// absentSourceMessage says which value names nothing, and what it may name:
// one of the configured choices, or, when there is none, that one must be
// configured first. It is the shape absentLeafrefMessage gives a leafref.
func absentSourceMessage(leaf *gyang.Entry, value string, names []string) string {
	var tb textbuf.Buffer
	tb.Str("no ").Str(leaf.Name).Str(" ").Quoted(value).Str(" is configured")
	if len(names) == 0 {
		return tb.Str(", and none is configured: configure the entry ").Str(leaf.Name).Str(" names first, or correct the value").String()
	}
	writeChoices(&tb, names)
	return tb.String()
}

// writeChoices appends the configured choices a refusal names, sorted names
// at most referenceChoicesMax of them, and how many more exist.
func writeChoices(tb *textbuf.Buffer, names []string) {
	shown := names[:min(len(names), referenceChoicesMax)]
	tb.Str(": name one of ").Quoted(strings.Join(shown, " "))
	if len(names) > len(shown) {
		tb.Str(" (").Int(int64(len(names) - len(shown))).Str(" more configured)")
	}
}

// referenceDataPath answers the data path of leaf, whose parent closes
// frames: each node's name below the root, a list entry's map key after it,
// and the leaf's name last. It is the leafPath ReferenceInstances takes.
func referenceDataPath(frames []dataNode, leaf *gyang.Entry) []string {
	at := make([]string, 0, 2*len(frames))
	for _, node := range frames[1:] {
		at = append(at, node.entries[0].Name)
		if node.entries[0].IsList() {
			at = append(at, node.key)
		}
	}
	return append(at, leaf.Name)
}
