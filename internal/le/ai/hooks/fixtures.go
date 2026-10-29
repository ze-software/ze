// Design: docs/architecture/core-design.md -- native Go gates run through le
// Overview: hookcheck.go -- selftest orchestration and structured report
//
// Typed fixture data owns the former producer population. Native probes protect
// each category, and a source digest detects drift in the actual hook files.
package aihooks

import (
	stdcontext "context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	"github.com/ze-software/ze/internal/le/hookruntime"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

const (
	fixtureSitesExpected       = 393
	fixtureChecksExpected      = 538
	fixtureUniqueNamesExpected = 538
	fixtureCategoriesExpected  = 24
)

var (
	// Re-sealed 2026-09-25 for the same spec, Phase 2: one site message names
	// `./le ai rules index-update` (since renamed `ai rules update index`).
	// No exit code changed.
	// Re-sealed 2026-09-26: the owner removed the hooks that judged wording. The
	// Stop hook no longer matches phrases, and the agent-brief gate, the LSP gate
	// and the two per-prompt reminders are gone with their fixtures.
	// Re-sealed 2026-10-23 for spec-plan-release-organization: the delegation
	// message names plan/post-release/engineering/spec-fixture.md. No verdict,
	// fixture identity, generator or population changed.
	// Re-sealed 2026-10-23: the spec validator asks for any `./le test` or
	// `./le verify` command rather than `./le verify worktree`, and the site
	// validate-spec-verify-current-fast-accepted names the targeted form.
	// Re-sealed 2026-10-24 for spec-weakening-approval-lives-in-the-commit: the
	// weakened-hatch sites expect the `./le test weakened approve` command in
	// place of a ledger shard row.
	// Re-sealed 2026-10-26 for spec-le-cli-usability D-P4-reseal (owner
	// approved): the rendered-rule site expects `./le ai rules update index`,
	// the Phase 4 spelling of `ai rules index-update`. No exit code changed.
	// Re-sealed 2026-10-26 when test changes moved from the owner to the
	// ze-test-judge agent: the weakened-missing-row site expects
	// `/ze-test-judge` in place of `ask Thomas first`. No exit code changed.
	fixtureSiteDigest = [sha256.Size]byte{
		0xe6, 0x82, 0x7d, 0xc9, 0x12, 0xe2, 0xa6, 0x8b,
		0x30, 0xfa, 0xc0, 0x03, 0x9d, 0xfc, 0x5c, 0xb8,
		0x2d, 0x59, 0x2c, 0xd7, 0x3d, 0x6d, 0x1a, 0x05,
		0x71, 0xde, 0x8c, 0x63, 0x2a, 0x4e, 0xda, 0xe0,
	}
	// Re-sealed 2026-09-25 for the same spec, Phase 2: it hashes the site and
	// category text above, which name the subject-first commands.
	// Re-sealed 2026-09-26: the owner removed the hooks that judged wording. The
	// Stop hook no longer matches phrases, and the agent-brief gate, the LSP gate
	// and the two per-prompt reminders are gone with their fixtures.
	// Re-sealed 2026-10-23 for spec-plan-release-organization: the expanded
	// delegation fixture carries the same relocated spec path as its site.
	// Re-sealed 2026-10-23: the expanded catalog carries the new
	// validate-spec-verify-current-fast-accepted fixture.
	// Re-sealed 2026-10-24 for spec-weakening-approval-lives-in-the-commit: the
	// weakened-hatch fixture tree writes the owner's approval file under the
	// main thread's commit namespace instead of a ledger shard.
	// Re-sealed 2026-10-26 for spec-le-cli-usability D-P4-reseal (owner
	// approved): the expanded rendered-rule fixture carries the same
	// `./le ai rules update index` text as its site.
	// Re-sealed 2026-10-26 when test changes moved from the owner to the
	// ze-test-judge agent: the expanded weakened-missing-row fixture carries
	// the same `/ze-test-judge` needle as its site in place of
	// `ask Thomas first`. No exit code changed.
	fixtureCatalogDigest = [sha256.Size]byte{
		0x6e, 0x16, 0xae, 0x4b, 0xe7, 0xe9, 0xab, 0x87,
		0xae, 0x77, 0xb2, 0x05, 0x65, 0x63, 0x85, 0x9d,
		0xa3, 0x61, 0xb8, 0x2e, 0x14, 0x52, 0xeb, 0x0b,
		0xce, 0x64, 0x35, 0xdd, 0xc4, 0x25, 0x10, 0xe4,
	}
	// Re-sealed 2026-09-24 for spec-le-subject-first-command-tree:
	// the design-ref producer moved from internal/le/consistency to
	// internal/le/doc/consistency. One producer path, no category or verdict.
	// Re-sealed 2026-09-25 for the same spec, Phase 2: the raw-job-admission
	// category's allowed example is `./le test unit core`.
	// Re-sealed 2026-09-26: the owner removed the hooks that judged wording. The
	// Stop hook no longer matches phrases, and the agent-brief gate, the LSP gate
	// and the two per-prompt reminders are gone with their fixtures.
	// Re-sealed 2026-10-23 for spec-plan-release-organization: governed-doc
	// allow/refuse examples use the current post-release/engineering category.
	// The category population and producer mappings are unchanged.
	// Re-sealed 2026-10-23: the validate-spec allow probe names
	// `./le verify current fast` instead of `./le verify worktree`.
	fixtureCategoryDigest = [sha256.Size]byte{
		0xa0, 0xf5, 0xb2, 0xe4, 0x21, 0xa5, 0x2a, 0x9f,
		0x72, 0xa3, 0xbf, 0x09, 0xcb, 0x3c, 0x64, 0x48,
		0x7d, 0xe8, 0xf2, 0x98, 0x4f, 0xa6, 0xe5, 0xf1,
		0x42, 0x7b, 0xd6, 0x84, 0xa0, 0xc8, 0x49, 0x67,
	}
	// Re-sealed 2026-09-25 for spec-le-subject-first-command-tree:
	// the weakened-test action and proposed-edit owners moved from
	// internal/le/testweakened to internal/le/test/weakened. Two owner paths,
	// no boundary or verdict.
	fixtureBoundaryDigest = [sha256.Size]byte{
		0xe7, 0x36, 0xfc, 0xa9, 0x7d, 0x8e, 0xc4, 0x31,
		0x7a, 0x8a, 0xd8, 0xad, 0x70, 0x07, 0x8e, 0xa4,
		0xbc, 0x4d, 0xe3, 0x06, 0xdd, 0x30, 0x9e, 0x40,
		0x53, 0xc2, 0xe6, 0x97, 0xb3, 0x02, 0xa5, 0xb0,
	}
	// Updated 2026-09-11: postInvalidateDerived joined posttool-writeedit and
	// preMaterializeDerived joined pretool-bash, with the two hardcoded
	// os.Stat blocks in hookSessionStart replaced by a loop over
	// internal/le/derived. Neither check judges an edit, so neither adds a
	// refusal this population would describe: each answers by removing or
	// rebuilding a file, and its polarities are driven through hookruntime.Probe
	// in internal/le/hookruntime (postwrite_test.go, bash_test.go) and end to
	// end in internal/le/derived/derived_test.go.
	//
	// Moved again on review: the write check now accumulates its refusals
	// instead of returning at the first, `tracked` fails CLOSED on a stat error
	// that is not ErrNotExist, and the read check matches a directory holding
	// an artifact as well as its full path. Each polarity is driven through
	// Probe in that package's own tests.
	//
	// Moved a third time on review round 2: hookSessionStart went back to
	// rebuilding only an ABSENT artifact, because the hook carries a 5 second
	// timeout in .claude/settings.json. None of that adds a refusal this
	// population describes; TestSessionStartLeavesAPresentArtifactAlone
	// (internal/le/hookruntime/lifecycle_test.go) holds the bound.
	//
	// Moved a fourth time, over the four commits of the derived-indexes spec
	// that landed after the third move: bd11ff12af, 8ae7424594, 6126dad349 and
	// 940fb364fb. Three producer changes and one comment. A read now asks
	// whether the artifact is WHOLE rather than whether it is PRESENT, which is
	// a question a stat cannot settle for a directory (artifactWhole, bash.go).
	// Session start builds only an artifact whose registration says so, because
	// the renders are not the same size and the hook's budget is fixed
	// (derived.SessionStartPolicy, lifecycle.go). An invalidation removes a
	// directory artifact whole, because os.Remove refuses a directory that holds
	// anything and such an artifact would otherwise sit there stale
	// (removeArtifact, postwrite.go). None of the three adds a refusal this
	// population describes, so no fixture is owed.
	// Updated 2026-09-21: the owner removed the review model restriction.
	// TestReviewAgentHasNoModelRestriction covers the dispatcher behavior.
	// Updated 2026-09-24 for b9779d6fbb, which removed the verification-archive
	// exemption from the weakened-test actions, and 90f52dabe9, which makes
	// AGENTS.md the one instruction file. The write hook refused both root
	// instruction files before and still does; only the refusal text for
	// CLAUDE.md changed. Neither adds a refusal this population describes, so
	// no fixture is owed.
	//
	// Re-sealed 2026-09-24 for spec-le-subject-first-command-tree:
	// the hook sources import the moved packages under their new paths
	// (internal/le/ai/sync, internal/le/ai/rules, internal/le/doc/ste,
	// internal/le/doc/index) and name `le ai hooks` and `le ai sync update` in
	// comments and messages. No hook decision changed.
	// Re-sealed 2026-09-25 for the same spec: writeedit.go imports the
	// weakened-test checker from internal/le/test/weakened, under the alias
	// testweakened that goimports writes because the package clause no longer
	// matches the directory name. One import line, no hook decision changed.
	// Re-sealed 2026-09-25 for the same spec, Phase 1c: bash.go admits the
	// renamed harness. harnessRunner treated the harness binary names (and
	// their cross-build suffixes) as the functional runner, and
	// heavyArea reads the subject-first `le test <suite>` spelling, with `test
	// harness` heavy. Pinned by TestBashHookAdmitsLeTest in bash_test.go.
	// Re-sealed 2026-09-25 for the same spec, Phase 2: heavyArea in bash.go
	// treats `le go lint` as heavy (the new name of `verify lint`), and the
	// parity model in parity.go reads `ai hooks unit` beside `hook-check unit`.
	// Pinned by TestLossyPipeReadsTheTwoWordArea.
	// Re-sealed 2026-09-25 for the same spec, Phase 3: heavyArea in bash.go
	// reads a suite only under `test`, because the retired top-level suite names
	// answer `unknown command`. Pinned by TestLossyPipeReadsTheTwoWordArea.
	// Re-sealed 2026-09-25 for the same spec, final pass: goimports names the
	// moved packages lepath, leroot and specpath in the hookruntime import
	// blocks, because their package clauses no longer match the directory
	// names. Import lines only, no hook decision changed.
	// Re-sealed 2026-09-25 at closure: the heavyArea comment in
	// hookruntime/bash.go lost stale text. Comment only, no hook decision changed.
	// Re-sealed 2026-09-26: the owner removed the hooks that judged wording. The
	// Stop hook no longer matches phrases, and the agent-brief gate, the LSP gate
	// and the two per-prompt reminders are gone with their fixtures. The same
	// seal covers bash.go, whose test-deletion check now reads each command's own
	// operands.
	// Re-sealed 2026-09-26 again: the bash.go test-deletion check is line-wide and fail-closed.
	// Re-sealed 2026-09-27: writeStyleGuideRead refuses a Go write until the
	// context read the style guide, recorded by hookSourceRead and bashStyleGuideRead.
	// Pinned by styleguide_test.go in hookruntime. Only a whole read counts:
	// a Read with no offset and a limit that reaches the last line, or a Bash
	// cat, bat, nl, less or more of the file with no pipe, redirection,
	// substitution or +N start line.
	// Re-sealed 2026-09-27 after lint and compound-guard fixes: split guards in
	// runsDeletion, testPath and hookStop; hookSessionStart returns nothing.
	// Re-sealed 2026-10-05 over unchanged committed source inputs.
	// Re-sealed 2026-10-12 over three reviewed edits that change no verdict:
	// runtime.go checkName uses strings.CutLast (59e2927e52), lifecycle.go
	// reads the spec Status vocabulary from specstatus (69893c680c), and the
	// bash.go staging refusal names `remove` without a prior rm (07ad5adb5d).
	//
	// Re-sealed 2026-10-14 over the per-agent identity: session.go exports
	// ZE_AGENT_ID beside the parent session id and reads a spec claim per
	// agent with the parent's as fallback, lifecycle.go and writeedit.go read
	// claims through it, and writeWeakening keys the shard on the payload's
	// agent and the RFC approval on the main thread's namespace. Review round
	// 1: the prefix writer blocks (exit 2) an agent id that cannot name a file
	// instead of prefixing nothing, runtime.go returns that code, and the
	// style-guide marker joins session and agent through
	// lepath.AgentFingerprint. Review round 2: an agent id with an absent or
	// unsafe payload session id blocks the Bash prefix and the weakening check
	// (exit 2) instead of running as the main thread. 2026-10-18: the
	// wait-loop refusal names `./le verify current`, the removed `mode full`
	// spelling gone.
	// Re-sealed 2026-10-20 for the design-door gate: writeDesignDoorsRead
	// (writeedit.go, with its door parsing in readgate.go, now a hashed
	// source) refuses a Go, YANG or templ write until the context read
	// ai/doors/system.md and every door whose
	// door-base header is the longest prefix of the file, each in full. The
	// style guide and the doors share one read record (documentReadMarker,
	// gatedDocument), so bashStyleGuideRead became bashDocumentRead and the
	// style-guide marker is now .doc-read-<escaped path>-<fingerprint>. Pinned
	// by doorgate_test.go and styleguide_test.go in hookruntime. No fixture
	// category is owed: the style gate it extends has none, and Probe reaches
	// the new check through the registry. Review round 1: a page under
	// ai/doors other than README.md with no usable header refuses, a file
	// outside the checkout after symlink resolution is not gated, the
	// unreadable-doors refusal is split by cause, and the marker escapes `+`.
	// Re-sealed 2026-10-23 for spec-plan-release-organization: subagent context
	// resolves the claim in current categories (and explains resolution errors),
	// validation refuses obsolete categories, and design evidence uses current
	// spec discovery. Also includes the previously committed a887d11f38 cleanup:
	// FieldsSeq in doorBases and AsType in writeDesignDoorsRead, with unchanged
	// behavior. The prior seal matches the sources immediately before that commit.
	// Re-sealed 2026-10-23: validateSpecText accepts any `./le test` or
	// `./le verify` command in place of the literal `./le verify worktree`.
	// Re-sealed 2026-10-23 (spec-verification-debt-derived, phase 4b): the
	// session-start hook reports the derived verification debt
	// (reportDerivedDebt over commit.DebtWarning) in place of the ledger, and
	// the bash parity allows the `push` verb in place of the retired debt verbs.
	// Re-sealed 2026-10-23 (spec-verification-debt-derived, review round 2):
	// the reportDerivedDebt comment now defers to derivedDebt.owed and names
	// the warning for a covered HEAD whose push is refused. Comment only.
	// Re-sealed 2026-10-24 for spec-weakening-approval-lives-in-the-commit:
	// writeWeakening keys the approval on the main thread's commit namespace,
	// and the session-start text no longer names ledger shards. Re-sealed
	// again for its review: the hook reads approvals through the shared
	// LoadApprovals, which refuses a bare unit and a row a landed commit used.
	// Re-sealed again for its second review: the audit skips a message line
	// whose unit is malformed, the shared approval code refuses one unit
	// changed in two directories (SharedUnits), and rfc.TrailerLanded exempts
	// only git's exact "not a git repository" message.
	// Re-sealed 2026-10-25: the lossy-pipe refusal and the spec checklist
	// hint name `./le verify changed` (35421aae45), and the weakening approve
	// action's parameters declare their kind and help (f21dd47e50). Text and
	// metadata only; no verdict changed.
	// Re-sealed 2026-10-25 for D-P7-short-long (AC-55): the weakened area's
	// actions declare a one-line Summary and its keyword Help lines fit 60
	// characters. Text only; no verdict changed.
	// Re-sealed 2026-10-25 for the Phase 4 rename of `ai sync write` to `ai
	// sync update`: the session-start hint names the new spelling. Text only;
	// no verdict changed.
	// Re-sealed 2026-10-26 for the Phase 4 rename of `le test rpki` to `le
	// mock rpki` (c699212581): one bash.go comment names the new spelling.
	// Comment only; no verdict changed.
	// Re-sealed 2026-10-26 for the Phase 4 move of `le commit audit` to `le
	// test weakened check`: weakened/actions.go gains the check action and a
	// Does word on proposed. The proposed hook's answer is unchanged.
	// Re-sealed 2026-10-26: postFormatGo says a lint run that did not complete
	// as not run instead of passing it, and posttool-bash (postbash.go, now in
	// hookSourcePaths) names Go files a shell command left unformatted. Neither
	// refuses an edit this population describes: both report after the tool
	// ran, and their polarities are driven through Probe and postLintVerdict in
	// internal/le/hookruntime (postbash_test.go, postwrite_test.go).
	// Re-sealed 2026-10-26: the post-edit lint blocks only on the edited file
	// or a package that does not build, the post-command gofmt blocks only on a
	// file the command names, and a blocking run drops code-0 hints. Each is a
	// post-tool report that blocks less; none refuses an edit this population
	// describes (postwrite_test.go, postbash_test.go, runtime_test.go).
	// Re-sealed 2026-10-29 over three committed changes and one lint repair:
	// 8a109bc036 lets the weakening check pass a file outside the checkout
	// (TestWeakeningPassesAFileOutsideTheCheckout), 9b751cd2fc moves the plan
	// file refusal from .claude/plan/ to plan/drafts/ze-plan-<name>, and
	// 85e880d9aa makes the startup hook name TRIGGERS.md. The lint repair
	// ranges over strings.FieldsSeq in postbash.go and names the Bash tool
	// through the toolBash constant in runtime.go; neither changes a verdict.
	// Each committed change states its own refusal change and carries its own
	// test.
	hookSourcesDigest = [sha256.Size]byte{
		0x9e, 0x29, 0x25, 0xa9, 0x51, 0xd0, 0xb0, 0x71,
		0x03, 0x36, 0x74, 0x7d, 0x5e, 0xad, 0x55, 0x6e,
		0x3d, 0x2d, 0x16, 0x33, 0x98, 0xbb, 0x21, 0xce,
		0x49, 0xee, 0x18, 0x40, 0x99, 0x5e, 0x3b, 0xe5,
	}
)

type fixtureMessage struct {
	match string
	text  string
}

type fixtureNameGenerator struct {
	prefix string
	suffix string
	labels []string
}

type fixtureSite struct {
	category     string
	name         string
	generator    fixtureNameGenerator
	expectedExit int
	messages     []fixtureMessage
}

type fixtureSpec struct {
	category     string
	name         string
	expectedExit int
	messages     []fixtureMessage
	site         int
	variant      int
}

type fixtureCategory struct {
	name     string
	runner   string
	owner    string
	evidence string
	allow    string
	refuse   string
}

var fixtureCategories = [...]fixtureCategory{
	{categoryFormatAlloc, "run_format_alloc", hookWriteEditFile, "func writeGoPatterns(", "return fmt.Errorf(\"x\")", "return fmt.\u0053printf(\"%d\", n)"},
	{categoryDesignRef, "run_design_ref", "internal/le/doc/consistency/consistency.go", "func (c *checker) checkDesignRefs(", "// Design: docs/x.md\npackage x", "package x"},
	{categoryTestFirst, "run_test_first", hookWriteEditFile, "func writeSpecStatus(", "in-progress", "design"},
	{categoryRenderedRule, "run_rendered_rule", hookWriteEditFile, "func writeRenderedRule(", "ai/rules/points/commands/x.md", "ai/rules/commands.md"},
	{categoryRFCLanguage, "run_rfc_language", hookWriteEditFile, "func writePointLanguage(", "The caller MUST stop.", "The caller stops."},
	{categoryValidateSpec, "run_validate_spec", hookLifecycleFile, "func hookValidateSpec(", probeCompleteSpec, "# Spec: probe\n"},
	{categoryCommitGate, "run_commit_gate", hookPostWriteFile, "func postDeferral(", "implemented now", "deferred to later"},
	{categorySessionID, "run_session_id", "internal/le/hookruntime/session.go", "func runSessionID(", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "../shared"},
	{categoryRFCTestGuard, weakenedProposedFixture, hookWriteEditFile, writeWeakeningAnchor, probeUntaggedEdit, probeTaggedEdit},
	{categoryWeakenedHatch, weakenedProposedFixture, hookWriteEditFile, writeWeakeningAnchor, probeLedgeredSkip, probeUnledgeredSkip},
	{categoryRFCApproval, weakenedProposedFixture, hookWriteEditFile, writeWeakeningAnchor, probeLedgeredEdit, probeUnledgeredEdit},
	{categoryDraftIncubator, "run_draft_incubator", hookWriteEditFile, writeWeakeningAnchor, "test/draft/probe_test.go", "test/unit/probe_test.go"},
	{categoryGovernedDocEdit, "run_governed_doc_edit", hookBashFile, "func bashGovernedWrite(", "cat plan/post-release/engineering/spec-x.md", "echo x > plan/post-release/engineering/spec-x.md"},
	{categoryMarkSourceRead, "run_mark_source_read", hookLifecycleFile, "func hookSourceRead(", "internal/probe/probe.go", "docs/probe.md"},
	{categoryDesignGate, "run_design_gate", hookWriteEditFile, "func writeDesignEvidence(", "source-read", "no-source-read"},
	{categoryDelegation, "run_delegation", hookLifecycleFile, "func hookStop(", "no-claimed-spec", probeClaimedSpec},
	{categorySessionState, "run_session_state", hookLifecycleFile, "func hookEndSummary(", sessionStateKept, sessionStateRotated},
	{categorySessionStateLocation, "run_session_state_location", hookLifecycleFile, "func stateFile(", "/state/session-state-", "tmp/session/shared/"},
	{categorySubagentContext, "run_subagent_context", hookLifecycleFile, "func hookSubagentContext(", probeClaimedSpec, "no-claimed-spec"},
	{categoryRawJobAdmission, "run_raw_job_admission", hookBashFile, "func bashRawHeavy(", "./le test unit core", "go test ./..."},
	{categoryJournalRowShape, "run_journal_row_shape", hookPostWriteFile, "func postJournal(", "| 2026-08-22 | spec-x | hooks | symptom | fixed: fix |", "| 2026-08-22 | spec-x | hooks | broken |"},
	{categoryScriptWeakeningArms, "run_script_weakening_arms", hookWriteEditFile, writeWeakeningAnchor, "self.assertEqual(1, f())", "@pytest.mark.xfail\nself.assertEqual(1, f())"},
	{categoryCISleepMarker, "run_ci_sleep_marker", hookWriteEditFile, "func writeCISleep(", "# sleep(timer): tracker ticks\ntime.sleep(2)", "time.sleep(2)"},
	{categoryYangDescription, "run_yang_description", hookWriteEditFile, "func writeYangDescription(", yangProbeAllow, yangProbeRefuse},
}

type fixtureProducerBoundary struct {
	category       string
	actionOwner    string
	actionEvidence string
	nativeOwner    string
	nativeEvidence string
}

var fixtureProducerBoundaries = [...]fixtureProducerBoundary{
	{
		categoryRFCTestGuard,
		weakenedActionsFile,
		proposedVerbAnchor,
		weakenedProposedFile,
		"func proposedRFCChanges(",
	},
	{
		categoryWeakenedHatch,
		weakenedActionsFile,
		proposedVerbAnchor,
		weakenedProposedFile,
		"func proposedFindings(",
	},
	{
		categoryRFCApproval,
		weakenedActionsFile,
		proposedVerbAnchor,
		weakenedProposedFile,
		"func proposedLedger(",
	},
}

var hookSourcePaths = [...]string{
	"internal/le/hookruntime/runtime.go",
	"internal/le/hookruntime/session.go",
	hookBashFile,
	hookWriteEditFile,
	hookPostWriteFile,
	"internal/le/hookruntime/postbash.go",
	hookLifecycleFile,
	"internal/le/hookruntime/readgate.go",
	weakenedActionsFile,
	weakenedProposedFile,
}

func runFixtures(root string) ([]Result, int) {
	results := make([]Result, 0, len(fixtureCategories)+2)
	results = append(results,
		checkFixturePopulation(fixtureCategories[:], fixtureSites[:], fixtureCatalog),
		checkHookSources(root))
	for _, category := range &fixtureCategories {
		results = append(results, checkFixtureCategory(category, fixtureCatalog))
	}
	return results, len(fixtureCatalog)
}

func checkFixturePopulation(
	categories []fixtureCategory,
	sites []fixtureSite,
	fixtures []fixtureSpec,
) Result {
	result := Result{Name: "behavioral-fixture-population", Passed: true}
	if len(sites) != fixtureSitesExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("typed Results.check population is ").Int(int64(len(sites))).
			Str(" sites, want ").Int(fixtureSitesExpected).String()
		return result
	}
	if current := fixtureSiteContentDigest(sites); current != fixtureSiteDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("typed Results.check site content changed: got %x, want %x", current, fixtureSiteDigest)
		return result
	}
	if len(fixtures) != fixtureChecksExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("expanded fixture population is ").Int(int64(len(fixtures))).
			Str(" rows, want ").Int(fixtureChecksExpected).String()
		return result
	}
	if current := fixtureDigest(fixtures); current != fixtureCatalogDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("expanded fixture content changed: got %x, want %x", current, fixtureCatalogDigest)
		return result
	}
	type identity struct {
		site    int
		variant int
	}
	identities := make(map[identity]struct{}, len(fixtures))
	names := make(map[string]struct{}, len(fixtures))
	for _, fixture := range fixtures {
		key := identity{site: fixture.site, variant: fixture.variant}
		if _, exists := identities[key]; exists {
			result.Passed = false
			result.Code = 2
			result.Message = "expanded fixture identity is duplicated"
			return result
		}
		identities[key] = struct{}{}
		names[fixture.name] = struct{}{}
	}
	if len(names) != fixtureUniqueNamesExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("expanded fixture names contain ").Int(int64(len(names))).
			Str(" unique values, want ").Int(fixtureUniqueNamesExpected).String()
		return result
	}
	if len(categories) != fixtureCategoriesExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("typed category population is ").Int(int64(len(categories))).
			Str(" rows, want ").Int(fixtureCategoriesExpected).String()
		return result
	}
	if current := categoryDigest(categories); current != fixtureCategoryDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("typed category or producer mapping changed: got %x, want %x", current, fixtureCategoryDigest)
		return result
	}
	if current := producerBoundaryDigest(fixtureProducerBoundaries[:]); current != fixtureBoundaryDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("typed native producer boundary changed: got %x, want %x", current, fixtureBoundaryDigest)
	}
	return result
}

