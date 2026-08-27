// Design: docs/architecture/core-design.md -- one instruction is one file
// Overview: rules.go -- the corpus predicate these walks read
// Detail: difflib.go -- the drift page a stale rule is reported as
// Detail: render.go -- the read side, which turns a point tree back into a rule
// Detail: coverage.go -- the join that reads these points by id
//
// points.go is the split-and-render half of scripts/dev/rules_points.py.
//
// Each instruction becomes a checked-in file whose PATH is its id. The rendered
// ai/rules/<rule>.md comes from those files. Thus, agents read the same bytes as
// before the corpus split.
//
// The split is a pure LINE PARTITION. Each nonblank line belongs to one point
// body or the manifest header. The renderer restores blank separator lines.
// Byte identity then follows from the design. verifyPartition checks the
// partition itself. A dropped line and an equivalent rendered replacement would
// otherwise compare equal.
//
// The layout has a FIXED depth of two: rule directory, section directory, and
// point file. The manifest stores order, so reordering does not change a point
// id. But a point can exist without a manifest entry. Every walk reports that
// case as an error. Skipping it would silently omit an instruction.

package rules

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// delim is the frontmatter delimiter. The corpus has no horizontal rules, so a
// body line cannot look like a header terminator. Parsing finds the terminator
// before the body starts.
const delim = "---"

// diffBudget is how many lines of a unified diff either gate prints. A page
// longer than that is a rewrite rather than a drift.
const diffBudget = 24

// slugMax is the longest slug the split will derive before it cuts at a word
// boundary.
const slugMax = 60

// pointIndent is the two spaces a manifest point line carries. No slug starts
// with a space, so the point shape and the section shape cannot be confused.
const pointIndent = "  "

// tightMark records a section heading that sits DIRECTLY under the block above
// it. It is not a slug character, so it can never be read as part of a
// directory name.
const tightMark = "^"

// manifestName is the file that makes a directory a rule's point directory.
const manifestName = "manifest.md"

// These are the five point kinds. `heading` and `fence` are STRUCTURAL. They
// name sections or quote data, but state no enforceable instruction. A later
// gate excludes them from its denominator.
const (
	kindDirective = "directive"
	kindTable     = "table"
	kindNote      = "note"
	kindHeading   = "heading"
	kindFence     = "fence"
)

// kinds is the closed set that a point's `kind:` can name. The array follows
// derivation order.
var kinds = [...]string{kindDirective, kindTable, kindNote, kindHeading, kindFence}

// levels contains RFC 2119 levels in detection order. It is shorter than
// rfcLevels because the split records a detected level, not all keyword forms.
var levels = [...]string{levelMustNot, levelMust, levelShould, levelMay}

// These are the manifest's frontmatter fields, in lowercase. The manifest is
// the rule's structural SPINE. It stores the title, metadata block, and reading
// order exactly.
const (
	keyTitle    = "title"
	keyWhen     = "when"
	keySeverity = "severity"
	keyRelated  = "related"
)

// headerKeys is the closed set that a manifest header can name.
var headerKeys = [...]string{keyTitle, keyWhen, keySeverity, keyRelated}

// pointKeys contains a point file's frontmatter fields.
//
// `rationale` is a repo-relative path to the record that explains the point.
// `excepted-by` names points that create exceptions to THIS point. A general
// instruction must include its exception, or readers can stop at a misleading
// statement. No other gate sees this required repetition. Both values are
// HEADER fields, so the rendered body remains byte-identical.
var pointKeys = [...]string{"kind", "level", "stage", wordRationale, "excepted-by"}

// exceptedBy is the key naming this point's exceptions. One general instruction
// can have several, and performance/banned-patterns is the measured case with
// two.
const exceptedBy = "excepted-by"

