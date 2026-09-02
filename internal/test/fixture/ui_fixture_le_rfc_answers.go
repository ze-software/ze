package fixture

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	Register("ui/le-rfc-answers", uiDriver(leRFCAnswers))
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

	root := os.Getenv("ZE_REPO_ROOT")
	leRFCAnswersRequire(root != "", "ZE_REPO_ROOT is not set")
	root, err = filepath.Abs(root)
	leRFCAnswersRequireNoError(err, "resolve ZE_REPO_ROOT")
	info, err := os.Stat(root)
	leRFCAnswersRequireNoError(err, "stat ZE_REPO_ROOT")
	leRFCAnswersRequire(info.IsDir(), "ZE_REPO_ROOT is not a directory: %s", root)

	here, _, err := temporaryLEFixtureWorkspace("le-rfc-answers-")
	leRFCAnswersRequireNoError(err, "create fixture directory")
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := uiLEBinary(root)
	leRFCAnswersRequireNoError(err, "locate native le binary")

	runLE := func(tree string, args ...string) leRFCAnswersResult {
		return leRFCAnswersRun(ctx, here, map[string]string{envRepoRoot: tree}, binary, args...)
	}

	// The extraction envelope is a read-only JSON document derived from the
	// checkout, and all three registers are published even when one is empty.
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
	leRFCAnswersRequire(enrolled > 100,
		"rfc extraction-status read only %d enrolled RFC(s)", enrolled)
	leRFCAnswersRequire(signed+backlog == enrolled,
		"signed (%d) and backlog (%d) do not cover enrolled (%d): %s",
		signed, backlog, enrolled, extraction.stdout)
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

	// The public check has three answers. Each one is driven from a tree this
	// case controls. The SHARED checkout reports whatever backlog is standing
	// that day. An assertion that the gate found SOMETHING is met by that
	// backlog, so it stays green when the gate goes blind. The checkout is
	// therefore held to the contract alone, and the discrimination comes from
	// a violation the fixture writes itself.
	check := leRFCAnswersRunCheck(runLE, root, "checkout")

	// A tree the gate cannot READ answers `cannot run` and exits 2. Clean and
	// unreadable MUST NOT render the same. A gate that never ran, reported as
	// a pass, is the silent zero this repository bans
	// (ai/rules/principles.md).
	unreadable := filepath.Join(here, "unreadable")
	leRFCAnswersRequireNoError(os.MkdirAll(unreadable, 0o750), "create the unreadable tree")
	blind := runLE(unreadable, "rfc", actionCheck)
	leRFCAnswersRequire(blind.code == 2,
		"a tree the gate cannot read exited %d, want gate code 2\nstdout:\n%s\nstderr:\n%s",
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

	// One export carries both halves of the discrimination. The only difference
	// between the two runs below is the line the fixture wrote. The seeded run
	// gives this case a red it reaches on its own. A checkout-wide count is
	// satisfied by ambient debt. A violation this fixture put there is
	// satisfied by nothing else.
	seedTree := leRFCAnswersExportHEAD(ctx, root, here, "check-seed")
	clean := leRFCAnswersRunCheck(runLE, seedTree, "clean export")
	leRFCAnswersRequire(!strings.Contains(clean.page, leRFCAnswersSeedID),
		"a clean export already names %s, so seeding it proves nothing:\n%s",
		leRFCAnswersSeedID, clean.page)

	leRFCAnswersSeedRequirement(seedTree)
	seeded := leRFCAnswersRunCheck(runLE, seedTree, "seeded export")
	leRFCAnswersRequire(seeded.code == 2,
		"one gated MUST that no test tags exited %d, want gate code 2:\n%s",
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

	// The listing is itself part of the public contract: every action is
	// present, in published order, and exactly the mutating ones are marked as
	// writers.
	listing := runLE(root, "rfc")
	leRFCAnswersRequire(listing.code == 0,
		"rfc listing exited %d\nstdout:\n%s\nstderr:\n%s",
		listing.code, listing.stdout, listing.stderr)
	leRFCAnswersRequire(listing.stderr == "", "rfc listing wrote to stderr: %q", listing.stderr)
	listed := map[string]string{}
	lines := strings.Split(listing.stdout, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			listed[fields[0]] = fields[1]
		}
	}
	wantListed := map[string]string{
		"extraction-create":   wordWrites,
		"extraction-status":   fieldChecks,
		"tagged-scope":        fieldChecks,
		"discriminate":        fieldChecks,
		"discriminate-record": wordWrites,
		actionCheck:           fieldChecks,
		actionSelftest:        fieldChecks,
		"reseal":              wordWrites,
		actionIndexUpdate:     wordWrites,
	}
	leRFCAnswersRequire(reflect.DeepEqual(listed, wantListed),
		"rfc listing does not name nine actions with exactly four writers:\n%s", listing.stdout)
	for _, action := range []string{"extraction-status", actionCheck, actionSelftest, "reseal", actionIndexUpdate} {
		refused := runLE(root, "rfc", action, "rfc7606")
		leRFCAnswersRequire(refused.code == 2,
			"rfc %s accepted a stem argument or exited %d\nstdout:\n%s\nstderr:\n%s",
			action, refused.code, refused.stdout, refused.stderr)
		leRFCAnswersRequire(strings.Contains(refused.stderr, "takes no arguments"),
			"rfc %s refusal omitted 'takes no arguments': %q", action, refused.stderr)
	}
	for _, action := range []string{"extraction-create", "tagged-scope"} {
		refused := runLE(root, "rfc", action, "rfc7606")
		leRFCAnswersRequire(refused.code == 2,
			"rfc %s accepted an unkeyed value or exited %d\nstdout:\n%s\nstderr:\n%s",
			action, refused.code, refused.stdout, refused.stderr)
		leRFCAnswersRequire(strings.Contains(refused.stderr, "keyword"),
			"rfc %s refusal omitted the keyword grammar: %q", action, refused.stderr)
	}

	// A no-op re-seal must not alter any byte in the shared checkout. Both the
	// plain and JSON renderings execute the writer and therefore are covered by
	// snapshots taken before and after all of those invocations.
	checkoutAuditBefore := leRFCAnswersAuditState(root)
	reseal := runLE(root, "rfc", "reseal")
	leRFCAnswersRequire(reseal.code == 0,
		"rfc reseal refused this checkout with exit %d\nstdout:\n%s\nstderr:\n%s",
		reseal.code, reseal.stdout, reseal.stderr)
	leRFCAnswersRequire(reseal.stderr == "", "rfc reseal wrote to stderr: %q", reseal.stderr)
	leRFCAnswersRequire(strings.Contains(reseal.stdout, "nothing to re-seal"),
		"fresh-checkout rfc reseal omitted its no-op verdict:\n%s", reseal.stdout)
	leRFCAnswersRequire(!strings.Contains(reseal.stdout, "\x1b["),
		"rfc reseal emitted terminal escapes: %q", reseal.stdout)

	resealAgain := runLE(root, "rfc", "reseal")
	leRFCAnswersRequire(resealAgain.code == 0 && resealAgain.stderr == "",
		"second fresh-checkout rfc reseal failed with exit %d\nstdout:\n%s\nstderr:\n%s",
		resealAgain.code, resealAgain.stdout, resealAgain.stderr)
	leRFCAnswersRequire(resealAgain.stdout == reseal.stdout,
		"two no-op re-seals rendered different bytes\nfirst:\n%s\nsecond:\n%s",
		reseal.stdout, resealAgain.stdout)

	resealJSON := runLE(root, "rfc", "reseal", "|", "json")
	leRFCAnswersRequire(resealJSON.code == 0,
		"rfc reseal | json exited %d\nstdout:\n%s\nstderr:\n%s",
		resealJSON.code, resealJSON.stdout, resealJSON.stderr)
	leRFCAnswersRequire(resealJSON.stderr == "",
		"rfc reseal | json wrote to stderr: %q", resealJSON.stderr)
	resealPayload := leRFCAnswersJSONObject(resealJSON.stdout, "rfc reseal | json")
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(resealPayload), []string{"refused", "resealed"}),
		"rfc reseal | json returned fields %v, want refused and resealed: %s",
		leRFCAnswersMapKeys(resealPayload), resealJSON.stdout)
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersAuditState(root), checkoutAuditBefore),
		"a no-op re-seal changed rfc/audit in the shared checkout")

	// Build the set of tagged files from the audit records. Appending below all
	// functions preserves units while changing every cited file's whole-file
	// identity, forcing all unit-bearing verdicts into the re-sealable state.
	cited := leRFCAnswersCitedFiles(checkoutAuditBefore)
	leRFCAnswersRequire(len(cited) >= 10,
		"HEAD audit records cite only %d tagged file(s)", len(cited))

	resealTreeA := leRFCAnswersExportHEAD(ctx, root, here, "reseal-a")
	resealTreeB := leRFCAnswersExportHEAD(ctx, root, here, "reseal-b")
	leRFCAnswersShift(resealTreeA, cited)
	leRFCAnswersShift(resealTreeB, cited)
	shiftedBeforeA := leRFCAnswersAuditState(resealTreeA)
	shiftedBeforeB := leRFCAnswersAuditState(resealTreeB)
	leRFCAnswersRequire(reflect.DeepEqual(shiftedBeforeA, shiftedBeforeB),
		"two clean HEAD exports began with different rfc/audit bytes")

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

	// index-update owns the ledger and every requirements shard. It runs only
	// over clean HEAD exports. Equivalent exports must produce the same page,
	// file set, and bytes; JSON rendering gets a third export so it cannot read
	// pages written by either plain invocation.
	checkoutPages := leRFCAnswersGenerated(root)
	indexTreeA := leRFCAnswersExportHEAD(ctx, root, here, "index-a")
	indexTreeB := leRFCAnswersExportHEAD(ctx, root, here, "index-b")
	indexA := runLE(indexTreeA, "rfc", "index-update")
	indexB := runLE(indexTreeB, "rfc", "index-update")
	for name, result := range map[string]leRFCAnswersResult{"index-a": indexA, "index-b": indexB} {
		leRFCAnswersRequire(result.code == 0,
			"%s rfc index-update exited %d\nstdout:\n%s\nstderr:\n%s",
			name, result.code, result.stdout, result.stderr)
		leRFCAnswersRequire(result.stderr == "",
			"%s rfc index-update wrote to stderr: %q", name, result.stderr)
		leRFCAnswersRequire(!strings.Contains(result.stdout, "\x1b["),
			"%s rfc index-update emitted terminal escapes: %q", name, result.stdout)
	}
	leRFCAnswersRequire(indexA.stdout == indexB.stdout,
		"equivalent HEAD exports rendered different index-update bytes\nfirst:\n%s\nsecond:\n%s",
		indexA.stdout, indexB.stdout)
	pagesA := leRFCAnswersGenerated(indexTreeA)
	pagesB := leRFCAnswersGenerated(indexTreeB)
	leRFCAnswersRequire(len(pagesA) > 100,
		"rfc index-update wrote only %d generated file(s)", len(pagesA))
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersMapKeys(pagesA), leRFCAnswersMapKeys(pagesB)),
		"equivalent index updates wrote different file sets: %v",
		leRFCAnswersSymmetricDifference(leRFCAnswersMapKeys(pagesA), leRFCAnswersMapKeys(pagesB)))
	for name, body := range pagesA {
		leRFCAnswersRequire(bytes.Equal(body, pagesB[name]),
			"equivalent index updates wrote different bytes into %s", name)
	}

	indexJSONTree := leRFCAnswersExportHEAD(ctx, root, here, "index-json")
	indexJSON := runLE(indexJSONTree, "rfc", "index-update", "|", "json")
	leRFCAnswersRequire(indexJSON.code == 0,
		"rfc index-update | json exited %d\nstdout:\n%s\nstderr:\n%s",
		indexJSON.code, indexJSON.stdout, indexJSON.stderr)
	leRFCAnswersRequire(indexJSON.stderr == "",
		"rfc index-update | json wrote to stderr: %q", indexJSON.stderr)
	indexPayload := leRFCAnswersJSONObject(indexJSON.stdout, "rfc index-update | json")
	leRFCAnswersRequire(
		reflect.DeepEqual(leRFCAnswersMapKeys(indexPayload), []string{"deleted", "files", "ledger", "shards"}),
		"rfc index-update | json returned fields %v, want deleted, files, ledger, and shards: %s",
		leRFCAnswersMapKeys(indexPayload), indexJSON.stdout)
	leRFCAnswersRequire(
		leRFCAnswersJSONInt(indexPayload["shards"], "index-update.shards") == int64(len(pagesA)-1),
		"rfc index-update reported %d shard(s), but the generated tree holds %d",
		leRFCAnswersJSONInt(indexPayload["shards"], "index-update.shards"), len(pagesA)-1,
	)

	// The ledger files the report NAMES are held to the same parity as the
	// shards, and the population is read off the report rather than copied into
	// this fixture, so a file the generator adds is covered the day it is added
	// (ai/rules/evidence.md).
	ledgerFiles := leRFCAnswersStrings(indexPayload["files"], "index-update.files")
	leRFCAnswersRequire(len(ledgerFiles) != 0,
		"rfc index-update named no ledger file beside its shards: %s", indexJSON.stdout)
	for _, relative := range ledgerFiles {
		written := leRFCAnswersRead(indexJSONTree, relative)
		leRFCAnswersRequire(len(written) != 0,
			"rfc index-update named %s and wrote nothing there", relative)
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

	// Every mutating success above targeted an export. Invocations targeting the
	// shared checkout were either read-only, a verified no-op, or rejected at
	// argument validation, so both owned filesystem regions must remain exact.
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersGenerated(root), checkoutPages),
		"the fixture changed generated requirement pages in the shared checkout")
	leRFCAnswersRequire(reflect.DeepEqual(leRFCAnswersAuditState(root), checkoutAuditBefore),
		"the fixture changed rfc/audit in the shared checkout")

	fmt.Println("OK")
	return nil
}