// hookSourceContentDigest reads every hook and native producer file under root
// and answers one digest over their paths and their bodies.
func hookSourceContentDigest(root string) ([sha256.Size]byte, error) {
	var tb textbuf.Buffer
	for _, path := range &hookSourcePaths {
		body, err := readCheckoutFile(root, path)
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		tb.Str(path).Byte(0).Str(string(body)).Byte(0)
	}
	return sha256.Sum256([]byte(tb.String())), nil
}

func checkHookSources(root string) Result {
	result := Result{Name: "hook-source-drift", Passed: true}
	current, err := hookSourceContentDigest(root)
	if err != nil {
		result.Passed = false
		result.Code = 2
		result.Message = err.Error()
		return result
	}
	if current != hookSourcesDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("actual hook or native producer changed: got %x, want %x", current, hookSourcesDigest)
	}
	return result
}

func checkFixtureCategory(category fixtureCategory, fixtures []fixtureSpec) Result {
	var tb textbuf.Buffer
	result := Result{Name: tb.Str("fixture-").Str(category.name).String(), Passed: true}
	found := false
	for _, fixture := range fixtures {
		if fixture.category == category.name {
			found = true
			break
		}
	}
	if !found {
		result.Passed = false
		result.Code = 2
		result.Message = "typed fixture category is missing"
		return result
	}
	allowed, err := categoryVerdict(category.name, category.allow)
	if err != nil {
		result.Passed = false
		result.Code = 2
		result.Message = "native probe could not run: " + err.Error()
		return result
	}
	if !allowed {
		result.Passed = false
		result.Code = 1
		result.Message = "native allow probe was refused"
		return result
	}
	refused, err := categoryVerdict(category.name, category.refuse)
	if err != nil {
		result.Passed = false
		result.Code = 2
		result.Message = "native probe could not run: " + err.Error()
		return result
	}
	if refused {
		result.Passed = false
		result.Code = 1
		result.Message = "native refusal probe was allowed"
	}
	return result
}

