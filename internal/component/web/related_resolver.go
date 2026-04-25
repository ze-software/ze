// Design: docs/architecture/web-interface.md -- V2 workbench related-tool resolver
// Related: handler_workbench.go -- workbench handler that emits resolved tool data
// Related: ../config/related.go -- RelatedTool struct and parser
//
// Spec: plan/spec-web-2-operator-workbench.md (Placeholder Sources, Resolved-Value Validation).
//
// The resolver substitutes placeholders in a RelatedTool's command template
// against the user's working tree, then validates the resolved tokens
// against the spec's safety rules. The browser never sees a raw command;
// it only knows tool ids and context paths and POSTs them. The handler
// then calls Resolve to produce the trusted command before dispatch.

package web

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// Resolution is the outcome of resolving one RelatedTool against a row's
// configuration context.
type Resolution struct {
	// Command is the trusted, fully-substituted command string ready for
	// the dispatcher. Empty when Disabled.
	Command string
	// Disabled is true when a placeholder could not resolve and the
	// descriptor's `empty` rule asked for the tool to be hidden/disabled.
	Disabled bool
	// DisabledReason explains why the tool is disabled (rendered in the
	// disabled tooltip and surfaced in error responses).
	DisabledReason string
}

// RelatedResolver resolves placeholder substitutions for ze:related tool
// command templates against a user's working tree. Construct one per
// request; the resolver holds no mutable state beyond its inputs.
type RelatedResolver struct {
	schema *config.Schema
	tree   *config.Tree
}

// NewRelatedResolver builds a resolver tied to the given schema and the
// caller's working tree (typically the editor session's tree, not the
// committed tree, so unsaved changes are visible to placeholder lookups
// per the spec's Placeholder Sources note).
func NewRelatedResolver(schema *config.Schema, tree *config.Tree) *RelatedResolver {
	return &RelatedResolver{schema: schema, tree: tree}
}

const (
	relatedMaxRelativeSegments = 16
	relatedMaxResolvedValueLen = 256
	relatedMaxResolvedCmdLen   = 4096
)

// Resolve substitutes placeholders in tool.Command using values from the
// working tree at contextPath. Returns:
//   - (resolution, nil) on success or graceful disable.
//   - (nil, error) on validation failures (unsafe value, depth exceeded,
//     malformed placeholder) -- these are programming/input errors and
//     should never reach an end user as a successful response.
func (r *RelatedResolver) Resolve(tool *config.RelatedTool, contextPath []string) (*Resolution, error) {
	if tool == nil {
		return nil, fmt.Errorf("related resolver: nil tool")
	}

	// Walk the working tree to the row subtree.
	rowSubtree := walkTree(r.tree, r.schema, contextPath)

	// Pre-scan the template for placeholder depth limits before substitution.
	if err := validatePlaceholderDepths(tool.Command); err != nil {
		return nil, err
	}

	resolved, missing, err := r.substitute(tool.Command, contextPath, rowSubtree)
	if err != nil {
		return nil, err
	}

	if missing && tool.Empty == config.RelatedEmptyDisable {
		return &Resolution{
			Disabled:       true,
			DisabledReason: "required context value missing",
		}, nil
	}
	if missing && tool.Empty == config.RelatedEmptyOmit {
		// Caller is expected to drop the tool entirely. Disabled+empty
		// command communicates that.
		return &Resolution{
			Disabled:       true,
			DisabledReason: "omit-on-missing",
		}, nil
	}

	// Empty=allow: a missing placeholder leaves a hole in the template.
	// Collapse runs of whitespace and trim the edges so the dispatcher sees
	// a tokenizable command, not `peer  detail` with a stray double space
	// where a substitution failed.
	if missing {
		resolved = collapseSpaces(resolved)
	}

	if len(resolved) > relatedMaxResolvedCmdLen {
		return nil, fmt.Errorf("related resolver: resolved command length %d exceeds max %d", len(resolved), relatedMaxResolvedCmdLen)
	}
	return &Resolution{Command: resolved}, nil
}

// collapseSpaces returns s with each run of ASCII spaces or tabs collapsed
// to a single space, and leading/trailing whitespace trimmed. Newlines
// inside a command template are not expected; if present they are also
// collapsed since command tokens are whitespace-separated.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true // leading
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	out := b.String()
	return strings.TrimRight(out, " ")
}

