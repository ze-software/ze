package site

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// fixtureFigures is one walked RFC whose every bar segment holds a different
// count, so a segment read from the wrong field cannot match by accident.
//
// MUST: 20 rows owed; 5 lower-layer (2 OS verified, 3 OS gap), 4
// not-applicable and 2 feature-declined left out of the attainable rows, so 9
// attainable, 3 of them proven: 6 open (1 proof pending, 2 exclusion
// unchecked, 0 not assessed, 3 not built), MUST bar 3+2+3+6 = 14, not owed 6.
// SHOULD: 13 rows; 1 proven, 2 not-applicable, 3 lower-layer (1 OS verified,
// 2 OS gap), then 7 open (2 proof pending, 0 exclusion unchecked, 4 not
// assessed, 1 not built), which sum back to 13. The private {intent} split of
// the open SHOULD rows (2 not yet, 1 not ever, 4 not decided) is kept too, and
// no section publishes it.
func fixtureFigures() rfc.StemFigures {
	return rfc.StemFigures{
		Stem:   "rfc9999",
		Walked: true,
		Mandatory: rfc.Figure{Proven: 3, Total: 20, Attainable: 9, Excluded: map[string]int{
			rfc.AnnotationLowerLayer: 5, rfc.AnnotationNotApplicable: 4, rfc.AnnotationFeatureDeclined: 2,
		}},
		Optional: rfc.Figure{Proven: 1, Total: 13, Attainable: 8, Excluded: map[string]int{
			rfc.AnnotationNotApplicable: 2, rfc.AnnotationLowerLayer: 3,
		}},
		MandatoryByKind: map[string]int{"unit": 2, "functional": 1},
		Unmet:           []string{"RFC9999-2-4"},
		OSVerified:      2,
		OSGap:           3,
		// The optional OS rows (owner, 2026-10-31): split as the MUST ones are.
		OptionalOSVerified: 1,
		OptionalOSGap:      2,
		ProofPending:       1,
		ExclusionUnchecked: 2,
		NotAssessed:        0,
		NotBuilt:           3,
		// The open optional rows, split as the open MUST ones are.
		OptionalProofPending:       2,
		OptionalExclusionUnchecked: 0,
		OptionalNotAssessed:        4,
		OptionalNotBuilt:           1,
		NotYet:                     2,
		NotEver:                    1,
		NotDecided:                 4,
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
	got, err := rfcStemDataOf("rfc9999", "Widgets", fixtureFigures())
	if err != nil {
		t.Fatal(err)
	}
	want := rfcStemData{
		Stem: "rfc9999", Title: "Widgets", Walked: true,
		Must: &rfc.MustSections{
			Total: 20, Kept: 14, ZeProven: 3, OSVerified: 2, OSGap: 3,
			ProofPending: 1, ExclusionUnchecked: 2, NotAssessed: 0, NotBuilt: 3, NotOwed: 6,
			NotOwedByKind: map[string]int{rfc.AnnotationNotApplicable: 4, rfc.AnnotationFeatureDeclined: 2},
		},
		Should: &rfc.ShouldSections{
			Total: 13, Proven: 1, OSVerified: 1, OSGap: 2, NotOwed: 2,
			ProofPending: 2, ExclusionUnchecked: 0, NotAssessed: 4, NotBuilt: 1,
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
	for part := range strings.SplitSeq(path, ".") {
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
	if got, want := names(rfcBars.Must), []string{"ze-proven", "os-verified", "proof-pending", "os-gap",
		"exclusion-unchecked", "not-assessed", "not-built"}; !slices.Equal(got, want) {
		t.Errorf("MUST bar order %v, want %v", got, want)
	}
	want := []string{"ze-proven", "os-verified", "should-proven", "not-owed", "proof-pending", "os-gap",
		"exclusion-unchecked", "not-assessed", "not-built"}
	if got := names(rfcBars.Overview); !slices.Equal(got, want) {
		t.Errorf("overview bar order %v, want %v", got, want)
	}
}

// TestAnUnwalkedRFCPublishesNoCounts proves an RFC whose requirement list no
// walk bounded publishes its unmet disclosure and no figure.
func TestAnUnwalkedRFCPublishesNoCounts(t *testing.T) {
	figures := fixtureFigures()
	figures.Walked = false
	got, err := rfcStemDataOf("rfc9999", "", figures)
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
		"OS split differs from the lower-layer rows":        {func(f *rfc.StemFigures) { f.OSGap++ }, "OS verified and OS gap"},
		"proven exceeds the attainable rows":                {func(f *rfc.StemFigures) { f.Mandatory.Proven = 10 }, "Ze-proven MUST rows"},
		"MUST sections differ from the MUST total":          {func(f *rfc.StemFigures) { f.Mandatory.Total++ }, "the MUST sections"},
		"MUST open sections differ from the open rows":      {func(f *rfc.StemFigures) { f.NotBuilt++ }, "not assessed and not built MUST rows"},
		"SHOULD states differ from the open rows":           {func(f *rfc.StemFigures) { f.OptionalNotAssessed++ }, "not assessed and not built SHOULD rows"},
		"SHOULD OS split differs from the lower-layer rows": {func(f *rfc.StemFigures) { f.OptionalOSGap++ }, "OS verified and OS gap SHOULD rows"},
		"SHOULD sections differ from the SHOULD total":      {func(f *rfc.StemFigures) { f.Optional.Total++ }, "the SHOULD sections"},
	} {
		t.Run(name, func(t *testing.T) {
			figures := fixtureFigures()
			check.edit(&figures)
			_, err := rfcStemDataOf("rfc9999", "", figures)
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
	got, err := rfcProfileDataOf(profiles, metas)
	if err != nil {
		t.Fatal(err)
	}
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
// NewRenderInput) and give its stem an extraction sign-off, so it is walked
// and publishes counts. Seed the output with a file for a stem that no longer
// exists, publish, and walk data/rfc/ against the index. The published counts
// are compared with the numbers the corpus states (one MUST row, a {gap}, so
// one not built) and with the rfc.StemFigures fields directly, never with
// rfcStemDataOf, the function under test.
func TestDataIndexAndFilesAgree(t *testing.T) {
	root, summary, marker := partialPublicationTree(t)
	writePartialPublicationFile(t, root, "rfc/short/rfc9999.md", strings.ReplaceAll(summary, marker, "{gap: native sending absent}"))
	if err := rfc.FixtureProof(root, rfc.FixtureStem, map[string]string{"2:1": "RFC9999-2-1"}); err != nil {
		t.Fatal(err)
	}
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
	model := figures["rfc9999"]
	if !model.Walked {
		t.Fatal("the signed-off stem is not walked, so its counts would not be published")
	}
	var got rfcStemData
	readPublishedJSON(t, output, index.RFCs[0].File, &got)
	if !got.Walked || got.Must == nil || got.Should == nil {
		t.Fatalf("the walked stem published no counts: %+v", got)
	}
	wantMust := rfc.MustSections{Total: 1, Kept: 1, NotBuilt: 1, NotOwedByKind: map[string]int{}}
	if !reflect.DeepEqual(*got.Must, wantMust) {
		t.Errorf("published MUST counts\n got: %+v\nwant: %+v", *got.Must, wantMust)
	}
	wantShould := rfc.ShouldSections{NotOwedByKind: map[string]int{}}
	if !reflect.DeepEqual(*got.Should, wantShould) {
		t.Errorf("published SHOULD counts\n got: %+v\nwant: %+v", *got.Should, wantShould)
	}
	for _, pair := range []struct {
		field     string
		got, want int
	}{
		{"must.total", got.Must.Total, model.Mandatory.Total},
		{"must.ze-proven", got.Must.ZeProven, model.Mandatory.Proven},
		{"must.os-verified", got.Must.OSVerified, model.OSVerified},
		{"must.os-gap", got.Must.OSGap, model.OSGap},
		{"must.proof-pending", got.Must.ProofPending, model.ProofPending},
		{"must.exclusion-unchecked", got.Must.ExclusionUnchecked, model.ExclusionUnchecked},
		{"must.not-assessed", got.Must.NotAssessed, model.NotAssessed},
		{"must.not-built", got.Must.NotBuilt, model.NotBuilt},
		{"the four open MUST sections",
			got.Must.ProofPending + got.Must.ExclusionUnchecked + got.Must.NotAssessed + got.Must.NotBuilt,
			model.Mandatory.Attainable - model.Mandatory.Proven},
		{"should.total", got.Should.Total, model.Optional.Total},
		{"should.proven", got.Should.Proven, model.Optional.Proven},
		{"should.os-verified", got.Should.OSVerified, model.OptionalOSVerified},
		{"should.os-gap", got.Should.OSGap, model.OptionalOSGap},
		{"should.proof-pending", got.Should.ProofPending, model.OptionalProofPending},
		{"should.exclusion-unchecked", got.Should.ExclusionUnchecked, model.OptionalExclusionUnchecked},
		{"should.not-assessed", got.Should.NotAssessed, model.OptionalNotAssessed},
		{"should.not-built", got.Should.NotBuilt, model.OptionalNotBuilt},
	} {
		if pair.got != pair.want {
			t.Errorf("%s is %d, the model states %d", pair.field, pair.got, pair.want)
		}
	}
	// The {intent} split is a private planning fact: the published file names
	// none of its keys.
	raw, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(index.RFCs[0].File)))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"not-ever"`, `"not-yet"`, `"not-decided"`} {
		if bytes.Contains(raw, []byte(key)) {
			t.Errorf("the published file carries the private intent key %s: %s", key, raw)
		}
	}
	if !slices.Equal(got.Unmet, []string{"RFC9999-2-1"}) {
		t.Fatalf("the gap row is not disclosed: %+v", got.Unmet)
	}
}

// TestNoPublishedRFCDataNamesAnIntent proves AC-7 and R-7 over every file the
// RFC data step and the requirement ledger publish, not over a list of known
// files: no JSON key and no string value is one of the three intent states,
// no key names an intent, and no string carries the marker or the not-ever
// reason, so a type that later embeds a row's intent cannot leak it unseen.
//
// Method: the walked one-RFC fixture of TestDataIndexAndFilesAgree, plus two
// open SHOULD rows marked {intent: not-yet} and {intent: not-ever; ...}. The
// model's private NotYet and NotEver counts are checked first, so the guard is
// proven to have had something to leak. publishRFCData and publishRFCLedger
// then write the artifact, every file under data/rfc/ and
// data/rfc-requirements.json is decoded and walked key by key, and each of the
// four kinds of file must have been read, so an empty artifact cannot pass.
func TestNoPublishedRFCDataNamesAnIntent(t *testing.T) {
	const notEverReason = "Ze keeps no discard counter"
	root, summary, marker := partialPublicationTree(t)
	summary = strings.ReplaceAll(summary, marker, "{gap: native sending absent}") +
		"- [ ] [RFC9999-2-2] [SHOULD] A speaker SHOULD log every widget it discards. (§2) {intent: not-yet}\n" +
		"- [ ] [RFC9999-2-3] [SHOULD] A speaker SHOULD count every widget it discards. (§2) " +
		`{intent: not-ever; "A speaker SHOULD count every widget it discards."; ` + notEverReason + "}\n"
	writePartialPublicationFile(t, root, "rfc/short/rfc9999.md", summary)
	writePartialPublicationFile(t, root, "rfc/full/rfc9999.txt", "Test RFC 9999\n\n2.  Widgets\n\n"+
		"A widget MUST be zero when sent and ignored on receipt.\n"+
		"A speaker SHOULD log every widget it discards.\nA speaker SHOULD count every widget it discards.\n")
	if err := rfc.FixtureProof(root, rfc.FixtureStem, map[string]string{"2:1": "RFC9999-2-1"}); err != nil {
		t.Fatal(err)
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
	model := figures[rfc.FixtureStem]
	if !model.Walked {
		t.Fatal("the signed-off stem is not walked, so its counts would not be published")
	}
	if model.NotYet != 1 || model.NotEver != 1 {
		t.Fatalf("the intent rows did not reach the figures (not yet %d, not ever %d, want 1 and 1), "+
			"so the guard below would have nothing to leak", model.NotYet, model.NotEver)
	}

	output := t.TempDir()
	paths := Paths{Repository: root, Output: output}
	if err := publishRFCData(paths); err != nil {
		t.Fatal(err)
	}
	if err := publishRFCLedger(paths); err != nil {
		t.Fatal(err)
	}
	published := []string{rfcLedgerFile}
	err = filepath.WalkDir(filepath.Join(output, filepath.FromSlash(rfcDataDir)), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		published = append(published, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, rel := range published {
		switch {
		case rel == rfcLedgerFile:
			kinds["ledger"]++
		case rel == rfcDataIndexFile:
			kinds["index"]++
		case strings.HasPrefix(rel, rfcDataProfilesDir+"/"):
			kinds["profile"]++
		default:
			kinds["stem"]++
		}
		body, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		var hits []string
		publishedIntentHits(value, "$", notEverReason, &hits)
		for _, hit := range hits {
			t.Errorf("%s publishes the private intent at %s", rel, hit)
		}
	}
	for _, kind := range []string{"index", "stem", "profile", "ledger"} {
		if kinds[kind] == 0 {
			t.Errorf("no %s file was published, so the guard read none: %v", kind, published)
		}
	}
}

// publishedIntentHits appends, by JSON path, every key and string value of
// value that names an intent: a key or value that is exactly one of the three
// intent states, a key containing "intent", and a string carrying the marker
// or the fixture's not-ever reason. Numbers, booleans and nulls name nothing.
// The recursion is bounded by the nesting of a file this package marshals.
func publishedIntentHits(value any, path, reason string, hits *[]string) {
	states := []string{"not-ever", "not-yet", "not-decided"}
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			at := path + "." + key
			if slices.Contains(states, key) || strings.Contains(strings.ToLower(key), "intent") {
				*hits = append(*hits, at+" (key)")
			}
			publishedIntentHits(child, at, reason, hits)
		}
	case []any:
		for index, child := range node {
			publishedIntentHits(child, path+"["+strconv.Itoa(index)+"]", reason, hits)
		}
	case string:
		if slices.Contains(states, node) || strings.Contains(node, "{intent") || strings.Contains(node, reason) {
			*hits = append(*hits, path+" = "+strconv.Quote(node))
		}
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
	stem, err := rfcStemDataOf("rfc9999", "Widgets", fixtureFigures())
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
	if err := refreshNativeSurfaces(t.Context(), paths); err != nil {
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

// rfcBarsGolden is the bar layout and one per-RFC file as this package writes
// them. internal/le/site/browsertest reads it as its index and checks its own
// per-RFC fixtures against the sample's keys, so the browser test draws the
// layout the build publishes rather than a copy of it.
const rfcBarsGolden = "testdata/rfc-bars.json"

// updateRFCBars rewrites rfcBarsGolden from the code instead of comparing.
var updateRFCBars = flag.Bool("update", false, "rewrite testdata/rfc-bars.json from rfcBars and a sample per-RFC file")

// rfcBarsGoldenContent is what rfcBarsGolden must hold: rfcBars and the
// per-RFC file of fixtureFigures, written as rfcDataOf writes one, with the
// requirement row count and who implements the summary filled in: the 20
// MUST and 13 SHOULD rows the figures count and two MAY rows they do not, and
// an enrolled summary Ze implements in Go with a public row.
func rfcBarsGoldenContent(t *testing.T) []byte {
	t.Helper()
	stem, err := rfcStemDataOf("rfc9999", "Widgets", fixtureFigures())
	if err != nil {
		t.Fatal(err)
	}
	stem.Requirements = 35
	rfcStemImplementer(&stem, &rfc.Meta{Enrolment: "enrolled", Implementation: "ze", Row: true})
	content, err := json.MarshalIndent(map[string]any{"bars": rfcBars, "stem": stem}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(content, '\n')
}

// TestTheBarLayoutGoldenIsCurrent proves the golden the browser test reads is
// the layout and the per-RFC file shape this package publishes.
//
// Method: marshal rfcBars and a sample per-RFC file and compare them with the
// golden byte for byte; with -update, write the golden instead.
func TestTheBarLayoutGoldenIsCurrent(t *testing.T) {
	want := rfcBarsGoldenContent(t)
	if *updateRFCBars {
		if err := os.WriteFile(rfcBarsGolden, want, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(rfcBarsGolden)
	if err != nil {
		t.Fatalf("read %s: %v. Write it with: ./le job run label unit-pkg command go test -run TestTheBarLayoutGoldenIsCurrent ./internal/le/site/ -args -update", rfcBarsGolden, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale: rfcBars or the per-RFC file changed, so the browser test draws an old layout.\n"+
			"Refresh it with: ./le job run label unit-pkg command go test -run TestTheBarLayoutGoldenIsCurrent ./internal/le/site/ -args -update\n"+
			" got: %s\nwant: %s", rfcBarsGolden, got, want)
	}
}
