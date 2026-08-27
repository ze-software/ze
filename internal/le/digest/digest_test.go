package digest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// tree builds a fixture checkout from a path -> content map and answers its
// root. Every path is written relative to the root, so a case reads as the tree
// it describes.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", rel, err)
		}
	}
	return root
}

// problems answers the failure details of a fixture check. Thus, a case asserts
// what the gate SAYS, not only how many things it said.
func problems(t *testing.T, root string) []string {
	t.Helper()
	report, err := Check(root)
	if err != nil {
		t.Fatalf("checking the fixture: %v", err)
	}
	details := make([]string, 0, len(report.Errors))
	for _, problem := range report.Errors {
		details = append(details, problem.Detail)
	}
	return details
}

// VALIDATES: an anchor with a line number resolves through the digest's own
// declared base, and the resolution is recorded.
// PREVENTS: a base header that stops being read, which would make every
// subsystem-relative anchor unresolvable at once.
func TestAnAnchorResolvesThroughTheDeclaredBase(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/bgp.md":                 "<!-- digest-base: internal/component/bgp -->\n`peer.go:2`\n",
		"internal/component/bgp/peer.go":    "package bgp\nfunc Run() {}\n",
		"internal/component/other/peer2.go": "package other\n",
	})
	report, err := Check(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("a resolvable anchor was reported: %+v", report.Errors)
	}
	if len(report.Resolved) != 1 || report.Resolved[0].File != "internal/component/bgp/peer.go" {
		t.Fatalf("resolutions %+v, want the one file under the base", report.Resolved)
	}
	if report.Anchors != 1 || report.Digests != 1 {
		t.Fatalf("counted %d anchors across %d digests, want 1 and 1", report.Anchors, report.Digests)
	}
}

// VALIDATES: a bare name matching two files under two bases is refused rather
// than resolved against whichever base is listed first.
// PREVENTS: an anchor checked against the wrong same-named file, which would
// validate a line number that belongs to another package.
func TestAnAmbiguousAnchorFailsClosed(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/two.md":  "<!-- digest-base: internal/a, internal/b -->\n`peer.go:1`\n",
		"internal/a/peer.go": "package a\n",
		"internal/b/peer.go": "package b\n",
	})
	got := problems(t, root)
	want := "ambiguous -- matches internal/a/peer.go, internal/b/peer.go; qualify the path"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("problems %q, want exactly %q", got, want)
	}
}

// VALIDATES: a bare mention with no line number is informal shorthand and is
// never a finding, while a repo-relative link with no line must exist.
// PREVENTS: the two no-line branches collapsing into one, which would either
// flag every prose mention of a file name or stop checking doc links.
func TestTheTwoNoLineBranchesAnswerDifferently(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/bare.md": "<!-- digest-base: internal/a -->\n`nowhere.go` and `internal/a/gone.go`\n",
		"internal/a/here.go": "package a\n",
	})
	got := problems(t, root)
	want := "linked file does not exist"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("problems %q, want exactly %q for the repo-relative link alone", got, want)
	}
}

// VALIDATES: a reversed range is its own finding, separate from a line past the
// file end.
// PREVENTS: the reversed case reaching the range check, where `20-12` in a
// 30-line file reads as valid.
func TestAReversedRangeIsItsOwnFinding(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/r.md": "<!-- digest-base: internal/a -->\n`x.go:3-2`\n",
		"internal/a/x.go": "1\n2\n3\n4\n",
	})
	got := problems(t, root)
	if len(got) != 1 || got[0] != "reversed line range 3-2" {
		t.Fatalf("problems %q, want the reversed-range finding", got)
	}
}

// VALIDATES: a line past the end of the file names the file's real line count.
// PREVENTS: an off-by-one in the count, which would either accept an anchor one
// line past the end or reject the last line of every file.
func TestTheLastLineIsInRangeAndTheNextOneIsNot(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/r.md": "<!-- digest-base: internal/a -->\n`x.go:4` `x.go:5`\n",
		// Four lines, and the last one carries no trailing newline: a file
		// counted by newlines alone would answer 3 and reject `x.go:4`.
		"internal/a/x.go": "1\n2\n3\n4",
	})
	got := problems(t, root)
	want := "line 5 out of range (`internal/a/x.go` has 4 lines)"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("problems %q, want exactly %q", got, want)
	}
}

// VALIDATES: the anchor NAME is built on truthiness while the range check reads
// the value, so `x.go:0` is named bare and still fails the check.
// PREVENTS: a tidy-up that makes the two agree, which changes the message the
// script prints for a zero and breaks the parity proof.
func TestAZeroLineIsNamedBareAndStillChecked(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/z.md": "<!-- digest-base: internal/a -->\n`x.go:0`\n",
		"internal/a/x.go": "1\n2\n",
	})
	report, err := Check(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(report.Errors) != 1 {
		t.Fatalf("errors %+v, want one", report.Errors)
	}
	if report.Errors[0].Anchor != "x.go" {
		t.Fatalf("anchor named %q, want the bare path the script names", report.Errors[0].Anchor)
	}
	if report.Errors[0].Detail != "line 0 out of range (`internal/a/x.go` has 2 lines)" {
		t.Fatalf("detail %q, want the out-of-range finding", report.Errors[0].Detail)
	}
}

