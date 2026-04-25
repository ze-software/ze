// Design: docs/architecture/config/yang-config-design.md -- Ze YANG extensions and metadata storage
// Related: schema.go -- schema node structs that carry RelatedTool slices
// Related: yang_schema.go -- ze:related extraction at schema build time
//
// Spec: plan/spec-web-2-operator-workbench.md (Related Tool Metadata Contract).
//
// One `ze:related` YANG statement carries one descriptor encoded with the
// wire format described in the spec: `;`-separated key=value pairs, optional
// double quoting, `\"` and `\\` escapes inside quoted values. Multiple
// `ze:related` statements may sit on the same node; each one parses
// independently into a RelatedTool.

package config

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// RelatedPlacement describes where a related tool should appear in the UI.
type RelatedPlacement int

// RelatedPresentation describes how a related tool's output should be shown.
type RelatedPresentation int

// RelatedClass is a styling hint; authz is never inferred from this.
type RelatedClass int

// RelatedEmpty controls behavior when a placeholder fails to resolve.
type RelatedEmpty int

const (
	// RelatedPlacementDetail is the spec default placement.
	RelatedPlacementDetail RelatedPlacement = iota
	// RelatedPlacementGlobal renders in the section header / page-wide.
	RelatedPlacementGlobal
	// RelatedPlacementTable renders in the table-level toolbar.
	RelatedPlacementTable
	// RelatedPlacementRow renders inline on each list row.
	RelatedPlacementRow
	// RelatedPlacementField renders next to a single leaf.
	RelatedPlacementField
)

const (
	// RelatedPresentationModal is the spec default presentation.
	RelatedPresentationModal RelatedPresentation = iota
	// RelatedPresentationDrawer renders alongside the workspace.
	RelatedPresentationDrawer
	// RelatedPresentationPanel pins inside the workspace.
	RelatedPresentationPanel
)

const (
	// RelatedClassNone means the descriptor did not declare a styling hint.
	RelatedClassNone RelatedClass = iota
	// RelatedClassInspect signals a read-only inspection action.
	RelatedClassInspect
	// RelatedClassDiagnose signals a diagnostic action.
	RelatedClassDiagnose
	// RelatedClassRefresh signals a refresh-style action.
	RelatedClassRefresh
	// RelatedClassDanger signals a destructive action.
	RelatedClassDanger
)

const (
	// RelatedEmptyDisable is the spec default; tools with unresolved
	// placeholders render disabled.
	RelatedEmptyDisable RelatedEmpty = iota
	// RelatedEmptyOmit drops the tool entirely when a placeholder cannot resolve.
	RelatedEmptyOmit
	// RelatedEmptyAllow lets the tool render with the missing placeholder.
	RelatedEmptyAllow
)

// String forms used by tests, error messages, and the rendered HTML data
// attributes. Kept stable so YANG -> UI -> tests agree on tokens.

// String returns the canonical token for the placement.
func (p RelatedPlacement) String() string {
	switch p {
	case RelatedPlacementGlobal:
		return "global"
	case RelatedPlacementTable:
		return "table"
	case RelatedPlacementRow:
		return "row"
	case RelatedPlacementField:
		return "field"
	default:
		return "detail"
	}
}

// String returns the canonical token for the presentation.
func (p RelatedPresentation) String() string {
	switch p {
	case RelatedPresentationDrawer:
		return "drawer"
	case RelatedPresentationPanel:
		return "panel"
	default:
		return "modal"
	}
}

// String returns the canonical token for the class.
func (c RelatedClass) String() string {
	switch c {
	case RelatedClassInspect:
		return "inspect"
	case RelatedClassDiagnose:
		return "diagnose"
	case RelatedClassRefresh:
		return "refresh"
	case RelatedClassDanger:
		return "danger"
	default:
		return ""
	}
}

// String returns the canonical token for the empty-behavior.
func (e RelatedEmpty) String() string {
	switch e {
	case RelatedEmptyOmit:
		return "omit"
	case RelatedEmptyAllow:
		return "allow"
	default:
		return configDisable
	}
}

