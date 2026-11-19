// VALIDATES: `./le commit create` refuses a commit that stales an RFC proof
// record (it changes the tagged unit or its claim, or deletes the record while
// its tag stays) unless it carries the re-taken record, and names each stale
// record, why, and the `./le rfc record` line that re-takes it. A commit that
// changes only the producer a record broke stales nothing, because the proof
// belongs to the test (ai/META-DECISIONS.md, "Test changes").
// METHOD: the selftest's sealed record over its two sources is the parent; each
// test edits the tree the commit leaves and judges the change through the
// registered Judge, with Before, After and BeforeUnder read from maps.
// PREVENTS: a commit that changes a tagged test or its claim without re-taking
// its record, which leaves a published proof nobody observed, a gate that bills
// a commit for a record the parent already held stale, and one that bills a
// producer refactor for a proof the unchanged test still carries.
package rfc

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// recordedParent answers the parent tree of a commit: the selftest's sources
// and one record sealed against them, rendered into its record file.
func recordedParent(t *testing.T, unsealed *DiscriminationRecord) (map[string]string, DiscriminationRecord) {
	t.Helper()
	files := selftestDiscriminationSources()
	record := sealFixture(t, files, unsealed)
	putDiscrimination(t, files, record)
	return files, record
}

// mutantRecord and revertRecord are the two proof routes over the selftest's
// tagged unit and producer.
func mutantRecord() *DiscriminationRecord {
	return &DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteMutant, Producer: selftestProducerUnit,
		Break: selftestBreak}
}

func revertRecord() *DiscriminationRecord {
	return &DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteRevert, Producer: selftestProducerUnit,
		Break: revertBody}
}

// mapSide reads one side of a fixture commit from its files.
func mapSide(files map[string]string) changecheck.Side {
	return func(path string) (string, bool, error) {
		text, present := files[path]
		return text, present, nil
	}
}

// commitOf answers the change that turns before into after: every path whose
// text differs is carried, every path after lacks is removed.
func commitOf(before, after map[string]string) changecheck.Change {
	var paths, removed []string
	for rel, text := range after {
		if was, held := before[rel]; !held || was != text {
			paths = append(paths, rel)
		}
	}
	for rel := range before {
		if _, held := after[rel]; !held {
			removed = append(removed, rel)
		}
	}
	slices.Sort(paths)
	slices.Sort(removed)
	return changecheck.Change{
		Paths: paths, Removed: removed,
		Before: mapSide(before), After: mapSide(after),
		BeforeUnder: func(dir string) (map[string]string, error) {
			held := map[string]string{}
			for rel, text := range before {
				if strings.HasPrefix(rel, dir+"/") {
					held[rel] = text
				}
			}
			return held, nil
		},
	}
}

// judgeCommit judges the commit from before to after and answers the refusal.
func judgeCommit(t *testing.T, before, after map[string]string) string {
	t.Helper()
	if maps.Equal(before, after) {
		t.Fatal("the fixture commit changes nothing, so no verdict of it can discriminate")
	}
	text, err := judgeStaleRecords(commitOf(before, after))
	if err != nil {
		t.Fatalf("the gate could not judge the fixture commit: %v", err)
	}
	return text
}

// requireRefusal fails unless text refuses and says every one of want.
func requireRefusal(t *testing.T, text string, want ...string) {
	t.Helper()
	if text == "" {
		t.Fatal("the commit was admitted; it stales a record and carries no re-taken one")
	}
	for _, one := range want {
		if !strings.Contains(text, one) {
			t.Errorf("the refusal does not say %q:\n%s", one, text)
		}
	}
}

// requireAdmitted fails unless text is empty.
func requireAdmitted(t *testing.T, text string) {
	t.Helper()
	if text != "" {
		t.Fatalf("the commit was refused, and it stales no record:\n%s", text)
	}
}

// retakePrefix is the start of the re-take line every refusal of the selftest
// record prints.
const retakePrefix = "./le rfc record id " + selftestRIDSend + " polarity positive unit " +
	selftestDiscriminationUnit + " route "

