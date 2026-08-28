// Design: docs/architecture/core-design.md -- the authored half of the two-way link
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// tags.go finds `RFC requirement: <ID> <polarity>` in the three shapes a test
// can take. The link between an obligation and its proof is two-way and only
// ONE side is authored: the tag lives in the test, so it dies with the test. A
// hand-written back-link would outlive the test it names.
//
// Each reader refuses to report "no tags" for a file it could not read. That is
// the whole reason the Python reader is tokenized rather than regex-scanned: a
// scenario check is full of quoted protocol text, and a `#` inside a string is
// not a comment.
package rfc

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	goTagRE           = regexp.MustCompile(`^\s*//\s*RFC requirement:\s*(?P<rest>.*)$`)
	ciTagRE           = regexp.MustCompile(`^#\s*RFC requirement:\s*(?P<rest>.*)$`)
	terminatorRE      = regexp.MustCompile(`terminator=(?P<name>[A-Za-z0-9_]+)`)
	changedTagRE      = regexp.MustCompile(`RFC requirement:\s*[A-Za-z0-9][A-Za-z0-9.-]*-\d+`)
	goLineCommentRE   = regexp.MustCompile(`(?m)//.*$`)
	hashLineCommentRE = regexp.MustCompile(`(?m)^[ \t]*#.*$`)
	goImportLineRE    = regexp.MustCompile(`^(?:import\s*\(|\)|import\s+)?(?:[A-Za-z_.][A-Za-z0-9_.]*\s+)?"[^"]+"$`)
	goImportDelimRE   = regexp.MustCompile(`^(?:import\s*\(|\))$`)
)

// parseTagRest reads the words after the marker: the id, then the mandatory
// polarity. Polarity is never inferred, because a negative-only test passes if
// the code rejects everything and a positive-only one passes if it accepts
// everything. Only the pair pins behavior to the requirement.
func parseTagRest(rest, where string) (Tag, error) {
	var tb textbuf.Buffer
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return Tag{}, parseErr(tb.Str(where).Str(": empty 'RFC requirement:' tag"))
	}
	rid := strings.TrimRight(parts[0], tagPunct)
	if len(parts) < 2 {
		return Tag{}, parseErr(tb.Str(where).Str(": tag for ").Str(rid).
			Str(" has no polarity. Polarity is mandatory and never inferred: ").
			Str("'RFC requirement: ").Str(rid).Str(" positive|negative -- note'"))
	}
	polarity := strings.TrimRight(strings.ToLower(parts[1]), tagPunct)
	if !polarities[polarity] {
		return Tag{}, parseErr(tb.Str(where).Str(": tag for ").Str(rid).
			Str(" has invalid polarity ").Str(pyRepr(parts[1])).Str("; expected one of ").
			Str(pyRepr(polarityNames())))
	}
	return Tag{RID: rid, Polarity: polarity}, nil
}

// tagWhere renders the `<path>:<line>` a tag's message opens with.
func tagWhere(path string, line int) string {
	var tb textbuf.Buffer
	return tb.Str(path).Byte(':').Int(int64(line)).String()
}

// scanGoTags finds `// RFC requirement: <ID> <polarity>` anywhere in a Go test
// file.
//
// Deliberately not limited to doc comments: one function can cover a dozen
// requirements across a hundred table cases, so tags must be placeable inline
// at the case. A function-level-only tag would stay green after the single
// enforcing case was deleted.
func scanGoTags(src, path string) ([]Tag, error) {
	var out []Tag
	for i, line := range strings.Split(src, "\n") {
		found := goTagRE.FindStringSubmatch(line)
		if found == nil {
			continue
		}
		tag, err := parseTagRest(found[1], tagWhere(path, i+1))
		if err != nil {
			return nil, err
		}
		tag.File, tag.Line = path, i+1
		out = append(out, tag)
	}
	return out, nil
}

// scanCITags finds `# RFC requirement: <ID> <polarity>` in a .ci file.
//
// Two constraints come from the real parser: a comment is only a comment at
// line start after trimming, and content inside a `terminator=` block is RAW
// FILE CONTENT rather than .ci syntax. Scanning those blocks would invent
// phantom tags from an embedded shell script's own comments.
func scanCITags(src, path string) ([]Tag, error) {
	var out []Tag
	terminator := ""
	for i, line := range strings.Split(src, "\n") {
		if terminator != "" {
			if strings.TrimSpace(line) == terminator {
				terminator = ""
			}
			continue
		}
		stripped := strings.TrimSpace(line)
		if found := ciTagRE.FindStringSubmatch(stripped); found != nil {
			tag, err := parseTagRest(found[1], tagWhere(path, i+1))
			if err != nil {
				return nil, err
			}
			tag.File, tag.Line = path, i+1
			out = append(out, tag)
			continue
		}
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if found := terminatorRE.FindStringSubmatch(stripped); found != nil {
			terminator = found[1]
		}
	}
	return out, nil
}

