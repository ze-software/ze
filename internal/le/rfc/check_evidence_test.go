package rfc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// backlogFile is an evidence backlog holding one count per kind. Every case
// varies only the {not-applicable} count, the kind its fixture row carries;
// the other two kinds are held at zero.
func backlogFile(notApplicable string) string {
	return "# fixture\nfeature-declined 0\nlower-layer 0\nnot-applicable " + notApplicable + "\n"
}

// backlogCounts is backlogFile as the parsed HEAD^ counts.
func backlogCounts(notApplicable int) map[string]int {
	return map[string]int{AnnotationFeatureDeclined: 0, AnnotationLowerLayer: 0, AnnotationNotApplicable: notApplicable}
}

// VALIDATES: the exclusion evidence ratchet -- an untouched row lacking
// evidence is carried by the committed count; a row the commit added or
// changed is refused by id and is NOT also counted; a count that fell must be
// lowered in the file; a count that rose names the rows, tells the author to
// give rows their evidence and never to raise the line; a committed line that
// rose against HEAD^ or the pushed branch is refused naming the kind and both
// counts; a malformed committed file and a kind counted twice are refused; a kind at zero is refused row by row; git that cannot be read
// refuses; only an absent HEAD^ passes, and says so; a missing or partial
// count file is refused.
// METHOD: legacy {not-applicable} rows of the fixture RFC, judged against
// HEAD^ baselines and count files varied per case.
func TestExclusionEvidenceRatchet(t *testing.T) {
	legacy := Requirement{RFC: selftestStem, RID: "RFC9999-2-1", Level: levelMust, Section: "2",
		Source: selftestSummaryRel, Line: 7, Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "ze sends no widget"}}
	second := legacy
	second.RID = "RFC9999-2-2"
	prints := map[string]string{legacy.RID: evidenceFingerprint(&legacy), second.RID: evidenceFingerprint(&second)}
	changed := map[string]string{legacy.RID: "2\x00not-applicable\x00an older reason", second.RID: evidenceFingerprint(&second)}
	onlySecond := map[string]string{second.RID: evidenceFingerprint(&second)}
	read := func(prints map[string]string, committed int) evidenceBaseline {
		return evidenceBaseline{state: priorRead, prints: prints, backlog: backlogCounts(committed), backlogAt: priorRevision}
	}
	pushedAt := func(base evidenceBaseline, committed int) evidenceBaseline {
		base.pushed, base.pushedAt = backlogCounts(committed), backlogRevision
		return base
	}
	cases := []struct {
		name    string
		backlog string
		rows    []Requirement
		base    evidenceBaseline
		want    []string
		notice  string
	}{
		{"an untouched row the count carries", backlogFile("1"), []Requirement{legacy}, read(prints, 1), nil, ""},
		{"a new row is refused and not counted", backlogFile("1"), []Requirement{legacy, second}, read(map[string]string{legacy.RID: prints[legacy.RID]}, 1),
			[]string{"RFC9999-2-2 is annotated {not-applicable} and lacks the evidence"}, ""},
		{"a changed row is refused and not counted", backlogFile("1"), []Requirement{legacy, second}, read(changed, 1),
			[]string{"RFC9999-2-1 is annotated {not-applicable} and lacks the evidence"}, ""},
		{"a row of a new summary", backlogFile("0"), []Requirement{legacy}, read(onlySecond, 0),
			[]string{"RFC9999-2-1 is annotated {not-applicable}"}, ""},
		{"no HEAD^ to compare", backlogFile("1"), []Requirement{legacy}, evidenceBaseline{state: priorAbsent, reason: "the checkout has no commit before HEAD"},
			nil, "no HEAD^"},
		{"git that cannot be read", backlogFile("1"), []Requirement{legacy}, evidenceBaseline{state: priorUnreadable, reason: "git ls-tree HEAD^ failed"},
			[]string{"cannot read HEAD^", "git ls-tree HEAD^ failed"}, ""},
		{"a count that fell", backlogFile("2"), []Requirement{legacy}, read(prints, 2),
			[]string{"set the line to `not-applicable 1`"}, ""},
		{"a count that rose", backlogFile("1"), []Requirement{legacy, second}, read(prints, 1),
			[]string{"2 {not-applicable} rows the commit left alone lack their evidence, and the committed count is 1",
				"the line never rises: give 1 of these rows the evidence", selftestSummaryRel + ":7 RFC9999-2-1", selftestSummaryRel + ":7 RFC9999-2-2"}, ""},
		{"a committed line that rose against HEAD^", backlogFile("2"), []Requirement{legacy, second}, read(prints, 1),
			[]string{"the {not-applicable} count rose from 1 at HEAD^ to 2", "`not-applicable 1`"}, ""},
		{"a committed line that fell against HEAD^", backlogFile("1"), []Requirement{legacy}, read(prints, 2), nil, ""},
		{"a committed line HEAD^ raised above the pushed branch", backlogFile("2"), []Requirement{legacy, second}, pushedAt(read(prints, 2), 1),
			[]string{"the {not-applicable} count rose from 1 at origin/main to 2", "`not-applicable 1`"}, ""},
		{"a committed line at the pushed count", backlogFile("1"), []Requirement{legacy}, pushedAt(read(prints, 2), 1), nil, ""},
		{"a committed file that does not parse", backlogFile("1"), []Requirement{legacy},
			evidenceBaseline{state: priorRead, prints: prints, malformed: "the committed rfc/exclusion-evidence-backlog.txt at HEAD^ is malformed"},
			[]string{"at HEAD^ is malformed"}, ""},
		{"a kind counted twice", backlogFile("1") + "not-applicable 1\n", []Requirement{legacy}, read(prints, 1),
			[]string{"a second line counts {not-applicable}"}, ""},
		{"a kind at zero", backlogFile("0"), []Requirement{legacy}, read(prints, 0),
			[]string{"RFC9999-2-1 is annotated {not-applicable} and lacks the evidence"}, ""},
		{"no count file", "", []Requirement{legacy}, read(prints, 1), []string{"cannot read the committed evidence counts"}, ""},
		{"a kind with no line", "feature-declined 0\nlower-layer 0\n", []Requirement{legacy}, read(prints, 1),
			[]string{"no line counts {not-applicable}"}, ""},
		{"an unknown kind", backlogFile("1") + "gap 3\n", []Requirement{legacy}, read(prints, 1),
			[]string{"'gap' is not a kind this ratchet counts"}, ""},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := t.TempDir()
			files := map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource}
			if one.backlog != "" {
				files[evidenceBacklogRel] = one.backlog
			}
			writeFixtureFiles(t, tree, files)
			errs, notice := checkExclusionEvidence(tree, one.rows, one.base)
			// Every refusing case refuses once; its wants are parts of that one message.
			if refusals := min(len(one.want), 1); len(errs) != refusals {
				t.Fatalf("got %d refusals, want %d: %q", len(errs), refusals, errs)
			}
			joined := strings.Join(errs, "\n")
			for _, want := range one.want {
				if !strings.Contains(joined, want) {
					t.Errorf("refusals %q do not say %q", errs, want)
				}
			}
			if !strings.Contains(notice, one.notice) || (one.notice == "") != (notice == "") {
				t.Errorf("notice %q, want one saying %q", notice, one.notice)
			}
		})
	}
}

