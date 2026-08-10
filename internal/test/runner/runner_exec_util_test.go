// VALIDATES: the orchestrated .ci runner awaits foreground quick-exit ze
//            subcommands (config/show/format/...) but not daemons (`ze -`,
//            web servers, hub/start/cli/monitor), via isQuickExitZeCommand.
// PREVENTS: (1) the race where two un-awaited quick-exit ze steps share the
//            client stdout/stderr buffers and a later step clobbers an earlier
//            step's output (isis-config, pipe-operators); (2) the inverse
//            regression where a daemon (e.g. `ze --web ... --insecure-web`) is
//            mis-classified as quick-exit and awaited, hanging the loop forever.

package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCopyTestScripts verifies the ze_api-under-uid-drop fix: the .py test-support
// modules are copied from <baseDir>/test/scripts into the observer's tmpfs workdir
// (which is on its sys.path), and non-.py files and a missing source dir are handled.
func TestCopyTestScripts(t *testing.T) {
	base := t.TempDir()
	srcDir := filepath.Join(base, "test", "scripts")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ze_api.py"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "notes.txt"), []byte("skip me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	// A test-shipped module of the same name must win (not be clobbered by the repo copy).
	if err := os.WriteFile(filepath.Join(dst, "shared.py"), []byte("OWN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "shared.py"), []byte("REPO\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyTestScripts(base, dst); err != nil {
		t.Fatalf("copyTestScripts: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "ze_api.py")); err != nil || string(got) != "X=1\n" {
		t.Errorf("ze_api.py not copied correctly: got %q err %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "shared.py")); err != nil || string(got) != "OWN\n" {
		t.Errorf("a test's own module must not be clobbered: got %q err %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "notes.txt")); !os.IsNotExist(err) {
		t.Error("non-.py file must not be copied")
	}

	// A missing source dir is not an error (a test with no observer needs nothing).
	if err := copyTestScripts(t.TempDir(), dst); err != nil {
		t.Errorf("missing source dir must be a no-op, got %v", err)
	}
}

// TestWithParallelHeadroom checks that per-test timeouts are widened only when
// the Run executes tests concurrently (concurrency > 1), leaving serial runs
// (-p 1 or a single selected test) and the unset/zero state untouched so a real
// slowdown still surfaces against the authored timeout.
func TestWithParallelHeadroom(t *testing.T) {
	const base = 10 * time.Second
	cases := []struct {
		name        string
		concurrency int
		want        time.Duration
	}{
		{"zero (outside a Run) unchanged", 0, base},
		{"serial run unchanged", 1, base},
		{"parallel run widened", 2, base * ParallelTimeoutHeadroom},
		{"high concurrency widened", 20, base * ParallelTimeoutHeadroom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &Runner{concurrency: c.concurrency}
			if got := r.withParallelHeadroom(base); got != c.want {
				t.Fatalf("concurrency=%d: got %v, want %v", c.concurrency, got, c.want)
			}
		})
	}
}

// TestParallelFactor checks the integer multiplier used for COUNT-based inner
// budgets (the HTTP readiness-poll retry count). It must match the concurrency
// gate withParallelHeadroom uses for durations: 1 serial, ParallelTimeoutHeadroom
// concurrent. The two must agree so the fixed inner readiness gates (bind barrier,
// daemon.ready wait, HTTP wait/retry) scale identically and stay tight serially.
func TestParallelFactor(t *testing.T) {
	const base = 4 * time.Second
	cases := []struct {
		name        string
		concurrency int
		wantFactor  int
	}{
		{"zero (outside a Run) not widened", 0, 1},
		{"serial run not widened", 1, 1},
		{"parallel run widened", 2, ParallelTimeoutHeadroom},
		{"high concurrency widened", 20, ParallelTimeoutHeadroom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &Runner{concurrency: c.concurrency}
			if got := r.parallelFactor(); got != c.wantFactor {
				t.Fatalf("concurrency=%d: parallelFactor()=%d, want %d", c.concurrency, got, c.wantFactor)
			}
			// The duration and count budgets must scale by the same factor, or a
			// widened wait would poll a still-tight number of times.
			wantDur := base * time.Duration(c.wantFactor)
			if got := r.withParallelHeadroom(base); got != wantDur {
				t.Fatalf("concurrency=%d: withParallelHeadroom(%v)=%v, want %v", c.concurrency, base, got, wantDur)
			}
		})
	}
}

// TestFirstZeSubcommand checks that the ze verb is found past leading flags and
// that the daemon "read config from stdin" sentinel "-" is not treated as a verb.
func TestFirstZeSubcommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"plain verb", []string{"config", "validate", "-"}, "config"},
		{"leading flags skipped", []string{"-d", "--color", "doctor", "x.conf"}, "doctor"},
		{"bare dash is not a verb", []string{"-"}, ""},
		{"dash then nothing", []string{"--debug", "-"}, ""},
		{"isis verb", []string{"isis"}, "isis"},
		{"empty args", nil, ""},
		{"web daemon", []string{"--web", "8080", "x.conf"}, "8080"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstZeSubcommand(c.args); got != c.want {
				t.Fatalf("firstZeSubcommand(%q) = %q, want %q", c.args, got, c.want)
			}
		})
	}
}

