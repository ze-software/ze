// Design: docs/contributing/gh-pages.md -- the compliance index filters its RFC tables
// Related: rfccompliance.go -- the tables the filter selects rows of
// Related: rfcprofiles.go -- the profile index links each RFC count here

package site

import (
	"encoding/json"
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
// that does nothing. The overview sums only measured rows selected by the link
// or text filter, using the model's counts and registered reason labels.
var rfcFilterHTML = `<div class="rfc-filter" id="rfc-filter" hidden data-ceiling-reasons="` +
	html.EscapeString(rfcCeilingReasons()) + `" data-rfc-number-max="` +
	strconv.Itoa(int(^uint(0)>>1)) + `">
<div class="rfc-overview" id="rfc-overview" hidden>
<h3 id="rfc-overview-title"></h3>
<div class="rfc-overview-figures">
<div><span>` + rfcIndexMandatoryLabel + `</span><strong id="rfc-overview-mandatory"></strong><div class="rfc-bar"><span id="rfc-overview-mandatory-bar"></span></div><small id="rfc-overview-mandatory-count"></small></div>
<div><span>` + rfcIndexOptionalLabel + `</span><strong id="rfc-overview-optional"></strong><div class="rfc-bar"><span id="rfc-overview-optional-bar"></span></div><small id="rfc-overview-optional-count"></small></div>
</div>
<p id="rfc-overview-scope"></p>
<p id="rfc-overview-ceiling"></p>
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
  function figure(id, proven, total) {
    var share = total ? 100 * proven / total : 0;
    document.getElementById(id).textContent = total ? share.toFixed(1) + "%" : "no rows";
    document.getElementById(id + "-bar").style.width = share.toFixed(1) + "%";
    document.getElementById(id + "-count").textContent = proven + " of " + total + " requirements proven";
  }
  function ratio(count, total) {
    return total ? count + " of " + total + " (" + (100 * count / total).toFixed(1) + "%)" : "no rows";
  }
  var reasons = JSON.parse(box.dataset.ceilingReasons);
  var rfcNumberMax = BigInt(box.dataset.rfcNumberMax);
  function rfcNumber(stem) {
    var match = /^rfc([0-9]+)$/.exec(stem);
    if (!match) { return null; }
    var number = BigInt(match[1]);
    return number <= rfcNumberMax ? number : null;
  }
  // Match rfc.statusRowCompare: numeric RFCs first, then other stems by name.
  function stemBefore(left, right) {
    var l = rfcNumber(left), r = rfcNumber(right);
    if (l !== null && r !== null) { return l < r; }
    if (l !== null) { return true; }
    if (r !== null) { return false; }
    return left < right;
  }
  function exclusions(counts, total) {
    var parts = [];
    Object.keys(reasons).sort().forEach(function (kind) {
      if (counts[kind]) { parts.push(reasons[kind] + ": " + ratio(counts[kind], total)); }
    });
    return parts.length ? "; excluded: " + parts.join("; ") : "";
  }
  function lower(held, row, attainable, total) {
    if (!total) { return held; }
    var stem = row.dataset.rfc;
    if (!held || attainable * held.total < held.attainable * total ||
        (attainable * held.total === held.attainable * total && stemBefore(stem, held.stem))) {
      return {stem: stem, attainable: attainable, total: total};
    }
    return held;
  }
  function lowest(label, value) {
    return value ? " Lowest " + label + " ceiling: " + value.stem + " " + ratio(value.attainable, value.total) + "." : "";
  }
  function summarize(shown) {
    var mp = 0, mt = 0, ma = 0, op = 0, ot = 0, oa = 0, unwalked = 0;
    var me = {}, oe = {}, lm = null, lo = null;
    shown.forEach(function (row) {
      if (row.dataset.walked !== "1") { unwalked++; return; }
      mp += +row.dataset.mp; mt += +row.dataset.mt; op += +row.dataset.op; ot += +row.dataset.ot;
      ma += +row.dataset.ma; oa += +row.dataset.oa;
      var mandatory = JSON.parse(row.dataset.me), optional = JSON.parse(row.dataset.oe);
      Object.keys(mandatory).forEach(function (kind) { me[kind] = (me[kind] || 0) + mandatory[kind]; });
      Object.keys(optional).forEach(function (kind) { oe[kind] = (oe[kind] || 0) + optional[kind]; });
      lm = lower(lm, row, +row.dataset.ma, +row.dataset.mt);
      lo = lower(lo, row, +row.dataset.oa, +row.dataset.ot);
    });
    var ceilingText = "Mandatory ceiling: " + ratio(ma, mt) + exclusions(me, mt) +
      ". Optional ceiling: " + ratio(oa, ot) + exclusions(oe, ot) + ".";
    if (mt + ot && ma === mt && oa === ot) {
      ceilingText += " Every RFC shown with measured rows can reach 100% under current classifications.";
    }
    ceilingText += lowest("mandatory", lm) + lowest("optional", lo) + " ` + rfcCeilingPolicy + `";
    document.getElementById("rfc-overview-ceiling").textContent = ceilingText;
    document.getElementById("rfc-overview-title").textContent = profile ? "Profile " + profile : "The RFCs shown";
    figure("rfc-overview-mandatory", mp, mt);
    figure("rfc-overview-optional", op, ot);
    var scope = shown.length + " RFCs shown";
    if (unwalked) { scope += "; " + unwalked + " not walked or without a summary, so not counted"; }
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
    summarize(shown);
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

// rfcStemRows reads the corpus graph's figures, including RFCs no profile lists.
// Profile rows share those counts and supply them for profile-only ledgers.
func rfcStemRows(ledger rfcLedger) map[string]*rfc.TreeRow {
	rows := map[string]*rfc.TreeRow{}
	keep := func(list []*rfc.TreeRow) {
		for _, row := range list {
			if _, held := rows[row.Stem]; !held {
				rows[row.Stem] = row
			}
		}
	}
	keep(ledger.Graph.Nodes)
	for _, report := range ledger.Profiles {
		keep(report.Rows)
		if report.Optional != nil {
			keep(report.Optional.Rows)
		}
	}
	return rows
}

// rfcNoProfileCell marks a ledger with neither a graph nor a profile figure.
const rfcNoProfileCell = "no profile"

// rfcFigureCells answers one RFC's mandatory and optional cells, each a bar
// with its figure, as the profile pages render them.
func rfcFigureCells(row *rfc.TreeRow) (mandatory, optional string) {
	if row == nil {
		return rfcNoProfileCell, rfcNoProfileCell
	}
	mandatoryText, optionalText := row.Cells()
	mandatoryCeiling, optionalCeiling := rfcRowCeilingCells(row)
	return rfcProfileBarHTML(row, mandatoryText, row.MandatoryProven, row.MandatoryTotal) + mandatoryCeiling,
		rfcProfileBarHTML(row, optionalText, row.OptionalProven, row.OptionalTotal) + optionalCeiling
}

// rfcFigureMirrorCells answers the same two cells as text, for the mirror.
func rfcFigureMirrorCells(row *rfc.TreeRow) (mandatory, optional string) {
	if row == nil {
		return rfcNoProfileCell, rfcNoProfileCell
	}
	mandatory, optional = row.Cells()
	if !row.Walked {
		return mandatory, optional
	}
	return mandatory + "; ceiling " + rfcCeilingFigure(row.MandatoryAttainable, row.MandatoryTotal, row.MandatoryExcluded),
		optional + "; ceiling " + rfcCeilingFigure(row.OptionalAttainable, row.OptionalTotal, row.OptionalExcluded)
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
			`" data-ot="` + strconv.Itoa(row.OptionalTotal) +
			`" data-ma="` + strconv.Itoa(row.MandatoryAttainable) + `" data-oa="` + strconv.Itoa(row.OptionalAttainable) +
			`" data-me="` + html.EscapeString(rfcExcludedJSON(row.MandatoryExcluded)) +
			`" data-oe="` + html.EscapeString(rfcExcludedJSON(row.OptionalExcluded)) + `"`
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
			"\" means the ledger carries no figure for the RFC. The ceiling below each bar is the attainable share under current classifications"},
		{rfcIndexOptionalLabel, "the same proof and ceiling shares for its " + rfcKeywordSeries(rfc.OptionalKeywords()) + " requirements"},
		{rfcIndexMustsLabel, "the mandatory requirements the gate holds Ze to"},
		{rfcIndexGapsLabel, "requirements the summary declares Ze does not meet yet"},
		{rfcIndexGapsTested, "of those gaps, the ones a test demonstrates"},
		{rfcIndexUntestedLabel, "mandatory requirements with no test at all"},
	}
}

