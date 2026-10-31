package site

import (
	"bytes"
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcLedgerFixtureData answers the data/rfc/ files a build would publish
// beside a test ledger: one per stem, walked when the stem's model row is,
// its MUST rows split into Ze proven and Ze gap and its SHOULD rows into
// proven and not decided, and one per profile from the profile's tree rows.
// The compliance producer reads these files back, so a fixture that
// published only the ledger would publish no figure at all.
func rfcLedgerFixtureData(ledger rfcLedger) rfcData {
	data := rfcData{index: rfcDataIndex{Bars: rfcBars}}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		row := entry.Figures
		// The fixture's summary declares its coverage's requirement rows, and
		// an enrolled entry with a public area is one rfc.Implements answers.
		stem := rfcStemData{Stem: entry.Stem, Title: entry.Title, Unmet: []string{},
			Requirements: entry.Coverage.Requirements, Implementer: "ze",
			Implemented: entry.Enrolled && entry.PublicArea != ""}
		if row != nil && row.Walked {
			stem.Walked = true
			stem.ProvenByKind = map[string]int{}
			stem.Must = &rfc.MustSections{Total: row.Mandatory.Total, Kept: row.Mandatory.Total,
				ZeProven: row.Mandatory.Proven, ZeGap: row.Mandatory.Total - row.Mandatory.Proven,
				NotOwedByKind: map[string]int{}}
			stem.Should = &rfc.ShouldSections{Total: row.Optional.Total, Proven: row.Optional.Proven,
				NotDecided: row.Optional.Total - row.Optional.Proven, NotOwedByKind: map[string]int{}}
		}
		enrolment := "enrolled"
		if entry.Disposition != nil {
			enrolment = entry.Disposition.Kind
		}
		data.stems = append(data.stems, stem)
		data.index.RFCs = append(data.index.RFCs, rfcIndexRFC{Stem: entry.Stem, File: rfcStemDataFile(entry.Stem),
			Title: entry.Title, Walked: stem.Walked, Enrolment: enrolment, Gated: entry.Enrolled})
	}
	for _, report := range ledger.Profiles {
		profile := rfcProfileData{Name: report.Profile, Kind: report.Kind, Title: report.Title,
			Required: []string{}, Optional: []string{}}
		for _, row := range report.Rows {
			profile.Required = append(profile.Required, row.Stem)
		}
		if report.Optional != nil {
			for _, row := range report.Optional.Rows {
				profile.Optional = append(profile.Optional, row.Stem)
			}
		}
		data.profiles = append(data.profiles, profile)
		data.index.Profiles = append(data.index.Profiles, rfcIndexProfile{Name: profile.Name,
			File: rfcProfileDataFile(profile.Name), Kind: profile.Kind, Title: profile.Title})
	}
	return data
}

// rfcDetailFiguresLedger publishes the profile fixture's seven RFCs, rfc6 not
// enrolled and rfc7 enrolled but third-party, with the profile fixture's data: rfc1 has every MUST bar segment
// non-zero and rfc3 is not walked.
func rfcDetailFiguresLedger(t *testing.T) (Paths, rfcLedger) {
	t.Helper()
	ledger := rfcProfileFiguresLedger()
	ledger.Stems = append(ledger.Stems, rfcLedgerStem{Stem: "rfc5", Display: "RFC 5", Enrolled: true},
		rfcLedgerStem{Stem: "rfc6", Display: "RFC 6",
			Disposition: &rfcLedgerDisposition{Kind: "backlog", Reason: "the extraction is owed"}},
		// The enrolment row says enrolled and the summary is third-party, so,
		// as rfc.Meta.Disposition answers, the kind that took it off the gate
		// is its Implementation.
		rfcLedgerStem{Stem: "rfc7", Display: "RFC 7", Implementation: "third-party",
			Disposition: &rfcLedgerDisposition{Kind: "third-party", Reason: "Linux XFRM performs it"}})
	for index := range ledger.Stems {
		ledger.Stems[index].Title = "Title " + ledger.Stems[index].Stem
		ledger.Stems[index].SummaryPath = "rfc/short/" + ledger.Stems[index].Stem + ".md"
	}
	paths := rfcCompliancePathsWith(t, &rfcCompliance{}, ledger)
	// The fixture's own data, with counts no ledger figure could derive.
	data := rfcProfileFiguresData()
	if err := os.RemoveAll(filepath.Join(paths.Output, rfcDataDir)); err != nil {
		t.Fatal(err)
	}
	if err := writeRFCData(paths.Output, &data); err != nil {
		t.Fatal(err)
	}
	return paths, ledger
}