func expandFixtureSites(sites []fixtureSite) []fixtureSpec {
	count := 0
	for _, site := range sites {
		if site.name != "" {
			count++
			continue
		}
		count += len(site.generator.labels)
	}
	fixtures := make([]fixtureSpec, 0, count)
	for siteIndex, site := range sites {
		if site.name != "" {
			fixtures = append(fixtures, fixtureSpec{
				category: site.category, name: site.name, expectedExit: site.expectedExit,
				messages: site.messages, site: siteIndex + 1, variant: 1,
			})
			continue
		}
		for variantIndex, label := range site.generator.labels {
			fixtures = append(fixtures, fixtureSpec{
				category:     site.category,
				name:         site.generator.prefix + label + site.generator.suffix,
				expectedExit: site.expectedExit,
				messages:     site.messages,
				site:         siteIndex + 1,
				variant:      variantIndex + 1,
			})
		}
	}
	return fixtures
}

func fixtureSiteContentDigest(sites []fixtureSite) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, site := range sites {
		tb.Str(site.category).Byte(0).Str(site.name).Byte(0).
			Str(site.generator.prefix).Byte(0).Str(site.generator.suffix).Byte(0)
		for _, label := range site.generator.labels {
			tb.Str(label).Byte(0)
		}
		tb.Int(int64(site.expectedExit)).Byte(0)
		for _, message := range site.messages {
			tb.Str(message.match).Byte(0).Str(message.text).Byte(0)
		}
	}
	return sha256.Sum256([]byte(tb.String()))
}