// VALIDATES: every refusal of the ratchet names the row or the file line and
// the edit that fixes it (ai/rules/user-facing-errors.md): a row refusal names
// the summary line, the id and the format; an untouched row whose evidence
// form no longer passes its check is refused by id, naming the failing check,
// and is not counted, so a commit that breaks one row's evidence while it
// backfills another cannot keep the count level.
// METHOD: an untouched row whose quoted evidence is not in its cited section,
// judged once under a committed count of 0 (refused by id) and then beside a
// legacy row with no evidence form under a committed count of 1, which that
// legacy row alone meets.
func TestExclusionEvidenceRefusalsNameTheFix(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource,
		evidenceBacklogRel: backlogFile("0")})
	broken := evidenceRow(t, "RFC9999-2-1", "2",
		`not-applicable: "A receiver MUST NOT reorder the frames of one widget."; binds a receiver; ze is a speaker; it reorders nothing`)
	errs, _ := checkExclusionEvidence(tree, []Requirement{broken},
		evidenceBaseline{state: priorRead, prints: map[string]string{broken.RID: evidenceFingerprint(&broken)}, backlog: backlogCounts(0)})
	if len(errs) != 1 {
		t.Fatalf("got %d refusals, want the row: %q", len(errs), errs)
	}
	for _, want := range []string{selftestSummaryRel + ":7", "RFC9999-2-1", "is in section 3", notApplicableFormat} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("the row refusal %q does not say %q", errs[0], want)
		}
	}
	writeFixtureFiles(t, tree, map[string]string{evidenceBacklogRel: backlogFile("1")})
	legacy := evidenceRow(t, "RFC9999-2-2", "2", "not-applicable: ze sends no widget")
	swapped, _ := checkExclusionEvidence(tree, []Requirement{broken, legacy},
		evidenceBaseline{state: priorRead, prints: map[string]string{broken.RID: evidenceFingerprint(&broken),
			legacy.RID: evidenceFingerprint(&legacy)}, backlog: backlogCounts(1)})
	if len(swapped) != 1 {
		t.Fatalf("got %d refusals, want the broken row alone: %q", len(swapped), swapped)
	}
	for _, want := range []string{selftestSummaryRel + ":7: RFC9999-2-1", "no longer passes", "is in section 3", "restore"} {
		if !strings.Contains(swapped[0], want) {
			t.Errorf("the broken-evidence refusal %q does not say %q", swapped[0], want)
		}
	}
}

