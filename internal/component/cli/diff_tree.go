// Design: docs/architecture/config/yang-config-design.md — tree-aware config diff
// Related: diff.go — line-based LCS diff (fallback)
// Related: model_render.go — viewport rendering with gutter markers

package cli

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// Boolean string constants used in config presence/flex nodes.
const (
	boolTrue  = "true"
	boolFalse = "false"
)

// nodeWalker provides schema-ordered iteration over child nodes.
// Satisfied by *config.Schema, *config.ContainerNode, *config.ListNode, *config.FlexNode, *config.InlineListNode.
type nodeWalker interface {
	Children() []string
	Get(name string) config.Node
}

// computeTreeAnnotatedDiff compares two config strings using the schema tree structure.
// Returns annotated diff lines where structural boundaries (closing braces) are correctly
// associated with their container, solving the LCS misalignment problem.
func computeTreeAnnotatedDiff(original, modified string, schema *config.Schema) ([]diffLine, error) {
	parser := config.NewParser(schema)

	origTree, err := parser.Parse(original)
	if err != nil {
		return nil, err
	}
	modTree, err := parser.Parse(modified)
	if err != nil {
		return nil, err
	}

	var lines []diffLine
	diffWalkChildren(&lines, origTree, modTree, schema, 0)
	return lines, nil
}

// annotateContentWithTreeDiff compares two config strings using tree-aware diff.
// Falls back to line-based diff on parse error.
func annotateContentWithTreeDiff(original, modified string, schema *config.Schema) (string, map[int]int) {
	if original == modified {
		return modified, nil
	}

	diffs, err := computeTreeAnnotatedDiff(original, modified, schema)
	if err != nil {
		// Fall back to line-based diff on parse error
		return annotateContentWithGutter(original, modified)
	}

	if len(diffs) == 0 {
		return modified, nil
	}

	return renderDiffLines(diffs)
}

// renderDiffLines converts diff lines into an annotated string with line mapping.
func renderDiffLines(diffs []diffLine) (string, map[int]int) {
	var b strings.Builder
	lineMapping := make(map[int]int)
	displayLine := 0
	workingLine := 0

	for _, dl := range diffs {
		if displayLine > 0 {
			b.WriteByte('\n')
		}
		displayLine++

		b.WriteByte(byte(dl.Marker))
		b.WriteByte(' ')
		b.WriteString(dl.Text)

		switch dl.Marker {
		case diffUnchanged, diffAdded, diffModified:
			workingLine++
			lineMapping[displayLine] = workingLine
		case diffRemoved: // removed lines have no working content counterpart
		}
	}

	return b.String(), lineMapping
}

// diffWalkChildren walks children in schema order, comparing two trees.
func diffWalkChildren(lines *[]diffLine, orig, mod *config.Tree, walker nodeWalker, indent int) {
	for _, name := range walker.Children() {
		child := walker.Get(name)
		diffWalkNode(lines, orig, mod, name, child, indent)
	}
	diffExtraTreeValues(lines, orig, mod, walker.Children(), indent)
}

// diffWalkNode dispatches a single node comparison by schema type.
func diffWalkNode(lines *[]diffLine, orig, mod *config.Tree, name string, node config.Node, indent int) {
	prefix := strings.Repeat("\t", indent)

	switch n := node.(type) {
	case *config.LeafNode:
		diffLeaf(lines, orig, mod, name, prefix)

	case *config.MultiLeafNode:
		diffMultiLeaf(lines, orig, mod, name, prefix)

	case *config.BracketLeafListNode:
		diffBracketLeaf(lines, orig, mod, name, prefix)

	case *config.ValueOrArrayNode:
		diffValueOrArray(lines, orig, mod, name, prefix)

	case *config.ContainerNode:
		if n.Presence {
			diffPresenceContainer(lines, orig, mod, name, n, indent)
		} else {
			diffContainer(lines, orig, mod, name, n, indent)
		}

	case *config.ListNode:
		diffList(lines, orig, mod, name, n, indent)

	case *config.FlexNode:
		diffFlex(lines, orig, mod, name, n, indent)

	case *config.FreeformNode, *config.InlineListNode: // unsupported — text fallback
		diffNodeFallback(lines, orig, mod, name, node, indent)
	}
}

// --- Leaf-type diffs ---

func diffLeaf(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origVal, origOk := orig.Get(name)
	modVal, modOk := mod.Get(name)

	origFmt := formatLeafValue(origVal)
	modFmt := formatLeafValue(modVal)

	switch {
	case origOk && modOk && origFmt == modFmt:
		*lines = append(*lines, diffLine{diffUnchanged, prefix + name + " " + modFmt})
	case origOk && modOk:
		*lines = append(*lines, diffLine{diffModified, prefix + name + " " + modFmt})
	case origOk:
		*lines = append(*lines, diffLine{diffRemoved, prefix + name + " " + origFmt})
	case modOk:
		*lines = append(*lines, diffLine{diffAdded, prefix + name + " " + modFmt})
	}
}

