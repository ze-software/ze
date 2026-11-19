// Design: docs/contributing/rfc-conformance-gates.md -- selected discrimination reports
// Related: discriminate.go -- the status reader and the unchanged record verifier
// Related: discriminate_test.go -- fixture records sealed by the production minter

package rfc

import (
	"strings"
	"testing"
)

// A requirement report must publish only its selected proof and tag status.
// Stale records with ambiguous tagged units in the same or another stem must
// stay out of that output.
// This checks output selection, not which sources the verifier reads.
func TestDiscriminationStatusIDOutputExcludesOtherIDs(t *testing.T) {
	for _, otherRID := range []string{selftestRIDDrop, selftestRIDUpdater} {
		t.Run(otherRID, func(t *testing.T) {
			files := discriminationScopeFixture(t, otherRID)
			status, err := discriminationStatusOf(discriminationTree(t, files), selftestRIDSend, "", func(rid string) bool {
				return rid == selftestRIDSend
			})
			if err != nil {
				t.Fatalf("an unrelated stale record blocked the selected requirement: %v", err)
			}
			if len(status.Records) != 1 {
				t.Fatalf("selected requirement has %d proofs, want its one valid proof", len(status.Records))
			}
			if status.Records[0].RID != selftestRIDSend {
				t.Errorf("reported proof belongs to %s, want %s", status.Records[0].RID, selftestRIDSend)
			}
			if len(status.Stale) != 0 {
				t.Errorf("unrelated records appeared as selected stale proofs: %+v", status.Stale)
			}
			if len(status.Unproven) != 0 {
				t.Errorf("the verified selected tag was reported unproven: %+v", status.Unproven)
			}
		})
	}
}

// A stem report must publish its own valid proof and exclude another stem's
// stale record. This checks output selection, not source-read isolation.
func TestDiscriminationStatusStemOutputExcludesOtherStems(t *testing.T) {
	files := discriminationScopeFixture(t, selftestRIDUpdater)
	status, err := discriminationStatusOf(discriminationTree(t, files), selftestStem, "", func(rid string) bool {
		return hasRIDStem(rid, selftestStem)
	})
	if err != nil {
		t.Fatalf("another stem's stale record blocked the selected stem: %v", err)
	}
	if len(status.Records) != 1 {
		t.Fatalf("selected stem has %d proofs, want its one valid proof", len(status.Records))
	}
	if status.Records[0].RID != selftestRIDSend {
		t.Errorf("reported proof belongs to %s, want %s", status.Records[0].RID, selftestRIDSend)
	}
	if len(status.Stale) != 0 {
		t.Errorf("another stem's record appeared as a selected stale proof: %+v", status.Stale)
	}
}

// An ambiguous selected tagged unit resolves as absent, so its record becomes
// stale and its positive tag becomes unproven. Both selectors must retain the
// fixture's existing unproven negative tag, and the stem retains its valid proof.
func TestDiscriminationStatusSelectedAmbiguityIsStale(t *testing.T) {
	files := discriminationScopeFixture(t, selftestRIDDrop)
	root := discriminationTree(t, files)
	cases := []struct {
		name     string
		selector string
		selected func(string) bool
		proofs   int
	}{
		{"id", selftestRIDDrop, func(rid string) bool { return rid == selftestRIDDrop }, 0},
		{"stem", selftestStem, func(rid string) bool { return hasRIDStem(rid, selftestStem) }, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := discriminationStatusOf(root, tc.selector, "", tc.selected)
			if err != nil {
				t.Fatalf("read the selected ambiguous unit's status: %v", err)
			}
			if len(status.Records) != tc.proofs {
				t.Fatalf("verified proofs = %d, want %d", len(status.Records), tc.proofs)
			}
			for index := range status.Records {
				if status.Records[index].RID != selftestRIDSend {
					t.Errorf("an ambiguous unit published a verified proof: %+v", status.Records[index])
				}
			}
			if len(status.Stale) != 1 {
				t.Fatalf("stale proofs = %d, want the one whose unit is ambiguous", len(status.Stale))
			}
			want := Cover{RID: selftestRIDDrop, Polarity: PolarityPositive,
				Unit: discriminationScopeTestPath + "::TestOtherWidget"}
			if status.Stale[0].Cover() != want {
				t.Errorf("stale proof names %+v, want %+v", status.Stale[0].Cover(), want)
			}
			if len(status.Unproven) != 2 {
				t.Fatalf("unproven tags = %d, want the selected positive and negative tags", len(status.Unproven))
			}
			unproven := map[string]string{
				PolarityPositive: discriminationScopeTestPath,
				PolarityNegative: selftestCIPath,
			}
			for _, tag := range status.Unproven {
				if tag.RID != selftestRIDDrop {
					t.Errorf("unproven tag belongs to %s, want %s", tag.RID, selftestRIDDrop)
				}
				if tag.File != unproven[tag.Polarity] {
					t.Errorf("unproven %s tag belongs to %s, want %s", tag.Polarity, tag.File, unproven[tag.Polarity])
				}
				delete(unproven, tag.Polarity)
			}
		})
	}
}

