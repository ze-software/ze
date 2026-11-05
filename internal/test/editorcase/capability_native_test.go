//go:build linux

// Design: docs/architecture/testing/runner-architecture.md -- native editor capability admission.
package editorcase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
)

// TestEditorSuiteNativeCapabilityTarget admits a declared net-admin capability
// in a private user namespace through the registered editor command. A FIFO
// holds the actual editor while the parent reads its suite's effective mask;
// the subsequent dirty assertion and removed work prove execution and cleanup.
func TestEditorSuiteNativeCapabilityTarget(t *testing.T) {
	admission := admitLeaseSuites(t)
	root := t.TempDir()
	fifo := filepath.Join(root, "entry.conf")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module capability-proof\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := "option=needs-linux:caps=net-admin\n" +
		"tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\n" +
		"option=file:path=test.conf\noption=timeout:value=20s\n" +
		"input=type:text=load file absolute replace " + fifo + "\ninput=enter\n" +
		"input=type:text=set bgp router-id 5.6.7.8\ninput=enter\nexpect=dirty:true\n"
	if err := os.WriteFile(filepath.Join(root, "capability.et"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	log, err := os.Create(filepath.Join(root, "suite.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	})
	cmd := exec.CommandContext(ctx, admission.executable, "test", "editor", "--dir", root, "--all", "--verbose")
	cmd.Dir = root
	cmd.Env = append(env.Without(os.Environ(), job.ParentKey, job.AttachKey),
		job.ParentVariable+"="+admission.parent, job.AttachKey+"=0",
		"TMPDIR="+root, "ze_le_store_trim=off")
	cmd.Stdout, cmd.Stderr = io.MultiWriter(log, admission.progress), log
	plugin.KillGroupOnCancel(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWUSER
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}}
	cmd.SysProcAttr.GidMappingsEnableSetgroups = false
	if err := plugin.StartChild(cmd); err != nil {
		t.Fatalf("start owned capability target: %v", err)
	}
	suite := &leaseSuite{cmd: cmd, ctx: ctx, done: make(chan error, 1), root: root, fifo: fifo, log: log.Name(), writer: -1}
	go suite.waitCapabilityTarget()
	t.Cleanup(func() {
		cancel()
		if !suite.joined {
			<-suite.done
		}
		if suite.writer >= 0 {
			if err := syscall.Close(suite.writer); err != nil {
				t.Error(err)
			}
		}
	})
	suite.enter(t)
	status, err := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/status")
	if err != nil {
		t.Fatal(err)
	}
	var effective string
	for _, line := range strings.Split(string(status), "\n") {
		if value, found := strings.CutPrefix(line, "CapEff:"); found {
			effective = strings.TrimSpace(value)
		}
	}
	mask, err := strconv.ParseUint(effective, 16, 64)
	if err != nil {
		t.Fatalf("owned target capability mask %q: %v", effective, err)
	}
	if mask&(uint64(1)<<12) == 0 {
		t.Fatalf("owned editor target lacks CAP_NET_ADMIN: %s", effective)
	}
	t.Logf("owned editor target CapEff=%s; declared net-admin admitted", effective)
	work := suite.ownedWork(t)
	suite.release(t)
	suite.join(t, false)
	if _, err := os.Stat(work); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("editor work remains after publication: %v", err)
	}
	report := suite.requireReport(t, "VERIFY STEP: ")
	passed := false
	for _, line := range strings.Split(report, "\n") {
		payload, found := strings.CutPrefix(line, "VERIFY STEP: ")
		if !found {
			continue
		}
		var step struct {
			Kind   string `json:"kind"`
			Assert string `json:"assert"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(payload), &step); err != nil {
			t.Fatal(err)
		}
		if step.Kind != "expect" {
			continue
		}
		if step.Assert != "dirty" {
			continue
		}
		if step.Status != "pass" {
			continue
		}
		passed = true
	}
	if !passed {
		t.Fatalf("capability target did not pass the authored dirty assertion:\n%s", report)
	}
}

// waitCapabilityTarget is the sole waiter for this suite's native process.
// The owner MUST receive done before removing its files.
func (suite *leaseSuite) waitCapabilityTarget() {
	suite.done <- suite.cmd.Wait()
}
