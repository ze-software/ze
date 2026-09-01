// Design: website/AI.md -- how a test is cited on a published RFC page
// Overview: rfcdetail.go -- the requirement rows; rfcevidence.go -- the proof state
//
// A citation names the TEST, not the file it lives in. The page used to print
// `internal/component/bgp/message/rfc4271_test.go` beside
// `TestRFC4271MarkerAllOnesOnSend`, and the path is machinery: it is how a
// reader gets there, not what they are looking at, and the link already
// resolves to the exact line (owner review, 2026-09-01). The path stays
// REACHABLE rather than removed -- it is the link's title, and it is the link's
// target in the mirror, where it is machine-readable anyway.
package site

import (
	"html"
	"path"
	"strings"
)

// rfcCitationName answers what a reader sees for one tagged unit.
//
// Three shapes, and only the first two have a function. A Go test and an
// interop checker are named by their function. A `.ci` or `.et` scenario IS the
// file, so it is named by the file's base name: "the whole file is the unit"
// beside a path told a reader nothing they could use.
func rfcCitationName(unit string) string {
	if _, function, held := strings.Cut(unit, "::"); held {
		return function
	}
	return path.Base(unit)
}

// rfcCitationQualified answers the name a reader sees where TWO tagged units on
// one page carry the same function name.
//
// Three such collisions exist in this corpus, and all three put both units on
// ONE page: TestRFC5176MessageAuthenticatorZeroesBothFields under authradius
// and radius, TestRFC5880SimplePasswordLengthOutOfRangeRefused under bfd/auth
// and bfd, and TestRecognizeNLRIRejectsUnimplementedTypes under nlri/mup and
// nlri/mvpn. Dropping the path there would render two DIFFERENT tests
// identically, which is the one thing this change must not do, so the package
// directory comes back -- and only there.
func rfcCitationQualified(unit string) string {
	name := rfcCitationName(unit)
	file, _, held := strings.Cut(unit, "::")
	if !held {
		return name
	}
	if directory := path.Base(path.Dir(file)); directory != "" && directory != "." {
		return directory + "/" + name
	}
	return name
}

// rfcAmbiguousNames answers the test names that more than one unit on this page
// carries, which are the names a reader could not tell apart.
func rfcAmbiguousNames(entry *rfcLedgerStem) map[string]bool {
	files := map[string]map[string]bool{}
	for index := range entry.Requirements {
		for _, cover := range entry.Requirements[index].Covers {
			name := rfcCitationName(cover.Unit)
			if files[name] == nil {
				files[name] = map[string]bool{}
			}
			files[name][cover.Unit] = true
		}
	}
	ambiguous := map[string]bool{}
	for name, units := range files {
		if len(units) > 1 {
			ambiguous[name] = true
		}
	}
	return ambiguous
}

// rfcCitationHTML renders one citation: the test's name, linked to the line its
// tag is written on, with the full path reachable as the link's title.
func rfcCitationHTML(cover *rfcLedgerCover, ambiguous map[string]bool) string {
	name := rfcCitationName(cover.Unit)
	if ambiguous[name] {
		name = rfcCitationQualified(cover.Unit)
	}
	if cover.File == "" {
		return "<code>" + html.EscapeString(name) + "</code>"
	}
	return `<a href="` + html.EscapeString(repositoryLineURL(cover.File, cover.Line)) +
		`" title="` + html.EscapeString(cover.File) + `" target="_blank" rel="noopener"><code>` +
		html.EscapeString(name) + "</code></a>"
}

// rfcCitationMirror states the same citation. The path is the link's target, so
// a mirror loses nothing by not printing it.
func rfcCitationMirror(cover *rfcLedgerCover, ambiguous map[string]bool) string {
	name := rfcCitationName(cover.Unit)
	if ambiguous[name] {
		name = rfcCitationQualified(cover.Unit)
	}
	if cover.File == "" {
		return "`" + name + "`"
	}
	return "[`" + name + "`](" + repositoryLineURL(cover.File, cover.Line) + ")"
}

// rfcCitationsMirror states one requirement's citations of one polarity, the
// tier leading each one.
//
// The mirror DIVERGES from the page here, and it has to: Markdown cannot nest a
// table inside a cell, so the page's per-citation grid becomes a labeled line
// with the citations joined. What is the same is the order -- the tier before
// the name it qualifies -- and the population.
//
// Built from the tagged units rather than from the shard's own markdown: the
// units carry the file, the line and the carrier as fields, and every citation
// the shard prints is one of them. TestEveryShardCitationIsATaggedUnit holds
// that over the corpus, so this renders the same population the shard does
// without re-reading a format nobody declared.
func rfcCitationsMirror(requirement *rfcLedgerRequirement, polarity string,
	ambiguous map[string]bool,
) string {
	parts := make([]string, 0, len(requirement.Covers))
	for index := range requirement.Covers {
		cover := &requirement.Covers[index]
		if cover.Polarity != polarity {
			continue
		}
		parts = append(parts, "`"+rfcCitationCarrier(cover)+"` "+
			rfcCitationMirror(cover, ambiguous))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// rfcCitationCarrier answers the kind and tier beside a citation, which is what
// says whether anything runs it.
func rfcCitationCarrier(cover *rfcLedgerCover) string {
	if strings.TrimSpace(cover.Carrier) == "" {
		return "no carrier claims this path"
	}
	return cover.Carrier
}
