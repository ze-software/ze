// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started
// Related: sysproc.go -- KillGroupOnCancel, the query's caller, and the stop these attributes serve
// Related: process/exitwait_linux.go -- newPluginShell, the live start's caller
// Related: process/process.go -- startExternal, which starts the live plugin through StartChild

//go:build linux

package plugin

import (
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

// NewSysProcAttr answers the attributes every forked plugin is started with,
// the live start and the declaration query alike. The query takes them through
// KillGroupOnCancel (sysproc.go) and the live start through newPluginShell
// (process/exitwait_linux.go), and each caller pairs them with the kill that
// reads them: either half alone stops the shell and leaves the plugin.
//
// Setpgid puts the plugin and everything it starts into one process group, so
// a stop can reach the whole tree. A run string is given to a shell, and a
// shell that does not exec-optimize the string stays alive with the plugin as
// its own child, so a kill aimed at the direct child alone leaves the plugin
// running. Killing the group is the other half of it (KillProcessGroup).
//
// Pdeathsig SIGKILL makes the kernel signal the child when ze dies, however ze
// dies: a crash, a SIGKILL out of turn, or a kill from outside. Without it a
// crashed ze leaves long-running helpers reparented to init, where they hold
// inherited resources such as lock descriptors and Unix sockets for hours
// before an operator reaps them by hand. Pdeathsig is a Linux-only field, so
// sysproc_other.go carries the same function for every other platform.
//
// Pdeathsig binds the direct child, which is the shell. It reaches the plugin
// for a run string the shell exec-optimizes, because the plugin IS that child
// then. A run string the shell keeps a process for leaves the plugin a
// grandchild, and a crashed ze kills the shell and orphans the plugin: the
// kernel offers no death signal for a grandchild. A daemon that stops the
// plugin itself is covered whatever the run string is, because that stop
// signals the group.
//
// The kernel's word "parent" here is the THREAD that forked, not ze: the death
// signal is sent when that thread exits. A fork with these attributes MUST go
// through StartChild, which forks from a thread that lives as long as ze.
func NewSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
}

// forkRequest is one start the spawner thread performs.
type forkRequest struct {
	cmd    *exec.Cmd
	answer chan<- error
}

var (
	spawnerOnce     sync.Once
	spawnerRequests chan forkRequest
)

// StartChild starts cmd from ze's spawner thread and returns cmd.Start's
// answer. Every fork that carries NewSysProcAttr MUST start through here.
//
// Pdeathsig is bound to the forking thread, and the Go runtime destroys a
// thread whose goroutine returns while it holds runtime.LockOSThread. Ze does
// that on purpose: a capability probe runs on a locked thread it has moved
// into a throwaway network namespace, and lets the runtime destroy it
// (kernelcap.InThrowawayNetworkNamespace), and the start and reload gates run
// such probes. The runtime picks that thread from its pool, so a plugin forked
// on a plain goroutine earlier could be SIGKILLed with no log line the next
// time a probe ran on its thread. The spawner holds its thread locked for the
// life of ze, so no other goroutine ever runs on it and nothing destroys it.
//
// Safe for concurrent use: starts are serialized on the one thread, and each
// caller waits only for its own fork, not for the child.
func StartChild(cmd *exec.Cmd) error {
	spawnerOnce.Do(startSpawner)
	answer := make(chan error, 1)
	spawnerRequests <- forkRequest{cmd: cmd, answer: answer}
	return <-answer
}

// startSpawner creates the request channel and the worker that serves it. It
// runs once, on the first StartChild.
func startSpawner() {
	spawnerRequests = make(chan forkRequest)
	go runSpawner(spawnerRequests)
}

// runSpawner is the spawner worker. It locks its OS thread and never unlocks
// it, so the thread every plugin was forked from serves no other goroutine.
// The channel is never closed: the loop ends only with ze, and that end is
// the death the plugins' Pdeathsig is meant for.
func runSpawner(requests <-chan forkRequest) {
	runtime.LockOSThread()
	for request := range requests {
		request.answer <- request.cmd.Start()
	}
}
