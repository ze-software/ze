// Design: docs/architecture/config/yang-config-design.md — tree-aware config diff
// Related: diff.go — line-based LCS diff (fallback)
// Related: model_render.go — viewport rendering with gutter markers

package cli

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
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

	// Mask every secret leaf on both sides. The diff then renders the
	// placeholder rather than the stored value. An unchanged secret is not
	// flagged as changed. Idempotent when the inputs were masked upstream.
	//
	// A CHANGED secret reads as the same placeholder on each side, so this diff
	// cannot see it. Editor.changedSecretsAt names it instead, and
	// setViewportData writes that line (model_render.go). Recovering it here is
	// not possible: by the time this function has text, both values are gone.
	origTree = config.MaskSecrets(origTree, schema)
	modTree = config.MaskSecrets(modTree, schema)

	// A comment is not configuration, so no comment makes a line changed. The
	// walk takes its structure from the modified tree WITH its comments, the
	// tree Serialize printed the working content from: a comment inside a
	// container makes Serialize print it as a block rather than inline, and the
	// diff must print the same statement lines for renderDiffLines to map them.
	// Comparisons read comment-free text: the original side is stripped here,
	// and the text comparisons of the modified side strip their own subtree
	// (diffNodeFallback). The diff prints no comment line (emitReindentedLines).
	origTree = config.WithoutComments(origTree)

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

	return renderDiffLines(diffs, modified)
}

// renderDiffLines converts diff lines into an annotated string with line mapping.
// The map sends a display line to the line of working (the content validation
// line numbers index) that holds the same statement. The diff prints no comment
// line and no blank line. Match statements rather than their ordinal positions:
// a removed/moved row consumes no source line, and an input not yet formatted
// may use a block where the serializer prints an inline statement. Such a row
// has no exact source counterpart and must not borrow another statement's error.
func renderDiffLines(diffs []diffLine, working string) (string, map[int]int) {
	statementLines := make(map[string][]int)
	for i, line := range splitDiffLines(working) {
		if isCommentOrBlankLine(line) {
			continue
		}
		statement := diffStatementKey(line)
		statementLines[statement] = append(statementLines[statement], i+1)
	}
	var b textbuf.Buffer
	lineMapping := make(map[int]int)
	displayLine := 0
	workingLine := 0

	for _, dl := range diffs {
		if displayLine > 0 {
			b.Byte('\n')
		}
		displayLine++

		b.Byte(byte(dl.Marker)).Byte(' ').Str(dl.Text)

		switch dl.Marker {
		case diffUnchanged, diffAdded, diffModified:
			positions := statementLines[diffStatementKey(dl.Text)]
			next, _ := slices.BinarySearch(positions, workingLine+1)
			if next < len(positions) {
				workingLine = positions[next]
				lineMapping[displayLine] = workingLine
			}
		case diffRemoved: // removed lines have no working content counterpart
		default:
			panic("BUG: invalid diff marker")
		}
	}

	return b.String(), lineMapping
}

// diffStatementKey matches display rows to source rows without rewriting config.
// Ignore trailing comments, but not a '#' inside a quoted key or value.
func diffStatementKey(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		if quote != 0 {
			if line[i] == '\\' {
				i++
				continue
			}
			if line[i] == quote {
				quote = 0
			}
			continue
		}
		switch line[i] {
		case '"', '\'':
			quote = line[i]
		case '#':
			return strings.TrimSpace(line[:i])
		}
	}
	return strings.TrimSpace(line)
}

// isCommentOrBlankLine reports whether a printed line holds no statement: it is
// empty, or a `#` comment on its own line. A statement never starts with `#`,
// because the tokenizer reads `#` outside a quoted string as a comment.
func isCommentOrBlankLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return true
	}
	return trimmed[0] == '#'
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
		if !n.Ephemeral {
			diffList(lines, orig, mod, name, n, indent)
		}

	case *config.FlexNode:
		diffFlex(lines, orig, mod, name, n, indent)

	case *config.InlineListNode:
		diffListEntries(lines, orig, mod, name, n, false, indent)

	case *config.FreeformNode:
		diffNodeFallback(lines, orig, mod, name, node, indent)
	}
}

// --- Leaf-type diffs ---

