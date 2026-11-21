// Design: docs/architecture/core-design.md -- le's native development gates
//
// Package readrecord owns the per-agent records of what an agent has read of a
// markdown page: the whole-read marker the door gate keeps for a door, and the
// section read record, one append-only file per page and agent that holds the
// digest of every section a Read returned whole
// (spec-design-needs-owner-approval, "The records"
// and the Changed-Wire Census rows "Section read record").
//
// The writer is the Read hook (internal/le/hookruntime, hookSourceRead); the
// readers are the hook's owed-set computation and `le spec design judge`. The
// format, the key a checkout's copy carries (CheckoutLabel, CheckoutKey) and
// where a reader outside the hook looks (Stores) live here, outside
// hookruntime, so both sides read one declaration: the spec design package
// cannot import hookruntime, which imports it.
//
// A section is a heading and every line up to the next heading of its level
// or a higher one, its trailing blank lines trimmed. Its digest is the SHA-256
// of that text, so a row added to the section stales the record, and an edit
// elsewhere in the page leaves it current.
package readrecord

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// Section is one heading's span of a markdown page.
type Section struct {
	// Heading is the heading's text, its #s and surrounding blanks removed.
	Heading string
	// Level is the number of #s, 1 to 6.
	Level int
	// Start and End are the 1-based lines of the heading and of the section's
	// last non-blank line, inclusive.
	Start int
	End   int
	// Through is the 1-based line a Read must reach for RecordRead to judge
	// the section whole: the heading that closes it, else the last line of
	// the text it was read from, so a Read that returns it shows where the
	// section ends.
	Through int
	// Text is the section's lines, Start to End, joined by "\n".
	Text string
	// closed reports whether a heading of its level or a higher one ended it,
	// rather than the end of the text it was read from.
	closed bool
}

// ReadLimit answers the Read limit that, from offset Start, records the
// section: the lines from its heading through Through.
func (s Section) ReadLimit() int { return s.Through - s.Start + 1 }

// Digest is the SHA-256 of the section's text, in hex.
func (s Section) Digest() string {
	sum := sha256.Sum256([]byte(s.Text))
	return hex.EncodeToString(sum[:])
}

// Sections answers every section of text, in page order. A heading inside a
// fenced code block is not a heading. The loop is bounded by the line count.
func Sections(text string) []Section {
	return sectionsOf(pageLines(text))
}

// Find answers the first section of sections whose heading is heading.
func Find(sections []Section, heading string) (Section, bool) {
	for _, section := range sections {
		if section.Heading == heading {
			return section, true
		}
	}
	return Section{}, false
}

// pageLines splits text into its lines, each with a trailing carriage return
// removed, and without the empty element a final newline leaves.
func pageLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, "\r")
	}
	return lines
}

// heading is one heading line: its 0-based index, level and text.
type heading struct {
	index int
	level int
	text  string
}

// sectionsOf finds the headings of lines outside fences, then closes each at
// the next heading of its level or a higher one.
func sectionsOf(lines []string) []Section {
	var headings []heading
	inFence := false
	for index, line := range lines {
		if fenceLine(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		level, text, ok := headingOf(line)
		if !ok {
			continue
		}
		headings = append(headings, heading{index: index, level: level, text: text})
	}
	sections := make([]Section, 0, len(headings))
	for position, opened := range headings {
		last := len(lines) - 1
		through := len(lines)
		closed := false
		for _, next := range headings[position+1:] {
			if next.level <= opened.level {
				last = next.index - 1
				through = next.index + 1
				closed = true
				break
			}
		}
		for last > opened.index && strings.TrimSpace(lines[last]) == "" {
			last--
		}
		sections = append(sections, Section{
			Heading: opened.text, Level: opened.level,
			Start: opened.index + 1, End: last + 1, Through: through,
			Text:   strings.Join(lines[opened.index:last+1], "\n"),
			closed: closed,
		})
	}
	return sections
}

// fenceLine reports whether line opens or closes a fenced code block.
func fenceLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "```") {
		return true
	}
	return strings.HasPrefix(trimmed, "~~~")
}

// headingOf reads an ATX heading: one to six #s, a blank, then the text.
func headingOf(line string) (level int, text string, ok bool) {
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 {
		return 0, "", false
	}
	if level > 6 {
		return 0, "", false
	}
	if level == len(line) {
		return 0, "", false
	}
	if line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level:]), true
}

// SessionDir is the directory every per-agent read record lives in: the
// session root's tmp/session, which the session reaper clears.
func SessionDir(sessionRoot string) string {
	return filepath.Join(sessionRoot, "tmp", "session")
}

