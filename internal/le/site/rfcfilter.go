// Design: docs/contributing/gh-pages.md -- the compliance index filters its RFC tables
// Related: rfccompliance.go -- the tables the filter selects rows of
// Related: rfcprofiles.go -- the profile index links each RFC count here

package site

import (
	"html"
	"net/url"
	"strconv"
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
// that does nothing. The overview inside it is filled only when a link names
// the RFCs to show; its last sentence is static, the ceiling every figure in
// it is read against.
var rfcFilterHTML = `<div class="rfc-filter" id="rfc-filter" hidden>
<div class="rfc-overview" id="rfc-overview" hidden>
<h3 id="rfc-overview-title"></h3>
<div class="rfc-overview-figures">
<div><span>` + rfcIndexMandatoryLabel + `</span><strong id="rfc-overview-mandatory"></strong><div class="rfc-bar"><span id="rfc-overview-mandatory-bar"></span></div><small id="rfc-overview-mandatory-count"></small></div>
<div><span>` + rfcIndexOptionalLabel + `</span><strong id="rfc-overview-optional"></strong><div class="rfc-bar"><span id="rfc-overview-optional-bar"></span></div><small id="rfc-overview-optional-count"></small></div>
</div>
<p id="rfc-overview-scope"></p>
<p>` + html.EscapeString(rfcIndexCeiling) + `</p>
</div>
<label for="rfc-filter-text">Filter by RFC number or title</label>
<input id="rfc-filter-text" type="search" autocomplete="off" placeholder="4271, graceful restart">
<p id="rfc-filter-note" role="status"></p>
</div>
`

// rfcFilterScript selects the rows of both RFC tables. A link names the RFCs
// to show in its rfc parameter, a comma-separated list of stems, and may name
// the profile they came from in its profile parameter; the text box narrows
// what is shown further. A listed stem with no row is named, so a profile RFC
// that has no summary yet is not silently dropped.
//
// The overview sums the shown rows' figures the way rfc.Rollup sums a
// profile's: only walked RFCs contribute requirements, and the others are
// counted apart, so a link naming a profile's RFCs reads the profile's own
// percentages.
const rfcFilterScript = `<style>
.rfc-filter { margin: .8rem 0 1rem; padding: .8rem 1rem; border-radius: 14px; background: var(--panel-strong); border: 1px solid var(--line); }
.rfc-filter label, .rfc-overview-figures span { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-filter input { width: 100%; max-width: 28rem; margin: .35rem 0; padding: .45rem .6rem; border-radius: 10px; border: 1px solid var(--line-strong); background: var(--panel); color: var(--text); font: inherit; }
.rfc-filter p { margin: .3rem 0 0; color: var(--muted); }
.rfc-overview { margin: 0 0 1rem; padding-bottom: .9rem; border-bottom: 1px solid var(--line); }
.rfc-overview h3 { margin: 0 0 .6rem; }
.rfc-overview-figures { display: grid; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); gap: 1rem; }
.rfc-overview-figures strong { display: block; font-size: clamp(1.5rem, 3vw, 2rem); line-height: 1.1; color: var(--text); }
.rfc-overview-figures small { color: var(--muted); }
.rfc-legend { display: grid; grid-template-columns: max-content 1fr; gap: .2rem .8rem; margin: .6rem 0; }
.rfc-legend dt { font-weight: 700; }
.rfc-legend dd { margin: 0; color: var(--muted); }
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
  function figure(id, proven, total) {
    var share = total ? 100 * proven / total : 0;
    document.getElementById(id).textContent = total ? share.toFixed(1) + "%" : "no rows";
    document.getElementById(id + "-bar").style.width = share.toFixed(1) + "%";
    document.getElementById(id + "-count").textContent = proven + " of " + total + " requirements proven";
  }
  function summarise(shown) {
    var mp = 0, mt = 0, op = 0, ot = 0, unwalked = 0;
    shown.forEach(function (row) {
      if (row.dataset.walked !== "1") { unwalked++; return; }
      mp += +row.dataset.mp; mt += +row.dataset.mt; op += +row.dataset.op; ot += +row.dataset.ot;
    });
    document.getElementById("rfc-overview-title").textContent = profile ? "Profile " + profile : "The RFCs this link lists";
    figure("rfc-overview-mandatory", mp, mt);
    figure("rfc-overview-optional", op, ot);
    var scope = shown.length + " RFCs shown";
    if (unwalked) { scope += "; " + unwalked + " not walked or not in a profile, so not counted"; }
    if (absent.length) { scope += "; " + absent.length + " with no summary yet"; }
    document.getElementById("rfc-overview-scope").textContent = scope + ".";
    overview.hidden = false;
    // The overview states the ceiling, so the legend's copy is hidden.
    var ceiling = document.getElementById("rfc-legend-ceiling");
    if (ceiling) { ceiling.hidden = true; }
  }
  function apply() {
    var text = input.value.trim().toLowerCase();
    var shown = [];
    rows.forEach(function (row) {
      var keep = (!wanted || wanted.has(row.dataset.rfc)) && (!text || row.textContent.toLowerCase().indexOf(text) >= 0);
      row.hidden = !keep;
      if (keep) { shown.push(row); }
    });
    if (wanted) { summarise(shown); }
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

// rfcStemRows answers, for every RFC some profile lists, the profile row that
// carries its two figures. Every profile row of one stem carries the same
// figures, because rfc.PriorityOf takes them from one table, so the index
// shows the numbers the profile pages show and computes none of its own.
func rfcStemRows(ledger rfcLedger) map[string]*rfc.TreeRow {
	rows := map[string]*rfc.TreeRow{}
	keep := func(list []*rfc.TreeRow) {
		for _, row := range list {
			if _, held := rows[row.Stem]; !held {
				rows[row.Stem] = row
			}
		}
	}
	for _, report := range ledger.Profiles {
		keep(report.Rows)
		if report.Optional != nil {
			keep(report.Optional.Rows)
		}
	}
	return rows
}

// rfcNoProfileCell is the figure cell of an RFC no profile lists: its figure
// is computed for profiles only, so it has none to show.
const rfcNoProfileCell = "no profile"

// rfcFigureCells answers one RFC's mandatory and optional cells, each a bar
// with its figure, as the profile pages render them.
func rfcFigureCells(row *rfc.TreeRow) (mandatory, optional string) {
	if row == nil {
		return rfcNoProfileCell, rfcNoProfileCell
	}
	mandatoryText, optionalText := row.Cells()
	return rfcProfileBarHTML(row, mandatoryText, row.MandatoryProven, row.MandatoryTotal),
		rfcProfileBarHTML(row, optionalText, row.OptionalProven, row.OptionalTotal)
}

// rfcFigureMirrorCells answers the same two cells as text, for the mirror.
func rfcFigureMirrorCells(row *rfc.TreeRow) (mandatory, optional string) {
	if row == nil {
		return rfcNoProfileCell, rfcNoProfileCell
	}
	return row.Cells()
}

// rfcFigureRowCells answers one table row of the RFC named by stem, carrying
// the stem and its figures as attributes, so the overview above the tables
// sums exactly what the profile rollup sums: the walked RFCs' rows.
func rfcFigureRowCells(stem string, row *rfc.TreeRow, cells ...string) string {
	attributes := `data-rfc="` + html.EscapeString(stem) + `"`
	if row != nil && row.Summary {
		walked := "0"
		if row.Walked {
			walked = "1"
		}
		attributes += ` data-walked="` + walked + `" data-mp="` + strconv.Itoa(row.MandatoryProven) +
			`" data-mt="` + strconv.Itoa(row.MandatoryTotal) + `" data-op="` + strconv.Itoa(row.OptionalProven) +
			`" data-ot="` + strconv.Itoa(row.OptionalTotal) + `"`
	}
	return "<tr " + attributes + ">" + strings.TrimPrefix(rfcRowCells(cells...), "<tr>")
}

