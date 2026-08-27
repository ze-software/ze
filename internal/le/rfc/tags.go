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
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	goTagRE      = regexp.MustCompile(`^\s*//\s*RFC requirement:\s*(?P<rest>.*)$`)
	ciTagRE      = regexp.MustCompile(`^#\s*RFC requirement:\s*(?P<rest>.*)$`)
	terminatorRE = regexp.MustCompile(`terminator=(?P<name>[A-Za-z0-9_]+)`)
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
			Str(pyRepr(Polarities())))
	}
	return Tag{RID: rid, Polarity: polarity}, nil
}

// tagWhere renders the `<path>:<line>` a tag's message opens with.
func tagWhere(path string, line int) string {
	var tb textbuf.Buffer
	return tb.Str(path).Byte(':').Int(int64(line)).String()
}

// ScanGoTags finds `// RFC requirement: <ID> <polarity>` anywhere in a Go test
// file.
//
// Deliberately not limited to doc comments: one function can cover a dozen
// requirements across a hundred table cases, so tags must be placeable inline
// at the case. A function-level-only tag would stay green after the single
// enforcing case was deleted.
func ScanGoTags(src, path string) ([]Tag, error) {
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

// ScanCITags finds `# RFC requirement: <ID> <polarity>` in a .ci file.
//
// Two constraints come from the real parser: a comment is only a comment at
// line start after trimming, and content inside a `terminator=` block is RAW
// FILE CONTENT rather than .ci syntax. Scanning those blocks would invent
// phantom tags from an embedded shell script's own comments.
func ScanCITags(src, path string) ([]Tag, error) {
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

// ScanPythonTags finds `# RFC requirement: <ID> <polarity>` in an interop
// scenario's check.py.
//
// It reads COMMENTS, never lines: a scenario check is full of quoted protocol
// text, and a `#` inside a string or a docstring is not a comment. The comment
// must also be the first thing on its line, mirroring the other two readers.
//
// It fails closed on a file the reader cannot make sense of. That is exactly
// the condition under which comment extraction is untrustworthy, so reporting
// "no tags" would be a zero that looks like an answer.
func ScanPythonTags(src, path string) ([]Tag, error) {
	comments, err := pyComments(src)
	if err != nil {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(path).Str(": cannot tokenize as Python (").Err(err).
			Str("); a file whose comments cannot be read cannot be reported as carrying ").
			Str("no RFC requirement tags"))
	}
	var out []Tag
	for _, comment := range comments {
		if strings.TrimSpace(comment.before) != "" {
			continue // trailing comment, not a line-start tag
		}
		found := ciTagRE.FindStringSubmatch(strings.TrimSpace(comment.text))
		if found == nil {
			continue
		}
		tag, err := parseTagRest(found[1], tagWhere(path, comment.line))
		if err != nil {
			return nil, err
		}
		tag.File, tag.Line = path, comment.line
		out = append(out, tag)
	}
	return out, nil
}
