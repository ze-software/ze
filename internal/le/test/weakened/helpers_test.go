// Design: docs/architecture/testing/test-health.md -- stale verdicts refuse, and a helper's change reaches its callers.
// Related: helpers.go -- the call rule; testweakened.go -- the stale-row refusal in Check.
package testweakened

import (
	"slices"
	"strings"
	"testing"
)

// commitWeakenedFixture commits every pending change of the fixture
// repository with message, which may carry approval trailers.
func commitWeakenedFixture(t *testing.T, root, message string) {
	t.Helper()
	if !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root, "-c", "user.email=t@t", "-c", "user.name=t",
			"-c", "commit.gpgsign=false", "commit", "-q", "-m", message) {
		t.Fatal("commit the fixture change")
	}
}

// TestCheckRefusesAChangedUnitWhoseApprovalIsStale is the silent stale row.
//
// VALIDATES: a judged unit edited again after its verdict, with no structural
// finding, is refused by Check with a problem naming the unit, the stale
// reason, the approve command and the stale row's removal; the same row at the
// judged content still admits the change and carries its trailer.
// PREVENTS: a commit landing text no judge saw because the changed-unit loop
// dropped why its row could not claim it.
func TestCheckRefusesAChangedUnitWhoseApprovalIsStale(t *testing.T) {
	t.Parallel()
	root := newParityRepository(t)
	const path = "pkg/a_test.go"
	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) { require.Equal(t, 2, got) }\n")
	const reason = "judge: the corrected expectation"
	if _, err := Approve(root, fixtureSession, "pkg.TestA", reason); err != nil {
		t.Fatal(err)
	}
	judged := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	if judged.ExitCode() != 0 || !slices.Equal(judged.Approvals.Trailers, []string{ApprovalTrailerLine("pkg.TestA", reason)}) {
		t.Fatalf("judged content = code %d, trailers %q, problems %v; want the verdict carried",
			judged.ExitCode(), judged.Approvals.Trailers, judged.Problems)
	}

	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) { require.Equal(t, 3, got) }\n")
	stale := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	if stale.ExitCode() != 1 {
		t.Fatalf("stale row = code %d, problems %v; want a refusal", stale.ExitCode(), stale.Problems)
	}
	for _, want := range []string{
		"pkg/a_test.go changes pkg.TestA after its ze-test-judge verdict",
		"is stale (the unit changed after the verdict",
		ApproveCommand("pkg.TestA"),
		"remove the stale row from " + fixtureApproval,
	} {
		if !containsProblem(stale.Problems, want) {
			t.Fatalf("stale refusal misses %q: %v", want, stale.Problems)
		}
	}
	if len(stale.Problems) != 1 || len(stale.Approvals.Trailers) != 0 {
		t.Fatalf("stale row = problems %v, trailers %q; want one refusal and no verdict", stale.Problems, stale.Approvals.Trailers)
	}
}

// TestCheckStaysSilentOnALandedApproval keeps the spent-row contract.
//
// VALIDATES: a row whose trailer a landed commit carries is loaded as landed
// and not stale, and says nothing about a later change of its unit that has
// no structural finding.
// PREVENTS: the stale refusal reading a spent row as a pending verdict.
func TestCheckStaysSilentOnALandedApproval(t *testing.T) {
	t.Parallel()
	root := newParityRepository(t)
	const path = "pkg/a_test.go"
	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) { require.Equal(t, 2, got) }\n")
	const reason = "judge: the corrected expectation"
	if _, err := Approve(root, fixtureSession, "pkg.TestA", reason); err != nil {
		t.Fatal(err)
	}
	commitWeakenedFixture(t, root, "land the judged change\n\n"+ApprovalTrailerLine("pkg.TestA", reason))
	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) { require.Equal(t, 3, got) }\n")
	rows, problems := LoadApprovals(root, WeakeningApprovals(fixtureSession))
	if len(problems) != 0 || len(rows) != 1 || rows[0].Carrier == "" || rows[0].Stale != "" {
		t.Fatalf("landed row = rows %+v, problems %v; want one landed row that is not stale", rows, problems)
	}
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	if result.ExitCode() != 0 || len(result.Approvals.Trailers) != 0 {
		t.Fatalf("landed row = code %d, trailers %q, problems %v; want silence", result.ExitCode(), result.Approvals.Trailers, result.Problems)
	}
}