// VALIDATES: a {lower-layer} layer names a component a reader can check: a
// vendored module vendor/modules.txt lists and this checkout carries, a kernel
// CONFIG_ option Ze's kernel config builds in (=y, as the build verifies a
// required option, and no later `is not set`, an include counting after every
// profile fragment as the builder layers it) and a require manifest lists, an
// include's own included, or a lower-case package of the Go standard library
// whose directory under $(go env GOROOT)/src holds a non-test .go file;
// cmd/, vendor/, a testdata, `_` or `.` segment, `.` and `..` are refused.
// METHOD: the parser reads the path out of the layer, and the evidence reader
// judges each form against a fixture checkout holding one vendored module,
// the two kernel config fragments and one included fragment; a checkout
// holding no kernel config refuses every linux: path.
func TestLowerLayerNamesItsLayerPath(t *testing.T) {
	parsed, err := parseAnnotation("lower-layer: Linux IPv6 neighbor discovery, linux:CONFIG_IPV6; "+
		"internal/x/nd.go::install installs the address, and the kernel answers", "fixture")
	if err != nil {
		t.Fatalf("a lower-layer payload naming its layer path did not parse: %v", err)
	}
	if parsed.LayerPath != "linux:CONFIG_IPV6" {
		t.Errorf("LayerPath is %q, want linux:CONFIG_IPV6", parsed.LayerPath)
	}
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{
		"vendor/github.com/insomniacslk/dhcp/dhcpv6/doc.go": "package dhcpv6\n",
		vendorModulesRel: "# github.com/insomniacslk/dhcp v0.0.0-20240829085014-a3a4c1f04475\n## explicit; go 1.20\n" +
			"github.com/insomniacslk/dhcp/dhcpv6\n# github.com/gone/module v1.0.0\n## explicit\n",
		kernelConfigRels[0]: "# ze-include: inc\nCONFIG_IPV6=y\n# CONFIG_WIDGET is not set\nCONFIG_LOCALVERSION=\"-ze\"\n" +
			"CONFIG_LATER=y\nCONFIG_UNREQUIRED=y\n",
		kernelConfigRels[1]: "# runtime\nCONFIG_XFRM_USER=y\nCONFIG_NF_TABLES=n\n# CONFIG_LATER is not set\n" +
			"CONFIG_MODULAR=m\n# CONFIG_INC is not set\n",
		kernelRequireRel(kernelConfigRels[0]): "# fixture\nCONFIG_IPV6\nCONFIG_LATER\n",
		kernelRequireRel(kernelConfigRels[1]): "CONFIG_XFRM_USER=y\nCONFIG_MODULAR\n",
		// The builder (resolveKernelProfile) layers every include after
		// every profile fragment, and verifies the include's own manifest.
		"tools/kernel-builder/common/inc.config":  "CONFIG_INC=y\nCONFIG_INCONLY=y\n",
		"tools/kernel-builder/common/inc.require": "CONFIG_INC\nCONFIG_INCONLY\n",
	})
	cases := []struct{ path, refused string }{
		{"linux:CONFIG_IPV6", ""},
		{"linux:CONFIG_XFRM_USER", ""},
		{"linux:CONFIG_INC", ""},
		{"linux:CONFIG_INCONLY", ""},
		{"linux:CONFIG_MODULAR", "is not enabled"},
		{"go:crypto/x509", ""},
		{"vendor/github.com/insomniacslk/dhcp", ""},
		{"vendor/github.com/insomniacslk/dhcp/dhcpv6", "names no module vendor/modules.txt lists"},
		{"", "names no component a reader can check"},
		{"vendor/github.com/gone/module", "cannot be opened in this checkout"},
		{"vendor/github.com/nowhere/module", "names no module vendor/modules.txt lists"},
		{"vendor/.", "holds a `.` segment"},
		{"vendor/modules.txt", "names no module vendor/modules.txt lists"},
		{"vendor/github.com", "names no module vendor/modules.txt lists"},
		{"vendor/GITHUB.com/insomniacslk/dhcp", "names no module vendor/modules.txt lists"},
		{"vendor/../go.mod", "climbs out of its source tree"},
		{"linux:CONFIG_LATER", "is not enabled"},
		{"linux:CONFIG_UNREQUIRED", "listed in none of gokrazy/kernel/kernel.require, gokrazy/kernel/runtime.require, tools/kernel-builder/common/inc.require"},
		{"linux:CONFIG_WIDGET", "is not enabled"},
		{"linux:CONFIG_NF_TABLES", "is not enabled"},
		{"linux:CONFIG_LOCALVERSION", "is not enabled"},
		{"linux:net/ipv6/ndisc.c", "names no CONFIG_ option"},
		{"linux:CONFIG_X/../y", "climbs out of its source tree"},
		{"go:crypto/nosuchpackage", "is not a package of the Go standard library"},
		{"go:github.com/x/y", "is not a package of the Go standard library"},
		{"go:.", "holds a `.` segment"},
		{"go:/", "is not a package of the Go standard library"},
		{"go:NET/HTTP", "is not a package of the Go standard library"},
		{"go:net/http/testdata", "which the go tool ignores"},
		{"go:go/doc/testdata", "which the go tool ignores"},
		{"go:net/_private", "which the go tool ignores"},
		{"go:net/.hidden", "which the go tool ignores"},
		{"go:cmd/go", "is not a package of the Go standard library"},
		{"go:vendor/golang.org/x/crypto/chacha20", "is not a package of the Go standard library"},
		{"go:../src/net", "climbs out of its source tree"},
	}
	reader := newEvidenceReader(tree)
	for _, one := range cases {
		missing, lacks := reader.layerPathGap(one.path)
		if one.refused == "" {
			if lacks {
				t.Errorf("%q was refused: %s", one.path, missing)
			}
			continue
		}
		if !lacks || !strings.Contains(missing, one.refused) {
			t.Errorf("%q answered (%q, %v), want a refusal saying %q", one.path, missing, lacks, one.refused)
		}
	}
	bare := t.TempDir()
	missing, lacks := newEvidenceReader(bare).layerPathGap("linux:CONFIG_IPV6")
	if !lacks || !strings.Contains(missing, "cannot read") {
		t.Errorf("a checkout with no kernel config answered (%q, %v), want a refusal", missing, lacks)
	}
}

