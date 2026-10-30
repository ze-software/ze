package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// fixtureFigures is one walked RFC whose every bar segment holds a different
// count, so a segment read from the wrong field cannot match by accident.
//
// MUST: 20 rows owed; 5 lower-layer (2 OS verified, 3 OS gap), 4
// not-applicable and 2 feature-declined left out of the attainable rows, so 9
// attainable, 3 of them proven: Ze gap 6, MUST bar 3+2+3+6 = 14, not owed 6.
// SHOULD: 10 rows; 1 proven, 2 not-applicable, then 2 not yet, 1 not ever and
// 4 not decided, which sum back to 10.
func fixtureFigures() rfc.StemFigures {
	return rfc.StemFigures{
		Stem:   "rfc9999",
		Walked: true,
		Mandatory: rfc.Figure{Proven: 3, Total: 20, Attainable: 9, Excluded: map[string]int{
			rfc.AnnotationLowerLayer: 5, rfc.AnnotationNotApplicable: 4, rfc.AnnotationFeatureDeclined: 2,
		}},
		Optional: rfc.Figure{Proven: 1, Total: 10, Attainable: 8, Excluded: map[string]int{
			rfc.AnnotationNotApplicable: 2,
		}},
		MandatoryByKind: map[string]int{"unit": 2, "functional": 1},
		Unmet:           []string{"RFC9999-2-4"},
		OSVerified:      2,
		OSGap:           3,
		NotYet:          2,
		NotEver:         1,
		NotDecided:      4,
	}
}

// TestPerRFCDataCountsEverySegment proves the per-RFC file carries the counts
// of both bars, each read from the field of rfc.StemFigures that answers it,
// and the per-kind split of the rows Ze does not owe.
//
// Method: state one RFC's figures with a distinct count in every field and
// compare the whole published value, so a swapped or summed-wrong segment is a
// difference in the diff rather than a coincidence.
func TestPerRFCDataCountsEverySegment(t *testing.T) {
	got, err := rfcStemDataOf("rfc9999", rfc.Meta{Title: "Widgets"}, fixtureFigures())
	if err != nil {
		t.Fatal(err)
	}
	want := rfcStemData{
		Stem: "rfc9999", Title: "Widgets", Walked: true,
		Must: &rfcMustCounts{
			Total: 20, Kept: 14, ZeProven: 3, OSVerified: 2, OSGap: 3, ZeGap: 6, NotOwed: 6,
			NotOwedByKind: map[string]int{rfc.AnnotationNotApplicable: 4, rfc.AnnotationFeatureDeclined: 2},
		},
		Should: &rfcShouldCounts{
			Total: 10, Proven: 1, NotOwed: 2, NotEver: 1, NotYet: 2, NotDecided: 4,
			NotOwedByKind: map[string]int{rfc.AnnotationNotApplicable: 2},
		},
		ProvenByKind: map[string]int{"unit": 2, "functional": 1},
		Unmet:        []string{"RFC9999-2-4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("per-RFC data\n got: %+v %+v %+v\nwant: %+v %+v %+v", got, got.Must, got.Should, want, want.Must, want.Should)
	}
	// Every segment of both bars adds back to the totals the bar layout names.
	for _, bar := range []struct {
		name     string
		segments []rfcBarSegment
		total    int
	}{{"must", rfcBars.Must, got.Must.Kept}, {"overview", rfcBars.Overview, got.Must.Total + got.Should.Total}} {
		sum := 0
		for _, segment := range bar.segments {
			for _, field := range segment.Fields {
				sum += got.field(t, field)
			}
		}
		if sum != bar.total {
			t.Errorf("%s bar segments sum to %d, want %d", bar.name, sum, bar.total)
		}
	}
}

// field answers one dotted count field of the published file, read back
// through its JSON form so the test reads the names a browser reads.
func (d rfcStemData) field(t *testing.T, path string) int {
	t.Helper()
	body, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("field %s: %v is not an object", path, value)
		}
		value = object[part]
	}
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("field %s is %v, not a count", path, value)
	}
	return int(number)
}

// TestTheBarLayoutKeepsTheAgreedOrder proves AC-12's order is the one the
// published layout declares, segment by segment.
func TestTheBarLayoutKeepsTheAgreedOrder(t *testing.T) {
	names := func(segments []rfcBarSegment) []string {
		var out []string
		for _, segment := range segments {
			out = append(out, segment.Segment)
		}
		return out
	}
	if got, want := names(rfcBars.Must), []string{"ze-proven", "os-verified", "os-gap", "ze-gap"}; !slices.Equal(got, want) {
		t.Errorf("MUST bar order %v, want %v", got, want)
	}
	want := []string{"ze-proven", "os-verified", "should-proven", "not-owed", "not-ever", "not-yet",
		"not-decided", "os-gap", "ze-gap"}
	if got := names(rfcBars.Overview); !slices.Equal(got, want) {
		t.Errorf("overview bar order %v, want %v", got, want)
	}
}

