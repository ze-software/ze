// Design: docs/architecture/testing/runner-architecture.md -- semantic UI answer contracts.

package fixture

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

func init() {
	Register("ui/le-rfc-answers", uiDriver(leRFCAnswers))
	Register("ui/le-rfc-partial-proof", uiDriver(leRFCPartialDriver("partial", leRFCPartialProof)))
	Register("ui/le-rfc-partial-proof-render", uiDriver(leRFCPartialDriver("partial-render", leRFCPartialProofRender)))
	Register("ui/le-rfc-partial-proof-selector", uiDriver(leRFCPartialDriver("partial-selector", leRFCPartialProofSelector)))
	Register("ui/le-rfc-partial-proof-malformed", uiDriver(leRFCPartialDriver("partial-malformed", leRFCPartialProofMalformed)))
}

type leRFCAnswersFailure struct {
	message string
}

func (f leRFCAnswersFailure) Error() string { return f.message }

type leRFCAnswersResult struct {
	stdout string
	stderr string
	code   int
}

func leRFCAnswers(ctx context.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if failure, ok := recovered.(leRFCAnswersFailure); ok {
				err = failure
				return
			}
			panic(recovered)
		}
	}()

	here, _, err := temporaryLEFixtureWorkspace("le-rfc-answers-")
	leRFCAnswersRequireNoError(err, "create fixture directory")
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	leRFCAnswersRequireNoError(err, "locate native le binary")
	root := leRFCAnswersTree(ctx, here, binary)

	runLE := func(tree string, args ...string) leRFCAnswersResult {
		return leRFCAnswersRun(ctx, here, map[string]string{envRepoRoot: tree}, binary, args...)
	}

	// The extraction envelope is a read-only JSON document derived from the
	// owned tree, and all three registers are published even when one is empty.
	extraction := runLE(root, "rfc", "extraction-status")
	leRFCAnswersRequire(extraction.code == 0,
		"rfc extraction-status exited %d\nstdout:\n%s\nstderr:\n%s",
		extraction.code, extraction.stdout, extraction.stderr)
	leRFCAnswersRequire(extraction.stderr == "",
		"rfc extraction-status wrote to stderr: %q", extraction.stderr)
	envelope := leRFCAnswersJSONObject(extraction.stdout, "rfc extraction-status")
	enrolled := leRFCAnswersJSONInt(envelope["enrolled"], "extraction-status.enrolled")
	signed := leRFCAnswersJSONInt(envelope["signed"], "extraction-status.signed")
	backlog := leRFCAnswersJSONInt(envelope["backlog"], "extraction-status.backlog")
	leRFCAnswersRequire(enrolled == 1,
		"rfc extraction-status read %d enrolled RFC(s), want the one authored summary", enrolled)
	leRFCAnswersRequire(signed+backlog == enrolled,
		"signed (%d) and backlog (%d) do not cover enrolled (%d): %s",
		signed, backlog, enrolled, extraction.stdout)
	leRFCAnswersRequire(signed == 0 && backlog == 1,
		"the unsigned owned summary answered signed=%d backlog=%d, want 0 and 1", signed, backlog)
	registers, ok := envelope["signed-by-register"].(map[string]any)
	leRFCAnswersRequire(ok,
		"extraction-status.signed-by-register is not an object: %s", extraction.stdout)
	registerNames := leRFCAnswersMapKeys(registers)
	leRFCAnswersRequire(reflect.DeepEqual(registerNames, []string{"manual-walk", "prose", "rfc2119"}),
		"extraction-status published registers %v, want all three registers", registerNames)
	registerTotal := int64(0)
	for name, value := range registers {
		registerTotal += leRFCAnswersJSONInt(value, "extraction-status.signed-by-register."+name)
	}
	leRFCAnswersRequire(registerTotal == signed,
		"register total %d does not equal signed total %d: %s",
		registerTotal, signed, extraction.stdout)

	for _, operator := range []string{renderJSON, renderYAML} {
		rendered := runLE(root, "rfc", "extraction-status", "|", operator)
		leRFCAnswersRequire(rendered.code == 0,
			"rfc extraction-status | %s exited %d\nstdout:\n%s\nstderr:\n%s",
			operator, rendered.code, rendered.stdout, rendered.stderr)
		leRFCAnswersRequire(rendered.stderr == "",
			"rfc extraction-status | %s wrote to stderr: %q", operator, rendered.stderr)
		leRFCAnswersRequire(strings.Contains(rendered.stdout, "enrolled"),
			"rfc extraction-status | %s rendered no envelope: %.200s", operator, rendered.stdout)
	}
	extractionJSON := runLE(root, "rfc", "extraction-status", "|", "json")
	leRFCAnswersRequire(extractionJSON.code == 0,
		"second JSON rendering of extraction-status exited %d: %s",
		extractionJSON.code, extractionJSON.stderr)
	asJSON := leRFCAnswersJSONObject(extractionJSON.stdout, "rfc extraction-status | json")
	leRFCAnswersRequire(
		leRFCAnswersJSONInt(asJSON["enrolled"], "rendered extraction-status.enrolled") == enrolled,
		"rfc extraction-status | json answered a different enrolled count: %s", extractionJSON.stdout,
	)

	// The check compares page and JSON over a fixed baseline, then a copy with
	// exactly one new untagged requirement. No ambient checkout debt can meet
	// the seeded requirement's assertion.
	check := leRFCAnswersRunCheck(runLE, root, "owned tree")

	// A tree the gate cannot READ answers `cannot run` and exits 2. Clean and
	// unreadable MUST NOT render the same. A gate that never ran, reported as
	// a pass, is the silent zero this repository bans
	// (ai/rules/principles.md).
	unreadable := filepath.Join(here, "unreadable")
	leRFCAnswersRequireNoError(os.MkdirAll(unreadable, 0o750), "create the unreadable tree")
	blind := runLE(unreadable, "rfc", actionCheck)
	leRFCAnswersRequire(blind.code == 3,
		"a tree the gate cannot read exited %d, want could-not-run code 3\nstdout:\n%s\nstderr:\n%s",
		blind.code, blind.stdout, blind.stderr)
	leRFCAnswersRequire(strings.HasPrefix(blind.stdout, "rfc-requirements: cannot run: "),
		"a tree the gate cannot read did not say so:\n%s", blind.stdout)
	blindJSON := runLE(unreadable, "rfc", actionCheck, "|", renderJSON)
	blindReport := leRFCAnswersJSONObject(blindJSON.stdout, "unreadable rfc check | json")
	leRFCAnswersRequireDiscriminationKeys(blindReport, "unreadable", blindJSON.stdout)
	reason, ok := blindReport["cannot-run"].(string)
	leRFCAnswersRequire(ok && reason != "",
		"an unreadable tree answered no cannot-run reason: %s", blindJSON.stdout)
	_, blindViolations := blindReport["violations"]
	leRFCAnswersRequire(!blindViolations,
		"an unreadable tree answered a violation list rather than a refusal: %s", blindJSON.stdout)

	// Both halves use the same owned input. The seeded requirement alone must
	// add a refusal without losing any unrelated baseline finding.
	seedTree := leRFCAnswersCopyTree(root, here, "check-seed")
	clean := leRFCAnswersRunCheck(runLE, seedTree, "clean export")
	leRFCAnswersRequire(!strings.Contains(clean.page, leRFCAnswersSeedID),
		"a clean export already names %s, so seeding it proves nothing:\n%s",
		leRFCAnswersSeedID, clean.page)

	leRFCAnswersSeedRequirement(seedTree)
	seeded := leRFCAnswersRunCheck(runLE, seedTree, "seeded export")
	leRFCAnswersRequire(seeded.code == 1,
		"one gated MUST that no test tags exited %d, want found-a-problem code 1:\n%s",
		seeded.code, seeded.page)
	refused := false
	var named []string
	for _, violation := range seeded.violations {
		if !strings.Contains(violation, leRFCAnswersSeedID) {
			continue
		}
		named = append(named, violation)
		refused = refused || strings.Contains(violation, "has no test and no annotation")
	}
	leRFCAnswersRequire(refused,
		"the gate did not refuse the seeded requirement %s for carrying no test; it named it in %d other row(s): %q",
		leRFCAnswersSeedID, len(named), named)
	held := make(map[string]bool, len(seeded.violations))
	for _, violation := range seeded.violations {
		held[violation] = true
	}
	for _, violation := range clean.violations {
		leRFCAnswersRequire(held[violation],
			"seeding one requirement lost an unrelated violation: %q", violation)
	}
	leRFCAnswersRequire(len(seeded.violations) > len(clean.violations),
		"the seeded export reported %d violation(s) and the clean one %d",
		len(seeded.violations), len(clean.violations))

	// The report is one payload, so the two remaining operators the catalog
	// offers render it rather than refusing it (ai/rules/cli.md).
	for _, operator := range []string{renderYAML, renderTable} {
		rendered := runLE(seedTree, "rfc", actionCheck, "|", operator)
		leRFCAnswersRequire(rendered.code == seeded.code,
			"rfc check | %s changed action exit %d to %d: %s",
			operator, seeded.code, rendered.code, rendered.stderr)
		leRFCAnswersRequire(strings.Contains(rendered.stdout, leRFCAnswersSeedID),
			"rfc check | %s rendered no seeded row: %.400s", operator, rendered.stdout)
	}

	// The selftest reports the real-tree/public-check property explicitly. Its
	// verdict must agree with the public check above in plain and JSON forms.
	selftest := runLE(root, "rfc", "selftest")
	leRFCAnswersRequire(selftest.stderr == "",
		"rfc selftest wrote to stderr: %q", selftest.stderr)
	if check.code == 0 {
		leRFCAnswersRequire(selftest.code == 0,
			"green-tree rfc selftest exited %d\nstdout:\n%s\nstderr:\n%s",
			selftest.code, selftest.stdout, selftest.stderr)
		leRFCAnswersRequire(selftest.stdout == "rfc_requirements selftest OK\n",
			"green rfc selftest printed a different verdict: %q", selftest.stdout)
	} else {
		leRFCAnswersRequire(selftest.code == 1,
			"red-tree rfc selftest exited %d, want 1\nstdout:\n%s\nstderr:\n%s",
			selftest.code, selftest.stdout, selftest.stderr)
		leRFCAnswersRequire(strings.Contains(selftest.stdout, "real-tree/public-check"),
			"red rfc selftest omitted its real-tree row:\n%s", selftest.stdout)
	}

	selftestJSON := runLE(root, "rfc", "selftest", "|", "json")
	leRFCAnswersRequire(selftestJSON.code == selftest.code,
		"rfc selftest | json changed action exit %d to %d\nstdout:\n%s\nstderr:\n%s",
		selftest.code, selftestJSON.code, selftestJSON.stdout, selftestJSON.stderr)
	leRFCAnswersRequire(selftestJSON.stderr == "",
		"rfc selftest | json wrote to stderr: %q", selftestJSON.stderr)
	selftestPayload, ok := leRFCAnswersJSON(selftestJSON.stdout, "rfc selftest | json").([]any)
	leRFCAnswersRequire(ok && len(selftestPayload) != 0,
		"rfc selftest | json returned no rows: %s", selftestJSON.stdout)
	realTreeRows := 0
	for _, value := range selftestPayload {
		row, ok := value.(map[string]any)
		leRFCAnswersRequire(ok,
			"rfc selftest | json returned a non-object row: %s", selftestJSON.stdout)
		if row["case"] != "real-tree/public-check" {
			continue
		}
		realTreeRows++
		passed, ok := row["passed"].(bool)
		leRFCAnswersRequire(ok,
			"real-tree/public-check has no boolean passed field: %s", selftestJSON.stdout)
		leRFCAnswersRequire(passed == (selftestJSON.code == 0),
			"real-tree/public-check passed=%v disagrees with exit %d: %s",
			passed, selftestJSON.code, selftestJSON.stdout)
	}
	leRFCAnswersRequire(realTreeRows == 1,
		"rfc selftest | json returned %d real-tree/public-check rows, want one: %s",
		realTreeRows, selftestJSON.stdout)

	// Required public actions retain their effect markers as the registry grows.
	// New actions must have a description and a valid marker, not force this
	// consumer test to copy another registry or pin its population.
	listing := runLE(root, "rfc")
	leRFCAnswersRequire(listing.code == 0,
		"rfc listing exited %d\nstdout:\n%s\nstderr:\n%s",
		listing.code, listing.stdout, listing.stderr)
	leRFCAnswersRequire(listing.stderr == "", "rfc listing wrote to stderr: %q", listing.stderr)
	listed := map[string]string{}
	// The area page opens with the area's name, then a Usage block, then an
	// Actions block. An action row is indented two spaces and carries its
	// name, its bracketed effect and a description; a wrapped description
	// continues on lines indented further, which belong to the row above.
	lines := strings.Split(listing.stdout, "\n")
	leRFCAnswersRequire(strings.TrimSpace(lines[0]) == "le rfc",
		"rfc listing has no area heading:\n%s", listing.stdout)
	actionsAt := slices.Index(lines, "Actions:")
	leRFCAnswersRequire(actionsAt > 0, "rfc listing has no Actions block:\n%s", listing.stdout)
	for _, line := range lines[actionsAt+1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		if strings.HasPrefix(line, "   ") {
			continue
		}
		fields := strings.Fields(line)
		leRFCAnswersRequire(len(fields) >= 3,
			"rfc listing has an action without an effect or description: %q", line)
		_, duplicate := listed[fields[0]]
		leRFCAnswersRequire(!duplicate, "rfc listing repeats action %q", fields[0])
		effect := strings.TrimSuffix(strings.TrimPrefix(fields[1], "["), "]")
		leRFCAnswersRequire(fields[1] == "["+effect+"]" && (effect == fieldChecks || effect == wordWrites),
			"rfc listing action %q has invalid effect %q", fields[0], fields[1])
		listed[fields[0]] = effect
	}
	wantListed := map[string]string{
		"extraction-create":   wordWrites,
		"extraction-classify": wordWrites,
		"extraction-status":   fieldChecks,
		"tagged-scope":        fieldChecks,
		"discriminate":        fieldChecks,
		"discriminate-record": wordWrites,
		actionCheck:           fieldChecks,
		actionSelftest:        fieldChecks,
		"approve":             wordWrites,
		"reseal":              wordWrites,
		"audit-stamp":         wordWrites,
		actionUpdate:          wordWrites,
		"quote-backfill":      wordWrites,
	}
	for action, effect := range wantListed {
		leRFCAnswersRequire(listed[action] == effect,
			"rfc listing action %q has effect %q, want %q:\n%s",
			action, listed[action], effect, listing.stdout)
	}
	for _, action := range []string{"extraction-status", actionCheck, actionSelftest, "reseal", actionUpdate} {
		refused := runLE(root, "rfc", action, "rfc7606")
		leRFCAnswersRequire(refused.code == 2,
			"rfc %s accepted a stem argument or exited %d\nstdout:\n%s\nstderr:\n%s",
			action, refused.code, refused.stdout, refused.stderr)
		leRFCAnswersRequire(strings.Contains(refused.stderr, "takes no arguments"),
			"rfc %s refusal omitted 'takes no arguments': %q", action, refused.stderr)
	}
	for _, action := range []string{"extraction-create", "extraction-classify", "tagged-scope"} {
		refused := runLE(root, "rfc", action, "rfc7606")
		leRFCAnswersRequire(refused.code == 2,
			"rfc %s accepted an unkeyed value or exited %d\nstdout:\n%s\nstderr:\n%s",
			action, refused.code, refused.stdout, refused.stderr)
		leRFCAnswersRequire(strings.Contains(refused.stderr, "keyword"),
			"rfc %s refusal omitted the keyword grammar: %q", action, refused.stderr)
	}

	// A no-op re-seal must not alter any byte. Both the plain and JSON renderings
	// execute the writer and therefore are covered by snapshots taken before and
	// after all of those invocations.
	//
	// Writers operate on independent copies, never on the baseline inputs.
	// Prime the owned audit once so the no-op contract starts from fresh data.
	checkoutAuditBefore := leRFCAnswersAuditState(root)
	noopTree := leRFCAnswersCopyTree(root, here, "reseal-noop")
	prime := runLE(noopTree, "rfc", "reseal")
	leRFCAnswersRequire(prime.code == 0 && prime.stderr == "",
		"priming rfc reseal on the owned copy failed with exit %d\nstdout:\n%s\nstderr:\n%s",
		prime.code, prime.stdout, prime.stderr)
	noopAuditBefore := leRFCAnswersAuditState(noopTree)
	reseal := runLE(noopTree, "rfc", "reseal")
	leRFCAnswersRequire(reseal.code == 0,
		"rfc reseal refused the primed owned copy with exit %d\nstdout:\n%s\nstderr:\n%s",
		reseal.code, reseal.stdout, reseal.stderr)
	leRFCAnswersRequire(reseal.stderr == "", "rfc reseal wrote to stderr: %q", reseal.stderr)
	leRFCAnswersRequire(strings.Contains(reseal.stdout, "nothing to re-seal"),
		"primed-export rfc reseal omitted its no-op verdict:\n%s", reseal.stdout)
	leRFCAnswersRequire(!strings.Contains(reseal.stdout, "\x1b["),
		"rfc reseal emitted terminal escapes: %q", reseal.stdout)

	resealAgain := runLE(noopTree, "rfc", "reseal")
	leRFCAnswersRequire(resealAgain.code == 0 && resealAgain.stderr == "",
		"second primed-export rfc reseal failed with exit %d\nstdout:\n%s\nstderr:\n%s",
		resealAgain.code, resealAgain.stdout, resealAgain.stderr)
	leRFCAnswersRequire(resealAgain.stdout == reseal.stdout,
		"two no-op re-seals rendered different bytes\nfirst:\n%s\nsecond:\n%s",
		reseal.stdout, resealAgain.stdout)

	resealJSON := runLE(noopTree, "rfc", "reseal", "|", "json")
	leRFCAnswersRequire(resealJSON.code == 0,
		"rfc reseal | json exited %d\nstdout:\n%s\nstderr:\n%s",
		resealJSON.code, resealJSON.stdout, resealJSON.stderr)
	leRFCAnswersRequire(resealJSON.stderr == "",
		"rfc reseal | json wrote to stderr: %q", resealJSON.stderr)
	resealPayload := leRFCAnswersJSONObject(resealJSON.stdout, "rfc reseal | json")
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(resealPayload), []string{"refused", "resealed"}),
		"rfc reseal | json returned fields %v, want refused and resealed: %s",
		leRFCAnswersMapKeys(resealPayload), resealJSON.stdout)
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersAuditState(noopTree), noopAuditBefore),
		"a no-op re-seal changed rfc/audit in the primed owned copy")

	// Build the set of tagged files from the audit records. Appending below all
	// functions preserves units while changing every cited file's whole-file
	// identity, forcing all unit-bearing verdicts into the re-sealable state.
	cited := leRFCAnswersCitedFiles(noopAuditBefore)
	leRFCAnswersRequire(reflect.DeepEqual(cited, []string{"internal/widget/widget_test.go"}),
		"owned audit records cite %v, want the authored widget test", cited)

	resealTreeA := leRFCAnswersCopyTree(root, here, "reseal-a")
	resealTreeB := leRFCAnswersCopyTree(root, here, "reseal-b")
	leRFCAnswersShift(resealTreeA, cited)
	leRFCAnswersShift(resealTreeB, cited)
	shiftedBeforeA := leRFCAnswersAuditState(resealTreeA)
	shiftedBeforeB := leRFCAnswersAuditState(resealTreeB)
	leRFCAnswersRequire(reflect.DeepEqual(shiftedBeforeA, shiftedBeforeB),
		"two owned copies began with different rfc/audit bytes")

	shiftedA := runLE(resealTreeA, "rfc", "reseal")
	shiftedB := runLE(resealTreeB, "rfc", "reseal")
	for name, result := range map[string]leRFCAnswersResult{"reseal-a": shiftedA, "reseal-b": shiftedB} {
		leRFCAnswersRequire(result.code == 0,
			"%s rfc reseal exited %d\nstdout:\n%s\nstderr:\n%s",
			name, result.code, result.stdout, result.stderr)
		leRFCAnswersRequire(result.stderr == "", "%s rfc reseal wrote to stderr: %q", name, result.stderr)
		leRFCAnswersRequire(strings.Contains(result.stdout, "re-sealed "),
			"%s shift moved no verdict into the re-sealable state:\n%s", name, result.stdout)
		leRFCAnswersRequire(!strings.Contains(result.stdout, "\x1b["),
			"%s rfc reseal emitted terminal escapes: %q", name, result.stdout)
	}
	leRFCAnswersRequire(shiftedA.stdout == shiftedB.stdout,
		"equivalent shifted exports rendered different re-seal bytes\nfirst:\n%s\nsecond:\n%s",
		shiftedA.stdout, shiftedB.stdout)
	shiftedAfterA := leRFCAnswersAuditState(resealTreeA)
	shiftedAfterB := leRFCAnswersAuditState(resealTreeB)
	leRFCAnswersRequire(reflect.DeepEqual(shiftedAfterA, shiftedAfterB),
		"equivalent shifted exports wrote different bytes into rfc/audit")
	leRFCAnswersRequire(!reflect.DeepEqual(shiftedAfterA, shiftedBeforeA),
		"rfc reseal called records shifted but rewrote no rfc/audit bytes")
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(shiftedAfterA), leRFCAnswersMapKeys(shiftedBeforeA)),
		"rfc reseal added or removed an audit file")

	// Once re-sealed, the same shifted export is fresh and a second invocation
	// must be a byte-preserving no-op.
	shiftedFresh := runLE(resealTreeA, "rfc", "reseal")
	leRFCAnswersRequire(shiftedFresh.code == 0 && shiftedFresh.stderr == "",
		"second shifted-export rfc reseal failed with exit %d\nstdout:\n%s\nstderr:\n%s",
		shiftedFresh.code, shiftedFresh.stdout, shiftedFresh.stderr)
	leRFCAnswersRequire(strings.Contains(shiftedFresh.stdout, "nothing to re-seal"),
		"second shifted-export rfc reseal did not report a no-op:\n%s", shiftedFresh.stdout)
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersAuditState(resealTreeA), shiftedAfterA),
		"a second re-seal changed already-fresh audit bytes")

	// update owns the ledger and every requirements shard. Equivalent
	// copies must produce the same page, file set, and bytes; JSON rendering
	// gets a third copy so it cannot read pages from either plain invocation.
	checkoutPages := leRFCAnswersGenerated(root)
	indexTreeA := leRFCAnswersCopyTree(root, here, "index-a")
	indexTreeB := leRFCAnswersCopyTree(root, here, "index-b")
	indexA := runLE(indexTreeA, "rfc", "update")
	indexB := runLE(indexTreeB, "rfc", "update")
	for name, result := range map[string]leRFCAnswersResult{"index-a": indexA, "index-b": indexB} {
		leRFCAnswersRequire(result.code == 0,
			"%s rfc update exited %d\nstdout:\n%s\nstderr:\n%s",
			name, result.code, result.stdout, result.stderr)
		leRFCAnswersRequire(result.stderr == "",
			"%s rfc update wrote to stderr: %q", name, result.stderr)
		leRFCAnswersRequire(!strings.Contains(result.stdout, "\x1b["),
			"%s rfc update emitted terminal escapes: %q", name, result.stdout)
	}
	leRFCAnswersRequire(indexA.stdout == indexB.stdout,
		"equivalent owned copies rendered different rfc update bytes\nfirst:\n%s\nsecond:\n%s",
		indexA.stdout, indexB.stdout)
	pagesA := leRFCAnswersGenerated(indexTreeA)
	pagesB := leRFCAnswersGenerated(indexTreeB)
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(pagesA), []string{"ai/RFC-REQUIREMENTS.md", "rfc/requirements/rfc4456.md"}),
		"rfc update wrote %v, want the ledger and the one authored RFC's shard", leRFCAnswersMapKeys(pagesA))
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(pagesA), leRFCAnswersMapKeys(pagesB)),
		"equivalent index updates wrote different file sets: %v",
		leRFCAnswersSymmetricDifference(leRFCAnswersMapKeys(pagesA), leRFCAnswersMapKeys(pagesB)))
	for name, body := range pagesA {
		leRFCAnswersRequire(bytes.Equal(body, pagesB[name]),
			"equivalent index updates wrote different bytes into %s", name)
	}

	indexJSONTree := leRFCAnswersCopyTree(root, here, "index-json")
	indexJSON := runLE(indexJSONTree, "rfc", "update", "|", "json")
	leRFCAnswersRequire(indexJSON.code == 0,
		"rfc update | json exited %d\nstdout:\n%s\nstderr:\n%s",
		indexJSON.code, indexJSON.stdout, indexJSON.stderr)
	leRFCAnswersRequire(indexJSON.stderr == "",
		"rfc update | json wrote to stderr: %q", indexJSON.stderr)
	indexPayload := leRFCAnswersJSONObject(indexJSON.stdout, "rfc update | json")
	leRFCAnswersRequire(
		reflect.DeepEqual(leRFCAnswersMapKeys(indexPayload), []string{"deleted", "files", "ledger", "shards"}),
		"rfc update | json returned fields %v, want deleted, files, ledger, and shards: %s",
		leRFCAnswersMapKeys(indexPayload), indexJSON.stdout)
	leRFCAnswersRequire(
		leRFCAnswersJSONInt(indexPayload["shards"], "update.shards") == int64(len(pagesA)-1),
		"rfc update reported %d shard(s), but the generated tree holds %d",
		leRFCAnswersJSONInt(indexPayload["shards"], "update.shards"), len(pagesA)-1,
	)

	// The ledger files the report NAMES are held to the same parity as the
	// shards, and the population is read off the report rather than copied into
	// this fixture, so a file the generator adds is covered the day it is added
	// (ai/rules/evidence.md).
	ledgerFiles := leRFCAnswersStrings(indexPayload["files"], "update.files")
	leRFCAnswersRequire(len(ledgerFiles) != 0,
		"rfc update named no ledger file beside its shards: %s", indexJSON.stdout)
	for _, relative := range ledgerFiles {
		written := leRFCAnswersRead(indexJSONTree, relative)
		leRFCAnswersRequire(len(written) != 0,
			"rfc update named %s and wrote nothing there", relative)
		leRFCAnswersRequire(bytes.Equal(written, leRFCAnswersRead(indexTreeA, relative)),
			"equivalent index updates wrote different bytes into %s", relative)
	}

	pagesJSON := leRFCAnswersGenerated(indexJSONTree)
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(pagesJSON), leRFCAnswersMapKeys(pagesA)),
		"JSON-rendered index update wrote a different file set: %v",
		leRFCAnswersSymmetricDifference(leRFCAnswersMapKeys(pagesJSON), leRFCAnswersMapKeys(pagesA)))
	for name, body := range pagesA {
		leRFCAnswersRequire(bytes.Equal(body, pagesJSON[name]),
			"JSON-rendered index update wrote different bytes into %s", name)
	}

	// Every writer targeted a copy; read-only and invalid-argument invocations
	// must leave the baseline's two owned filesystem regions unchanged.
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersGenerated(root), checkoutPages),
		"the fixture changed generated requirement pages in its baseline tree")
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersAuditState(root), checkoutAuditBefore),
		"the fixture changed rfc/audit in its baseline tree")

	fmt.Println("OK")
	return nil
}