func diffMultiLeaf(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origVal, origOk := orig.Get(name)
	modVal, modOk := mod.Get(name)

	switch {
	case origOk && modOk && origVal == modVal:
		*lines = append(*lines, diffLine{diffUnchanged, prefix + name + " " + origVal})
	case origOk && modOk:
		*lines = append(*lines, diffLine{diffModified, prefix + name + " " + modVal})
	case origOk:
		*lines = append(*lines, diffLine{diffRemoved, prefix + name + " " + origVal})
	case modOk:
		*lines = append(*lines, diffLine{diffAdded, prefix + name + " " + modVal})
	}
}

func diffBracketLeaf(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origVal, origOk := orig.Get(name)
	modVal, modOk := mod.Get(name)

	fmtBracket := func(v string) string { return "[ " + v + " ]" }

	switch {
	case origOk && modOk && origVal == modVal:
		*lines = append(*lines, diffLine{diffUnchanged, prefix + name + " " + fmtBracket(origVal)})
	case origOk && modOk:
		*lines = append(*lines, diffLine{diffModified, prefix + name + " " + fmtBracket(modVal)})
	case origOk:
		*lines = append(*lines, diffLine{diffRemoved, prefix + name + " " + fmtBracket(origVal)})
	case modOk:
		*lines = append(*lines, diffLine{diffAdded, prefix + name + " " + fmtBracket(modVal)})
	}
}

func diffValueOrArray(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origItems := orig.GetSlice(name)
	modItems := mod.GetSlice(name)

	origText := formatSlice(origItems)
	modText := formatSlice(modItems)

	switch {
	case origText != "" && modText != "" && origText == modText:
		*lines = append(*lines, diffLine{diffUnchanged, prefix + name + " " + origText})
	case origText != "" && modText != "":
		*lines = append(*lines, diffLine{diffModified, prefix + name + " " + modText})
	case origText != "":
		*lines = append(*lines, diffLine{diffRemoved, prefix + name + " " + origText})
	case modText != "":
		*lines = append(*lines, diffLine{diffAdded, prefix + name + " " + modText})
	}
}

// --- Container diff ---

func diffContainer(lines *[]diffLine, orig, mod *config.Tree, name string, node *config.ContainerNode, indent int) {
	prefix := strings.Repeat("\t", indent)
	origChild := orig.GetContainer(name)
	modChild := mod.GetContainer(name)

	switch {
	case origChild == nil && modChild == nil:
		return
	case origChild == nil:
		emitContainerMarked(lines, name, modChild, node, diffAdded, indent)
	case modChild == nil:
		emitContainerMarked(lines, name, origChild, node, diffRemoved, indent)
	default: // both present
		*lines = append(*lines, diffLine{diffUnchanged, prefix + name + " {"})
		diffWalkChildren(lines, origChild, modChild, node, indent+1)
		*lines = append(*lines, diffLine{diffUnchanged, prefix + "}"})
	}
}

func diffPresenceContainer(lines *[]diffLine, orig, mod *config.Tree, name string, node *config.ContainerNode, indent int) {
	prefix := strings.Repeat("\t", indent)
	origVal, origValOk := orig.Get(name)
	modVal, modValOk := mod.Get(name)
	origChild := orig.GetContainer(name)
	modChild := mod.GetContainer(name)

	// Simple value form
	if (origValOk || modValOk) && origChild == nil && modChild == nil {
		origFmt := formatPresenceValue(name, origVal, origValOk)
		modFmt := formatPresenceValue(name, modVal, modValOk)
		switch {
		case origFmt == modFmt:
			*lines = append(*lines, diffLine{diffUnchanged, prefix + origFmt})
		case origFmt == "":
			*lines = append(*lines, diffLine{diffAdded, prefix + modFmt})
		case modFmt == "":
			*lines = append(*lines, diffLine{diffRemoved, prefix + origFmt})
		case origFmt != modFmt:
			*lines = append(*lines, diffLine{diffModified, prefix + modFmt})
		}
		return
	}

	// Block form — delegate to container diff
	diffContainer(lines, orig, mod, name, node, indent)
}

// --- List diff ---

