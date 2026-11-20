package completion

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

func TestWordsShowProducesTabSeparatedOutput(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for 'show' tree")
	}

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Errorf("expected tab-separated pair, got: %q", line)
		}
		if parts[0] == "" {
			t.Errorf("empty word in line: %q", line)
		}
		if parts[1] == "" {
			t.Errorf("empty description in line: %q", line)
		}
	}
}

func TestWordsShowProducesOutput(t *testing.T) {
	var buf bytes.Buffer

	writeWords(&buf, []string{"show"})

	if strings.TrimSpace(buf.String()) == "" {
		t.Fatal("expected non-empty output for show context")
	}
}

func TestWordsDeepPath(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show", "bgp", "peer"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for 'show bgp peer' subcommands")
	}

	if !strings.Contains(output, "list\t") {
		t.Errorf("expected 'list' in show bgp peer subcommands, got: %s", output)
	}
}

func TestWordsInvalidPath(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show", "nonexistent"})
	if code != 0 {
		t.Fatalf("expected exit 0 for invalid path, got %d", code)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output for invalid path, got: %q", buf.String())
	}
}

func TestWordsEmptyArgs(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, nil)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestWordsUnknownContext(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"unknown"})
	if code != 0 {
		t.Fatalf("expected exit 0 for unknown context, got %d", code)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output for unknown context, got: %q", buf.String())
	}
}

// VALIDATES: AC-2 — ValueHints (families) appear in words output for show bgp rib node.
// PREVENTS: ValueHints not flowing through the TreeCompleter delegation.
func TestWordsShowBGPRibIncludesFamilyHints(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show", "bgp", "rib"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()

	// Static children should be present.
	if !strings.Contains(output, "best\t") {
		t.Error("show bgp rib output missing static child 'best'")
	}
	// ValueHints families should be present.
	if !strings.Contains(output, "ipv4/mpls-vpn\t") {
		t.Error("show bgp rib output missing family ValueHint 'ipv4/mpls-vpn'")
	}
	if !strings.Contains(output, "l2vpn/evpn\t") {
		t.Error("show bgp rib output missing family ValueHint 'l2vpn/evpn'")
	}
}

// VALIDATES: pipe operators are filtered from words output.
// PREVENTS: shell completion showing pipe operators as command suggestions.
// VALIDATES: AC-2 — ze completion words env get returns public env keys.
// PREVENTS: env context missing from shell completion path.
func TestWordsEnvGetIncludesPublicKeys(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"env", "get"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for 'env get' completion")
	}

	if !strings.Contains(output, "ze.log\t") {
		t.Error("env get output should contain 'ze.log' as a completable key")
	}

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Errorf("expected tab-separated pair, got: %q", line)
		}
	}
}

// VALIDATES: AC-3 — ze completion words show env get returns same catalog.
// PREVENTS: show-tree env path diverging from env context.
func TestWordsShowEnvGetIncludesPublicKeys(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show", "env", "get"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for 'show env get' completion")
	}

	if !strings.Contains(output, "ze.log\t") {
		t.Error("show env get output should contain 'ze.log' as a completable key")
	}
}

// VALIDATES: AC-7 — env completion words never include Private entries.
// PREVENTS: private env vars leaking through shell completion.
func TestWordsEnvGetExcludesPrivate(t *testing.T) {
	var buf bytes.Buffer
	writeWords(&buf, []string{"env", "get"})
	output := buf.String()

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) > 0 && strings.Contains(parts[0], "<") {
			t.Errorf("env completion should not contain angle-bracket key: %q", parts[0])
		}
	}
}

func TestWordsPipeOperatorsFiltered(t *testing.T) {
	var buf bytes.Buffer
	// "show" with no further path lists top-level commands.
	// TreeCompleter.Complete("") returns commands, not pipes.
	// But to be thorough, verify no pipe operators appear in any output.
	code := writeWords(&buf, []string{"show"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	for _, pipe := range []string{"match\t", "count\t", "table\t", "no-more\t"} {
		if strings.Contains(output, pipe) {
			t.Errorf("words output should not contain pipe operator %q", pipe)
		}
	}
}

// TestCompletionTreeCoversEveryCanonicalVerb asserts that `ze completion words
// <verb>` walks the tree rooted at that verb, for every verb command.Verbs
// holds, and consumes the verb from the path it returns. This is what the
// shell asks when an operator types `ze <verb> <TAB>`.
//
// The assertion is against the registry rather than a list of words. Until
// 2026-11-20 only `show` was answered here and every other verb completed
// solely behind the removed `ze run` wrapper, so `ze clear <TAB>` offered
// nothing.
func TestCompletionTreeCoversEveryCanonicalVerb(t *testing.T) {
	for verb := range command.Verbs {
		// A registered root of the same spelling owns the word, as in dispatch.
		if _, found, err := rootCommandTree(verb); err != nil {
			t.Fatalf("rootCommandTree(%q): %v", verb, err)
		} else if found {
			continue
		}
		tree, rest, err := completionTree([]string{verb, "alpha"})
		if err != nil {
			t.Fatalf("completionTree(%q): %v", verb, err)
		}
		if tree == nil {
			t.Errorf("completionTree(%q) returned no tree", verb)
			continue
		}
		if len(rest) != 1 || rest[0] != "alpha" {
			t.Errorf("completionTree(%q) rest = %v, want [alpha]: the verb roots the tree, so it leaves the path", verb, rest)
		}
	}
}

// VALIDATES: `ze clear <TAB>` offers nouns under clear through the real
// entry point, writeWords, with no `run` word in front.
// PREVENTS: a verb other than show completing nothing in the shell.
func TestWordsCompletesAVerbOtherThanShow(t *testing.T) {
	var buf bytes.Buffer
	if code := writeWords(&buf, []string{command.VerbClear, ""}); code != 0 {
		t.Fatalf("writeWords(clear): exit %d", code)
	}
	// The nouns under clear depend on which plugins this binary registers, so
	// the assertion is that the verb's tree answers, not which noun it holds.
	if !strings.Contains(buf.String(), "\t") {
		t.Errorf("`ze completion words clear` offers nothing:\n%s", buf.String())
	}
}

// VALIDATES: `run` is no completion subject. `ze run` was replaced by direct
// verb dispatch, so the shell has nothing to offer after it.
// PREVENTS: completion leading an operator into a command that only prints a
// deprecation error. A word that is neither a root nor a verb answers nothing.
func TestCompletionTreeAnswersNothingForRun(t *testing.T) {
	for _, word := range []string{"run", "not-a-verb"} {
		tree, _, err := completionTree([]string{word, command.VerbShow})
		if err != nil {
			t.Fatalf("completionTree(%q): %v", word, err)
		}
		if tree != nil {
			t.Errorf("completionTree(%q) returned a tree: the word names no command", word)
		}
	}
}