// substitute walks the template, expanding placeholders and validating
// each resolved value. Returns the resolved command, a missing-flag set
// when at least one placeholder could not resolve a value, and any
// validation error.
func (r *RelatedResolver) substitute(template string, contextPath []string, rowSubtree *config.Tree) (string, bool, error) {
	var (
		out     strings.Builder
		missing bool
		i       = 0
	)
	for i < len(template) {
		// Find next ${ marker.
		next := strings.Index(template[i:], "${")
		if next < 0 {
			out.WriteString(template[i:])
			break
		}
		out.WriteString(template[i : i+next])
		i += next + 2 // consume ${
		end := strings.Index(template[i:], "}")
		if end < 0 {
			return "", false, fmt.Errorf("related resolver: unterminated placeholder")
		}
		body := template[i : i+end]
		i += end + 1 // consume }

		value, ok, err := r.resolvePlaceholder(body, contextPath, rowSubtree)
		if err != nil {
			return "", false, err
		}
		if !ok {
			missing = true
			continue
		}
		if err := validateResolvedValue(value); err != nil {
			return "", false, err
		}
		out.WriteString(value)
	}
	return out.String(), missing, nil
}

// resolvePlaceholder returns the value for one placeholder body (the text
// inside `${...}`). Returns (value, found, error).
func (r *RelatedResolver) resolvePlaceholder(body string, contextPath []string, rowSubtree *config.Tree) (string, bool, error) {
	// Split source from the rest at the first ':'.
	source, args, hasArgs := strings.Cut(body, ":")
	source = strings.TrimSpace(source)
	args = strings.TrimSpace(args)

	switch source {
	case "key":
		if hasArgs {
			return "", false, fmt.Errorf("related resolver: ${key} takes no arguments")
		}
		key := lastListEntryKey(contextPath)
		if key == "" {
			return "", false, nil
		}
		return key, true, nil

	case "current-path":
		return strings.Join(contextPath, "/"), true, nil

	case "leaf":
		// Field-level only; return the last segment when contextPath looks
		// like a leaf path. v1 leaves field-level resolution to the caller.
		if len(contextPath) == 0 {
			return "", false, nil
		}
		return contextPath[len(contextPath)-1], true, nil

	case "value":
		// Field-level placeholder. The caller is expected to set up the
		// row subtree to point at the field's container; we look up the
		// last path segment as the leaf name.
		if rowSubtree == nil || len(contextPath) == 0 {
			return "", false, nil
		}
		v, ok := rowSubtree.Get(contextPath[len(contextPath)-1])
		if !ok {
			return "", false, nil
		}
		return v, true, nil

	case "path":
		return r.resolvePath(args, contextPath, rowSubtree, false)

	case "path-inherit":
		return r.resolvePath(args, contextPath, rowSubtree, true)

	default:
		return "", false, fmt.Errorf("related resolver: unknown placeholder source %q", source)
	}
}

// resolvePath handles ${path:rel} and ${path:rel|key} placeholders, plus
// the path-inherit variant that walks one parent list entry on miss.
func (r *RelatedResolver) resolvePath(args string, contextPath []string, rowSubtree *config.Tree, inherit bool) (string, bool, error) {
	rel, fallback, hasFallback := strings.Cut(args, "|")
	rel = strings.TrimSpace(rel)
	fallback = strings.TrimSpace(fallback)
	segments, err := splitRelativePath(rel)
	if err != nil {
		return "", false, err
	}

	// Try the row subtree first.
	if v, ok := lookupRelative(rowSubtree, segments); ok {
		return v, true, nil
	}

	// path-inherit: walk one parent list entry up. The spec caps the walk
	// at one parent (Spec D7 plus Failure Routing row "out of scope: cap
	// inheritance at one parent walk in v1"). The parent must itself end at
	// a list entry per the schema; if the operator forged a non-list
	// context_path the inherit step is skipped rather than walking into a
	// container that happens to share a name.
	if inherit {
		if parent := parentListEntry(contextPath); parent != nil && r.schema != nil && isListEntryPath(r.schema, parent) {
			parentSubtree := walkTree(r.tree, r.schema, parent)
			if v, ok := lookupRelative(parentSubtree, segments); ok {
				return v, true, nil
			}
		}
	}

	// Fallback to list key when the descriptor permitted it.
	if hasFallback && fallback == "key" {
		if key := lastListEntryKey(contextPath); key != "" {
			return key, true, nil
		}
	}
	return "", false, nil
}

