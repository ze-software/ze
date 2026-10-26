package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcProfilesFixtureLedger states a ledger with three profiles, one of each
// group, whose rows cover every state a node can be in: walked and measured,
// not walked, no summary, no level, and absent.
func rfcProfilesFixtureLedger() rfcLedger {
	zero, one := 0, 1
	walked := &rfc.TreeRow{Stem: "rfc4271", Title: "A Border Gateway Protocol 4 (BGP-4)", Level: &zero,
		Aggregate: "bgp-core", Summary: true, Walked: true, MandatoryProven: 3, MandatoryTotal: 4,
		OptionalProven: 1, OptionalTotal: 2, Component: []string{}, Path: "route-server: rfc4271 (root)"}
	unwalked := &rfc.TreeRow{Stem: "rfc7947", Title: "Internet Exchange BGP Route Server", Level: &one,
		Aggregate: "bgp-core", Summary: true, MandatoryTotal: 9, Component: []string{"rfc7948"},
		Path: "route-server: rfc7947 (root)"}
	textOnly := &rfc.TreeRow{Stem: "rfc1997", Level: &one, Component: []string{},
		Path: "route-server: rfc7947 builds-on rfc1997"}
	noLevel := &rfc.TreeRow{Stem: "rfc4762", Title: "VPLS", Summary: true, Walked: true,
		MandatoryProven: 0, MandatoryTotal: 0, Component: []string{}, Path: "route-server: rfc4762 (root)"}
	absent := &rfc.TreeRow{Stem: "rfc8212", Absent: true, Component: []string{},
		Path: "route-server: listed under Absent"}
	release := &rfc.TreeReport{Profile: "route-server", Title: "Route server", Kind: "use-case", Release: true,
		Rollup: &rfc.ScopeRollup{Rollup: rfc.Rollup{RFCs: 5, NotWalked: 2, Absent: 1,
			Mandatory: rfc.Figure{Proven: 3, Total: 4}, Optional: rfc.Figure{Proven: 1, Total: 2}}},
		Rows: []*rfc.TreeRow{walked, unwalked, textOnly, noLevel, absent}}
	other := &rfc.TreeReport{Profile: "vpn-concentrator", Title: "VPN concentrator", Kind: "use-case",
		Rollup: &rfc.ScopeRollup{Rollup: rfc.Rollup{RFCs: 1, NotWalked: 1}},
		Rows:   []*rfc.TreeRow{unwalked}}
	aggregate := &rfc.TreeReport{Profile: "bgp-core", Title: "BGP core", Heading: "BGP core protocol",
		Kind: "technology", Rollup: &rfc.ScopeRollup{Rollup: rfc.Rollup{RFCs: 2, NotWalked: 1,
			Mandatory: rfc.Figure{Proven: 3, Total: 4}, Optional: rfc.Figure{Proven: 1, Total: 2}}},
		Rows: []*rfc.TreeRow{walked, unwalked}}
	return rfcLedger{
		Stems: []rfcLedgerStem{
			{Stem: "rfc4271", Display: "RFC 4271", Enrolled: true},
			{Stem: "rfc7947", Display: "RFC 7947", Enrolled: true},
			{Stem: "rfc4762", Display: "RFC 4762", Enrolled: true},
		},
		// The ledger lists use cases by name, then aggregates, as
		// rfc.PriorityOf answers them; the index reorders release first.
		Profiles: []*rfc.TreeReport{release, other, aggregate},
	}
}

// renderRFCProfilesFixture publishes the stated ledger into a fresh artifact
// and runs the producer over it.
func renderRFCProfilesFixture(t *testing.T, ledger rfcLedger) (string, []string) {
	t.Helper()
	output := t.TempDir()
	content, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeNamedArtifact(output, rfcLedgerFile, string(content)); err != nil {
		t.Fatal(err)
	}
	// A page this family wrote for a profile that no longer exists, and a
	// directory somebody else put under the same prefix.
	retired := filepath.Join(output, filepath.FromSlash(rfcProfilesDirectory), "withdrawn")
	foreign := filepath.Join(output, filepath.FromSlash(rfcProfilesDirectory), "authored")
	for directory, page := range map[string]string{retired: "<h1" + rfcProfileMarker + ">gone</h1>", foreign: "<h1>mine</h1>"} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, pageIndexFile), []byte(page), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	routes, err := renderRFCProfiles(Paths{Repository: t.TempDir(), Source: t.TempDir(), Output: output})
	if err != nil {
		t.Fatalf("renderRFCProfiles: %v", err)
	}
	return output, routes
}

