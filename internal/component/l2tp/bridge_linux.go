// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-3 / A-4 LAC data-plane bridge
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 2 (LAC forwards PPP; no local termination)

//go:build linux

package l2tp

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// PPP channel ioctls for the LAC data-plane bridge. golang.org/x/sys/unix
// does not export these for the amd64/arm64/riscv64 default arch, so -- as
// with ppp/mtu_linux.go's pppiocConnect -- they are defined locally.
//
//	PPPIOCGCHAN     _IOR('t', 55, int)  -- read a pppox socket's channel number
//	PPPIOCBRIDGECHAN _IOW('t', 53, int) -- bridge this channel to another number
//	PPPIOCUNBRIDGECHAN _IO('t', 52)     -- tear the bridge down
const (
	pppiocGChan        = 0x80047437
	pppiocBridgeChan   = 0x40047435
	pppiocUnbridgeChan = 0x00007434
)

// channelNumber returns the kernel PPP channel number underlying a pppox
// socket (PPPIOCGCHAN). This is the identifier PPPIOCBRIDGECHAN bridges to.
func channelNumber(pppoxFD int) (int, error) {
	var num int32
	//nolint:gosec // PPPIOCGCHAN's user pointer is the kernel's documented contract; no Go wrapper in golang.org/x/sys/unix
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(pppoxFD),
		uintptr(pppiocGChan), uintptr(unsafe.Pointer(&num)))
	if errno != 0 {
		return 0, fmt.Errorf("l2tp: PPPIOCGCHAN(fd=%d): %w", pppoxFD, errno)
	}
	return int(num), nil
}

// bridgeChannels cross-connects the L2TP session's PPP channel (l2tpChanFD, a
// /dev/ppp fd attached to the pppol2tp channel via PPPIOCATTCHAN) to the
// relayed subscriber's PPPoE channel (identified by peerPppoxFD, the PPPoE
// pppox socket). PPP frames then flow directly between the two channels in
// the kernel -- the LAC forwards PPP without terminating it (RFC 2661), so no
// local PPP unit runs for the subscriber.
//
// A single PPPIOCBRIDGECHAN establishes the bidirectional link: the kernel
// records each channel as the other's bridge. On kernels without
// PPPIOCBRIDGECHAN the caller falls back to relayFramesUserspace.
func bridgeChannels(l2tpChanFD, peerPppoxFD int) error {
	peerNum, err := channelNumber(peerPppoxFD)
	if err != nil {
		return err
	}
	arg := int32(peerNum)
	//nolint:gosec // PPPIOCBRIDGECHAN's user pointer is the kernel's documented contract; no Go wrapper in golang.org/x/sys/unix
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(l2tpChanFD),
		uintptr(pppiocBridgeChan), uintptr(unsafe.Pointer(&arg)))
	if errno != 0 {
		return fmt.Errorf("l2tp: PPPIOCBRIDGECHAN(chan-fd=%d, peer-num=%d): %w",
			l2tpChanFD, peerNum, errno)
	}
	return nil
}

// unbridgeChannel tears down a channel bridge (PPPIOCUNBRIDGECHAN), used on
// session teardown so the two channels can be closed independently.
func unbridgeChannel(l2tpChanFD int) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(l2tpChanFD),
		uintptr(pppiocUnbridgeChan), 0)
	if errno != 0 {
		return fmt.Errorf("l2tp: PPPIOCUNBRIDGECHAN(chan-fd=%d): %w", l2tpChanFD, errno)
	}
	return nil
}

// maybeBridgePPPoE bridges a LAC-relayed session's pppol2tp channel to the
// subscriber's PPPoE channel when the session carries one (ev.pppoeChannelFD
// set, lnsMode false). Returns true when a bridge was requested (so the
// caller skips starting a local PPP unit), along with any error. A false
// return means "not a relay session; run PPP locally as usual".
func maybeBridgePPPoE(ev kernelSetupEvent, l2tpChanFD int) (bridged bool, err error) {
	if ev.lnsMode || ev.pppoeChannelFD <= 0 {
		return false, nil
	}
	if berr := bridgeChannels(l2tpChanFD, ev.pppoeChannelFD); berr != nil {
		// A-4 fallback: on a kernel without PPPIOCBRIDGECHAN, a userspace
		// frame relay between the two channel fds keeps the call functional
		// (slower). Surfaced to the caller which logs and records the perf
		// implication; the bridge is still "requested" so PPP is not run.
		return true, berr
	}
	return true, nil
}
