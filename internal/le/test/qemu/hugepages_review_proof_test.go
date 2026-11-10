//go:build linux || darwin

// Design: docs/architecture/testing/qemu-integration.md -- hugepages invocation ownership.
package testqemu

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// TestHugepagesNormalCompletionCooperativelyStopsVM reaches both actual verb
// assertions, then requires its QEMU command to receive SIGINT and join its own
// child. A KILL-only teardown cannot create the interrupted receipt. Semantic
// process fixtures do not establish actual image, kernel or hardware boot proof.
func TestHugepagesNormalCompletionCooperativelyStopsVM(t *testing.T) {
	work, result := hugepagesReviewInvocation(t, "normal")
	if result.Code != 0 {
		t.Fatalf("normal invocation failed: %+v", result)
	}
	for _, name := range []string{"vm.interrupted", "vm.stopped", "vm-descendant.stopped"} {
		if _, err := os.Stat(filepath.Join(work, name)); err != nil {
			t.Errorf("normal completion did not cooperatively join QEMU: %s: %v", name, err)
		}
	}
	hugepagesRootStopped(t, work, "vm")
	hugepagesRootStopped(t, work, "vm-descendant")
}

// TestHugepagesCancellationReturnsNoPayload inspects the value returned by the
// registered action, even on a nonzero status. Unlike a helper that suppresses
// encoding on failure, this proves the dispatcher gives a real renderer no
// report from which it could publish a reservation verdict after cancellation.
func TestHugepagesCancellationReturnsNoPayload(t *testing.T) {
	for _, phase := range []string{"host-build", "init", "image-build", "kernel", "memory"} {
		t.Run(phase, func(t *testing.T) {
			_, result := hugepagesReviewInvocation(t, phase)
			if result.Code == 0 {
				t.Fatal("canceled invocation answered success")
			}
			if !bytes.Equal(result.Payload, []byte("null")) {
				t.Fatalf("canceled registered action retained a renderable payload: %s", result.Payload)
			}
		})
	}
}

type hugepagesReviewResult struct {
	Code    int             `json:"code"`
	Payload json.RawMessage `json:"payload"`
}

// hugepagesReviewInvocation calls the actual registered verb in a subprocess.
// Its result envelope records payloads regardless of the action's status.
func hugepagesReviewInvocation(t *testing.T, phase string) (string, hugepagesReviewResult) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	bin := filepath.Join(work, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go", qemuBinary(hostArch()), "sshpass", "mkfs.ext4", "debugfs"} {
		test := "TestHugepagesRootProcess"
		role := name
		if name == qemuBinary(hostArch()) {
			test = "TestHugepagesReviewProcess"
			role = "review-vm"
		}
		body := "#!/bin/sh\nexport HP_ROOT_ROLE=" + role + "\nexec \"$HP_ROOT_SELF\" -test.run='^" + test + "$' -- \"$@\"\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestHugepagesReviewProcess$")
	cmd.Env = append(os.Environ(), "HP_ROOT_ROLE=review-verb", "HP_ROOT_SELF="+self,
		"HP_ROOT_WORK="+work, "HP_ROOT_BLOCK="+phase, "ZE_REPO_ROOT="+work,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DOCKER_HOST=unix:///nonexistent/hugepages-proof.sock")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	plugin.KillGroupOnCancel(cmd)
	if err := plugin.StartChild(cmd); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("review verb fixture could not be joined")
		}
	}()
	if phase != "normal" {
		hugepagesRootAwait(t, filepath.Join(work, phase+".ready"))
		if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-done:
		done <- err
		if err != nil {
			t.Fatalf("result carrier failed: %v\n%s", err, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("registered action did not finish bounded teardown")
	}
	var result hugepagesReviewResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("read actual action result: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return work, result
}

// TestHugepagesReviewProcess is active only for these private subprocess roles.
// The VM fixture, like QEMU, owns and cooperatively joins a child on SIGINT.
func TestHugepagesReviewProcess(t *testing.T) {
	role := os.Getenv("HP_ROOT_ROLE")
	if role == "review-verb" {
		settings := map[string]string{
			"ze.vpp.hp.arch": hostArch(), "ze.vpp.hp.keep": "0",
			"ze.vpp.hp.aarch64.bios": os.Getenv("HP_ROOT_SELF"),
			"ze.vpp.hp.pagesize":     DefaultPageSize, "ze.vpp.hp.size": DefaultReservation,
			"ze.vpp.hp.memory": DefaultMemory, "ze.vpp.hp.ssh.port": "0",
		}
		for key, value := range settings {
			if err := os.Setenv(key, value); err != nil {
				t.Fatal(err)
			}
		}
		env.ResetCache()
		payload, code := Answer([]string{"vpp-hugepages"})
		if err := json.NewEncoder(os.Stdout).Encode(struct {
			Code    int `json:"code"`
			Payload any `json:"payload"`
		}{code, payload}); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	if role != "review-vm" {
		return
	}
	// The handler is never stopped: it stays installed until os.Exit, so a
	// second signal cannot replace the fixture's exit 0 with a signal death.
	caught := make(chan os.Signal, 1)
	signal.Notify(caught, syscall.SIGTERM, os.Interrupt)
	hugepagesRootSocket(t, "vm")
	child := exec.CommandContext(t.Context(), os.Getenv("HP_ROOT_SELF"), "-test.run=^TestHugepagesRootProcess$")
	child.Env = append(os.Environ(), "HP_ROOT_ROLE=descendant", "HP_ROOT_PHASE=vm")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	hugepagesRootAwait(t, filepath.Join(os.Getenv("HP_ROOT_WORK"), "vm-descendant.ready"))
	hugepagesRootMark(t, "vm.ready")
	select {
	case stopped := <-caught:
		if stopped != os.Interrupt {
			t.Fatalf("normal completion delivered %v, want SIGINT", stopped)
		}
		hugepagesRootMark(t, "vm.interrupted")
		if err := child.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
		hugepagesRootMark(t, "vm.stopped")
		os.Exit(0)
	case <-time.After(25 * time.Second):
		t.Fatal("VM fixture never received normal shutdown")
	}
}