// VALIDATES: the prior revision is told apart three ways: a checkout with no
// git and a root commit have no HEAD^ (absent), a repository with two commits
// has one (read), and a shallow clone, a parent whose object is gone and a
// repository git cannot read are unreadable, never absent. Each unreadable
// reason carries its own fix, and the ratchet's refusal ends with that fix
// rather than a fixed sentence about another command.
// METHOD: fixture directories driven through priorRevisionState and
// checkExclusionEvidence; the parent object is removed from the loose object
// store `git commit` wrote it to.
func TestPriorRevisionState(t *testing.T) {
	refusesWith := func(t *testing.T, tree, fix string) {
		t.Helper()
		state, reason := priorRevisionState(tree)
		if state != priorUnreadable {
			t.Errorf("answered %v (%s), want unreadable", state, reason)
			return
		}
		writeFixtureFiles(t, tree, map[string]string{evidenceBacklogRel: backlogFile("0")})
		errs, _ := checkExclusionEvidence(tree, nil, evidenceBaseline{state: state, reason: reason})
		if len(errs) != 1 || !strings.Contains(errs[0], fix) || strings.Contains(errs[0], "Repair the checkout until `git cat-file") {
			t.Errorf("the refusal %q does not end with its own fix %q alone", errs, fix)
		}
	}
	if state, _ := priorRevisionState(t.TempDir()); state != priorAbsent {
		t.Errorf("a directory with no git answered %v, want absent", state)
	}
	root := t.TempDir()
	gitFixture(t, root, []string{"init", "-q"})
	if state, reason := priorRevisionState(root); state != priorAbsent {
		t.Errorf("a repository with no commit answered %v (%s), want absent", state, reason)
	}
	writeFixtureFiles(t, root, map[string]string{"a.txt": "a\n"})
	commitFixture(t, root, "one")
	if state, _ := priorRevisionState(root); state != priorAbsent {
		t.Errorf("a root commit answered %v, want absent", state)
	}
	commitFixtureEmptyTip(t, root)
	if state, reason := priorRevisionState(root); state != priorRead {
		t.Errorf("two commits answered %v (%s), want read", state, reason)
	}
	shallow := filepath.Join(t.TempDir(), "shallow")
	gitFixture(t, root, []string{"clone", "-q", "--depth", "1", "file://" + root, shallow})
	if state, _ := priorRevisionState(shallow); state != priorUnreadable {
		t.Errorf("a shallow clone answered %v, want unreadable", state)
	}
	t.Run("a shallow clone", func(t *testing.T) { refusesWith(t, shallow, "git fetch --unshallow") })
	parent, failure := gitAnswer(root, "rev-parse", "HEAD^")
	if failure != "" {
		t.Fatalf("name the fixture's parent: %s", failure)
	}
	if err := os.Remove(filepath.Join(root, ".git", "objects", parent[:2], parent[2:])); err != nil {
		t.Fatalf("drop the fixture's parent object: %v", err)
	}
	if state, reason := priorRevisionState(root); state != priorUnreadable {
		t.Errorf("a parent whose object is gone answered %v (%s), want unreadable", state, reason)
	}
	t.Run("a parent whose object is gone", func(t *testing.T) { refusesWith(t, root, "git fsck") })
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("garbage\n"), 0o600); err != nil {
		t.Fatalf("corrupt the fixture HEAD: %v", err)
	}
	if state, _ := priorRevisionState(root); state != priorUnreadable {
		t.Errorf("a corrupt repository answered %v, want unreadable", state)
	}
	t.Run("a corrupt repository", func(t *testing.T) { refusesWith(t, root, "repair the checkout until that command answers") })
}

