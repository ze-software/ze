// Design: docs/architecture/testing/interop.md -- sanitized non-ESN outbound sequence evidence.
// Related: socket_linux.go -- vendor readback; replay_test.go -- bounded metadata projection.
// UAPI: https://github.com/torvalds/linux/blob/v7.3/include/uapi/linux/xfrm.h
package main

import (
	"encoding/binary"
	"errors"
)

// extendedWindowSequence projects only OSeq from the fixed replay header described by
// vendor nl.XfrmReplayStateEsn. There is no vendor decoder for this slice-bearing type.
// The caller MUST reject the state's ESN flag first. This function MUST NOT read or
// reconstruct its bitmap, and refuses upper bits rather than silently discarding them.
func extendedWindowSequence(value []byte) (uint64, error) {
	if len(value) < 24 {
		return 0, errors.New("short extended-window replay header")
	}
	words := uint64(binary.NativeEndian.Uint32(value[:4]))
	if 24+words*4 != uint64(len(value)) {
		return 0, errors.New("extended-window replay bitmap length disagrees with its header")
	}
	window := uint64(binary.NativeEndian.Uint32(value[20:24]))
	if window > words*32 {
		return 0, errors.New("extended-window replay window exceeds its bitmap")
	}
	if binary.NativeEndian.Uint32(value[12:16]) != 0 {
		return 0, errors.New("non-ESN outbound sequence has unexpected upper bits")
	}
	return uint64(binary.NativeEndian.Uint32(value[4:8])), nil
}