// leRFCAnswersTree owns only the inputs needed by the action/pipe contract.
// FixtureFiles is the gate's shared source of valid summary metadata; the
// independent real-corpus gates still judge every production RFC.
func leRFCAnswersTree(ctx context.Context, parent, binary string) string {
	root := filepath.Join(parent, "tree")
	files := make(map[string]string)
	for relative, body := range rfc.FixtureFiles() {
		relative = strings.ReplaceAll(relative, "9999", "4456")
		files[relative] = strings.ReplaceAll(body, "9999", "4456")
	}
	files["go.mod"] = "module fixture.invalid/rfc\n\ngo 1.26\n"
	files["feature-gates.txt"] = "ze_probe\tinternal/widget\n"
	files["internal/widget/widget_test.go"] = "package widget\n\nimport \"testing\"\n\n" +
		"func TestWidget(t *testing.T) {\n" +
		"\t// RFC requirement: RFC4456-2-1 positive\n" +
		"\t// RFC requirement: RFC4456-2-1 negative\n" +
		"\tif !acceptWidget(1) || acceptWidget(0) {\n\t\tt.Fatal(\"widget boundary\")\n\t}\n}\n" +
		"\nfunc acceptWidget(value int) bool { return value == 1 }\n"
	files["pending.json"] = `{"rfc":"rfc4456","requirements":{"RFC4456-2-1":{"verdict":"enforced","note":"The owned test exercises accepted and rejected widget input."}}}`
	for relative, body := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		leRFCAnswersRequireNoError(os.MkdirAll(filepath.Dir(path), 0o750), "create owned RFC directory")
		leRFCAnswersRequireNoError(os.WriteFile(path, []byte(body), 0o600), "write owned RFC input "+relative)
	}
	// Share only Go's content-addressed build artifacts, never an RFC answer.
	// The native package checker otherwise creates a cold cache per copy.
	cache, err := filepath.Abs(filepath.Join(os.Getenv(envRepoRoot), "cache"))
	leRFCAnswersRequireNoError(err, "resolve native build cache")
	leRFCAnswersRequireNoError(os.Symlink(cache, filepath.Join(root, "cache")), "link native build cache")
	// The gate compiles the owned test through gotoolchain.New, which reads
	// the patch set of the checkout it runs in.
	leRFCAnswersRequireNoError(leFixtureStdpatch(root), "give the owned tree the standard-library patch set")
	stamp := leRFCAnswersRun(ctx, parent, map[string]string{envRepoRoot: root}, binary,
		"rfc", "audit-stamp", "stem", "rfc4456", "from", filepath.Join(root, "pending.json"))
	leRFCAnswersRequire(stamp.code == 0 && stamp.stderr == "",
		"stamping the owned RFC audit exited %d\nstdout:\n%s\nstderr:\n%s", stamp.code, stamp.stdout, stamp.stderr)
	return root
}