func diffLeaf(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origVal, origOk := orig.Get(name)
	modVal, modOk := mod.Get(name)

	origFmt := formatLeafValue(origVal)
	modFmt := formatLeafValue(modVal)

	var tb textbuf.Buffer
	switch {
	case origOk && modOk && origFmt == modFmt:
		*lines = append(*lines, diffLine{diffUnchanged, tb.Str(prefix).Str(name).Byte(' ').Str(modFmt).String()})
	case origOk && modOk:
		*lines = append(*lines, diffLine{diffModified, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(modFmt).String()})
	case origOk:
		*lines = append(*lines, diffLine{diffRemoved, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(origFmt).String()})
	case modOk:
		*lines = append(*lines, diffLine{diffAdded, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(modFmt).String()})
	}
}

func diffMultiLeaf(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origVal, origOk := orig.Get(name)
	modVal, modOk := mod.Get(name)

	var tb textbuf.Buffer
	switch {
	case origOk && modOk && origVal == modVal:
		*lines = append(*lines, diffLine{diffUnchanged, tb.Str(prefix).Str(name).Byte(' ').Str(origVal).String()})
	case origOk && modOk:
		*lines = append(*lines, diffLine{diffModified, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(modVal).String()})
	case origOk:
		*lines = append(*lines, diffLine{diffRemoved, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(origVal).String()})
	case modOk:
		*lines = append(*lines, diffLine{diffAdded, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(modVal).String()})
	}
}

func diffBracketLeaf(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origVal, origOk := bracketDisplay(orig, name)
	modVal, modOk := bracketDisplay(mod, name)

	var tb textbuf.Buffer
	switch {
	case origOk && modOk && origVal == modVal:
		*lines = append(*lines, diffLine{diffUnchanged, tb.Str(prefix).Str(name).Str(" [ ").Str(origVal).Str(" ]").String()})
	case origOk && modOk:
		*lines = append(*lines, diffLine{diffModified, tb.Str(prefix).Str(name).Str(" [ ").Str(modVal).Str(" ]").String()})
	case origOk:
		*lines = append(*lines, diffLine{diffRemoved, tb.Str(prefix).Str(name).Str(" [ ").Str(origVal).Str(" ]").String()})
	case modOk:
		*lines = append(*lines, diffLine{diffAdded, tb.Str(prefix).Str(name).Str(" [ ").Str(modVal).Str(" ]").String()})
	}
}

// bracketDisplay renders a bracket leaf-list's members for the tree diff, a
// deactivated member included and marked (memberDisplay): the mirror names the
// active members only, so diffing it showed a deactivation as a deletion.
// A value stored with no members is rendered as it stands.
func bracketDisplay(tree *config.Tree, name string) (string, bool) {
	if members := memberDisplay(tree, name); len(members) > 0 {
		return textbuf.Join(members, " "), true
	}
	return tree.Get(name)
}

// memberDisplay renders a leaf-list's members for the tree diff, marking any
// deactivated member with the inactive: prefix. Per-member deactivation lives
// out-of-band on the Tree; reconstructing the compact prefix here keeps a
// deactivation change visible in the diff (byte-identical to the pre-refactor
// in-band display) rather than silently dropping the member.
func memberDisplay(tree *config.Tree, name string) []string {
	state := tree.GetMultiValuesState(name)
	if len(state) == 0 {
		return nil
	}
	out := make([]string, len(state))
	var tb textbuf.Buffer
	for i, m := range state {
		if m.Inactive {
			out[i] = tb.Reset().Str("inactive:").Str(m.Value).String()
		} else {
			out[i] = m.Value
		}
	}
	return out
}

func diffValueOrArray(lines *[]diffLine, orig, mod *config.Tree, name, prefix string) {
	origText := formatSlice(memberDisplay(orig, name))
	modText := formatSlice(memberDisplay(mod, name))

	var tb textbuf.Buffer
	switch {
	case origText != "" && modText != "" && origText == modText:
		*lines = append(*lines, diffLine{diffUnchanged, tb.Str(prefix).Str(name).Byte(' ').Str(origText).String()})
	case origText != "" && modText != "":
		*lines = append(*lines, diffLine{diffModified, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(modText).String()})
	case origText != "":
		*lines = append(*lines, diffLine{diffRemoved, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(origText).String()})
	case modText != "":
		*lines = append(*lines, diffLine{diffAdded, tb.Reset().Str(prefix).Str(name).Byte(' ').Str(modText).String()})
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
		if config.CanInlineContainer(modChild, node) {
			diffContainerInline(lines, origChild, modChild, name, node, prefix)
		} else {
			var tb textbuf.Buffer
			*lines = append(*lines, diffLine{diffUnchanged, tb.Str(prefix).Str(name).Str(" {").String()})
			diffWalkChildren(lines, origChild, modChild, node, indent+1)
			*lines = append(*lines, diffLine{diffUnchanged, tb.Reset().Str(prefix).Byte('}').String()})
		}
	}
}