// VALIDATES: through the real entry point (check), a commit that raises the
// committed backlog line against HEAD^ is refused naming the kind and both
// counts, and a commit that lowers it while backfilling the row passes.
// METHOD: two-commit fixture repositories (HEAD^ and HEAD), checked out
// detached as `./le verify worktree` does, over the check fixture corpus with
// one SHOULD row annotated {not-applicable}.
func TestCheckRefusesABacklogRiseAgainstPrior(t *testing.T) {
	const sentence = "A speaker MUST send the widget."
	legacy := "- [ ] [RFC9999-2-3] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends no widget}"
	backfilled := "- [ ] [RFC9999-2-3] [SHOULD] " + sentence + ` (§2) {not-applicable: "` + sentence +
		`"; binds a speaker; ze is a receiver; ze sends no widget}`
	base := fixtureCorpus()
	base[selftestSummaryRel] = quoteFixtureSummary(legacy)
	base[evidenceBacklogRel] = backlogFile("1")

	rose := commitFixtureTip(t, base, map[string]string{evidenceBacklogRel: backlogFile("2")}, nil)
	gitFixture(t, rose, []string{"checkout", "-q", "--detach"})
	report, code := Check(rose, nil)
	if code != 1 {
		t.Fatalf("a backlog rise answered %d, want 1:\n%s", code, report.Text())
	}
	if !strings.Contains(strings.Join(report.Violations, "\n"), "the {not-applicable} count rose from 1 at HEAD^ to 2") {
		t.Errorf("the rise was not refused naming the kind and both counts:\n%s", report.Text())
	}

	fell := commitFixtureTip(t, base, map[string]string{selftestSummaryRel: quoteFixtureSummary(backfilled),
		evidenceBacklogRel: backlogFile("0")}, nil)
	gitFixture(t, fell, []string{"checkout", "-q", "--detach"})
	if report, code := Check(fell, nil); code != 0 {
		t.Fatalf("a backfill that lowers the line answered %d, want 0:\n%s", code, report.Text())
	}
}

