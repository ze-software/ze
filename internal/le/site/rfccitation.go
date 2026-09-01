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
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
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

// rfcCitationCarrier answers the kind and tier beside a citation, which is what
// says whether anything runs it.
func rfcCitationCarrier(cover *rfcLedgerCover) string {
	if strings.TrimSpace(cover.Carrier) == "" {
		return "no carrier claims this path"
	}
	return cover.Carrier
}

// rfcTestRow is one line of a requirement's tests block: a citation, or the
// statement that one polarity has none.
//
// Cover is nil for an absent polarity. Rank and Ranked are the carrier's place
// in the reading order internal/le/rfc declares.
type rfcTestRow struct {
	Polarity string
	Carrier  string
	Cover    *rfcLedgerCover
	Rank     int
	Ranked   bool
	Sort     string
}

// rfcTestRows answers one requirement's test lines, in reading order.
//
// Ordered by KIND and TIER first, so a reader sees at a glance whether an
// obligation is carried by unit tests or by a nightly interop run, then by
// polarity inside a group, positive before negative (owner review,
// 2026-09-01). The rank comes from rfc.CarrierLabelRank rather than from a
// sequence written here: the vocabulary and its order live together, so a kind
// added there cannot land at the end of this page in silence.
//
// A carrier the vocabulary does not rank sorts AFTER every ranked one, by its
// own label, rather than at rank zero where unit/verify lives. It is a fact
// about the carrier table, and TestEveryCarrierTheVocabularyDeclaresIsRanked
// reddens on it rather than letting the page guess.
//
// The absent-polarity row takes the rank of the requirement's BEST carrier, so
// the gap shows inside the group the eye is already reading. A requirement with
// no test at all has no group, and both rows sort first.
//
// The sort is TOTAL and STABLE: two citations alike in kind, tier and polarity
// are ordered by the name a reader sees, so a rebuild cannot churn the page.
func rfcTestRows(requirement *rfcLedgerRequirement) []rfcTestRow {
	rows := make([]rfcTestRow, 0, len(requirement.Covers)+2)
	best, bestKnown, bestFound := 0, false, false
	for index := range requirement.Covers {
		cover := &requirement.Covers[index]
		carrier := rfcCitationCarrier(cover)
		rank, ranked := rfc.CarrierLabelRank(cover.Carrier)
		rows = append(rows, rfcTestRow{Polarity: cover.Polarity, Carrier: carrier,
			Cover: cover, Rank: rank, Ranked: ranked,
			Sort: rfcCitationName(cover.Unit)})
		if !bestFound || rfcRowIsBefore(rank, ranked, carrier, best, bestKnown, "") {
			best, bestKnown, bestFound = rank, ranked, true
		}
	}
	for _, polarity := range []string{rfc.PolarityPositive, rfc.PolarityNegative} {
		if rfcHasPolarity(requirement, polarity) {
			continue
		}
		rows = append(rows, rfcTestRow{Polarity: polarity, Carrier: "no test",
			Rank: best, Ranked: bestKnown})
	}
	sort.SliceStable(rows, func(left, right int) bool {
		one, two := &rows[left], &rows[right]
		if one.Ranked != two.Ranked {
			return one.Ranked
		}
		if one.Rank != two.Rank {
			return one.Rank < two.Rank
		}
		if !one.Ranked && one.Carrier != two.Carrier {
			return one.Carrier < two.Carrier
		}
		if one.Polarity != two.Polarity {
			return rfcPolarityRank(one.Polarity) < rfcPolarityRank(two.Polarity)
		}
		return one.Sort < two.Sort
	})
	return rows
}

// rfcRowIsBefore answers whether one carrier reads before another, which is how
// the best carrier of a requirement is chosen for its absent-polarity rows.
func rfcRowIsBefore(rank int, ranked bool, carrier string,
	bestRank int, bestRanked bool, bestCarrier string,
) bool {
	if ranked != bestRanked {
		return ranked
	}
	if rank != bestRank {
		return rank < bestRank
	}
	return carrier < bestCarrier
}

// rfcPolarityRank orders the two directions as a reader reads them: what Ze
// does, then what it refuses. rfc.Polarities answers them SORTED, which puts
// negative first, so the display order is stated here.
func rfcPolarityRank(polarity string) int {
	if polarity == rfc.PolarityPositive {
		return 0
	}
	return 1
}