// rfcCeilingPolicy qualifies every computed ceiling, including filtered views.
const rfcCeilingPolicy = "Ceilings describe current classifications, not an immutable Linux maximum. " +
	"Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes."

// The closed annotation registry is immutable; retain its sorted names once
// rather than allocate and sort them for every table cell.
var rfcCeilingKinds = rfc.AnnotationKinds()

// rfcIndexCeilingRows selects the same unique RFCs that the index tables expose.
func rfcIndexCeilingRows(ledger rfcLedger) []*rfc.TreeRow {
	figures := rfcStemRows(ledger)
	rows := make([]*rfc.TreeRow, 0, len(ledger.Stems))
	for index := range ledger.Stems {
		if row := figures[ledger.Stems[index].Stem]; row != nil {
			rows = append(rows, row)
		}
	}
	return rows
}

// rfcCeilingStatement reads the model's walked-only sums and exact-ratio minima.
func rfcCeilingStatement(rows []*rfc.TreeRow) string {
	ceiling := rfc.CeilingOf(rows)
	var out textbuf.Buffer
	out.Str("Mandatory ceiling: ").Str(rfcCeilingFigure(ceiling.Mandatory.Attainable,
		ceiling.Mandatory.Total, ceiling.Mandatory.Excluded)).
		Str(". Optional ceiling: ").Str(rfcCeilingFigure(ceiling.Optional.Attainable,
		ceiling.Optional.Total, ceiling.Optional.Excluded)).Byte('.')
	if ceiling.Mandatory.Total+ceiling.Optional.Total > 0 {
		if ceiling.Mandatory.Attainable == ceiling.Mandatory.Total && ceiling.Optional.Attainable == ceiling.Optional.Total {
			out.Str(" Every RFC shown with measured rows can reach 100% under current classifications.")
		}
	}
	if ceiling.LowestMandatory != nil {
		out.Str(" Lowest mandatory ceiling: ").Str(ceiling.LowestMandatory.Stem).Byte(' ').
			Str(ceiling.LowestMandatory.String()).Byte('.')
	}
	if ceiling.LowestOptional != nil {
		out.Str(" Lowest optional ceiling: ").Str(ceiling.LowestOptional.Stem).Byte(' ').
			Str(ceiling.LowestOptional.String()).Byte('.')
	}
	return out.Str(" ").Str(rfcCeilingPolicy).String()
}