// RelatedTool is one parsed `ze:related` descriptor attached to a schema
// node. Stored on ContainerNode, ListNode, and LeafNode at schema build
// time. Templates and handlers consume the same struct so the ze:related
// declaration is the single source of truth.
type RelatedTool struct {
	ID           string
	Label        string
	Command      string
	Placement    RelatedPlacement
	Presentation RelatedPresentation
	Confirm      string
	Requires     []string
	Class        RelatedClass
	Empty        RelatedEmpty
}

// Length bounds enforced by the parser. Spec Boundary Tests row.
const (
	relatedIDMaxLen      = 64
	relatedLabelMaxLen   = 48
	relatedCommandMaxLen = 512
)

// ParseRelatedDescriptor parses one `ze:related` argument string into a
// RelatedTool. Returns an error if any field violates the wire format,
// references an unknown key, or omits a required field. The string carries
// exactly one descriptor; multiple ze:related statements parse independently.
func ParseRelatedDescriptor(arg string) (*RelatedTool, error) {
	fields, err := parseDescriptorFields(arg)
	if err != nil {
		return nil, err
	}

	tool := &RelatedTool{
		Placement:    RelatedPlacementDetail,
		Presentation: RelatedPresentationModal,
		Empty:        RelatedEmptyDisable,
	}
	seen := make(map[string]bool, len(fields))

	for _, f := range fields {
		if seen[f.key] {
			return nil, fmt.Errorf("ze:related: duplicate key %q", f.key)
		}
		seen[f.key] = true

		switch f.key {
		case "id":
			if f.value == "" {
				return nil, fmt.Errorf("ze:related: id must not be empty")
			}
			if len(f.value) > relatedIDMaxLen {
				return nil, fmt.Errorf("ze:related: id length %d exceeds max %d", len(f.value), relatedIDMaxLen)
			}
			tool.ID = f.value

		case "label":
			if f.value == "" {
				return nil, fmt.Errorf("ze:related: label must not be empty")
			}
			if len(f.value) > relatedLabelMaxLen {
				return nil, fmt.Errorf("ze:related: label length %d exceeds max %d", len(f.value), relatedLabelMaxLen)
			}
			tool.Label = f.value

		case "command":
			if f.value == "" {
				return nil, fmt.Errorf("ze:related: command must not be empty")
			}
			if len(f.value) > relatedCommandMaxLen {
				return nil, fmt.Errorf("ze:related: command length %d exceeds max %d", len(f.value), relatedCommandMaxLen)
			}
			tool.Command = f.value

		case "placement":
			p, perr := parsePlacement(f.value)
			if perr != nil {
				return nil, perr
			}
			tool.Placement = p

		case "presentation":
			p, perr := parsePresentation(f.value)
			if perr != nil {
				return nil, perr
			}
			tool.Presentation = p

		case "confirm":
			tool.Confirm = f.value

		case "requires":
			tool.Requires = splitRequires(f.value)

		case "class":
			c, cerr := parseClass(f.value)
			if cerr != nil {
				return nil, cerr
			}
			tool.Class = c

		case "empty":
			e, eerr := parseEmpty(f.value)
			if eerr != nil {
				return nil, eerr
			}
			tool.Empty = e

		default:
			return nil, fmt.Errorf("ze:related: unknown key %q", f.key)
		}
	}

	if tool.ID == "" {
		return nil, fmt.Errorf("ze:related: missing required field id")
	}
	if tool.Label == "" {
		return nil, fmt.Errorf("ze:related: missing required field label")
	}
	if tool.Command == "" {
		return nil, fmt.Errorf("ze:related: missing required field command")
	}

	return tool, nil
}

// descriptorField is one parsed key=value pair before semantic validation.
type descriptorField struct {
	key   string
	value string
}

