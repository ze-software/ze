// Design: docs/architecture/site-facts.md -- RFC completion data: one file per RFC, read in the browser
// Detail: website/assets/js/rfc-figures.js fills every data-ze-rfc element and draws both bars from these files.
// Related: build.go publishes these files in refreshNativeSurfaces, before any producer runs.
package site

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// The RFC completion data lives under one directory the build owns whole:
// one file per RFC, one per profile, and the index naming every one of them.
const (
	rfcDataDir         = "data/rfc"
	rfcDataProfilesDir = rfcDataDir + "/profiles"
	rfcDataIndexFile   = rfcDataDir + "/index.json"
)

// rfcStemData is data/rfc/<stem>.json: one RFC's section counts, split by
// rfc.SectionsOf from rfc.StemFiguresOf and never counted here.
//
// Must and Should are null for an RFC no walk bounded (Walked false): its
// requirement list is a claim nobody checked against the RFC text, so it
// publishes no figure (docs/contributing/rfc-conformance-gates.md, "Coverage
// figures"). Unmet is published either way, because it is the summary's own
// disclosure rather than a measurement.
type rfcStemData struct {
	Stem   string `json:"stem"`
	Title  string `json:"title,omitempty"`
	Walked bool   `json:"walked"`
	// Requirements is every requirement row the summary declares, at every
	// level and whether or not a walk bounded it: the size of the list, never
	// a measure of Ze. It is the one count an unwalked RFC publishes.
	Requirements int `json:"requirements"`
	// Implementer is the summary's Implementation kind (ze, mixed,
	// third-party or foundation): who implements the document.
	Implementer string `json:"implementer"`
	// Implemented is rfc.Implements over the summary, the ONE definition of
	// an RFC Ze implements, so a page counts that population by summing this
	// flag rather than restating the predicate.
	Implemented bool                `json:"implemented"`
	Must        *rfc.MustSections   `json:"must"`
	Should      *rfc.ShouldSections `json:"should"`
	// ProvenByKind is rfc.StemFigures.MandatoryByKind: the Ze-proven MUST
	// rows by the strongest kind of test covering each one.
	ProvenByKind map[string]int `json:"proven-by-kind"`
	Unmet        []string       `json:"unmet"`
}

// rfcBarSegment is one segment of a bar: the class name its color hangs on
// (`rfc-seg-<Segment>`), the words it carries beside its count, and the
// dotted fields of a per-RFC file whose sum is its count.
type rfcBarSegment struct {
	Segment string   `json:"segment"`
	Label   string   `json:"label"`
	Fields  []string `json:"fields"`
}

// rfcBarLayout is both bars, left to right. Total names the fields whose sum
// the segments must reach, which the renderer checks before it draws.
type rfcBarLayout struct {
	Must          []rfcBarSegment `json:"must"`
	MustTotal     []string        `json:"must-total"`
	Overview      []rfcBarSegment `json:"overview"`
	OverviewTotal []string        `json:"overview-total"`
}

// rfcMustTotalField is the per-RFC field that counts every MUST row, the
// kept ones and the ones not owed: the overview bar's total and the MUST row
// count the index and its mirror print.
const rfcMustTotalField = "must.total"

// rfcBars is the ONE declaration of both bars' segments, their order and
// their words (spec-rfc-figures-sectioned-presentation, AC-12). It is
// published in the index, so the browser draws what this says and holds no
// list of its own.
var rfcBars = rfcBarLayout{
	Must: []rfcBarSegment{
		{"ze-proven", "Ze proven", []string{"must.ze-proven"}},
		{"os-verified", "OS verified", []string{"must.os-verified"}},
		{"os-gap", "OS testing gap", []string{"must.os-gap"}},
		{"ze-gap", "Ze correctness gap", []string{"must.ze-gap"}},
	},
	MustTotal: []string{"must.kept"},
	Overview: []rfcBarSegment{
		{"ze-proven", "Ze MUST proven", []string{"must.ze-proven"}},
		{"os-verified", "OS verified", []string{"must.os-verified", "should.os-verified"}},
		{"should-proven", "SHOULD proven", []string{"should.proven"}},
		{"not-owed", "Not owed", []string{"must.not-owed", "should.not-owed"}},
		{"not-ever", "SHOULD not ever", []string{"should.not-ever"}},
		{"not-yet", "SHOULD not yet", []string{"should.not-yet"}},
		{"not-decided", "SHOULD not decided", []string{"should.not-decided"}},
		{"os-gap", "OS testing gap", []string{"must.os-gap", "should.os-gap"}},
		{"ze-gap", "Ze correctness gap", []string{"must.ze-gap"}},
	},
	OverviewTotal: []string{rfcMustTotalField, "should.total"},
}