// TestRFCDetailPageShowsItsFiguresFromItsFile renders the detail pages
// through the producer and reads rfc1's page and mirror: both bars and every
// figure of the table are marked with rfc1 as their subject and carry the
// value data/rfc/rfc1.json states, the page loads rfc-figures.js, the mirror
// links the file, and no ceiling is left. An unwalked RFC shows "not walked".
func TestRFCDetailPageShowsItsFiguresFromItsFile(t *testing.T) {
	paths, _ := rfcDetailFiguresLedger(t)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	page := readRFCProfileFile(t, paths.Output, rfcComplianceDirectory+"/rfc1/index.html")
	mirror := readRFCProfileFile(t, paths.Output, rfcComplianceDirectory+"/rfc1/index.md")
	want := map[string]string{
		"must.ze-proven": "3", "must.os-verified": "2", "must.os-gap": "1", "must.ze-gap": "1,200",
		"must.kept": "1,206", "must.total": "1,216", "must.not-owed": "10",
		"must.not-owed-by-kind." + rfc.AnnotationNotApplicable: "4", "should.total": "10", "should.not-decided": "2",
	}
	for field, value := range want {
		element := `<span data-ze-rfc="rfc1:` + field + `">` + value + `</span>`
		if !strings.Contains(page, element) {
			t.Errorf("the page carries no %s", element)
		}
	}
	mustBar := regexp.MustCompile(`<div data-ze-rfc-bar="rfc1:must">([^<]*)</div>`).FindStringSubmatch(page)
	if len(mustBar) != 2 || mustBar[1] != "Ze proven 3, OS verified 2, OS testing gap 1, Ze correctness gap 1,200" {
		t.Errorf("the MUST bar reads %q, not its four segments in order", mustBar)
	}
	if !strings.Contains(page, `<div data-ze-rfc-bar="rfc1:overview">Ze MUST proven 3,`) {
		t.Error("the page draws no overview bar for rfc1")
	}
	if !strings.Contains(page, `assets/rfc-figures.js`) {
		t.Error("the page does not load rfc-figures.js, so its figures would never refresh")
	}
	if strings.Contains(html.UnescapeString(page), "eiling") {
		t.Error("the page still states a ceiling")
	}
	for _, text := range []string{"Ze proven 3, OS verified 2", "| Ze proven | 3 |", "data/rfc/rfc1.json"} {
		if !strings.Contains(mirror, text) {
			t.Errorf("the mirror carries no %q", text)
		}
	}
	unwalked := readRFCProfileFile(t, paths.Output, rfcComplianceDirectory+"/rfc3/index.html")
	if !strings.Contains(unwalked, `<span data-ze-rfc="rfc3:must.ze-proven">not walked</span>`) {
		t.Error("an unwalked RFC's figure does not read not walked")
	}
}

// TestRFCDetailPageRefusesAnRFCItsDataDoesNotHold publishes a ledger stem the
// index does not list: the page would print figures the files cannot back,
// so the producer refuses it by stem (AC-18).
func TestRFCDetailPageRefusesAnRFCItsDataDoesNotHold(t *testing.T) {
	paths, ledger := rfcDetailFiguresLedger(t)
	ledger.Stems = append(ledger.Stems, rfcLedgerStem{Stem: "rfc77", Display: "RFC 77", Enrolled: true})
	writeLedgerArtifact(t, paths.Output, ledger)
	_, err := renderRFCCompliance(paths)
	if err == nil || !strings.Contains(err.Error(), "rfc77") {
		t.Fatalf("a stem with no data file published, or was refused without its name: %v", err)
	}
}

// rfcCompliancePagesGolden is the detail and index pages the compliance
// producer renders over rfcDetailFiguresLedger, with the RFC data they read:
// the browser tests in browsertest/ load exactly these.
const rfcCompliancePagesGolden = "testdata/rfc-compliance-pages.json"

// rfcCompliancePagesGoldenContent renders the fixture and answers the golden.
func rfcCompliancePagesGoldenContent(t *testing.T) []byte {
	t.Helper()
	// The footer stamps the build time; a fixed one keeps the golden stable.
	clock := buildClock
	buildClock = func() time.Time { return time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { buildClock = clock })
	paths, _ := rfcDetailFiguresLedger(t)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	detail := rfcComplianceDirectory + "/rfc1/" + pageIndexFile
	files := map[string]string{detail: readRFCProfileFile(t, paths.Output, detail),
		rfcComplianceDest: readRFCProfileFile(t, paths.Output, rfcComplianceDest)}
	data := rfcProfileFiguresData()
	files[rfcDataIndexFile] = readRFCProfileFile(t, paths.Output, rfcDataIndexFile)
	for _, entry := range data.index.RFCs {
		files[entry.File] = readRFCProfileFile(t, paths.Output, entry.File)
	}
	for _, entry := range data.index.Profiles {
		files[entry.File] = readRFCProfileFile(t, paths.Output, entry.File)
	}
	content, err := json.MarshalIndent(map[string]any{"detail": detail, "list": rfcComplianceDest, "files": files}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(content, '\n')
}

// TestTheCompliancePagesGoldenIsCurrent holds testdata/rfc-compliance-pages.json
// to what the producer renders, so the browser tests load the real pages.
//
// Method: render the fixture and compare; -update rewrites the file.
// PREVENTS: a browser test passing over a page the build no longer writes.
func TestTheCompliancePagesGoldenIsCurrent(t *testing.T) {
	want := rfcCompliancePagesGoldenContent(t)
	if *updateRFCBars {
		if err := os.WriteFile(rfcCompliancePagesGolden, want, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(rfcCompliancePagesGolden)
	if err != nil {
		t.Fatalf("read %s: %v. Write it with: ./le job run label unit-pkg command go test -run TestTheCompliancePagesGoldenIsCurrent ./internal/le/site/ -args -update", rfcCompliancePagesGolden, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale: a compliance page changed, so the browser tests load an old page.\n"+
			"Refresh it with: ./le job run label unit-pkg command go test -run TestTheCompliancePagesGoldenIsCurrent ./internal/le/site/ -args -update",
			rfcCompliancePagesGolden)
	}
}