func leRFCAnswersCopyTree(root, parent, name string) string {
	destination := filepath.Join(parent, name)
	leRFCAnswersRequireNoError(os.MkdirAll(destination, 0o750), "create owned RFC copy")
	entries, err := os.ReadDir(root)
	leRFCAnswersRequireNoError(err, "read owned RFC inputs")
	for _, entry := range entries {
		if entry.Name() == "cache" {
			continue
		}
		target := filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			leRFCAnswersRequireNoError(os.CopyFS(target, os.DirFS(filepath.Join(root, entry.Name()))), "copy owned RFC directory "+entry.Name())
		} else {
			leRFCAnswersRequireNoError(os.WriteFile(target, leRFCAnswersRead(root, entry.Name()), 0o600), "copy owned RFC file "+entry.Name())
		}
	}
	leRFCAnswersRequireNoError(os.Symlink(filepath.Join(root, "cache"), filepath.Join(destination, "cache")), "link copied tree's build cache")
	return destination
}

// The seeded requirement is deliberately distinct from the one authored by
// leRFCAnswersTree. Its id and section agree, so coverage, not shape, refuses it.
const (
	leRFCAnswersSeedSummary = "rfc/short/rfc4456.md"
	leRFCAnswersSeedID      = "RFC4456-8-97"
	leRFCAnswersSeedRow     = "- [ ] [" + leRFCAnswersSeedID +
		"] [MUST] A requirement this case seeded, which no test tags (§8)\n"
)