// parseDescriptorFields tokenizes the argument string into key=value pairs.
// Whitespace around separators is ignored; values may be quoted to embed
// `;`, `=`, `}`, or whitespace; `\"` and `\\` are the only valid escapes.
func parseDescriptorFields(arg string) ([]descriptorField, error) {
	var fields []descriptorField

	i := 0
	for i < len(arg) {
		i = skipSpaces(arg, i)
		if i >= len(arg) {
			break
		}

		key, ki, kerr := readKey(arg, i)
		if kerr != nil {
			return nil, kerr
		}
		i = ki

		i = skipSpaces(arg, i)
		if i >= len(arg) || arg[i] != '=' {
			return nil, fmt.Errorf("ze:related: expected '=' after key %q", key)
		}
		i++ // consume '='

		i = skipSpaces(arg, i)
		value, vi, verr := readValue(arg, i)
		if verr != nil {
			return nil, verr
		}
		i = vi

		fields = append(fields, descriptorField{key: key, value: value})

		i = skipSpaces(arg, i)
		if i >= len(arg) {
			break
		}
		if arg[i] != ';' {
			return nil, fmt.Errorf("ze:related: expected ';' after value for %q", key)
		}
		i++ // consume ';'

		// Reject empty fields after a semicolon (e.g. trailing `;` followed by
		// another `;`). A trailing `;` with only whitespace after it is fine.
		j := skipSpaces(arg, i)
		if j < len(arg) && arg[j] == ';' {
			return nil, fmt.Errorf("ze:related: empty field at offset %d", j)
		}
	}

	return fields, nil
}

