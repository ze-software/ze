package testdeployment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// proofDocker is a docker on PATH for the proofs that build their own daemon.
// It records every argv, holds every image already, answers the kernel probe
// with one absent row, and answers any other command with success.
const proofDocker = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*kernel-capabilities*)
  echo '{"ready": false, "capabilities": [{"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "XFRM_MSG_MIGRATE_STATE: invalid argument"}]}'
  exit 1 ;;
esac
exit 0
`

// VALIDATES: AC-3 through the deployment proofs that build their own daemon
// (l2tp-test, vpp-test, vpp-iface-test) and the SRv6 service-route proof. Each
// refuses a daemon kernel that lacks an enrolled feature, naming it and the
// kernel release, before any container starts.
// PREVENTS: a proof that runs Ze in Docker without the check the interop suites
// make. Removing the check from any one Run turns its subtest red.
//
// NO_BUILD=1 keeps each run from cross-compiling: the test plants a stand-in at
// the path the build would write, which is the binary the probe mounts.
func TestDaemonProofsRefuseMissingKernelFeature(t *testing.T) {
	proofs := []struct {
		name   string
		goarch func(tree string) string
		run    func(tree string) error
	}{
		{"l2tp-test", func(tree string) string { return NewL2TP(tree).Goarch }, func(tree string) error {
			_, err := NewL2TP(tree).Run()
			return err
		}},
		{"vpp-test", func(tree string) string { return newVPP(tree).Goarch }, func(tree string) error {
			_, err := newVPP(tree).Run()
			return err
		}},
		{"vpp-iface-test", func(tree string) string { return newVPPIface(tree).Goarch }, func(tree string) error {
			_, err := newVPPIface(tree).Run()
			return err
		}},
		// TestVPPSRv6ServiceRoute (build tag integration) starts its VPP container
		// and Ze through the same preparation step, so it carries the same check.
		{"vpp-srv6-service-route", func(tree string) string { return newVPP(tree).Goarch }, func(tree string) error {
			return newVPP(tree).prepareDaemon()
		}},
	}
	for _, proof := range proofs {
		t.Run(proof.name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(proofDocker), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
				t.Fatalf("write the docker stub: %v", err)
			}
			record := filepath.Join(bin, "record")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DOCKER_RECORD", record)
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
	_, err := NewL2TP(tree).Run()
	if err == nil {
		t.Fatal("l2tp-test proceeded under NO_BUILD with no daemon built")
	}
	if !strings.Contains(err.Error(), daemonRel(NewL2TP(tree).Goarch)) {
		t.Errorf("refusal does not name the missing daemon: %v", err)
	}
}
