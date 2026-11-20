// Design: docs/contributing/spec-workflow.md -- the spec's Design documents section
// Related: specfile.go -- reads the spec the derivation starts from
// Related: check.go -- judges the spec's table against this derivation
//
// documents.go derives the rows a spec's ## Design documents owes: every
// design page its files link and every decision row those pages carry
// (spec-design-needs-owner-approval, AC-13).
package specdesign

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	"github.com/ze-software/ze/internal/le/doc/doorindex"
	docindex "github.com/ze-software/ze/internal/le/doc/index"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
)

// architecturePages is the tree whose anchored pages are design
// documentation; the guides and feature pages the index also anchors are not.
const architecturePages = "docs/architecture/"

// notDesignPages are the trees of docs/ that hold no design documentation: a
// page the spec modifies there owes no row unless a header links it.
var notDesignPages = []string{"docs/guide/", "docs/features/"}

// owedPage is one design page the spec owes a row on, with what derived it.
type owedPage struct {
	Document    string   `json:"document"`
	DerivedFrom []string `json:"derived-from"`
	// Missing is a page a header names that the checkout does not hold:
	// still owed, so the header is corrected rather than the row dropped.
	Missing bool `json:"missing,omitempty"`
}

// owedDecision is one decision row of a derived page: the spec owes it a row
// of its own.
type owedDecision struct {
	Document string `json:"document"`
	Line     int    `json:"line"`
	Decision string `json:"decision"`
	Date     string `json:"date"`
	// Derived names the page row it hangs from.
	DerivedFrom []string `json:"derived-from"`
}

// derivation is the answer of `le spec design documents`.
type derivation struct {
	Spec      string         `json:"spec"`
	Pages     []owedPage     `json:"pages"`
	Decisions []owedDecision `json:"decisions"`
	// modified is the spec's own Files to Modify pages, which the check
	// refuses as silent.
	modified []string
}

// Text renders the derivation for a person: one line per page, one per
// decision, each with what derived it.
func (d derivation) Text() string {
	var tb textbuf.Buffer
	tb.Str(d.Spec).Str(": ").Str(strconv.Itoa(len(d.Pages))).Str(" design pages, ").
		Str(strconv.Itoa(len(d.Decisions))).Str(" decision rows owe a verdict in ## ").Str(documentsHeading).Byte('\n')
	for _, page := range d.Pages {
		tb.Str("  ").Str(page.Document)
		if page.Missing {
			tb.Str(" (does not exist)")
		}
		tb.Str("  (").Str(strings.Join(page.DerivedFrom, "; ")).Str(")\n")
	}
	for _, decision := range d.Decisions {
		tb.Str("  ").Str(decision.Document).Str(", ").Str(decisionsSection).Str(", decision of ").
			Str(decision.Date).Str(": \"").Str(decision.Decision).Str("\"\n")
	}
	return tb.String()
}

// derive computes the rows spec owes, from the checkout at root: the system
// door; for each source file of Files to Modify and Files to Create, every
// page its `// Design:` header names, every architecture page the doc index
// anchors to it, and the doors covering it; every design page the spec
// modifies; then every decision row of those pages.
func derive(root string, spec specFile) (derivation, error) {
	doors, err := doorindex.Load(root)
	if err != nil {
		return derivation{}, fmt.Errorf("load the doors: %w", err)
	}
	// An empty index is an answer here, not a failure: the anchored pages
	// add to the header pages, which are derived without it, and a checkout
	// whose docs/ carries no anchor links no page by anchor.
	index, err := docindex.DocumentsByPath(root)
	if err != nil {
		return derivation{}, fmt.Errorf("derive the document index: %w", err)
	}
	result := derivation{Spec: spec.relative, modified: spec.modifiedPages()}
	add := func(page, why string) {
		for position := range result.Pages {
			if result.Pages[position].Document != page {
				continue
			}
			if !slices.Contains(result.Pages[position].DerivedFrom, why) {
				result.Pages[position].DerivedFrom = append(result.Pages[position].DerivedFrom, why)
			}
			return
		}
		result.Pages = append(result.Pages, owedPage{Document: page, DerivedFrom: []string{why}})
	}
	add(doorindex.SystemDoor, "the door every change reads")
	for _, file := range speccitation.SpecSourceFiles(spec.text) {
		pages, err := speccitation.DeclaredDesignDocuments(root, file)
		if err != nil {
			return derivation{}, err
		}
		for _, page := range pages {
			add(page, "the // Design: page of "+file)
		}
		for _, page := range index[file] {
			if strings.HasPrefix(page, architecturePages) {
				add(page, "anchored to "+file)
			}
		}
		for _, door := range doors.Covering(file) {
			add(door, "the door covering "+file)
		}
	}
	for _, page := range result.modified {
		if designPage(page) {
			add(page, "a page this spec modifies")
		}
	}
	if err := result.addDecisions(root); err != nil {
		return derivation{}, err
	}
	return result, nil
}

// designPage reports whether a page the spec modifies is design
// documentation: a door, or a page under docs/ outside the guides and the
// feature pages.
func designPage(page string) bool {
	if strings.HasPrefix(page, doorindex.Dir+"/") {
		return true
	}
	if !strings.HasPrefix(page, "docs/") {
		return false
	}
	for _, tree := range notDesignPages {
		if strings.HasPrefix(page, tree) {
			return false
		}
	}
	return true
}

// addDecisions reads every derived page once and adds a decision for each row
// of its Design decisions section. A page the checkout does not hold is
// marked Missing; one that cannot be read is an error, never a page with no
// decision.
func (d *derivation) addDecisions(root string) error {
	for position := range d.Pages {
		page := d.Pages[position].Document
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page))) //nolint:gosec // a checkout-relative page a header or the spec names
		if errors.Is(err, os.ErrNotExist) {
			d.Pages[position].Missing = true
			continue
		}
		if err != nil {
			return fmt.Errorf("read the design page %s: %w", page, err)
		}
		for _, row := range docconsistency.ParseDecisions(string(body)).Rows {
			d.Decisions = append(d.Decisions, owedDecision{
				Document: page, Line: row.Line, Decision: row.Decision, Date: row.Date,
				DerivedFrom: d.Pages[position].DerivedFrom,
			})
		}
	}
	return nil
}
