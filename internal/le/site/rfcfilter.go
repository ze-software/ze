// Design: docs/contributing/gh-pages.md -- the compliance index filters its RFC tables
// Related: rfccompliance.go -- the tables the filter selects rows of
// Related: rfcprofiles.go -- the profile index links each RFC count here

package site

import (
	"net/url"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcListAnchor is the id of the enrolled-RFC section, where the filter sits.
const rfcListAnchor = "rfc-list"

// rfcFilterHTML is the filter above the enrolled-RFC table. It is hidden until
// the script runs, so a reader without scripts sees every row and no control
// that does nothing.
const rfcFilterHTML = `<div class="rfc-filter" id="rfc-filter" hidden>
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
const rfcFilterScript = `<style>
.rfc-filter { margin: .8rem 0 1rem; padding: .8rem 1rem; border-radius: 14px; background: var(--panel-strong); border: 1px solid var(--line); }
.rfc-filter label { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-filter input { width: 100%; max-width: 28rem; margin: .35rem 0; padding: .45rem .6rem; border-radius: 10px; border: 1px solid var(--line-strong); background: var(--panel); color: var(--text); font: inherit; }
.rfc-filter p { margin: .3rem 0 0; color: var(--muted); }
</style>
<script>
(function () {
  var box = document.getElementById("rfc-filter");
  var input = document.getElementById("rfc-filter-text");
  var note = document.getElementById("rfc-filter-note");
  if (!box || !input || !note) { return; }
  var rows = Array.prototype.slice.call(document.querySelectorAll("tr[data-rfc]"));
  var params = new URLSearchParams(window.location.search);
  var listed = (params.get("rfc") || "").split(",").map(function (s) { return s.trim().toLowerCase(); }).filter(Boolean);
  var wanted = listed.length ? new Set(listed) : null;
  var profile = params.get("profile") || "";
  if (!/^[a-z0-9][a-z0-9-]*$/.test(profile)) { profile = ""; }
  var held = new Set(rows.map(function (row) { return row.dataset.rfc; }));
  var absent = listed.filter(function (stem) { return !held.has(stem); });
  function apply() {
    var text = input.value.trim().toLowerCase();
    var shown = 0;
    rows.forEach(function (row) {
      var keep = (!wanted || wanted.has(row.dataset.rfc)) && (!text || row.textContent.toLowerCase().indexOf(text) >= 0);
      row.hidden = !keep;
      if (keep) { shown++; }
    });
    note.textContent = "";
    var lead = wanted ? "Showing " + shown + " of the " + listed.length + " RFCs the link lists" : "Showing " + shown + " of " + rows.length + " RFCs";
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
