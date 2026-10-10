// The PPPoE lab offers its access-concentrator scenarios only in a ze_l2tp
// build (internal/le/interoplab/pppoe/checkers_l2tp.go), and without them its
// discovery refuses before the kernel check this test is about. Every build
// that runs these proofs carries every feature tag, and so does ./le test unit.

//go:build ze_l2tp

package testdeployment

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/interoplab"
	interopl2tp "github.com/ze-software/ze/internal/le/interoplab/l2tp"
	interoppppoe "github.com/ze-software/ze/internal/le/interoplab/pppoe"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// kernelLackingDocker is a docker on PATH whose daemon kernel lacks PPPoL2TP.
// It records every argv, passes the L2TP and PPPoE labs' own module preflights so the run
// reaches the Docker kernel check, answers that probe with one absent row, and
// answers every other command with success and no output.
const kernelLackingDocker = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*SecurityOptions*) echo '["name=seccomp,profile=builtin","name=cgroupns"]' ;;
*ze-l2tp-preflight*) printf 'DEV_PPP=ok\nL2TP_PPP=ok\nIP_L2TP=ok\n' ;;
*ze-pppoe-preflight*) printf 'DEV_PPP=ok\nPPPOE=ok\n' ;;
*kernel-capabilities*)
  echo '{"ready": false, "capabilities": [{"subsystem": "l2tp-ppp", "kernel": "CONFIG_PPPOL2TP", "state": "absent", "reason": "protocol not supported"}]}'
  exit 1 ;;
esac
exit 0
`

// VALIDATES: AC-3 through the docker-* deployment proofs, typed as an operator
// types them. Each one refuses a daemon kernel that lacks a feature, naming it,
// and builds no image.
// PREVENTS: a deployment proof that runs Ze in Docker without the check the
// interop suites make.
func TestDockerDeploymentRefusesMissingKernelFeature(t *testing.T) {
	for _, verb := range []string{"docker-l2tp-ppp-test", "docker-pppoe-accel-test"} {
		t.Run(verb, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(kernelLackingDocker), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
				t.Fatalf("write the docker stub: %v", err)
			}
			record := filepath.Join(dir, "record")
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DOCKER_RECORD", record)
			// NO_BUILD keeps the run from cross-compiling ze: the stub never
			// reads the binary, and the check is what this test is about. The
			// check refuses a staged path that holds no file, so the run works
			// in a mirror of this checkout that holds a stand-in for each
			// lab's staged ze, and the shared checkout is never written.
			t.Setenv("NO_BUILD", "1")
			useStagedCheckout(t,
				interoplab.StagedZePath("", interopl2tp.LabBinaries()),
				interoplab.StagedZePath("", interoppppoe.LabBinaries()))

			answer, code := Answer([]string{verb})
			if code != 1 {
				t.Fatalf("%s answered %d on a kernel lacking PPPoL2TP, want 1: %+v", verb, code, answer)
			}
			report, ok := answer.(interoplab.SuiteReport)
			if !ok {
				t.Fatalf("%s answered %T, want interoplab.SuiteReport", verb, answer)
			}
			// The route is per host OS: the install action on Linux, the
			// Ze-kernel QEMU guest elsewhere (interoplab.DockerKernelRoute).
			route := interoplab.DockerKernelRoute(runtime.GOOS)
			for _, want := range []string{"l2tp-ppp", "CONFIG_PPPOL2TP", "6.8.0-117-generic", route} {
				if !strings.Contains(report.SetupError, want) {
					t.Errorf("setup error does not name %q: %s", want, report.SetupError)
				}
			}
			recorded, err := os.ReadFile(record) //nolint:gosec // the test's own temp file
			if err != nil {
				t.Fatalf("read the docker record: %v", err)
			}
			if !strings.Contains(string(recorded), "kernel-capabilities") {
				t.Errorf("%s never probed the kernel:\n%s", verb, recorded)
			}
			if strings.Contains(string(recorded), "build") {
				t.Errorf("%s built an image after the refusal:\n%s", verb, recorded)
			}
		})
	}
}

// useStagedCheckout points lepath.Root at a mirror of this checkout: every
// entry is a symbolic link to the real one, except each path in standIns
// (relative to the root), which is a regular stand-in file. env.Get answers
// from a cache built once, so the cache is reset for the test and after it.
func useStagedCheckout(t *testing.T, standIns ...string) {
	t.Helper()
	source, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}
	target := t.TempDir()
	mirrorWithStandIns(t, source, target, standIns)
	t.Setenv("ZE_REPO_ROOT", target)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// mirrorWithStandIns links every entry of source into target, except the
// entries standIns pass through: a stand-in's own name becomes a file, and a
// directory on its way is made and mirrored one level down. The depth is
// bounded by the stand-in paths, which the test names.
func mirrorWithStandIns(t *testing.T, source, target string, standIns []string) {
	t.Helper()
	below := map[string][]string{}
	files := map[string]bool{}
	for _, standIn := range standIns {
		first, rest, nested := strings.Cut(filepath.ToSlash(standIn), "/")
		if !nested {
			files[first] = true
			continue
		}
		below[first] = append(below[first], rest)
	}
	entries, err := os.ReadDir(source)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read %s: %v", source, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if files[name] || below[name] != nil {
			continue
		}
		if err := os.Symlink(filepath.Join(source, name), filepath.Join(target, name)); err != nil {
			t.Fatalf("mirror %s: %v", name, err)
		}
	}
	for name := range files {
		if err := os.WriteFile(filepath.Join(target, name), []byte("stand-in"), 0o600); err != nil {
			t.Fatalf("write the stand-in %s: %v", name, err)
		}
	}
	for name, rests := range below {
		if err := os.Mkdir(filepath.Join(target, name), 0o750); err != nil {
			t.Fatalf("make %s: %v", name, err)
		}
		mirrorWithStandIns(t, filepath.Join(source, name), filepath.Join(target, name), rests)
	}
}
