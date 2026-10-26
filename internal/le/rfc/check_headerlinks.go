// Design: docs/contributing/rfc-conformance-gates.md -- the RFC dependency graph
// Related: graph/graph.go -- the header facts this compares against
//
// check_headerlinks.go holds each summary's lineage cells to the RFC headers.
// The header is the authority: a summary's `Updates` and `Obsoletes` cells
// restate its own text's header, and its `Updated by` cell restates the
// headers of the corpus texts that name it.
package rfc

import (
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// checkHeaderLinks answers one refusal per lineage cell that disagrees with the
// headers. A summary whose text is not in rfc/full/ or rfc/drafts/ has no
// header to compare with, and is skipped: checkReferenceSources and the
// sign-off gates already speak about a missing text.
//
// `Updates` and `Obsoletes` restate the summary's own header and must equal it
// exactly. `Updated by` restates other texts' headers, and every corpus RFC it
// names must be one whose header updates this one; an RFC outside the corpus
// is free, because no header in the repository can speak for it. An updater
// the cell omits is not refused: the graph holds the reverse relation
// (graph.Graph.Updaters), and requiring the cell to copy it would declare one
// fact twice. `Obsoleted by` is not judged here, because it declares the
// successor the {superseded} markers resolve against, a summary's reading
// that can name an RFC whose header says Updates (RFC 8210 restates RFC 6810);
// checkSuperseded holds it.
func checkHeaderLinks(metas map[string]Meta, g *graph.Graph) []string {
	var errs []string
	for _, stem := range sortMetaStems(metas) {
		if !g.Has(stem) {
			continue
		}
		meta := metas[stem]
		updates, _ := g.HeaderUpdates(stem)
		obsoletes, _ := g.HeaderObsoletes(stem)
		if !sameStems(meta.Updates, updates) {
			errs = append(errs, headerLinkRefusal(stem, metaUpdates, meta.Updates, updates, "its own header"))
		}
		if !sameStems(meta.Obsoletes, obsoletes) {
			errs = append(errs, headerLinkRefusal(stem, metaObsoletes, meta.Obsoletes, obsoletes, "its own header"))
		}
		if updaters := g.Updaters(stem); !namesOnly(meta.UpdatedBy, updaters, g) {
			errs = append(errs, headerLinkRefusal(stem, metaUpdatedBy, meta.UpdatedBy, updaters,
				"the headers of the corpus texts that update it"))
		}
	}
	return errs
}

// sameStems answers whether two stem lists hold the same set.
func sameStems(a, b []string) bool {
	return slices.Equal(sortedStems(a), sortedStems(b))
}

func sortedStems(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

// namesOnly answers whether every corpus document the cell names is in
// headers; a document outside the corpus is not judged.
func namesOnly(cell, headers []string, g *graph.Graph) bool {
	for _, stem := range cell {
		if !g.Has(stem) {
			continue
		}
		if !slices.Contains(headers, stem) {
			return false
		}
	}
	return true
}

// headerLinkRefusal names the summary, the cell, both values and the fix.
func headerLinkRefusal(stem, label string, cell, header []string, source string) string {
	var tb textbuf.Buffer
	tb.Str(summaryRel).Byte('/').Str(stem).Str(".md: the Meta `").Str(label).Str("` cell names ").
		Str(stemList(cell)).Str(", but ").Str(source).Str(" says ").Str(stemList(header)).
		Str(". The RFC header is the authority: write the cell to match it (`None` for nothing)")
	return tb.String()
}

// stemList spells a stem list for a reader: `RFC 1997, RFC 4271`, or `None`.
func stemList(stems []string) string {
	if len(stems) == 0 {
		return "None"
	}
	var tb textbuf.Buffer
	for i, stem := range sortedStems(stems) {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(rfcName(stem))
	}
	return tb.String()
}