func fixtureDigest(fixtures []fixtureSpec) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, fixture := range fixtures {
		tb.Str(fixture.category).Byte(0).Str(fixture.name).Byte(0).
			Int(int64(fixture.expectedExit)).Byte(0).
			Int(int64(fixture.site)).Byte(0).Int(int64(fixture.variant)).Byte(0)
		for _, message := range fixture.messages {
			tb.Str(message.match).Byte(0).Str(message.text).Byte(0)
		}
	}
	return sha256.Sum256([]byte(tb.String()))
}

func categoryDigest(categories []fixtureCategory) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, category := range categories {
		tb.Str(category.name).Byte(0).Str(category.runner).Byte(0).
			Str(category.owner).Byte(0).Str(category.evidence).Byte(0).
			Str(category.allow).Byte(0).Str(category.refuse).Byte(0)
	}
	return sha256.Sum256([]byte(tb.String()))
}

func producerBoundaryDigest(boundaries []fixtureProducerBoundary) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, boundary := range boundaries {
		tb.Str(boundary.category).Byte(0).
			Str(boundary.actionOwner).Byte(0).Str(boundary.actionEvidence).Byte(0).
			Str(boundary.nativeOwner).Byte(0).Str(boundary.nativeEvidence).Byte(0)
	}
	return sha256.Sum256([]byte(tb.String()))
}

