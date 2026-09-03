// VALIDATES: every production byte of this package is the byte its author
// sealed, so a behavior change here is deliberate and reviewed.
// PREVENTS: an edit to a table, an edge case or a closed set that no behavioral
// test reaches, landing with nobody stating whether an audit verdict moved.

package rfc

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// TestNativeImplementationFixture replaces the retired cross-runtime oracle.
// The digest pins every production byte that supplied its tables and edge-case
// decisions, while the behavioral tests in this package pin their outcomes.
func TestNativeImplementationFixture(t *testing.T) {
	// The digest changes on EVERY non-test byte of this package, so each edit owes
	// a re-seal. What the re-seal owes back is one line saying whether any VERDICT
	// moved, which is the only thing this test cannot see for itself. The change
	// itself is in its commit message, and repeating it here made this comment a
	// changelog nobody reads.
	//
	// Re-sealed 2026-08-31, for spec-rfc-tag-claim-discrimination. Three verdicts
	// moved, all on the ESCAPE, and all deliberately: it is tied to the claim it
	// discharges rather than to any file an author names, its producer must be
	// code the tagged unit reaches, and a producer key naming a function its file
	// does not declare is refused. A record staled by an edit nobody has committed
	// is reported rather than refused (owner decision, 2026-08-31).
	//
	// Over this tree no verdict moved: no record in it carries an escape, and the
	// violations `./le rfc check` reports are other sessions' corpus and tag work.
	//
	// Re-sealed 2026-08-31 for the feature-out-of-scope exclusion kind. No verdict
	// moved: the change adds one entry to the closed exclusion vocabulary that
	// ParseExtractionArtifact accepts, and rfc/audit/ records no exclusion kind.
	//
	// Re-sealed 2026-09-01, for plan/spec-publish-the-rfc-requirement-ledger.md.
	// No verdict moved. The change is an EXPORT pass: the polarities, the
	// annotation kinds, the audit verdicts, the discrimination routes, the proof
	// states, the site dispositions, the cover key, the discrimination verdict,
	// the per-RFC coverage row and the markdown cell escape are renamed to their
	// exported spellings so internal/le/site can publish them, `Audit.Record`
	// answers one verdict typed, the audit vocabulary carries the sentence each
	// word means, `RequirementRows` becomes the ONE producer of a shard's six
	// cells with `RenderShards` formatting what it answers, `parseEnrolled` keeps
	// the enrolment reason, and a summary's Meta `| Title |` row becomes a parsed
	// fact. Verified by regenerating: `ai/RFC-REQUIREMENTS.md` and 189 shards
	// re-rendered byte-identical apart from `rfc/requirements/rfc4724.md`, which
	// was stale against a tag committed before this work.
	//
	// The digest covers every non-test byte of this package, so it also carries
	// the uncommitted edits other sessions hold in this shared checkout.
	//
	// Re-sealed 2026-09-01, for the two baselines the discrimination obligation
	// reads (owner decision). Two verdicts moved. A tag owes its proof where the
	// TIP COMMIT added it against HEAD^, so a tag only in somebody's working tree
	// is nobody's violation and the author meets it inside the detached verify
	// worktree. A stale record's drift is judged at the granularity the record
	// fingerprints, so an unrelated uncommitted edit elsewhere in the producer's
	// file no longer downgrades a committed drift to a report.
	//
	// Re-sealed 2026-09-01, for the escape's reach test. One verdict moved: a
	// carrier that runs the daemon is no longer exempt from reach. The exemption
	// read that a .ci or an interop scenario reaches every compiled file, which is
	// true and is why it had to go: a predicate every file satisfies ties the
	// escape to no claim, so the route shut for unit tags stayed open for the 94
	// .ci and 37 interop ones. Those two reasons are now unreachable on that
	// carrier by construction.
	// Re-sealed 2026-09-01, for the second publication pass over
	// plan/spec-publish-the-rfc-requirement-ledger.md. No verdict moved. Three
	// edits, all uncommitted work of that spec: `ExtractionSection.Title`
	// derives a section's own name from the opening sentence of its reason so a
	// published row can print "3 - Constructing the Next Hop field" rather than
	// "3"; `Verified` and `Proves` move to sit under the types they receive;
	// and one comment takes the US spelling. None reads a record, changes a
	// verdict, or moves a requirement into or out of a bucket.
	// Re-sealed again on 2026-09-01. The value moved twice for two reasons and
	// only the first is this session's. This session's edits are the three
	// above and no verdict moved for them. The rest of the move is three
	// commits other sessions landed on this package between the two seals --
	// 803f1b696, 3a7bca913 and 99483ef93 -- and NONE of them re-sealed, so the
	// digest was already red at HEAD before this working tree touched it. This
	// note does not vouch for those three: each carries its own verdict review
	// in its own commit message, and a shared checkout gives no way to seal one
	// change without absorbing another (the paragraph above says so).
	// Re-sealed 2026-09-01 for the exclusion-kind meanings. No verdict moved.
	// `exclusionKinds` changes from a set to a map of kind to the sentence the
	// kind says, which is the shape `auditVerdicts` already has and for the
	// same reason: internal/le/site aggregates these counts across the corpus
	// and a reader who meets `binds-another-role` cannot act on the word alone.
	// `ExclusionKindMeaning` and `ExclusionPresumedWrong` are readers of that
	// table. The membership of the closed set is byte-identical, and the one
	// call site that tested it now tests the map for presence.
	// Re-sealed 2026-09-01 for the exclusion GROUP and the structured finding.
	// No verdict moved for either. `exclusionKinds` gains a group beside each
	// meaning, so a published page can tell an obligation that never bound Ze
	// from one Ze owes; the membership of the closed set is byte-identical.
	// `Finding` carries the parts a check held before it formatted its line,
	// `CheckReport.Findings` becomes the one accumulator and `Violations` is
	// rendered from it, and `Text` takes a pointer receiver because the struct
	// passed the linter's size floor. Every message the gate prints is
	// byte-identical: the checks author them exactly as before and the finding
	// carries them.
	//
	// Re-sealed 2026-09-01 for the fixture half of the meta migration: every
	// fixture summary in this package's tests now carries the `## Meta` table
	// its enrolment and its public row are declared in. No verdict moved for
	// that work and none could, because it added no production byte -- the
	// digest covers no test file.
	//
	// Re-sealed 2026-09-01 for the migration itself, which the paragraph above
	// had only absorbed. Enrolment and the public support claim are declared in
	// each summary's `## Meta` table and the three ledger files are generated
	// from it; `parseEnrolled`, `parseDispositions`, `parseStatusLedger` and
	// their loaders are gone, `checkStatusCompleteness` with them, and three
	// refusals of `checkSummaryDisposition` are unrepresentable rather than
	// retired. Two dispositions are new -- `source-restricted` for a standard
	// whose text may not be redistributed, and `out-of-scope` for one whose
	// extraction is done and whose feature the owner declined -- and each
	// carries its own guard. No AUDIT verdict moved: the audit schema, its
	// freshness and its ratchet are untouched by this work, and the gate's
	// findings differ only where a message names the summary rather than a
	// retired file.
	//
	// That value already covers the carrier reading order, sealed by the same
	// hash: `editor` and `unknown` become named constants beside the three
	// kinds that already were, and `carrierKindOrder`, `carrierTierOrder`,
	// `CarrierKinds`, `CarrierTiers`, `CarrierRank` and `CarrierLabelRank`
	// declare the order evidence reads in beside the vocabulary it orders. No
	// verdict moved: the carrier table is byte-identical in Name, Kind, Tier,
	// Prefix and Suffix, the two literals replaced spelled the same words, and
	// the rank has no reader inside this package.
	// Re-sealed 2026-09-01 for the published disposition vocabulary and for
	// three exported functions with no production caller. `dispositionKinds`
	// states what each un-enrolled kind MEANS, beside `exclusionKinds` and for
	// the same reason: a page printing `source-restricted` and stopping has
	// told a reader nothing, and the sixth kind, `out-of-scope`, landed the
	// same day. `DispositionKinds` and `DispositionKindMeaning` publish it.
	// `CarrierKinds`, `CarrierTiers` and `ExclusionGroups` are gone: each was
	// called by tests alone, which `./le doc wiring` refuses, and each test now
	// reads the vocabulary it was wrapping. No verdict moved and no behavior
	// with it -- the kinds, their order and their groups are unchanged, and
	// nothing in this package read the three deleted wrappers.
	//
	// The digest is over the TREE, so it was computed with another session's
	// `checkPublicRowMonotonic` present in check.go and check_status.go. Two
	// commits changing one package cannot each carry a standalone value:
	// whichever lands second re-seals, and this note says which two changes the
	// value here already covers.
	// Re-sealed 2026-09-01 for the findings of an independent review of the
	// ledger migration, and one of them was a real hole rather than a polish.
	// `checkPublicRowMonotonic` restores the guard `checkStatusCompleteness`
	// carried: a public row that DISAPPEARS while its RFC stays enrolled is a
	// representable state, not an unrepresentable one, and the commit message
	// that retired the check claimed `checkRetiredRequirements` covered it when
	// that ratchet reads requirement ids and never a `Support` cell. With it,
	// `out-of-scope` must declare the extraction its premise rests on,
	// `source-restricted` may not be written over a source text that is in the
	// tree, an unescaped pipe in a Meta value is refused rather than truncating
	// it, the generated remainder counts every kind the parser accepts rather
	// than a hand-written five, and `summaryMetas` COLLECTS its parse errors so
	// one summary mid-edit no longer stops the gate for every session sharing
	// this checkout. No audit verdict moved: the audit schema, its freshness and
	// its ratchets are untouched, and what changed is which trees the gate
	// refuses.
	// Re-sealed 2026-09-02, and the value moved for a reason the reader itself
	// changed rather than for anything this package now does differently: the
	// digest is taken over HEAD's committed blobs, so it no longer describes
	// whatever any session happened to have uncommitted. A third review round
	// caught the first version of this note attributing the new value to the
	// round-2 fixes, which by construction are not in it.
	//
	// What the round-2 round found, recorded here because the next re-seal is
	// where a reader looks for it: two of the FIRST round's fixes were wrong
	// rather than incomplete.
	// `checkPublicRowMonotonic` was keyed on enrolment and the checks it
	// protects iterate the ROWS, so a support-promising row on a non-enrolled
	// summary could still be retired in silence -- `rfc/short/rfc9384.md` is the
	// live instance, `non-normative` with a row reading "Supported within BFD".
	// It is keyed on the row now. And `NewRenderInput`'s refusal on an
	// unparsable Meta table sat inside a branch no production caller takes,
	// because `Collect` always returns a non-nil map; `Collected` carries its
	// `MetaProblems` and the refusal runs on every path. The generated
	// remainder's kind meanings are derived from the closed set beside its
	// counts, since a header describing kinds by ordinal is a claim about a
	// sorted order. No audit verdict moved: what changed is which trees the gate
	// refuses and what one generated file says about itself.
	//
	// Re-sealed 2026-09-02, for four commits: the second refusal arm on
	// checkUnprovenSupport and the Proof cell it publishes beside every public
	// row, ProvenShareOf as the one producer of the published proof share, the
	// textbuf conversion of check_audit.go, and one dead Reset in meta.go.
	//
	// No verdict moved, and `./le rfc reseal` says so independently: nothing is
	// in the shifted state. What the gate reports is other sessions' work and
	// predates these commits -- two RFC 7606 audit verdicts stale against a
	// func-scoped change in check_rfc.go, which reseal REFUSES because a human
	// must re-read them; three discrimination records whose tagged units moved;
	// an rfc8671 extraction sign-off that bounds nothing; and MUST-level rows in
	// rfc5798 and rfc8671 carrying neither a test nor an annotation.
	//
	// The new arm is what makes the last of those visible rather than green: a
	// row promising Supported over a checklist with zero both-polarity proofs is
	// refused now, where the old arm only refused a row over an EMPTY checklist.
	// Seven summaries said Supported over zero proofs and now say Partial.
	//
	// Re-sealed 2026-09-03, for two commits, and both WIDEN the closed
	// annotation vocabulary rather than change what any check decides:
	// {lower-layer}, for an obligation a layer under Ze performs, and
	// {feature-declined}, for one whose condition is false because Ze declined
	// the OPTIONAL feature it hangs on. Each arrives with its own refusal,
	// checkLowerLayerProducer and checkFeatureDeclined, and the corpus test
	// beside this one now reads its population from AnnotationKinds(), so both
	// were held to the share rule on the day they were written.
	//
	// One published verdict moved, and the commit that moved it says so: rfc4552
	// reads Partial, because two RFC 4552 obligations Ze does not meet are
	// disclosed now rather than absent.
	//
	// `./le rfc reseal` re-stamped five RFC 7606 verdicts on this run and
	// refused two. None of the five is this package's doing: each is mechanical,
	// the unit fingerprints are byte-identical, and what moved is the file hash
	// of session_validate_test.go under another session's edit. The two
	// refusals are the same pair the 2026-09-02 note names, still waiting on the
	// human RFC re-read that reseal will not do for them.
	const want = "9242b1968d44b9acff0107139eb170679559b95a1bc789a1c9e753f4eb84dbcf"
	// HEAD's committed bytes, never the working tree. A seal taken over the
	// working tree states a fact about one transient moment: it passed for the
	// session that minted it and was RED on a clean clone, because the value it
	// recorded had absorbed whatever every other session happened to have
	// uncommitted at that instant. This test was in that state at 8267f3e52,
	// found by an independent review on 2026-09-02, and it is the same
	// shared-checkout hazard the discrimination records fingerprint against
	// HEAD to avoid (docs/contributing/rfc-conformance-gates.md, owner
	// decision 2026-08-31).
	//
	// The cost is that the seal trails its own change by one commit: the author
	// commits, this goes red on the next run, and the re-seal rides in the
	// following commit. That is the right way round. A tripwire that fires for
	// the author after they commit is doing its job; one that is red for
	// everybody who did not make the change is not.
	root, err := lepath.Root()
	if err != nil {
		t.Skipf("resolve checkout: %v", err)
	}
	paths, ok := gitTreePaths(root, "internal/le/rfc", ".go")
	if !ok {
		t.Skip("git cannot list HEAD, so there is no committed state to seal")
	}
	blobs, known := gitCatBlobs(root, headRevision, paths)
	if !known {
		t.Skip("git cannot read HEAD's blobs, so there is no committed state to seal")
	}
	sort.Strings(paths)
	digest := sha256.New()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, held := blobs[path]
		if !held {
			t.Fatalf("HEAD lists %s and holds no blob for it", path)
		}
		digest.Write([]byte(filepath.Base(path)))
		digest.Write([]byte{0})
		digest.Write([]byte(content))
		digest.Write([]byte{0})
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		t.Fatalf("native RFC fixture digest = %s, want %s; review the behavior change and update the owned fixture", got, want)
	}
}

func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve checkout root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("checkout root %s has no go.mod: %v", root, err)
	}
	return root
}
