//go:build linux

// Design: docs/architecture/testing/runner-architecture.md -- caller attributes and child ownership.

package runner

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestServerProcessPreservesCallerCredential(t *testing.T) {
	credential := &syscall.Credential{Uid: uint32(os.Getuid()), Gid: uint32(os.Getgid()), NoSetGroups: true}
	requested := &syscall.SysProcAttr{Credential: credential}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	command.SysProcAttr = requested
	child, err := StartServerProcess(command)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := child.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if command.SysProcAttr.Credential != credential {
		t.Error("owned child lost the caller's requested credential")
	}
	canonical := plugin.NewSysProcAttr()
	if command.SysProcAttr.Setpgid != canonical.Setpgid {
		t.Error("owned child lost its canonical process group")
	}
	if command.SysProcAttr.Pdeathsig != canonical.Pdeathsig {
		t.Error("owned child lost its canonical parent-death signal")
	}
	if requested.Setpgid {
		t.Error("child startup mutated the caller's requested attributes")
	}
	if requested.Pdeathsig != 0 {
		t.Error("child startup mutated the caller's requested parent-death signal")
	}
}