// diffContainerInline emits a single diff line for a container that the serializer
// would render inline (e.g., "bgp openwait 140" instead of "bgp {\n\topenwait 140\n}").
// This keeps the diff output line count consistent with Serialize, preventing
// line-mapping drift in validation highlighting.
func diffContainerInline(lines *[]diffLine, origChild, modChild *config.Tree, name string, node *config.ContainerNode, prefix string) {
	// The working shape is authoritative even if comments made the original
	// print differently or a nested plain list changed its presentation order.
	modText := strings.TrimRight(config.SerializeSubtree(modChild, node), "\n")

	var tb textbuf.Buffer
	modLine := tb.Str(prefix).Str(name).Byte(' ').Str(modText).String()

	if config.SubtreesEqual(origChild, modChild, node) {
		*lines = append(*lines, diffLine{diffUnchanged, modLine})
	} else {
		*lines = append(*lines, diffLine{diffModified, modLine})
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
		var tb textbuf.Buffer
		switch {
		case origFmt == modFmt:
			*lines = append(*lines, diffLine{diffUnchanged, tb.Str(prefix).Str(origFmt).String()})
		case origFmt == "":
			*lines = append(*lines, diffLine{diffAdded, tb.Reset().Str(prefix).Str(modFmt).String()})
		case modFmt == "":
			*lines = append(*lines, diffLine{diffRemoved, tb.Reset().Str(prefix).Str(origFmt).String()})
		case origFmt != modFmt:
			*lines = append(*lines, diffLine{diffModified, tb.Reset().Str(prefix).Str(modFmt).String()})
		}
		return
	}

	// Block form — delegate to container diff
	diffContainer(lines, orig, mod, name, node, indent)
}

// --- List diff ---

func diffList(lines *[]diffLine, orig, mod *config.Tree, name string, node *config.ListNode, indent int) {
	diffListEntries(lines, orig, mod, name, node, node.UserOrdered, indent)
}

// diffListEntries follows the working printer. Only a user-ordered list treats
// a change in relative order as a move; additions and deletions alone do not
// move the surviving entries.
func diffListEntries(lines *[]diffLine, orig, mod *config.Tree, name string, node config.Node, ordered bool, indent int) {
	origEntries := orig.GetList(name)
	modEntries := mod.GetList(name)
	origKeys := diffListKeys(orig, name)
	modKeys := diffListKeys(mod, name)
	if len(origKeys) == 0 && len(modKeys) == 0 {
		return
	}
	var moved map[string]bool
	if ordered {
		moved = diffListMoves(origKeys, modKeys)
	}
	// A run of deleted/moved entries belongs after the preceding surviving
	// original entry. A separate leading run also permits an empty string key.
	removed := make(map[string][]string)
	var leading []string
	anchor, anchored := "", false
	for _, key := range origKeys {
		if modEntries[key] == nil || moved[key] {
			if anchored {
				removed[anchor] = append(removed[anchor], key)
			} else {
				leading = append(leading, key)
			}
			continue
		}
		anchor = key
		anchored = true
	}
	grouped := false
	removedGrouped := false
	if list, ok := node.(*config.ListNode); ok {
		removedGrouped = config.ListUsesMultiBlock(origEntries, list)
		entries := modEntries
		if len(entries) == 0 {
			entries = origEntries
		}
		grouped = config.ListUsesMultiBlock(entries, list)
	}
	prefix := strings.Repeat("\t", indent)
	groupMarker := diffUnchanged
	if len(origKeys) == 0 {
		groupMarker = diffAdded
	}
	if len(modKeys) == 0 {
		groupMarker = diffRemoved
	}
	if grouped {
		*lines = append(*lines, diffLine{groupMarker, prefix + name + " {"})
		indent++
	}
	emitRemoved := func(keys []string) {
		for _, key := range keys {
			text, _ := diffPrintedListEntry(name, key, origEntries[key], node, grouped && removedGrouped)
			emitReindentedLines(lines, text, diffRemoved, indent)
		}
	}
	emitRemoved(leading)
	for _, key := range modKeys {
		entry := modEntries[key]
		text, block := diffPrintedListEntry(name, key, entry, node, grouped)
		switch {
		case origEntries[key] == nil, moved[key]:
			emitReindentedLines(lines, text, diffAdded, indent)
		default:
			diffListEntry(lines, origEntries[key], entry, node, text, block, indent)
		}
		if !moved[key] {
			emitRemoved(removed[key])
		}
	}
	if grouped {
		*lines = append(*lines, diffLine{groupMarker, prefix + "}"})
	}
}