// yangProbeAllow and yangProbeRefuse are the two YANG bodies the summary gate
// must separate: a ze:help inside the bounds, and one past the 96-character cap
// writeYangDescription enforces. The summary is the ze:help statement; the
// description beside it is the long explanation and carries no cap.
const (
	yangProbeAllow  = "leaf a {\n  ze:help \"A short summary.\";\n}"
	yangProbeRefuse = "leaf a {\n  ze:help \"" +
		"A summary written far past the ninety-six character cap so the gate has something it must refuse here.\";\n}"
)

// categoryProbe binds one fixture category to the producer that DECIDES its
// verdict. Exactly one of three fields names that producer: check is a Go
// function registered in hookruntime's nativeHookActions, resolved there rather
// than listed twice; lifecycle is a kind runLifecycleHook serves; finding is
// the consistency check whose silence means "allowed", for the one category
// whose producer is not a hook at all.
//
// tool is the Claude tool the payload claims, file the path it carries, and
// slot the payload field the category's allow/refuse value fills. wrap is a
// format string with one %s when the producer needs the value inside a larger
// document; an empty wrap passes the value through.
type categoryProbe struct {
	check     string
	lifecycle string
	finding   string
	tool      string
	file      string
	slot      string
	wrap      string
	// session is the id the payload claims. A check that resolves a session
	// reads it before it reads the checkout, so setting it makes such a check
	// answer about a tree the probe controls rather than about the tree the
	// running session happens to be in.
	session string
	// tree builds a throwaway checkout for the value under test and returns its
	// root. It is set for a check whose verdict comes from FILESYSTEM state
	// rather than from the payload: a session marker, a claimed spec, the file
	// as it stands on disk. An unset tree leaves the check over the real
	// checkout, which is right for a check that reads only what it is handed.
	tree func(value string) (string, error)
	// answer reads the producer's response and reports whether it ALLOWED the
	// value. The default reads the exit code, which is the whole of what a
	// check says. A hook that answers with the TEXT it wrote, or with a FILE it
	// left in the tree, names the function that reads that instead.
	answer func(said probeAnswer) (bool, error)
}

// probeAnswer is everything one producer said: its exit code, the two streams
// it wrote joined, the tree it judged, and the raw value it judged.
type probeAnswer struct {
	code   int
	output string
	root   string
	value  string
}

// probeSession is the session id every tree-building probe claims. It is not a
// real session: it names the marker files the probe writes under its own
// throwaway root, and lepath.ValidSessionID is the only thing it must satisfy.
const probeSession = "probe-fixture-session"

// The paths every tree-building probe writes to. One name per kind of file
// keeps the trees and the payloads that address them in step.
const (
	probeSpecName    = "spec-probe.md"
	probeTestPath    = "probe/probe_test.go"
	probeJournalPath = "plan/journal/probe.md"
	probeGoPath      = "internal/probe/probe.go"
)

// probeGitTimeout bounds each local Git operation that builds a probe tree.
const probeGitTimeout = 30 * time.Second

// probeTree makes a throwaway checkout holding files, each keyed by a
// slash-separated path under its root. Every tree-building probe goes through
// it, so a tree that cannot be built answers with the error that stopped it
// rather than with an empty root the caller would read as a refusal.
func probeTree(name string, files map[string]string) (string, error) {
	root, err := os.MkdirTemp("", "ze-probe-"+name+"-")
	if err != nil {
		return "", err
	}
	for path, body := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return root, err
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			return root, err
		}
	}
	return root, nil
}

// blankProbeTree builds an empty checkout, for a hook whose verdict comes from
// what it WRITES into a tree rather than from anything it reads there. It also
// keeps such a hook off the real checkout, where it would leave its marker or
// its snapshot behind.
func blankProbeTree(string) (string, error) {
	return probeTree("blank", nil)
}

// designGateTree builds a checkout whose session recorded a Go source read, or
// recorded nothing. That is the whole of what writeDesignEvidence judges, and
// judging it needs a tree because the evidence is a marker file rather than
// anything in the payload.
func designGateTree(value string) (string, error) {
	if value != "source-read" {
		return probeTree("design-gate", nil)
	}
	return probeTree("design-gate", map[string]string{
		"tmp/session/.source-read-go-" + probeSession: "probe\n",
	})
}

// specStatusTree builds a checkout whose session claims one spec, carrying the
// Status under test. writeSpecStatus reads the claim marker and then the spec's
// own Status row, so a payload on its own tells it nothing.
func specStatusTree(value string) (string, error) {
	return probeTree("spec-status", map[string]string{
		"tmp/session/.session-" + probeSession:           probeSpecName + "\n",
		"plan/post-release/engineering/" + probeSpecName: "| Status | " + value + " |\n",
	})
}

