package qemu

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/functional"
)

// VALIDATES: the suites that program the guest's network state run in a
// per-test network namespace, and their children are told to enter it.
// PREVENTS: the run cutting its own transport. `all-tests` ran every suite in
// the guest ROOT namespace, test/firewall/firewall-nat-exclude.ci installs a
// nat prerouting chain there, and the run's own SSH session lives in it. The
// 2026-09-05 run died inside functional/firewall with ssh's exit 255, and the
// twelve suites, the unit pass, the installer phase and the integration phase
// that were still to come never ran and were never accounted for
// (plan/journal/gate-excludes-part-of-its-population.md).
//
// A guest is what would prove the namespace is really entered. Without one,
// what is provable here is the mapping and the environment the mapping
// produces: which suites are routed, that the routed child carries every
// variable the .ci runner reads before it will enter a namespace, and that a
// child NOT routed carries none of them. The launcher's own guest proof is
// `./le qemu netns-test`, which asserts the guest root nft ruleset is byte
// identical before and after.

// perTestSuites is the set this run routes through the per-test namespace,
// stated here rather than read from vmSuites: a test that derives its
// expectation from the table under test agrees with any edit to it, including
// the edit that puts firewall back in the transport's namespace.
var perTestSuites = []string{"firewall", "policy", "ospf", "ospfv3"}

// The mapping itself. An edit that moves a suite between namespaces has to
// change this line, which is the point.
func TestTheSuitesThatProgramTheGuestNetworkRunOutsideTheTransportsNamespace(t *testing.T) {
	for _, suite := range vmSuites {
		want := guestRoot
		if slices.Contains(perTestSuites, suite.Name) {
			want = perTest
		}
		if suite.Namespace != want {
			t.Errorf("suite %q runs in the %s namespace, want %s", suite.Name, suite.Namespace, want)
		}
	}

	// Every routed suite is a real one, so a rename cannot leave this list
	// naming a suite nothing runs.
	for _, name := range perTestSuites {
		if _, declared := functional.SuiteNamed(name); !declared {
			t.Errorf("%q is routed through the per-test namespace and is not a declared suite", name)
		}
	}
}

// Every suite states where it runs. A row added without an answer is refused
// before the run starts, rather than inheriting the answer that cut the
// transport.
func TestASuiteThatDoesNotSayWhereItRunsIsRefused(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run

	silent := vmSuites[0]
	silent.Namespace = namespaceUnspecified
	restore := vmSuites[0]
	vmSuites[0] = silent
	defer func() { vmSuites[0] = restore }()

	err := run.verify()
	if err == nil {
		t.Fatal("a suite with no declared namespace was accepted")
	}
	if !errors.Is(err, ErrNamespaceUnspecified) {
		t.Errorf("the refusal is %v, want it to be ErrNamespaceUnspecified", err)
	}
	if !strings.Contains(err.Error(), silent.Name) {
		t.Errorf("the refusal is %q, want it to name the suite %q", err, silent.Name)
	}
}

// The routed child carries what the .ci runner reads. Without ZE_TEST_NETNS it
// never enters a namespace, and without a non-root uid it refuses to run at all
// (errNetnsNeedsUID, internal/test/runner/runner_exec_util.go), so both are
// asserted rather than one.
func TestARoutedSuiteChildIsToldToEnterItsOwnNamespace(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()

	routed := environmentOfSuite(t, rec, "firewall")
	for key, want := range map[string]string{
		netnsModeKey:   netnsModeValue,
		netnsUIDKey:    netnsUID,
		netnsGIDKey:    netnsUID,
		netnsConfigKey: netnsStateDir,
		zeBinKey:       netnsCapDir + "/" + zeName,
		strippedBinKey: netnsCapDir + "/" + zeStrippedName,
	} {
		if got := valueOf(routed, key); got != want {
			t.Errorf("the firewall child was given %s=%q, want %q", key, got, want)
		}
	}
}