// TestIsQuickExitZeCommand verifies that quick-exit ze subcommands (awaited in
// the loop) are distinguished from daemon invocations (started, not awaited).
// The quick cases mirror real .ci command shapes across the isis/parse/ui
// suites; the daemon cases mirror the config-file invocations used by
// ldp/rsvpte/static/reload (`ze -`, `ze x.conf`) plus the explicit daemon verbs
// so a future edit cannot silently start awaiting a daemon (which would hang).
func TestIsQuickExitZeCommand(t *testing.T) {
	quick := [][]string{
		{"config", "validate", "-"},
		{"show", "bgp", "peer", "list"},
		{"explain", "doctor-isis-net-missing"},
		{"doctor", "--json", "isis-mismatch.conf"}, // .conf is an arg to a verb, not the daemon config
		{"isis", "decode"},
		{"schema", "tree"},
		{"version"},
		{"completion", "bash"},
		{"bgp", "decode", "deadbeef"}, // offline bgp tool, not a daemon
		{"pipe", "json"},              // pipe-operators race source
		{"debug"},                     // ze debug help
		{"env"},
		{"run", "help"}, // run help is quick-exit, unlike a run daemon
		{"interface", "list"},
		{"-d", "config", "validate", "-"}, // leading flag, still quick
	}
	for _, a := range quick {
		if !isQuickExitZeCommand(a) {
			t.Errorf("isQuickExitZeCommand(%q) = false, want true", a)
		}
	}

	daemon := [][]string{
		{"-"},                                  // ze -  (config from stdin)
		{"x.conf"},                             // ze config-file
		{"ze-bgp.conf"},                        // bare config filename used by reload tests
		{"hub.conf"},                           // config file named hub.conf, not the hub verb
		{"--plugin", "ze.bgp-adj-rib-in", "-"}, // config from stdin with plugins
		{"--web", "8080", "x.conf"},            // web daemon with a config file
		{"--web", "8081", "--insecure-web"},    // web-server daemon, no config file (web-tool-decode)
		{"--pprof", "127.0.0.1:9000", "-"},     // pprof + config from stdin
		{"hub"},                                // explicit daemon verb
		{"start"},                              // explicit daemon verb
		{"cli"},                                // interactive, blocks on stdin
		{"monitor", "bgp"},                     // continuous streaming
	}
	for _, a := range daemon {
		if isQuickExitZeCommand(a) {
			t.Errorf("isQuickExitZeCommand(%q) = true, want false", a)
		}
	}
}