// CheckoutLabel names dir, a checkout other than the session's root, both
// resolved: its path relative to that root when it lies inside it
// (`.claude/worktrees/<name>`), else its absolute path.
func CheckoutLabel(sessionRoot, dir string) string {
	relative, err := filepath.Rel(sessionRoot, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	if !filepath.IsLocal(relative) {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(relative)
}

// CheckoutKey is how a record's name carries relative, a checkout-relative
// path, for the checkout label names (CheckoutLabel): unchanged for the
// session's root (label ""), and under the SHA-256 of the label for another
// checkout, so a read of one checkout's copy is no record for another's, and a
// checkout's path of any depth keeps the name inside NAME_MAX (a name too long
// to create would refuse the edit forever).
func CheckoutKey(label, relative string) string {
	if label == "" {
		return relative
	}
	digest := sha256.Sum256([]byte(label))
	return hex.EncodeToString(digest[:]) + "/" + relative
}

// Store is one place a reader looks for an agent's read records of one
// checkout's pages: a session root's tmp/session, and the label that session
// root gives the checkout ("" when the checkout is that root).
type Store struct {
	// Dir is the session root's tmp/session (SessionDir).
	Dir   string
	label string
}

// Key answers the key relative, a path of the store's checkout, carries in
// the store's record names (CheckoutKey).
func (store Store) Key(relative string) string { return CheckoutKey(store.label, relative) }

// Stores answers every store that can hold an agent's read records of the
// checkout at root, as the Read hook writes them. The hook keeps every record
// in its session root's tmp/session, the harness's project directory, which a
// process started from a Bash call is not told. For a session started in the
// checkout that root is the checkout itself. For a worktree the harness opened
// under another checkout's .claude/worktrees (lepath.WorktreeMainCheckout), a
// session started in that holding checkout and working in the worktree keeps
// them in the holder's tmp/session, under the worktree's label. A record in
// either store names this checkout's copy and this agent alone, so a reader
// counts one found in either; a record of another checkout's copy carries
// another key and counts in neither.
func Stores(root string) []Store {
	// A root whose links cannot be resolved is judged as spelled, a guard:
	// the hook labels a checkout from its resolved directory, so a spelling
	// through a link can miss the holder's store, which leaves the rows owed
	// and refused with the Read that clears them, never counted unread as read.
	checkout := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		checkout = resolved
	}
	stores := make([]Store, 0, 2)
	stores = append(stores, Store{Dir: SessionDir(root)})
	holder := lepath.WorktreeMainCheckout(checkout)
	if holder == checkout {
		return stores
	}
	return append(stores, Store{Dir: SessionDir(holder), label: CheckoutLabel(holder, checkout)})
}

// markerName escapes a key into one file name component.
var markerName = strings.NewReplacer("%", "%25", "/", "%2F", "+", "%2B")

// WholeReadMarker names the file recording one agent's whole read of the
// document key names (its checkout-relative path, or the checkout-qualified
// key the door gate spells for another checkout's copy), for fingerprint
// (lepath.AgentFingerprint). The file's existence is the record.
func WholeReadMarker(sessionDir, key, fingerprint string) string {
	return filepath.Join(sessionDir, ".doc-read-"+markerName.Replace(key)+"-"+fingerprint)
}

// SectionMarker names the append-only file holding one agent's section reads
// of the page key names, for fingerprint. The key is hashed, so a checkout
// path of any depth keeps the name inside NAME_MAX.
func SectionMarker(sessionDir, key, fingerprint string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(sessionDir, ".section-read-"+hex.EncodeToString(sum[:])+"-"+fingerprint)
}

// RecordRead appends to marker one line for every section a Read returned
// whole. content is what the Read returned, from line start (1-based) of a
// page of total lines. A section is whole when the content holds its heading
// and the heading that ends it (Section.Through), or reaches the page's last
// line. A section cut by the Read's end records nothing, so the next edit
// still owes it.
//
// The Read tool counts the empty line after a page's final newline in total,
// while pageLines drops it, so a Read of the whole page ends one line short of
// total: the end is reached at start+len(lines) >= total, not only past it.
// That bound also admits a Read that stopped one line short of a page whose
// last line holds text, and that is safe: SectionRead compares the recorded
// digest with the section as it stands, and a Read cut short digests a
// different text for the last section, so it never counts as its read.
func RecordRead(marker, content string, start, total int) error {
	lines := pageLines(content)
	reachesEnd := start+len(lines) >= total
	var record strings.Builder
	for _, section := range sectionsOf(lines) {
		if !section.closed && !reachesEnd {
			continue
		}
		record.WriteString(section.Digest())
		record.WriteByte('\t')
		record.WriteString(section.Heading)
		record.WriteByte('\n')
	}
	if record.Len() == 0 {
		return nil
	}
	file, err := os.OpenFile(marker, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // a marker under the session's tmp/session, named by SectionMarker
	if err != nil {
		return fmt.Errorf("record the sections read: %w", err)
	}
	if _, err := file.WriteString(record.String()); err != nil {
		file.Close() //nolint:errcheck,gosec // the write error is the one reported
		return fmt.Errorf("record the sections read: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("record the sections read: %w", err)
	}
	return nil
}

// SectionRead reports whether marker records a read of section as it stands:
// a line carrying the section's current digest and heading. A marker that does
// not exist records nothing, which answers false with no error; a marker that
// cannot be read is an error, never "not read".
func SectionRead(marker string, section Section) (bool, error) {
	file, err := os.Open(marker) //nolint:gosec // a marker under the session's tmp/session, named by SectionMarker
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the section record %s: %w", marker, err)
	}
	defer file.Close() //nolint:errcheck // read-only

	want := section.Digest() + "\t" + section.Heading
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == want {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("read the section record %s: %w", marker, err)
	}
	return false, nil
}
