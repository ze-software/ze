// Design: docs/contributing/gh-pages.md -- the compliance index filters its RFC tables
// Related: rfccompliance.go -- the tables the filter selects rows of
// Related: rfcprofiles.go -- the profile index links each RFC count here

package site

import (
	"html"
	"net/url"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcListAnchor is the id of the enrolled-RFC section, where the filter sits,
// and rfcDeclinedAnchor the id of the not-enrolled one after it.
const (
	rfcListAnchor     = "rfc-list"
	rfcDeclinedAnchor = "rfc-declined"
)

// rfcListSlot marks the top of the compliance index, under its title. A link
// that names RFCs is asking about those RFCs, so the script moves both RFC
// tables here, and the reader meets the overview first rather than after
// every gate card.
const rfcListSlot = `<div id="rfc-list-slot"></div>` + "\n"

// rfcFilterHTML is the filter above the enrolled-RFC table. It is hidden until
// the script runs, so a reader without scripts sees every row and no control
// that does nothing.
const rfcFilterHTML = `<div class="rfc-filter" id="rfc-filter" hidden>
<label for="rfc-filter-text">Filter by RFC number or title</label>
<input id="rfc-filter-text" type="search" autocomplete="off" placeholder="4271, graceful restart">
<p id="rfc-filter-note" role="status"></p>
</div>
`

// rfcListOverviewTitle heads the overview while no link or text narrows the
// tables.
const rfcListOverviewTitle = "Every RFC listed"

// rfcTotalLabel heads the Total row's first cell on the page and the mirror.
const rfcTotalLabel = "Total"

// rfcListOverviewHTML is the overview above the RFC tables: the two bars and
// the Total row over the subject all, which the filter script narrows to the
// rows it shows by setting data-ze-rfc-stems, so a link naming a profile's RFCs
// reads that profile's own counts. It is visible without scripts, holding the
// last build's totals over every RFC listed.
func rfcListOverviewHTML() string {
	var out textbuf.Buffer
	out.Str(`<div class="rfc-overview" id="rfc-overview">`).Byte('\n')
	out.Str(`<h3 id="rfc-overview-title">`).Str(rfcListOverviewTitle).Str("</h3>\n")
	out.Str(rfcBarsHTML(rfcAllSubject))
	out.Str(rfcTotalRowHTML(rfcAllSubject, "rfc-overview-check"))
	out.Str(`<p id="rfc-overview-scope">`).Str(rfcValueHTML(rfcAllSubject, "rfcs")).
		Str(" RFCs, of which ").Str(rfcValueHTML(rfcAllSubject, "not-walked")).
		Str(" not walked, so their rows count in no figure.</p>\n</div>\n")
	return out.String()
}

// rfcListOverviewMirror states the same overview in the mirror.
func rfcListOverviewMirror() string {
	var out textbuf.Buffer
	out.Str("### ").Str(rfcListOverviewTitle).Str("\n\n").Str(rfcBarsMirror(rfcAllSubject)).Byte('\n')
	out.Str(rfcTotalRowMirror(rfcAllSubject))
	out.Str(rfcValueMirror(rfcAllSubject, "rfcs")).Str(" RFCs, of which ").
		Str(rfcValueMirror(rfcAllSubject, "not-walked")).Str(" not walked, so their rows count in no figure.\n\n")
	return out.String()
}

// rfcTotalRowHTML renders one subject's Total row, the MUST sections, the
// rows not owed and the total they add up to, and the check line that adds
// them, every value a marked element.
func rfcTotalRowHTML(subject, checkID string) string {
	var out textbuf.Buffer
	columns := rfcListTotalColumns()
	labels := make([]string, 0, 1+len(columns))
	labels = append(labels, rfcTotalLabel)
	values := make([]string, 0, 1+len(columns))
	values = append(values, rfcTotalLabel)
	for _, column := range columns {
		labels = append(labels, column.Label)
		values = append(values, rfcValueHTML(subject, column.Field))
	}
	out.Str(rfcTableHTML(rfcHeadCells(labels...), rfcRowCells(values...))).Byte('\n')
	out.Str(`<p id="`).Str(checkID).Str(`">Check: `)
	for index, column := range columns {
		switch {
		case index == len(columns)-1:
			out.Str(" = ")
		case index != 0:
			out.Str(" + ")
		}
		out.Str(rfcValueHTML(subject, column.Field))
	}
	out.Str(" ").Str(html.EscapeString(rfcListCheckWords())).Str("</p>\n")
	return out.String()
}

// rfcTotalRowMirror states the same Total row and check line in a mirror.
func rfcTotalRowMirror(subject string) string {
	var out textbuf.Buffer
	columns := rfcListTotalColumns()
	labels := make([]string, 0, 1+len(columns))
	labels = append(labels, rfcTotalLabel)
	values := make([]string, 0, 1+len(columns))
	values = append(values, rfcTotalLabel)
	check := make([]string, 0, len(columns))
	for _, column := range columns {
		labels = append(labels, column.Label)
		values = append(values, rfcValueMirror(subject, column.Field))
		check = append(check, rfcValueMirror(subject, column.Field))
	}
	out.Str(rfcMirrorHead(labels...)).Str(rfcMirrorRow(values...)).Byte('\n')
	out.Str("Check: ").Str(strings.Join(check[:len(check)-1], " + ")).Str(" = ").Str(check[len(check)-1]).
		Str(" ").Str(rfcListCheckWords()).Str("\n\n")
	return out.String()
}

// rfcListTotalColumns answers the Total row's columns: each MUST bar segment
// in the bar's own order, then the MUST rows not owed, then the MUST total
// they add up to. The segments come from rfcBars, the layout the files
// publish, so the row cannot name a section the bars do not draw.
func rfcListTotalColumns() []rfcProfileFigure {
	columns := make([]rfcProfileFigure, 0, len(rfcBars.Must)+2)
	for _, segment := range rfcBars.Must {
		if len(segment.Fields) != 1 {
			panic("BUG: a MUST bar segment sums more than one field: " + segment.Segment)
		}
		columns = append(columns, rfcProfileFigure{segment.Label, segment.Fields[0]})
	}
	return append(columns, rfcProfileFigure{"Not owed", rfcMustNotOwedField}, rfcProfileFigure{"MUST rows", "must.total"})
}

// rfcListCheckWords says what the check line adds up.
func rfcListCheckWords() string {
	return "MUST rows: the four MUST bar sections and the rows not owed add up to every MUST row of the walked RFCs."
}

// rfcFilterScript selects the rows of both RFC tables. A link names the RFCs
// to show in its rfc parameter, a comma-separated list of stems, and may name
// the profile they came from in its profile parameter; the text box narrows
// what is shown further. A listed stem with no row is named, so a profile RFC
// that has no summary yet is not silently dropped.
//
// The overview's figures are rfc-figures.js's: this script only tells it which
// RFCs are shown, through data-ze-rfc-stems, and asks it to fill the overview
// again. The script sums walked RFCs only and counts the others apart, as a
// profile page does, so a link naming a profile's RFCs reads the profile's own
// counts.
const rfcFilterScript = `<style>
.rfc-filter { margin: .8rem 0 1rem; padding: .8rem 1rem; border-radius: 14px; background: var(--panel-strong); border: 1px solid var(--line); }
.rfc-filter label { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-filter input { width: 100%; max-width: 28rem; margin: .35rem 0; padding: .45rem .6rem; border-radius: 10px; border: 1px solid var(--line-strong); background: var(--panel); color: var(--text); font: inherit; }
.rfc-filter p { margin: .3rem 0 0; color: var(--muted); }
.rfc-overview { margin: 0 0 1rem; padding-bottom: .9rem; border-bottom: 1px solid var(--line); }
.rfc-overview h3 { margin: 0 0 .6rem; }
.rfc-legend, .rfc-glossary { display: grid; grid-template-columns: max-content 1fr; gap: .35rem .8rem; margin: .6rem 0; }
.rfc-legend dt, .rfc-glossary dt { font-weight: 700; }
.rfc-legend dd, .rfc-glossary dd { margin: 0; color: var(--muted); }
.rfc-glossary-fold { margin: .8rem 0; padding: .6rem 1rem; border-radius: 14px; background: var(--panel-strong); border: 1px solid var(--line); }
.rfc-glossary-fold summary { cursor: pointer; font-weight: 700; }
@media (max-width: 640px) { .rfc-legend, .rfc-glossary { grid-template-columns: 1fr; } .rfc-legend dd, .rfc-glossary dd { margin-bottom: .4rem; } }
</style>
<script>
(function () {
  var box = document.getElementById("rfc-filter");
  var input = document.getElementById("rfc-filter-text");
  var note = document.getElementById("rfc-filter-note");
  var overview = document.getElementById("rfc-overview");
  if (!box || !input || !note || !overview) { return; }
  var rows = Array.prototype.slice.call(document.querySelectorAll("tr[data-rfc]"));
  var params = new URLSearchParams(window.location.search);
  var listed = (params.get("rfc") || "").split(",").map(function (s) { return s.trim().toLowerCase(); }).filter(Boolean);
  var wanted = listed.length ? new Set(listed) : null;
  var profile = params.get("profile") || "";
  if (!/^[a-z0-9][a-z0-9-]*$/.test(profile)) { profile = ""; }
  var held = new Set(rows.map(function (row) { return row.dataset.rfc; }));
  var absent = listed.filter(function (stem) { return !held.has(stem); });
  var title = document.getElementById("rfc-overview-title");
  function summarize(shown, text) {
    title.textContent = profile ? "Profile " + profile : (wanted || text ? "The RFCs shown" : "` + rfcListOverviewTitle + `");
    overview.setAttribute("data-ze-rfc-stems", shown.map(function (row) { return row.dataset.rfc; }).join(","));
    if (window.zeRFCFigures) { window.zeRFCFigures.fill(overview); }
  }
  function apply() {
    var text = input.value.trim().toLowerCase();
    var shown = [];
    rows.forEach(function (row) {
      var keep = (!wanted || wanted.has(row.dataset.rfc)) && (!text || row.textContent.toLowerCase().indexOf(text) >= 0);
      row.hidden = !keep;
      if (keep) { shown.push(row); }
    });
    summarize(shown, text);
    note.textContent = "";
    var lead = wanted ? "Showing " + shown.length + " of the " + listed.length + " RFCs the link lists" : "Showing " + shown.length + " of " + rows.length + " RFCs";
    note.appendChild(document.createTextNode(lead));
    if (profile) {
      note.appendChild(document.createTextNode(", for the profile "));
      var back = document.createElement("a");
      back.href = "profiles/" + profile + "/";
      back.textContent = profile;
      note.appendChild(back);
    }
    note.appendChild(document.createTextNode(". "));
    if (absent.length) {
      note.appendChild(document.createTextNode("No summary yet: " + absent.join(", ") + ". "));
    }
    if (wanted) {
      var all = document.createElement("a");
      all.href = window.location.pathname + "#` + rfcListAnchor + `";
      all.textContent = "Show every RFC";
      note.appendChild(all);
    }
  }
  var slot = document.getElementById("rfc-list-slot");
  if (wanted && slot) {
    ["` + rfcListAnchor + `", "` + rfcDeclinedAnchor + `"].forEach(function (id) {
      var section = document.getElementById(id);
      if (section) { slot.parentNode.insertBefore(section, slot); }
    });
  }
  input.addEventListener("input", apply);
  box.hidden = false;
  apply();
})();
</script>
`

// rfcProfileListHref answers the link from the profile index to the compliance
// index showing only the RFCs a profile counts: the stems of its rows, the
// same set its RFC count counts.
func rfcProfileListHref(report *rfc.TreeReport) string {
	stems := make([]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		stems = append(stems, row.Stem)
	}
	var out textbuf.Buffer
	out.Str("../?profile=").Str(url.QueryEscape(report.Profile)).
		Str("&rfc=").Str(url.QueryEscape(strings.Join(stems, ","))).
		Str("#").Str(rfcListAnchor)
	return out.String()
}

// rfcNoProfileCell marks a ledger with no figure for a cell.
const rfcNoProfileCell = "no profile"

// rfcFigureCells answers one RFC's two cells, its MUST bar and its overview
// bar, drawn from data/rfc/<stem>.json as the profile pages draw them.
func rfcFigureCells(stem string) (mustBar, overviewBar string) {
	return rfcBarMarkHTML(stem, "must"), rfcBarMarkHTML(stem, "overview")
}

// rfcFigureMirrorCells answers the same two cells for the mirror.
func rfcFigureMirrorCells(stem string) (mustBar, overviewBar string) {
	return rfcBarMirror(stem, "must"), rfcBarMirror(stem, "overview")
}

// rfcFigureRowCells answers one table row of the RFC named by stem, carrying
// the stem, which the filter script selects rows by and hands to
// rfc-figures.js as the RFCs shown.
func rfcFigureRowCells(stem string, cells ...string) string {
	return `<tr data-rfc="` + html.EscapeString(stem) + `">` + strings.TrimPrefix(rfcRowCells(cells...), "<tr>")
}

// rfcMissingRow is the Area cell of an RFC whose summary declares no public
// row. It reads as a DECISION rather than as an omission: readSupport
// (internal/le/rfc/meta.go) refuses a Meta table with no `| Support |` row,
// and a summary writing `-` there declares that it renders none.
const rfcMissingRow = "No public row declared"

// The enrolled table's short column names. rfcIndexLegend says what each one
// counts, so the header stays short enough to read.
const (
	rfcIndexAreaLabel      = "Area"
	rfcIndexMustsLabel     = "MUSTs"
	rfcIndexGapsLabel      = "Gaps"
	rfcIndexGapsTested     = "Gaps tested"
	rfcIndexUntestedLabel  = "Untested"
	rfcIndexMandatoryLabel = "MUST bar"
	rfcIndexOptionalLabel  = "Overview bar"
)

// rfcIndexLegend is each column's meaning, in the order the columns appear.
func rfcIndexLegend() [][2]string {
	return [][2]string{
		{rfcIndexAreaLabel, "the area the RFC's public row is filed under, or that its summary declares no public row"},
		{rfcIndexMandatoryLabel, "the RFC's " + rfcKeywordSeries(rfc.MandatoryKeywords()) +
			" rows the stack owes, by section, read from data/rfc/<stem>.json. \"not walked\" means the " +
			"requirement list has not yet been checked against the RFC text, so no row is counted"},
		{rfcIndexOptionalLabel, "the same MUST rows with its " + rfcKeywordSeries(rfc.OptionalKeywords()) +
			" rows and the rows not owed, which do not bind Ze"},
		{rfcIndexMustsLabel, "the mandatory requirements the gate holds Ze to"},
		{rfcIndexGapsLabel, "requirements the summary declares Ze does not meet yet"},
		{rfcIndexGapsTested, "of those gaps, the ones a test demonstrates"},
		{rfcIndexUntestedLabel, "mandatory requirements with no test at all"},
	}
}

// rfcIndexLegendHTML renders the legend under the enrolled table's heading.
func rfcIndexLegendHTML() string {
	var out textbuf.Buffer
	out.Str(`<dl class="rfc-legend">`).Byte('\n')
	for _, entry := range rfcIndexLegend() {
		out.Str("<dt>").Str(html.EscapeString(entry[0])).Str("</dt><dd>").
			Str(html.EscapeString(entry[1])).Str("</dd>\n")
	}
	out.Str("</dl>\n")
	return out.String()
}

// rfcIndexLegendMirror states the legend in the mirror.
func rfcIndexLegendMirror() string {
	var out textbuf.Buffer
	for _, entry := range rfcIndexLegend() {
		out.Str("- **").Str(entry[0]).Str("**: ").Str(entry[1]).Str(".\n")
	}
	out.Str("\n")
	return out.String()
}