// validatePlaceholderDepths scans the template for path/path-inherit
// placeholders and rejects any whose relative path exceeds the segment cap.
func validatePlaceholderDepths(template string) error {
	i := 0
	for i < len(template) {
		next := strings.Index(template[i:], "${")
		if next < 0 {
			break
		}
		i += next + 2
		end := strings.Index(template[i:], "}")
		if end < 0 {
			return fmt.Errorf("related resolver: unterminated placeholder")
		}
		body := template[i : i+end]
		i += end + 1

		source, args, _ := strings.Cut(body, ":")
		source = strings.TrimSpace(source)
		if source != "path" && source != "path-inherit" {
			continue
		}
		rel, _, _ := strings.Cut(strings.TrimSpace(args), "|")
		segs, err := splitRelativePath(rel)
		if err != nil {
			return err
		}
		if len(segs) > relatedMaxRelativeSegments {
			return fmt.Errorf("related resolver: relative path depth %d exceeds max %d", len(segs), relatedMaxRelativeSegments)
		}
	}
	return nil
}

// splitRelativePath validates and splits a relative path into segments.
// Returns an error for empty paths, leading or trailing `/`, or invalid
// characters per the spec's Relative Path Grammar.
func splitRelativePath(rel string) ([]string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil, fmt.Errorf("related resolver: empty relative path")
	}
	if strings.HasPrefix(rel, "/") || strings.HasSuffix(rel, "/") {
		return nil, fmt.Errorf("related resolver: relative path must not start or end with '/'")
	}
	parts := strings.Split(rel, "/")
	if slices.Contains(parts, "") {
		return nil, fmt.Errorf("related resolver: empty segment in relative path")
	}
	return parts, nil
}

// lookupRelative walks the subtree following a list of YANG identifier
// segments. The final segment names a leaf; intermediate segments name
// containers.
func lookupRelative(subtree *config.Tree, segments []string) (string, bool) {
	if subtree == nil || len(segments) == 0 {
		return "", false
	}
	t := subtree
	for i, seg := range segments {
		if i == len(segments)-1 {
			v, ok := t.Get(seg)
			if !ok || v == "" {
				return "", false
			}
			return v, true
		}
		t = t.GetContainer(seg)
		if t == nil {
			return "", false
		}
	}
	return "", false
}

// lastListEntryKey returns the trailing key of a context path that points
// at a list entry (e.g. ["bgp","peer","thomas"] returns "thomas"). Returns
// "" when the path is empty.
func lastListEntryKey(contextPath []string) string {
	if len(contextPath) == 0 {
		return ""
	}
	return contextPath[len(contextPath)-1]
}

// parentListEntry returns the context path of the parent list entry, or
// nil when no parent list entry exists. For ["bgp","group","g1","peer","p1"]
// it returns ["bgp","group","g1"].
func parentListEntry(contextPath []string) []string {
	// Walk back two segments (peer, p1) past the row, then check if a
	// previous list entry exists. The pattern is `<list> <key>` repeating.
	if len(contextPath) < 4 {
		return nil
	}
	return contextPath[:len(contextPath)-2]
}

// validateResolvedValue rejects values that contain whitespace, shell
// metacharacters, or exceed the per-value length cap (Spec Resolved-Value
// Validation rules).
func validateResolvedValue(v string) error {
	if v == "" {
		return nil // missing is handled separately
	}
	if len(v) > relatedMaxResolvedValueLen {
		return fmt.Errorf("related resolver: unsafe value: length %d exceeds %d", len(v), relatedMaxResolvedValueLen)
	}
	for _, r := range v {
		if r <= ' ' || r == 0x7f {
			return fmt.Errorf("related resolver: unsafe value: contains control or whitespace")
		}
		if isShellMeta(r) {
			return fmt.Errorf("related resolver: unsafe value: contains shell metacharacter %q", r)
		}
		if !isAllowedValueChar(r) {
			// Bracket characters are accepted only when the whole value
			// parses as a bracketed IPv6 address literal.
			if r == '[' || r == ']' {
				if !isBracketedIPv6(v) {
					return fmt.Errorf("related resolver: unsafe value: brackets only allowed for IPv6 literal")
				}
				continue
			}
			return fmt.Errorf("related resolver: unsafe value: disallowed character %q", r)
		}
	}
	return nil
}

func isAllowedValueChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	return r == '.' || r == ':' || r == '-' || r == '_' || r == '/'
}

func isShellMeta(r rune) bool {
	switch r {
	case ';', '&', '|', '`', '$', '(', ')', '<', '>', '\\', '"', '\'', '\n':
		return true
	}
	return false
}

func isBracketedIPv6(v string) bool {
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return false
	}
	addr := v[1 : len(v)-1]
	ip := net.ParseIP(addr)
	return ip != nil && ip.To4() == nil
}