// TestAnUnwalkedRFCPublishesNoCounts proves an RFC whose requirement list no
// walk bounded publishes its unmet disclosure and no figure.
func TestAnUnwalkedRFCPublishesNoCounts(t *testing.T) {
	figures := fixtureFigures()
	figures.Walked = false
	got, err := rfcStemDataOf("rfc9999", rfc.Meta{}, figures)
	if err != nil {
		t.Fatal(err)
	}
	if got.Walked || got.Must != nil || got.Should != nil || got.ProvenByKind != nil {
		t.Fatalf("an unwalked RFC published counts: %+v", got)
	}
	if !slices.Equal(got.Unmet, []string{"RFC9999-2-4"}) {
		t.Fatalf("an unwalked RFC lost its unmet disclosure: %+v", got.Unmet)
	}
}

// TestFiguresThatDoNotAddUpAreRefused proves the producer refuses, by stem,
// figures whose split does not add back to the model's own totals, rather
// than publishing a bar whose segments disagree with its total.
func TestFiguresThatDoNotAddUpAreRefused(t *testing.T) {
	// Each case breaks one sum, and its refusal names the sum that broke.
	for name, check := range map[string]struct {
		edit func(*rfc.StemFigures)
		says string
	}{
		"OS split differs from the lower-layer rows":   {func(f *rfc.StemFigures) { f.OSGap++ }, "OS verified and OS gap"},
		"proven exceeds the attainable rows":           {func(f *rfc.StemFigures) { f.Mandatory.Proven = 10 }, "Ze-proven MUST rows"},
		"MUST sections differ from the MUST total":     {func(f *rfc.StemFigures) { f.Mandatory.Total++ }, "the MUST sections"},
		"SHOULD states differ from the open rows":      {func(f *rfc.StemFigures) { f.NotDecided++ }, "not decided SHOULD rows"},
		"SHOULD sections differ from the SHOULD total": {func(f *rfc.StemFigures) { f.Optional.Total++ }, "the SHOULD sections"},
	} {
		t.Run(name, func(t *testing.T) {
			figures := fixtureFigures()
			check.edit(&figures)
			_, err := rfcStemDataOf("rfc9999", rfc.Meta{}, figures)
			if err == nil {
				t.Fatal("figures that do not add up were published")
			}
			if !strings.HasPrefix(err.Error(), "rfc9999: ") || !strings.Contains(err.Error(), check.says) {
				t.Fatalf("the refusal does not name the stem and %q: %v", check.says, err)
			}
		})
	}
}