var (
	pointsH1 = regexp.MustCompile(`^# (\S.*)$`)
	// The three metadata keys, in the exact case the consumers parse.
	pointsMeta = regexp.MustCompile(`^\*\*(When|Severity|Related):\*\* (\S.*)$`)
	fence      = regexp.MustCompile("^[ \t\n\r\f\v]*(`{3,}|~{3,})(.*)$")
	heading    = regexp.MustCompile(`^#{1,6}[ \t\n\r\f\v]`)
	// A `##` heading opens a SECTION. Deeper headings are substructure within a
	// section. Giving them directories would make depth depend on rule nesting.
	sectionHeading = regexp.MustCompile(`^##[ \t\n\r\f\v]+\S`)
	listItem       = regexp.MustCompile(`^[ \t\n\r\f\v]*(-|\d+[.)])[ \t\n\r\f\v]+\S`)
	slugSafe       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	htmlComment    = regexp.MustCompile(`(?s)<!--.*?-->`)
	markdownLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	notSlugRun     = regexp.MustCompile(`[^a-z0-9]+`)
	// A section line names the directory, then carries the heading VERBATIM
	// after one space. A directory slug can hold no space, so the split is
	// unambiguous and the heading is recovered byte-exactly.
	manifestSection = regexp.MustCompile(`^(\^?)(\S+) (##[ \t\n\r\f\v]+\S.*)$`)
	manifestPoint   = regexp.MustCompile(`^ {2}(\S.*)$`)
)

// Point is one block of a rule, with the source lines it owns.
type Point struct {
	Slug  string
	Kind  string
	Level string
	Stage string
	Body  []string
	// Start is the index of the first source line this point owns, and End is
	// one past the last. Both are zero for a point read back from a file: the
	// partition is a property of a split, not of a point on disk.
	Start int
	End   int
	// Rationale is a repo-relative path to why this instruction exists.
	Rationale string
	// ExceptedBy is the comma-separated ids of the points that except this one.
	ExceptedBy string
}

// Section describes one `##` section: its directory, exact heading, and points.
//
// Heading stores the source line VERBATIM. Slug is its directory name and
// cannot store capitalization, punctuation, or the marker. Thus, the manifest
// records Heading for the renderer.
//
// Tight records a heading directly below the preceding block, with no blank
// line. The corpus contains one. Without Tight, the renderer would insert a new
// blank line.
type Section struct {
	Slug    string
	Heading string
	Start   int
	Tight   bool
	Points  []Point
}

// Split is the result of splitting one rendered rule.
type Split struct {
	Stem        string
	Header      map[string]string
	HeaderStart int
	HeaderEnd   int
	Sections    []Section
	LineCount   int
}

// IDs answers every point id this split would write, `<rule>/<section>/<slug>`.
func (s Split) IDs() []string {
	var out []string
	var tb textbuf.Buffer
	for _, section := range s.Sections {
		for _, point := range section.Points {
			tb.Reset()
			out = append(out, tb.Str(s.Stem).Byte('/').Str(section.Slug).
				Byte('/').Str(point.Slug).String())
		}
	}
	return out
}

// ManifestSection is one section line of a manifest: the directory, the
// heading, and the point slugs under it.
type ManifestSection struct {
	Slug    string
	Heading string
	Tight   bool
	Points  []string
}

// pointsError names a malformed manifest, a malformed point, or a lossy
// partition. The name is spelled out because the round-trip report prints the
// exception CLASS beside the message, as Python's type(err).__name__ does.
const pointsError = "RulePointsError"

// bodyLines splits file text into lines and requires one trailing newline.
//
// This strict rule preserves the round trip. Files with no final newline or a
// final blank line cannot pass through a renderer that emits exactly one final
// newline. The corpus has neither form. Guessing would silently normalize a
// measurable property.
func bodyLines(text, what string) ([]string, error) {
	var tb textbuf.Buffer
	if !strings.HasSuffix(text, "\n") {
		return nil, errors.New(tb.Str(what).Str(": must end with a newline").String())
	}
	if strings.HasSuffix(text, "\n\n") {
		return nil, errors.New(tb.Str(what).Str(": must not end with a blank line").String())
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n"), nil
}

// blockRanges answers maximal nonblank runs from start as half-open ranges.
//
// A blank line ends a run, except inside a fenced block. The corpus has 66 blank
// lines in fences. A stateless walker would split a fence. Only the opening
// marker character can close that fence.
//
// A `##` heading outside a fence also ends a run. It opens a section directory.
// If the preceding block swallowed the heading, later point ids would name a
// section that readers never see.
func blockRanges(lines []string, start int) [][2]int {
	var ranges [][2]int
	i := start
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		blockStart := i
		marker := byte(0)
		for i < len(lines) {
			line := lines[i]
			found := fence.FindStringSubmatch(line)
			if marker == 0 {
				if found != nil {
					marker = found[1][0]
					i++
					continue
				}
				if strings.TrimSpace(line) == "" {
					break
				}
				if i > blockStart && sectionHeading.MatchString(line) {
					break
				}
				i++
				continue
			}
			i++
			if closesFence(found, marker) {
				marker = 0
			}
		}
		ranges = append(ranges, [2]int{blockStart, i})
	}
	return ranges
}

