package rfc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureProofUnsealed is the pair of records the partial fixture's tagged
// .ci carries, before sealing.
func fixtureProofUnsealed() []DiscriminationRecord {
	var records []DiscriminationRecord
	for _, polarity := range []string{PolarityPositive, PolarityNegative} {
		records = append(records, DiscriminationRecord{RID: selftestRIDSend, Polarity: polarity,
			Unit: selftestCIPath, Route: RouteRevert, Producer: stampProducerKey,
			Break: selftestBreak, Citation: "expect=stdout:contains=OK"})
	}
	return records
}

// fixtureProofFigure answers the figure the real pipeline derives for the
// selftest stem in the tree at root.
func fixtureProofFigure(t *testing.T, root string) StemFigures {
	t.Helper()
	collected, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	in, err := NewRenderInput(root, collected, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	figures, err := StemFiguresOf(&in)
	if err != nil {
		t.Fatal(err)
	}
	return figures[selftestStem]
}

// VALIDATES: FixtureProof leaves a stem walked and its records verified, as
// the production figure reads them, so another package's test can build a
// proven stem without spelling a sha.
// METHOD: build the partial fixture, derive its figure before and after
// FixtureProof, then drop the partial marker so the pair earns whole credit.
// PREVENTS: a helper whose sign-off or records count nothing, which would let
// the tests built on it pass for the wrong reason.
func TestFixtureProofWalksTheStemAndVerifiesItsRecords(t *testing.T) {
	files := partialFiles()
	files[selftestWorkflowRel] = selftestWorkflow
	files[FixtureAggregateRel] = FixtureAggregateProfile
	root := t.TempDir()
	writeFixtureFiles(t, root, files)
	if before := fixtureProofFigure(t, root); before.Walked {
		t.Fatalf("the fixture was walked before any sign-off: %+v", before)
	}
	if err := FixtureProof(root, selftestStem, map[string]string{"2:1": selftestRIDSend}, fixtureProofUnsealed()...); err != nil {
		t.Fatal(err)
	}
	partial := fixtureProofFigure(t, root)
	if !partial.Walked || partial.Mandatory.Total != 1 || partial.Mandatory.Proven != 0 {
		t.Fatalf("a partial row was credited, or the stem is unwalked: %+v", partial)
	}
	writeFixtureFiles(t, root, map[string]string{
		selftestSummaryRel: strings.Replace(files[selftestSummaryRel], " "+partialMarker, "", 1),
	})
	whole := fixtureProofFigure(t, root)
	if !whole.Walked || whole.Mandatory.Total != 1 || whole.Mandatory.Proven != 1 {
		t.Fatalf("verified records on a walked stem proved nothing: %+v", whole)
	}
}

// VALIDATES: FixtureProof refuses, rather than writes, a sign-off the
// evaluator rejects or a record that does not resolve.
// METHOD: map the site to a requirement the summary lacks; name a unit the
// tree does not hold.
// PREVENTS: a fixture that silently counts nothing.
func TestFixtureProofRefusesWhatWouldCountNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		mapped  map[string]string
		records []DiscriminationRecord
		want    string
	}{
		"unknown requirement": {map[string]string{"2:1": "RFC9999-9-9"}, nil, "sign-off for " + selftestStem},
		"absent unit": {map[string]string{"2:1": selftestRIDSend}, []DiscriminationRecord{{
			RID: selftestRIDSend, Polarity: PolarityPositive, Unit: "test/plugin/absent.ci",
			Route: RouteRevert, Producer: stampProducerKey, Break: selftestBreak,
		}}, "absent.ci"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFixtureFiles(t, root, partialFiles())
			err := FixtureProof(root, selftestStem, tc.mapped, tc.records...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want a refusal naming %q, got %v", tc.want, err)
			}
		})
	}
}

// VALIDATES: FixtureProof refuses any stem but FixtureStem, and writes nothing
// for it.
// METHOD: hand it a real enrolled stem over a fixture tree; the refusal must
// name the stem and the command that earns a real proof, and the tree must
// hold no sign-off or record afterwards.
// PREVENTS: an exported helper that seals records no red was observed for,
// aimed at a stem the published ledger counts.
func TestFixtureProofRefusesARealStem(t *testing.T) {
	root := t.TempDir()
	writeFixtureFiles(t, root, partialFiles())
	err := FixtureProof(root, "rfc4271", map[string]string{"2:1": selftestRIDSend}, fixtureProofUnsealed()...)
	if err == nil {
		t.Fatal("FixtureProof accepted rfc4271, a stem other than the synthetic fixture")
	}
	for _, want := range []string{"rfc4271", FixtureStem, "./le rfc record"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not name %q", err, want)
		}
	}
	for _, rel := range []string{extractionRel, discriminationRel} {
		entries, readErr := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(entries) > 0 {
			t.Fatalf("a refused stem still wrote %d file(s) under %s", len(entries), rel)
		}
	}
}
