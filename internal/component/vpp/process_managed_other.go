//go:build !linux

// Design: docs/research/vpp-deployment-reference.md -- Linux managed peer ownership
package vpp

import (
	"errors"
	"os/exec"
)

func startManagedProcess(*exec.Cmd) (*processWitness, error) {
	return nil, errors.New("vpp: managed physical process ownership requires Ze's Linux runtime kernel")
}