// rfcProfileData is data/rfc/profiles/<name>.json. Required and Optional are
// the two tiers, never merged: a use case's optional members are listed and
// never counted in its required figures, and an aggregate's optional tier
// (`| Aggregate tier | optional |`) holds members that leave its main table.
type rfcProfileData struct {
	Name string `json:"name"`
	// Kind is rfc.ProfileKindUseCase or rfc.ProfileKindTechnology, the word
	// the profile file declares.
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	Release  bool     `json:"release"`
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

// rfcDataIndex is data/rfc/index.json: the bar layout, then every RFC and
// every profile with the file that holds it.
type rfcDataIndex struct {
	Bars     rfcBarLayout      `json:"bars"`
	RFCs     []rfcIndexRFC     `json:"rfcs"`
	Profiles []rfcIndexProfile `json:"profiles"`
}

// rfcIndexRFC is one summary in the index.
type rfcIndexRFC struct {
	Stem      string `json:"stem"`
	File      string `json:"file"`
	Title     string `json:"title,omitempty"`
	Walked    bool   `json:"walked"`
	Enrolment string `json:"enrolment"`
	// Gated is rfc.Meta.Enrolled: the enrolment row says enrolled AND the
	// Implementation kind counts against Ze's Go. The enrolled subject reads
	// it, so a third-party or foundation summary marked enrolled, which the
	// gate never holds Ze to, is never summed as enrolled.
	Gated bool `json:"gated"`
}

// rfcIndexProfile is one profile in the index.
type rfcIndexProfile struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

// rfcData is everything one build publishes under data/rfc/.
type rfcData struct {
	index    rfcDataIndex
	stems    []rfcStemData
	profiles []rfcProfileData
}

// liveRFCData answers the RFC completion data of one checkout. It is a
// variable so a build test over a synthetic tree can state it, the way
// liveRequirementLedger is stated.
var liveRFCData = collectRFCData

// publishRFCData writes data/rfc/ into the artifact: every file the index
// names and nothing else.
//
// It runs in refreshNativeSurfaces rather than as a producer, for the reason
// publishRFCLedger does: the RFC pages read these files back.
func publishRFCData(paths Paths) error {
	data, err := liveRFCData(paths.Repository)
	if err != nil {
		return err
	}
	return writeRFCData(paths.Output, &data)
}

// collectRFCData reads one checkout's corpus through the production readers
// and answers its data.
func collectRFCData(tree string) (rfcData, error) {
	collected, err := rfc.Collect(tree)
	if err != nil {
		return rfcData{}, err
	}
	input, err := rfc.NewRenderInput(tree, collected, nil, nil)
	if err != nil {
		return rfcData{}, err
	}
	return rfcDataOf(&input)
}

// rfcDataOf derives every file from rfc.StemFiguresOf and the corpus's own
// Meta and profile declarations.
func rfcDataOf(in *rfc.RenderInput) (rfcData, error) {
	figures, err := rfc.StemFiguresOf(in)
	if err != nil {
		return rfcData{}, err
	}
	stems := make([]string, 0, len(figures))
	for stem := range figures {
		stems = append(stems, stem)
	}
	slices.Sort(stems)
	requirements, err := rfcRequirementCounts(in.Requirements, figures)
	if err != nil {
		return rfcData{}, err
	}
	data := rfcData{index: rfcDataIndex{Bars: rfcBars}}
	for _, stem := range stems {
		meta := in.Metas[stem]
		stemData, err := rfcStemDataOf(stem, meta.Title, figures[stem])
		if err != nil {
			return rfcData{}, err
		}
		stemData.Requirements = requirements[stem]
		rfcStemImplementer(&stemData, &meta)
		data.stems = append(data.stems, stemData)
		data.index.RFCs = append(data.index.RFCs, rfcIndexRFC{Stem: stem, File: rfcStemDataFile(stem),
			Title: meta.Title, Walked: stemData.Walked, Enrolment: meta.Enrolment, Gated: meta.Enrolled()})
	}
	data.profiles, err = rfcProfileDataOf(in.Profiles, in.Metas)
	if err != nil {
		return rfcData{}, err
	}
	for index := range data.profiles {
		profile := &data.profiles[index]
		data.index.Profiles = append(data.index.Profiles, rfcIndexProfile{Name: profile.Name,
			File: rfcProfileDataFile(profile.Name), Kind: profile.Kind, Title: profile.Title})
	}
	return data, nil
}

// rfcStemImplementer writes who implements the summary into its file: the
// declared Implementation kind, and rfc.Implements, read rather than restated,
// so the pages count the RFCs Ze implements as the retired proof share did.
func rfcStemImplementer(stemData *rfcStemData, meta *rfc.Meta) {
	stemData.Implementer = meta.Implementation
	stemData.Implemented = rfc.Implements(meta)
}

// rfcRequirementCounts counts the requirement rows each summary declares. A
// row whose RFC has no figures is refused by id: it would be counted in no
// file, so the corpus total the pages sum would silently leave it out.
func rfcRequirementCounts(requirements []rfc.Requirement, figures map[string]rfc.StemFigures) (map[string]int, error) {
	out := make(map[string]int, len(figures))
	for index := range requirements {
		requirement := &requirements[index]
		if _, held := figures[requirement.RFC]; !held {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str(requirement.RID).Str(" names the RFC ").Str(requirement.RFC).
				Str(", which rfc.StemFiguresOf answers no figures for, so data/rfc/ would publish no file counting it; ").
				Str("check the summary's file name under rfc/short/, then ").Str(rfcPublishMissing).String())
		}
		out[requirement.RFC]++
	}
	return out, nil
}

