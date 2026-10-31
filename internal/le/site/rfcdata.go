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

// rfcStemData is data/rfc/<stem>.json: one RFC's section counts, read from
// rfc.StemFiguresOf and never counted here.
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
	Implemented bool             `json:"implemented"`
	Must        *rfcMustCounts   `json:"must"`
	Should      *rfcShouldCounts `json:"should"`
	// ProvenByKind is rfc.StemFigures.MandatoryByKind: the Ze-proven MUST
	// rows by the strongest kind of test covering each one.
	ProvenByKind map[string]int `json:"proven-by-kind"`
	Unmet        []string       `json:"unmet"`
}

// rfcMustCounts splits every MUST row Ze or the stack under it owes.
//
// Total is every counted MUST row. Kept is the MUST bar: the rows the stack
// must keep for Ze to be RFC compliant, ZeProven + OSVerified + OSGap + ZeGap.
// NotOwed is the rest (not applicable, declined, OS blocked), split by
// annotation kind, so Kept + NotOwed is Total.
type rfcMustCounts struct {
	Total         int            `json:"total"`
	Kept          int            `json:"kept"`
	ZeProven      int            `json:"ze-proven"`
	OSVerified    int            `json:"os-verified"`
	OSGap         int            `json:"os-gap"`
	ZeGap         int            `json:"ze-gap"`
	NotOwed       int            `json:"not-owed"`
	NotOwedByKind map[string]int `json:"not-owed-by-kind"`
}