// diffListKeys reads the recorded keys, then delegates missing/stale handling
// to the same producer the printers use. These parsed trees are not mutated.
func diffListKeys(tree *config.Tree, name string) []string {
	recorded := tree.GetListOrdered(name)
	order := make([]string, len(recorded))
	for i, entry := range recorded {
		order[i] = entry.Key
	}
	return config.ListEntryKeys(tree.GetList(name), order)
}

// diffListMoves retains a longest common subsequence of unique entry keys.
// Mapping to original positions makes it an O(n log n) increasing-subsequence
// problem, not the quadratic line LCS used by the parse-error fallback.
func diffListMoves(original, working []string) map[string]bool {
	positions := make(map[string]int, len(original))
	for i, key := range original {
		positions[key] = i
	}
	common := make([]string, 0, min(len(original), len(working)))
	for _, key := range working {
		if _, ok := positions[key]; ok {
			common = append(common, key)
		}
	}
	tails := make([]int, 0, len(common))
	previous := make([]int, len(common))
	for i, key := range common {
		lo, hi := 0, len(tails)
		for lo < hi {
			middle := lo + (hi-lo)/2
			if positions[common[tails[middle]]] < positions[key] {
				lo = middle + 1
			} else {
				hi = middle
			}
		}
		previous[i] = -1
		if lo > 0 {
			previous[i] = tails[lo-1]
		}
		if lo == len(tails) {
			tails = append(tails, i)
		} else {
			tails[lo] = i
		}
	}
	moved := make(map[string]bool, len(common))
	for _, key := range common {
		moved[key] = true
	}
	if len(tails) > 0 {
		for i := tails[len(tails)-1]; i >= 0; i = previous[i] {
			delete(moved, common[i])
		}
	}
	return moved
}

// diffListEntry uses the printer's statement shape. A block is walked by
// schema; a positional or inline statement is one semantic comparison.
func diffListEntry(lines *[]diffLine, orig, mod *config.Tree, node config.Node, text string, block bool, indent int) {
	if config.SubtreesEqual(orig, mod, node) {
		emitReindentedLines(lines, text, diffUnchanged, indent)
		return
	}
	if !block {
		emitReindentedLines(lines, text, diffModified, indent)
		return
	}
	var printed []diffLine
	emitReindentedLines(&printed, text, diffUnchanged, indent)
	opening := printed[0]
	if orig.IsInactive() != mod.IsInactive() {
		opening.Marker = diffModified
	}
	*lines = append(*lines, opening)
	if walker, ok := node.(nodeWalker); ok {
		diffWalkChildren(lines, orig, mod, walker, indent+1)
	}
	*lines = append(*lines, printed[len(printed)-1])
}

// diffPrintedListEntry delegates shape and quoting to the working serializer.
// Only the three list node kinds dispatched by diffWalkNode reach this helper.
func diffPrintedListEntry(name, key string, entry *config.Tree, node config.Node, grouped bool) (string, bool) {
	var text textbuf.Buffer
	block := true
	switch n := node.(type) {
	case *config.ListNode:
		if grouped {
			config.WritePositionalListEntry(&text, entry, key, n, "")
			block = false
		} else {
			block = config.WriteListEntry(&text, entry, name, key, n, "", "")
		}
	case *config.FlexNode:
		config.WriteFlexListEntry(&text, entry, name, key, n, "")
	case *config.InlineListNode:
		block = config.WriteInlineListEntry(&text, entry, name, key, n, "")
	}
	return text.String(), block
}

