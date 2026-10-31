package site

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcProfileFiguresData is the RFC data the profile figure tests publish:
// rfc1 holds a count in every segment and a Ze gap past a thousand, rfc2 only
// Ze-proven MUST rows, rfc3 is unwalked, rfc4 sits in edge's optional tier.
// Profile edge requires rfc1 to rfc3; core requires rfc2 alone, so its
// register is complete; lab requires rfc2 and the unwalked rfc3, so its
// register is open with no gap. Lab has no page: only set answers read it.
// rfc5 is walked with only an OS gap, rfc6 with only a Ze gap: each keeps its
// own register open by one clause alone. rfc6 is the one RFC not enrolled,
// and every RFC declares a requirement count, the unwalked rfc3 included.
func rfcProfileFiguresData() rfcData {
	stems := []rfcStemData{
		{Stem: "rfc1", Title: "One", Walked: true, Requirements: 1300, Unmet: []string{}, ProvenByKind: map[string]int{},
			Must: &rfc.MustSections{Kept: 1206, ZeProven: 3, OSVerified: 2, OSGap: 1, ZeGap: 1200, NotOwed: 10,
				NotOwedByKind: map[string]int{rfc.AnnotationNotApplicable: 4, rfc.AnnotationFeatureDeclined: 6}},
			Should: &rfc.ShouldSections{Total: 10, Proven: 1, OSVerified: 1, OSGap: 1, NotOwed: 2, NotEver: 1, NotYet: 2, NotDecided: 2,
				NotOwedByKind: map[string]int{rfc.AnnotationNotApplicable: 2}}},
		{Stem: "rfc2", Title: "Two", Walked: true, Requirements: 6, Unmet: []string{}, ProvenByKind: map[string]int{},
			Must:   &rfc.MustSections{Total: 4, Kept: 4, ZeProven: 4, NotOwedByKind: map[string]int{}},
			Should: &rfc.ShouldSections{NotOwedByKind: map[string]int{}}},
		{Stem: "rfc3", Title: "Three", Requirements: 7, Unmet: []string{}},
		{Stem: "rfc5", Title: "Five", Walked: true, Requirements: 3, Unmet: []string{}, ProvenByKind: map[string]int{},
			Must:   &rfc.MustSections{Total: 3, Kept: 3, ZeProven: 2, OSGap: 1, NotOwedByKind: map[string]int{}},
			Should: &rfc.ShouldSections{NotOwedByKind: map[string]int{}}},
		{Stem: "rfc6", Title: "Six", Walked: true, Requirements: 5, Unmet: []string{}, ProvenByKind: map[string]int{},
			Must:   &rfc.MustSections{Total: 3, Kept: 3, ZeProven: 1, ZeGap: 2, NotOwedByKind: map[string]int{}},
			Should: &rfc.ShouldSections{NotOwedByKind: map[string]int{}}},
		{Stem: "rfc4", Title: "Four", Walked: true, Requirements: 9, Unmet: []string{}, ProvenByKind: map[string]int{},
			Must:   &rfc.MustSections{Total: 9, Kept: 9, ZeProven: 9, NotOwedByKind: map[string]int{}},
			Should: &rfc.ShouldSections{NotOwedByKind: map[string]int{}}},
		{Stem: "rfc7", Title: "Seven", Requirements: 2, Unmet: []string{}},
	}
	// rfc1's MUST total is its kept rows plus its rows not owed.
	stems[0].Must.Total = stems[0].Must.Kept + stems[0].Must.NotOwed
	data := rfcData{index: rfcDataIndex{Bars: rfcBars}, stems: stems, profiles: []rfcProfileData{
		{Name: "core", Kind: "use-case", Title: "Core", Required: []string{"rfc2"}, Optional: []string{}},
		{Name: "edge", Kind: "use-case", Title: "Edge", Release: true, Required: []string{"rfc1", "rfc2", "rfc3"}, Optional: []string{"rfc4"}},
		{Name: "lab", Kind: "use-case", Title: "Lab", Required: []string{"rfc2", "rfc3"}, Optional: []string{}},
	}}
	// rfc6 is not enrolled, and rfc7 is enrolled but third-party, which the
	// gate never holds Ze to, so the enrolled subject sums the other five: a
	// reader that took the enrolment word would count six. rfc4 is enrolled
	// with no public row, so rfc.Implements leaves it out and the RFCs Ze
	// implements are the other four.
	for index := range stems {
		enrolment := "enrolled"
		if stems[index].Stem == "rfc6" {
			enrolment = "backlog"
		}
		gated := enrolment == "enrolled" && stems[index].Stem != "rfc7"
		stems[index].Implementer = "ze"
		if stems[index].Stem == "rfc7" {
			stems[index].Implementer = "third-party"
		}
		stems[index].Implemented = gated && stems[index].Stem != "rfc4"
		data.index.RFCs = append(data.index.RFCs, rfcIndexRFC{Stem: stems[index].Stem, File: rfcStemDataFile(stems[index].Stem),
			Title: stems[index].Title, Walked: stems[index].Walked, Enrolment: enrolment, Gated: gated})
	}
	for index := range data.profiles {
		profile := &data.profiles[index]
		data.index.Profiles = append(data.index.Profiles, rfcIndexProfile{Name: profile.Name,
			File: rfcProfileDataFile(profile.Name), Kind: profile.Kind, Title: profile.Title})
	}
	return data
}