// VALIDATES: priorEvidence reads the committed counts from the newest commit
// that holds the backlog file, at HEAD^ and at the pushed branch: a file
// deleted in HEAD^ still compares the counts it held before, so deleting it in
// one commit and restoring it raised in the next is refused; a history that
// never held it compares nothing; a malformed committed file refuses, telling
// the author to commit a valid one, while its counts compare from the newest
// ancestor whose copy parses and the rows are still judged; summaries HEAD^
// could not read refuse.
// METHOD: fixture repositories of three commits, the pushed branch set with
// `git update-ref refs/remotes/origin/main`, judged through priorEvidence and
// checkExclusionEvidence.
func TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog(t *testing.T) {
	commits := func(t *testing.T, bodies ...string) string {
		t.Helper()
		root := t.TempDir()
		gitFixture(t, root, []string{"init", "-q"})
		writeFixtureFiles(t, root, map[string]string{"a.txt": "a\n"})
		for i, body := range bodies {
			if body == "" && i > 0 {
				commitFixtureEmptyTip(t, root)
				continue
			}
			if body != "" {
				writeFixtureFiles(t, root, map[string]string{evidenceBacklogRel: body})
			}
			commitFixture(t, root, "commit "+strconv.Itoa(i))
		}
		return root
	}

	deleted := commits(t, backlogFile("1"), fixtureRemoved)
	gitFixture(t, deleted, []string{"update-ref", "refs/remotes/origin/main", "HEAD"})
	commitFixtureEmptyTip(t, deleted)
	holder, failure := gitAnswer(deleted, "rev-parse", "HEAD~2")
	if failure != "" {
		t.Fatalf("name the commit holding the file: %s", failure)
	}
	base := priorEvidence(deleted, map[string]string{}, map[string]bool{}, true)
	if base.state != priorRead || base.backlog[AnnotationNotApplicable] != 1 || !strings.Contains(base.backlogAt, holder) {
		t.Errorf("HEAD^ deleted the file: got state %v, counts %v at %q, want 1 read at %s", base.state, base.backlog, base.backlogAt, holder)
	}
	if base.pushed[AnnotationNotApplicable] != 1 || !strings.Contains(base.pushedAt, holder) {
		t.Errorf("the pushed branch deleted the file: got counts %v at %q, want 1 at %s", base.pushed, base.pushedAt, holder)
	}
	writeFixtureFiles(t, deleted, map[string]string{evidenceBacklogRel: backlogFile("3")})
	errs, _ := checkExclusionEvidence(deleted, nil, base)
	if joined := strings.Join(errs, "\n"); !strings.Contains(joined, "rose from 1 at "+holder) {
		t.Errorf("a line restored above the deleted file's count was not refused: %q", errs)
	}

	never := commits(t, "", "")
	if base := priorEvidence(never, map[string]string{}, map[string]bool{}, true); base.state != priorRead || base.backlog != nil || base.malformed != "" {
		t.Errorf("a history that never held the file answered %v, counts %v, malformed %q; want read with nothing to compare",
			base.state, base.backlog, base.malformed)
	}

	malformed := commits(t, "not-applicable many\n", "")
	base = priorEvidence(malformed, map[string]string{}, map[string]bool{}, true)
	for _, want := range []string{"is malformed", "Commit a valid file", "not a whole number"} {
		if !strings.Contains(base.malformed, want) {
			t.Errorf("a malformed committed file answered %q, want it to say %q", base.malformed, want)
		}
	}
	writeFixtureFiles(t, malformed, map[string]string{evidenceBacklogRel: backlogFile("0")})
	if errs, _ := checkExclusionEvidence(malformed, nil, base); len(errs) != 1 || errs[0] != base.malformed {
		t.Errorf("a malformed committed file was refused as %q, want the malformed refusal alone", errs)
	}

	// A malformed HEAD^ copy over a valid ancestor: the counts compare from
	// that ancestor, so a commit that breaks the file cannot let the next
	// raise it, and the rows are still judged.
	fallback := commits(t, backlogFile("1"), "not-applicable many\n", "")
	valid, failure := gitAnswer(fallback, "rev-parse", "HEAD~2")
	if failure != "" {
		t.Fatalf("name the commit holding the valid file: %s", failure)
	}
	base = priorEvidence(fallback, map[string]string{}, map[string]bool{}, true)
	if base.backlog[AnnotationNotApplicable] != 1 || !strings.Contains(base.backlogAt, valid) || base.malformed == "" {
		t.Errorf("a malformed HEAD^ over a valid ancestor answered counts %v at %q, malformed %q; want 1 at %s and the malformed refusal",
			base.backlog, base.backlogAt, base.malformed, valid)
	}
	writeFixtureFiles(t, fallback, map[string]string{evidenceBacklogRel: backlogFile("3")})
	added := Requirement{RFC: selftestStem, RID: "RFC9999-2-1", Level: levelMust, Section: "2",
		Source: selftestSummaryRel, Line: 7, Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "ze sends no widget"}}
	errs, _ = checkExclusionEvidence(fallback, []Requirement{added}, base)
	joined := strings.Join(errs, "\n")
	for _, want := range []string{"is malformed", "rose from 1 at " + valid, "RFC9999-2-1 is annotated {not-applicable}"} {
		if !strings.Contains(joined, want) {
			t.Errorf("a malformed HEAD^ over a valid ancestor was refused as %q, want it to say %q", errs, want)
		}
	}

	if base := priorEvidence(never, nil, nil, false); base.state != priorUnreadable {
		t.Errorf("summaries HEAD^ could not read answered %v, want unreadable", base.state)
	}
}

// VALIDATES: a row of a summary HEAD^ could not parse is judged against the
// fingerprint the pushed branch records for it, and is new when none is
// recorded, so a summary nobody could read never shelters a changed row.
// METHOD: evidenceRowIsNew over baselines that mark the fixture stem
// unreadable, with and without pushed fingerprints.
func TestEvidenceRowOfAnUnreadableSummary(t *testing.T) {
	row := Requirement{RFC: selftestStem, RID: "RFC9999-2-1", Section: "2",
		Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "ze sends no widget"}}
	held := evidenceFingerprint(&row)
	unreadable := map[string]bool{selftestStem: true}
	cases := []struct {
		name   string
		pushed map[string]string
		isNew  bool
	}{
		{"no fingerprint recorded anywhere", nil, true},
		{"the pushed branch records it unchanged", map[string]string{row.RID: held}, false},
		{"the pushed branch records another reason", map[string]string{row.RID: "2\x00not-applicable\x00older"}, true},
	}
	for _, one := range cases {
		base := evidenceBaseline{state: priorRead, prints: map[string]string{row.RID: held}, unreadable: unreadable, pushedPrints: one.pushed, pushedRead: one.pushed != nil}
		if got := evidenceRowIsNew(&row, base); got != one.isNew {
			t.Errorf("%s: evidenceRowIsNew answered %v, want %v", one.name, got, one.isNew)
		}
	}
}