// --- Flex node diff ---

func diffFlex(lines *[]diffLine, orig, mod *config.Tree, name string, node *config.FlexNode, indent int) {
	prefix := strings.Repeat("\t", indent)
	var tb textbuf.Buffer

	// Flex values may have repeated statements alongside named entries.
	origText := diffFlexValueText(orig, name)
	modText := diffFlexValueText(mod, name)
	_, origScalar := orig.Get(name)
	_, modScalar := mod.Get(name)
	switch {
	case origText == modText:
		emitReindentedLines(lines, modText, diffUnchanged, indent)
	case origText == "":
		emitReindentedLines(lines, modText, diffAdded, indent)
	case modText == "":
		emitReindentedLines(lines, origText, diffRemoved, indent)
	case origScalar && modScalar:
		emitReindentedLines(lines, modText, diffModified, indent)
	default:
		emitReindentedLines(lines, origText, diffRemoved, indent)
		emitReindentedLines(lines, modText, diffAdded, indent)
	}

	// Container form
	origChild := orig.GetContainer(name)
	modChild := mod.GetContainer(name)
	if origChild != nil || modChild != nil {
		openLine := tb.Reset().Str(prefix).Str(name).Str(" {").String()
		closeLine := tb.Reset().Str(prefix).Byte('}').String()
		switch {
		case origChild == nil:
			*lines = append(*lines, diffLine{diffAdded, openLine})
			emitFlexChildrenMarked(lines, modChild, node, diffAdded, indent+1)
			*lines = append(*lines, diffLine{diffAdded, closeLine})
		case modChild == nil:
			*lines = append(*lines, diffLine{diffRemoved, openLine})
			emitFlexChildrenMarked(lines, origChild, node, diffRemoved, indent+1)
			*lines = append(*lines, diffLine{diffRemoved, closeLine})
		default: // both present
			*lines = append(*lines, diffLine{diffUnchanged, openLine})
			diffWalkChildren(lines, origChild, modChild, node, indent+1)
			*lines = append(*lines, diffLine{diffUnchanged, closeLine})
		}
	}
	diffListEntries(lines, orig, mod, name, node, false, indent)
}

// diffFlexValueText follows the Flex scalar/multiple-value precedence without
// dropping repeated values when this same node also holds named entries.
func diffFlexValueText(tree *config.Tree, name string) string {
	if value, ok := tree.Get(name); ok {
		return formatFlexValue(name, value, true)
	}
	var text textbuf.Buffer
	for _, member := range tree.GetMultiValuesState(name) {
		text.Str(name).Byte(' ').Str(config.QuoteIfNeeded(member.Value)).Byte('\n')
	}
	return text.String()
}

// --- Fallback for unsupported node types ---