// rfcProfileFiguresLedger is the ledger whose trees list the same members.
// Its rows carry no figure: the pages read every count from data/rfc/.
func rfcProfileFiguresLedger() rfcLedger {
	zero, one := 0, 1
	row := func(stem string, level *int, path string) *rfc.TreeRow {
		return &rfc.TreeRow{Stem: stem, Title: "RFC " + stem, Level: level, Summary: true, Component: []string{}, Path: path}
	}
	edge := &rfc.TreeReport{Profile: "edge", Title: "Edge", Kind: "use-case", Release: true,
		Rows: []*rfc.TreeRow{row("rfc1", &zero, "edge: required by rfc/short/rfc1.md"),
			row("rfc2", &zero, "edge: required by rfc/short/rfc2.md"), row("rfc3", &one, "edge: required by rfc/short/rfc3.md")},
		Optional: &rfc.TreeTier{Rows: []*rfc.TreeRow{row("rfc4", &zero, "edge: optional in rfc/short/rfc4.md")}}}
	core := &rfc.TreeReport{Profile: "core", Title: "Core", Kind: "use-case",
		Rows: []*rfc.TreeRow{row("rfc2", &zero, "core: required by rfc/short/rfc2.md")}, Optional: &rfc.TreeTier{}}
	return rfcLedger{Stems: []rfcLedgerStem{{Stem: "rfc1", Display: "RFC 1", Enrolled: true},
		{Stem: "rfc2", Display: "RFC 2", Enrolled: true}, {Stem: "rfc3", Display: "RFC 3", Enrolled: true},
		{Stem: "rfc4", Display: "RFC 4", Enrolled: true}}, Profiles: []*rfc.TreeReport{core, edge}}
}

// publishRFCProfileFigures writes the ledger and the RFC data into a fresh
// artifact, as refreshNativeSurfaces does before the producers run.
func publishRFCProfileFigures(t *testing.T, ledger rfcLedger, data rfcData) string {
	t.Helper()
	output := t.TempDir()
	content, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeNamedArtifact(output, rfcLedgerFile, string(content)); err != nil {
		t.Fatal(err)
	}
	if err := writeRFCData(output, &data); err != nil {
		t.Fatal(err)
	}
	return output
}

// renderRFCProfileFigures runs the producer over a published artifact.
func renderRFCProfileFigures(output string) error {
	_, err := renderRFCProfiles(Paths{Repository: output, Source: filepath.Join(output, "no-source"), Output: output})
	return err
}

// readRFCProfileFile reads one file the producer wrote.
func readRFCProfileFile(t *testing.T, output, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(name))) //nolint:gosec // the test's own temporary artifact
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// The fallback texts the fixture's bars and figures carry.
const (
	edgeRequiredMust = "Ze proven 7, OS verified 2, OS testing gap 1, Ze correctness gap 1,200"
	edgeRequiredView = "Ze MUST proven 7, OS verified 3, SHOULD proven 1, Not owed 12, SHOULD not ever 1, " +
		"SHOULD not yet 2, SHOULD not decided 2, OS testing gap 2, Ze correctness gap 1,200"
	edgeOptionalMust = "Ze proven 9"
)