// VALIDATES: the role a {not-applicable} quote binds is found in the quote at
// word boundaries, and a role that holds the other as whole words is refused
// either way, so `binds s` and `binds speaker; ze is BGP speaker` no longer
// pass while `binds a server; ze is an observer` does.
// METHOD: parseAnnotation over payloads quoting one sentence.
func TestNotApplicableRolesAreWholeWordsAndDistinct(t *testing.T) {
	const quote = `"A BGP speaker MUST send the widget before any frame."`
	cases := []struct{ payload, refused string }{
		{quote + "; binds a speaker; ze is a receiver; it sends no widget", ""},
		{quote + "; binds a BGP speaker; ze is a receiver; it sends no widget", ""},
		{quote + "; binds s; ze is a receiver; it sends no widget", "is not in the quoted sentence"},
		{quote + "; binds speak; ze is a receiver; it sends no widget", "is not in the quoted sentence"},
		{quote + "; binds speaker; ze is BGP speaker; it sends no widget", "is the role ze fills or a part of it"},
		{quote + "; binds BGP speaker; ze is speaker; it sends no widget", "holds the role ze fills"},
		// "server" is inside "observer" as letters, not as a word.
		{`"A server MUST send the widget before any frame."; binds a server; ze is an observer; it sends no widget`, ""},
		{`"An observer MUST send the widget before any frame."; binds an observer; ze is a server; it sends no widget`, ""},
	}
	for _, one := range cases {
		_, err := parseAnnotation("not-applicable: "+one.payload, "fixture")
		if one.refused == "" {
			if err != nil {
				t.Errorf("{not-applicable: %s} was refused: %v", one.payload, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), one.refused) {
			t.Errorf("{not-applicable: %s} answered %v, want a refusal saying %q", one.payload, err, one.refused)
		}
	}
}

// VALIDATES: through the real entry point (check), a row the tip commit adds
// lacking its evidence, and a row whose reason the tip commit edits, are each
// refused by id, though the committed count would carry an untouched row.
// METHOD: two-commit fixture repositories over the check fixture corpus, the
// tip adding RFC9999-2-4 in one and rewording RFC9999-2-3's reason in the other.
func TestCheckRefusesANewAndAnEditedRowLackingEvidence(t *testing.T) {
	const sentence = "A speaker MUST send the widget."
	legacy := "- [ ] [RFC9999-2-3] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends no widget}"
	base := fixtureCorpus()
	base[selftestSummaryRel] = quoteFixtureSummary(legacy)
	base[evidenceBacklogRel] = backlogFile("1")
	cases := []struct{ name, summary, refused string }{
		{"a new row", quoteFixtureSummary(legacy, "- [ ] [RFC9999-2-4] [SHOULD] "+sentence+" (§2) {not-applicable: ze sends none}"),
			"RFC9999-2-4 is annotated {not-applicable} and lacks the evidence"},
		{"an edited row", quoteFixtureSummary("- [ ] [RFC9999-2-3] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends no widget at all}"),
			"RFC9999-2-3 is annotated {not-applicable} and lacks the evidence"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tip := commitFixtureTip(t, base, map[string]string{selftestSummaryRel: one.summary}, nil)
			gitFixture(t, tip, []string{"checkout", "-q", "--detach"})
			report, code := Check(tip, nil)
			if code != 1 || !strings.Contains(strings.Join(report.Violations, "\n"), one.refused) {
				t.Errorf("answered %d, want 1 refusing %q:\n%s", code, one.refused, report.Text())
			}
		})
	}
}

// VALIDATES: a row is counted against the backlog only when HEAD^ and the
// pushed origin/main both record it unchanged, so one red commit that
// backfills row A and adds a lacking row R, keeping the line level, cannot let
// the next commit carry R as backlog: R is refused by id. A clone with no
// pushed branch judges against HEAD^ alone, as documented, and counts R.
// METHOD: three commits through the real entry point (check): the pushed base
// with legacy row RFC9999-2-3, the commit that backfills it and adds
// RFC9999-2-4 lacking evidence, and an empty tip, checked out detached.
func TestCheckRefusesARowAddedSinceThePushedBranch(t *testing.T) {
	const sentence = "A speaker MUST send the widget."
	legacy := "- [ ] [RFC9999-2-3] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends no widget}"
	backfilled := "- [ ] [RFC9999-2-3] [SHOULD] " + sentence + ` (§2) {not-applicable: "` + sentence +
		`"; binds a speaker; ze is a receiver; ze sends no widget}`
	added := "- [ ] [RFC9999-2-4] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends none}"
	base := fixtureCorpus()
	base[selftestSummaryRel] = quoteFixtureSummary(legacy)
	base[evidenceBacklogRel] = backlogFile("1")
	tip := commitFixtureTip(t, base, map[string]string{selftestSummaryRel: quoteFixtureSummary(backfilled, added)}, nil)
	gitFixture(t, tip, []string{"update-ref", "refs/remotes/origin/main", "HEAD^"})
	commitFixtureEmptyTip(t, tip)
	gitFixture(t, tip, []string{"checkout", "-q", "--detach"})
	const refused = "RFC9999-2-4 is annotated {not-applicable} and lacks the evidence"
	report, code := Check(tip, nil)
	if code != 1 || !strings.Contains(strings.Join(report.Violations, "\n"), refused) {
		t.Errorf("a row added since the pushed branch answered %d, want 1 refusing %q:\n%s", code, refused, report.Text())
	}
	gitFixture(t, tip, []string{"update-ref", "-d", "refs/remotes/origin/main"})
	if report, code := Check(tip, nil); code != 0 {
		t.Errorf("with no pushed branch, a row HEAD^ holds unchanged answered %d, want 0 (counted):\n%s", code, report.Text())
	}
}

