// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_fastpath_linux.go -- loadESPFormFastPathCollection, the load this probe shares with start
// Related: espform_fastpath_probe_other.go -- the probe off Linux

//go:build linux

package dataplane

import (
	"errors"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

// ESPFormFastPathReady answers whether the fast path's program would load in this
// process now, before any Child SA needs it: the fast path loads only at the first
// watched SPI (espFormFastPath.start), so until then nothing else says whether it can.
// It runs the load start runs, loadESPFormFastPathCollection, and closes what it
// loaded, so it proves the same things start needs from the kernel: the capabilities,
// the verifier's verdict on both programs, the maps, and the TCX attach.
//
// It changes nothing a packet meets. The programs and maps it creates belong to file
// descriptors it closes before it returns, so the kernel frees them, and the TCX probe
// attaches to an interface index no device holds (probeESPFormTCX), so nothing is
// attached. The one lasting change is the memlock limit rlimit.RemoveMemlock raises,
// which start raises the same way. It answers nil when the load succeeded, and
// otherwise the error with its fault (espFormFastPathFaultOf). Safe for concurrent use.
func ESPFormFastPathReady() (ESPFormFastPathFault, error) {
	coll, err := loadESPFormFastPathCollection()
	if err != nil {
		return espFormFastPathFaultOf(err), err
	}
	coll.Close()
	return ESPFormFastPathFaultUnspecified, nil
}

// espFormFastPathFaultOf classifies a load failure. EPERM is the kernel refusing a
// process without CAP_BPF or CAP_NET_ADMIN (ESPFormFastPathFaultCapability), and
// ebpf.ErrNotSupported a kernel without TCX (probeESPFormTCX). Any other answer, the
// verifier's EACCES or EINVAL among them, says nothing a capability or a kernel upgrade
// is known to change: Unknown.
func espFormFastPathFaultOf(err error) ESPFormFastPathFault {
	if errors.Is(err, unix.EPERM) {
		return ESPFormFastPathFaultCapability
	}
	if errors.Is(err, ebpf.ErrNotSupported) {
		return ESPFormFastPathFaultKernel
	}
	return ESPFormFastPathFaultUnknown
}