const (
	helperFixtureA = "hpkg/a_test.go"
	helperFixtureB = "hpkg/b_test.go"
	helperFixtureN = "hpkg/new_test.go"
	// helperBaselineA holds the helper requireReady, a helper no test calls
	// (unused), TestMain, which calls requireReady and is no test, a direct
	// caller, a caller through viaWrapper in the other file, and TestAlone,
	// which names requireReady only as a method after a selector.
	helperBaselineA = "package hpkg\n" +
		"func requireReady(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, \"marker one\", got)\n}\n" +
		"func unused(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n" +
		"func TestMain(m *testing.M) { requireReady(nil) }\n" +
		"func TestDirect(t *testing.T) {\n\tx := 1\n\trequireReady(t)\n\t_ = x\n}\n" +
		"func TestIndirect(t *testing.T) { viaWrapper(t) }\n" +
		"func TestAlone(t *testing.T) {\n\ts.requireReady()\n\trequire.True(t, ok)\n}\n"
	// helperBaselineB is a file the fixture commits never carry: a wrapper,
	// and a direct caller whose verdict such a commit must not touch.
	helperBaselineB = "package hpkg\n" +
		"func viaWrapper(t *testing.T) { requireReady(t) }\n" +
		"func TestOther(t *testing.T) { requireReady(t) }\n"
)

// helperFixture commits the helper package beside any extra baseline files,
// then writes edit(helperBaselineA) as the change the commit carries.
func helperFixture(t *testing.T, extra map[string]string, edit func(string) string) string {
	t.Helper()
	root := newParityRepository(t)
	writeParityFile(t, root, helperFixtureA, helperBaselineA)
	writeParityFile(t, root, helperFixtureB, helperBaselineB)
	for path, content := range extra {
		writeParityFile(t, root, path, content)
	}
	commitWeakenedFixture(t, root, "helper baseline")
	writeParityFile(t, root, helperFixtureA, edit(helperBaselineA))
	return root
}

func approveFixtureUnits(t *testing.T, root, reason string, units ...string) {
	t.Helper()
	for _, unit := range units {
		if _, err := Approve(root, fixtureSession, unit, reason); err != nil {
			t.Fatal(err)
		}
	}
}

func findingUnits(findings []Finding) []string {
	units := make([]string, 0, len(findings))
	for _, finding := range findings {
		units = append(units, finding.Unit())
	}
	slices.Sort(units)
	return units
}

// TestCheckNamesAHelperWeakeningByTheHelper is the helper's own verdict.
//
// VALIDATES: an assertion removed inside a helper is a finding of the helper
// itself, even when a caller carries a live row for its own change in the same
// commit; only the helper's own row admits it, and that row goes stale, and
// refuses, once the helper changes again.
// PREVENTS: a caller's row, which judged the caller's text, admitting a
// helper weakening its judge never saw.
func TestCheckNamesAHelperWeakeningByTheHelper(t *testing.T) {
	t.Parallel()
	root := helperFixture(t, nil, func(text string) string {
		text = strings.Replace(text, "\tx := 1\n", "\tx := 2\n", 1)
		return strings.Replace(text, "\trequire.Equal(t, \"marker one\", got)\n", "", 1)
	})
	approveFixtureUnits(t, root, "judge: TestDirect's new value is the documented one", "hpkg.TestDirect")
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}})
	if result.ExitCode() != 1 || !slices.Equal(findingUnits(result.Findings), []string{"hpkg.requireReady"}) {
		t.Fatalf("helper weakening = code %d, findings %v, problems %v; want a refusal of hpkg.requireReady",
			result.ExitCode(), result.Findings, result.Problems)
	}
	if !containsDetail(result.Findings[0].Details, "removing assertions") ||
		!containsProblem(result.Problems, "weakens hpkg.requireReady") {
		t.Fatalf("helper weakening = findings %v, problems %v", result.Findings, result.Problems)
	}

	const reason = "judge: the removed check is made by the caller"
	approveFixtureUnits(t, root, reason, "hpkg.requireReady")
	approved := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}})
	if approved.ExitCode() != 0 || !slices.Contains(approved.Approvals.Trailers, ApprovalTrailerLine("hpkg.requireReady", reason)) {
		t.Fatalf("approved helper = code %d, trailers %q, problems %v", approved.ExitCode(), approved.Approvals.Trailers, approved.Problems)
	}

	// Another weakening of the helper than the one its judge read.
	writeParityFile(t, root, helperFixtureA, strings.Replace(helperBaselineA,
		"\trequire.NoError(t, err)\n\trequire.Equal(t, \"marker one\", got)\n", "\trequire.Equal(t, \"marker one\", got)\n", 1))
	stale := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}})
	if stale.ExitCode() != 1 || !containsProblem(stale.Problems, "weakens hpkg.requireReady, and the approval naming it is stale") {
		t.Fatalf("re-edited helper = code %d, problems %v; want the helper's stale row refused", stale.ExitCode(), stale.Problems)
	}
}