// scanLegacyPythonTags reads the deleted interop checker shape from the parent
// commit during this cutover. Current-tree carriers never select it.
func scanLegacyPythonTags(src, path string) ([]Tag, error) {
	var out []Tag
	for index, line := range strings.Split(src, "\n") {
		stripped := strings.TrimSpace(line)
		found := ciTagRE.FindStringSubmatch(stripped)
		if found == nil || !strings.HasPrefix(stripped, "#") {
			continue
		}
		tag, err := parseTagRest(found[1], tagWhere(path, index+1))
		if err != nil {
			return nil, err
		}
		tag.File, tag.Line = path, index+1
		out = append(out, tag)
	}
	return out, nil
}

// ChangedTags answers the RFC tags whose test behaviour changed between old
// and new.
//
// Removing a tag is itself a change even though tags are comments. Otherwise
// comment-only, whitespace-only, and Go import-only edits are ignored. The
// caller supplies one tagged unit (or the whole file for a file-scoped
// carrier), so changing an untagged sibling cannot claim an RFC approval.
func ChangedTags(path, oldText, newText string) []string {
	oldTags := changedTagNames(oldText)
	if len(oldTags) == 0 {
		return nil
	}
	newTags := changedTagNames(newText)
	var dropped []string
	for _, tag := range oldTags {
		if !containsSorted(newTags, tag) {
			dropped = append(dropped, tag)
		}
	}
	if len(dropped) > 0 {
		return dropped
	}
	if importOnlyGoEdit(path, oldText, newText) {
		return nil
	}
	if behaviorBytes(path, oldText) == behaviorBytes(path, newText) {
		return nil
	}
	return oldTags
}

func changedTagNames(text string) []string {
	found := changedTagRE.FindAllString(text, -1)
	if len(found) == 0 {
		return nil
	}
	sort.Strings(found)
	return compactStrings(found)
}

func containsSorted(values []string, value string) bool {
	index := sort.SearchStrings(values, value)
	return index < len(values) && values[index] == value
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func importOnlyGoEdit(path, oldText, newText string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	seen := false
	for _, side := range []string{oldText, newText} {
		for line := range strings.SplitSeq(goLineCommentRE.ReplaceAllString(side, ""), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			seen = true
			if !goImportLineRE.MatchString(line) && !goImportDelimRE.MatchString(line) {
				return false
			}
		}
	}
	return seen
}

// dropCarrierBlocks removes the body of every `tmpfs=` block, keeping the
// opening directive and the terminator line. A `stdin=` block is left whole:
// it is carrier input, and `stdin=peer:terminator=EOF_PEER` holds the
// `expect=bgp:...:hex=...` wire expectations themselves.
//
// scanCITags already refuses to read inside such a block, because the content
// is RAW FILE CONTENT rather than .ci syntax. behaviorBytes owes the same rule:
// a block holds a fixture the carrier writes to disk, not an assertion the
// carrier makes, so moving a fixture out of the file changes no tested
// behaviour. Without this, replacing an embedded fixture with a compiled one
// read as a change to every RFC requirement the carrier tags, and 27 tests
// demanded owner approval for an edit that touched no assertion.
//
// What still counts as a change: a dropped tag, which ChangedTags answers
// before it reaches here, and any edit to the carrier's own directive lines,
// which are outside every block.
func dropCarrierBlocks(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	terminator := ""
	for _, line := range lines {
		if terminator != "" {
			if strings.TrimSpace(line) == terminator {
				terminator = ""
				kept = append(kept, line)
			}
			continue
		}
		kept = append(kept, line)
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if !strings.HasPrefix(stripped, "tmpfs=") {
			continue
		}
		if found := terminatorRE.FindStringSubmatch(stripped); found != nil {
			terminator = found[1]
		}
	}
	return strings.Join(kept, "\n")
}

func behaviorBytes(path, text string) string {
	if strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et") ||
		strings.HasSuffix(path, ".py") {
		text = dropCarrierBlocks(text)
		text = hashLineCommentRE.ReplaceAllString(text, "")
	} else {
		text = goLineCommentRE.ReplaceAllString(text, "")
	}
	return strings.Map(func(char rune) rune {
		if unicode.IsSpace(char) {
			return -1
		}
		return char
	}, text)
}
