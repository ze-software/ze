// VALIDATES: every word the ./le launcher script consumes before le starts is
// one le declares, and each is on the root page, in `le help ai | json` and
// among the completion candidates at its place (AC-51, D-P4-launcher of
// spec-le-cli-usability). The words are read from the script itself.
// PREVENTS: a word added to the script that no help page, reference or
// completion shows, and a declared launcher word the script no longer reads.
package leroot_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

var (
	// scriptCase opens the script's top-level test of its first argument.
	scriptCase = regexp.MustCompile(`^case \$\{1:-\} in$`)
	// scriptArm is one arm of that case: `--name)`, `--name=*)`.
	scriptArm = regexp.MustCompile(`^(\S+)\)$`)
	// scriptIf is a top-level test of the first argument: `if [ "${1:-}" = rollback ]`.
	scriptIf = regexp.MustCompile(`^if \[ "\$\{1:-\}" = "?([^" \]]+)"? \]`)
	// scriptValue is a test of the --name value: `[ "$name" = session ]`.
	scriptValue = regexp.MustCompile(`\[ "\$name" = "?([^" \]]+)"? \]`)
)

// branchKind says what one branch of the script does with the word it tested.
type branchKind uint8

const (
	branchConsumes branchKind = iota + 1 // the script acts on the word itself
	branchForwards                       // the binary receives the word unchanged
	branchRefuses                        // the script only refuses the word: a retired word, not one le accepts
)

// classify reads one branch body. A body that execs the binary with "$@" and
// shifts nothing hands the word on; a body of nothing but echo lines and
// `exit 2` refuses it; anything else consumes it.
func classify(body []string) branchKind {
	joined := strings.Join(body, "\n")
	if strings.Contains(joined, `exec "$binary" "$@"`) && !strings.Contains(joined, "shift") {
		return branchForwards
	}
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "echo ") && trimmed != "exit 2" {
			return branchConsumes
		}
	}
	return branchRefuses
}

// scriptWords answers the words the script consumes: its top-level case arms
// and `if` tests of the first argument, and its tests of the --name value. The
// whole script is read, not only the part before the first exec, because the
// --name branch execs early and `rollback` is tested after it.
func scriptWords(t *testing.T, script string) []string {
	t.Helper()
	lines := strings.Split(script, "\n")
	var words []string
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if match := scriptIf.FindStringSubmatch(line); match != nil {
			end := index + 1
			for end < len(lines) && lines[end] != "fi" {
				end++
			}
			if classify(lines[index+1:end]) == branchConsumes {
				words = append(words, match[1])
			}
			index = end
			continue
		}
		if !scriptCase.MatchString(line) {
			continue
		}
		for index++; index < len(lines) && lines[index] != "esac"; index++ {
			arm := scriptArm.FindStringSubmatch(lines[index])
			if arm == nil {
				continue
			}
			end := index + 1
			for end < len(lines) && strings.TrimSpace(lines[end]) != ";;" {
				end++
			}
			word := strings.TrimSuffix(arm[1], "*")
			if word != "" && classify(lines[index+1:end]) == branchConsumes {
				words = append(words, word)
			}
			index = end
		}
	}
	for _, match := range scriptValue.FindAllStringSubmatch(script, -1) {
		words = append(words, match[1])
	}
	slices.Sort(words)
	return slices.Compact(words)
}

func TestEveryWordTheLauncherConsumesIsInHelp(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	script, err := os.ReadFile(filepath.Join(root, "le"))
	if err != nil {
		t.Fatalf("read the le script: %v", err)
	}
	consumed := scriptWords(t, string(script))
	if len(consumed) == 0 {
		t.Fatal("read no word from the le script, so this test would prove nothing: its parser no longer matches the script")
	}

	declared := map[string]bool{}
	for _, word := range leroot.LauncherWords() {
		declared[word.Word] = true
		for _, value := range word.Values {
			declared[value.Value] = true
		}
	}
	for _, word := range consumed {
		if !declared[word] && leroot.LookupCommand(word) == nil {
			t.Errorf("the le script consumes %q, which neither leroot.LauncherWords nor an le area declares", word)
		}
	}
	for word := range declared {
		if !slices.Contains(consumed, word) {
			t.Errorf("leroot.LauncherWords declares %q, which the le script does not read", word)
		}
	}

	page, _ := streams(t, func() { leroot.Dispatch("le", []string{"help"}) })
	reference, _ := streams(t, func() { leroot.Dispatch("le", []string{"help", "ai", "all", "|", "json"}) })
	first, err := leroot.Complete([]string{""})
	if err != nil {
		t.Fatalf("complete the first word: %v", err)
	}
	afterName, err := leroot.Complete([]string{"--name", ""})
	if err != nil {
		t.Fatalf("complete the value after --name: %v", err)
	}
	glued, err := leroot.Complete([]string{"--name="})
	if err != nil {
		t.Fatalf("complete the value glued to --name=: %v", err)
	}
	offered := func(candidates []leaction.Candidate, value string) bool {
		return slices.ContainsFunc(candidates, func(candidate leaction.Candidate) bool { return candidate.Value == value })
	}
	for _, word := range consumed {
		if !strings.Contains(page, word) {
			t.Errorf("`le help` does not name %q", word)
		}
		if !strings.Contains(reference, `"`+word) && !strings.Contains(reference, " "+word) {
			t.Errorf("`le help ai all | json` does not name %q", word)
		}
		if !offered(first, word) && !offered(afterName, word) && !offered(glued, "--name="+word) {
			t.Errorf("completion offers %q at no place", word)
		}
	}
}
