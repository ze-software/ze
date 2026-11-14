package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// guestBuilds counts the calls a feature run made to each guest build step.
type guestBuilds struct {
	daemons, le, image, kernel int
}

// fakeGuestSteps answers guest build steps over tree that build nothing and
// count their calls into counted. The le step writes a small executable at the
// path it is given, as the real cross-build does, so the run can read its digest.
func fakeGuestSteps(t *testing.T, tree string, counted *guestBuilds) *guestSteps {
	t.Helper()
	return &guestSteps{
		daemons: func(string, string) error { counted.daemons++; return nil },
		le: func(rel, _ string) error {
			counted.le++
			full := filepath.Join(tree, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				return err
			}
			return os.WriteFile(full, []byte("#!/bin/sh\n"), 0o700) //nolint:gosec // the fake le must be executable
		},
		image:  func(string) (string, error) { counted.image++; return "/img", nil },
		kernel: func(string) (string, error) { counted.kernel++; return "/k", nil },
	}
}

// echoSet answers a binary set whose le is a script that prints its own argv
// and a PASS line, so a test reads which command feature run chose.
func echoSet(t *testing.T) testfunctional.BinarySet {
	t.Helper()
	set := t.TempDir()
	script := "#!/bin/sh\necho \"argv: $*\"\necho \"1.0s  1/1  PASS  a  gated\"\n"
	if err := os.WriteFile(filepath.Join(set, testfunctional.LE), []byte(script), 0o700); err != nil { //nolint:gosec // the fake le must be executable
		t.Fatal(err)
	}
	return testfunctional.BinarySet{Dir: set}
}

func repoToolchain(t *testing.T) gotoolchain.Toolchain {
	t.Helper()
	toolchain, err := gotoolchain.New(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("toolchain: %v", err)
	}
	return toolchain
}

// VALIDATES: a .ci the runner says this host skips (runner.NeedsGuest) is run
// in the QEMU guest through `le test qemu docker-lab ... test qemu all test <path>`, and a
// .ci it does not skip still runs through its host runner.
// PREVENTS: feature run observing a host SKIP of a capability-gated test and
// recording nothing, with no route to the place the test really runs.
//
// Method: the set's le is a script that prints its own argv and a PASS line,
// so the test reads which command feature run chose; the guest steps are fakes,
// so nothing compiles.
func TestAGuestOnlyTestRunsInTheQEMUGuest(t *testing.T) {
	toolchain := repoToolchain(t)
	set := echoSet(t)
	for _, guest := range []bool{true, false} {
		tree := t.TempDir()
		runner := &repoRunner{tree: tree, toolchain: toolchain, prepared: true, set: set,
			needsGuest: func(string) (bool, error) { return guest, nil },
			guestSteps: fakeGuestSteps(t, tree, &guestBuilds{})}
		seen, err := runner.run("test/policy/gated.ci")
		if err != nil {
			t.Fatalf("guest=%v: %v", guest, err)
		}
		inGuest := strings.Contains(seen.output, "argv: test qemu docker-lab ") &&
			strings.Contains(seen.output, " test qemu all test test/policy/gated.ci")
		if inGuest != guest {
			t.Fatalf("guest=%v: the command feature run chose was:\n%s", guest, seen.output)
		}
		if problem := observedPass("test/policy/gated.ci", seen); problem != "" {
			t.Fatalf("guest=%v: the test's own PASS line was not read: %s", guest, problem)
		}
	}
}

// VALIDATES: a feature run with several guest-routed tests builds the guest
// route once, in its preflight, and every guest boot after it names that build
// to docker-lab (guest-le, image), so no boot builds the le again from the
// working tree; the guest command runs the le that build wrote.
// PREVENTS: a feature run cross-building the guest le once per guest test from
// a tree other sessions edit, so one half-edited package voids the whole run
// and its tests judge as many builds as it has guest tests.
//
// Method: fake guest steps count their calls; the set's le prints its argv.
func TestAFeatureRunBuildsTheGuestRouteOnce(t *testing.T) {
	tree := t.TempDir()
	counted := &guestBuilds{}
	runner := &repoRunner{tree: tree, toolchain: repoToolchain(t), prepared: true, set: echoSet(t),
		needsGuest: func(string) (bool, error) { return true, nil },
		hostReady:  func() error { t.Error("a guest-only run consulted the host preflight"); return nil },
		guestSteps: fakeGuestSteps(t, tree, counted)}
	items := []string{"test/policy/a.ci", "test/policy/b.ci", "test/policy/c.ci"}
	if err := runner.preflight(items); err != nil {
		t.Fatal(err)
	}
	if *counted != (guestBuilds{daemons: 1, le: 1, image: 1, kernel: 1}) {
		t.Fatalf("the preflight built %+v, want each guest step once before the first test", *counted)
	}
	le := filepath.ToSlash(filepath.Join(guestBinariesRel, "linux-"+testqemu.GuestArch(), "le"))
	for _, item := range items {
		seen, err := runner.run(item)
		if err != nil {
			t.Fatalf("%s: %v", item, err)
		}
		for _, want := range []string{" kernel /k image /img harness guest-le " + le + " timeout ",
			" " + le + " test qemu all test " + item} {
			if !strings.Contains(seen.output, want) {
				t.Fatalf("%s: the docker-lab call lacks %q:\n%s", item, want, seen.output)
			}
		}
	}
	if *counted != (guestBuilds{daemons: 1, le: 1, image: 1, kernel: 1}) {
		t.Fatalf("three guest tests built %+v, want each guest step once in the whole run", *counted)
	}
}

// VALIDATES: a guest le replaced or removed after the run built it refuses the
// next guest test, naming the path and the fix, rather than booting another build.
// PREVENTS: a second feature run's build silently becoming part of this run's
// evidence.
func TestAFeatureRunRefusesAGuestLeReplacedSinceItsBuild(t *testing.T) {
	for _, change := range []string{"replaced", "removed"} {
		t.Run(change, func(t *testing.T) {
			tree := t.TempDir()
			runner := &repoRunner{tree: tree, toolchain: repoToolchain(t), prepared: true, set: echoSet(t),
				needsGuest: func(string) (bool, error) { return true, nil },
				guestSteps: fakeGuestSteps(t, tree, &guestBuilds{})}
			if err := runner.preflight([]string{"test/policy/a.ci"}); err != nil {
				t.Fatal(err)
			}
			full := filepath.Join(tree, runner.guest.le)
			var err error
			if change == "replaced" {
				err = os.WriteFile(full, []byte("#!/bin/sh\nexit 0\n"), 0o700) //nolint:gosec // the fake le must be executable
			} else {
				err = os.Remove(full)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.run("test/policy/a.ci")
			if err == nil {
				t.Fatalf("a %s guest le still booted", change)
			}
			if !strings.Contains(err.Error(), runner.guest.le) || !strings.Contains(err.Error(), "Fix: run one `./le feature run` at a time") {
				t.Fatalf("the refusal does not name the le and its fix: %v", err)
			}
		})
	}
}