func TestSyncWriterCapsOutput(t *testing.T) {
	sw := &syncWriter{pattern: "needle"}

	// Fill to exactly the cap with a first write.
	half := make([]byte, maxOutputBytes/2)
	for i := range half {
		half[i] = 'a'
	}
	n1, err := sw.Write(half)
	if err != nil {
		t.Fatalf("first Write error: %v", err)
	}
	if n1 != len(half) {
		t.Fatalf("first Write returned %d, want %d", n1, len(half))
	}

	// Second write overflows the cap; buffer should stop at maxOutputBytes.
	overflow := make([]byte, maxOutputBytes)
	for i := range overflow {
		overflow[i] = 'b'
	}
	n2, err := sw.Write(overflow)
	if err != nil {
		t.Fatalf("second Write error: %v", err)
	}
	// Write MUST report the caller's full length even though the cap truncated
	// what it stored. This assertion previously demanded the truncated count,
	// which encoded a real bug: os/exec's per-stream io.Copy goroutine treats a
	// short count as io.ErrShortWrite, aborts the copy, and the child then dies
	// on EPIPE while cmd.Wait() reports "short write" -- read by an
	// expect=exit:code=0 as a test failure. The cap bounds memory; it must never
	// signal. Assertion strengthened, not relaxed: it still pins an exact value.
	if n2 != len(overflow) {
		t.Fatalf("second Write returned %d, want %d (the caller's full length)", n2, len(overflow))
	}

	// The cap bounds content at maxOutputBytes and then says so exactly once, so
	// a reader can tell a truncated capture from a complete one.
	got := sw.String()
	if want := maxOutputBytes + len(truncationMarker); len(got) != want {
		t.Fatalf("buffered output = %d bytes, want %d (cap + one truncation marker)", len(got), want)
	}
	if n := strings.Count(got, truncationMarker); n != 1 {
		t.Fatalf("truncation marker appears %d times, want exactly 1", n)
	}

	// A further overflowing write must not append a second marker.
	if _, err := sw.Write(overflow); err != nil {
		t.Fatalf("third Write error: %v", err)
	}
	if n := strings.Count(sw.String(), truncationMarker); n != 1 {
		t.Fatalf("after a second overflow the marker appears %d times, want 1", n)
	}
}