// The test file the weakening probes judge, and the edits of it they hand the
// gate. Each RFC requirement tag sits INSIDE a function body on purpose:
// rfc.UnitAt widens a tag written above the func line to the whole file, and
// every edit anywhere in the file would then change that tag.
const (
	probeTaggedTest = `package probe

func TestTagged(t *testing.T) {
	// RFC requirement: RFC4271-6.2-1 -- the tagged unit
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestPlain(t *testing.T) {
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
	probeUntaggedEdit = `package probe

func TestTagged(t *testing.T) {
	// RFC requirement: RFC4271-6.2-1 -- the tagged unit
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestPlain(t *testing.T) {
	if hold(2) != 3 {
		t.Fatal("two")
	}
}
`
	probeTaggedEdit = `package probe

func TestTagged(t *testing.T) {
	// RFC requirement: RFC4271-6.2-1 -- the tagged unit
	if hold(1) != 9 {
		t.Fatal("one")
	}
}

func TestPlain(t *testing.T) {
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
)

// Two tagged units, and the edit of each. Which unit an edit reaches is the
// whole difference the RFC approval file turns on.
const (
	probeTwoTaggedTests = `package probe

func TestLedgered(t *testing.T) {
	// RFC requirement: RFC4271-6.2-1 -- the unit the ledger names
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestUnledgered(t *testing.T) {
	// RFC requirement: RFC4271-6.3-1 -- the unit no row names
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
	probeLedgeredEdit = `package probe

func TestLedgered(t *testing.T) {
	// RFC requirement: RFC4271-6.2-1 -- the unit the ledger names
	if hold(1) != 9 {
		t.Fatal("one")
	}
}

func TestUnledgered(t *testing.T) {
	// RFC requirement: RFC4271-6.3-1 -- the unit no row names
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
	probeUnledgeredEdit = `package probe

func TestLedgered(t *testing.T) {
	// RFC requirement: RFC4271-6.2-1 -- the unit the ledger names
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestUnledgered(t *testing.T) {
	// RFC requirement: RFC4271-6.3-1 -- the unit no row names
	if hold(2) != 9 {
		t.Fatal("two")
	}
}
`
)

// The same two units untagged, and a t.Skip added to each in turn. Adding a
// skip stops the unit running, which is what the weakening ledger authorizes
// one row at a time.
const (
	probeUntaggedTests = `package probe

func TestLedgered(t *testing.T) {
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestUnledgered(t *testing.T) {
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
	probeLedgeredSkip = `package probe

func TestLedgered(t *testing.T) {
	t.Skip("probe")
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestUnledgered(t *testing.T) {
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
	probeUnledgeredSkip = `package probe

func TestLedgered(t *testing.T) {
	if hold(1) != 1 {
		t.Fatal("one")
	}
}

func TestUnledgered(t *testing.T) {
	t.Skip("probe")
	if hold(2) != 2 {
		t.Fatal("two")
	}
}
`
)

// probeLedgerRows is a ledger holding one row, naming probe.TestLedgered. The
// weakening ledger and the session's RFC approval file parse through the
// same table grammar, so one body serves both.
const probeLedgerRows = "| Test | Reason |\n| --- | --- |\n| probe.TestLedgered | the row the probe authorizes |\n"

// rfcGuardTree builds a checkout holding the tagged test as it stands and no
// RFC approval file, so an edit that changes what the tagged unit proves has
// nowhere to be authorized.
func rfcGuardTree(string) (string, error) {
	return committedProbeTree("rfc-guard", map[string]string{probeTestPath: probeTaggedTest})
}

// rfcApprovalTree adds the session's RFC approval file, the one `./le rfc
// approve` writes under tmp/, to two tagged units, one of which a row names.
func rfcApprovalTree(string) (string, error) {
	return probeLedgerTree("rfc-approval", probeTwoTaggedTests, lepath.SessionCommitSession, rfc.ApprovalPath)
}

// weakenedHatchTree adds the owner's weakening approval file to two untagged
// units, one of which a row names.
func weakenedHatchTree(string) (string, error) {
	return probeLedgerTree("weakened-hatch", probeUntaggedTests, lepath.SessionCommitSession, testweakened.ApprovalPath)
}

// probeLedgerTree writes the ledger rows at the path THIS tree resolves for
// the running session, rather than at a fixed file name. The hook derives the
// same path from the same identity, so the fixture proves the derivation as
// well as the row matching: a file written anywhere else would read as absent.
// namespace is the resolver the reader keys the file on: the main thread's,
// for a weakening approval and an RFC approval alike.
func probeLedgerTree(name, test string, namespace func(root string) (string, error), ledgerPath func(session string) string) (string, error) {
	root, err := committedProbeTree(name, map[string]string{probeTestPath: test})
	if err != nil {
		return root, err
	}
	session, err := namespace(root)
	if err != nil {
		return root, err
	}
	ledger := filepath.Join(root, filepath.FromSlash(ledgerPath(session)))
	if err := os.MkdirAll(filepath.Dir(ledger), 0o750); err != nil {
		return root, err
	}
	return root, os.WriteFile(ledger, []byte(probeLedgerRows), 0o600)
}

// draftIncubatorTree holds one tagged test twice, inside the draft incubator
// and outside it. Only the path differs, which is what the exemption reads.
func draftIncubatorTree(string) (string, error) {
	return committedProbeTree("draft-incubator", map[string]string{
		"test/draft/probe_test.go": probeTaggedTest,
		"test/unit/probe_test.go":  probeTaggedTest,
	})
}

// journalTree builds a checkout holding one journal file: the header row every
// journal owes, and the row under test beneath it. postJournal re-reads the
// file from disk, so the row has to be written there rather than passed.
// specjournal.ValidateFile also reads the file at git HEAD to tell a new row,
// which the length cap binds, from one already committed, so the tree is a git
// repository. Its HEAD has no commit, so the row under test is a new row and the
// cap binds it.
func journalTree(value string) (string, error) {
	root, err := probeTree("journal", map[string]string{
		probeJournalPath: "| Date | Spec | Surface | Symptom | Fix |\n" +
			"| --- | --- | --- | --- | --- |\n" + value + "\n",
	})
	if err != nil {
		return root, err
	}
	return root, initProbeRepository(root)
}

// probeCompleteSpec is a spec body that satisfies every section, citation and
// checklist item validateSpecText requires. Its Status is skeleton because that
// is the one status hookValidateSpec judges without also auditing design
// document anchors, which no throwaway tree carries.
const probeCompleteSpec = `# Spec: probe

| Status | skeleton |
| Updated | 2026-01-01 |

## Task

Give the validate-spec probe a body the hook accepts.

## Required Reading

- ai/rules/testing.md

## Current Behavior

- [ ] ` + "`internal/probe/probe.go`" + `

## Data Flow

### Entry Point

The hook payload.

### Transformation Path

The probe body.

### Boundaries Crossed

None.

### Integration Points

None.

## Wiring Test

| Entry Point | Feature Code | Test |
| probe -> hook | probe | probe |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
| probe | probe |

## Files to Modify

- ` + "`internal/probe/probe.go`" + `

## Implementation Steps

1. Probe.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] ./le verify current fast
`

// validateSpecTree writes the spec body under test where hookValidateSpec reads
// it. The hook re-reads the file from disk rather than judging what the payload
// carried, so the body has to exist under a root the probe owns.
func validateSpecTree(value string) (string, error) {
	return probeTree("validate-spec", map[string]string{"plan/post-release/engineering/" + probeSpecName: value})
}

// probeClaimedSpec is the fixture value that asks claimedSpecTree for a
// checkout whose session claims a spec.
const probeClaimedSpec = "claimed-spec"

// claimedSpecTree builds a checkout whose session claims a spec, or claims
// none. Naming the parent's spec is the one thing in the subagent context that
// the tree decides, and a claim with no subagent spawned is the open state the
// stop hook warns about.
func claimedSpecTree(value string) (string, error) {
	if value != probeClaimedSpec {
		return probeTree(probeClaimedSpec, nil)
	}
	return probeTree(probeClaimedSpec, map[string]string{
		"tmp/session/.session-" + probeSession:           probeSpecName + "\n",
		"plan/post-release/engineering/" + probeSpecName: "| Status | in-progress |\n",
	})
}

// The two values the end-of-session summary must separate: a handoff heading,
// which survives whatever else rotates out, and a snapshot heading pushed past
// the two-snapshot window by the filler, which does not.
const (
	sessionStateKept    = "## Phase 4 handoff"
	sessionStateRotated = "## Session: 2019-12-31T00:00:00Z"
	sessionStateFiller  = "\n\n## Session: 2020-01-01T00:00:00Z\n\nBranch: probe\n"
)

// sessionSummaryTree builds a checkout the summary hook will write in, runs the
// hook once, then appends the value to the state file THAT run chose. The
// producer is asked where its state file lives rather than told, because a
// probe that rebuilt the path would carry a copy of the naming rule it exists
// to hold the producer to.
//
// The summary writes nothing unless git reports a dirty tree, so the tree is a
// repository with one untracked file in it.
func sessionSummaryTree(value string) (string, error) {
	root, err := probeTree("session-state", map[string]string{"PROBE": "probe\n"})
	if err != nil {
		return root, err
	}
	if err := initProbeRepository(root); err != nil {
		return root, err
	}
	if _, _, found := hookruntime.ProbeLifecycle("session-end-summary", hookruntime.Payload{}, root); !found {
		return root, errors.New("hookruntime serves no session-end-summary hook")
	}
	states, err := stateFilesUnder(root)
	if err != nil {
		return root, err
	}
	if len(states) != 1 {
		return root, fmt.Errorf("the summary hook wrote %d state files under the probe tree, want 1", len(states))
	}
	file, err := os.OpenFile(states[0], os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the path is the state file the hook just wrote under the probe tree
	if err != nil {
		return root, err
	}
	_, writeErr := file.WriteString(sessionStateFiller + "\n\n" + value + "\n\nBranch: probe\n")
	return root, errors.Join(writeErr, file.Close())
}

// committedProbeTree records the original test in HEAD before a Write proposal
// is judged. An initialized repository without HEAD cannot prove ownership.
func committedProbeTree(name string, files map[string]string) (string, error) {
	root, err := probeTree(name, files)
	if err != nil {
		return root, err
	}
	if err := initProbeRepository(root); err != nil {
		return root, err
	}
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
			"-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Original probe"},
	} {
		ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), probeGitTimeout)
		command := exec.CommandContext(ctx, "git", args...) //nolint:gosec // Fixed git binary and literal argv above; fixture content never becomes an argument.
		command.Dir = root
		output, err := command.CombinedOutput()
		cancel()
		if err != nil {
			return root, fmt.Errorf("git %s in the probe tree: %w: %s", args[0], err, strings.TrimSpace(string(output)))
		}
	}
	return root, nil
}

// initProbeRepository makes the probe tree a git repository. session.EndSummary
// reads `git status --porcelain` and returns without writing when it is empty,
// so a plain directory would leave the probe with no answer at all.
func initProbeRepository(root string) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), probeGitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "init", "--quiet")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git init in the probe tree: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// stateFilesUnder answers the session state files a hook wrote under root.
func stateFilesUnder(root string) ([]string, error) {
	return filepath.Glob(filepath.Join(root, "tmp", "session", "*", "state", "*.md"))
}