// sectionHeadings answers all `##` headings outside fenced blocks, in order.
//
// It derives them independently from RENDERED bytes. blockRanges decides the
// same issue during the split. RenderDir compares both answers. A heading
// swallowed by the splitter is invisible to other gates because rendered bytes
// remain equal. Agreement between these derivations is the available evidence.
func sectionHeadings(lines []string) []string {
	var out []string
	marker := byte(0)
	for _, line := range lines {
		found := fence.FindStringSubmatch(line)
		if marker == 0 {
			if found != nil {
				marker = found[1][0]
				continue
			}
			if sectionHeading.MatchString(line) {
				out = append(out, line)
			}
			continue
		}
		if closesFence(found, marker) {
			marker = 0
		}
	}
	return out
}

// closesFence reports whether a line closes an open fence. It requires the same
// marker character and no info string.
func closesFence(found []string, marker byte) bool {
	return len(found) == 3 && found[1][0] == marker && strings.TrimSpace(found[2]) == ""
}

// classify answers a block's point classification from its first line.
func classify(lines []string) string {
	first := lines[0]
	switch {
	case fence.MatchString(first):
		return kindFence
	case heading.MatchString(first):
		return kindHeading
	case strings.HasPrefix(strings.TrimLeft(first, " \t"), "|"):
		return kindTable
	case listItem.MatchString(first) || strings.HasPrefix(first, "**"):
		return kindDirective
	default:
		return kindNote
	}
}

// levelOf answers the strongest RFC 2119 level a block states, or "".
//
// RE2 has no Python lookbehind or lookahead. This function reads adjacent
// characters instead. Letters around a keyword make it part of a longer word.
func levelOf(lines []string) string {
	text := strings.Join(lines, "\n")
	for _, level := range levels {
		if hasStandaloneKeyword(text, level) {
			return level
		}
	}
	return ""
}

// hasStandaloneKeyword reports whether text carries keyword with no ASCII
// letter on either side of it.
func hasStandaloneKeyword(text, keyword string) bool {
	for offset := 0; ; {
		at := strings.Index(text[offset:], keyword)
		if at < 0 {
			return false
		}
		start := offset + at
		offset = start + 1
		if isASCIILetter(runeBefore(text, start)) || isASCIILetter(runeAt(text, start+len(keyword))) {
			continue
		}
		return true
	}
}

// isASCIILetter reports what Python's [A-Za-z] matches.
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// slugify answers a path-safe id derived from a block's first meaningful line.
//
// A fence's first line is its marker, which says nothing, so the info string and
// the first non-blank line inside it stand in for it.
func slugify(lines []string, kind string) string {
	source := lines[0]
	if kind == kindFence {
		info := ""
		if found := fence.FindStringSubmatch(source); found != nil {
			info = strings.TrimSpace(found[2])
		}
		inner := ""
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) != "" {
				inner = line
				break
			}
		}
		source = inner
		if info != "" {
			var tb textbuf.Buffer
			source = tb.Str(info).Byte(' ').Str(inner).String()
		}
	}
	text := htmlComment.ReplaceAllString(source, " ")
	text = markdownLink.ReplaceAllString(text, "$1")
	text = strings.NewReplacer("`", " ", "*", " ", "_", " ").Replace(text)
	slug := strings.Trim(notSlugRun.ReplaceAllString(strings.ToLower(text), "-"), "-")
	if len(slug) > slugMax {
		slug = slug[:slugMax]
		if cut := strings.LastIndexByte(slug, '-'); cut > 0 {
			slug = slug[:cut]
		}
		slug = strings.Trim(slug, "-")
	}
	if slug == "" {
		return kind
	}
	return slug
}

