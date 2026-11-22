// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared typed PPP setup
// Related: created_channel_linux.go -- original channel file and namespace
// Related: created_unit_linux.go -- original unit creation and counter receipt

//go:build linux

package kernel

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	pppiocGUnit   = 0x80047456 // PPPIOCGUNIT: _IOR('t', 86, int)
	pppiocSNPMODE = 0x4008744b // PPPIOCSNPMODE: _IOW('t', 75, struct npioctl)
)

// A collision retries only after the original ordered socket definitively
// rejected it and the original rejected file closed. Uncertainty never retries.
const (
	unitCreationAttempts = 5
	unitCreationBackoff  = 20 * time.Millisecond
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

// DevPPPSetup attaches the original channel and creates the original unit using
// the already captured creator. Both named and unnamed units use its ordered
// creation socket and actual echo. PPPIOCCONNECT remains deferred until LCP.
// The transport owner MUST retain and serialize the original socket file against Close for this
// call. NewUnitCreator MUST be captured before transport/native allocation.
// Every nonnil result remains the caller's owner on EVERY outcome. On failure
// setup closes the original unit, then channel, and retains all close causes;
// returned pointers retain those exact outcomes. Close is idempotent. A caller
// MUST NOT reconstruct or close either file from a numeric descriptor label.
// Transfer a successful pair with unit.SessionConfig(channel), then NewSession.
func DevPPPSetup(creator *UnitCreator, pppox *os.File, name string) (*CreatedChannel, *CreatedUnit, error) {
	return devPPPSetup(creator, pppox, name, devPPPSetupOps{
		channel: CreatePPPChannel, createUnit: createPPPSetupUnit, closeChannel: (*CreatedChannel).Close,
		closeUnit: (*CreatedUnit).Close, rejected: (*CreatedUnit).CreationRejected,
		pause: time.Sleep,
	})
}

type devPPPSetupOps struct {
	channel      func(*os.File) (*CreatedChannel, error)
	createUnit   func(*UnitCreator, *CreatedChannel, string) (*CreatedUnit, error)
	closeChannel func(*CreatedChannel) error
	closeUnit    func(*CreatedUnit) error
	rejected     func(*CreatedUnit) bool
	pause        func(time.Duration)
}

func devPPPSetup(creator *UnitCreator, pppox *os.File, name string, ops devPPPSetupOps) (channel *CreatedChannel, unit *CreatedUnit, err error) {
	if err := validateDevPPPSetup(creator, ops); err != nil {
		return nil, nil, err
	}
	unitNeedsClose := false
	defer func() {
		if failure := recover(); failure != nil {
			err = opererr.Wrap(errors.Join(err, sessionPanicFailure(failure)), "PPP setup stopped unexpectedly",
				"The original unpublished channel and unit are being closed.", "ze doctor")
		}
		if err == nil {
			return
		}
		if unitNeedsClose {
			err = RecordSetupCleanup(err, closeDevPPPSetup(func() error { return ops.closeUnit(unit) }))
		}
		if channel != nil {
			err = RecordSetupCleanup(err, closeDevPPPSetup(func() error { return ops.closeChannel(channel) }))
		}
	}()
	channel, err = ops.channel(pppox)
	if err != nil {
		return channel, nil, err
	}
	if channel == nil {
		return nil, nil, devPPPSetupFailure("PPP setup returned no original channel")
	}
	for attempt := range unitCreationAttempts {
		unit, err = ops.createUnit(creator, channel, name)
		unitNeedsClose = unit != nil
		if err == nil {
			if unit == nil {
				return channel, nil, devPPPSetupFailure("PPP setup returned no original unit")
			}
			return channel, unit, nil
		}
		if unit == nil {
			return channel, nil, err
		}
		if !ops.rejected(unit) {
			return channel, unit, err
		}
		if !devPPPSetupCollision(err) {
			return channel, unit, err
		}
		if attempt == unitCreationAttempts-1 {
			return channel, unit, opererr.Wrap(err, fmt.Sprintf("PPP interface %q is still unavailable after %d creation attempts", name, unitCreationAttempts),
				"The previous interface may still be closing, or another program holds its name.", "ze doctor")
		}
		closeErr := closeDevPPPSetup(func() error { return ops.closeUnit(unit) })
		unitNeedsClose = false
		if closeErr != nil {
			return channel, unit, RecordSetupCleanup(err, closeErr)
		}
		// Only a definitively rejected, successfully closed unit is forgotten.
		// The same live original channel stays reserved throughout the retries.
		unit = nil
		// sleep(no-signal): ppp_mutex release and a previous device's final name
		// release have no waiter; only a fresh bounded exclusive create decides.
		ops.pause(unitCreationBackoff)
	}
	return channel, unit, devPPPSetupFailure("PPP setup made no unit creation attempt")
}

func validateDevPPPSetup(creator *UnitCreator, ops devPPPSetupOps) error {
	if creator == nil {
		return devPPPSetupFailure("PPP setup has no original interface creator")
	}
	if creator.create == nil {
		return devPPPSetupFailure("PPP setup has an uninitialized interface creator")
	}
	if ops.createUnit == nil {
		return devPPPSetupFailure("PPP setup has no original namespace unit creation operation")
	}
	if ops.channel == nil {
		return devPPPSetupFailure("PPP setup has no original channel operation")
	}
	if ops.closeChannel == nil {
		return devPPPSetupFailure("PPP setup has no original channel cleanup operation")
	}
	if ops.closeUnit == nil {
		return devPPPSetupFailure("PPP setup has no original unit cleanup operation")
	}
	if ops.rejected == nil {
		return devPPPSetupFailure("PPP setup has no native creation outcome reader")
	}
	if ops.pause == nil {
		return devPPPSetupFailure("PPP setup has no bounded collision retry wait")
	}
	return nil
}

func devPPPSetupCollision(cause error) bool {
	if errors.Is(cause, unix.EBUSY) {
		return true
	}
	return errors.Is(cause, unix.EEXIST)
}

func devPPPSetupFailure(problem string) error {
	return opererr.New(problem, "The subscriber cannot safely use its original PPP interface.", "ze doctor")
}

func closeDevPPPSetup(closeOwner func() error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = sessionPanicFailure(failure)
		}
	}()
	return closeOwner()
}

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}