// TestLockedBuilderConcurrentWrites drives lockedBuilder the way os/exec does:
// one copy goroutine per stream per client process, all appending to the same
// accumulator.
//
// VALIDATES: every line written by every concurrent producer survives, so an
// expect=stderr:pattern= sees output the process really printed.
// PREVENTS: regressing the runner-side half of the bmp-locrib (test 97) flake.
// clientStdout/clientStderr were a bare strings.Builder shared by the ze daemon
// and every cmd=background helper; concurrent appends dropped whole lines, so a
// python collector's sentinel vanished from the capture and the test failed
// under load claiming a pattern the collector had in fact printed.
func TestLockedBuilderConcurrentWrites(t *testing.T) {
	var b lockedBuilder

	const producers = 8
	const linesEach = 500

	var wg sync.WaitGroup
	wg.Add(producers)
	for p := range producers {
		go func() {
			defer wg.Done()
			line := []byte("PRODUCER-" + strconv.Itoa(p) + "\n")
			for range linesEach {
				if _, err := b.Write(line); err != nil {
					t.Errorf("Write: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got := b.String()
	for p := range producers {
		want := "PRODUCER-" + strconv.Itoa(p) + "\n"
		if n := strings.Count(got, want); n != linesEach {
			t.Errorf("producer %d wrote %d lines, capture has %d", p, linesEach, n)
		}
	}
}

// TestLockedBuilderCapsOutput mirrors TestSyncWriterCapsOutput: the cap bounds
// memory, but Write still reports the caller's full length so io.Copy does not
// abort with ErrShortWrite and truncate the rest of a process's output.
func TestLockedBuilderCapsOutput(t *testing.T) {
	var b lockedBuilder

	half := bytes.Repeat([]byte{'a'}, maxOutputBytes/2)
	n1, err := b.Write(half)
	if err != nil {
		t.Fatalf("first Write error: %v", err)
	}
	if n1 != len(half) {
		t.Fatalf("first Write returned %d, want %d", n1, len(half))
	}

	overflow := bytes.Repeat([]byte{'b'}, maxOutputBytes)
	n2, err := b.Write(overflow)
	if err != nil {
		t.Fatalf("second Write error: %v", err)
	}
	if n2 != len(overflow) {
		t.Fatalf("second Write returned %d, want %d (the caller's full length)", n2, len(overflow))
	}

	// The orchestrated clientStdout/clientStderr were UNCAPPED bare builders
	// before lockedBuilder replaced them, so this cap is new. It must announce
	// itself: otherwise a positive expect=stdout:pattern= whose needle lands past
	// 10 MB fails over a capture that looks complete (ai/rules/evidence.md).
	got := b.String()
	if want := maxOutputBytes + len(truncationMarker); len(got) != want {
		t.Fatalf("buffered output = %d bytes, want %d (cap + one truncation marker)", len(got), want)
	}
	if n := strings.Count(got, truncationMarker); n != 1 {
		t.Fatalf("truncation marker appears %d times, want exactly 1", n)
	}
}

// TestResolveOrchestratedTimeout verifies the precedence of the three timeout
// sources for orchestrated (cmd=) tests, and in particular that the record-level
// `option=timeout:value=` is honored at all.
//
// VALIDATES: a declared .ci timeout takes effect on the cmd= path.
// PREVENTS: the regression this fixed -- `option=timeout:` was read only on the
// non-orchestrated path, so every cmd= test silently ran on the global default
// while its .ci claimed a budget. test/appliance/vpp-hugepages-qemu.ci was
// killed at 15s despite declaring 900s.
func TestResolveOrchestratedTimeout(t *testing.T) {
	const suggested = 15 * time.Second
	fg := func(to string) []RunCommand {
		return []RunCommand{{Mode: modeForeground, Timeout: to}}
	}

	for _, tc := range []struct {
		name   string
		record string
		cmds   []RunCommand
		want   time.Duration
	}{
		{"nothing declared falls back", "", nil, suggested},
		{"record-level option wins over the fallback", "900s", nil, 900 * time.Second},
		{"per-command timeout wins over the record option", "900s", fg("30s"), 30 * time.Second},
		{"per-command timeout alone", "", fg("42s"), 42 * time.Second},
		{"unparseable record option is ignored", "later", nil, suggested},
		{"unparseable per-command timeout keeps the record option", "900s", fg("soon"), 900 * time.Second},
		{"background commands do not set the budget", "", []RunCommand{{Mode: modeBackground, Timeout: "99s"}}, suggested},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveOrchestratedTimeout(suggested, tc.record, tc.cmds); got != tc.want {
				t.Errorf("resolveOrchestratedTimeout(%v, %q, %v) = %v, want %v", suggested, tc.record, tc.cmds, got, tc.want)
			}
		})
	}
}

// VALIDATES: childEnv carries GOTRACEBACK=all and preserves caller entries, AND
// that every exec site in this package routes through it -- the helper being
// correct is worth nothing if a launch path bypasses it.
// PREVENTS: the bypass this test was added without and did not catch. The first
// version wired childEnv into runTest's two sites only; every .ci carrying `cmd=`
// directives (all of test/ospf, including the ospf-ldp-sync-restore crash this
// came from) returns early to runOrchestrated, whose exec site was still building
// its own env. 82 ze daemons in one ze-ospf-test run, none with the variable set.
//
// NOTE on what the variable is for: a runtime THROW ("fatal error: ...") already
// dumps every goroutine without it. It matters for a user-level runtime panic,
// which otherwise prints only the panicking goroutine. See childEnv's doc.
func TestChildEnvCarriesGotraceback(t *testing.T) {
	env := childEnv()
	if !slices.Contains(env, "GOTRACEBACK=all") {
		t.Fatalf("childEnv() must set GOTRACEBACK=all, got %d entries without it", len(env))
	}

	// Caller entries come AFTER, so a caller that sets its own GOTRACEBACK wins:
	// os/exec dedups its Env keeping the LAST occurrence (exec.go, dedupEnvCase).
	env = childEnv("ze_test_bgp_port=1179", "GOTRACEBACK=single")
	last := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOTRACEBACK=") {
			last = kv
		}
	}
	if last != "GOTRACEBACK=single" {
		t.Errorf("caller override must win, last GOTRACEBACK entry = %q", last)
	}
	if !slices.Contains(env, "ze_test_bgp_port=1179") {
		t.Error("childEnv dropped a caller entry")
	}
}

// VALIDATES: no exec site in this package builds a child environment by hand.
// PREVENTS: a launch path silently opting out of childEnv, which is exactly how
// runOrchestrated -- the path every `cmd=`-driven .ci takes -- was missed.
func TestEveryExecSiteUsesChildEnv(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		checked++
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "append(os.Environ()") {
				continue
			}
			t.Errorf("%s:%d builds a child environment by hand:\n\t%s\n"+
				"use childEnv(...) so the child inherits the runner's standard env",
				name, i+1, strings.TrimSpace(line))
		}
	}
	if checked == 0 {
		t.Fatal("scanned no source files; the test cannot be gating anything")
	}
}