// TestCheckKeepsTheFindingOfAHelperNoTestCalls is the uncalled helper.
//
// VALIDATES: a weakened helper that no test calls is reported under its own
// name, and no test becomes a changed unit for it.
// PREVENTS: an uncalled helper's weakening vanishing for want of a caller.
func TestCheckKeepsTheFindingOfAHelperNoTestCalls(t *testing.T) {
	t.Parallel()
	root := helperFixture(t, nil, func(text string) string {
		return strings.Replace(text, "\trequire.Equal(t, 1, got)\n", "", 1)
	})
	approveFixtureUnits(t, root, "judge: reserved for another change", "hpkg.TestDirect")
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}})
	if result.ExitCode() != 1 || !slices.Equal(findingUnits(result.Findings), []string{"hpkg.unused"}) {
		t.Fatalf("uncalled helper = code %d, findings %v, problems %v", result.ExitCode(), result.Findings, result.Problems)
	}
	if len(result.Approvals.Trailers) != 0 {
		t.Fatalf("an uncalled helper spent %q", result.Approvals.Trailers)
	}
}

// TestCheckSpendsOnlyTheCommittedCallersOfAChangedHelper is the helper as a
// changed unit.
//
// VALIDATES: a helper change the detector does not count (a literal) makes
// each existing test of the commit's files that calls it, directly or through
// a wrapper in a file the commit does not carry, a changed unit, so its live
// row is carried and spent. A caller in a file the commit does not carry, a test
// that names the helper only after a selector, and TestMain keep their rows.
// PREVENTS: a caller's verdict staying live past the commit that changed what
// it judged, and a verdict for a file the commit does not touch being spent.
func TestCheckSpendsOnlyTheCommittedCallersOfAChangedHelper(t *testing.T) {
	t.Parallel()
	root := helperFixture(t, nil, func(text string) string {
		return strings.Replace(text, "marker one", "marker two", 1)
	})
	const reason = "judge: the new marker is the message the hook prints"
	// TestDirect's own text is unchanged by the literal edit, so only the
	// helper can make it a changed unit.
	approveFixtureUnits(t, root, reason,
		"hpkg.TestDirect", "hpkg.TestIndirect", "hpkg.TestOther", "hpkg.TestAlone", "hpkg.TestMain")
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}})
	if result.ExitCode() != 0 || len(result.Findings) != 0 {
		t.Fatalf("helper literal change = code %d, findings %v, problems %v", result.ExitCode(), result.Findings, result.Problems)
	}
	trailers := slices.Sorted(slices.Values(result.Approvals.Trailers))
	want := []string{ApprovalTrailerLine("hpkg.TestDirect", reason), ApprovalTrailerLine("hpkg.TestIndirect", reason)}
	slices.Sort(want)
	if !slices.Equal(trailers, want) {
		t.Fatalf("helper literal trailers = %q, want %q", result.Approvals.Trailers, want)
	}
	sites := make([]string, 0, len(result.ApprovalSites))
	for _, site := range result.ApprovalSites {
		sites = append(sites, site.Unit)
	}
	slices.Sort(sites)
	if !slices.Equal(sites, []string{"hpkg.TestDirect", "hpkg.TestIndirect"}) {
		t.Fatalf("approval sites = %q, want the direct and the wrapper-reached committed callers", sites)
	}
}

