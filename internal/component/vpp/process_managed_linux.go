//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- atomic managed child identity
// Related: process_peer_linux.go -- the same pidfs validation for socket peers
package vpp

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

// startManagedProcess consumes the child pidfd returned atomically by clone.
// Go's os.Process duplicates an explicitly requested PidFD for its own handle;
// this witness owns the original through the complete managed run. Existing
// process-group, namespace and credential attributes remain intact. The caller
// MUST join cmd before releasing the successful witness.
func startManagedProcess(cmd *exec.Cmd) (*processWitness, error) {
	if cmd == nil {
		return nil, errors.New("vpp: managed command is nil")
	}
	if strconv.IntSize != 64 {
		return nil, errors.New("vpp: managed process identity requires a supported 64-bit Ze runtime kernel")
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.PidFD != nil {
		return nil, errors.New("vpp: managed command already has a child process descriptor owner")
	}
	descriptor := -1
	cmd.SysProcAttr.PidFD = &descriptor
	defer func() { cmd.SysProcAttr.PidFD = nil }()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if descriptor < 0 {
		return nil, reapManagedStart(cmd, errors.New("vpp: atomic child process descriptors are unavailable; run Ze's runtime kernel"))
	}
	witness, err := processWitnessForPIDFD(descriptor)
	if err != nil {
		return nil, reapManagedStart(cmd, err)
	}
	return witness, nil
}

// Every failure after Start owns the child and joins it before returning.
func reapManagedStart(cmd *exec.Cmd, failure error) error {
	return errors.Join(failure, stopVPPChild(context.Background(), cmd))
}