// A suite that is NOT routed carries none of it. ze then runs as root in the
// guest namespace, which is what every suite outside the four expects.
func TestASuiteInTheGuestNamespaceIsToldNothingAboutNamespaces(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()

	plain := environmentOfSuite(t, rec, "encode")
	for _, key := range []string{netnsModeKey, netnsUIDKey, netnsGIDKey, netnsConfigKey} {
		if value := valueOf(plain, key); value != "" {
			t.Errorf("the encode child was given %s=%q, want it unset: a suite that does not need"+
				" a namespace must not run ze as a dropped user", key, value)
		}
	}
	if got := valueOf(plain, zeBinKey); strings.HasPrefix(got, netnsCapDir) {
		t.Errorf("the encode child was pointed at the capability copy %q", got)
	}
}

// The capability copies are made before the first suite, and the run refuses to
// continue when that fails. A setcap on the 9p mount buys nothing, so the
// binaries are copied to the guest's own tmpfs first.
func TestTheCapabilityCopiesAreMadeBeforeTheFirstSuite(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()

	if len(rec.calls) == 0 {
		t.Fatal("the run emitted no command at all")
	}
	first := strings.Join(rec.calls[0], " ")
	for _, want := range []string{
		"sh -c",
		setcapCommand + " " + netnsCapabilities + " " + netnsCapDir + "/" + zeName,
		setcapCommand + " " + netnsCapabilities + " " + netnsCapDir + "/" + zeStrippedName,
		"chown 1000:1000 " + netnsStateDir,
	} {
		if !strings.Contains(first, want) {
			t.Errorf("the preparation command does not carry %q:\n  %s", want, first)
		}
	}
}

// A guest without libcap cannot give ze its capabilities, and the run says so
// before it boots through an hour of suites that would each fail on it.
func TestAGuestWithoutSetcapIsRefusedBeforeAnythingRuns(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Look = func(name string) (string, error) {
		if name == setcapCommand {
			return "", errors.New("executable file not found in $PATH")
		}
		return "/usr/sbin/" + name, nil
	}

	_, code := run.Execute()
	if code == 0 {
		t.Fatal("a guest with no setcap started the run")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d children ran before the refusal, want 0", len(rec.calls))
	}
}

// Skipping every routed suite needs no preparation and no libcap. The tight
// loop that skips them must not be blocked by a package it will not use.
func TestARunThatSkipsEveryRoutedSuiteNeedsNoCapabilities(t *testing.T) {
	run := vmFixture(t)
	run.Skip = append(append([]string(nil), run.Skip...), perTestSuites...)
	rec := &recorder{}
	run.Run = rec.run
	run.Look = func(string) (string, error) { return "", errors.New("executable file not found in $PATH") }

	_, code := run.Execute()
	if code != 0 {
		t.Fatalf("a run that skips every routed suite exited %d", code)
	}
	for _, call := range rec.calls {
		if strings.Contains(strings.Join(call, " "), setcapCommand) {
			t.Errorf("the run prepared capabilities it does not need: %s", strings.Join(call, " "))
		}
	}
}

// environmentOfSuite answers the environment the child of one suite was given.
func environmentOfSuite(t *testing.T, rec *recorder, suite string) []string {
	t.Helper()
	for i, call := range rec.calls {
		if slices.Contains(call, suite) {
			return rec.envs[i]
		}
	}
	t.Fatalf("no child ran for suite %q", suite)
	return nil
}

// valueOf answers one variable of an environment block, or the empty string.
func valueOf(environ []string, key string) string {
	prefix := key + "="
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}

// The dropped user has to exec through the shim directory. A root-only one
// answers ze's plugin relay with "Permission denied", the plugin never starts,
// and the test times out naming neither the directory nor the user.
func TestTheShimDirectoryIsTraversableByTheDroppedUser(t *testing.T) {
	run := vmFixture(t)
	if err := os.MkdirAll(run.BinDir, 0o700); err != nil {
		t.Fatalf("pre-create the shim directory: %v", err)
	}
	if err := run.shim(); err != nil {
		t.Fatalf("shim: %v", err)
	}

	info, err := os.Stat(run.BinDir)
	if err != nil {
		t.Fatalf("stat the shim directory: %v", err)
	}
	if info.Mode().Perm()&0o005 != 0o005 {
		t.Errorf("the shim directory is %v, want it readable and traversable by every user:"+
			" a suite in a per-test namespace execs ze-test through it as uid %s", info.Mode().Perm(), netnsUID)
	}
}