// uniqueSlug answers slug, or the first `slug-N` free in taken, and registers
// the result. The suffix starts at 2, so the first collision reads as the
// second of its name.
func uniqueSlug(slug string, taken map[string]bool) string {
	candidate := slug
	var tb textbuf.Buffer
	for n := 1; taken[candidate]; {
		n++
		tb.Reset()
		candidate = tb.Str(slug).Byte('-').Int(int64(n)).String()
	}
	taken[candidate] = true
	return candidate
}

// parseHeader reads the H1 and the metadata block. It answers the header and
// the index one past its last line. Thus, the next blank line remains an
// ordinary block separator instead of header content.
func parseHeader(lines []string, stem string) (map[string]string, int, error) {
	var tb textbuf.Buffer
	if len(lines) == 0 {
		return nil, 0, errors.New(tb.Str(stem).Str(": file is empty").String())
	}
	title := pointsH1.FindStringSubmatch(lines[0])
	if title == nil {
		return nil, 0, errors.New(tb.Str(stem).Str(": first line must be '# Title'").String())
	}
	if len(lines) < 2 || strings.TrimSpace(lines[1]) != "" {
		return nil, 0, errors.New(tb.Str(stem).Str(": one blank line must follow the title").String())
	}

	header := map[string]string{keyTitle: title[1]}
	index := 2
	var seen []string
	for index < len(lines) {
		meta := pointsMeta.FindStringSubmatch(lines[index])
		if meta == nil {
			break
		}
		header[strings.ToLower(meta[1])] = meta[2]
		seen = append(seen, meta[1])
		index++
	}

	var canon []string
	for _, key := range canonKeys {
		if slices.Contains(seen, key) {
			canon = append(canon, key)
		}
	}
	if strings.Join(seen, ",") != strings.Join(canon, ",") ||
		!slices.Contains(seen, "When") || !slices.Contains(seen, "Severity") {
		found := "nothing"
		if len(seen) > 0 {
			tb.Reset()
			found = tb.Str(pyListRepr(seen)).String()
		}
		tb.Reset()
		return nil, 0, errors.New(tb.Str(stem).
			Str(": metadata must be When, Severity, then optional Related (found ").
			Str(found).Byte(')').String())
	}
	if index >= len(lines) || strings.TrimSpace(lines[index]) != "" {
		tb.Reset()
		return nil, 0, errors.New(tb.Str(stem).Str(": one blank line must follow the metadata").String())
	}
	return header, index, nil
}

// verifyPartition fails closed when the split is not a total partition of the
// source lines.
//
// A round trip cannot detect a splitter that drops a line when the renderer
// restores an equivalent line. This check verifies the design property instead
// of its visible effect.
func verifyPartition(lines []string, split Split) error {
	owner := make([]string, len(lines))
	type claim struct {
		name       string
		start, end int
	}
	claims := []claim{{"header", split.HeaderStart, split.HeaderEnd}}
	var tb textbuf.Buffer
	for _, section := range split.Sections {
		// A section owns exactly its heading line. The manifest holds that
		// line, so it is claimed here or it would read as owned by nothing.
		tb.Reset()
		claims = append(claims, claim{tb.Str(section.Slug).Byte('/').String(), section.Start, section.Start + 1})
		for _, point := range section.Points {
			claims = append(claims, claim{point.Slug, point.Start, point.End})
		}
	}
	for _, c := range claims {
		for i := c.start; i < c.end; i++ {
			if owner[i] != "" {
				tb.Reset()
				return errors.New(tb.Str(split.Stem).Str(": line ").Int(int64(i + 1)).
					Str(" claimed by ").Str(owner[i]).Str(" and ").Str(c.name).String())
			}
			owner[i] = c.name
		}
	}
	for i, who := range owner {
		if who == "" && strings.TrimSpace(lines[i]) != "" {
			tb.Reset()
			return errors.New(tb.Str(split.Stem).Str(": line ").Int(int64(i + 1)).
				Str(" is non-blank and belongs to no point").String())
		}
	}
	return nil
}

