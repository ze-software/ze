// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared /dev/ppp setup for L2TP and PPPoE

//go:build linux

package ppp

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

const (
	pppiocGChan   = 0x80047437 // PPPIOCGCHAN: _IOR('t', 55, int)
	pppiocAttChan = 0x40047438 // PPPIOCATTCHAN: _IOW('t', 56, int)
	pppiocNewUnit = 0xc004743e // PPPIOCNEWUNIT: allocate PPP unit (creates pppN)
	pppiocGUnit   = 0x80047456 // PPPIOCGUNIT: _IOR('t', 86, int)
	pppiocSNPMODE = 0x4008744b // PPPIOCSNPMODE: _IOW('t', 75, struct npioctl)
)

// namedUnitRetries bounds the EBUSY (ppp_mutex contention, ppp_nl_newlink uses
// mutex_trylock to avoid lock inversion) and EEXIST (previous session's device
// not yet released) retries a named RTM_NEWLINK survives. The budget is
// deliberate: an operator misconfiguration that collides a configured name
// with a persistent kernel device must surface quickly, so the error names the
// device and the fix.
const (
	namedUnitRetries = 5
	namedUnitBackoff = 20 * time.Millisecond
)

// SetIPv4Mode controls IPv4 in both directions without detaching the PPP unit.
// The caller MUST serialize this call with descriptor closure and MUST keep the
// unit descriptor reserved for the duration of the call.
// Linux API: https://github.com/torvalds/linux/blob/v6.18/include/uapi/linux/ppp-ioctl.h
// npioctl native-endian layout: 0..3 int protocol | 4..7 enum NPmode mode.
// RFC 1661 Section 3.6: "Any supported network-layer protocol packets received
// when the corresponding NCP is not in the Opened state MUST be silently discarded."
func SetIPv4Mode(unitFD int, mode NPMode) error {
	value, err := mode.kernelValue()
	if err != nil {
		return err
	}
	request := [2]int32{0x21, value}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(unitFD), pppiocSNPMODE,
		uintptr(unsafe.Pointer(&request))) //nolint:gosec // ioctl requires a native npioctl pointer.
	if errno != 0 {
		return fmt.Errorf("ppp: cannot change IPv4 forwarding on the PPP unit; check PPP support with ze doctor: %w", errno)
	}
	return nil
}

// DevPPPSetup opens /dev/ppp, attaches the PPP channel from the PPPoX
// socket (L2TP or PPPoE), allocates a PPP unit, and returns the channel fd,
// unit fd, and unit number.
//
// When name is empty, the kernel names the device pppN through PPPIOCNEWUNIT:
//  1. PPPIOCGCHAN on pppoxFD -> channel index
//  2. open /dev/ppp, PPPIOCATTCHAN -> channel fd
//  3. open /dev/ppp, PPPIOCNEWUNIT -> unit fd + pppN interface
//
// When name is non-empty, step 3 instead sends RTM_NEWLINK with kind "ppp"
// and IFLA_PPP_DEV_FD on the opened /dev/ppp file so the kernel keeps the
// name the request carries (Linux drivers/net/ppp/ppp_generic.c
// ppp_nl_newlink, v6.18). PPPIOCGUNIT on the unit descriptor then reads back
// the auto-assigned unit number PPPIOCCONNECT needs. EBUSY (ppp_mutex
// contention, ppp_nl_newlink uses mutex_trylock) and EEXIST (previous
// session's device still pending teardown) retry with a bounded backoff.
//
// PPPIOCCONNECT (step 4) is deferred to after LCP completes.
func DevPPPSetup(pppoxFD int, name string) (chanFD, unitFD, unitNum int, err error) {
	chanIdx, err := ioctlGetInt(pppoxFD, pppiocGChan)
	if err != nil {
		return -1, -1, -1, fmt.Errorf("ppp: PPPIOCGCHAN: %w", err)
	}

	chanFD, err = openDevPPP()
	if err != nil {
		return -1, -1, -1, err
	}
	if err := ioctlSetInt(chanFD, pppiocAttChan, chanIdx); err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, fmt.Errorf("ppp: PPPIOCATTCHAN: %w", err)
	}

	unitFD, err = openDevPPP()
	if err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, err
	}
	if name == "" {
		unitNum, err = ioctlGetSetInt(unitFD, pppiocNewUnit, -1)
		if err != nil {
			unix.Close(unitFD) //nolint:errcheck // rollback
			unix.Close(chanFD) //nolint:errcheck // rollback
			return -1, -1, -1, fmt.Errorf("ppp: PPPIOCNEWUNIT: %w", err)
		}
		return chanFD, unitFD, unitNum, nil
	}
	if err := createPPPLink(unitFD, name); err != nil {
		unix.Close(unitFD) //nolint:errcheck // rollback
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, err
	}
	unitNum, err = ioctlGetInt(unitFD, pppiocGUnit)
	if err != nil {
		unix.Close(unitFD) //nolint:errcheck // rollback
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, fmt.Errorf("ppp: PPPIOCGUNIT on %s: %w; the configured pppoe-client name matches a kernel device another program created, rename the entry so its key is unique", name, err)
	}
	return chanFD, unitFD, unitNum, nil
}

