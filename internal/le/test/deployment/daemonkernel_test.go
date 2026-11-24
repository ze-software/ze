package testdeployment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// proofDocker is a docker on PATH for the proofs that build their own daemon.
// It records every argv, holds every image already, answers the kernel probe
// with one absent row, and answers any other command with success.
const proofDocker = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*SecurityOptions*) echo '["name=seccomp,profile=builtin","name=cgroupns"]' ;;
*kernel-capabilities*)
  echo '{"ready": false, "capabilities": [{"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "XFRM_MSG_MIGRATE_STATE: invalid argument"}]}'
  exit 1 ;;
esac
exit 0
`

// stubDaemon is the DOCKER_HOST a test names when it puts a docker stub on
// PATH. The stub answers every command itself and dials nothing, so no daemon
// is reached. Naming one keeps the verdict off the host: on macOS an unset
// DOCKER_HOST makes dockerhost.Environment look for the `ze` Colima profile's
// socket and refuse, before the stub runs, whenever that profile is stopped.
const stubDaemon = "unix:///nonexistent/ze-test-docker-stub.sock"

// daemonProof is one deployment proof that builds its own daemon: its name,
// the architecture it builds for, and the call that runs it.
type daemonProof struct {
	name   string
	goarch func(tree string) string
	run    func(tree string) error
}

// daemonProofs answers the kernel-dependent deployment proofs that build their
// own daemon and run Ze in Docker.
func daemonProofs() []daemonProof {
	return []daemonProof{
		{"l2tp", func(tree string) string { return NewL2TP(tree).Goarch }, func(tree string) error {
			_, err := NewL2TP(tree).Run(context.Background())
			return err
		}},
	}
}

// vppUserspaceProofs answers the VPP userspace proofs that run without special
// host kernel capabilities (plan/records/ruling-interop-scoped-kernel-capabilities.md).
func vppUserspaceProofs() []daemonProof {
	return []daemonProof{
		{"vpp", func(tree string) string { return newVPP(tree).Goarch }, func(tree string) error {
			_, err := newVPP(tree).Run(context.Background())
			return err
		}},
		{"vpp-iface", func(tree string) string { return newVPPIface(tree).Goarch }, func(tree string) error {
			_, err := newVPPIface(tree).Run(context.Background())
			return err
		}},
		// TestVPPSRv6ServiceRoute (build tag integration) starts its VPP container
		// and Ze through the same preparation step.
		{"vpp-srv6-service-route", func(tree string) string { return newVPP(tree).Goarch }, func(tree string) error {
			return newVPP(tree).prepareDaemon()
		}},
	}
}

// VALIDATES: AC-3 through the kernel-dependent deployment proofs that build
// their own daemon (l2tp). Each refuses a daemon kernel that lacks a required
// feature, naming it and the kernel release, before any container starts.
// PREVENTS: a kernel-dependent proof running Ze in Docker without checking the
// host kernel capabilities it needs. Removing the check turns its subtest red.
//
// NO_BUILD=1 keeps each run from cross-compiling: the test plants a stand-in at
// the path the build would write, which is the binary the probe mounts.
func TestDaemonProofsRefuseMissingKernelFeature(t *testing.T) {
	proofs := daemonProofs()
	for _, proof := range proofs {
		t.Run(proof.name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(proofDocker), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
				t.Fatalf("write the docker stub: %v", err)
			}
			record := filepath.Join(bin, "record")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DOCKER_RECORD", record)
			t.Setenv("DOCKER_HOST", stubDaemon)
			t.Setenv("NO_BUILD", "1")

			tree := t.TempDir()
			daemon := filepath.Join(tree, daemonRel(proof.goarch(tree)))
			if err := os.MkdirAll(filepath.Dir(daemon), 0o750); err != nil {
				t.Fatalf("make the daemon directory: %v", err)
			}
			if err := os.WriteFile(daemon, []byte("stand-in"), 0o600); err != nil {
				t.Fatalf("write the stand-in daemon: %v", err)
			}

			err := proof.run(tree)
			if err == nil {
				t.Fatalf("%s proceeded on a kernel lacking XFRM_MSG_MIGRATE_STATE", proof.name)
			}
			for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "6.8.0-117-generic"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal does not name %q: %v", want, err)
				}
			}
			recorded, readErr := os.ReadFile(record) //nolint:gosec // the test's own temp file
			if readErr != nil {
				t.Fatalf("read the docker record: %v", readErr)
			}
			if !strings.Contains(string(recorded), "kernel-capabilities") {
				t.Errorf("%s never probed the kernel:\n%s", proof.name, recorded)
			}
			for line := range strings.SplitSeq(string(recorded), "\n") {
				if strings.HasPrefix(line, "run ") && !strings.Contains(line, "kernel-capabilities") {
					t.Errorf("%s started a container after the refusal: %s", proof.name, line)
				}
			}
		})
	}
}

// VALIDATES: under NO_BUILD a proof with no daemon at the build's path refuses,
// naming the path, rather than probing with nothing.
// PREVENTS: NO_BUILD turning a missing binary into a Docker bind-mount of an
// empty directory, which the probe would report as an unreadable answer.
func TestDaemonProofNoBuildNeedsTheDaemon(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(proofDocker), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the docker stub: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_RECORD", filepath.Join(bin, "record"))
	t.Setenv("NO_BUILD", "1")

	tree := t.TempDir()
	_, err := NewL2TP(tree).Run(t.Context())
	if err == nil {
		t.Fatal("l2tp proceeded under NO_BUILD with no daemon built")
	}
	if !strings.Contains(err.Error(), daemonRel(NewL2TP(tree).Goarch)) {
		t.Errorf("refusal does not name the missing daemon: %v", err)
	}
}

// proofDockerNoImage is proofDocker on a host that holds no image yet: `image
// inspect` fails, so a proof that reached ensureImage would pull.
const proofDockerNoImage = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*SecurityOptions*) echo '["name=seccomp,profile=builtin","name=cgroupns"]' ;;
*kernel-capabilities*)
  echo '{"ready": false, "capabilities": [{"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "XFRM_MSG_MIGRATE_STATE: invalid argument"}]}'
  exit 1 ;;