// TestCheckAttributesNoNewOrRemovedTest is the committed side's population.
//
// VALIDATES: a new test file that calls the changed helper lends its calls
// but its new test spends no row, and a removed file lends no call, so a test
// reaching the helper only through the removed wrapper is not attributed.
// PREVENTS: a new test spending a semantic verdict, and a removed file
// deciding which tests changed.
func TestCheckAttributesNoNewOrRemovedTest(t *testing.T) {
	t.Parallel()
	literal := func(text string) string { return strings.Replace(text, "marker one", "marker two", 1) }
	t.Run("new file", func(t *testing.T) {
		t.Parallel()
		root := helperFixture(t, nil, literal)
		writeParityFile(t, root, helperFixtureN, "package hpkg\nfunc TestNew(t *testing.T) { requireReady(t) }\n")
		approveFixtureUnits(t, root, "judge: reserved", "hpkg.TestNew")
		result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA, helperFixtureN}})
		if result.ExitCode() != 0 || len(result.Approvals.Trailers) != 0 {
			t.Fatalf("new caller = code %d, trailers %q, problems %v; want no row spent", result.ExitCode(), result.Approvals.Trailers, result.Problems)
		}
	})
	t.Run("removed file", func(t *testing.T) {
		t.Parallel()
		root := helperFixture(t, nil, literal)
		approveFixtureUnits(t, root, "judge: reserved", "hpkg.TestIndirect")
		result := Check(Request{
			Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}, Removed: []string{helperFixtureB},
		})
		if slices.Contains(result.Approvals.Trailers, ApprovalTrailerLine("hpkg.TestIndirect", "judge: reserved")) {
			t.Fatalf("a removed wrapper attributed TestIndirect: trailers %q", result.Approvals.Trailers)
		}
		// The file stem is weakenedUnits' residual unit: the removed file's
		// package clause sits outside every declaration and is emptied too.
		if !slices.Equal(findingUnits(result.Findings), []string{"hpkg.TestOther", "hpkg.b_test", "hpkg.viaWrapper"}) {
			t.Fatalf("removed file findings = %v; want its own deletions and its stem", result.Findings)
		}
	})
}

// TestCheckReadsBrokenFilesByWhoCarriesThem is the parse contract.
//
// VALIDATES: a test file at the anchor that does not parse and that the
// commit does not carry is skipped, so the commit is judged; one the commit
// carries refuses with the file named.
// PREVENTS: a broken file another change left behind refusing every commit
// that touches its package, and a broken committed file hiding its callers.
func TestCheckReadsBrokenFilesByWhoCarriesThem(t *testing.T) {
	t.Parallel()
	literal := func(text string) string { return strings.Replace(text, "marker one", "marker two", 1) }
	const broken = "package hpkg\nfunc TestBroken( {\n"
	t.Run("not carried", func(t *testing.T) {
		t.Parallel()
		root := helperFixture(t, map[string]string{"hpkg/broken_test.go": broken}, literal)
		result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA}})
		if result.ExitCode() != 0 {
			t.Fatalf("an uncarried broken file = code %d, problems %v; want the commit judged", result.ExitCode(), result.Problems)
		}
	})
	t.Run("carried", func(t *testing.T) {
		t.Parallel()
		root := helperFixture(t, nil, literal)
		writeParityFile(t, root, helperFixtureN, broken)
		result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{helperFixtureA, helperFixtureN}})
		if result.ExitCode() != 2 || !containsProblem(result.Problems, "cannot read the helper calls in "+helperFixtureN) {
			t.Fatalf("a carried broken file = code %d, problems %v; want cannot-run naming it", result.ExitCode(), result.Problems)
		}
	})
}