// leRFCAnswersCheckAnswer is one `rfc check` run: the code it exited with, the
// page a person reads, and the report a program reads.
type leRFCAnswersCheckAnswer struct {
	code       int
	page       string
	violations []string
}

// leRFCAnswersRunCheck holds one tree's check to its contract and answers what
// it reported.
//
// The page and the payload are two renderings of ONE report. The bullets the
// page lists and the rows `violations` carries are therefore compared against
// each other, never asserted apart. `CheckReport.Text` is reached by a type
// assertion on the answer. When it stopped matching, the violation page
// disappeared with no error, no log line and no change of exit code.
func leRFCAnswersRunCheck(
	runLE func(string, ...string) leRFCAnswersResult,
	tree, name string,
) leRFCAnswersCheckAnswer {
	plain := runLE(tree, "rfc", actionCheck)
	leRFCAnswersRequire(plain.code == 0 || plain.code == 1,
		"%s: rfc check exited %d, want 0 or gate code 1\nstdout:\n%s\nstderr:\n%s",
		name, plain.code, plain.stdout, plain.stderr)
	leRFCAnswersRequire(plain.stderr == "", "%s: rfc check wrote to stderr: %q", name, plain.stderr)
	leRFCAnswersRequire(!strings.Contains(plain.stdout, "\x1b["),
		"%s: rfc check emitted terminal escapes: %q", name, plain.stdout)

	structured := runLE(tree, "rfc", actionCheck, "|", renderJSON)
	leRFCAnswersRequire(structured.code == plain.code,
		"%s: rfc check | json changed action exit %d to %d\nstdout:\n%s\nstderr:\n%s",
		name, plain.code, structured.code, structured.stdout, structured.stderr)
	leRFCAnswersRequire(structured.stderr == "",
		"%s: rfc check | json wrote to stderr: %q", name, structured.stderr)
	report := leRFCAnswersJSONObject(structured.stdout, name+": rfc check | json")
	leRFCAnswersRequireDiscriminationKeys(report, name, structured.stdout)
	answer := leRFCAnswersCheckAnswer{code: plain.code, page: plain.stdout}

	if plain.code == 0 {
		leRFCAnswersRequire(strings.HasPrefix(plain.stdout, "rfc-requirements OK:"),
			"%s: a clean check omitted its stable summary:\n%s", name, plain.stdout)
		if _, listed := report["violations"]; listed {
			leRFCAnswersRequire(false,
				"%s: a clean check still carried a violation list: %s", name, structured.stdout)
		}
		for _, field := range []string{
			"gated", "enrolled", "tags", fieldEvidence, "signed",
			"signed-by-register", "audit-verdicts", "audit-done", "audit-total",
		} {
			if _, present := report[field]; !present {
				leRFCAnswersRequire(false,
					"%s: a clean check | json omitted %q: %s", name, field, structured.stdout)
			}
		}
		return answer
	}

	// Gate code 1 is a judged tree with violations. The caller of this helper asked for the
	// tree to be JUDGED. A tree the gate cannot READ is named here, never read
	// as an empty violation list. It is the refusal the unreadable tree above
	// is held to.
	if reason, refused := report["cannot-run"].(string); refused {
		leRFCAnswersRequire(false, "%s: the gate could not run over this tree: %s", name, reason)
	}
	answer.violations = leRFCAnswersStrings(report["violations"], name+": rfc check | json violations")
	leRFCAnswersRequire(len(answer.violations) != 0,
		"%s: a red check answered no violation row: %s", name, structured.stdout)
	var tb textbuf.Buffer
	header := tb.Str("rfc-requirements: ").Int(int64(len(answer.violations))).Str(" violation(s)").String()
	leRFCAnswersRequire(strings.HasPrefix(plain.stdout, header),
		"%s: the page does not open with %q:\n%s", name, header, plain.stdout)
	bullets := leRFCAnswersBullets(plain.stdout)
	leRFCAnswersRequire(slices.Equal(bullets, answer.violations),
		"%s: the page lists %d violation(s) and the payload carries %d, "+
			"so the two are no longer renderings of one report\npage:\n%s",
		name, len(bullets), len(answer.violations), plain.stdout)
	return answer
}