// The enrolled table's short column names. rfcIndexLegend says what each one
// counts, so the header stays short enough to read.
const (
	rfcIndexStatusLabel    = "Status"
	rfcIndexMustsLabel     = "MUSTs"
	rfcIndexGapsLabel      = "Gaps"
	rfcIndexGapsTested     = "Gaps tested"
	rfcIndexUntestedLabel  = "Untested"
	rfcIndexMandatoryLabel = rfcProfileMandatoryLabel
	rfcIndexOptionalLabel  = rfcProfileOptionalLabel
)

// rfcIndexLegend is each column's meaning, in the order the columns appear.
func rfcIndexLegend() [][2]string {
	return [][2]string{
		{rfcIndexStatusLabel, "the support status the RFC's summary declares"},
		{rfcIndexMandatoryLabel, "the share of the RFC's " + rfcKeywordSeries(rfc.MandatoryKeywords()) +
			" requirements a test proves by a recorded break. \"not walked\" means the requirement list " +
			"has not yet been checked against the RFC text, so no figure is counted; \"" + rfcNoProfileCell +
			"\" means no profile lists the RFC"},
		{rfcIndexOptionalLabel, "the same share for its " + rfcKeywordSeries(rfc.OptionalKeywords()) + " requirements"},
		{rfcIndexMustsLabel, "the mandatory requirements the gate holds Ze to"},
		{rfcIndexGapsLabel, "requirements the summary declares Ze does not meet yet"},
		{rfcIndexGapsTested, "of those gaps, the ones a test demonstrates"},
		{rfcIndexUntestedLabel, "mandatory requirements with no test at all"},
	}
}

// rfcIndexCeiling says what a gap is. No requirement is recorded as one Linux
// cannot provide, the one exception to full coverage the owner allows, so no
// RFC's attainable figure is below 100%.
const rfcIndexCeiling = "Every gap is work Ze owes. No requirement is recorded as impossible on Linux, " +
	"so every RFC here can reach 100%."

// rfcIndexLegendHTML renders the legend under the enrolled table's heading.
func rfcIndexLegendHTML() string {
	var out textbuf.Buffer
	out.Str(`<dl class="rfc-legend">`).Byte('\n')
	for _, entry := range rfcIndexLegend() {
		out.Str("<dt>").Str(html.EscapeString(entry[0])).Str("</dt><dd>").
			Str(html.EscapeString(entry[1])).Str("</dd>\n")
	}
	out.Str("</dl>\n<p id=\"rfc-legend-ceiling\">").Str(html.EscapeString(rfcIndexCeiling)).Str("</p>\n")
	return out.String()
}

// rfcIndexLegendMirror states the legend in the mirror.
func rfcIndexLegendMirror() string {
	var out textbuf.Buffer
	for _, entry := range rfcIndexLegend() {
		out.Str("- **").Str(entry[0]).Str("**: ").Str(entry[1]).Str(".\n")
	}
	out.Str("\n").Str(rfcIndexCeiling).Str("\n\n")
	return out.String()
}