// createPPPLink sends RTM_NEWLINK kind "ppp" with IFLA_PPP_DEV_FD so the
// kernel keeps the configured name. EBUSY and EEXIST retry with a bounded
// backoff; any other error names the condition and the fix.
func createPPPLink(unitFD int, name string) error {
	for attempt := range namedUnitRetries {
		err := sendPPPLinkCreate(unitFD, name)
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.EBUSY) && !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("ppp: rtnetlink create kind=ppp name=%s: %w", name, err)
		}
		if attempt == namedUnitRetries-1 {
			return fmt.Errorf("ppp: rtnetlink create kind=ppp name=%s: %w after %d attempts; a previous session device with the same name is still being released, or another program holds the name, rename the pppoe-client entry or wait for the kernel to release it", name, err, namedUnitRetries)
		}
		time.Sleep(namedUnitBackoff)
	}
	// Reached only if namedUnitRetries is not positive: no create was attempted,
	// so the device does not exist and success would be a lie.
	return fmt.Errorf("ppp: rtnetlink create kind=ppp name=%s: no attempt made (retry budget %d); this is a Ze defect, report it", name, namedUnitRetries)
}

// sendPPPLinkCreate issues one RTM_NEWLINK. Its body is IFLA_IFNAME and the
// IFLA_LINKINFO{IFLA_INFO_KIND="ppp", IFLA_INFO_DATA{IFLA_PPP_DEV_FD=unitFD}}
// nest the kernel's ppp_nl_newlink parser expects.
func sendPPPLinkCreate(unitFD int, name string) error {
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	req.AddData(nl.NewIfInfomsg(unix.AF_UNSPEC))
	req.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name)))
	linkInfo := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
	linkInfo.AddRtAttr(nl.IFLA_INFO_KIND, nl.NonZeroTerminated("ppp"))
	infoData := linkInfo.AddRtAttr(nl.IFLA_INFO_DATA, nil)
	infoData.AddRtAttr(unix.IFLA_PPP_DEV_FD, nl.Uint32Attr(uint32(unitFD)))
	req.AddData(linkInfo)
	_, err := req.Execute(unix.NETLINK_ROUTE, 0)
	return err
}

func openDevPPP() (int, error) {
	f, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		return -1, fmt.Errorf("ppp: open /dev/ppp: %w", err)
	}
	rawFD, err := dupFD(f)
	f.Close() //nolint:errcheck // source replaced by rawFD
	if err != nil {
		return -1, fmt.Errorf("ppp: dup /dev/ppp fd: %w", err)
	}
	return rawFD, nil
}

func dupFD(f *os.File) (int, error) {
	raw, err := f.SyscallConn()
	if err != nil {
		return -1, err
	}
	var fd int
	var opErr error
	if err := raw.Control(func(fdp uintptr) {
		fd, opErr = unix.Dup(int(fdp))
	}); err != nil {
		return -1, err
	}
	return fd, opErr
}

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

func ioctlSetInt(fd int, req uint, val int) error {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}