// TestRFCProfilePagesCarryTheMarkersAndTheLastBuildValues proves a profile
// page marks each tier's two bars and every figure with the subject the RFC
// data answers, writes the last build's value inside each marked element,
// marks each RFC row's bars, loads rfc-figures.js, and states the same values
// in its mirror with links to the files they came from.
//
// Method: publish a ledger and RFC data for two profiles, run the producer,
// and read edge's page, edge's mirror and the index.
// PREVENTS: a page printing a figure the files do not hold, a tier drawn
// without a marker, a marked element left empty, and a mirror holding no value.
func TestRFCProfilePagesCarryTheMarkersAndTheLastBuildValues(t *testing.T) {
	output := publishRFCProfileFigures(t, rfcProfileFiguresLedger(), rfcProfileFiguresData())
	if err := renderRFCProfileFigures(output); err != nil {
		t.Fatalf("renderRFCProfiles: %v", err)
	}
	page := readRFCProfileFile(t, output, rfcProfilesDirectory+"/edge/"+pageIndexFile)
	for _, want := range []string{
		`<div data-ze-rfc-bar="profile/edge/required:must">` + edgeRequiredMust + `</div>`,
		`<div data-ze-rfc-bar="profile/edge/required:overview">` + edgeRequiredView + `</div>`,
		`<div data-ze-rfc-bar="profile/edge/optional:must">` + edgeOptionalMust + `</div>`,
		`<div data-ze-rfc-bar="profile/edge/optional:overview">Ze MUST proven 9</div>`,
		`<span data-ze-rfc="profile/edge/required:rfcs">3</span>`,
		`<span data-ze-rfc="profile/edge/required:not-walked">1</span>`,
		`<span data-ze-rfc="profile/edge/required:register">not complete</span>`,
		`<span data-ze-rfc="profile/edge/required:must.total">1,220</span>`,
		`<span data-ze-rfc="profile/edge/required:must.kept">1,210</span>`,
		`<span data-ze-rfc="profile/edge/required:must.ze-gap">1,200</span>`,
		`<span data-ze-rfc="profile/edge/required:must.not-owed-by-kind.feature-declined">6</span>`,
		`<span data-ze-rfc="profile/edge/required:must.not-owed-by-kind.linux-limit">0</span>`,
		`<span data-ze-rfc="profile/edge/required:should.not-decided">2</span>`,
		`<span data-ze-rfc="profile/edge/optional:register">complete</span>`,
		`<span data-ze-rfc="profile/edge/optional:must.ze-proven">9</span>`,
		`<div data-ze-rfc-bar="rfc1:must">Ze proven 3, OS verified 2, OS testing gap 1, Ze correctness gap 1,200</div>`,
		`<div data-ze-rfc-bar="rfc3:overview">not walked</div>`,
		`<div data-ze-rfc-bar="rfc4:must">Ze proven 9</div>`,
		`<script src="../../../../assets/rfc-figures.js" defer></script>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("edge's page omits %s", want)
		}
	}
	for _, empty := range []*regexp.Regexp{rfcValueElement, rfcBarElement} {
		if left := empty.FindString(page); left != "" {
			t.Errorf("a marked element carries no fallback: %s", left)
		}
	}
	mirror := readRFCProfileFile(t, output, rfcProfilesDirectory+"/edge/"+pageMirrorFile)
	for _, want := range []string{"**MUST bar**: " + edgeRequiredMust, "**Overview bar**: " + edgeRequiredView,
		"**MUST bar**: " + edgeOptionalMust, "| Mandatory register | not complete |", "| Ze correctness gap | 1,200 |",
		"[data/rfc/profiles/edge.json](../../../../data/rfc/profiles/edge.json)",
		"[data/rfc/index.json](../../../../data/rfc/index.json)"} {
		if !strings.Contains(mirror, want) {
			t.Errorf("edge's mirror omits %q", want)
		}
	}
	if strings.Contains(mirror, "{{ze-rfc") {
		t.Errorf("edge's mirror kept an unfilled mark:\n%s", mirror)
	}
	index := readRFCProfileFile(t, output, rfcProfilesDest)
	for _, want := range []string{
		`<div data-ze-rfc-bar="profile/edge/required:must">` + edgeRequiredMust + `</div>`,
		`<div data-ze-rfc-bar="profile/core/required:must">Ze proven 4</div>`,
		`<span data-ze-rfc="profile/edge/required:rfcs">3</span></a>`,
		`<span data-ze-rfc="profile/core/required:register">complete</span>`,
		`<span data-ze-rfc="profile/edge/required:not-walked">1</span>`,
		`<script src="../../../assets/rfc-figures.js" defer></script>`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("the profile index omits %s", want)
		}
	}
	indexMirror := readRFCProfileFile(t, output, rfcProfilesDirectory+"/"+pageMirrorFile)
	if !strings.Contains(indexMirror, "| "+edgeRequiredMust+" |") || strings.Contains(indexMirror, "{{ze-rfc") {
		t.Errorf("the index mirror does not state edge's MUST bar:\n%s", indexMirror)
	}
}

// TestRFCFigureSetAnswersAsTheScriptDoes proves the fallback filler answers
// each case the way rfc-figures.js fillValue and drawBar do.
//
// Method: one figure set, each answer named by its case.
// PREVENTS: a fallback that differs from what the browser then writes.
func TestRFCFigureSetAnswersAsTheScriptDoes(t *testing.T) {
	output := publishRFCProfileFigures(t, rfcProfileFiguresLedger(), rfcProfileFiguresData())
	set, err := loadRFCFigureSet(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ name, subject, field, want string }{
		{"a count grouped by thousands", "rfc1", "must.ze-gap", "1,200"},
		{"a count below a thousand", "rfc1", "must.ze-proven", "3"},
		{"an unwalked RFC", "rfc3", "must.ze-proven", "not walked"},
		{"an empty tier", "profile/core/optional", "must.ze-proven", "no rows"},
		{"a by-kind count no file holds", "rfc2", "must.not-owed-by-kind.not-applicable", "0"},
		{"an unwalked member keeps a gapless register open", "profile/lab/required", "register", rfcRegisterOpen},
		{"a gapless walked set is complete", "profile/core/required", "register", rfcRegisterComplete},
		{"an empty set is not complete", "profile/core/optional", "register", rfcRegisterOpen},
		{"a Ze gap alone keeps the register open", "rfc6", "register", rfcRegisterOpen},
		{"an OS gap alone keeps the register open", "rfc5", "register", rfcRegisterOpen},
		{"a walked RFC with neither gap is complete", "rfc2", "register", rfcRegisterComplete},
		{"a set sums walked members only", "profile/edge/required", "walked", "2"},
	} {
		got, err := set.value(check.subject, check.field)
		if err != nil {
			t.Errorf("%s: %v", check.name, err)
			continue
		}
		if got != check.want {
			t.Errorf("%s: %s:%s is %q, want %q", check.name, check.subject, check.field, got, check.want)
		}
	}
	for _, refused := range []struct{ subject, field string }{
		{"rfc1", "must.no-such-count"}, {"rfc9", "must.total"}, {"profile/nobody/required", "rfcs"},
		{"profile/edge/everything", "rfcs"}, {"rfc1", "must.not-owed-by-kind.declind"},
	} {
		if got, err := set.value(refused.subject, refused.field); err == nil {
			t.Errorf("%s:%s answered %q, want a refusal", refused.subject, refused.field, got)
		}
	}
	if got, err := set.bar("profile/core/optional", "must"); err != nil || got != "no rows" {
		t.Errorf("an empty tier's bar: %q, %v", got, err)
	}
	if got, err := set.bar("rfc1", "sideways"); err == nil {
		t.Errorf("an undeclared bar answered %q", got)
	}
	for _, stray := range []string{`<span class="x" data-ze-rfc="rfc1:must.total"></span>`,
		`<p data-ze-rfc="rfc1:must.total"></p>`, `<div data-ze-rfc-bar="rfc1:must" title="t"></div>`} {
		if got, err := set.fillHTML("<main>" + stray + "</main>"); err == nil {
			t.Errorf("a marker out of the fill's shape published as %q, want a refusal", got)
		}
	}
	if got, err := set.fillMirror("| x | {{ze-rfc rfc1 must.total}} |"); err == nil {
		t.Errorf("a mistyped mirror token published as %q, want a refusal", got)
	}
	must, isMap := set.stems["rfc2"]["must"].(map[string]any)
	if !isMap {
		t.Fatalf("the fixture's rfc2 file holds no must counts: %v", set.stems["rfc2"])
	}
	must["kept"] = float64(5)
	if got, err := set.bar("rfc2", "must"); err == nil {
		t.Errorf("a bar whose segments miss its total answered %q", got)
	}
}

// TestRFCProfilePagesRefuseFiguresTheyCannotTrust proves the producer refuses
// a profile whose table and bars would count different RFCs, and a page
// whose RFC data file is missing, naming each.
//
// Method: a ledger tree one member short of the profile file, then a removed
// per-RFC file.
// PREVENTS: a bar over RFCs the table does not show, and a sum that silently
// leaves an RFC out (AC-18 at the build).
func TestRFCProfilePagesRefuseFiguresTheyCannotTrust(t *testing.T) {
	short := rfcProfileFiguresLedger()
	short.Profiles[1].Rows = short.Profiles[1].Rows[:2]
	err := renderRFCProfileFigures(publishRFCProfileFigures(t, short, rfcProfileFiguresData()))
	if err == nil || !strings.Contains(err.Error(), "profile edge: the required tier") {
		t.Errorf("a tree one member short of its profile file: %v", err)
	}
	output := publishRFCProfileFigures(t, rfcProfileFiguresLedger(), rfcProfileFiguresData())
	if err := os.Remove(filepath.Join(output, filepath.FromSlash(rfcStemDataFile("rfc2")))); err != nil {
		t.Fatal(err)
	}
	err = renderRFCProfileFigures(output)
	if err == nil || !strings.Contains(err.Error(), rfcStemDataFile("rfc2")) || !strings.Contains(err.Error(), "./le site build") {
		t.Errorf("a missing per-RFC file: %v", err)
	}
}

// rfcProfilePageGolden is the rendered edge page and the RFC data it reads,
// which the browser test serves: that package cannot reach the producer.
const rfcProfilePageGolden = "testdata/rfc-profile-page.json"

// rfcProfilePageGoldenContent renders the fixture and answers the golden.
func rfcProfilePageGoldenContent(t *testing.T) []byte {
	t.Helper()
	// The footer stamps the build time; a fixed one keeps the golden stable.
	clock := buildClock
	buildClock = func() time.Time { return time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { buildClock = clock })
	output := publishRFCProfileFigures(t, rfcProfileFiguresLedger(), rfcProfileFiguresData())
	if err := renderRFCProfileFigures(output); err != nil {
		t.Fatal(err)
	}
	page := rfcProfilesDirectory + "/edge/" + pageIndexFile
	files := map[string]string{page: readRFCProfileFile(t, output, page)}
	data := rfcProfileFiguresData()
	files[rfcDataIndexFile] = readRFCProfileFile(t, output, rfcDataIndexFile)
	for _, entry := range data.index.RFCs {
		files[entry.File] = readRFCProfileFile(t, output, entry.File)
	}
	for _, entry := range data.index.Profiles {
		files[entry.File] = readRFCProfileFile(t, output, entry.File)
	}
	content, err := json.MarshalIndent(map[string]any{"page": page, "files": files}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(content, '\n')
}

// TestTheProfilePageGoldenIsCurrent holds testdata/rfc-profile-page.json to
// what the producer renders, so the browser test loads the real page.
//
// Method: render the fixture and compare; -update rewrites the file.
// PREVENTS: a browser test passing over a page the build no longer writes.
func TestTheProfilePageGoldenIsCurrent(t *testing.T) {
	want := rfcProfilePageGoldenContent(t)
	if *updateRFCBars {
		if err := os.WriteFile(rfcProfilePageGolden, want, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(rfcProfilePageGolden)
	if err != nil {
		t.Fatalf("read %s: %v. Write it with: ./le job run label unit-pkg command go test -run TestTheProfilePageGoldenIsCurrent ./internal/le/site/ -args -update", rfcProfilePageGolden, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale: the profile page changed, so the browser test loads an old page.\n"+
			"Refresh it with: ./le job run label unit-pkg command go test -run TestTheProfilePageGoldenIsCurrent ./internal/le/site/ -args -update",
			rfcProfilePageGolden)
	}
}