"image inspect"*) exit 1 ;;
esac
exit 0
`

// VALIDATES: each kernel-dependent proof that builds its own daemon checks the
// Docker kernel before it inspects or pulls its peer image, so a refused host
// downloads nothing.
// PREVENTS: an xl2tpd or other kernel-dependent peer image pull on a host the
// kernel capability check is about to refuse.
func TestDaemonProofsCheckTheKernelBeforeThePull(t *testing.T) {
	for _, proof := range daemonProofs() {
		t.Run(proof.name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(proofDockerNoImage), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
				t.Fatalf("write the docker stub: %v", err)
			}
			record := filepath.Join(bin, "record")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DOCKER_RECORD", record)
			t.Setenv("DOCKER_HOST", stubDaemon)
			t.Setenv("NO_BUILD", "1")

			tree := t.TempDir()
			daemon := filepath.Join(tree, daemonRel(proof.goarch(tree)))
			if err := os.MkdirAll(filepath.Dir(daemon), 0o750); err != nil {
				t.Fatalf("make the daemon directory: %v", err)
			}
			if err := os.WriteFile(daemon, []byte("stand-in"), 0o600); err != nil {
				t.Fatalf("write the stand-in daemon: %v", err)
			}

			if err := proof.run(tree); err == nil || !strings.Contains(err.Error(), "CONFIG_XFRM_MIGRATE") {
				t.Fatalf("%s: err = %v, want the kernel refusal", proof.name, err)
			}
			recorded, readErr := os.ReadFile(record) //nolint:gosec // the test's own temp file
			if readErr != nil {
				t.Fatalf("read the docker record: %v", readErr)
			}
			for line := range strings.SplitSeq(string(recorded), "\n") {
				if strings.HasPrefix(line, "pull ") || strings.HasPrefix(line, "image inspect ") {
					t.Errorf("%s reached its image before the kernel refusal: %s", proof.name, line)
				}
			}
		})
	}
}

// proofDockerVPPSkip fails fast on any `kernel-capabilities` probe and on
// detached container start (`run`), while answering `image inspect` with
// success so VPP proofs proceed past buildCheckedDaemon and readyImage without
// pulling or waiting for VPP sockets.
const proofDockerVPPSkip = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*SecurityOptions*) echo '["name=seccomp,profile=builtin","name=cgroupns"]' ;;
*kernel-capabilities*)
  echo '{"ready": false, "capabilities": [{"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "XFRM_MSG_MIGRATE_STATE: invalid argument"}]}'
  exit 1 ;;
"image inspect"*) exit 0 ;;
run*) exit 1 ;;
esac
exit 0
`