// designRefTree writes the Go file under test where the consistency walk finds
// it. checkDesignRefs judges a file only under internal/, pkg/ or cmd/.
func designRefTree(value string) (string, error) {
	return probeTree("design-ref", map[string]string{probeGoPath: value})
}

// saidTheValue reports whether the hook's own text carries the value. It is the
// verdict for a hook whose answer IS what it wrote: the delegation reminder, or
// the path a state file went to. Such a hook exits 0 whatever it says, so its
// exit code separates nothing.
func saidTheValue(said probeAnswer) (bool, error) {
	return strings.Contains(said.output, said.value), nil
}

// wroteASourceMarker reports whether hookSourceRead left a marker in the tree.
// The hook exits 0 for every path it is handed, so the marker file is the only
// thing that tells a source read from a document read.
func wroteASourceMarker(said probeAnswer) (bool, error) {
	markers, err := filepath.Glob(filepath.Join(said.root, "tmp", "session", ".source-read-*-"+probeSession))
	if err != nil {
		return false, err
	}
	return len(markers) != 0, nil
}

// namedTheClaimedSpec reports whether the subagent context carries the spec the
// parent session claimed.
func namedTheClaimedSpec(said probeAnswer) (bool, error) {
	return strings.Contains(said.output, "plan/post-release/engineering/"+probeSpecName), nil
}

// keptTheStateText reports whether the value survives in the state file the
// summary rewrote. Rotation is the behavior under test and the hook exits 0
// either way, so the file it left behind is the answer.
func keptTheStateText(said probeAnswer) (bool, error) {
	states, err := stateFilesUnder(said.root)
	if err != nil {
		return false, err
	}
	if len(states) != 1 {
		return false, fmt.Errorf("the summary hook left %d state files under the probe tree, want 1", len(states))
	}
	body, err := os.ReadFile(states[0]) //nolint:gosec // the path is the state file the hook wrote under the probe tree
	if err != nil {
		return false, err
	}
	return strings.Contains(string(body), said.value), nil
}

const (
	slotContent   = "content"
	slotPath      = "path"
	slotCommand   = "command"
	slotSessionID = "session-id"
	// slotOffPayload says the value never reaches the payload at all: it shapes
	// the throwaway tree, or the answer looks for it in what the hook wrote. It
	// is named rather than left blank so no probe can forget to say where its
	// value goes.
	slotOffPayload = "off-payload"
)

// categoryProbes is the set of categories whose verdict asks the producer
// instead of restating it. A category is here or on the ungroundedCategories
// baseline in fixture_grounding_test.go, and that test refuses a third state.
//
//nolint:gochecknoglobals // fixture data, read by the category probes
var categoryProbes = map[string]categoryProbe{
	categoryFormatAlloc:          {check: "writeGoPatterns", tool: toolWriteName, file: probeGoPath, slot: slotContent},
	categoryRenderedRule:         {check: "writeRenderedRule", tool: toolWriteName, slot: slotPath},
	categoryRFCLanguage:          {check: "writePointLanguage", tool: toolWriteName, file: "ai/rules/points/probe/probe/probe.md", slot: slotContent, wrap: "---\nkind: directive\nstage:\n---\n%s\n"},
	categoryCISleepMarker:        {check: "writeCISleep", tool: toolWriteName, file: "./test/probe/probe.ci", slot: slotContent},
	categoryGovernedDocEdit:      {check: "bashGovernedWrite", tool: "Bash", slot: slotCommand},
	categoryRawJobAdmission:      {check: "bashRawHeavy", tool: "Bash", slot: slotCommand},
	categoryYangDescription:      {check: "writeYangDescription", tool: toolWriteName, file: "internal/plugins/probe/yang/probe.yang", slot: slotContent},
	categoryDesignGate:           {check: "writeDesignEvidence", tool: toolWriteName, file: "plan/post-release/engineering/" + probeSpecName, slot: slotContent, session: probeSession, tree: designGateTree},
	categoryCommitGate:           {check: "postDeferral", tool: toolWriteName, file: "docs/probe.md", slot: slotContent},
	categoryTestFirst:            {check: "writeSpecStatus", tool: toolWriteName, file: probeGoPath, slot: slotOffPayload, session: probeSession, tree: specStatusTree},
	categoryJournalRowShape:      {check: "postJournal", tool: toolWriteName, file: probeJournalPath, slot: slotOffPayload, tree: journalTree},
	categoryRFCTestGuard:         {check: weakeningCheckName, tool: toolWriteName, file: probeTestPath, slot: slotContent, tree: rfcGuardTree},
	categoryRFCApproval:          {check: weakeningCheckName, tool: toolWriteName, file: probeTestPath, slot: slotContent, tree: rfcApprovalTree},
	categoryWeakenedHatch:        {check: weakeningCheckName, tool: toolWriteName, file: probeTestPath, slot: slotContent, tree: weakenedHatchTree},
	categoryDraftIncubator:       {check: weakeningCheckName, tool: toolWriteName, slot: slotPath, tree: draftIncubatorTree},
	categoryValidateSpec:         {lifecycle: "validate-spec", tool: toolWriteName, file: "plan/post-release/engineering/" + probeSpecName, slot: slotOffPayload, tree: validateSpecTree},
	categorySessionID:            {lifecycle: "session-id", slot: slotSessionID},
	categoryMarkSourceRead:       {lifecycle: "mark-source-read", slot: slotPath, session: probeSession, tree: blankProbeTree, answer: wroteASourceMarker},
	categoryDelegation:           {lifecycle: "block-premature-stop", slot: slotOffPayload, session: probeSession, tree: claimedSpecTree},
	categorySubagentContext:      {lifecycle: "subagent-context", slot: slotOffPayload, session: probeSession, tree: claimedSpecTree, answer: namedTheClaimedSpec},
	categorySessionStateLocation: {lifecycle: "pre-compact-save", slot: slotOffPayload, tree: blankProbeTree, answer: saidTheValue},
	categorySessionState:         {lifecycle: "session-end-summary", slot: slotOffPayload, tree: sessionSummaryTree, answer: keptTheStateText},
	categoryDesignRef:            {finding: "design-refs", slot: slotOffPayload, tree: designRefTree},
}