func skipSpaces(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// readKey reads a bare identifier up to the next `=`, whitespace, or end.
// Keys are never quoted.
func readKey(s string, i int) (string, int, error) {
	start := i
	for i < len(s) {
		c := s[i]
		if c == '=' || c == ' ' || c == '\t' || c == ';' {
			break
		}
		i++
	}
	if i == start {
		return "", i, fmt.Errorf("ze:related: empty key at offset %d", start)
	}
	return s[start:i], i, nil
}

// readValue reads either a quoted or bare value. Bare values terminate at
// `;` or end of input; quoted values terminate at an unescaped `"`.
func readValue(s string, i int) (string, int, error) {
	if i < len(s) && s[i] == '"' {
		return readQuotedValue(s, i)
	}
	return readBareValue(s, i)
}

func readBareValue(s string, i int) (string, int, error) {
	start := i
	for i < len(s) && s[i] != ';' {
		i++
	}
	value := strings.TrimRight(s[start:i], " \t")
	return value, i, nil
}

func readQuotedValue(s string, i int) (string, int, error) {
	if s[i] != '"' {
		return "", i, fmt.Errorf("ze:related: expected '\"' at offset %d", i)
	}
	i++ // consume opening "

	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\\':
			if i+1 >= len(s) {
				return "", i, fmt.Errorf("ze:related: trailing backslash inside quoted value")
			}
			next := s[i+1]
			switch next {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case ',':
				// Preserve `\,` verbatim. splitRequires interprets the
				// sequence as a literal comma in list-valued fields. Other
				// fields pass it through as a two-character substring; the
				// general-escape table only blesses `\"` and `\\`, so
				// callers that care should reject the literal backslash
				// during semantic validation if needed.
				b.WriteByte('\\')
				b.WriteByte(',')
			default:
				return "", i, fmt.Errorf("ze:related: unknown escape sequence \\%c", next)
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", i, fmt.Errorf("ze:related: unterminated quoted value")
}

// splitRequires walks the value, separating on bare commas while honoring
// the `\,` escape that embeds a literal comma. readQuotedValue preserves
// `\,` verbatim so the escape boundary survives down to here.
func splitRequires(s string) []string {
	if s == "" {
		return nil
	}
	var (
		parts []string
		buf   strings.Builder
	)
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == ',' {
			buf.WriteByte(',')
			i++
			continue
		}
		if s[i] == ',' {
			if v := strings.TrimSpace(buf.String()); v != "" {
				parts = append(parts, v)
			}
			buf.Reset()
			continue
		}
		buf.WriteByte(s[i])
	}
	if v := strings.TrimSpace(buf.String()); v != "" {
		parts = append(parts, v)
	}
	return parts
}

func parsePlacement(v string) (RelatedPlacement, error) {
	switch v {
	case "detail", "":
		return RelatedPlacementDetail, nil
	case "global":
		return RelatedPlacementGlobal, nil
	case "table":
		return RelatedPlacementTable, nil
	case "row":
		return RelatedPlacementRow, nil
	case "field":
		return RelatedPlacementField, nil
	default:
		return 0, fmt.Errorf("ze:related: invalid placement %q", v)
	}
}

func parsePresentation(v string) (RelatedPresentation, error) {
	switch v {
	case "modal", "":
		return RelatedPresentationModal, nil
	case "drawer":
		return RelatedPresentationDrawer, nil
	case "panel":
		return RelatedPresentationPanel, nil
	default:
		return 0, fmt.Errorf("ze:related: invalid presentation %q", v)
	}
}

func parseClass(v string) (RelatedClass, error) {
	switch v {
	case "":
		return RelatedClassNone, nil
	case "inspect":
		return RelatedClassInspect, nil
	case "diagnose":
		return RelatedClassDiagnose, nil
	case "refresh":
		return RelatedClassRefresh, nil
	case "danger":
		return RelatedClassDanger, nil
	default:
		return 0, fmt.Errorf("ze:related: invalid class %q", v)
	}
}

func parseEmpty(v string) (RelatedEmpty, error) {
	switch v {
	case configDisable, "":
		return RelatedEmptyDisable, nil
	case "omit":
		return RelatedEmptyOmit, nil
	case "allow":
		return RelatedEmptyAllow, nil
	default:
		return 0, fmt.Errorf("ze:related: invalid empty %q", v)
	}
}

// ValidateRelatedAgainstCommandTree verifies that each tool's command
// template, after canonical placeholder substitution, can reach a node in
// the operational command tree. v1 validates the static prefix only -- the
// run of literal tokens before the first `${...}` placeholder. Tokens
// after a placeholder are not validated because they may sit beneath a
// dynamic-children position (peer name, address family) that is only
// discoverable at runtime. The static prefix is sufficient to catch
// typos and renamed commands at schema-load time, which is the spec's
// stated goal (Metadata Validation Rules row 6).
//
// Returns one error per failing tool; an empty slice means every tool
// referenced a known command-tree path.
//
// When the tree is empty (no -cmd YANG modules registered, e.g. in unit
// tests that exercise the schema without plugin command modules), this
// function skips validation: with nothing to match against, every tool
// would otherwise fail. The schema build remains a best-effort sanity
// gate; the production binary always loads the full command tree and
// therefore always runs the check.
func ValidateRelatedAgainstCommandTree(tools []*RelatedTool, tree *command.Node) []error {
	if tree == nil || len(tree.Children) == 0 {
		return nil
	}
	var errs []error
	for _, tool := range tools {
		prefix := staticCommandPrefix(tool.Command)
		if len(prefix) == 0 {
			// First token is a placeholder; nothing static to validate.
			continue
		}
		if !commandTreeHasPath(tree, prefix) {
			errs = append(errs, fmt.Errorf("ze:related %q: command %q has no static prefix %q in the command tree",
				tool.ID, tool.Command, strings.Join(prefix, " ")))
		}
	}
	return errs
}

// staticCommandPrefix returns the leading whitespace-separated tokens of
// the command template, stopping at the first `${...}` placeholder.
func staticCommandPrefix(template string) []string {
	head, _, _ := strings.Cut(template, "${")
	head = strings.TrimSpace(head)
	if head == "" {
		return nil
	}
	return strings.Fields(head)
}

// commandTreeHasPath reports whether the static tokens form a valid prefix
// in the command tree (each successive token names a child of the previous
// node).
func commandTreeHasPath(root *command.Node, tokens []string) bool {
	node := root
	for _, tok := range tokens {
		if node == nil || node.Children == nil {
			return false
		}
		child, ok := node.Children[tok]
		if !ok {
			return false
		}
		node = child
	}
	return true
}
