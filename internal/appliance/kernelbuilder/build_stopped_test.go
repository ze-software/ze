package kernelbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildStopCase is one way a build is stopped, as Build itself meets it. A
// canceled build is canceled at the step under test (atStep), so the stop
// cannot land in an earlier step that names its own stop; a deadline is long
// enough that the fast steps before the one under test finish first.
type buildStopCase struct {
	name    string
	context func() (ctx context.Context, cancel, atStep context.CancelFunc)
	want    error
	say     string
}

// buildStopCases are the three stops stopCases covers, timed for a run of
// Build rather than of one step.
func buildStopCases() []buildStopCase {
	const after = 2 * time.Second
	noop := func() {}
	return []buildStopCase{
		{"canceled", func() (context.Context, context.CancelFunc, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			return ctx, cancel, cancel
		}, context.Canceled, "the kernel build was canceled"},
		{"caller deadline", func() (context.Context, context.CancelFunc, context.CancelFunc) {
			ctx, cancel := context.WithTimeout(context.Background(), after)
			return ctx, cancel, noop
		}, context.DeadlineExceeded, "the kernel build ran out of the time its caller gave it"},
		{"build limit", func() (context.Context, context.CancelFunc, context.CancelFunc) {
			ctx, cancel := WithBuildLimit(context.Background(), after)
			return ctx, cancel, noop
		}, context.DeadlineExceeded, "the kernel build did not finish within the 2s limit Ze gives one kernel build"},
	}
}

// checkStoppedPartWay fails t unless err is Build's backstop report for tc:
// the stop named, the raw step error kept inside it, and a deadline ending
// with the backstop's own next step.
func checkStoppedPartWay(t *testing.T, err error, tc buildStopCase, stepSaid string) {
	t.Helper()
	if err == nil {
		t.Fatal("a stopped build was reported as a build")
	}
	report := err.Error()
	if !errors.Is(err, tc.want) {
		t.Errorf("the report does not carry %v:\n%v", tc.want, err)
	}
	if !strings.Contains(report, "the kernel build was stopped part way (its last step said: "+stepSaid) {
		t.Errorf("a stop a step returned raw is not named by Build:\n%v", err)
	}
	if !strings.Contains(report, "): "+tc.say) {
		t.Errorf("the report does not say %q:\n%v", tc.say, err)
	}
	if !errors.Is(tc.want, context.DeadlineExceeded) {
		return
	}
	lines := strings.Split(report, "\n")
	if last, want := lines[len(lines)-1], "the build output above shows the last step it reached"; last != want {
		t.Errorf("a build out of time does not end with %q:\n%v", want, err)
	}
}

// qemuBuildRequest is a request Build accepts for the QEMU backend on this
// host, with its one config fragment written under a fresh root.
func qemuBuildRequest(t *testing.T, builder string) Request {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "configs/kernel.config", "CONFIG_X=y\n")
	discard, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { discard.Close() }) //nolint:errcheck,gosec // test output sink
	return Request{
		Root: root, Version: "7.1.1", Arch: hostGOARCH, Profile: "qemu", Target: "installer", Modules: "no",
		SourceDir: "configs", OutputDir: "out", BuilderDir: "tools/kernel-builder", CommonDir: "common",
		Fragments: []string{"configs/kernel.config"}, Builder: builder,
		Stdout: discard, Stderr: discard,
	}
}

// fakeQEMUListing puts both QEMU binaries on PATH, each answering -accel help
// with accels, and plays a host with no Docker.
func fakeQEMUListing(t *testing.T, accels string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'Accelerators supported in QEMU binary:\\n" + accels + "'\n"
	for _, name := range []string{"qemu-system-aarch64", "qemu-system-x86_64"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil { //nolint:gosec // test executable
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := commandAvailable
	commandAvailable = func(name string) bool { return name != backendDocker }
	t.Cleanup(func() { commandAvailable = old })
}

// cacheAlpineISO writes an Alpine ISO and its checksum sidecar where
// ensureAlpineISO looks under XDG_CACHE_HOME, so the build makes no download.
func cacheAlpineISO(t *testing.T, arch string) {
	t.Helper()
	cache := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CACHE_HOME", cache)
	dir := filepath.Join(cache, "ze", "alpine-iso")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "alpine-virt-" + alpineVersion + "." + alpineMinor + "-" + alpineArch(arch) + ".iso"
	body := []byte("not an ISO")
	sum := sha256.Sum256(body)
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".sha256"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestBuildNamesAStopAStepReturnedRaw proves Build's own backstop call: a
// stop that lands after the worker build, in a step that returns its
// library's error raw, is reported by Build as the build stopping part way,
// with that error kept. The step it reaches is QEMU's Start under
// exec.CommandContext, which answers "start QEMU: context canceled"
// (freeTCPPort's Listen of a literal address does not read a done context);
// on a host with no aarch64 firmware it is qemuArgs' refusal, raw as well. It drives Build itself on the QEMU path: a cached
// Alpine ISO, a stubbed worker build that returns once the stop has landed,
// and a PATH QEMU that lists both accelerators.
//
// VALIDATES: round-10 ISSUE 1 (the backstop's call site in Build).
// PREVENTS: a stop at QEMU's Start or freeTCPPort's Listen reported as
// "start QEMU: context canceled", or as the limit's own noun phrase.
func TestBuildNamesAStopAStepReturnedRaw(t *testing.T) {
	for _, tc := range buildStopCases() {
		t.Run(tc.name, func(t *testing.T) {
			fakeQEMUListing(t, "hvf\\nkvm\\ntcg\\n")
			oldProbes, oldWorker := acceleratorProbes, runWorkerBuild
			t.Cleanup(func() { acceleratorProbes, runWorkerBuild = oldProbes, oldWorker })
			usable := func(context.Context) error { return nil }
			acceleratorProbes = map[string]func(context.Context) error{accelHVF: usable, accelKVM: usable}
			req := qemuBuildRequest(t, backendQEMU)
			cacheAlpineISO(t, req.Arch)
			ctx, cancel, atStep := tc.context()
			defer cancel()
			runWorkerBuild = func(ctx context.Context, _ Request, _ []string, _ ...string) error {
				atStep()
				<-ctx.Done()
				return nil
			}
			checkStoppedPartWay(t, Build(ctx, req), tc, "")
		})
	}
}

// TestBuildNamesAStopDuringBuilderSelection proves Build names a stop that
// lands while SelectBuilder probes the accelerator: on a Mac, probeHVF runs
// sysctl, and a stop there comes back as exec's error inside the refusal
// "no builder available". A PATH QEMU that lists hvf only plays that Mac, and
// the probe is stopped while it runs the real probeHVF.
//
// VALIDATES: round-10 NOTE 1 (SelectBuilder's error through stopReport).
// PREVENTS: a Ctrl-C during the builder probe read as "this host cannot use
// hvf", a refusal of the host rather than the operator's own stop.
func TestBuildNamesAStopDuringBuilderSelection(t *testing.T) {
	for _, tc := range buildStopCases() {
		t.Run(tc.name, func(t *testing.T) {
			fakeQEMUListing(t, "hvf\\ntcg\\n")
			old := acceleratorProbes
			t.Cleanup(func() { acceleratorProbes = old })
			ctx, cancel, atStep := tc.context()
			defer cancel()
			acceleratorProbes = map[string]func(context.Context) error{accelHVF: func(probe context.Context) error {
				atStep()
				<-ctx.Done()
				return probeHVF(probe)
			}}
			checkStoppedPartWay(t, Build(ctx, qemuBuildRequest(t, "")), tc, "no builder available: ")
		})
	}
}