// The seeded requirement, and the summary that carries it.
//
// A gated MUST that no test tags is the gate's central refusal. It is the one
// violation worth seeding. rfc4456 carries it because its summary is enrolled
// and declares its rows under section 8. The id is anchored to the section its
// own text cites, so a free-form id is refused for its SHAPE before the
// coverage check reads it. 97 sits far above the rows section 8 holds, and the
// clean-export assertion fails loudly if the corpus ever allocates it.
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
	leRFCAnswersRequire(plain.code == 0 || plain.code == 2,
		"%s: rfc check exited %d, want 0 or gate code 2\nstdout:\n%s\nstderr:\n%s",
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
			"gated", "enrolled", "tags", areaEvidence, "signed",
			"signed-by-register", "audit-verdicts", "audit-done", "audit-total",
		} {
			if _, present := report[field]; !present {
				leRFCAnswersRequire(false,
					"%s: a clean check | json omitted %q: %s", name, field, structured.stdout)
			}
		}
		return answer
	}

	// Gate code 2 carries two answers. The caller of this helper asked for the
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
	values := make(map[string]string, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		if key, value, found := strings.Cut(entry, "="); found {
			values[key] = value
		}
	}
	maps.Copy(values, overrides)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	return environment
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
	sort.Strings(keys)
	return keys
}