// The comparison reads the modified side without its comments, as the original
// already is (computeTreeAnnotatedDiff). The lines it emits for that side come
// from the commented text, whose shape is the one Serialize printed: a comment
// inside an inline list makes it print one line per entry.
func diffNodeFallback(lines *[]diffLine, orig, mod *config.Tree, name string, node config.Node, indent int) {
	origText := serializeNodeText(orig, name, node, indent, false)
	modText := serializeNodeText(mod, name, node, indent, false)
	modBare := serializeNodeText(mod, name, node, indent, true)

	switch {
	case origText == modBare:
		emitTextLines(lines, modText, diffUnchanged)
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

// emitContainerMarked uses the working serializer's inline or block shape.
func emitContainerMarked(lines *[]diffLine, name string, tree *config.Tree, node *config.ContainerNode, marker diffMarker, indent int) {
	prefix := strings.Repeat("\t", indent)
	var tb textbuf.Buffer
	if !node.Presence && config.CanInlineContainer(tree, node) {
		text := strings.TrimRight(config.SerializeSubtree(tree, node), "\n")
		*lines = append(*lines, diffLine{marker, tb.Str(prefix).Str(name).Byte(' ').Str(text).String()})
		return
	}
	*lines = append(*lines, diffLine{marker, tb.Str(prefix).Str(name).Str(" {").String()})
	emitReindentedLines(lines, config.SerializeSubtree(tree, node), marker, indent+1)
	*lines = append(*lines, diffLine{marker, tb.Reset().Str(prefix).Byte('}').String()})
}

// emitFlexChildrenMarked serializes children of a flex node and marks all lines.
func emitFlexChildrenMarked(lines *[]diffLine, tree *config.Tree, node *config.FlexNode, marker diffMarker, indent int) {
	emitReindentedLines(lines, config.SerializeSubtree(tree, node), marker, indent)
}

// emitReindentedLines splits text into lines, re-indents from indent 0 to target, and emits with marker.
// A comment on its own line and a blank line are skipped: the diff prints
// statements only (renderDiffLines).
func emitReindentedLines(lines *[]diffLine, text string, marker diffMarker, indent int) {
	prefix := strings.Repeat("\t", indent)
	for _, line := range splitDiffLines(text) {
		if isCommentOrBlankLine(line) {
			continue
		}
		*lines = append(*lines, diffLine{marker, prefix + line})
	}
}

// emitTextLines splits text into lines and emits each with the given marker,
// skipping a comment on its own line and a blank line as emitReindentedLines does.
func emitTextLines(lines *[]diffLine, text string, marker diffMarker) {
	for _, line := range splitDiffLines(text) {
		if isCommentOrBlankLine(line) {
			continue
		}
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
	slices.Sort(keys)

	var tb textbuf.Buffer
	for _, k := range keys {
		origVal, origOk := orig.Get(k)
		modVal, modOk := mod.Get(k)

		origFmt := config.QuoteIfNeeded(origVal)
		modFmt := config.QuoteIfNeeded(modVal)

		switch {
		case origOk && modOk && origFmt == modFmt:
			*lines = append(*lines, diffLine{diffUnchanged, tb.Reset().Str(prefix).Str(k).Byte(' ').Str(origFmt).String()})
		case origOk && modOk:
			*lines = append(*lines, diffLine{diffModified, tb.Reset().Str(prefix).Str(k).Byte(' ').Str(modFmt).String()})
		case origOk:
			*lines = append(*lines, diffLine{diffRemoved, tb.Reset().Str(prefix).Str(k).Byte(' ').Str(origFmt).String()})
		case modOk:
			*lines = append(*lines, diffLine{diffAdded, tb.Reset().Str(prefix).Str(k).Byte(' ').Str(modFmt).String()})
		}
	}
}

// serializeNodeText serializes a single node from a tree for fallback comparison.
// bare prints the node's block without the comments it carries.
func serializeNodeText(tree *config.Tree, name string, node config.Node, indent int, bare bool) string {
	prefix := strings.Repeat("\t", indent)
	var tb textbuf.Buffer

	// For leaf-like nodes, check if value exists
	if v, ok := tree.Get(name); ok {
		return tb.Str(prefix).Str(name).Byte(' ').Str(config.QuoteIfNeeded(v)).String()
	}

	// For container-like nodes, check if container exists
	if child := tree.GetContainer(name); child != nil {
		if bare {
			child = config.WithoutComments(child)
		}
		inner := config.SerializeSubtree(child, node)
		if inner == "" {
			return ""
		}
		tb.Str(prefix).Str(name).Str(" {\n")
		for _, line := range splitDiffLines(inner) {
			tb.Repeat("\t", indent+1).Str(line).Byte('\n')
		}
		tb.Str(prefix).Byte('}')
		return tb.String()
	}

	return ""
}

// --- Value formatting ---

func formatLeafValue(v string) string {
	return config.QuoteIfNeeded(diffNormalizeBool(v))
}

func formatPresenceValue(name, v string, ok bool) string {
	if !ok {
		return ""
	}
	if v == boolTrue {
		return name
	}
	var tb textbuf.Buffer
	return tb.Str(name).Byte(' ').Str(config.QuoteIfNeeded(v)).String()
}

func formatFlexValue(name, v string, ok bool) string {
	if !ok {
		return ""
	}
	if v == boolTrue {
		return name
	}
	var tb textbuf.Buffer
	return tb.Str(name).Byte(' ').Str(config.QuoteIfNeeded(v)).String()
}

func formatSlice(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return config.QuoteIfNeeded(items[0])
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = config.QuoteIfNeeded(item)
	}
	var tb textbuf.Buffer
	return tb.Str("[ ").Join(parts, " ").Str(" ]").String()
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