// leRFCAnswersRequireDiscriminationKeys checks the six figures that render at
// zero. They are published DEBT, so an absent key would let a consumer read
// "nothing is proven yet" as "this gate has no such stage".
func leRFCAnswersRequireDiscriminationKeys(report map[string]any, name, document string) {
	for _, field := range []string{
		"discrimination-proven", "discrimination-owed", "discrimination-escaped",
		"discrimination-changed", "discrimination-backlog", "discrimination-unresolved",
	} {
		if _, present := report[field]; !present {
			leRFCAnswersRequire(false,
				"%s: rfc check | json omitted %q: %s", name, field, document)
		}
	}
}

// leRFCAnswersBullets answers the violations a check page lists, in order.
func leRFCAnswersBullets(page string) []string {
	var rows []string
	for line := range strings.SplitSeq(page, "\n") {
		if rest, found := strings.CutPrefix(line, "  * "); found {
			rows = append(rows, rest)
		}
	}
	return rows
}

// leRFCAnswersRead answers one file of a tree the fixture owns.
func leRFCAnswersRead(tree, relative string) []byte {
	path := filepath.Join(tree, filepath.FromSlash(relative))
	body, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own export
	leRFCAnswersRequireNoError(err, "read "+relative)
	return body
}

// leRFCAnswersStrings answers one JSON list of strings.
func leRFCAnswersStrings(value any, description string) []string {
	rows, ok := value.([]any)
	leRFCAnswersRequire(ok, "%s is not a list: %#v", description, value)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		text, ok := row.(string)
		leRFCAnswersRequire(ok, "%s holds a non-string row: %#v", description, row)
		out = append(out, text)
	}
	return out
}