// probeVerdict asks the producer whether it allows this value. It answers an
// ERROR, never a bare false, when it could not put the question: a tree that
// failed to build, a check hookruntime no longer registers, a lifecycle kind it
// no longer serves. False is the REFUSAL, and a probe that could not run must
// not be read as one.
func probeVerdict(probe categoryProbe, value string) (bool, error) {
	root := ""
	if probe.tree != nil {
		built, err := probe.tree(value)
		if built != "" {
			defer func() { _ = os.RemoveAll(built) }()
		}
		if err != nil {
			return false, fmt.Errorf("build the probe tree for %q: %w", value, err)
		}
		root = built
	}
	code, output, err := askProducer(probe, value, root)
	if err != nil {
		return false, err
	}
	if probe.check == weakeningCheckName && code != 0 {
		if code != 2 {
			return false, fmt.Errorf("weakening probe returned unexpected code %d: %s", code, output)
		}
		if !strings.Contains(output, "proposed test evidence change is not authorized") {
			return false, fmt.Errorf("weakening probe failed before judging authorization: %s", output)
		}
	}
	if probe.answer != nil {
		return probe.answer(probeAnswer{code: code, output: output, root: root, value: value})
	}
	return code == 0, nil
}

// askProducer runs the one producer this probe names and answers its exit code
// with everything it wrote.
func askProducer(probe categoryProbe, value, root string) (int, string, error) {
	switch {
	case probe.finding != "":
		return consistencyVerdict(probe.finding, root)
	case probe.lifecycle != "":
		code, output, found := hookruntime.ProbeLifecycle(probe.lifecycle, probePayload(probe, value), root)
		if !found {
			return 0, "", fmt.Errorf("hookruntime serves no lifecycle hook named %q", probe.lifecycle)
		}
		return code, output, nil
	case probe.check != "":
		code, message, found := hookruntime.Probe(probe.check, probePayload(probe, value), root)
		if !found {
			return 0, "", fmt.Errorf("hookruntime registers no check named %q", probe.check)
		}
		return code, message, nil
	default:
		return 0, "", errors.New("the probe names no producer: set check, lifecycle or finding")
	}
}

// consistencyVerdict walks the tree with the consistency gate and refuses when
// the named check reported on it. design-ref's producer is that gate rather
// than a hook, so it is the one category asked here.
func consistencyVerdict(finding, root string) (int, string, error) {
	if root == "" {
		return 0, "", errors.New("the consistency gate needs a tree of its own to walk")
	}
	for _, one := range docconsistency.Check(root).Findings {
		if one.Check == finding {
			return 2, one.Message, nil
		}
	}
	return 0, "", nil
}

// probePayload builds the hook envelope the producer reads.
func probePayload(probe categoryProbe, value string) hookruntime.Payload {
	input := map[string]any{}
	if probe.file != "" {
		input["file_path"] = probe.file
	}
	payload := hookruntime.Payload{ToolName: probe.tool, ToolInput: input}
	if probe.session != "" {
		payload.SessionID = probe.session
	}
	if probe.wrap != "" {
		value = fmt.Sprintf(probe.wrap, value)
	}
	switch probe.slot {
	case slotContent:
		input["content"] = value
	case slotPath:
		input["file_path"] = value
		input["content"] = "probe"
	case slotCommand:
		input["command"] = value
	case slotSessionID:
		payload.SessionID = value
	}
	return payload
}

func categoryVerdict(category, value string) (bool, error) {
	if probe, bound := categoryProbes[category]; bound {
		return probeVerdict(probe, value)
	}
	if category == categoryScriptWeakeningArms {
		// UNGROUNDED, and ungroundedCategories says so. This restates a rule
		// that lives in testweakened.detectPythonVerdicts, which no hook
		// reaches: testweakened.Proposed returns before it for every path that
		// is not a _test.go or a .ci/.et under test/, so a Python test file is
		// never judged by writeWeakening at all.
		return !strings.Contains(value, "@pytest.mark.xfail") &&
			!strings.Contains(value, "@unittest.expectedFailure"), nil
	}
	return false, fmt.Errorf("category %q has neither a probe nor a baseline restatement", category)
}

// The fixture catalog's own vocabulary: the message match kinds, the behavioral
// category names, the producer files a category is bound to, and the message
// fragments and generator labels that more than two rows repeat.
const (
	matchContains                = "contains"
	matchNotContains             = "not-contains"
	matchEquals                  = "equals"
	matchSuffix                  = "suffix"
	categoryFormatAlloc          = "format-alloc"
	categoryDesignRef            = "design-ref"
	categoryTestFirst            = "test-first"
	categoryRenderedRule         = "rendered-rule"
	categoryRFCLanguage          = "rfc-language"
	categoryValidateSpec         = "validate-spec"
	categoryCommitGate           = "commit-gate"
	categorySessionID            = "session-id"
	categoryWeakenedHatch        = "weakened-hatch"
	categoryRFCApproval          = "rfc-approval"
	categoryDraftIncubator       = "draft-incubator"
	categoryGovernedDocEdit      = "governed-doc-edit"
	categoryDelegation           = "delegation"
	categoryDesignGate           = "design-gate"
	categoryRFCTestGuard         = "rfc-test-guard"
	categoryJournalRowShape      = "journal-row-shape"
	categoryRawJobAdmission      = "raw-job-admission"
	categoryScriptWeakeningArms  = "script-weakening-arms"
	categoryCISleepMarker        = "ci-sleep-marker"
	categoryYangDescription      = "yang-description"
	categorySessionStateLocation = "session-state-location"
	categorySessionState         = "session-state"
	categoryMarkSourceRead       = "mark-source-read"
	categorySubagentContext      = "subagent-context"
	hookLifecycleFile            = "internal/le/hookruntime/lifecycle.go"
	weakenedActionsFile          = "internal/le/test/weakened/actions.go"
	weakenedProposedFile         = "internal/le/test/weakened/proposed.go"
	delegationHeading            = "Delegation:"
	planningRule                 = "planning.md"
	goSuffix                     = ".go"
	governedPrefix               = "governed-"
	kindMakefile                 = "makefile"
	kindShell                    = "shell"
	kindYang                     = "yang"
	labelDot                     = "dot"
	entryPointPlaceholder        = "Entry Point contains placeholder"
	writeWeakeningAnchor         = "func writeWeakening("
	removingExpectations         = "removing expectations"
	statesNoRFCLevel             = "states no RFC 2119 level"
)

// The remaining producer files and repeated fixture strings.
const (
	labelDotDot             = "dot-dot"
	ribHoldsApproval        = ".TestRibHolds reason \""
	hookWriteEditFile       = "internal/le/hookruntime/writeedit.go"
	hookBashFile            = "internal/le/hookruntime/bash.go"
	hookPostWriteFile       = "internal/le/hookruntime/postwrite.go"
	toolWriteName           = "Write"
	weakeningCheckName      = "writeWeakening"
	weakenedProposedFixture = "test-weakened proposed"
)

// proposedVerbAnchor is the registration line that binds the weakened proposed
// action to its command surface.
const proposedVerbAnchor = `Verb:   "proposed"`

// labelTrailingNewline is the generator label for a trailing-newline input.
const (
	labelTrailingNewline = "trailing-newline"
)