// VALIDATES: a digest carrying a subsystem-relative line anchor and no base
// header is refused, and the refusal stops that digest.
// PREVENTS: a headerless digest resolving its anchors against the tree root,
// where a bare name matches everything and nothing is checked.
func TestAMissingBaseHeaderStopsTheDigest(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/n.md": "`x.go:1` `y.go:900`\n",
		"internal/a/x.go": "1\n",
	})
	got := problems(t, root)
	if len(got) != 1 || !strings.HasPrefix(got[0], "no `<!-- digest-base:") {
		t.Fatalf("problems %q, want the single header finding", got)
	}
}

// VALIDATES: a declared base that is not a directory is reported, and the
// message carries the Python list rendering the script interpolated.
// PREVENTS: the brackets and quotes being read as formatting and dropped, which
// would change a message two halves must agree on byte for byte.
func TestADeclaredBaseThatDoesNotExistIsReportedWithItsPythonRendering(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/b.md": "<!-- digest-base: internal/gone -->\n`x.go:1`\n",
	})
	got := problems(t, root)
	want := []string{
		"declared base subtree `internal/gone` does not exist",
		"file not found under ['internal/gone']",
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("problems %q, want %q", got, want)
	}
}

// VALIDATES: a tree holding no digest is REFUSED rather than answered green.
// PREVENTS: the script's fail-open, where a renamed ai/digests reads exactly
// like a tree whose every anchor resolves.
func TestATreeWithNoDigestIsRefused(t *testing.T) {
	root := tree(t, map[string]string{"go.mod": "module example.com/m\n"})
	if _, err := Check(root); err == nil {
		t.Fatal("a tree with no digest was accepted, so the gate is green over a population it never found")
	}

	// A directory holding only the README is the same fact: the README is not
	// a digest, so nothing was checked.
	root = tree(t, map[string]string{"ai/digests/README.md": "how to write one\n"})
	if _, err := Check(root); err == nil {
		t.Fatal("a digest directory holding only its README was accepted")
	}
}

// VALIDATES: the base index skips the directories the script skipped, so a
// vendored copy of a file cannot make an anchor ambiguous.
// PREVENTS: a vendor/ or testdata/ tree entering the index, where every
// duplicated file name would fail closed and the gate would be red
// for a reason no digest author can fix.
func TestTheIndexSkipsVendorAndDotDirectories(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/v.md":            "<!-- digest-base: internal/a -->\n`x.go:1`\n",
		"internal/a/x.go":            "1\n",
		"internal/a/vendor/dep/x.go": "1\n",
		"internal/a/testdata/x.go":   "1\n",
		"internal/a/.hidden/x.go":    "1\n",
	})
	if got := problems(t, root); len(got) != 0 {
		t.Fatalf("problems %q, want none: the duplicates all sit under skipped directories", got)
	}
}

// VALIDATES: an anchor CARRYING A SLASH is matched by suffix rather than by
// base name, so a partial path narrows what it can mean.
// PREVENTS: `b/x.go` matching every x.go under the base, which turns a
// deliberately qualified anchor into an ambiguous one and reports the author
// for qualifying it.
func TestAPartialPathIsMatchedBySuffixAndNotByBaseName(t *testing.T) {
	// The base-plus-path join is `internal/b/x.go`, which does not exist, so
	// the suffix branch is what has to resolve this. A base-name match would
	// take internal/other/x.go too and report the anchor as ambiguous.
	root := tree(t, map[string]string{
		"ai/digests/s.md":      "<!-- digest-base: internal -->\n`b/x.go:1`\n",
		"internal/deep/b/x.go": "1\n",
		"internal/other/x.go":  "1\n",
	})
	if got := problems(t, root); len(got) != 0 {
		t.Fatalf("problems %q, want none: the slash narrows the anchor to internal/deep/b/x.go", got)
	}
}

// VALIDATES: a file under a skipped directory is not in the index even when the
// anchor has NO direct hit to short-circuit on.
// PREVENTS: the skip list only appearing to work because an exact base-plus-path
// hit answers first, which is what every anchor written against its own base has.
func TestASkippedDirectoryIsAbsentEvenWithNoDirectHit(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/s.md":          "<!-- digest-base: internal -->\n`x.go:1`\n",
		"internal/a/x.go":          "1\n",
		"internal/a/vendor/x.go":   "1\n",
		"internal/a/testdata/x.go": "1\n",
	})
	if got := problems(t, root); len(got) != 0 {
		t.Fatalf("problems %q, want none: only internal/a/x.go is in the index", got)
	}
}

