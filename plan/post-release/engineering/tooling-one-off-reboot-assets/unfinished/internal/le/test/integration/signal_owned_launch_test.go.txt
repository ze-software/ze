// Design: docs/architecture/testing/interop.md -- inherited child run ownership.
// Related: signal_ownership_test.go -- fake daemon helper.

package testintegration

import (
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestSignalSweepRemovesItsLaunchedChildsResources supplies the launcher's
// nonce with a different process's PID/start labels to the actual cleaner.
// Its positive complements the frozen foreign-resource survival regression:
// making every automatic sweep dry cannot satisfy both tests.
func TestSignalSweepRemovesItsLaunchedChildsResources(t *testing.T) {
	runner := &signalOwnershipRunner{}
	docker := interoplab.NewFixtureDocker(runner)
	if _, err := docker.CreateNetwork(t.Context(), interoplab.NetworkSpec{
		Name: "label-source", Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
	}); err != nil {
		t.Fatalf("obtain fixture owner's labels: %v", err)
	}
	runner.labels["ze.lab.launch"] = strings.TrimSuffix(runner.labels["ze.lab.run"], "-other-run")
	runner.labels["ze.lab.pid"] = "4343"
	runner.labels["ze.lab.pid-start"] = "200"
	text := sweepAfterSignal(t.Context(), dockerPeersVerb, docker, gaterun.ProcessGroup{}, groupExitTimeout)
	if !strings.Contains(text, "1 removed") {
		t.Errorf("owned-child cleanup report = %q, want one removed resource", text)
	}
	removed := 0
	for _, command := range runner.commands {
		if strings.HasPrefix(command, "docker network rm ") {
			removed++
		}
	}
	if removed != 1 {
		t.Errorf("owned-child network removals = %d, want 1", removed)
	}
}

// TestSignalSweepPreservesResourcesWithoutLaunch checks resources from older
// or directly invoked foreign processes that have no launch provenance.
func TestSignalSweepPreservesResourcesWithoutLaunch(t *testing.T) {
	runner := &signalOwnershipRunner{}
	docker := interoplab.NewFixtureDocker(runner)
	if _, err := docker.CreateNetwork(t.Context(), interoplab.NetworkSpec{
		Name: "label-source", Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
	}); err != nil {
		t.Fatalf("obtain fixture owner's labels: %v", err)
	}
	delete(runner.labels, "ze.lab.launch")
	text := sweepAfterSignal(t.Context(), dockerPeersVerb, docker, gaterun.ProcessGroup{}, groupExitTimeout)
	if !strings.Contains(text, "other-run-network") {
		t.Errorf("automatic cleanup never reported the retained resource without launch provenance: %q", text)
	}
	for _, command := range runner.commands {
		if strings.HasPrefix(command, "docker network rm ") {
			t.Errorf("automatic cleanup removed a resource without launch provenance: %s", command)
		}
	}
}

// TestOwnedRunIdentityChild is a subprocess helper. The fake docker executable
// records this real child's actual create argv without contacting a daemon.
func TestOwnedRunIdentityChild(t *testing.T) {
	if os.Getenv("TEST_INTEROP_OWNERSHIP_CHILD") == "" {
		t.Skip("subprocess helper for TestDockerChildInheritsOwnedRunIdentity")
	}
	recorder := os.Getenv("TEST_INTEROP_OWNERSHIP_DOCKER")
	if recorder == "" {
		t.Fatal("the owned subprocess has no inert Docker recorder path")
	}
	// Toolchain activation can prepend its directory to the inherited PATH.
	// Resolve only this test's recorder before the real wrapper issues a command.
	t.Setenv("PATH", filepath.Dir(recorder)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_CONTEXT", "")
	resolved, err := exec.LookPath(interoplab.DockerExecutable)
	if err != nil {
		t.Fatalf("resolve the inert Docker recorder: %v", err)
	}
	if resolved != recorder {
		t.Fatalf("Docker resolved to %q instead of this test's inert recorder %q", resolved, recorder)
	}
	docker := interoplab.NewDocker()
	if _, err := docker.CreateNetwork(t.Context(), interoplab.NetworkSpec{
		Name: "child-probe", Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
	}); err != nil {
		t.Fatalf("record the child's labeled create: %v", err)
	}
}

// TestDockerChildInheritsOwnedRunIdentity drives the actual launch boundary,
// then records the real child's owner labels through an inert docker script.
// The script only writes this test's own argv file. A fresh child's unrelated
// random nonce fails the assertion, even if a cleanup-only unit passes.
func TestDockerChildInheritsOwnedRunIdentity(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	directory := t.TempDir()
	capture := filepath.Join(directory, "create-argv")
	program := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$TEST_INTEROP_OWNERSHIP_CAPTURE\"\n"
	recorder := filepath.Join(directory, "docker")
	if err := os.WriteFile(recorder, []byte(program), 0o700); err != nil {
		t.Fatalf("write inert docker recorder: %v", err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_HOST", "unix:///test-owned-fake-docker.sock")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("TEST_INTEROP_OWNERSHIP_CAPTURE", capture)
	t.Setenv("TEST_INTEROP_OWNERSHIP_CHILD", "1")
	t.Setenv("TEST_INTEROP_OWNERSHIP_DOCKER", recorder)
	// A dotted inherited alias must not override this invocation's identity.
	t.Setenv("ze.lab.launch", "unrelated-inherited-run")
	runner := &signalOwnershipRunner{}
	docker := interoplab.NewFixtureDocker(runner)
	if _, err := docker.CreateNetwork(t.Context(), interoplab.NetworkSpec{
		Name: "label-source", Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
	}); err != nil {
		t.Fatalf("obtain fixture owner's labels: %v", err)
	}
	want := strings.TrimSuffix(runner.labels["ze.lab.run"], "-other-run")
	argv := []string{executable, "-test.run", "^TestOwnedRunIdentityChild$"}
	if _, code := runRequiringDocker(t.Context(), dockerPeersVerb, fixedArgv(argv), root, docker); code != 0 {
		t.Fatalf("actual child launch code = %d, want 0", code)
	}
	created, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("read actual child create argv: %v", err)
	}
	if !strings.Contains(string(created), "ze.lab.launch="+want+"\n") {
		t.Errorf("child's owner labels do not carry launcher's nonce %q:\n%s", want, created)
	}
	if strings.Contains(string(created), "ze.lab.run="+want+"\n") {
		t.Errorf("child lost its separate per-process run identity:\n%s", created)
	}
}
