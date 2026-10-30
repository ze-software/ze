package rfc

import (
	"os"
	"path/filepath"
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
// lowered in the file; a count that rose names the rows and is refused; a
// committed line that rose against HEAD^ is refused naming the kind and both
// counts; a kind at zero is refused row by row; git that cannot be read
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
		return evidenceBaseline{state: priorRead, prints: prints, backlog: backlogCounts(committed)}
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
			[]string{"2 {not-applicable} rows the commit left alone lack their evidence, and the committed count is 1", "set it back to `not-applicable 2`"}, ""},
		{"a committed line that rose against HEAD^", backlogFile("2"), []Requirement{legacy, second}, read(prints, 1),
			[]string{"the {not-applicable} count rose from 1 at HEAD^ to 2", "`not-applicable 1`"}, ""},
		{"a committed line that fell against HEAD^", backlogFile("1"), []Requirement{legacy}, read(prints, 2), nil, ""},
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
// the summary line, the id and the format; a rise names the rows whose
// evidence form no longer holds.
// METHOD: an untouched row whose quoted evidence is not in its cited section,
// judged once under a committed count of 0 (refused by id) and then twice over
// under a committed count of 1, so the count rose above the line.
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
	rose, _ := checkExclusionEvidence(tree, []Requirement{broken, broken},
		evidenceBaseline{state: priorRead, prints: map[string]string{broken.RID: evidenceFingerprint(&broken)}, backlog: backlogCounts(1)})
	joined := strings.Join(rose, "\n")
	for _, want := range []string{"no longer holds", selftestSummaryRel + ":7 RFC9999-2-1", "restore"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the rise refusal %q does not say %q", rose, want)
		}
	}
}

// VALIDATES: a {lower-layer} layer names a component a reader can check: a
// vendored module this checkout carries, a kernel CONFIG_ option Ze's kernel
// builds (=y or =m), or a package of the Go standard library the toolchain
// holds under $(go env GOROOT)/src.
// METHOD: the parser reads the path out of the layer, and the evidence reader
// judges each form against a fixture checkout holding one vendored module and
// the two kernel config fragments; a checkout holding no kernel config refuses
// every linux: path.
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
		kernelConfigRels[0]: "CONFIG_IPV6=y\n# CONFIG_WIDGET is not set\nCONFIG_LOCALVERSION=\"-ze\"\n",
		kernelConfigRels[1]: "# runtime\nCONFIG_XFRM_USER=m\nCONFIG_NF_TABLES=n\n",
	})
	cases := []struct{ path, refused string }{
		{"linux:CONFIG_IPV6", ""},
		{"linux:CONFIG_XFRM_USER", ""},
		{"go:crypto/x509", ""},
		{"vendor/github.com/insomniacslk/dhcp/dhcpv6", ""},
		{"", "names no component a reader can check"},
		{"vendor/github.com/gone/module", "cannot be opened in this checkout"},
		{"linux:CONFIG_WIDGET", "is not enabled"},
		{"linux:CONFIG_NF_TABLES", "is not enabled"},
		{"linux:CONFIG_LOCALVERSION", "is not enabled"},
		{"linux:net/ipv6/ndisc.c", "names no CONFIG_ option"},
		{"linux:CONFIG_X/../y", "climbs out of its source tree"},
		{"go:crypto/nosuchpackage", "is not a package of the Go standard library"},
		{"go:github.com/x/y", "is not a package of the Go standard library"},
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
// has one (read), and a repository git cannot read is unreadable, never absent.
// METHOD: fixture directories driven through priorRevisionState.
func TestPriorRevisionState(t *testing.T) {
	if state, _ := priorRevisionState(t.TempDir()); state != priorAbsent {
		t.Errorf("a directory with no git answered %v, want absent", state)
	}
	root := t.TempDir()
	gitFixture(t, root, []string{"init", "-q"})
	writeFixtureFiles(t, root, map[string]string{"a.txt": "a\n"})
	commitFixture(t, root, "one")
	if state, _ := priorRevisionState(root); state != priorAbsent {
		t.Errorf("a root commit answered %v, want absent", state)
	}
	commitFixtureEmptyTip(t, root)
	if state, reason := priorRevisionState(root); state != priorRead {
		t.Errorf("two commits answered %v (%s), want read", state, reason)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("garbage\n"), 0o600); err != nil {
		t.Fatalf("corrupt the fixture HEAD: %v", err)
	}
	if state, _ := priorRevisionState(root); state != priorUnreadable {
		t.Errorf("a corrupt repository answered %v, want unreadable", state)
	}
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