// SplitRule partitions one rendered rule into a header and an ordered list of
// points.
func SplitRule(text, stem string) (Split, error) {
	var split Split
	lines, err := bodyLines(text, stem)
	if err != nil {
		return split, err
	}
	header, headerEnd, err := parseHeader(lines, stem)
	if err != nil {
		return split, err
	}

	split = Split{Stem: stem, Header: header, HeaderStart: 0, HeaderEnd: headerEnd, LineCount: len(lines)}

	sectionsTaken := map[string]bool{}
	taken := map[string]bool{}
	previousEnd := headerEnd
	var tb textbuf.Buffer
	for _, span := range blockRanges(lines, headerEnd) {
		start, end := span[0], span[1]
		body := lines[start:end]
		isSection := sectionHeading.MatchString(body[0])
		gap := start - previousEnd
		// A `##` heading can follow its predecessor with no blank line.
		// blockRanges cuts the run there, so the gap is 0 instead of 1. Every
		// other block still requires exactly one blank line.
		if gap != 1 && (gap != 0 || !isSection) {
			tb.Reset()
			return split, errors.New(tb.Str(stem).Str(": line ").Int(int64(start + 1)).
				Str(" follows ").Int(int64(gap)).
				Str(" blank lines, not 1; the renderer joins blocks with exactly one").String())
		}
		previousEnd = end

		if isSection {
			if len(body) != 1 {
				tb.Reset()
				return split, errors.New(tb.Str(stem).Str(": line ").Int(int64(start + 1)).
					Str(" opens a `##` section and carries ").Int(int64(len(body) - 1)).
					Str(" more line(s) with no blank line between; a section heading names a directory and must stand alone").String())
			}
			// Point slugs are unique WITHIN a section, because the id carries
			// the section. Section slugs are unique within the rule.
			taken = map[string]bool{}
			split.Sections = append(split.Sections, Section{
				Slug:    uniqueSlug(slugify(body, kindHeading), sectionsTaken),
				Heading: body[0],
				Start:   start,
				Tight:   gap == 0,
			})
			continue
		}

		if len(split.Sections) == 0 {
			tb.Reset()
			return split, errors.New(tb.Str(stem).Str(": line ").Int(int64(start + 1)).
				Str(" comes before the first `##` section; every point must live in a section directory").String())
		}
		kind := classify(body)
		last := &split.Sections[len(split.Sections)-1]
		last.Points = append(last.Points, Point{
			Slug:  uniqueSlug(slugify(body, kind), taken),
			Kind:  kind,
			Level: levelOf(body),
			Body:  body,
			Start: start,
			End:   end,
		})
	}

	if previousEnd != len(lines) {
		tb.Reset()
		return split, errors.New(tb.Str(stem).Str(": file ends with a blank line").String())
	}
	if len(split.Sections) == 0 {
		tb.Reset()
		return split, errors.New(tb.Str(stem).Str(": carries a header but no `##` section").String())
	}
	for _, section := range split.Sections {
		if len(section.Points) == 0 {
			tb.Reset()
			return split, errors.New(tb.Str(stem).Str(": section ").Str(pyRepr(section.Slug)).
				Str(" holds no point; an empty directory carries no instruction and does not survive a clone").String())
		}
	}
	return split, verifyPartition(lines, split)
}

