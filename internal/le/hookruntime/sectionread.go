// Design: docs/architecture/core-design.md -- native edit hook policy
// Related: writeedit.go -- unreadDoors owes the sections this file finds, beside the doors
// Related: specdoors.go -- writeSpecDoorsRead hands it the pages a spec names
// Related: lifecycle.go -- hookSourceRead calls recordSectionRead for a markdown Read
//
// sectionread.go owns the Design decisions read: which pages an edit owes it
// for, the record a Read of a page leaves, and whether that record is current
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-1 to AC-7).
// The record's format is readrecord's, which `le spec design judge` reads too.
package hookruntime

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
)

// decisionsSectionName is the Design decisions heading as readrecord names a
// section: its text without the #s.
var decisionsSectionName = strings.TrimPrefix(docconsistency.DecisionsHeading, "## ")

// proposedHeaderLines bounds the header a proposed text's `// Design:` lines
// are read from: the bound speccitation.DeclaredDesignDocuments reads a file's
// header within, so the text an edit proposes and the file on disk are read
// alike.
const proposedHeaderLines = 25

// designPage is one page whose Design decisions section an edit may owe, and
// what made it owed: the file whose `// Design:` header names it, or "" for a
// page a spec names itself.
type designPage struct {
	page string
	via  string
}

// owedSection is one Design decisions section this context holds no current
// record of: its page (checkout-relative), its first and last lines, the line
// a Read must reach to record it, and the file that links the page ("" when a
// spec names the page itself). through is the heading that ends the section,
// or the page's last line: readrecord.RecordRead judges a section whole only
// when the Read returned that line, since the returned text alone must show
// where the section ends.
type owedSection struct {
	page    string
	start   int
	end     int
	through int
	via     string
}

// owedReading is what a context must read before an edit: the doors it has
// not read whole, as checkout.name spells them, and the Design decisions
// sections it holds no current record of.
type owedReading struct {
	doors    []string
	sections []owedSection
}

// empty reports that nothing is owed.
func (owed owedReading) empty() bool {
	return len(owed.doors) == 0 && len(owed.sections) == 0
}

// sectionReadMarker names the append-only file holding this context's section
// reads of page, checkout-relative in checkout. It answers "" when the session
// or the agent cannot be named, and every caller reads "" as no record.
func sectionReadMarker(ctx context, checkout documentCheckout, page string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	fingerprint, err := lepath.AgentFingerprint(id, ctx.payload.AgentID)
	if err != nil {
		return ""
	}
	return readrecord.SectionMarker(readrecord.SessionDir(ctx.root), checkout.markerKey(page), fingerprint)
}

// recordSectionRead records every section a Read of a markdown page returned
// whole, from the text the Read returned (readrecord.RecordRead), never from a
// later read of the file. The returned file must be the page the call named, in
// the checkout it lies in (gatedCheckout), so a copy of a page kept elsewhere
// records under its own path, which no header names. A Read hook never blocks
// the read it follows, so it answers 0 whatever happens.
func recordSectionRead(ctx context) int {
	if ctx.tool != toolRead {
		return 0
	}
	checkout, page, inside := gatedCheckout(ctx, absolutePath(ctx))
	if !inside {
		return 0
	}
	var response struct {
		Type string `json:"type"`
		File struct {
			Path       string  `json:"filePath"`
			Content    *string `json:"content"`
			StartLine  *int    `json:"startLine"`
			TotalLines int     `json:"totalLines"`
		} `json:"file"`
	}
	if err := json.Unmarshal(ctx.payload.ToolResponse, &response); err != nil {
		return 0
	}
	if response.Type != "text" || response.File.Content == nil || response.File.StartLine == nil {
		return 0
	}
	returned := response.File.Path
	if !filepath.IsAbs(returned) {
		returned = filepath.Join(ctx.root, returned)
	}
	returnedCheckout, returnedPage, returnedInside := gatedCheckout(ctx, returned)
	if !returnedInside || returnedCheckout != checkout || returnedPage != page {
		return 0
	}
	marker := sectionReadMarker(ctx, checkout, page)
	if marker == "" {
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		return 0
	}
	//nolint:errcheck // a Read hook never blocks; a record not written leaves the section owed, and the next edit's refusal names it
	_ = readrecord.RecordRead(marker, *response.File.Content, max(1, *response.File.StartLine), response.File.TotalLines)
	return 0
}

