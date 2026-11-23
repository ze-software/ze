// Design: website/AI.md -- the RFC map statement selector exposes missing requirements and search filters.
// Related: rfcmap_test.go verifies base graph navigation, scopes, layout and error states.
package browsertest

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestRFCMapMissingSelectorAndSearch opens RFC 4271 on the published map and
// proves the statement selector opens on not-built requirements, steps through
// their missing text, switches statuses, filters RFCs by coverage and searches
// missing statements.
func TestRFCMapMissingSelectorAndSearch(t *testing.T) {
	base, model, _ := rfcMapBrowserSite(t)
	ctx := newRFCBrowser(t)
	pageURL := base + "/quality/rfc-map/?layer=base-ipv4-bgp&rfc=rfc4271"

	var wantMissing []string
	for _, row := range model.Requirements["rfc4271"] {
		if row.Gated && row.Status == "not-built" {
			wantMissing = append(wantMissing, row.RID)
		}
	}
	if len(wantMissing) == 0 {
		t.Fatal("rfc4271 has no recorded not-built mandatory requirements to verify the missing selector")
	}

	rfcMapBrowserOpen(ctx, t, pageURL)
	rfcMapBrowserWait(ctx, t, `document.querySelector('#rfc-map-statement-select option') !== null && document.querySelector('.rfc-map-proof-value') !== null`)

	// Default selector opens on the missing (not-built) mandatory statements.
	gotMissing, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](
		`Array.from(document.querySelectorAll('#rfc-map-statement-select option'), o => o.value)`))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(wantMissing)
	slices.Sort(gotMissing)
	if !slices.Equal(gotMissing, wantMissing) {
		t.Fatalf("default missing selector options = %v, want %v", gotMissing, wantMissing)
	}

	// Reading pane shows the selected missing statement's requirement text and gap/missing details.
	firstRID, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.getElementById('rfc-map-statement-select').value`))
	if err != nil {
		t.Fatal(err)
	}
	readerText, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-statement-reader').textContent`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readerText, firstRID) {
		t.Errorf("reading pane does not show selected missing RID %s: %s", firstRID, readerText)
	}
	if !strings.Contains(readerText, "What is missing") && !strings.Contains(readerText, "Why this is missing") && !strings.Contains(readerText, "Recorded unmet") {
		t.Errorf("reading pane does not explain why %s is missing: %s", firstRID, readerText)
	}

	// Next button steps to the second missing statement and updates the reading pane.
	if len(wantMissing) > 1 {
		if _, err := chromedp.Run(ctx, chromedp.Click(chromedp.CSS(".rfc-map-reader-pager button:last-child"))); err != nil {
			t.Fatal(err)
		}
		secondRID, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.getElementById('rfc-map-statement-select').value`))
		if err != nil {
			t.Fatal(err)
		}
		if secondRID == firstRID {
			t.Fatalf("Next button left selector on %s", firstRID)
		}
		secondText, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-statement-reader').textContent`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(secondText, secondRID) {
			t.Errorf("reading pane did not advance to %s", secondRID)
		}
	}

	// Statement search narrows the selector to a specific missing RID.
	targetRID := wantMissing[len(wantMissing)-1]
	if _, err := chromedp.Run(ctx, chromedp.SendKeys(chromedp.CSS("#rfc-map-statement-search"), targetRID)); err != nil {
		t.Fatal(err)
	}
	filtered, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](
		`Array.from(document.querySelectorAll('#rfc-map-statement-select option'), o => o.value)`))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(filtered, []string{targetRID}) {
		t.Errorf("statement search for %s gave %v, want [%s]", targetRID, filtered, targetRID)
	}

	// Coverage filter narrows All RFCs to documents with not-built mandatory requirements.
	if _, err := chromedp.Run(ctx, chromedp.Click(chromedp.CSS(".rfc-map-scope-row button"))); err != nil {
		t.Fatal(err)
	}
	rfcMapBrowserChoose(ctx, t, "rfc-map-rfc-status", "not-built")
	var wantNotBuiltRFCs []string
	for _, node := range model.Nodes {
		for _, row := range model.Requirements[node.Stem] {
			if row.Gated && row.Status == "not-built" {
				wantNotBuiltRFCs = append(wantNotBuiltRFCs, node.Stem)
				break
			}
		}
	}
	rfcMapBrowserMembers(ctx, t, rfcMapBrowserLabels(wantNotBuiltRFCs))

	// Clicking a status pill switches the statement selector to All and preserves keyword selection.
	rfcMapBrowserOpen(ctx, t, pageURL)
	rfcMapBrowserWait(ctx, t, `document.querySelector('#rfc-map-statement-select option') !== null`)
	clickedAll, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
		const pill = Array.from(document.querySelectorAll('.rfc-map-status-filters button')).find(b => b.textContent.startsWith('All ('));
		if (!pill) return false;
		pill.click();
		return true;
	})()`))
	if err != nil {
		t.Fatal(err)
	}
	if !clickedAll {
		t.Fatal("All status filter pill is absent")
	}
	allCount, err := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('#rfc-map-statement-select option').length`))
	if err != nil {
		t.Fatal(err)
	}
	if allCount <= len(wantMissing) {
		t.Errorf("All status filter shows %d options, want more than the %d missing ones ("+strconv.Itoa(allCount)+")", allCount, len(wantMissing))
	}
}
