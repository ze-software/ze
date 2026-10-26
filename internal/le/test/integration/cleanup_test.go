package testintegration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// fakeCleaner records how the cleanup verb drove the orphan cleanup.
// canceled records, per CleanupOrphans call, whether its context was already
// done, and err is what every call answers.
type fakeCleaner struct {
	probed   bool
	dry      []bool
	canceled []bool
	report   interoplab.OrphanReport
	err      error
	// group, when set, is a process group whose presence each sweep records
	// in groupLive, so a test can tell whether the sweep ran before it emptied.
	group     int
	groupLive []bool
}

func (f *fakeCleaner) Probe(context.Context) error { f.probed = true; return nil }

func (f *fakeCleaner) CleanupOrphans(ctx context.Context, dry bool) (interoplab.OrphanReport, error) {
	f.dry = append(f.dry, dry)
	f.canceled = append(f.canceled, ctx.Err() != nil)
	if f.group > 0 {
		f.groupLive = append(f.groupLive, !errors.Is(syscall.Kill(-f.group, 0), syscall.ESRCH))
	}
	f.report.Dry = dry
	return f.report, f.err
}

// VALIDATES: the wiring row "./le test integration clean [dry] -> the
// registered action -> Docker.CleanupOrphans": the verb is in the area's
// table with the closed keyword grammar `dry`, and its handler probes Docker
// and runs one orphan cleanup, answering the report and its code.
func TestLabCleanupActionDispatches(t *testing.T) {
	listing := metadataOnly().Actions()
	found := false
	for _, row := range listing.Actions {
		if row.Verb == cleanupVerb {
			found = true
		}
	}
	if !found {
		t.Fatalf("`%s %s` is not in the area's listing", Area, cleanupVerb)
	}
	action := actionNamed(t, cleanupVerb)
	if action.NativeArgs == nil {
		t.Fatal("cleanup has no native argument-aware runner")
	}
	if len(action.Parameters) != 1 || action.Parameters[0].Keyword != "dry" {
		t.Fatalf("cleanup grammar = %#v, want the one keyword dry", action.Parameters)
	}

	cleaner := &fakeCleaner{report: interoplab.OrphanReport{Failed: []interoplab.OrphanResource{{Name: "lab-peer"}}}}
	payload, code := labCleanup(t.Context(), cleaner, false)
	if !cleaner.probed {
		t.Error("cleanup ran without probing Docker")
	}
	if !slices.Equal(cleaner.dry, []bool{false}) {
		t.Errorf("CleanupOrphans calls = %v, want one non-dry call", cleaner.dry)
	}
	if _, ok := payload.(interoplab.OrphanReport); !ok {
		t.Errorf("payload = %T, want the report so `| json` renders it", payload)
	}
	if code != 1 {
		t.Errorf("code = %d, want 1 for a failed removal", code)
	}
}

// VALIDATES: AC-10: `cleanup dry` asks the orphan cleanup for a dry run and
// exits 0; the dry run itself removing nothing is
// TestCleanupOrphansNeverTouchesUnlabeledVolumesOrCacheTags.
func TestLabCleanupDryRemovesNothing(t *testing.T) {
	cleaner := &fakeCleaner{}
	payload, code := labCleanup(t.Context(), cleaner, true)
	if !slices.Equal(cleaner.dry, []bool{true}) {
		t.Errorf("CleanupOrphans calls = %v, want one dry call", cleaner.dry)
	}
	report, ok := payload.(interoplab.OrphanReport)
	if !ok || !report.Dry {
		t.Errorf("payload = %#v, want a dry report", payload)
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	var arguments leaction.Arguments
	if cleanupDry(arguments) {
		t.Error("a bare `cleanup` was read as dry")
	}
}

// VALIDATES: AC-8 for the integration area: a native verb's root context
// cancels on SIGINT and on SIGTERM, so the teardown the lab already defers
// runs instead of the process dying with its containers left behind.
// PREVENTS: Go's default signal action ending le without running a defer.
func TestNativeRunnerCancelsOnSIGINT(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		run := nativeRunner("/fixture/root", func(ctx context.Context, _ string) (any, int) {
			if err := syscall.Kill(os.Getpid(), signal); err != nil {
				t.Fatalf("signal self: %v", err)
			}
			select {
			case <-ctx.Done():
				return "canceled", 1
			case <-time.After(10 * time.Second):
				return "not canceled", 0
			}
		})
		payload, code := run()
		if payload != "canceled" || code != 1 {
			t.Errorf("%v: payload %v code %d, want the root context canceled", signal, payload, code)
		}
	}
}