// VALIDATES: pushedFingerprints reads origin/main for real: a row of a summary
// HEAD^ cannot parse is judged against the pushed branch, so a row it records
// unchanged is not new and a row whose reason changed is refused by id; with
// no pushed branch every row of that summary is new.
// METHOD: a git fixture whose first commit (set as origin/main) holds two
// legacy rows, whose second holds an unparseable summary (a duplicate id), and
// whose tip keeps RFC9999-2-3 and rewords RFC9999-2-4, judged through
// baselineLevels, priorEvidence and checkExclusionEvidence.
func TestPushedFingerprintsJudgeASummaryHEADParentCannotParse(t *testing.T) {
	const sentence = "A speaker MUST send the widget."
	kept := "- [ ] [RFC9999-2-3] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends no widget}"
	reworded := "- [ ] [RFC9999-2-4] [SHOULD] " + sentence + " (§2) {not-applicable: ze sends none}"
	tree := t.TempDir()
	gitFixture(t, tree, []string{"init", "-q"})
	writeFixtureFiles(t, tree, map[string]string{evidenceBacklogRel: backlogFile("2"),
		selftestSummaryRel: quoteFixtureSummary(kept, "- [ ] [RFC9999-2-4] [SHOULD] "+sentence+" (§2) {not-applicable: ze sends no widget}")})
	commitFixture(t, tree, "pushed")
	gitFixture(t, tree, []string{"update-ref", "refs/remotes/origin/main", "HEAD"})
	writeFixtureFiles(t, tree, map[string]string{selftestSummaryRel: quoteFixtureSummary(kept, kept)})
	commitFixture(t, tree, "unparseable")
	writeFixtureFiles(t, tree, map[string]string{evidenceBacklogRel: backlogFile("1"), selftestSummaryRel: quoteFixtureSummary(kept, reworded)})
	commitFixture(t, tree, "tip")
	rows, err := ParseSummaryText(quoteFixtureSummary(kept, reworded), selftestStem, selftestSummaryRel)
	if err != nil {
		t.Fatalf("parse the tip summary: %v", err)
	}
	var keptRow, rewordedRow *Requirement
	for i := range rows {
		switch rows[i].RID {
		case "RFC9999-2-3":
			keptRow = &rows[i]
		case "RFC9999-2-4":
			rewordedRow = &rows[i]
		}
	}
	if keptRow == nil || rewordedRow == nil {
		t.Fatalf("the tip summary holds %v, want RFC9999-2-3 and RFC9999-2-4", rows)
	}
	judge := func(t *testing.T) evidenceBaseline {
		t.Helper()
		_, prints, unreadable, known := baselineLevels(tree)
		if !unreadable[selftestStem] {
			t.Fatalf("HEAD^'s duplicate-id summary parsed (unreadable %v), so the fixture proves nothing", unreadable)
		}
		return priorEvidence(tree, prints, unreadable, known)
	}

	base := judge(t)
	if evidenceRowIsNew(keptRow, base) {
		t.Errorf("a row origin/main records unchanged was judged new")
	}
	if !evidenceRowIsNew(rewordedRow, base) {
		t.Errorf("a row whose reason changed since origin/main was judged untouched")
	}
	errs, _ := checkExclusionEvidence(tree, rows, base)
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "RFC9999-2-4 is annotated {not-applicable}") || strings.Contains(joined, "RFC9999-2-3 is annotated") {
		t.Errorf("refusals %q, want RFC9999-2-4 refused by id and RFC9999-2-3 counted", errs)
	}

	gitFixture(t, tree, []string{"update-ref", "-d", "refs/remotes/origin/main"})
	base = judge(t)
	if !evidenceRowIsNew(keptRow, base) || !evidenceRowIsNew(rewordedRow, base) {
		t.Errorf("with no pushed branch, a row of a summary HEAD^ cannot parse was judged untouched")
	}
}