// TestProfileDataKeepsTheTiersApart proves a profile file lists its required
// and optional members as separate tiers, for a use case and for an
// aggregate, the aggregate's optional tier included.
func TestProfileDataKeepsTheTiersApart(t *testing.T) {
	profiles := rfc.Profiles{
		UseCases:   map[string]rfc.UseCase{"edge": {Name: "edge", Title: "Edge", Release: true}},
		Aggregates: map[string]rfc.Aggregate{"ospf": {Name: "ospf", Title: "OSPF"}},
	}
	metas := map[string]rfc.Meta{
		"rfc2328": {Aggregate: "ospf", UseCases: []string{"edge"}},
		"rfc4577": {Aggregate: "ospf", AggregateOptional: true, OptionalUseCases: []string{"edge"}},
		"rfc4576": {Aggregate: "ospf", AggregateOptional: true},
	}
	got := rfcProfileDataOf(profiles, metas)
	want := []rfcProfileData{
		{Name: "edge", Kind: "use-case", Title: "Edge", Release: true,
			Required: []string{"rfc2328"}, Optional: []string{"rfc4577"}},
		{Name: "ospf", Kind: "technology", Title: "OSPF",
			Required: []string{"rfc2328"}, Optional: []string{"rfc4576", "rfc4577"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profile data\n got: %+v\nwant: %+v", got, want)
	}
}

// TestDataIndexAndFilesAgree proves R-6: the build writes every file the
// index names and no file the index does not, a stale file from the previous
// artifact included, and each per-RFC file's counts equal rfc.StemFiguresOf
// over the same corpus (AC-16).
//
// Method: build a one-RFC corpus through the production readers (Collect,
// NewRenderInput), seed the output with a file for a stem that no longer
// exists, publish, and walk data/rfc/ against the index.
func TestDataIndexAndFilesAgree(t *testing.T) {
	root, summary, marker := partialPublicationTree(t)
	writePartialPublicationFile(t, root, "rfc/short/rfc9999.md", strings.ReplaceAll(summary, marker, "{gap: native sending absent}"))
	output := t.TempDir()
	writePartialPublicationFile(t, output, rfcDataDir+"/rfc0001.json", "{}\n")
	if err := publishRFCData(Paths{Repository: root, Output: output}); err != nil {
		t.Fatal(err)
	}
	var index rfcDataIndex
	readPublishedJSON(t, output, rfcDataIndexFile, &index)
	listed := map[string]bool{rfcDataIndexFile: true}
	for _, entry := range index.RFCs {
		listed[entry.File] = true
	}
	for _, entry := range index.Profiles {
		listed[entry.File] = true
	}
	if len(index.RFCs) != 1 || index.RFCs[0].Stem != "rfc9999" || index.RFCs[0].Title != "Widgets" ||
		index.RFCs[0].Enrolment != "enrolled" || len(index.Profiles) != 1 {
		t.Fatalf("index lost or invented a member: %+v", index)
	}
	err := filepath.WalkDir(filepath.Join(output, filepath.FromSlash(rfcDataDir)), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		if !listed[filepath.ToSlash(rel)] {
			t.Errorf("%s is published but no index entry names it", rel)
		}
		delete(listed, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Errorf("index entries with no file: %v", listed)
	}

	collected, err := rfc.Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	input, err := rfc.NewRenderInput(root, collected, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	figures, err := rfc.StemFiguresOf(&input)
	if err != nil {
		t.Fatal(err)
	}
	want, err := rfcStemDataOf("rfc9999", input.Metas["rfc9999"], figures["rfc9999"])
	if err != nil {
		t.Fatal(err)
	}
	var got rfcStemData
	readPublishedJSON(t, output, index.RFCs[0].File, &got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("published file differs from the model\n got: %+v\nwant: %+v", got, want)
	}
	if !slices.Equal(got.Unmet, []string{"RFC9999-2-1"}) {
		t.Fatalf("the gap row is not disclosed: %+v", got.Unmet)
	}
}

func readPublishedJSON(t *testing.T, output, rel string, into any) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
}

// TestTheNativeRefreshPublishesTheRFCData proves the build writes data/rfc/
// and publishes the renderer beside site.js: refreshNativeSurfaces, the step
// `./le site build` runs before any producer, carries both.
//
// Method: a stubbed build over a synthetic checkout whose stated RFC data
// holds one walked RFC and whose website carries rfc-figures.js.
func TestTheNativeRefreshPublishesTheRFCData(t *testing.T) {
	stubLiveInputs(t, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
	stem, err := rfcStemDataOf("rfc9999", rfc.Meta{Title: "Widgets"}, fixtureFigures())
	if err != nil {
		t.Fatal(err)
	}
	liveRFCData = func(string) (rfcData, error) {
		return rfcData{index: rfcDataIndex{Bars: rfcBars, RFCs: []rfcIndexRFC{{Stem: "rfc9999",
			File: rfcStemDataFile("rfc9999"), Title: "Widgets", Walked: true, Enrolment: "enrolled"}}},
			stems: []rfcStemData{stem}}, nil
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	output := filepath.Join(parent, "gh-pages")
	writeSourceAssets(t, source)
	writeSourceAsset(t, source, filepath.Join("js", "rfc-figures.js"), "window.zeRFCFigures = {};\n")
	roadmapGit(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(output, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths, err := resolvePaths(root, output)
	if err != nil {
		t.Fatal(err)
	}
	if err := refreshNativeSurfaces(paths); err != nil {
		t.Fatal(err)
	}
	var got rfcStemData
	readPublishedJSON(t, output, rfcStemDataFile("rfc9999"), &got)
	if !reflect.DeepEqual(got, stem) {
		t.Fatalf("published %+v, want %+v", got, stem)
	}
	if missing := checkNamedArtifacts(output); slices.Contains(missing, rfcDataIndexFile) {
		t.Fatalf("the build did not publish %s", rfcDataIndexFile)
	}
	if script := readArtifact(t, output, "assets/rfc-figures.js"); script != "window.zeRFCFigures = {};\n" {
		t.Fatalf("assets/rfc-figures.js published as %q", script)
	}
}