// rfcCeilingFigure preserves the denominator and names each excluded reason's share.
func rfcCeilingFigure(attainable, total int, excluded map[string]int) string {
	figure := rfc.Figure{Attainable: attainable, Total: total}
	var out textbuf.Buffer
	out.Str(figure.CeilingString())
	first := true
	for _, kind := range rfcCeilingKinds {
		count := excluded[kind]
		if count == 0 {
			continue
		}
		traits, known := rfc.AnnotationTraitsOf(kind)
		if !known {
			panic("BUG: registered annotation has no traits")
		}
		if first {
			out.Str("; excluded: ")
			first = false
		} else {
			out.Str("; ")
		}
		share := rfc.Figure{Proven: count, Total: total}
		out.Str(traits.CeilingLabel).Str(": ").Str(share.String())
	}
	return out.String()
}

// rfcRowCeilingCells adds no measurement to an unwalked RFC.
func rfcRowCeilingCells(row *rfc.TreeRow) (string, string) {
	if !row.Walked {
		return "", ""
	}
	return `<small class="rfc-ceiling">Ceiling: ` + html.EscapeString(rfcCeilingFigure(row.MandatoryAttainable, row.MandatoryTotal, row.MandatoryExcluded)) + `</small>`,
		`<small class="rfc-ceiling">Ceiling: ` + html.EscapeString(rfcCeilingFigure(row.OptionalAttainable, row.OptionalTotal, row.OptionalExcluded)) + `</small>`
}

// rfcExcludedJSON encodes only model count maps, which cannot fail JSON encoding.
func rfcExcludedJSON(excluded map[string]int) string {
	if len(excluded) == 0 {
		return "{}"
	}
	content, err := json.Marshal(excluded)
	if err != nil {
		panic("BUG: integer exclusion counts cannot encode as JSON")
	}
	return string(content)
}

// rfcCeilingReasons passes registered display labels to the browser, not a
// second classification policy or a manually maintained list of kinds.
func rfcCeilingReasons() string {
	reasons := map[string]string{}
	for _, kind := range rfcCeilingKinds {
		traits, known := rfc.AnnotationTraitsOf(kind)
		if !known {
			panic("BUG: registered annotation has no traits")
		}
		if traits.CeilingExcluded {
			reasons[kind] = traits.CeilingLabel
		}
	}
	content, err := json.Marshal(reasons)
	if err != nil {
		panic("BUG: string ceiling labels cannot encode as JSON")
	}
	return string(content)
}

// rfcIndexLegendHTML renders the legend under the enrolled table's heading.
func rfcIndexLegendHTML(rows []*rfc.TreeRow) string {
	var out textbuf.Buffer
	out.Str(`<dl class="rfc-legend">`).Byte('\n')
	for _, entry := range rfcIndexLegend() {
		out.Str("<dt>").Str(html.EscapeString(entry[0])).Str("</dt><dd>").
			Str(html.EscapeString(entry[1])).Str("</dd>\n")
	}
	out.Str("</dl>\n<p id=\"rfc-legend-ceiling\">").Str(html.EscapeString(rfcCeilingStatement(rows))).Str("</p>\n")
	return out.String()
}

// rfcIndexLegendMirror states the legend in the mirror.
func rfcIndexLegendMirror(rows []*rfc.TreeRow) string {
	var out textbuf.Buffer
	for _, entry := range rfcIndexLegend() {
		out.Str("- **").Str(entry[0]).Str("**: ").Str(entry[1]).Str(".\n")
	}
	out.Str("\n").Str(rfcCeilingStatement(rows)).Str("\n\n")
	return out.String()
}