// TestVPPUserspaceProofsSkipKernelCapabilitiesProbe proves that the VPP
// userspace proofs (vpp, vpp-iface, vpp-srv6-service-route) skip the Docker
// host kernel-capabilities probe and proceed past buildCheckedDaemon on stock
// Linux or older ze kernels (plan/records/ruling-interop-scoped-kernel-capabilities.md).
//
// VALIDATES: vpp, vpp-iface, and vpp-srv6-service-route (prepareDaemon) never
// invoke `kernel-capabilities` and advance to readyImage (`image inspect`).
// PREVENTS: a VPP userspace proof refusing a stock Linux or older ze kernel
// because an unrelated host kernel capability (such as SRv6 or MOBIKE) is absent.
func TestVPPUserspaceProofsSkipKernelCapabilitiesProbe(t *testing.T) {
	const vppStubImage = "ligato/vpp-base:26.06-release"
	for _, proof := range vppUserspaceProofs() {
		t.Run(proof.name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(proofDockerVPPSkip), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
				t.Fatalf("write the docker stub: %v", err)
			}
			record := filepath.Join(bin, "record")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DOCKER_RECORD", record)
			t.Setenv("DOCKER_HOST", stubDaemon)
			t.Setenv("NO_BUILD", "1")
			// env.Get caches os.Environ() on first read, so update the
			// registered key through env.Set rather than t.Setenv alone.
			prevImage := env.Get(VPPImageKey)
			if err := env.Set(VPPImageKey, vppStubImage); err != nil {
				t.Fatalf("set %s: %v", VPPImageKey, err)
			}
			t.Cleanup(func() {
				if err := env.Set(VPPImageKey, prevImage); err != nil {
					t.Errorf("restore %s: %v", VPPImageKey, err)
				}
			})

			tree := t.TempDir()
			daemon := filepath.Join(tree, daemonRel(proof.goarch(tree)))
			if err := os.MkdirAll(filepath.Dir(daemon), 0o750); err != nil {
				t.Fatalf("make the daemon directory: %v", err)
			}
			if err := os.WriteFile(daemon, []byte("stand-in"), 0o600); err != nil {
				t.Fatalf("write the stand-in daemon: %v", err)
			}

			err := proof.run(tree)
			if proof.name == "vpp-srv6-service-route" {
				if err != nil {
					t.Fatalf("%s prepareDaemon failed: %v", proof.name, err)
				}
			} else if err != nil {
				if strings.Contains(err.Error(), "CONFIG_XFRM_MIGRATE") {
					t.Fatalf("%s refused on an unrelated kernel capability: %v", proof.name, err)
				}
				if strings.Contains(err.Error(), "kernel-capabilities") {
					t.Fatalf("%s invoked kernel-capabilities: %v", proof.name, err)
				}
			}

			recorded, readErr := os.ReadFile(record) //nolint:gosec // the test's own temp file
			if readErr != nil {
				t.Fatalf("read the docker record: %v", readErr)
			}
			log := string(recorded)
			if strings.Contains(log, "kernel-capabilities") {
				t.Errorf("%s invoked kernel-capabilities:\n%s", proof.name, log)
			}
			inspected := false
			for line := range strings.SplitSeq(log, "\n") {
				if strings.HasPrefix(line, "image inspect ") {
					if strings.Contains(line, vppStubImage) {
						inspected = true
						break
					}
				}
			}
			if !inspected {
				t.Errorf("%s did not proceed past buildCheckedDaemon to readyImage:\n%s", proof.name, log)
			}
		})
	}
}