// VALIDATES: a repo-relative anchor resolves against the tree root and ignores
// the declared bases entirely.
// PREVENTS: a full path being searched under a base, where
// internal/a/internal/a/x.go is what would be looked for.
func TestARepoRelativeAnchorResolvesAgainstTheRoot(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/f.md": "<!-- digest-base: internal/b -->\n`internal/a/x.go:2`\n",
		"internal/a/x.go": "1\n2\n",
		"internal/b/y.go": "1\n",
	})
	if got := problems(t, root); len(got) != 0 {
		t.Fatalf("problems %q, want none", got)
	}
}

// VALIDATES: the answer is structured data with the script's own keys.
// PREVENTS: a payload whose keys drifted, which would break `| json` for every
// caller reading the script's document today.
func TestTheReportIsStructuredDataWithTheScriptsKeys(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/digests/k.md": "<!-- digest-base: internal/a -->\n`x.go:9`\n",
		"internal/a/x.go": "1\n",
	})
	report, err := Check(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encoding the report: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decoding the report: %v", err)
	}
	for _, key := range []string{"digests", "anchors", "errors", "resolved"} {
		if _, ok := document[key]; !ok {
			t.Errorf("the payload has no %q key: %s", key, raw)
		}
	}
	rows, _ := document["errors"].([]any)
	if len(rows) != 1 {
		t.Fatalf("errors %v, want one row", rows)
	}
	row, _ := rows[0].(map[string]any)
	for _, key := range []string{"digest", "anchor", "problem"} {
		if _, ok := row[key]; !ok {
			t.Errorf("an error row has no %q key: %s", key, raw)
		}
	}
}

// VALIDATES: the failure page goes to the diagnosis and the verdict line goes
// to the text, which is the stream split the script keeps.
// PREVENTS: both halves landing on one stream, where the migration could not
// compare stdout and stderr apart and a merged capture's interleaving
// would decide the verdict.
func TestTheVerdictAndTheFailurePageAreDifferentStreams(t *testing.T) {
	clean := Report{Digests: 2, Anchors: 3, Resolved: make([]Resolution, 3)}
	if got := clean.Text(); got != "checked 3 anchors across 2 digests, all resolve\n" {
		t.Errorf("clean verdict %q", got)
	}
	if got := clean.Diagnosis(); got != "" {
		t.Errorf("a clean run wrote a diagnosis: %q", got)
	}

	red := Report{
		Digests: 2, Anchors: 2,
		Errors: []Problem{
			{Digest: "ai/digests/a.md", Anchor: "x.go:9", Detail: "line 9 out of range"},
			{Digest: "ai/digests/a.md", Anchor: "y.go:1", Detail: "linked file does not exist"},
		},
	}
	if got := red.Text(); got != "" {
		t.Errorf("a failing run wrote to the verdict stream: %q", got)
	}
	page := red.Diagnosis()
	if !strings.HasPrefix(page, "digest anchor check FAILED: 2 bad anchor(s) in 1 digest(s)\n") {
		t.Errorf("diagnosis heading wrong: %q", page)
	}
	if !strings.HasSuffix(page, "ai/digests/README.md).\n") {
		t.Errorf("diagnosis remedy wrong: %q", page)
	}
}

// VALIDATES: no backtick token of the live corpus needs a Unicode word
// character to be classified, so the ASCII \w this package compiles
// answers what the script's Unicode \w answers.
// PREVENTS: the one divergence the two regular-expression engines have from
// going unmeasured, which is the whole reason the pattern strings are
// allowed to be identical.
func TestNoDigestTokenNeedsAUnicodeWordCharacter(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolving the checkout: %v", err)
	}
	names, err := DigestFiles(root)
	if err != nil {
		t.Fatalf("listing the digests: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("the checkout holds no digest, so this measurement proves nothing")
	}

	// The Unicode reading of the same pattern. A token that this accepts and
	// the package's ASCII pattern refuses is a token the two halves would
	// classify differently.
	unicodePattern := strings.Replace(AnchorPattern, `[\w./-]`, `[\p{L}\p{N}_./-]`, 1)
	unicodeRe := regexp.MustCompile(unicodePattern)

	for _, digestName := range names {
		raw, err := os.ReadFile(filepath.Join(root, "ai", "digests", digestName))
		if err != nil {
			t.Fatalf("reading %s: %v", digestName, err)
		}
		for _, match := range backtickRe.FindAllStringSubmatch(string(raw), -1) {
			token := strings.TrimSpace(match[1])
			if unicodeRe.MatchString(token) != anchorRe.MatchString(token) {
				t.Errorf("%s: token %q is classified differently by the Unicode and ASCII readings", digestName, token)
			}
		}
	}
}