// leRFCAnswersSeedRequirement writes one gated MUST that no test tags into a
// tree the fixture owns.
func leRFCAnswersSeedRequirement(tree string) {
	path := filepath.Join(tree, filepath.FromSlash(leRFCAnswersSeedSummary))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0) //nolint:gosec // the path is the fixture's own export
	leRFCAnswersRequireNoError(err, "open "+leRFCAnswersSeedSummary)
	_, writeErr := file.WriteString(leRFCAnswersSeedRow)
	closeErr := file.Close()
	leRFCAnswersRequireNoError(writeErr, "seed a requirement into "+leRFCAnswersSeedSummary)
	leRFCAnswersRequireNoError(closeErr, "close "+leRFCAnswersSeedSummary)
}

func leRFCAnswersRequire(condition bool, format string, args ...any) {
	if condition {
		return
	}
	panic(leRFCAnswersFailure{message: fmt.Sprintf("FAIL: "+format, args...)})
}

func leRFCAnswersRequireNoError(err error, operation string) {
	if err != nil {
		panic(leRFCAnswersFailure{message: fmt.Sprintf("FAIL: %s: %v", operation, err)})
	}
}

func leRFCAnswersRun(
	ctx context.Context,
	cwd string,
	overrides map[string]string,
	name string,
	args ...string,
) leRFCAnswersResult {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = cwd
	cmd.Env = leRFCAnswersEnvironment(overrides)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exit.ExitCode()
		} else {
			leRFCAnswersRequire(false, "execute %s: %v", name, err)
		}
	}
	if ctx.Err() != nil {
		leRFCAnswersRequire(false, "execute %s: %v", name, ctx.Err())
	}
	return leRFCAnswersResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func leRFCAnswersEnvironment(overrides map[string]string) []string {
	baseEnv := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			baseEnv = append(baseEnv, entry)
		}
	}
	return childEnvironment(baseEnv, overrides)
}

func leRFCAnswersJSON(text, description string) any {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	leRFCAnswersRequireNoError(decoder.Decode(&value), "decode "+description)
	var trailing any
	err := decoder.Decode(&trailing)
	leRFCAnswersRequire(err == io.EOF, "%s contains trailing non-whitespace data: %q", description, text)
	return value
}

func leRFCAnswersJSONObject(text, description string) map[string]any {
	value, ok := leRFCAnswersJSON(text, description).(map[string]any)
	leRFCAnswersRequire(ok, "%s did not return a JSON object: %s", description, text)
	return value
}

func leRFCAnswersJSONInt(value any, description string) int64 {
	number, ok := value.(json.Number)
	leRFCAnswersRequire(ok, "%s is not an integer: %#v", description, value)
	integer, err := number.Int64()
	leRFCAnswersRequireNoError(err, "decode integer "+description)
	return integer
}

func leRFCAnswersMapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// leRFCAnswersAuditState answers every verdict file under rfc/audit, keyed
// by "<stem>/<requirement-id>.json": one file per requirement.
func leRFCAnswersAuditState(tree string) map[string][]byte {
	directory := filepath.Join(tree, "rfc", "audit")
	stems, err := os.ReadDir(directory)
	leRFCAnswersRequireNoError(err, "read "+directory)
	state := map[string][]byte{}
	for _, stem := range stems {
		leRFCAnswersRequire(stem.IsDir(), "unexpected file in rfc/audit: %s", stem.Name())
		files, err := os.ReadDir(filepath.Join(directory, stem.Name()))
		leRFCAnswersRequireNoError(err, "read rfc/audit/"+stem.Name())
		for _, file := range files {
			name := stem.Name() + "/" + file.Name()
			body, err := os.ReadFile(filepath.Join(directory, stem.Name(), file.Name())) //nolint:gosec // the path is the fixture's own scratch file
			leRFCAnswersRequireNoError(err, "read rfc/audit/"+name)
			state[name] = body
		}
	}
	return state
}

func leRFCAnswersCitedFiles(audits map[string][]byte) []string {
	cited := map[string]struct{}{}
	for name, body := range audits {
		var verdict struct {
			Tests map[string]json.RawMessage `json:"tests"`
		}
		leRFCAnswersRequireNoError(json.Unmarshal(body, &verdict), "decode rfc/audit/"+name)
		for key := range verdict.Tests {
			path, _, _ := strings.Cut(key, "::")
			path, _, _ = strings.Cut(path, "#")
			leRFCAnswersRequire(path != "", "empty cited path in rfc/audit/%s", name)
			cited[path] = struct{}{}
		}
	}
	return leRFCAnswersMapKeys(cited)
}

func leRFCAnswersGenerated(tree string) map[string][]byte {
	generated := map[string][]byte{}
	ledgerPath := filepath.Join(tree, "ai", "RFC-REQUIREMENTS.md")
	if body, err := os.ReadFile(ledgerPath); err == nil { //nolint:gosec // the path is the fixture's own scratch file
		generated["ai/RFC-REQUIREMENTS.md"] = body
	} else if !os.IsNotExist(err) {
		leRFCAnswersRequireNoError(err, "read "+ledgerPath)
	}
	// rfc/requirements is derived on demand and ignored by git, so a checkout
	// that never ran rfc update holds no directory. Absence is a state the
	// caller compares like any other: a writer that fills the directory still
	// changes the answer.
	shardsPath := filepath.Join(tree, "rfc", "requirements")
	entries, err := os.ReadDir(shardsPath)
	if os.IsNotExist(err) {
		return generated
	}
	leRFCAnswersRequireNoError(err, "read "+shardsPath)
	for _, entry := range entries {
		leRFCAnswersRequire(!entry.IsDir(),
			"unexpected directory in rfc/requirements: %s", entry.Name())
		body, err := os.ReadFile(filepath.Join(shardsPath, entry.Name())) //nolint:gosec // the path is the fixture's own scratch file
		leRFCAnswersRequireNoError(err, "read rfc/requirements/"+entry.Name())
		generated[filepath.ToSlash(filepath.Join("rfc", "requirements", entry.Name()))] = body
	}
	return generated
}

func leRFCAnswersShift(tree string, paths []string) {
	const suffix = "\n// a line the parity test appended, below every function\n"
	for _, relative := range paths {
		path := filepath.Join(tree, filepath.FromSlash(relative))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0) //nolint:gosec // the path is the fixture's own scratch file
		leRFCAnswersRequireNoError(err, "open shifted file "+relative)
		_, writeErr := file.WriteString(suffix)
		closeErr := file.Close()
		leRFCAnswersRequireNoError(writeErr, "append shifted line to "+relative)
		leRFCAnswersRequireNoError(closeErr, "close shifted file "+relative)
	}
}

