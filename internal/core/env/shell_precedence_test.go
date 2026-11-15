// Design: docs/architecture/config/environment.md -- OS spellings and the shell spelling.
// Related: env.go -- ShellSafe and LookupIn; shell_test.go -- existing alias preservation tests.
//
// Goal: preserve the value a registered non-ze key has across a plugin shell.
// Method: compare the parent's lookup with the rewritten environment and with
// the environment a real /bin/sh child receives, for both input orders.

package env

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestShellSafePreservesNonZeDottedPrecedence holds the same precedence on
// both sides of the fork, including when an identifier spelling keeps its case.
//
// VALIDATES: a dotted registered key wins over lower-, upper-, and mixed-case
// identifier spellings in either input order, before and after a real shell;
// registered upper-case aliases and unrelated identifiers retain their spelling.
// PREVENTS: a forked plugin reading an underscore value while the daemon reads
// the dotted value because rewriting removed the dot that selected it.
func TestShellSafePreservesNonZeDottedPrecedence(t *testing.T) {
	const key = "exabgp.test.shell"
	const want = "from-dotted"
	for _, identifier := range []string{
		"exabgp_test_shell",
		"EXABGP_TEST_SHELL",
		"ExaBGP_Test_Shell",
	} {
		for _, order := range []string{"dotted-first", "dotted-last"} {
			t.Run(identifier+"/"+order, func(t *testing.T) {
				parent := []string{key + "=" + want, identifier + "=from-identifier"}
				if order == "dotted-last" {
					slices.Reverse(parent)
				}
				parent = append(parent,
					"PATH="+os.Getenv("PATH"),
					"TEST_SHELL_UPPER=alias-value",
					"UNRELATED_UPPER=unrelated-value",
				)
				if got := LookupIn(parent, key); got != want {
					t.Fatalf("parent reads %s=%q, want %q", key, got, want)
				}

				rewritten := ShellSafe(parent)
				if got := LookupIn(rewritten, key); got != want {
					t.Errorf("rewritten environment reads %s=%q, want %q", key, got, want)
				}
				ctx, cancel := context.WithTimeout(context.Background(), childEnvBudget)
				defer cancel()
				child := exec.CommandContext(ctx, "/bin/sh", "-c", "env")
				child.Env = rewritten
				out, err := child.Output()
				if err != nil {
					t.Fatalf("/bin/sh -c env: %v", err)
				}
				inherited := strings.Split(string(out), "\n")
				if got := LookupIn(inherited, key); got != want {
					t.Errorf("shell child reads %s=%q, want %q", key, got, want)
				}
				for _, entry := range []string{
					"TEST_SHELL_UPPER=alias-value",
					"UNRELATED_UPPER=unrelated-value",
				} {
					if !slices.Contains(inherited, entry) {
						t.Errorf("shell child lost unchanged entry %q", entry)
					}
				}
			})
		}
	}
}

// TestShellSafePreservesNonZeIdentifierPrecedence checks a registered key
// without a dotted spelling: the last identifier supplies the value and case.
//
// VALIDATES: alternate-case identifiers collapse to one entry without changing
// the last value the parent reads, and the surviving identifier keeps its case.
// PREVENTS: the shell's environment order deciding between duplicate spellings.
func TestShellSafePreservesNonZeIdentifierPrecedence(t *testing.T) {
	const key = "exabgp.test.shell"
	for _, last := range []string{"exabgp_test_shell", "EXABGP_TEST_SHELL", "ExaBGP_Test_Shell"} {
		t.Run(last, func(t *testing.T) {
			parent := []string{
				"exabgp_test_shell=first",
				"EXABGP_TEST_SHELL=second",
				last + "=last",
			}
			rewritten := ShellSafe(parent)
			if len(rewritten) != 1 {
				t.Fatalf("rewritten environment carries %d entries for one key, want 1", len(rewritten))
			}
			if rewritten[0] != last+"=last" {
				t.Errorf("surviving entry = %q, want %q", rewritten[0], last+"=last")
			}
			if got := LookupIn(rewritten, key); got != LookupIn(parent, key) {
				t.Errorf("child reads %s=%q, parent reads %q", key, got, LookupIn(parent, key))
			}
		})
	}
}