func diffList(lines *[]diffLine, orig, mod *config.Tree, name string, node *config.ListNode, indent int) {
	origEntries := orig.GetList(name)
	modEntries := mod.GetList(name)

	if len(origEntries) == 0 && len(modEntries) == 0 {
		return
	}

	// Collect all keys from both sides, sorted for deterministic output
	keySet := make(map[string]bool)
	for k := range origEntries {
		keySet[k] = true
	}
	for k := range modEntries {
		keySet[k] = true
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	prefix := strings.Repeat("\t", indent)

	for _, key := range keys {
		origEntry := origEntries[key]
		modEntry := modEntries[key]
		displayKey := config.StripListKeySuffix(key)

		// Build opening line
		var opening string
		if displayKey == config.KeyDefault {
			opening = name + " {"
		} else {
			opening = name + " " + diffQuoteIfNeeded(displayKey) + " {"
		}

		switch {
		case origEntry == nil && modEntry != nil:
			// Entire entry is new
			*lines = append(*lines, diffLine{diffAdded, prefix + opening})
			emitEntryChildrenMarked(lines, modEntry, node, diffAdded, indent+1)
			*lines = append(*lines, diffLine{diffAdded, prefix + "}"})
		case origEntry != nil && modEntry == nil:
			// Entire entry was removed
			*lines = append(*lines, diffLine{diffRemoved, prefix + opening})
			emitEntryChildrenMarked(lines, origEntry, node, diffRemoved, indent+1)
			*lines = append(*lines, diffLine{diffRemoved, prefix + "}"})
		case origEntry != nil && modEntry != nil:
			// Entry exists in both — recurse
			*lines = append(*lines, diffLine{diffUnchanged, prefix + opening})
			diffWalkChildren(lines, origEntry, modEntry, node, indent+1)
			*lines = append(*lines, diffLine{diffUnchanged, prefix + "}"})
		}
	}
}

// --- Flex node diff ---

func diffFlex(lines *[]diffLine, orig, mod *config.Tree, name string, node *config.FlexNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Value form
	origVal, origOk := orig.Get(name)
	modVal, modOk := mod.Get(name)
	if origOk || modOk {
		origFmt := formatFlexValue(name, origVal, origOk)
		modFmt := formatFlexValue(name, modVal, modOk)
		switch {
		case origFmt == modFmt:
			*lines = append(*lines, diffLine{diffUnchanged, prefix + origFmt})
		case origFmt == "":
			*lines = append(*lines, diffLine{diffAdded, prefix + modFmt})
		case modFmt == "":
			*lines = append(*lines, diffLine{diffRemoved, prefix + origFmt})
		case origFmt != modFmt:
			*lines = append(*lines, diffLine{diffModified, prefix + modFmt})
		}
	}

	// Container form
	origChild := orig.GetContainer(name)
	modChild := mod.GetContainer(name)
	if origChild != nil || modChild != nil {
		switch {
		case origChild == nil:
			*lines = append(*lines, diffLine{diffAdded, prefix + name + " {"})
			emitFlexChildrenMarked(lines, modChild, node, diffAdded, indent+1)
			*lines = append(*lines, diffLine{diffAdded, prefix + "}"})
		case modChild == nil:
			*lines = append(*lines, diffLine{diffRemoved, prefix + name + " {"})
			emitFlexChildrenMarked(lines, origChild, node, diffRemoved, indent+1)
			*lines = append(*lines, diffLine{diffRemoved, prefix + "}"})
		default: // both present
			*lines = append(*lines, diffLine{diffUnchanged, prefix + name + " {"})
			diffWalkChildren(lines, origChild, modChild, node, indent+1)
			*lines = append(*lines, diffLine{diffUnchanged, prefix + "}"})
		}
	}
}

// --- Fallback for unsupported node types ---

func diffNodeFallback(lines *[]diffLine, orig, mod *config.Tree, name string, node config.Node, indent int) {
	origText := serializeNodeText(orig, name, node, indent)
	modText := serializeNodeText(mod, name, node, indent)

	switch {
	case origText == modText:
		emitTextLines(lines, origText, diffUnchanged)
	case origText == "":
		emitTextLines(lines, modText, diffAdded)
	case modText == "":
		emitTextLines(lines, origText, diffRemoved)
	case origText != modText:
		emitTextLines(lines, origText, diffRemoved)
		emitTextLines(lines, modText, diffAdded)
	}
}

// --- Helpers ---

// emitContainerMarked emits an entire container block with a single marker.
func emitContainerMarked(lines *[]diffLine, name string, tree *config.Tree, node *config.ContainerNode, marker diffMarker, indent int) {
	prefix := strings.Repeat("\t", indent)
	*lines = append(*lines, diffLine{marker, prefix + name + " {"})
	emitReindentedLines(lines, config.SerializeSubtree(tree, node), marker, indent+1)
	*lines = append(*lines, diffLine{marker, prefix + "}"})
}

// emitEntryChildrenMarked serializes children of a list entry and marks all lines.
func emitEntryChildrenMarked(lines *[]diffLine, tree *config.Tree, node *config.ListNode, marker diffMarker, indent int) {
	emitReindentedLines(lines, config.SerializeSubtree(tree, node), marker, indent)
}

// emitFlexChildrenMarked serializes children of a flex node and marks all lines.
func emitFlexChildrenMarked(lines *[]diffLine, tree *config.Tree, node *config.FlexNode, marker diffMarker, indent int) {
	emitReindentedLines(lines, config.SerializeSubtree(tree, node), marker, indent)
}

// emitReindentedLines splits text into lines, re-indents from indent 0 to target, and emits with marker.
func emitReindentedLines(lines *[]diffLine, text string, marker diffMarker, indent int) {
	prefix := strings.Repeat("\t", indent)
	for _, line := range splitDiffLines(text) {
		*lines = append(*lines, diffLine{marker, prefix + line})
	}
}

// emitTextLines splits text into lines and emits each with the given marker.
func emitTextLines(lines *[]diffLine, text string, marker diffMarker) {
	for _, line := range splitDiffLines(text) {
		*lines = append(*lines, diffLine{marker, line})
	}
}

// diffExtraTreeValues compares values not in schema children list.
func diffExtraTreeValues(lines *[]diffLine, orig, mod *config.Tree, children []string, indent int) {
	prefix := strings.Repeat("\t", indent)
	schemaNames := make(map[string]bool, len(children))
	for _, name := range children {
		schemaNames[name] = true
	}

	// Collect extra keys from both trees
	allExtras := make(map[string]bool)
	for _, k := range orig.Values() {
		if !schemaNames[k] {
			allExtras[k] = true
		}
	}
	for _, k := range mod.Values() {
		if !schemaNames[k] {
			allExtras[k] = true
		}
	}

	keys := make([]string, 0, len(allExtras))
	for k := range allExtras {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		origVal, origOk := orig.Get(k)
		modVal, modOk := mod.Get(k)

		origFmt := diffQuoteIfNeeded(origVal)
		modFmt := diffQuoteIfNeeded(modVal)

		switch {
		case origOk && modOk && origFmt == modFmt:
			*lines = append(*lines, diffLine{diffUnchanged, prefix + k + " " + origFmt})
		case origOk && modOk:
			*lines = append(*lines, diffLine{diffModified, prefix + k + " " + modFmt})
		case origOk:
			*lines = append(*lines, diffLine{diffRemoved, prefix + k + " " + origFmt})
		case modOk:
			*lines = append(*lines, diffLine{diffAdded, prefix + k + " " + modFmt})
		}
	}
}

// serializeNodeText serializes a single node from a tree for fallback comparison.
func serializeNodeText(tree *config.Tree, name string, node config.Node, indent int) string {
	prefix := strings.Repeat("\t", indent)

	// For leaf-like nodes, check if value exists
	if v, ok := tree.Get(name); ok {
		return prefix + name + " " + diffQuoteIfNeeded(v)
	}

	// For container-like nodes, check if container exists
	if child := tree.GetContainer(name); child != nil {
		inner := config.SerializeSubtree(child, node)
		if inner == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString(prefix + name + " {\n")
		for _, line := range splitDiffLines(inner) {
			b.WriteString(strings.Repeat("\t", indent+1) + line + "\n")
		}
		b.WriteString(prefix + "}")
		return b.String()
	}

	return ""
}

// --- Formatting helpers (mirror config.quoteIfNeeded / normalizeBool) ---

func formatLeafValue(v string) string {
	return diffQuoteIfNeeded(diffNormalizeBool(v))
}

func formatPresenceValue(name, v string, ok bool) string {
	if !ok {
		return ""
	}
	if v == boolTrue {
		return name
	}
	return name + " " + diffQuoteIfNeeded(v)
}

func formatFlexValue(name, v string, ok bool) string {
	if !ok {
		return ""
	}
	if v == boolTrue {
		return name
	}
	return name + " " + diffQuoteIfNeeded(v)
}

func formatSlice(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return diffQuoteIfNeeded(items[0])
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = diffQuoteIfNeeded(item)
	}
	return "[ " + strings.Join(parts, " ") + " ]"
}

func diffNormalizeBool(v string) string {
	if v == boolTrue {
		return "enable"
	}
	if v == boolFalse {
		return "disable"
	}
	return v
}

func diffQuoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	needsQuote := false
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' || c == '\'' || c == '{' || c == '}' || c == ';' || c == '#' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default: // non-special character
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
