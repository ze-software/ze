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
		return rfcNoFallbackLeft(state) && state.Values["all:rfcs"] == "6"
	})
	for index, want := range []string{"19", "2", "2", "1,202", "10", "1,235"} {
		if got.Values["all:"+columns[index]] != want {
			t.Errorf("unfiltered, all:%s shows %q, want %q", columns[index], got.Values["all:"+columns[index]], want)
		}
	}
	if got.Values["all:not-walked"] != "1" {
		t.Errorf("unfiltered, all:not-walked shows %q, want 1", got.Values["all:not-walked"])
	}
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