// VALIDATES: (a) a commit that changes the tagged unit's behavior and carries no
// new record is refused, naming the record, the unit-changed state and the
// re-take line with the record's route.
func TestCommitGateRefusesAChangedUnitWithoutARecord(t *testing.T) {
	before, _ := recordedParent(t, mutantRecord())
	after := maps.Clone(before)
	after[selftestTestPath] = strings.Replace(selftestTestSource, "!= 1", "== 1", 1)

	requireRefusal(t, judgeCommit(t, before, after),
		selftestDiscriminationRel, ProofUnitChanged, selftestDiscriminationUnit,
		retakePrefix+"mutant report <gomu-report.json> mutant <file:line:column#n>",
		"./le rfc discriminate stem "+selftestStem+" report <gomu-report.json>",
		`"`+selftestBreak+`" in `+selftestProducerUnit)
}

// VALIDATES: (b) the same commit carrying the record re-taken over the code it
// leaves is admitted.
func TestCommitGateAdmitsAChangedUnitWithItsRetakenRecord(t *testing.T) {
	before, record := recordedParent(t, mutantRecord())
	after := maps.Clone(before)
	after[selftestTestPath] = strings.Replace(selftestTestSource, "!= 1", "== 1", 1)
	retaken := sealFixture(t, after, &record)
	putDiscrimination(t, after, retaken)
	if after[selftestDiscriminationRel] == before[selftestDiscriminationRel] {
		t.Fatal("the re-taken record matches the old one, so the fixture carries no re-take")
	}

	requireAdmitted(t, judgeCommit(t, before, after))
}

// VALIDATES: (c) a commit that changes the behavior of the producer the record
// broke, and touches no tagged unit, is admitted: the proof belongs to the test,
// and the unchanged test still catches what it caught (ai/META-DECISIONS.md,
// "Test changes").
func TestCommitGateAdmitsAChangedProducer(t *testing.T) {
	before, _ := recordedParent(t, revertRecord())
	after := maps.Clone(before)
	after[selftestProducerPath] = strings.Replace(selftestProducerSource, "return count", "return 0", 1)

	requireAdmitted(t, judgeCommit(t, before, after))
}

// VALIDATES: (d) a record the parent already holds stale is debt the commit did
// not make: a commit that touches the same file in an unrelated function is
// admitted.
// PREVENTS: the 581 records stale at HEAD blocking every commit near them.
func TestCommitGateAdmitsARecordAlreadyStaleAtTheParent(t *testing.T) {
	files, record := discriminationFixture(t)
	staleUnit := strings.Replace(selftestTestSource, "!= 1", "== 1", 1)
	if state := verifyFixture(t, files, map[string]string{selftestTestPath: staleUnit}, record)[0].State; state != ProofUnitChanged {
		t.Fatalf("the parent's record answered %q, want %q: the fixture is not stale at the parent", state, ProofUnitChanged)
	}
	before := maps.Clone(files)
	before[selftestTestPath] = staleUnit
	putDiscrimination(t, before, record)
	after := maps.Clone(before)
	after[selftestTestPath] = staleUnit + "\nfunc widgetHelper() int {\n\treturn 2\n}\n"

	requireAdmitted(t, judgeCommit(t, before, after))
}

// VALIDATES: the gate judges at the function a record fingerprints: an edit to
// an unrelated function of the unit's file leaves a fresh record fresh. The
// producer-file edit beside it is not a key the gate judges at all.
func TestCommitGateAdmitsAnUnrelatedFunctionInTheUnitFile(t *testing.T) {
	before, _ := recordedParent(t, mutantRecord())
	after := maps.Clone(before)
	after[selftestTestPath] = selftestTestSource + "\nfunc widgetHelper() int {\n\treturn 2\n}\n"
	after[selftestProducerPath] = selftestProducerSource + "\nfunc otherWidget() int {\n\treturn 3\n}\n"

	requireAdmitted(t, judgeCommit(t, before, after))
}

// VALIDATES: (e) a comment-only edit inside the unit changes no fingerprint, and
// the producer edit beside it stales nothing, so the commit is admitted.
func TestCommitGateAdmitsACommentOnlyEdit(t *testing.T) {
	before, _ := recordedParent(t, mutantRecord())
	after := maps.Clone(before)
	after[selftestTestPath] = strings.Replace(selftestTestSource,
		"func TestWidget() {\n", "func TestWidget() {\n\t// Check the count.\n", 1)
	after[selftestProducerPath] = strings.Replace(selftestProducerSource,
		"// SendWidget answers the widget the speaker sends.", "// SendWidget answers the count.", 1)

	requireAdmitted(t, judgeCommit(t, before, after))
}

