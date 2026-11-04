//go:build linux || darwin

package testqemu

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestQEMURunRootCancelsActualAdmission occupies the only real registry slot,
// waits for the actual verb's queue banner, and sends SIGHUP. The rooted verb
// must leave admission with status 129 while retaining the live holder's exact
// entry. Rooting only after admission or calling noncancelable Admit hangs.
func TestQEMURunRootCancelsActualAdmission(t *testing.T) {
	checkout, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := stageDockerLabTree(t)
	admission, err := job.NewIn(tree)
	if err != nil {
		t.Fatal(err)
	}
	admission.Slots = 1
	ticket, err := admission.Admit("foreign-holder", []string{"held-by-test"})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Kind != job.KindClaimed {
		t.Fatalf("holder did not claim the slot: %s", ticket.Kind)
	}
	defer ticket.Release(0)
	entry := filepath.Join(tree, ticket.Entry)
	before, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestQEMURunRootProcess$")
	cmd.Env = append(env.Without(os.Environ(), job.ParentKey, job.SlotsKey, "ze.repo.root"),
		"QEMU_ROOT_PROOF_ROLE=verb", "QEMU_ROOT_PROOF_BIN="+t.TempDir(),
		"QEMU_ROOT_PROOF_CHECKOUT="+checkout, "ZE_REPO_ROOT="+tree, "ZE_RUN_SLOTS=1")
	notes := &qemuQueueObservation{waiting: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = notes, notes
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
			t.Error("queued verb could not be joined")
		}
	}()
	select {
	case <-notes.waiting:
	case <-time.After(20 * time.Second):
		cancel()
		select {
		case exited := <-done:
			done <- exited
			t.Fatalf("verb never reached actual admission; helper exit: %v\n%s", exited, notes.String())
		case <-time.After(10 * time.Second):
			t.Fatal("verb never reached actual admission and could not be joined")
		}
	}
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		done <- err
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 129 {
			t.Fatalf("queued verb did not return rooted SIGHUP status: %v\n%s", err, notes.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("actual admission ignored the verb's cancellation root: %s", notes.String())
	}
	after, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("canceled waiter changed the live holder:\nbefore %s\nafter %s", before, after)
	}
}

// qemuQueueObservation observes the queue's actual emitted banner, not elapsed
// sleeps or PID polling. Safe for concurrent command writes and diagnostics.
type qemuQueueObservation struct {
	mu      sync.Mutex
	body    strings.Builder
	waiting chan struct{}
	once    sync.Once
}

func (o *qemuQueueObservation) Write(body []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	written, err := o.body.Write(body)
	if strings.Contains(o.body.String(), "["+runJobLabel+"] waiting:") {
		o.once.Do(func() { close(o.waiting) })
	}
	return written, err
}

func (o *qemuQueueObservation) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.body.String()
}
