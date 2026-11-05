//go:build linux || darwin

// Design: docs/architecture/testing/qemu-integration.md -- owned Docker-lab build admission.
package testqemu

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestDockerLabBuildCancellationLeavesOccupiedAdmission reaches BuildLinuxZe's
// actual occupied-slot wait before canceling its root. The compiler must never
// start, the waiter must join, and the unrelated live holder must remain intact.
func TestDockerLabBuildCancellationLeavesOccupiedAdmission(t *testing.T) {
	checkout, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	tree, compilerRecord := stageDockerLabTree(t)
	// Toolchain metadata queries remain real; only a compiler invocation is
	// recorded, so preparation cannot masquerade as an admitted build.
	compiler := "#!/bin/sh\nif [ \"$1\" = build ]; then\n  echo \"$*\" >> \"$GO_RECORD\"\n  exit 0\nfi\nexec \"$DOCKER_WAIT_GO\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(compilerRecord), "go"), []byte(compiler), 0o700); err != nil {
		t.Fatal(err)
	}
	admission, err := job.NewIn(tree)
	if err != nil {
		t.Fatal(err)
	}
	admission.Slots = 1
	ticket, err := admission.AdmitContext(t.Context(), "foreign-holder", []string{"held-by-test"})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Kind != job.KindClaimed {
		t.Fatalf("foreign holder did not claim a slot: %s", ticket.Kind)
	}
	defer ticket.Release(0)
	entry := filepath.Join(tree, ticket.Entry)
	before, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := os.Stat(entry)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command := exec.CommandContext(ctx, self, "-test.run=^TestDockerLabAdmissionWaitProcess$")
	command.Env = append(env.Without(os.Environ(), job.ParentKey, job.SlotsKey, "ze.repo.root"),
		"DOCKER_WAIT_CHECKOUT="+checkout, "DOCKER_WAIT_TREE="+tree,
		"DOCKER_WAIT_BIN="+filepath.Dir(compilerRecord), "ZE_RUN_SLOTS=1")
	notes := &dockerLabQueueObservation{qemuQueueObservation: qemuQueueObservation{waiting: make(chan struct{})}}
	command.Stdout, command.Stderr = notes, notes
	plugin.KillGroupOnCancel(command)
	if err := plugin.StartChild(command); err != nil {
		t.Fatal(err)
	}
	child := dockerLabAdmissionChild{command: command, done: make(chan struct{})}
	go child.wait()
	// MUST join the helper before the holder ticket and fixture are released.
	defer func() {
		cancel()
		select {
		case <-child.done:
		case <-time.After(10 * time.Second):
			t.Error("queued Docker-lab helper could not be joined")
		}
	}()
	select {
	case <-notes.waiting:
	case <-child.done:
		t.Fatalf("build ended before occupied admission: %v\n%s", child.err, notes.String())
	case <-time.After(20 * time.Second):
		t.Fatalf("build never reached occupied admission: %s", notes.String())
	}
	if err := command.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-child.done:
		if child.err != nil {
			t.Fatalf("helper did not observe rooted cancellation: %v\n%s", child.err, notes.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("Docker-lab admission ignored cancellation: %s", notes.String())
	}
	if _, err := os.Stat(compilerRecord); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("compiler ran or its absence is unknown: %v", err)
	}
	after, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("canceled waiter changed the foreign holder's entry bytes")
	}
	afterIdentity, err := os.Stat(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(identity, afterIdentity) {
		t.Error("canceled waiter replaced the foreign holder's entry")
	}
	entries, err := filepath.Glob(filepath.Join(tree, job.JobsDir, "*.job"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("waiter left admission claims behind: %v", entries)
	}
	if entries[0] != entry {
		t.Fatalf("remaining admission claim is not the foreign holder: %v", entries)
	}
}

type dockerLabAdmissionChild struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
}

// wait is the helper's sole reaper; closing done publishes its result.
func (c *dockerLabAdmissionChild) wait() {
	c.err = c.command.Wait()
	close(c.done)
}

type dockerLabQueueObservation struct {
	qemuQueueObservation
}

func (o *dockerLabQueueObservation) Write(body []byte) (int, error) {
	written, err := o.qemuQueueObservation.Write(body)
	if strings.Contains(o.String(), "[qemu-build-ze] waiting:") {
		o.once.Do(func() { close(o.waiting) })
	}
	return written, err
}

// TestDockerLabAdmissionWaitProcess is the isolated BuildLinuxZe caller. It
// returns test success only after the production entry reports canceled work.
func TestDockerLabAdmissionWaitProcess(t *testing.T) {
	checkout := os.Getenv("DOCKER_WAIT_CHECKOUT")
	if checkout == "" {
		return
	}
	t.Setenv("PATH", filepath.Join(os.Getenv("GOROOT"), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_REPO_ROOT", checkout)
	env.ResetCache()
	if _, err := gotoolchain.New(checkout); err != nil {
		t.Fatal(err)
	}
	compiler, err := filepath.EvalSymlinks(filepath.Join(os.Getenv("GOROOT"), "bin", "go"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_WAIT_GO", compiler)
	tree := os.Getenv("DOCKER_WAIT_TREE")
	t.Setenv("ZE_REPO_ROOT", tree)
	t.Setenv("PATH", os.Getenv("DOCKER_WAIT_BIN")+string(os.PathListSeparator)+os.Getenv("PATH"))
	env.ResetCache()
	ctx, stop := signal.NotifyContext(t.Context(), syscall.SIGHUP)
	defer stop()
	output, err := BuildLinuxZe(ctx, tree, GuestArch())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("queued build did not preserve cancellation: output=%q err=%v", output, err)
	}
	if output != "" {
		t.Fatalf("canceled build returned a successful output path: %s", output)
	}
}