// RenderText assembles rule text from a header and ordered sections.
//
// A section emits its heading line from the MANIFEST, verbatim, then its
// points' bodies. Every block is separated by exactly one blank line, which is
// the property blockRanges and SplitRule between them guarantee. A tight
// section is the one exception, and it is recorded rather than guessed.
func RenderText(header map[string]string, sections []Section) string {
	var tb textbuf.Buffer
	out := []string{
		tb.Str("# ").Str(header[keyTitle]).String(),
		"",
	}
	tb.Reset()
	out = append(out, tb.Str("**When:** ").Str(header[keyWhen]).String())
	tb.Reset()
	out = append(out, tb.Str("**Severity:** ").Str(header[keySeverity]).String())
	if header[keyRelated] != "" {
		tb.Reset()
		out = append(out, tb.Str("**Related:** ").Str(header[keyRelated]).String())
	}
	for _, section := range sections {
		if !section.Tight {
			out = append(out, "")
		}
		out = append(out, section.Heading)
		for _, point := range section.Points {
			out = append(out, "")
			out = append(out, point.Body...)
		}
	}
	tb.Reset()
	return tb.Join(out, "\n").Byte('\n').String()
}

// frontmatter splits a point or manifest file into its header fields and its
// body.
//
// Line 1 is the delimiter. The header ends at the next line that is exactly the
// delimiter. Everything after that line is an unparsed, verbatim body. Thus, a
// body whose first line is the delimiter still round-trips.
func frontmatter(text, what string) (map[string]string, []string, error) {
	var tb textbuf.Buffer
	lines, err := bodyLines(text, what)
	if err != nil {
		return nil, nil, err
	}
	if len(lines) == 0 || lines[0] != delim {
		return nil, nil, errors.New(tb.Str(what).Str(": first line must be '").Str(delim).Str("'").String())
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == delim {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, nil, errors.New(tb.Str(what).Str(": header is not terminated by '").Str(delim).Str("'").String())
	}

	fields := map[string]string{}
	for _, line := range lines[1:end] {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) == "" {
			tb.Reset()
			return nil, nil, errors.New(tb.Str(what).Str(": header line is not 'key: value': ").
				Str(pyRepr(line)).String())
		}
		fields[strings.TrimSpace(key)] = strings.TrimPrefix(value, " ")
	}
	return fields, lines[end+1:], nil
}

// FormatPoint renders one point as a file: a frontmatter header, then the body
// verbatim.
//
// FormatPoint always writes kind, level, and stage, including empty values. The
// split derives all three. An absent line would make the write lossy.
//
// It writes rationale and excepted-by ONLY when they carry a value. The split
// cannot derive these fields. An empty line would falsely claim that the point
// was examined but has no link.
func FormatPoint(point Point) string {
	var tb textbuf.Buffer
	head := []string{delim}
	for _, field := range [][2]string{
		{"kind", point.Kind}, {"level", point.Level}, {"stage", point.Stage},
	} {
		tb.Reset()
		if field[1] == "" {
			head = append(head, tb.Str(field[0]).Byte(':').String())
			continue
		}
		head = append(head, tb.Str(field[0]).Str(": ").Str(field[1]).String())
	}
	if point.Rationale != "" {
		tb.Reset()
		head = append(head, tb.Str("rationale: ").Str(point.Rationale).String())
	}
	if point.ExceptedBy != "" {
		tb.Reset()
		head = append(head, tb.Str(exceptedBy).Str(": ").Str(point.ExceptedBy).String())
	}
	head = append(head, delim)
	head = append(head, point.Body...)
	tb.Reset()
	return tb.Join(head, "\n").Byte('\n').String()
}