// VALIDATES: AC-22: docker-peers runs exactly the packages whose test files
// name the docker tag, sets ZE_REQUIRE_DOCKER=1, refuses an empty set, and
// picks up a package added later with no edit to the verb.
func TestDockerPeersVerbRunsEveryDockerPackage(t *testing.T) {
	root := t.TempDir()
	write := func(rel, constraint string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(constraint+"\n\npackage x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := dockerPeersArgv(root); err == nil {
		t.Fatal("docker-peers accepted a tree with no docker-tagged package")
	}

	write("internal/a/a_test.go", "//go:build docker")
	write("internal/b/b_test.go", "//go:build integration")
	write("cmd/c/c_test.go", "//go:build docker && linux")
	write("internal/d/testdata/d_test.go", "//go:build docker")
	write("internal/f/f_test.go", "//go:build dockerish")
	argv, err := dockerPeersArgv(root)
	if err != nil {
		t.Fatalf("dockerPeersArgv: %v", err)
	}
	wantPrefix := []string{goTool, "test", "-tags", dockerTag, "-count=1"}
	if !slices.Equal(argv[:len(wantPrefix)], wantPrefix) {
		t.Errorf("argv = %v, want it to open with %v", argv, wantPrefix)
	}
	packages := argv[len(argv)-2:]
	if !slices.Equal(packages, []string{"./cmd/c", "./internal/a"}) {
		t.Errorf("packages = %v, want exactly the docker-tagged ./cmd/c and ./internal/a (argv %v)", packages, argv)
	}

	write("internal/e/e_test.go", "//go:build docker")
	argv, err = dockerPeersArgv(root)
	if err != nil {
		t.Fatalf("dockerPeersArgv: %v", err)
	}
	if !slices.Contains(argv, "./internal/e") {
		t.Errorf("a docker package added later is not run: %v", argv)
	}

	environment := requireDockerEnvironment([]string{"PATH=/bin"})
	if !slices.Contains(environment, requireDockerVariable+"=1") || !slices.Contains(environment, "PATH=/bin") {
		t.Errorf("environment = %v, want PATH kept and %s=1 added", environment, requireDockerVariable)
	}
}

// VALIDATES: AC-15: live-rpki runs the three live RPKI tests through the same
// ZE_REQUIRE_DOCKER=1 path as docker-peers: its row is Native, never a bare
// argv that would run without the switch.
// PREVENTS: a nightly live-rpki run passing because Docker was absent and
// every test skipped.
func TestLiveRPKIRequiresDocker(t *testing.T) {
	var row *Action
	table := Table()
	for i := range table {
		if table[i].Verb == liveRPKIVerb {
			row = &table[i]
		}
	}
	if row == nil {
		t.Fatalf("no %s row", liveRPKIVerb)
	}
	if row.Argv != nil {
		t.Fatalf("%s runs a bare argv, which carries no %s", liveRPKIVerb, requireDockerVariable)
	}
	if row.Native == nil {
		t.Fatalf("%s has no native runner", liveRPKIVerb)
	}
	argv := liveRPKIArgv()
	for _, want := range []string{"live", "./internal/component/bgp/plugins/rpki/...", "TestLive"} {
		if !slices.Contains(argv, want) {
			t.Errorf("argv %v lacks %q", argv, want)
		}
	}
}
