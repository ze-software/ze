//go:build linux

// Design: docs/architecture/appliance/kernel-profiles.md -- PPP original-resource ABI.
// Related: session_pair_linux.go -- short leases on the two original files.

package kernel

import (
	"errors"
	"math"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Root0010 is the sole ABI. Query51 never connects; every peer operation runs
// on the original channel file with a fixed-width original peer-file request.
const (
	pppPairOwnerABI       = 1
	pppPairQueryUnit      = 0xc0147433
	pppPairConnect        = 0x40107432
	pppPairDisconnect     = 0x40107431
	pppPairBridge         = 0x40107430
	pppPairUnbridge       = 0x4010742f
	pppPairQueryChannel   = 0xc010742e
	pppPairConnectFeature = 1 << 6
	pppPairBridgeFeature  = 1 << 7
)

// Raw syscall DTOs, all fields 32 bits. Their zero value is not a request;
// each call builds ABI1 with zero flags/features/reserved and checked labels.
type pppPairPeer struct {
	abi      uint32
	fd       int32
	flags    uint32
	reserved uint32
}

type pppPairUnitOwner struct {
	abi      uint32
	features uint32
	ifindex  int32
	unit     int32
	reserved uint32
}

type pppPairChannelOwner struct {
	abi      uint32
	features uint32
	channel  int32
	reserved uint32
}

// Both directions make any layout drift a compile error on the target.
var (
	_ [16 - int(unsafe.Sizeof(pppPairPeer{}))]byte
	_ [int(unsafe.Sizeof(pppPairPeer{})) - 16]byte
	_ [20 - int(unsafe.Sizeof(pppPairUnitOwner{}))]byte
	_ [int(unsafe.Sizeof(pppPairUnitOwner{})) - 20]byte
	_ [16 - int(unsafe.Sizeof(pppPairChannelOwner{}))]byte
	_ [int(unsafe.Sizeof(pppPairChannelOwner{})) - 16]byte
)

// pppPairPeerOperation MUST run under both original file leases. Zero is a
// valid fd; neither a reusable unit number nor a reconstructed file enters.
func pppPairPeerOperation(channelFD int, command uintptr, peerFD int) error {
	if peerFD < 0 {
		return unix.EBADF
	}
	if peerFD > math.MaxInt32 {
		return unix.EBADF
	}
	peer := pppPairPeer{abi: pppPairOwnerABI, fd: int32(peerFD)}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(channelFD), command,
		uintptr(unsafe.Pointer(&peer))) //nolint:gosec // Root0010 fixed 16-byte peer ABI.
	if errno != 0 {
		return errno
	}
	return nil
}

func pppPairReadUnitOwner(fd int, owner *pppPairUnitOwner) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), pppPairQueryUnit,
		uintptr(unsafe.Pointer(owner))) //nolint:gosec // Root0010 fixed 20-byte query ABI.
	if errno != 0 {
		return errno
	}
	return nil
}

func pppPairReadChannelOwner(fd int, owner *pppPairChannelOwner) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), pppPairQueryChannel,
		uintptr(unsafe.Pointer(owner))) //nolint:gosec // Root0010 fixed 16-byte query ABI.
	if errno != 0 {
		return errno
	}
	return nil
}

// Native admission requires positive replies from BOTH actual originals.
// A fresh-file early probe only recognizes entry handling, never this grant.
// The original namespace is still active here. Later cleanup uses retained
// original files and its initial grant, without querying a retired object.
func inspectOriginalPairFiles(pair *claimedSessionPair, channelFD, unitFD int) error {
	index := pair.info.identity.NativeIndex
	if index == 0 {
		return errors.New("ppp: original unit has no native interface index")
	}
	if index > math.MaxInt32 {
		return errors.New("ppp: original unit index does not fit the native ABI")
	}
	if pair.info.number < 0 {
		return errors.New("ppp: original unit number is negative")
	}
	if pair.info.number > math.MaxInt32 {
		return errors.New("ppp: original unit number does not fit the native ABI")
	}
	unit := pppPairUnitOwner{abi: pppPairOwnerABI, ifindex: int32(index), unit: int32(pair.info.number)}
	if err := pppPairReadUnitOwner(unitFD, &unit); err != nil {
		return err
	}
	if unit.abi != pppPairOwnerABI {
		return errors.New("ppp: original unit query returned another ABI")
	}
	if unit.reserved != 0 {
		return errors.New("ppp: original unit query returned reserved data")
	}
	if unit.ifindex != int32(index) {
		return errors.New("ppp: original unit query changed its captured index")
	}
	if unit.unit != int32(pair.info.number) {
		return errors.New("ppp: original unit query changed its captured unit")
	}
	if unit.features&pppPairConnectFeature == 0 {
		return unix.EOPNOTSUPP
	}
	channel := pppPairChannelOwner{abi: pppPairOwnerABI}
	if err := pppPairReadChannelOwner(channelFD, &channel); err != nil {
		return err
	}
	if channel.abi != pppPairOwnerABI {
		return errors.New("ppp: original channel query returned another ABI")
	}
	if channel.reserved != 0 {
		return errors.New("ppp: original channel query returned reserved data")
	}
	if channel.channel < 0 {
		return errors.New("ppp: original channel query returned an invalid label")
	}
	if channel.features&pppPairConnectFeature == 0 {
		return unix.EOPNOTSUPP
	}
	pair.original = &originalPairReceipt{unitFeatures: unit.features, channelFeatures: channel.features}
	return nil
}
