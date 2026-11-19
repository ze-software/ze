//go:build !linux

// Design: docs/research/vpp-deployment-reference.md -- Linux physical peer requirement
package vpp

import (
	"errors"
	"net"
)

func captureProcessWitness(*net.UnixConn) (*processWitness, error) {
	return nil, errors.New("vpp: physical peer identity requires Linux SO_PEERPIDFD and pidfs; run VPP with Ze's runtime kernel")
}

func probePhysicalProcessCapability() error {
	return errors.New("VPP physical process ownership requires Linux SO_PEERPIDFD and pidfs")
}