func leRFCAnswersAuditState(tree string) map[string][]byte {
	directory := filepath.Join(tree, "rfc", "audit")
	entries, err := os.ReadDir(directory)
	leRFCAnswersRequireNoError(err, "read "+directory)
	state := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		leRFCAnswersRequire(!entry.IsDir(), "unexpected directory in rfc/audit: %s", entry.Name())
		body, err := os.ReadFile(filepath.Join(directory, entry.Name())) //nolint:gosec // the path is the fixture's own scratch file
		leRFCAnswersRequireNoError(err, "read rfc/audit/"+entry.Name())
		state[entry.Name()] = body
	}
	return state
}

func leRFCAnswersCitedFiles(audits map[string][]byte) []string {
	cited := map[string]struct{}{}
	for name, body := range audits {
		var document struct {
			Requirements map[string]struct {
				Tests map[string]json.RawMessage `json:"tests"`
			} `json:"requirements"`
		}
		leRFCAnswersRequireNoError(json.Unmarshal(body, &document), "decode rfc/audit/"+name)
		for _, verdict := range document.Requirements {
			for key := range verdict.Tests {
				path, _, _ := strings.Cut(key, "::")
				path, _, _ = strings.Cut(path, "#")
				leRFCAnswersRequire(path != "", "empty cited path in rfc/audit/%s", name)
				cited[path] = struct{}{}
			}
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
	shardsPath := filepath.Join(tree, "rfc", "requirements")
	entries, err := os.ReadDir(shardsPath)
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
	sort.Strings(difference)
	if len(difference) > 10 {
		difference = difference[:10]
	}
	return difference
}
