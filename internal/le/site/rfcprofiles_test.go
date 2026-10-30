package site

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcProfilesFixtureLedger states a ledger with three profiles, one of each
// group, whose rows cover every state a member can be in: walked and
// measured, not walked, and no level; the release profile also lists one
// optional RFC in its own tier.
func rfcProfilesFixtureLedger() rfcLedger {
	zero, one := 0, 1
	walked := &rfc.TreeRow{Stem: "rfc4271", Title: "A Border Gateway Protocol 4 (BGP-4)", Level: &zero,
		Aggregate: "bgp-core", Summary: true, Walked: true, MandatoryProven: 3, MandatoryTotal: 4, MandatoryAttainable: 4,
		OptionalProven: 1, OptionalTotal: 2, OptionalAttainable: 2, Component: []string{}, Path: "route-server: required by rfc/short/rfc4271.md"}
	unwalked := &rfc.TreeRow{Stem: "rfc7947", Title: "Internet Exchange BGP Route Server", Level: &one,
		Aggregate: "bgp-core", Summary: true, MandatoryTotal: 9, MandatoryAttainable: 9, Component: []string{"rfc7948"},
		Path: "route-server: required by rfc/short/rfc7947.md"}
	noLevel := &rfc.TreeRow{Stem: "rfc4762", Title: "VPLS", Summary: true, Walked: true,
		MandatoryProven: 0, MandatoryTotal: 0, Component: []string{}, Path: "route-server: required by rfc/short/rfc4762.md"}
	listed := &rfc.TreeRow{Stem: "rfc2918", Title: "Route Refresh Capability for BGP-4", Level: &zero,
		Summary: true, Walked: true, MandatoryProven: 1, MandatoryTotal: 2, MandatoryAttainable: 2, Component: []string{},
		Path: "route-server: optional in rfc/short/rfc2918.md"}
	release := &rfc.TreeReport{Profile: "route-server", Title: "Route server", Kind: "use-case", Release: true,
		Rollup: &rfc.ScopeRollup{RFCs: 3, NotWalked: 1,
			Mandatory: rfc.Figure{Proven: 3, Total: 4, Attainable: 4}, Optional: rfc.Figure{Proven: 1, Total: 2, Attainable: 2}},
		Rows: []*rfc.TreeRow{walked, unwalked, noLevel},
		Optional: &rfc.TreeTier{Rollup: rfc.ScopeRollup{RFCs: 1,
			Mandatory: rfc.Figure{Proven: 1, Total: 2, Attainable: 2}}, Rows: []*rfc.TreeRow{listed}}}
	other := &rfc.TreeReport{Profile: "vpn-concentrator", Title: "VPN concentrator", Kind: "use-case",
		Rollup: &rfc.ScopeRollup{RFCs: 1, NotWalked: 1},
		Rows:   []*rfc.TreeRow{unwalked}}
	aggregate := &rfc.TreeReport{Profile: "bgp-core", Title: "BGP core", Heading: "BGP core protocol",
		Kind: "technology", Rollup: &rfc.ScopeRollup{RFCs: 2, NotWalked: 1,
			Mandatory: rfc.Figure{Proven: 3, Total: 4, Attainable: 4}, Optional: rfc.Figure{Proven: 1, Total: 2, Attainable: 2}},
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
// profiles, an unwalked RFC read as a measurement, and an optional RFC
// counted in the profile's figures.
//
// TestRFCProfilePagesRendered proves AC-15 at the producer: the index and one
// page per profile are written and answered, release profiles lead the index,
// each page carries both figures, the not-walked count and a bar per RFC, an
// unwalked RFC renders as "not walked", the optional tier sits in its own
// "Optional (not counted)" table with its own rollup (AC-10), no page or
// index carries an absent count, and a retired page of this family is
// removed while a foreign directory stays.
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
	// A profile with an unwalked member must not read complete.
	if !strings.Contains(index, "Mandatory register") || !strings.Contains(index, "not complete") {
		t.Errorf("index carries no mandatory register column")
	}
	if mirror := readRFCProfilePage(t, output, "", "index.md"); !strings.Contains(mirror, "| not complete |") {
		t.Errorf("index mirror carries no mandatory register cell:\n%s", mirror)
	}

	page := readRFCProfilePage(t, output, "route-server", pageIndexFile)
	for _, fact := range []string{
		rfcProfileMarker, "Release profile",
		"<strong>75.0%</strong><b>3 of 4 (75.0%)</b>", // mandatory figure
		"<strong>50.0%</strong><b>1 of 2 (50.0%)</b>", // optional figure
		"<span>Not walked</span><strong>1</strong>",
		"<h3>Level 0</h3>", "<h3>Level 1</h3>",
		"<h3>No level: the corpus holds no text of the RFC</h3>",
		`<a href="../../rfc4271/">RFC 4271</a>`, "(cycle with rfc7948)",
		`<span style="width:75.0%">`, "not walked", "route-server: required by rfc/short/rfc7947.md",
		"<h2>Optional (not counted)</h2>", "Their own figures: mandatory 1 of 2 (50.0%)",
		"route-server: optional in rfc/short/rfc2918.md", `<span style="width:50.0%">`,
	} {
		if !strings.Contains(page, fact) {
			t.Errorf("route-server page lacks %q", fact)
		}
	}
	if headings := strings.Count(page, "<h3>Level 1</h3>"); headings != 1 {
		t.Errorf("route-server page carries %d Level 1 headings, want 1", headings)
	}
	if strings.Contains(page, "Absent") || strings.Contains(index, "Absent") {
		t.Error("a profile page or the index still carries an absent count: every listed RFC has a summary")
	}
	for _, keyword := range append(rfc.MandatoryKeywords(), rfc.OptionalKeywords()...) {
		if !strings.Contains(page, keyword+",") && !strings.Contains(page, keyword+" and") && !strings.Contains(page, keyword+" rows") {
			t.Errorf("route-server page cards do not name keyword %q", keyword)
		}
	}
	members := len(ledger.Profiles[0].Rows) + len(ledger.Profiles[0].Optional.Rows)
	if bars := strings.Count(page, `<div class="rfc-bar`); bars != 2*members {
		t.Errorf("route-server page carries %d bars, want two per RFC of either tier (%d)", bars, 2*members)
	}
	mirror := readRFCProfilePage(t, output, "route-server", "index.md")
	for _, fact := range []string{"# Route server", "**Mandatory**: 75.0% (3 of 4 (75.0%))", "### Level 0",
		"| [RFC 4271](../../rfc4271/) |", "| not walked |", "## Optional (not counted)",
		"| route-server: optional in rfc/short/rfc2918.md |"} {
		if !strings.Contains(mirror, fact) {
			t.Errorf("route-server mirror lacks %q", fact)
		}
	}

	aggregatePage := readRFCProfilePage(t, output, "bgp-core", pageIndexFile)
	if !strings.Contains(aggregatePage, "Technology aggregate") || strings.Contains(aggregatePage, "Why it is here") {
		t.Error("an aggregate page is labeled as one and carries no path column")
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

// writeRFCProfileUseCase writes one use-case page under source, carrying the
// stated body.
func writeRFCProfileUseCase(t *testing.T, source, name, body string) {
	t.Helper()
	directory := filepath.Join(source, "use-cases", name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// renderRFCProfilesWithUseCases publishes the fixture ledger with the stated
// source tree and answers the output and the producer's error.
func renderRFCProfilesWithUseCases(t *testing.T, source string) (string, error) {
	t.Helper()
	output := t.TempDir()
	content, err := json.Marshal(rfcProfilesFixtureLedger())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeNamedArtifact(output, rfcLedgerFile, string(content)); err != nil {
		t.Fatal(err)
	}
	_, err = renderRFCProfiles(Paths{Repository: t.TempDir(), Source: source, Output: output})
	return output, err
}

// VALIDATES: a profile page links back to every use-case page that links to it.
// PREVENTS: a reader who arrives on a profile's coverage page from the menu
// having no way to the use case it measures, and the two directions drifting.
//
// TestRFCProfilePageLinksBackToItsUseCase writes one use-case page linking the
// route-server profile and one linking nothing, renders, and reads both forms
// of each profile page: only route-server names the use case.
func TestRFCProfilePageLinksBackToItsUseCase(t *testing.T) {
	source := t.TempDir()
	writeRFCProfileUseCase(t, source, "ixp", "# Route Server at an IXP\n\nSee the "+
		"[coverage](../../quality/rfc-compliance/profiles/route-server/).\n")
	writeRFCProfileUseCase(t, source, "other", "# Unrelated\n\nNo profile link.\n")
	output, err := renderRFCProfilesWithUseCases(t, source)
	if err != nil {
		t.Fatalf("renderRFCProfiles: %v", err)
	}
	page := readRFCProfilePage(t, output, "route-server", pageIndexFile)
	if !strings.Contains(page, `<p>Use case: <a href="../../../../use-cases/ixp/">Route Server at an IXP</a></p>`) {
		t.Error("the route-server page does not link back to the use case that links to it")
	}
	mirror := readRFCProfilePage(t, output, "route-server", "index.md")
	if !strings.Contains(mirror, "Use case: [Route Server at an IXP](../../../../use-cases/ixp/)") {
		t.Error("the route-server mirror does not link back to its use case")
	}
	if strings.Contains(readRFCProfilePage(t, output, "bgp-core", pageIndexFile), "Use case:") {
		t.Error("bgp-core names a use case although no use-case page links to it")
	}
}

// VALIDATES: a use-case link to a profile that does not exist is refused.
// PREVENTS: a use-case page publishing a link to a coverage page nobody builds.
func TestRFCProfilesRefuseAUseCaseLinkingAnUnknownProfile(t *testing.T) {
	source := t.TempDir()
	writeRFCProfileUseCase(t, source, "ghost", "# Ghost\n\n"+
		"[coverage](../../quality/rfc-compliance/profiles/no-such-profile/)\n")
	_, err := renderRFCProfilesWithUseCases(t, source)
	if err == nil || !strings.Contains(err.Error(), `"no-such-profile"`) {
		t.Fatalf("err = %v, want the refusal naming the unknown profile", err)
	}
}

// TestRFCProfileCountLinksToItsRFCs proves each profile's RFC count on the
// index links to the compliance index filtered to exactly the RFCs it counts,
// in the page and in its mirror.
// Method: render the fixture and look for the route-server link naming its
// three required stems and not its optional one.
func TestRFCProfileCountLinksToItsRFCs(t *testing.T) {
	output, _ := renderRFCProfilesFixture(t, rfcProfilesFixtureLedger())
	const href = "../?profile=route-server&rfc=rfc4271%2Crfc7947%2Crfc4762#" + rfcListAnchor
	index := readRFCProfilePage(t, output, "", pageIndexFile)
	if want := `<a href="` + html.EscapeString(href) + `">3</a>`; !strings.Contains(index, want) {
		t.Errorf("index does not link the route-server RFC count as %s", want)
	}
	if strings.Contains(index, "rfc2918") {
		t.Error("the RFC count link names the optional rfc2918, which the count does not count")
	}
	if mirror := readRFCProfilePage(t, output, "", "index.md"); !strings.Contains(mirror, "[3]("+href+")") {
		t.Errorf("index mirror does not link the route-server RFC count:\n%s", mirror)
	}
}

// TestRFCProfileAggregateOptionalTier: an aggregate's optional tier, the
// members whose `Aggregate tier` is optional, renders in its own table with
// the aggregate's four columns and prose naming the row that declares it.
// Method: render the page body and its mirror for an aggregate carrying one
// optional member, and count the cells of that member's row.
// PREVENTS: an optional table one cell wider than its head, and prose that
// sends an aggregate's reader to the use-case row.
func TestRFCProfileAggregateOptionalTier(t *testing.T) {
	zero := 0
	listed := &rfc.TreeRow{Stem: "rfc4577", Title: "OSPF as the PE/CE Protocol", Level: &zero,
		Aggregate: "ospf", Summary: true, Walked: true, MandatoryTotal: 2, MandatoryAttainable: 2,
		Component: []string{}, Path: "ospf: optional in rfc/short/rfc4577.md"}
	report := &rfc.TreeReport{Profile: "ospf", Title: "OSPF", Heading: "OSPF", Kind: "technology",
		Rollup: &rfc.ScopeRollup{}, Rows: []*rfc.TreeRow{},
		Optional: &rfc.TreeTier{Rollup: rfc.ScopeRollup{RFCs: 1, Mandatory: rfc.Figure{Total: 2, Attainable: 2}},
			Rows: []*rfc.TreeRow{listed}}}
	display := map[string]string{"rfc4577": "RFC 4577"}
	body := rfcProfileBody(report, display, nil)
	mirror := rfcProfileMirror(report, display, nil)
	for name, text := range map[string]string{"body": body, "mirror": mirror} {
		if !strings.Contains(text, rfcProfileOptionalHeading) {
			t.Errorf("%s lacks the optional heading:\n%s", name, text)
		}
		if !strings.Contains(text, "Aggregate tier row") || strings.Contains(text, "Optional use cases row") {
			t.Errorf("%s does not send the reader to the Aggregate tier row:\n%s", name, text)
		}
		if strings.Contains(text, listed.Path) {
			t.Errorf("%s renders a path cell the aggregate's head has no column for:\n%s", name, text)
		}
	}
	for line := range strings.SplitSeq(mirror, "\n") {
		if !strings.Contains(line, "RFC 4577") {
			continue
		}
		if cells := strings.Count(line, "|") - 1; cells != len(rfcProfileColumns(report)) {
			t.Errorf("mirror row %q has %d cells, the head %d", line, cells, len(rfcProfileColumns(report)))
		}
	}
}
