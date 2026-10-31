package browsertest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestTheBrowserSumsTheFilteredRFCListFromItsFiles proves the compliance
// index's overview draws both bars and a Total row over exactly the RFCs the
// filter shows, read from their data/rfc/ files (AC-10), whose five MUST
// columns add up to the MUST total; that every row carries its own two bars;
// and that a missing file is named rather than dropped from the sum (AC-18).
// The index headline sums only the enrolled RFCs, and the corpus count every
// RFC's requirement rows.
//
// Method: serve the golden index with every marked element seeded
// "fallback"; open it unfiltered, then with a profile link naming rfc1, rfc2
// and rfc3; then remove rfc2.json and open the link again. Each wait holds
// until no seeded fallback is left and the overview shows the expected RFC
// count, so every asserted element has its final value.
// PREVENTS: an overview that sums other RFCs than it shows, a Total row that
// does not add up, and a sum that silently loses an RFC.
func TestTheBrowserSumsTheFilteredRFCListFromItsFiles(t *testing.T) {
	dir, golden, base := rfcCompliancePagesSite(t)
	ctx := newRFCBrowser(t)
	listURL := base + strings.TrimSuffix(golden.List, "index.html")
	columns := []string{"must.ze-proven", "must.os-verified", "must.os-gap", "must.ze-gap", "must.not-owed", "must.total"}

	got := loadRFCProfilePage(ctx, t, listURL, func(state *rfcProfilePageState) bool {
		return rfcNoFallbackLeft(state) && state.Values["all:rfcs"] == "7"
	})
	for index, want := range []string{"19", "2", "2", "1,202", "10", "1,235"} {
		if got.Values["all:"+columns[index]] != want {
			t.Errorf("unfiltered, all:%s shows %q, want %q", columns[index], got.Values["all:"+columns[index]], want)
		}
	}
	if got.Values["all:not-walked"] != "2" {
		t.Errorf("unfiltered, all:not-walked shows %q, want 2", got.Values["all:not-walked"])
	}
	// The index headline sums the enrolled RFCs alone, rfc6 left out and
	// rfc7, enrolled but third-party, left out too, and the corpus count sums
	// every RFC's rows, the unwalked rfc3 and rfc7 included.
	// rfc4 is enrolled with no public row, so Ze implements four of the five.
	for marker, want := range map[string]string{"enrolled:rfcs": "5", "enrolled:not-walked": "1",
		"enrolled:must.total": "1,232", "enrolled:must.ze-proven": "18", "enrolled:implemented": "4",
		"all:requirements": "1,332"} {
		if got.Values[marker] != want {
			t.Errorf("unfiltered, %s shows %q, want %q", marker, got.Values[marker], want)
		}
	}
	rfcHeadlineIsNeverNarrowed(t, "unfiltered", &got)
	if want := []string{"ze-proven=Ze proven 4"}; !reflect.DeepEqual(got.Bars["rfc2:must"], want) {
		t.Errorf("rfc2's row draws %q, want %q", got.Bars["rfc2:must"], want)
	}
	if want := []string{"not walked"}; !reflect.DeepEqual(got.Bars["rfc3:overview"], want) {
		t.Errorf("rfc3's row draws %q, want %q", got.Bars["rfc3:overview"], want)
	}

	filtered := listURL + "?profile=edge&rfc=rfc1,rfc2,rfc3#rfc-list"
	got = loadRFCProfilePage(ctx, t, filtered, func(state *rfcProfilePageState) bool {
		return rfcNoFallbackLeft(state) && state.Values["all:rfcs"] == "3"
	})
	for index, want := range []string{"7", "2", "1", "1,200", "10", "1,220"} {
		if got.Values["all:"+columns[index]] != want {
			t.Errorf("filtered, all:%s shows %q, want %q", columns[index], got.Values["all:"+columns[index]], want)
		}
	}
	if want := []string{"ze-proven=Ze proven 7", "os-verified=OS verified 2", "os-gap=OS testing gap 1",
		"ze-gap=Ze correctness gap 1,200"}; !reflect.DeepEqual(got.Bars["all:must"], want) {
		t.Errorf("filtered, the overview MUST bar draws %q, want %q", got.Bars["all:must"], want)
	}
	rfcHeadlineIsNeverNarrowed(t, "filtered", &got)

	if err := os.Remove(filepath.Join(dir, "data", "rfc", "rfc2.json")); err != nil {
		t.Fatal(err)
	}
	// Without the fragment, so the browser loads the page again rather than
	// scrolling the one it shows.
	got = loadRFCProfilePage(ctx, t, strings.TrimSuffix(filtered, "#rfc-list"), func(state *rfcProfilePageState) bool {
		return strings.Contains(state.Values["all:rfcs"], "data/rfc/rfc2.json")
	})
	if text := got.Values["all:must.total"]; !strings.Contains(text, "data/rfc/rfc2.json") || !strings.Contains(text, "./le site build") {
		t.Errorf("with rfc2.json missing the MUST total shows %q, want the file and the command named", text)
	}
}

// rfcHeadlineIsNeverNarrowed checks the index headline, which sits outside the
// list the filter narrows: its corpus counts and its enrolled sums read the
// whole fixture whatever the filter shows.
func rfcHeadlineIsNeverNarrowed(t *testing.T, view string, got *rfcProfilePageState) {
	t.Helper()
	for marker, want := range map[string]string{"all:rfcs": "7", "all:requirements": "1,332",
		"enrolled:rfcs": "5", "enrolled:implemented": "4"} {
		if value := got.Scoped["rfc-enrolled-sections|"+marker]; value != want {
			t.Errorf("%s, the headline's %s shows %q, want %q", view, marker, value, want)
		}
	}
}
