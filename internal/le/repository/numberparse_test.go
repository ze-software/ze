package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// numberParseTree builds a fixture checkout carrying one product file and an
// allowlist, and answers its root.
func numberParseTree(t *testing.T, source, allowlist string) string {
	t.Helper()
	root := tree(t, map[string]string{
		"internal/component/thing/thing.go": source,
		NumberParseAllowlistPath:            allowlist,
	})
	return root
}

// TestNumberParseGateNamesAnUnlistedFile proves the gate reports a 32-bit
// text-to-integer parse in a file no allowlist line justifies.
//
// VALIDATES: checkNumberParseSites answers an ISSUE for an unlisted file.
// PREVENTS: a seventh private AS-number parser landing unnoticed, which is what
// the six copies before it did.
func TestNumberParseGateNamesAnUnlistedFile(t *testing.T) {
	root := numberParseTree(t,
		"package thing\n\nfunc read(s string) {\n\t_, _ = strconv.ParseUint(s[2:], 10, 32)\n}\n",
		"# nothing allowed\n")

	findings, err := checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1: %v", len(findings), findings)
	}
	if findings[0].File != "internal/component/thing/thing.go" {
		t.Errorf("finding names %q, want the file that parses", findings[0].File)
	}
	if !strings.Contains(findings[0].Message, "asn.Parse") {
		t.Errorf("message = %q, want it to name the one reader", findings[0].Message)
	}
}

// TestNumberParseGateAcceptsAJustifiedFile proves an allowlist line is what
// makes a parse legal, and that the count is EXACT rather than a ceiling.
//
// VALIDATES: the count in the allowlist must equal the file's.
// PREVENTS: one justified parse licensing every later one in the same file,
// which is the hole a file-only allowlist would leave. A ceiling leaves a
// smaller version of the same hole: delete a justified parse and its slot stays
// open for an unjustified one.
func TestNumberParseGateAcceptsAJustifiedFile(t *testing.T) {
	const oneParse = "package thing\n\nfunc read(s string) {\n\t_, _ = strconv.ParseUint(s, 10, 32)\n}\n"
	root := numberParseTree(t, oneParse, "1 internal/component/thing/thing.go\n")
	fixture := filepath.Join(root, "internal", "component", "thing", "thing.go")
	findings, err := checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none for a justified parse", findings)
	}

	// A second parse in the same file differs from the line, and is reported.
	twoParses := oneParse + "\nfunc readAgain(s string) {\n\t_, _ = strconv.ParseUint(s, 10, 32)\n}\n"
	if err := os.WriteFile(fixture, []byte(twoParses), 0o600); err != nil {
		t.Fatalf("rewriting the fixture: %v", err)
	}
	findings, err = checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 for a file over its count", len(findings))
	}

	// A file UNDER its line is reported too, which is what makes the count
	// exact. The freed slot is the thing being refused.
	if err := os.WriteFile(fixture,
		[]byte("package thing\n"), 0o600); err != nil {
		t.Fatalf("rewriting the fixture: %v", err)
	}
	findings, err = checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 for a line whose file parses none", len(findings))
	}
	if !strings.Contains(findings[0].Message, "stale line") {
		t.Errorf("message = %q, want it to name the stale line", findings[0].Message)
	}
}

// TestNumberParseGateReadsOnlyTheThirtyTwoBitWidth proves the gate is about the
// width an AS number has. A port and a timer therefore stay out of the
// allowlist, because neither can hold one.
//
// VALIDATES: NumberParsePattern matches bitSize 32 and nothing else.
// PREVENTS: a gate so noisy that the next session regenerates the baseline
// unread.
func TestNumberParseGateReadsOnlyTheThirtyTwoBitWidth(t *testing.T) {
	root := numberParseTree(t,
		"package thing\n\nfunc read(s string) {\n"+
			"\t_, _ = strconv.ParseUint(s, 10, 16)\n"+
			"\t_, _ = strconv.ParseUint(s, 10, 64)\n"+
			"\t_, _ = strconv.Atoi(s)\n}\n",
		"# nothing allowed\n")

	findings, err := checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none: 16, 64 and Atoi are not AS-number widths", findings)
	}
}

// TestNumberParseGateSkipsTests proves a test file is not judged, because a
// test parses fixtures rather than what an operator typed.
func TestNumberParseGateSkipsTests(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/component/thing/thing_test.go": "package thing\n\nfunc read(s string) {\n\t_, _ = strconv.ParseUint(s, 10, 32)\n}\n",
		NumberParseAllowlistPath:                 "# nothing allowed\n",
	})
	findings, err := checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none for a test file", findings)
	}
}

// TestNumberParseGateRefusesToPassWithNoAllowlist proves the gate fails closed:
// a tree that parses 32-bit integers and carries no allowlist is an ERROR, not
// a pass.
//
// VALIDATES: readNumberParseAllowlist's missing-file error reaches the caller.
// PREVENTS: deleting the allowlist to make the gate quiet.
func TestNumberParseGateRefusesToPassWithNoAllowlist(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/component/thing/thing.go": "package thing\n\nfunc read(s string) {\n\t_, _ = strconv.ParseUint(s, 10, 32)\n}\n",
	})
	if _, err := checkNumberParseSites(root); err == nil {
		t.Fatal("a missing allowlist passed; the gate must fail closed")
	}
}

// TestNumberParseAllowlistMatchesThisTree proves the checked-in allowlist still
// describes the tree it was taken from, in both directions. Every file that
// parses is on the list with its exact count, and every line has a file that
// parses.
//
// The second direction is the one this docstring claimed while the code did
// nothing about it. The gate walked `counts` alone. A line for a deleted file
// and a count above the real one were both silent, and either one
// pre-authorizes the next parse written there.
//
// VALIDATES: the baseline in numberparse-allowlist.txt, line by line.
// PREVENTS: a new parse hiding behind a line the tree stopped needing.
func TestNumberParseAllowlistMatchesThisTree(t *testing.T) {
	root := repositoryTreeRoot(t)
	findings, err := checkNumberParseSites(root)
	if err != nil {
		t.Fatalf("checkNumberParseSites over the real tree: %v", err)
	}
	for _, finding := range findings {
		t.Errorf("%s: %s", finding.File, finding.Message)
	}
}

// repositoryTreeRoot answers this checkout's root, found by walking up from the
// test's working directory to the directory holding go.mod.
func repositoryTreeRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}
