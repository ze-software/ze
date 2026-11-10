// Design: docs/architecture/config/environment.md -- OS spellings, the shell spelling
// Overview: env.go -- normalize, ShellSafe
//
// Goal: prove every registered key has a spelling a shell passes on, and that
// a child started through /bin/sh -c reads each ze key with the value Get reads
// in the parent. Method: rewrite an environment with ShellSafe, start a real
// /bin/sh -c env with it, and read what the child was given.

package env

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// shellIdentifier is the POSIX name of a variable a shell keeps in the
// environment it hands a child: dash drops every other name.
var shellIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// childEnvBudget bounds the one shell the test starts, which runs env and exits.
const childEnvBudget = 10 * time.Second

// TestGetReadsHyphenAsSeparator holds the reading that gives a hyphenated key a
// shell spelling: a hyphen and an underscore are one separator.
//
// VALIDATES: ZE_TEST_HYPHEN_KEY, the only spelling of ze.test.hyphen-key an
// operator can export from a shell, is read by Get.
// PREVENTS: a hyphenated key that never reaches a process started by a shell,
// whose reader silently answers its default.
func TestGetReadsHyphenAsSeparator(t *testing.T) {
	const key = "ze.test.hyphen-key"
	t.Cleanup(ResetCache)
	for _, name := range []string{"ZE_TEST_HYPHEN_KEY", "ze_test_hyphen-key", key} {
		unsetAll(t, key)
		t.Setenv(name, "from-"+name)
		ResetCache()
		if got := Get(key); got != "from-"+name {
			t.Errorf("with %s set, Get(%q) = %q, want %q", name, key, got, "from-"+name)
		}
		os.Unsetenv(name) //nolint:errcheck // test setup, restored by t.Setenv
	}
}

// TestShellSafeSpellsEveryZeKeyAsAnIdentifier proves the child of a shell reads
// each ze key with the value the parent reads.
//
// VALIDATES: a dotted key, a hyphenated key and a registered exabgp key reach
// a /bin/sh -c child under one identifier spelling each; when the dotted and
// the underscore spelling of one key are both set, the child gets the dotted
// value in either order; an unrelated variable passes unchanged; every
// registered key, rewritten, is a shell identifier.
// PREVENTS: a forked plugin that runs on a key's default because dash dropped
// the only spelling the daemon carried.
func TestShellSafeSpellsEveryZeKeyAsAnIdentifier(t *testing.T) {
	for _, entry := range AllEntries() {
		if strings.Contains(entry.Key, "<") {
			continue
		}
		renamed := ShellSafe([]string{entry.Key + "=x"})
		name, _, _ := strings.Cut(renamed[0], "=")
		if !shellIdentifier.MatchString(name) {
			t.Errorf("registered key %q is passed to a shell as %q, which is not an identifier", entry.Key, name)
		}
	}

	for _, order := range [][]string{
		{"ZE_TEST_SET_VAL=underscore", "ze.test.set.val=dotted"},
		{"ze.test.set.val=dotted", "ZE_TEST_SET_VAL=underscore"},
	} {
		parent := append([]string{
			"PATH=" + os.Getenv("PATH"),
			"ze.test.env.check=dotted-check",
			"ze.test.hyphen-key=hyphen",
			"exabgp.test.shell=exabgp",
			"some.other.thing=untouched",
		}, order...)

		child := childEnvironment(t, ShellSafe(parent))

		want := map[string]string{
			"ZE_TEST_ENV_CHECK":  "dotted-check",
			"ZE_TEST_HYPHEN_KEY": "hyphen",
			"exabgp_test_shell":  "exabgp",
			"ZE_TEST_SET_VAL":    "dotted",
		}
		for name, value := range want {
			if got, ok := child[name]; !ok || got != value {
				t.Errorf("parent %v: the child reads %s=%q (present %v), want %q", order, name, got, ok, value)
			}
		}
		spelled := make(map[string]string, len(want))
		for name := range want {
			spelled[normalize(name)] = name
		}
		for name := range child {
			only, ours := spelled[normalize(name)]
			if ours && name != only {
				t.Errorf("parent %v: the child carries %q beside %q, two spellings of one key", order, name, only)
			}
		}
		if rewritten := ShellSafe(parent); !slices.Contains(rewritten, "some.other.thing=untouched") {
			t.Errorf("an unrelated variable was rewritten: %v", rewritten)
		}
	}
}

// childEnvironment starts /bin/sh -c env with environ, the way the daemon
// starts a plugin's run string, and answers the environment the child read.
func childEnvironment(t *testing.T, environ []string) map[string]string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), childEnvBudget)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "env")
	cmd.Env = environ
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("/bin/sh -c env: %v", err)
	}
	child := make(map[string]string)
	for line := range strings.SplitSeq(string(out), "\n") {
		name, value, named := strings.Cut(line, "=")
		if !named {
			continue
		}
		if _, twice := child[name]; twice {
			t.Errorf("the child carries %s twice", name)
		}
		child[name] = value
	}
	return child
}

// TestMustRegisterRefusesASpellingCollision holds the registry invariant the
// hyphen reading needs: two keys Get cannot tell apart are never both
// registered, so one value never answers both.
//
// VALIDATES: a key whose normalized spelling is another key's panics, as an
// alias does, and leaves none of its spellings claimed; registering one key
// twice does not.
// PREVENTS: ze.a.b-c and ze.a.b.c silently sharing whatever value is set.
func TestMustRegisterRefusesASpellingCollision(t *testing.T) {
	const first = "ze.test.collide.a-b"
	t.Cleanup(func() {
		delete(registered, first)
		delete(spellings, normalize(first))
	})
	MustRegister(EnvEntry{Key: first, Type: "string", Description: "first"})
	MustRegister(EnvEntry{Key: first, Type: "string", Description: "first again"})

	for _, entry := range []EnvEntry{
		{Key: "ze.test.collide.a.b", Type: "string", Description: "dotted twin"},
		{Key: "ze.test.collide.other", Type: "string", Description: "alias twin", Aliases: []string{"ZE_TEST_COLLIDE_A_B"}},
	} {
		if !panics(func() { MustRegister(entry) }) {
			t.Errorf("registering %+v beside %s did not panic", entry, first)
		}
		if _, kept := registered[entry.Key]; kept {
			t.Errorf("the refused key %s was registered", entry.Key)
		}
		for _, spelling := range append([]string{entry.Key}, entry.Aliases...) {
			if owner := spellings[normalize(spelling)]; owner == entry.Key {
				t.Errorf("the refused key %s left its spelling %s claimed", entry.Key, spelling)
			}
		}
	}
}

// panics reports whether fn panicked.
func panics(fn func()) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	fn()
	return false
}
