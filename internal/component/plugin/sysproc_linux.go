// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started
// Related: sysproc.go -- KillGroupOnCancel, the one caller, and the stop these attributes serve

//go:build linux

package plugin

import "syscall"

// NewSysProcAttr answers the attributes every forked plugin is started with,
// the live start and the declaration query alike. Its one caller is
// KillGroupOnCancel (sysproc.go), which both forks call, so a plugin started
// two ways is stopped one way: these attributes and the kill that reads them
// cannot be taken separately.
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
func NewSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
}