// rfcShouldCounts splits every SHOULD / MAY row: proven, performed by a layer
// under Ze (OS verified or OS gap, as a MUST row is), left out by an annotation
// that says it does not bind Ze (NotOwed, by kind), or open and declared not
// ever, not yet, or not at all (not decided). The seven add up to Total.
type rfcShouldCounts struct {
	Total         int            `json:"total"`
	Proven        int            `json:"proven"`
	OSVerified    int            `json:"os-verified"`
	OSGap         int            `json:"os-gap"`
	NotOwed       int            `json:"not-owed"`
	NotEver       int            `json:"not-ever"`
	NotYet        int            `json:"not-yet"`
	NotDecided    int            `json:"not-decided"`
	NotOwedByKind map[string]int `json:"not-owed-by-kind"`
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

// rfcStemDataOf splits one RFC's figures into the bar counts.
//
// Every count is a field of rfc.StemFigures or a difference of two of them,
// and the split is checked against the model's own totals: figures that do
// not add back are refused by stem rather than drawn as a bar whose segments
// disagree with its total.
func rfcStemDataOf(stem, title string, figures rfc.StemFigures) (rfcStemData, error) {
	out := rfcStemData{Stem: stem, Title: title, Walked: figures.Walked, Unmet: figures.Unmet}
	if out.Unmet == nil {
		out.Unmet = []string{}
	}
	if !figures.Walked {
		return out, nil
	}
	must, err := rfcMustCountsOf(stem, &figures)
	if err != nil {
		return rfcStemData{}, err
	}
	should, err := rfcShouldCountsOf(stem, &figures)
	if err != nil {
		return rfcStemData{}, err
	}
	out.Must, out.Should = must, should
	out.ProvenByKind = figures.MandatoryByKind
	if out.ProvenByKind == nil {
		out.ProvenByKind = map[string]int{}
	}
	return out, nil
}

// rfcMustCountsOf splits the MUST figure. A row a layer under Ze performs
// (traits OSMet) is OS verified or an OS gap, never "not owed"; every other
// exclusion kind is not owed.
func rfcMustCountsOf(stem string, figures *rfc.StemFigures) (*rfcMustCounts, error) {
	mandatory := &figures.Mandatory
	osByKind, notOwed, err := splitExcluded(stem, mandatory.Excluded)
	if err != nil {
		return nil, err
	}
	osRows := 0
	for _, count := range osByKind {
		osRows += count
	}
	if osRows != figures.OSVerified+figures.OSGap {
		return nil, figuresRefusal(stem, "the OS verified and OS gap MUST rows (", figures.OSVerified+figures.OSGap,
			") differ from the MUST rows a lower layer performs (", osRows)
	}
	if mandatory.Proven > mandatory.Attainable {
		return nil, figuresRefusal(stem, "the Ze-proven MUST rows (", mandatory.Proven,
			") exceed the MUST rows Ze's code owes (", mandatory.Attainable)
	}
	counts := &rfcMustCounts{
		Total: mandatory.Total, ZeProven: mandatory.Proven, OSVerified: figures.OSVerified, OSGap: figures.OSGap,
		ZeGap: mandatory.Attainable - mandatory.Proven, NotOwedByKind: notOwed,
	}
	for _, count := range notOwed {
		counts.NotOwed += count
	}
	counts.Kept = counts.ZeProven + counts.OSVerified + counts.OSGap + counts.ZeGap
	if counts.Kept+counts.NotOwed != counts.Total {
		return nil, figuresRefusal(stem, "the MUST sections (", counts.Kept+counts.NotOwed,
			") differ from the MUST rows counted (", counts.Total)
	}
	return counts, nil
}

// rfcShouldCountsOf splits the SHOULD / MAY figure as rfcMustCountsOf splits
// the MUST one: a row a layer under Ze performs (traits OSMet) is OS verified
// or an OS gap, never "not owed" (owner, 2026-10-31); every other exclusion
// kind is not owed.
func rfcShouldCountsOf(stem string, figures *rfc.StemFigures) (*rfcShouldCounts, error) {
	optional := &figures.Optional
	osByKind, notOwed, err := splitExcluded(stem, optional.Excluded)
	if err != nil {
		return nil, err
	}
	osRows := 0
	for _, count := range osByKind {
		osRows += count
	}
	if osRows != figures.OptionalOSVerified+figures.OptionalOSGap {
		return nil, figuresRefusal(stem, "the OS verified and OS gap SHOULD rows (", figures.OptionalOSVerified+figures.OptionalOSGap,
			") differ from the SHOULD rows a lower layer performs (", osRows)
	}
	counts := &rfcShouldCounts{Total: optional.Total, Proven: optional.Proven, OSVerified: figures.OptionalOSVerified,
		OSGap: figures.OptionalOSGap, NotEver: figures.NotEver, NotYet: figures.NotYet, NotDecided: figures.NotDecided,
		NotOwedByKind: notOwed}
	for _, count := range notOwed {
		counts.NotOwed += count
	}
	open := optional.Attainable - optional.Proven
	if open != counts.NotEver+counts.NotYet+counts.NotDecided {
		return nil, figuresRefusal(stem, "the not ever, not yet and not decided SHOULD rows (",
			counts.NotEver+counts.NotYet+counts.NotDecided, ") differ from the open SHOULD rows (", open)
	}
	if counts.Proven+counts.OSVerified+counts.OSGap+counts.NotOwed+open != counts.Total {
		return nil, figuresRefusal(stem, "the SHOULD sections (", counts.Proven+counts.OSVerified+counts.OSGap+counts.NotOwed+open,
			") differ from the SHOULD rows counted (", counts.Total)
	}
	return counts, nil
}

// splitExcluded answers the excluded rows a lower layer performs (traits
// OSMet) and the rest, each by annotation kind, so a second OSMet kind is
// counted under its own name. A kind internal/le/rfc does not declare is
// refused: its meaning is unknown, so no section can hold it.
func splitExcluded(stem string, excluded map[string]int) (osRows, notOwed map[string]int, err error) {
	osRows, notOwed = map[string]int{}, map[string]int{}
	for kind, count := range excluded {
		traits, known := rfc.AnnotationTraitsOf(kind)
		if !known {
			var tb textbuf.Buffer
			return nil, nil, errors.New(tb.Str(stem).Str(": the figures exclude rows of annotation kind `").Str(kind).
				Str("`, which internal/le/rfc does not declare, so no section of the RFC data can hold them. ").
				Str("Declare the kind in annotationKinds (internal/le/rfc/rfc.go) or fix the summary, then rerun `./le site build`").String())
		}
		if traits.OSMet {
			osRows[kind] += count
			continue
		}
		notOwed[kind] += count
	}
	return osRows, notOwed, nil
}

// figuresRefusal names a stem whose figures do not add back to their totals.
// It is a defect in rfc.StemFiguresOf, never in the summary, so the fix it
// names is there.
func figuresRefusal(stem, first string, got int, second string, want int) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(stem).Str(": ").Str(first).Int(int64(got)).Str(second).Int(int64(want)).
		Str("), so the RFC data would publish a bar whose segments disagree with its total. ").
		Str("This is a defect in rfc.StemFiguresOf (internal/le/rfc/profilecoverage.go): compare `./le rfc tree` for the stem, ").
		Str("fix the partition, then rerun `./le site build`").String())
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