// editedFilePages answers the pages the header of file (checkout-relative), the
// code file an edit writes, names: from the file on disk and from the text the
// edit proposes, every `// Design:` line of each (AC-2). A page the proposed
// header names that does not exist refuses the edit (AC-7), so a header
// cannot be written to link no page; a page only the disk copy names that does
// not exist is owed nothing, so the edit that corrects such a header passes. A
// header or a page that cannot be read refuses: the gate is fail-closed.
func editedFilePages(ctx context, checkout documentCheckout, file string) ([]designPage, *verdict) {
	disk, err := speccitation.DeclaredDesignDocuments(checkout.root, file)
	if err != nil {
		return nil, unreadableHeader(checkout, file, err)
	}
	var pages []designPage
	for _, page := range disk {
		pages = appendDesignPage(pages, designPage{page: page, via: file})
	}
	current, err := os.ReadFile(absolutePath(ctx)) //nolint:gosec // the path the edit itself names
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, unreadableHeader(checkout, file, err)
	}
	proposed, known := proposedSpec(ctx, string(current))
	if !known {
		// The Edit's old_string is not in the file, so the tool refuses the
		// edit itself and no proposed header exists to judge.
		return pages, nil
	}
	for _, link := range proposedDesignLinks(proposed) {
		exists, err := pageExists(checkout, link.page)
		if err != nil {
			return nil, unreadablePage(checkout, link.page, err)
		}
		if !exists {
			return nil, danglingHeader(checkout, file, link)
		}
		pages = appendDesignPage(pages, designPage{page: link.page, via: file})
	}
	return pages, nil
}

// specDesignPages answers the pages a spec edit owes the Design decisions of,
// from paths, the checkout-relative paths the spec names (specRepoPaths): each
// `.md` path itself, and every page the `// Design:` header of each named code
// file links (C-1: the header's pages only). A path the checkout does not hold
// owes nothing, and a header naming a missing page owes nothing for it:
// `./le doc consistency` reports that header, and the spec gate does not refuse
// a spec for a file it does not edit. A header that cannot be read refuses.
func specDesignPages(checkout documentCheckout, paths []string) ([]designPage, *verdict) {
	var pages []designPage
	for _, path := range paths {
		if !insideRoot(path) {
			continue
		}
		if strings.HasSuffix(path, ".md") {
			pages = appendDesignPage(pages, designPage{page: path})
			continue
		}
		if !codeFile(path) {
			continue
		}
		linked, err := speccitation.DeclaredDesignDocuments(checkout.root, path)
		if err != nil {
			return nil, unreadableHeader(checkout, path, err)
		}
		for _, page := range linked {
			pages = appendDesignPage(pages, designPage{page: page, via: path})
		}
	}
	return pages, nil
}

// appendDesignPage appends page unless pages already holds its page, so the
// first file that linked it is the one a refusal names.
func appendDesignPage(pages []designPage, page designPage) []designPage {
	for _, held := range pages {
		if held.page == page.page {
			return pages
		}
	}
	return append(pages, page)
}

// unreadSections answers each Design decisions section among pages that this
// context holds no current record of. A page that does not exist, or that
// carries no section, owes nothing and says nothing: it records no owner
// decision. A page or a record that cannot be read refuses, and so does a page
// whose section the two readers place on different lines: the gate is
// fail-closed.
func unreadSections(ctx context, checkout documentCheckout, pages []designPage) ([]owedSection, *verdict) {
	var owed []owedSection
	for _, linked := range pages {
		body, err := os.ReadFile(filepath.Join(checkout.root, filepath.FromSlash(linked.page))) //nolint:gosec // a checkout-relative page a header or the spec names
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, unreadablePage(checkout, linked.page, err)
		}
		text := string(body)
		decisions := docconsistency.ParseDecisions(text)
		if !decisions.Present() {
			continue
		}
		sections := readrecord.Sections(text)
		section, found := readrecord.Find(sections, decisionsSectionName)
		if !found || section.Start != decisions.Headings[0] {
			return nil, &verdict{2, "❌ BLOCKED: " + checkout.name(linked.page) + ": its " + docconsistency.DecisionsHeading +
				" section cannot be placed on its lines, so no read of it can be judged.\n" +
				"  Why: the gate owes the section's exact lines and binds a read to their digest.\n" +
				"  Fix: run ./le doc consistency and correct the section it names, then retry.\n  ai/rules/architecture.md"}
		}
		read, err := readrecord.SectionRead(sectionReadMarker(ctx, checkout, linked.page), section)
		if err != nil {
			return nil, &verdict{2, "❌ BLOCKED: the record of this context's reads of " + checkout.name(linked.page) +
				" cannot be read: " + err.Error() + "\n  Why: the gate is fail-closed, so an unreadable record is no read.\n" +
				"  Fix: read the section again with the Read tool, then retry.\n  ai/rules/architecture.md"}
		}
		if read {
			continue
		}
		owed = append(owed, owedSection{
			page: linked.page, start: section.Start, end: section.End,
			through: sectionThrough(sections, section, text), via: linked.via,
		})
	}
	return owed, nil
}