// TestSelfStopGraceStaysInsideTheBudget drives the derived grace at the budgets
// the plugin suite really uses.
//
// VALIDATES: the grace is a strict fraction of the wall-clock the daemon races,
// so it is reachable at every budget and always leaves the runner time to
// signal, reap and report inside the same deadline.
// PREVENTS: the constant this replaces. A wait unrelated to the deadline that
// kills the process is either never reached or larger than the whole run
// (`(*Runner).testBudgetEnv`, same reasoning).
// DISCRIMINATES: raise selfStopShare to 1.0 and the upper bound fails; drop it
// to 0 and the lower bound fails, which is terminateGracefully's behavior.
func TestSelfStopGraceStaysInsideTheBudget(t *testing.T) {
	for _, budget := range []time.Duration{
		time.Second, 15 * time.Second, 20 * time.Second, 2 * time.Minute,
	} {
		grace := selfStopGrace(budget)
		if grace <= 0 {
			t.Errorf("budget %v: grace %v is not a wait at all", budget, grace)
		}
		if grace >= budget {
			t.Errorf("budget %v: grace %v leaves the runner no time to tear down and report",
				budget, grace)
		}
	}
}

// TestTmpfsRequestsDaemonShutdown reads the marker out of the shapes an observer
// really writes it in.
//
// VALIDATES: a .ci whose embedded observer asks the daemon to stop is
// recognized, and one that never does is not.
// PREVENTS: giving the self-exit grace to the ~105 plugin .ci files whose daemon
// never stops itself, which would add the whole grace to each of their runs.
func TestTmpfsRequestsDaemonShutdown(t *testing.T) {
	cases := []struct {
		name  string
		files map[string][]byte
		want  bool
	}{
		{"no tmpfs at all", nil, false},
		{
			"observer dispatches it",
			map[string][]byte{"obs.run": []byte("dispatch(api, 'request shutdown')\n")},
			true,
		},
		{
			"second file carries it",
			map[string][]byte{
				"helper.py": []byte("def noop():\n    return None\n"),
				"obs.run":   []byte("api._call_engine('x', {'command': 'request shutdown'})\n"),
			},
			true,
		},
		{
			"observer never stops the daemon",
			map[string][]byte{"obs.run": []byte("dispatch(api, 'show bgp summary')\n")},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tmpfsRequestsDaemonShutdown(tc.files); got != tc.want {
				t.Errorf("tmpfsRequestsDaemonShutdown = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTerminateAfterSelfExitLetsTheProcessFinish drives both arms against real
// children.
//
// VALIDATES: a process that exits on its own inside the grace is reaped without
// a signal, and one that never exits is still torn down.
// PREVENTS: the failure this function exists for -- the runner signaling the
// daemon while the observer inside it was mid-RPC, which surfaced as
// "no response for ze-plugin-engine:dispatch-command" on six .ci files whose
// wire expectations had already passed.
// DISCRIMINATES: pass grace 0 and the first case's child is signaled before it
// can write its own exit status, which is what the assertion reads.
func TestTerminateAfterSelfExitLetsTheProcessFinish(t *testing.T) {
	t.Run("self exit inside the grace is not signaled", func(t *testing.T) {
		// Exits 7 on its own. A SIGTERM would make the status 143 instead, so
		// the exit code IS the evidence that no signal was sent.
		cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "sleep 0.2; exit 7") //nolint:gosec // fixed argv
		if err := cmd.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		terminateAfterSelfExit(cmd, 10*time.Second)
		if code := cmd.ProcessState.ExitCode(); code != 7 {
			t.Errorf("exit code = %d, want 7 (the child's own status, unsignaled)", code)
		}
	})

	t.Run("a daemon that never stops is still torn down", func(t *testing.T) {
		cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "sleep 300") //nolint:gosec // fixed argv
		if err := cmd.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		start := time.Now()
		terminateAfterSelfExit(cmd, 50*time.Millisecond)
		if elapsed := time.Since(start); elapsed > 30*time.Second {
			t.Errorf("teardown took %v; the grace is not bounding anything", elapsed)
		}
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() == 0 {
			t.Error("a child that never exits must be signaled, not reported as a clean exit")
		}
	})
}