func leRFCAnswersExportHEAD(ctx context.Context, root, parent, name string) string {
	destination := filepath.Join(parent, name)
	leRFCAnswersRequireNoError(os.RemoveAll(destination), "remove earlier export "+name)
	leRFCAnswersRequireNoError(os.MkdirAll(destination, 0o750), "create export "+name)
	archive := leRFCAnswersRun(ctx, root, nil, "git", "-C", root, "archive", "HEAD")
	leRFCAnswersRequire(archive.code == 0,
		"git archive HEAD exited %d\nstdout:\n%s\nstderr:\n%s",
		archive.code, archive.stdout, archive.stderr)

	reader := tar.NewReader(strings.NewReader(archive.stdout))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		leRFCAnswersRequireNoError(err, "read git archive")
		path := leRFCAnswersArchivePath(destination, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			leRFCAnswersRequireNoError(os.MkdirAll(path, os.FileMode(header.Mode)), "create "+header.Name)
		case tar.TypeReg:
			leRFCAnswersRequireNoError(os.MkdirAll(filepath.Dir(path), 0o750), "create parent for "+header.Name)
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)) //nolint:gosec // the path is the fixture's own scratch file
			leRFCAnswersRequireNoError(err, "create "+header.Name)
			_, copyErr := io.Copy(file, reader) //nolint:gosec // the archive is the fixture's own git export
			closeErr := file.Close()
			leRFCAnswersRequireNoError(copyErr, "extract "+header.Name)
			leRFCAnswersRequireNoError(closeErr, "close "+header.Name)
		case tar.TypeSymlink:
			// A link target is a second path into the export, so it is held to
			// the same containment as the entry name. The helper fails the
			// fixture when the joined path leaves the destination.
			leRFCAnswersRequire(!filepath.IsAbs(header.Linkname), "absolute symlink target %q", header.Linkname)
			leRFCAnswersArchivePath(destination,
				filepath.Join(filepath.Dir(filepath.FromSlash(header.Name)), filepath.FromSlash(header.Linkname)))
			leRFCAnswersRequireNoError(os.MkdirAll(filepath.Dir(path), 0o750), "create parent for "+header.Name)
			leRFCAnswersRequireNoError(os.Symlink(header.Linkname, path), "create symlink "+header.Name)
		case tar.TypeLink:
			target := leRFCAnswersArchivePath(destination, header.Linkname)
			leRFCAnswersRequireNoError(os.MkdirAll(filepath.Dir(path), 0o750), "create parent for "+header.Name)
			leRFCAnswersRequireNoError(os.Link(target, path), "create hard link "+header.Name)
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			// archive/tar applies these records to the following header.
		default:
			leRFCAnswersRequire(false, "unsupported archive entry %q with type %d", header.Name, header.Typeflag)
		}
	}
	return destination
}

func leRFCAnswersArchivePath(root, name string) string {
	clean := filepath.Clean(filepath.FromSlash(name))
	leRFCAnswersRequire(clean != "." && clean != "", "invalid empty archive path %q", name)
	leRFCAnswersRequire(!filepath.IsAbs(clean), "absolute archive path %q", name)
	leRFCAnswersRequire(clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)),
		"archive path escapes export: %q", name)
	path := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, path)
	leRFCAnswersRequireNoError(err, "validate archive path "+name)
	leRFCAnswersRequire(relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)),
		"archive path escapes export: %q", name)
	return path
}

func leRFCAnswersSymmetricDifference(a, b []string) []string {
	left := make(map[string]bool, len(a))
	right := make(map[string]bool, len(b))
	for _, value := range a {
		left[value] = true
	}
	for _, value := range b {
		right[value] = true
	}
	var difference []string
	for value := range left {
		if !right[value] {
			difference = append(difference, value)
		}
	}
	for value := range right {
		if !left[value] {
			difference = append(difference, value)
		}
	}
	slices.Sort(difference)
	if len(difference) > 10 {
		difference = difference[:10]
	}
	return difference
}

// The partial-proof fixture seeds one scoped row into a private HEAD export.
// Every `le rfc check` it runs walks that whole corpus, so its proofs are four
// fixtures rather than one: run as one, its ten passes, the first over a cold
// private build cache, took 554s on a loaded CI runner and the ui suite's 600s
// budget killed it. Each fixture seeds its own export, so they run in parallel
// and none reads another's tree; they share only the build cache.
const (
	leRFCPartialStem     = "rfc99999"
	leRFCPartialRID      = "RFC99999-2-1"
	leRFCPartialSummary  = "rfc/short/" + leRFCPartialStem + ".md"
	leRFCPartialShard    = "rfc/requirements/" + leRFCPartialStem + ".md"
	leRFCPartialProducer = "internal/le/rfc/ui_partial_fixture.go::UIPartialGapContext"
	leRFCPartialGap      = "MUST be zero when sent"
	leRFCPartialTested   = "ignored on receipt."
	leRFCPartialSentence = "A widget MUST be zero when sent and ignored on receipt."
	leRFCPartialMarker   = `{partial: tested "ignored on receipt."; gap "MUST be zero when sent"; native sending is absent at ` + leRFCPartialProducer + `}`
	leRFCPartialMeta     = "# RFC 99999\n\n## Meta\n\n| Field | Value |\n|---|---|\n" +
		"| Title | Scoped Widget Fixture |\n| Enrolment | enrolled |\n" +
		"| Enrolment reason | isolated tooling fixture |\n| Implementation | ze |\n" +
		"| Implementation reason | the fixture's own Go boundary |\n| Support | bgp-core |\n" +
		"| Support area | Scoped Widget Fixture |\n| Support status | Partial |\n" +
		"| Support coverage | scoped receive assertions |\n| Support remaining | One MUST row remains unmet: native sending is absent. |\n" +
		"\n## Compliance Checklist\n\n"
	leRFCPartialRow = "- [ ] [" + leRFCPartialRID + "] [MUST] " + leRFCPartialSentence + " (§2) " + leRFCPartialMarker + "\n"
)

// leRFCPartialTree is one fixture's private HEAD export and the shipped CLI
// that runs over it.
type leRFCPartialTree struct {
	tree  string
	runLE func(args ...string) leRFCAnswersResult
}

// write replaces one file of the export.
func (t leRFCPartialTree) write(rel, text string) {
	path := filepath.Join(t.tree, filepath.FromSlash(rel))
	leRFCAnswersRequireNoError(os.MkdirAll(filepath.Dir(path), 0o750), "create fixture parent")
	leRFCAnswersRequireNoError(os.WriteFile(path, []byte(text), 0o600), "write scoped fixture "+rel)
}

// seed writes the scoped row, its RFC text, its producer and its tagged test.
func (t leRFCPartialTree) seed() {
	_, statErr := os.Stat(filepath.Join(t.tree, leRFCPartialSummary))
	leRFCAnswersRequire(os.IsNotExist(statErr), "seed summary already exists: %v", statErr)
	t.write(leRFCPartialSummary, leRFCPartialMeta+leRFCPartialRow)
	t.write("rfc/full/"+leRFCPartialStem+".txt", "Test RFC 99999\n\n2.  Widgets\n\n"+leRFCPartialSentence+"\n")
	t.write("internal/le/rfc/ui_partial_fixture.go", "package rfc\n\nfunc UIPartialGapContext() {}\n")
	t.write("test/plugin/ui-partial-fixture.ci", "# RFC requirement: "+leRFCPartialRID+" positive -- accepts receipt of zero\n"+
		"# RFC requirement: "+leRFCPartialRID+" negative -- ignores nonzero on receipt\nexpect=stdout:contains=OK\n")
}

// checkJSON runs `le rfc check | json` and returns its exit code and report.
func (t leRFCPartialTree) checkJSON(what string) (int, map[string]any) {
	check := t.runLE("rfc", actionCheck, "|", renderJSON)
	leRFCAnswersRequire(check.code == 0 || check.code == 1, "%s did not answer a gate result: %s", what, check.stderr)
	report := leRFCAnswersJSONObject(check.stdout, what)
	leRFCAnswersRequire(report["cannot-run"] == nil, "%s could not run: %v", what, report["cannot-run"])
	return check.code, report
}

