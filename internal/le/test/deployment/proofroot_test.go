// Related: daemonkernel_test.go -- the docker stub and daemon stand-in these tests reuse.

package testdeployment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refusingDocker is a docker on PATH whose kernel probe passes, that holds
// every image, and that refuses every detached start after creating the
// container: the vhost-net refusal vpp-test met on 2026-10-19, which left
// `ze-vpp-evidence-2578373` in Created state. A removal prints the name, as
// docker does for a container it removed.
const refusingDocker = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*SecurityOptions*) echo '["name=seccomp,profile=builtin","name=cgroupns"]' ;;
*kernel-capabilities*) echo '{"ready": true, "capabilities": [{"subsystem": "l2tp", "kernel": "CONFIG_L2TP", "state": "present"}]}' ;;
"run -d"*|"run --detach"*)
  echo 'docker: Error response from daemon: error gathering device information while adding custom device "/dev/vhost-net": no such file or directory' >&2
  exit 125 ;;
"rm -f "*) for last; do :; done; echo "$last" ;;
esac
exit 0
`

// stageProofDocker puts script on PATH as docker, plants the stand-in daemon
// NO_BUILD reuses for goarch, and answers the tree and the docker record.
func stageProofDocker(t *testing.T, script, goarch, tree string) string {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the docker stub: %v", err)
	}
	record := filepath.Join(bin, "record")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_RECORD", record)
	t.Setenv("NO_BUILD", "1")

	daemon := filepath.Join(tree, daemonRel(goarch))
	if err := os.MkdirAll(filepath.Dir(daemon), 0o750); err != nil {
		t.Fatalf("make the daemon directory: %v", err)
	}
	if err := os.WriteFile(daemon, []byte("stand-in"), 0o600); err != nil {
		t.Fatalf("write the stand-in daemon: %v", err)
	}
	return record
}

// rootedProof is one signal-rooted deployment proof and the run it performs
// under a context.
type rootedProof struct {
	name   string
	goarch func(tree string) string
	run    func(ctx context.Context, tree string) error
}

// rootedProofs answers the container proofs a deployment verb roots on
// signals (signalRooted in actions.go).
func rootedProofs() []rootedProof {
	return []rootedProof{
		{"l2tp-test", func(tree string) string { return NewL2TP(tree).Goarch }, func(ctx context.Context, tree string) error {
			_, err := NewL2TP(tree).Run(ctx)
			return err
		}},
		{"vpp-test", func(tree string) string { return newVPP(tree).Goarch }, func(ctx context.Context, tree string) error {
			_, err := newVPP(tree).Run(ctx)
			return err
		}},
		{"vpp-iface-test", func(tree string) string { return newVPPIface(tree).Goarch }, func(ctx context.Context, tree string) error {
			_, err := newVPPIface(tree).Run(ctx)
			return err
		}},
	}
}

// VALIDATES: the kernel probe of each signal-rooted proof runs under the
// verb's root, so the root ending (a Ctrl-C) ends the probe before it starts.
// METHOD: each proof runs under a canceled root with a docker stub that
// records every command it is handed. A probe rooted on context.Background
// runs the stub; one under the root never starts it.
// PREVENTS: a Ctrl-C the docker client never sees, because it heads a
// process group of its own, leaving the probe to run to its bound.
func TestRootedProofsProbeTheKernelUnderTheVerbRoot(t *testing.T) {
	for _, proof := range rootedProofs() {
		t.Run(proof.name, func(t *testing.T) {
			tree := t.TempDir()
			record := stageProofDocker(t, proofDocker, proof.goarch(tree), tree)
			canceled, cancel := context.WithCancel(t.Context())
			cancel()

			if err := proof.run(canceled, tree); err == nil {
				t.Fatalf("%s under a canceled root answered no error", proof.name)
			}
			if recorded, readErr := os.ReadFile(record); readErr == nil { //nolint:gosec // the test's own temp file
				t.Fatalf("%s ran docker under a canceled root:\n%s", proof.name, recorded)
			}
		})
	}
}

// VALIDATES: G-2 through each proof's own Run: a detached start docker
// issued and refused, after the daemon created the container, is still
// followed by the proof's removal of that container.
// METHOD: the stub passes the kernel probe and refuses `docker run -d` with
// exit 125; the record must show an `rm -f` of the started name after it.
// PREVENTS: a proof that defers its removal only after a successful start
// (the `if err != nil { return }` above the defer), which
// TestFailedProofStartStillRemovesItsContainer, driving startProofContainer
// alone, cannot see.
func TestRefusedProofStartRemovesItsContainer(t *testing.T) {
	for _, proof := range rootedProofs() {
		t.Run(proof.name, func(t *testing.T) {
			tree := t.TempDir()
			record := stageProofDocker(t, refusingDocker, proof.goarch(tree), tree)

			runErr := proof.run(t.Context(), tree)
			if runErr == nil {
				t.Fatalf("%s answered no error for a refused start", proof.name)
			}
			recorded, err := os.ReadFile(record) //nolint:gosec // the test's own temp file
			if err != nil {
				t.Fatalf("read the docker record: %v", err)
			}
			started := ""
			removed := false
			for line := range strings.SplitSeq(string(recorded), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				if fields[0] == "run" && (fields[1] == "-d" || fields[1] == "--detach") {
					started = line
					continue
				}
				if started != "" && fields[0] == "rm" && strings.Contains(started, fields[len(fields)-1]) {
					removed = true
				}
			}
			if started == "" {
				t.Fatalf("%s never issued a detached start (%v):\n%s", proof.name, runErr, recorded)
			}
			if !removed {
				t.Fatalf("%s left the container of its refused start in place:\n%s", proof.name, recorded)
			}
		})
	}
}