// VALIDATES: (f) a commit that removes the producer, from its file or with its
// file, and touches no tagged unit, is admitted without reading the parent's
// records: a producer file is not a key this gate judges, because the proof
// belongs to the test (ai/META-DECISIONS.md, "Test changes").
func TestCommitGateAdmitsARemovedProducer(t *testing.T) {
	removals := map[string]func(map[string]string){
		"the function removed": func(after map[string]string) { after[selftestProducerPath] = "package sample\n" },
		"the file removed":     func(after map[string]string) { delete(after, selftestProducerPath) },
	}
	for name, remove := range removals {
		t.Run(name, func(t *testing.T) {
			before, _ := recordedParent(t, revertRecord())
			after := maps.Clone(before)
			remove(after)
			change := commitOf(before, after)
			change.BeforeUnder = func(string) (map[string]string, error) {
				t.Fatal("the gate listed the parent's records for a commit that touches only a producer")
				return nil, errors.New("unreachable: t.Fatal stops the test")
			}

			text, err := judgeStaleRecords(change)
			if err != nil {
				t.Fatalf("the gate could not judge the fixture commit: %v", err)
			}
			requireAdmitted(t, text)
		})
	}
}

// VALIDATES: a reworded claim stales the record exactly as a changed unit does.
func TestCommitGateRefusesARewordedClaim(t *testing.T) {
	before, _ := recordedParent(t, mutantRecord())
	after := maps.Clone(before)
	after[selftestTestPath] = strings.Replace(selftestTestSource, "sends exactly one", "sends exactly one or more", 1)

	requireRefusal(t, judgeCommit(t, before, after), ProofClaimChanged)
}

// VALIDATES: a tag the commit removes takes its record with it, so the orphan is
// not this gate's to refuse; the judge approves the removal elsewhere.
func TestCommitGateAdmitsARemovedTag(t *testing.T) {
	before, _ := recordedParent(t, mutantRecord())
	after := maps.Clone(before)
	after[selftestTestPath] = "package sample\n\nfunc TestWidget() {\n\tif SendWidget(1) == 1 {\n\t\tpanic(\"BUG\")\n\t}\n}\n"

	requireAdmitted(t, judgeCommit(t, before, after))
}

// VALIDATES: a commit that deletes a record while its tag stays is refused, so a
// deletion is not a way past a stale record. The producer edit beside it stales
// nothing by itself: the refusal is the deletion's.
func TestCommitGateRefusesADeletedRecordBesideItsTag(t *testing.T) {
	before, _ := recordedParent(t, revertRecord())
	after := maps.Clone(before)
	after[selftestProducerPath] = strings.Replace(selftestProducerSource, "return count", "return 0", 1)
	delete(after, selftestDiscriminationRel)

	requireRefusal(t, judgeCommit(t, before, after),
		"is deleted by this commit while its tag stays", retakePrefix+"revert producer "+selftestProducerUnit)
}

// VALIDATES: a commit that touches no file a record names, and no record file,
// is admitted without reading the parent's corpus, so a docs commit pays
// nothing for this gate.
func TestCommitGateReadsNothingForAnUnrelatedCommit(t *testing.T) {
	change := changecheck.Change{
		Paths:  []string{"docs/guide/widget.md"},
		Before: mapSide(nil), After: mapSide(map[string]string{"docs/guide/widget.md": "# Widget\n"}),
		BeforeUnder: func(string) (map[string]string, error) {
			t.Fatal("the gate listed the parent's records for a commit that touches none of their files")
			return nil, errors.New("unreachable: t.Fatal stops the test")
		},
	}
	text, err := judgeStaleRecords(change)
	if err != nil {
		t.Fatalf("the gate could not judge a docs commit: %v", err)
	}
	requireAdmitted(t, text)
}

// VALIDATES: the gate is registered in the changecheck seam under the command
// a refusal names, so `./le commit create` runs it.
func TestCommitGateIsRegistered(t *testing.T) {
	for _, check := range changecheck.Registered() {
		if check.Name == area+" check" {
			return
		}
	}
	t.Fatal("no change-set check is registered as `rfc check`, so no commit runs the stale-record gate")
}

// VALIDATES: an argument the shell would split or expand is quoted, and a key
// is printed bare.
func TestRecordWordQuotesWhatAShellSplits(t *testing.T) {
	for value, want := range map[string]string{
		selftestDiscriminationUnit: selftestDiscriminationUnit,
		"expect=widget sent":       "'expect=widget sent'",
		"it's":                     `'it'"'"'s'`,
		"":                         "''",
	} {
		if got := recordWord(value); got != want {
			t.Errorf("recordWord(%q) = %q, want %q", value, got, want)
		}
	}
}