// indexShard runs rfc update over the valid seeded row and returns the shard.
func (t leRFCPartialTree) indexShard() []byte {
	index := t.runLE("rfc", actionUpdate)
	leRFCAnswersRequire(index.code == 0, "rfc update refused valid scope: %s", index.stderr)
	shard := leRFCAnswersRead(t.tree, leRFCPartialShard)
	for _, fact := range []string{leRFCPartialRID, leRFCPartialTested, leRFCPartialGap, "zero whole-requirement credit"} {
		leRFCAnswersRequire(strings.Contains(string(shard), fact), "shard omitted %q", fact)
	}
	return shard
}

// refuses proves both gates refuse an invalid row and leave the shard as it was.
func (t leRFCPartialTree) refuses(shard []byte, row, refusal string) {
	t.write(leRFCPartialSummary, leRFCPartialMeta+row)
	bad := t.runLE("rfc", actionCheck, "|", renderJSON)
	leRFCAnswersRequire(bad.code == 1 && strings.Contains(bad.stdout, refusal), "invalid scope was accepted: %s", bad.stdout)
	badIndex := t.runLE("rfc", actionUpdate)
	leRFCAnswersRequire(badIndex.code == 2, "rfc update accepted invalid scope")
	unchanged := leRFCAnswersRead(t.tree, leRFCPartialShard)
	leRFCAnswersRequire(bytes.Equal(shard, unchanged), "refused rfc update changed shard")
}

// leRFCPartialDriver turns a failure panic into the fixture's error and hands
// the body a fresh private export of HEAD. The body exercises the shipped CLI,
// never the shared checkout's authored summaries or audits, so ambient corpus
// debt cannot satisfy the seeded RID.
func leRFCPartialDriver(name string, body func(leRFCPartialTree) string) func(context.Context) error {
	return func(ctx context.Context) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if failure, ok := recovered.(leRFCAnswersFailure); ok {
					err = failure
					return
				}
				panic(recovered)
			}
		}()
		root := os.Getenv(envRepoRoot)
		leRFCAnswersRequire(root != "", "ZE_REPO_ROOT is not set")
		here, _, err := temporaryLEFixtureWorkspace("le-rfc-" + name + "-")
		leRFCAnswersRequireNoError(err, "create scoped fixture workspace")
		defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup
		binary, err := nativeLEBinary()
		leRFCAnswersRequireNoError(err, "locate native le binary")
		tree := leRFCAnswersExportHEAD(ctx, root, here, "scoped")
		// Share Go's content-addressed build artifacts, as leRFCAnswersTree
		// does. `le rfc check` vets every tagged package, and a private cache
		// builds a patched GOROOT and the whole tree cold in every export:
		// four exports did that at GOMAXPROCS=1 and none finished in 675s.
		cache, err := filepath.Abs(filepath.Join(root, "cache"))
		leRFCAnswersRequireNoError(err, "resolve native build cache")
		leRFCAnswersRequireNoError(os.Symlink(cache, filepath.Join(tree, "cache")), "link native build cache")
		fmt.Println(body(leRFCPartialTree{tree: tree, runLE: func(args ...string) leRFCAnswersResult {
			return leRFCAnswersRun(ctx, here, map[string]string{envRepoRoot: tree}, binary, args...)
		}}))
		return nil
	}
}

// leRFCPartialProof proves the seeded row adds exactly one partial scope, is
// not refused, and discloses every fact of its scope in the JSON answer.
func leRFCPartialProof(t leRFCPartialTree) string {
	_, baselineReport := t.checkJSON("baseline scoped count")
	beforePartial := leRFCAnswersJSONInt(baselineReport["partial"], "baseline partial count")
	t.seed()
	_, report := t.checkJSON("scoped check")
	leRFCAnswersRequire(leRFCAnswersJSONInt(report["partial"], "partial count") == beforePartial+1,
		"seeded partial row did not increase the subset by exactly one: before=%d after=%v violations=%v",
		beforePartial, report["partial"], report["violations"])
	violations, _ := report["violations"].([]any)
	for _, violation := range violations {
		leRFCAnswersRequire(!strings.Contains(fmt.Sprint(violation), leRFCPartialRID), "valid scoped row refused: %v", violation)
	}
	scopes, ok := report["partial-scopes"].([]any)
	leRFCAnswersRequire(ok, "partial scopes absent from JSON")
	found := false
	for _, scope := range scopes {
		encoded, encodeErr := json.Marshal(scope)
		leRFCAnswersRequireNoError(encodeErr, "encode scope")
		if strings.Contains(string(encoded), leRFCPartialRID) {
			found = true
			for _, fact := range []string{leRFCPartialSentence, leRFCPartialTested, leRFCPartialGap, leRFCPartialProducer} {
				leRFCAnswersRequire(strings.Contains(string(encoded), fact), "scope omits %q: %s", fact, encoded)
			}
		}
	}
	leRFCAnswersRequire(found, "seeded scope absent from JSON")
	return "OK: partial scope stays source-bound and is counted once"
}

// leRFCPartialProofRender proves every renderer of `le rfc check` discloses
// the scoped facts with the JSON answer's exit code, and that audit-stamp
// refuses a whole-requirement verdict for a partial row.
func leRFCPartialProofRender(t leRFCPartialTree) string {
	t.seed()
	code, _ := t.checkJSON("scoped check")
	for _, operator := range []string{"", renderYAML, renderTable} {
		args := []string{"rfc", actionCheck}
		if operator != "" {
			args = append(args, "|", operator)
		}
		result := t.runLE(args...)
		leRFCAnswersRequire(result.code == code, "pipe changed check exit: %s", operator)
		for _, fact := range []string{leRFCPartialRID, leRFCPartialTested, leRFCPartialGap} {
			leRFCAnswersRequire(strings.Contains(result.stdout, fact), "%s omitted scoped fact %q", operator, fact)
		}
	}
	t.write("scratch/pending-partial.json", `{"rfc":"`+leRFCPartialStem+`","requirements":{"`+leRFCPartialRID+`":{"verdict":"enforced","note":"whole sentence supposedly proven"}}}`)
	stamp := t.runLE("rfc", "audit-stamp", "stem", leRFCPartialStem, "from", filepath.Join(t.tree, "scratch", "pending-partial.json"))
	leRFCAnswersRequire(stamp.code == 2 && strings.Contains(stamp.stderr+stamp.stdout, "cannot be 'enforced'"), "whole verdict accepted: %s%s", stamp.stdout, stamp.stderr)
	_, statErr := os.Stat(filepath.Join(t.tree, "rfc/audit/"+leRFCPartialStem+"/"+leRFCPartialRID+".json"))
	leRFCAnswersRequire(os.IsNotExist(statErr), "refused stamp wrote audit")
	return "OK: every renderer discloses the partial scope, and it stays outside whole proof"
}

// leRFCPartialProofSelector proves a tested selector absent from the parent
// sentence is refused by both gates, which leave the shard unchanged.
func leRFCPartialProofSelector(t leRFCPartialTree) string {
	t.seed()
	shard := t.indexShard()
	t.refuses(shard, strings.Replace(leRFCPartialRow, `tested "ignored on receipt."`, `tested "not in the parent"`, 1), "selector")
	return "OK: a selector outside the parent sentence is refused"
}

// leRFCPartialProofMalformed proves a malformed {partial} marker is refused
// by both gates, which leave the shard unchanged.
func leRFCPartialProofMalformed(t leRFCPartialTree) string {
	t.seed()
	shard := t.indexShard()
	t.refuses(shard, strings.Replace(strings.Replace(leRFCPartialRow, "{partial", "{ partial", 1), "native sending", "missing {native} sending", 1), "malformed {partial}")
	return "OK: a malformed partial marker is refused"
}