func readRFCProfilePage(t *testing.T, output, profile, file string) string {
	t.Helper()
	path := filepath.Join(output, filepath.FromSlash(rfcProfilesDirectory), profile, file)
	content, err := os.ReadFile(path) //nolint:gosec // the test's own temporary artifact
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// VALIDATES: AC-15 at the producer.
// PREVENTS: a profile with no page, an index that buries the release
// profiles, and an unwalked or absent RFC read as a measurement.
//
// TestRFCProfilePagesRendered proves AC-15 at the producer: the index and one
// page per profile are written and answered, release profiles lead the index,
// each page carries both figures, the not-walked and absent counts and a bar
// per RFC, an absent RFC renders at 0%, an unwalked one as "not walked", and
// a retired page of this family is removed while a foreign directory stays.
func TestRFCProfilePagesRendered(t *testing.T) {
	ledger := rfcProfilesFixtureLedger()
	output, routes := renderRFCProfilesFixture(t, ledger)

	want := []string{rfcProfilesRoute, rfcProfilesRoute + "route-server/",
		rfcProfilesRoute + "vpn-concentrator/", rfcProfilesRoute + "bgp-core/"}
	if !slices.Equal(routes, want) {
		t.Fatalf("routes = %v, want %v (every profile gets a page, release profiles first)", routes, want)
	}

	index := readRFCProfilePage(t, output, "", pageIndexFile)
	release := strings.Index(index, `href="route-server/"`)
	other := strings.Index(index, `href="vpn-concentrator/"`)
	aggregate := strings.Index(index, `href="bgp-core/"`)
	if release < 0 || other < release || aggregate < other {
		t.Fatalf("index order: route-server at %d, vpn-concentrator at %d, bgp-core at %d; want release first", release, other, aggregate)
	}
	for _, heading := range []string{"Release profiles", "Other use-case profiles", "Technology aggregates"} {
		if !strings.Contains(index, "<h2>"+heading+"</h2>") {
			t.Errorf("index carries no %q heading", heading)
		}
	}
	if mirror := readRFCProfilePage(t, output, "", "index.md"); !strings.Contains(mirror, "[Route server](route-server/)") {
		t.Errorf("index mirror does not link the release profile:\n%s", mirror)
	}

	page := readRFCProfilePage(t, output, "route-server", pageIndexFile)
	for _, fact := range []string{
		rfcProfileMarker, "Release profile",
		"<strong>75.0%</strong><b>3 of 4 (75.0%)</b>", // mandatory figure
		"<strong>50.0%</strong><b>1 of 2 (50.0%)</b>", // optional figure
		"<span>Not walked</span><strong>2</strong>",
		"<span>Absent</span><strong>1</strong>",
		"<h3>Level 0</h3>", "<h3>Level 1</h3>", "<h3>Absent from the corpus</h3>",
		"<h3>No level: the corpus holds no text of the RFC</h3>",
		`<a href="../../rfc4271/">RFC 4271</a>`, "(cycle with rfc7948)",
		`<span style="width:75.0%">`, "0% (absent from the corpus)", "not walked", "no summary",
		"route-server: rfc7947 builds-on rfc1997",
	} {
		if !strings.Contains(page, fact) {
			t.Errorf("route-server page lacks %q", fact)
		}
	}
	if bars := strings.Count(page, `<div class="rfc-bar`); bars != 2*len(ledger.Profiles[0].Rows) {
		t.Errorf("route-server page carries %d bars, want two per RFC (%d)", bars, 2*len(ledger.Profiles[0].Rows))
	}
	if strings.Contains(page, `<a href="../../rfc1997/">`) {
		t.Error("a node with no summary links a ledger page that does not exist")
	}
	mirror := readRFCProfilePage(t, output, "route-server", "index.md")
	for _, fact := range []string{"# Route server", "**Mandatory**: 75.0% (3 of 4 (75.0%))", "### Level 0",
		"| [RFC 4271](../../rfc4271/) |", "| 0% (absent from the corpus) |", "| not walked |"} {
		if !strings.Contains(mirror, fact) {
			t.Errorf("route-server mirror lacks %q", fact)
		}
	}

	aggregatePage := readRFCProfilePage(t, output, "bgp-core", pageIndexFile)
	if !strings.Contains(aggregatePage, "Technology aggregate") || strings.Contains(aggregatePage, "Why it is here") {
		t.Error("an aggregate page is labelled as one and carries no path column")
	}

	if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(rfcProfilesDirectory), "withdrawn")); !os.IsNotExist(err) {
		t.Errorf("the retired profile page survived the build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(rfcProfilesDirectory), "authored", pageIndexFile)); err != nil {
		t.Errorf("a directory this family did not write was removed: %v", err)
	}
}

// VALIDATES: the refusal of an empty profile set.
// PREVENTS: an index that links nothing passing every artifact check.
//
// TestRFCProfilesRefuseALedgerWithNoProfile proves the producer refuses to
// publish an index that links nothing.
func TestRFCProfilesRefuseALedgerWithNoProfile(t *testing.T) {
	ledger := rfcProfilesFixtureLedger()
	ledger.Profiles = nil
	output := t.TempDir()
	content, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeNamedArtifact(output, rfcLedgerFile, string(content)); err != nil {
		t.Fatal(err)
	}
	_, err = renderRFCProfiles(Paths{Repository: t.TempDir(), Source: t.TempDir(), Output: output})
	if err == nil || !strings.Contains(err.Error(), "names no RFC profile") {
		t.Fatalf("err = %v, want the refusal naming the missing profiles", err)
	}
}