// sectionThrough answers the line a Read of section must reach to record it:
// the heading of the first later section of its level or a higher one, which
// ends it, else the page's last line. sections is the page's, in page order.
func sectionThrough(sections []readrecord.Section, section readrecord.Section, text string) int {
	for _, later := range sections {
		if later.Start <= section.Start {
			continue
		}
		if later.Level <= section.Level {
			return later.Start
		}
	}
	return strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
}

// designLink is one page a `// Design:` line of a proposed text names, with
// the line, which a refusal quotes.
type designLink struct {
	line string
	page string
}

// proposedDesignLinks answers every page the `// Design:` lines of text's
// header name: its first proposedHeaderLines lines before the package clause,
// each line's pages as docconsistency.DesignTargets reads them.
func proposedDesignLinks(text string) []designLink {
	var links []designLink
	number := 0
	for line := range strings.Lines(text) {
		number++
		if number > proposedHeaderLines {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "package ") {
			break
		}
		if !strings.HasPrefix(line, "// Design:") {
			continue
		}
		for _, page := range docconsistency.DesignTargets(line) {
			links = append(links, designLink{line: line, page: page})
		}
	}
	return links
}

// pageExists reports whether page, checkout-relative, is a file of checkout. A
// page that leaves the checkout, or names a directory, is no page. A lookup
// that fails for another reason than absence is an error, never "absent".
func pageExists(checkout documentCheckout, page string) (bool, error) {
	if !insideRoot(filepath.ToSlash(filepath.Clean(page))) {
		return false, nil
	}
	info, err := os.Stat(filepath.Join(checkout.root, filepath.FromSlash(page)))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

// danglingHeader is the refusal of an edit whose proposed header names a page
// that does not exist (AC-7).
func danglingHeader(checkout documentCheckout, file string, link designLink) *verdict {
	return &verdict{2, "❌ BLOCKED: " + checkout.name(file) + ": its // Design: header names " + link.page + ", which does not exist:\n" +
		"    " + link.line + "\n" +
		"  Why: the header links a file to the page that documents it and holds Thomas's decisions about it; a link to no page binds the file to nothing.\n" +
		"  Fix: correct the header to name the page that documents this file, then retry.\n  ai/rules/architecture.md"}
}

// unreadableHeader is the refusal of an edit whose file's header cannot be read.
func unreadableHeader(checkout documentCheckout, file string, err error) *verdict {
	return &verdict{2, "❌ BLOCKED: the // Design: header of " + checkout.name(file) + " cannot be read: " + err.Error() +
		"\n  Why: the gate reads the header to name the design decisions the file is bound by, and is fail-closed.\n" +
		"  Fix: make the file readable, then retry.\n  ai/rules/architecture.md"}
}

// unreadablePage is the refusal of an edit owing a page that cannot be read.
func unreadablePage(checkout documentCheckout, page string, err error) *verdict {
	return &verdict{2, "❌ BLOCKED: " + checkout.name(page) + " cannot be read: " + err.Error() +
		"\n  Why: a // Design: header or the spec names it, and the gate cannot tell whether it records a decision.\n" +
		"  Fix: restore the page from git or make it readable, then retry.\n  ai/rules/architecture.md"}
}

// writeSectionLines appends to tb one list line per owed section: the page,
// the section and its lines, the Read that clears it (through the line that
// ends the section, owedSection.through), and, for a page a file other than
// subject links, that file.
func writeSectionLines(tb *textbuf.Buffer, checkout documentCheckout, subject string, sections []owedSection) {
	for _, section := range sections {
		first := strconv.Itoa(section.start)
		tb.Str("  - ").Str(checkout.name(section.page)).Str(", ").Str(docconsistency.DecisionsHeading).
			Str(" (lines ").Str(first).Str(" to ").Str(strconv.Itoa(section.end)).
			Str("): Read it with offset ").Str(first).Str(" and limit ").Str(strconv.Itoa(section.through - section.start + 1))
		if section.via != "" && section.via != subject {
			tb.Str(" (the // Design: page of ").Str(checkout.name(section.via)).Byte(')')
		}
		tb.Byte('\n')
	}
}