// A selected proof whose unit or claim moved must remain stale and return its
// tag to unproven. Scoping must never bypass fingerprint checks. A changed
// producer is the case just outside: the proof belongs to the test
// (ai/META-DECISIONS.md, "Test changes"), so it stays verified and its tag proven.
func TestDiscriminationStatusSelectedStaleProofUnproven(t *testing.T) {
	t.Run("producer stands", func(t *testing.T) {
		files, proof := discriminationFixture(t)
		putDiscrimination(t, files, proof)
		changed := strings.Replace(files[selftestProducerPath], "return count", "return 0", 1)
		if changed == files[selftestProducerPath] {
			t.Fatal("the producer rewrite changed nothing, so the case tests nothing")
		}
		files[selftestProducerPath] = changed
		status, err := discriminationStatusOf(discriminationTree(t, files), selftestRIDSend, "", func(rid string) bool {
			return rid == selftestRIDSend
		})
		if err != nil {
			t.Fatalf("read the selected proof beside a changed producer: %v", err)
		}
		if len(status.Records) != 1 {
			t.Fatalf("verified proofs = %d, want 1: the tagged unit and its claim are unchanged", len(status.Records))
		}
		if status.Records[0].Cover() != proof.Cover() {
			t.Errorf("verified proof names %+v, want %+v", status.Records[0].Cover(), proof.Cover())
		}
		if len(status.Stale) != 0 {
			t.Errorf("a changed producer staled the proof: %+v", status.Stale)
		}
		if len(status.Unproven) != 0 {
			t.Errorf("a changed producer returned the tag to unproven: %+v", status.Unproven)
		}
	})
	cases := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{"unit", selftestTestPath, "!= 1", "== 1"},
		{"claim", selftestTestPath, "SendWidget answers the count", "SendWidget preserves the count"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, proof := discriminationFixture(t)
			putDiscrimination(t, files, proof)
			files[tc.path] = strings.Replace(files[tc.path], tc.old, tc.new, 1)
			status, err := discriminationStatusOf(discriminationTree(t, files), selftestRIDSend, "", func(rid string) bool {
				return rid == selftestRIDSend
			})
			if err != nil {
				t.Fatalf("read the selected stale proof: %v", err)
			}
			if len(status.Records) != 0 {
				t.Errorf("a changed %s still published a verified proof: %+v", tc.name, status.Records)
			}
			if len(status.Stale) != 1 {
				t.Fatalf("stale proofs = %d, want 1", len(status.Stale))
			}
			if status.Stale[0].Cover() != proof.Cover() {
				t.Errorf("stale proof names %+v, want %+v", status.Stale[0].Cover(), proof.Cover())
			}
			if len(status.Unproven) != 1 {
				t.Fatalf("unproven tags = %d, want the selected tag", len(status.Unproven))
			}
			if status.Unproven[0].RID != selftestRIDSend {
				t.Errorf("unproven tag belongs to %s, want %s", status.Unproven[0].RID, selftestRIDSend)
			}
		})
	}
}

// Artifact shape remains validated across the corpus. An unknown JSON field
// must refuse even when its record belongs to an unselected requirement.
func TestDiscriminationStatusMalformedRecordRefused(t *testing.T) {
	for _, rid := range []string{selftestRIDSend, selftestRIDUpdater} {
		t.Run(rid, func(t *testing.T) {
			files, proof := discriminationFixture(t)
			putDiscrimination(t, files, proof)
			malformed := strings.Replace(files[discriminationRecordRel(selftestStem, selftestRIDSend)],
				"{", "{\"unexpected\":true,", 1)
			malformed = strings.Replace(malformed, selftestRIDSend, rid, 1)
			stem := selftestStem
			if rid == selftestRIDUpdater {
				stem = selftestStemUpdater
			}
			files[discriminationRecordRel(stem, rid)] = malformed
			_, err := discriminationStatusOf(discriminationTree(t, files), selftestRIDSend, "", func(one string) bool {
				return one == selftestRIDSend
			})
			if err == nil {
				t.Fatal("a record with an unknown JSON field was accepted")
			}
			if !strings.Contains(err.Error(), "unknown field \"unexpected\"") {
				t.Errorf("the refusal did not identify the malformed artifact: %v", err)
			}
		})
	}
}

const (
	discriminationScopeProducerPath = "internal/sample/scope.go"
	discriminationScopeTestPath     = "internal/sample/scope_test.go"
)

// discriminationScopeFixture seals both records while their tagged units
// resolve, then adds a second method of the tagged unit's name on another
// receiver. The stored bare TestOtherWidget unit key becomes ambiguous, so the
// other record's unit no longer resolves, without invalidating the JSON or the
// selected SendWidget proof. The ambiguity is on the UNIT key because only the
// unit and its claim stale a proof: an ambiguous producer leaves it standing
// (ai/META-DECISIONS.md, "Test changes").
func discriminationScopeFixture(t *testing.T, otherRID string) map[string]string {
	t.Helper()
	files, proof := discriminationFixture(t)
	files[discriminationScopeTestPath] = "package sample\n\n" +
		"type ScopeSuite struct{}\n\n" +
		"// RFC requirement: " + otherRID + " positive -- Count returns one widget.\n" +
		"func (ScopeSuite) TestOtherWidget() {\n\tif (ScopeSender{}).Count() != 1 {\n" +
		"\t\tpanic(\"BUG: the widget was not counted\")\n\t}\n}\n"
	files[discriminationScopeProducerPath] = "package sample\n\n" +
		"type ScopeSender struct{}\n\nfunc (ScopeSender) Count() int {\n\treturn 1\n}\n"
	other := sealFixture(t, files, &DiscriminationRecord{
		RID: otherRID, Polarity: PolarityPositive,
		Unit: discriminationScopeTestPath + "::TestOtherWidget", Route: RouteRevert,
		Producer: discriminationScopeProducerPath + "::Count",
		Break:    "body of Count replaced by a halt",
	})
	putDiscrimination(t, files, proof, other)
	files[discriminationScopeTestPath] += "\ntype ScopeShadow struct{}\n\n" +
		"func (ScopeShadow) TestOtherWidget() {}\n"
	return files
}