// ParsePoint reads one point file. It is the inverse of FormatPoint.
func ParsePoint(text, slug string) (Point, error) {
	var tb textbuf.Buffer
	fields, body, err := frontmatter(text, slug)
	if err != nil {
		return Point{}, err
	}
	var unknown []string
	for key := range fields {
		if !slices.Contains(pointKeys[:], key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Point{}, errors.New(tb.Str(slug).Str(": unknown header field(s) ").
			Str(pyListRepr(unknown)).String())
	}
	kind := fields["kind"]
	level := fields["level"]
	if !slices.Contains(kinds[:], kind) {
		return Point{}, errors.New(tb.Str(slug).Str(": kind must be one of ").
			Str(pyListRepr(kinds[:])).Str(", got ").Str(pyRepr(kind)).String())
	}
	if level != "" && !slices.Contains(levels[:], level) {
		return Point{}, errors.New(tb.Str(slug).Str(": level must be empty or one of ").
			Str(pyListRepr(levels[:])).Str(", got ").Str(pyRepr(level)).String())
	}
	if len(body) == 0 {
		return Point{}, errors.New(tb.Str(slug).Str(": has an empty body").String())
	}
	return Point{
		Slug: slug, Kind: kind, Level: level, Stage: fields["stage"],
		Body: body, Start: 0, End: len(body),
		Rationale:  strings.TrimSpace(fields["rationale"]),
		ExceptedBy: strings.TrimSpace(fields[exceptedBy]),
	}, nil
}

// ExceptionRefs answers the point ids an `excepted-by` value names, in the
// order it names them.
//
// An empty element is dropped instead of kept as an empty ref. A trailing comma
// is a separator typo, not a claim about a point. An empty ref would make the
// gate fail with a message that names nothing.
func ExceptionRefs(raw string) []string {
	var out []string
	for ref := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(ref); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// FormatManifest renders the manifest: the rule's spine in the header, the
// whole tree as body.
func FormatManifest(split Split) string {
	var tb textbuf.Buffer
	head := []string{delim}
	for _, field := range [][2]string{
		{keyTitle, split.Header[keyTitle]}, {keyWhen, split.Header[keyWhen]},
		{keySeverity, split.Header[keySeverity]},
	} {
		tb.Reset()
		head = append(head, tb.Str(field[0]).Str(": ").Str(field[1]).String())
	}
	if split.Header[keyRelated] != "" {
		tb.Reset()
		head = append(head, tb.Str("related: ").Str(split.Header[keyRelated]).String())
	}
	head = append(head, delim)
	for _, section := range split.Sections {
		mark := ""
		if section.Tight {
			mark = tightMark
		}
		tb.Reset()
		head = append(head, tb.Str(mark).Str(section.Slug).Byte(' ').Str(section.Heading).String())
		for _, point := range section.Points {
			tb.Reset()
			head = append(head, tb.Str(pointIndent).Str(point.Slug).String())
		}
	}
	tb.Reset()
	return tb.Join(head, "\n").Byte('\n').String()
}

// ParseManifest reads a manifest into its header fields and its ordered section
// tree.
//
// A body line that matches neither shape is an error, not a skip. A skipped line
// is an instruction that stops appearing in rendered output. This design exists
// to prevent that failure.
func ParseManifest(text, stem string) (map[string]string, []ManifestSection, error) {
	var tb textbuf.Buffer
	where := tb.Str(stem).Str("/").Str(manifestName).String()
	fields, body, err := frontmatter(text, where)
	if err != nil {
		return nil, nil, err
	}
	var unknown []string
	for key := range fields {
		if !slices.Contains(headerKeys[:], key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		tb.Reset()
		return nil, nil, errors.New(tb.Str(where).Str(": unknown field(s) ").
			Str(pyListRepr(unknown)).String())
	}
	for _, required := range [...]string{keyTitle, keyWhen, keySeverity} {
		if fields[required] == "" {
			tb.Reset()
			return nil, nil, errors.New(tb.Str(where).Str(": missing '").Str(required).Str("'").String())
		}
	}

	var sections []ManifestSection
	for number, line := range body {
		if point := manifestPoint.FindStringSubmatch(line); point != nil {
			if len(sections) == 0 {
				tb.Reset()
				return nil, nil, errors.New(tb.Str(where).Byte(':').Int(int64(number + 1)).
					Str(": point slug ").Str(pyRepr(point[1])).
					Str(" comes before any section line; every point lives in a section directory").String())
			}
			last := &sections[len(sections)-1]
			last.Points = append(last.Points, point[1])
			continue
		}
		section := manifestSection.FindStringSubmatch(line)
		if section == nil {
			tb.Reset()
			return nil, nil, errors.New(tb.Str(where).Byte(':').Int(int64(number + 1)).
				Str(": ").Str(pyRepr(line)).
				Str(" is neither a section line ('<dir-slug> ## Heading') nor an indented point slug").String())
		}
		sections = append(sections, ManifestSection{
			Slug: section[2], Heading: section[3], Tight: section[1] != "",
		})
	}

	if len(sections) == 0 {
		tb.Reset()
		return nil, nil, errors.New(tb.Str(where).Str(": lists no sections").String())
	}
	return fields, sections, nil
}

// refuseStale reports files a split does not produce. It never deletes them
// because a slug or section rename leaves an author's file stale. Reporting is
// the only safe answer (ai/rules/never-destroy-work.md).
func refuseStale(stem, where string, stale []string) error {
	if len(stale) == 0 {
		return nil
	}
	sort.Strings(stale)
	var tb textbuf.Buffer
	return errors.New(tb.Str(stem).Str(": ").Str(where).Str(" already holds ").
		Str(pyListRepr(stale)).
		Str(", which this split does not produce; remove them by hand or split into a clean directory").String())
}

// WriteSplit writes the manifest, then one file per point under
// `<stem>/<section>/`.
func WriteSplit(split Split, outDir string) error {
	ruleDir := filepath.Join(outDir, split.Stem)
	if err := os.MkdirAll(ruleDir, 0o750); err != nil {
		return err
	}

	want := map[string]bool{}
	for _, section := range split.Sections {
		want[section.Slug] = true
	}
	entries, err := os.ReadDir(ruleDir)
	if err != nil {
		return err
	}
	var stale []string
	for _, entry := range entries {
		switch {
		case entry.IsDir() && !want[entry.Name()]:
			stale = append(stale, entry.Name())
		case !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && entry.Name() != manifestName:
			stale = append(stale, entry.Name())
		}
	}
	if err := refuseStale(split.Stem, ruleDir, stale); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(ruleDir, manifestName), []byte(FormatManifest(split)), 0o600); err != nil {
		return err
	}
	var tb textbuf.Buffer
	for _, section := range split.Sections {
		sectionDir := filepath.Join(ruleDir, section.Slug)
		if err := os.MkdirAll(sectionDir, 0o750); err != nil {
			return err
		}
		written := map[string]bool{}
		for _, point := range section.Points {
			tb.Reset()
			written[tb.Str(point.Slug).Str(".md").String()] = true
		}
		// Directories are listed beside the files for the same reason as in the
		// rule-level call above. The tree has a fixed depth of two. Thus, a
		// directory inside a section holds points that nothing renders.
		entries, err := os.ReadDir(sectionDir)
		if err != nil {
			return err
		}
		stale = nil
		for _, entry := range entries {
			if entry.IsDir() {
				stale = append(stale, entry.Name())
				continue
			}
			if strings.HasSuffix(entry.Name(), ".md") && !written[entry.Name()] {
				stale = append(stale, entry.Name())
			}
		}
		if err := refuseStale(split.Stem, sectionDir, stale); err != nil {
			return err
		}
		for _, point := range section.Points {
			tb.Reset()
			path := filepath.Join(sectionDir, tb.Str(point.Slug).Str(".md").String())
			if err := os.WriteFile(path, []byte(FormatPoint(point)), 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}

// safeSlug refuses any slug that is not a bare lowercase path component.
//
// Security: a manifest names directories and files that the renderer opens. A
// separator, a leading dot, or a parent reference would permit a read outside
// the rule directory.
func safeSlug(stem, slug, what string) error {
	if slugSafe.MatchString(slug) {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str(stem).Str(": ").Str(what).Str(" slug ").Str(pyRepr(slug)).
		Str(" must be a bare lowercase path component; a separator, a leading dot or a parent reference is refused").String())
}

// pointDirs answers every rule's point directory, sorted. A directory is one
// exactly when it holds a manifest.
func pointDirs(pointsDir string) []string {
	entries, err := os.ReadDir(pointsDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(pointsDir, entry.Name())
		if info, err := os.Stat(filepath.Join(dir, manifestName)); err == nil && !info.IsDir() {
			out = append(out, dir)
		}
	}
	sort.Strings(out)
	return out
}