// rfcStemDataFile is the published path of one RFC's file.
func rfcStemDataFile(stem string) string { return rfcDataDir + "/" + stem + ".json" }

// rfcProfileDataFile is the published path of one profile's file.
func rfcProfileDataFile(name string) string { return rfcDataProfilesDir + "/" + name + ".json" }

// rfcStemDataOf publishes one RFC's figures as the bar counts rfc.SectionsOf
// splits them into. Figures that do not add back are refused by stem there,
// rather than drawn as a bar whose segments disagree with its total.
func rfcStemDataOf(stem, title string, figures rfc.StemFigures) (rfcStemData, error) {
	out := rfcStemData{Stem: stem, Title: title, Walked: figures.Walked, Unmet: figures.Unmet}
	if out.Unmet == nil {
		out.Unmet = []string{}
	}
	if !figures.Walked {
		return out, nil
	}
	sections, err := rfc.SectionsOf(stem, &figures)
	if err != nil {
		return rfcStemData{}, err
	}
	out.Must, out.Should = &sections.Must, &sections.Should
	out.ProvenByKind = figures.MandatoryByKind
	if out.ProvenByKind == nil {
		out.ProvenByKind = map[string]int{}
	}
	return out, nil
}

// rfcProfileDataOf answers one file per profile, use cases first and then
// aggregates, each sorted by name. Membership is rfc.ProfileTiersOf, which
// reads what each summary's Meta declares (Use cases, Optional use cases,
// Support or Aggregate, Aggregate tier): the membership `./le rfc tree` reads.
func rfcProfileDataOf(profiles rfc.Profiles, metas map[string]rfc.Meta) ([]rfcProfileData, error) {
	names := sortedKeys(profiles.UseCases)
	names = append(names, sortedKeys(profiles.Aggregates)...)
	out := make([]rfcProfileData, 0, len(names))
	for _, name := range names {
		tiers, err := rfc.ProfileTiersOf(profiles, metas, name)
		if err != nil {
			return nil, err
		}
		out = append(out, rfcProfileData{Name: tiers.Name, Kind: tiers.Kind, Title: tiers.Title,
			Release: tiers.Release, Required: tiers.Required, Optional: tiers.Optional})
	}
	return out, nil
}

// sortedKeys answers a map's keys in order.
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// writeRFCData replaces data/rfc/ in the artifact. The output was seeded from
// the previous artifact, so the directory is removed first: a file for a stem
// or profile that no longer exists would otherwise stay published beside an
// index that no longer names it (spec R-6).
func writeRFCData(output string, data *rfcData) error {
	if err := os.RemoveAll(filepath.Join(output, filepath.FromSlash(rfcDataDir))); err != nil {
		return err
	}
	for index := range data.stems {
		if err := writeRFCDataFile(output, rfcStemDataFile(data.stems[index].Stem), &data.stems[index]); err != nil {
			return err
		}
	}
	for index := range data.profiles {
		if err := writeRFCDataFile(output, rfcProfileDataFile(data.profiles[index].Name), &data.profiles[index]); err != nil {
			return err
		}
	}
	return writeRFCDataFile(output, rfcDataIndexFile, &data.index)
}

// writeRFCDataFile writes one value as indented JSON.
func writeRFCDataFile(output, name string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("encode ").Str(name).Str(": ").Str(err.Error()).String())
	}
	return writeNamedArtifact(output, name, string(content)+"\n")
}
