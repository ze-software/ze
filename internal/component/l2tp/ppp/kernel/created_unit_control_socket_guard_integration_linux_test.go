// Design: docs/architecture/appliance/kernel-profiles.md -- original-unit control constructor guard proof
// Direct negative UAPI probes judge kernel defense, never production admission.

//go:build integration && linux

package kernel_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestCreatedUnitControlSocketNativeCreator refuses a foreign real creator
// and a real separately attached file, while the actual original still works.
func TestCreatedUnitControlSocketNativeCreator(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original, _ := controlSocketNativeUnit(t, "zcctrl")
	foreign, _ := controlSocketNativeUnit(t, "zcctrlother")
	request := controlSocketNativeRequest{abi: 1, kind: 2,
		unit: int32(original.UnitNumber()), index: int32(original.Identity().NativeIndex)}
	var returned *os.File
	var nativeErr error
	borrowErr := foreign.WithDescriptor(func(fd int) error {
		returned, nativeErr = controlSocketNativeRaw(fd, &request)
		return nativeErr
	})
	controlSocketNativeRefusal(t, returned, errors.Join(nativeErr, borrowErr), unix.ESTALE)
	attached, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := attached.Close(); err != nil {
			t.Errorf("close task-owned attachment: %v", err)
		}
	})
	raw, err := attached.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var attachErr error
	controlErr := raw.Control(func(fd uintptr) {
		attachErr = unix.IoctlSetPointerInt(int(fd), 0x4004743d, original.UnitNumber())
	})
	if err := errors.Join(attachErr, controlErr); err != nil {
		t.Fatal(err)
	}
	returned = nil
	nativeErr = nil
	controlErr = raw.Control(func(fd uintptr) {
		returned, nativeErr = controlSocketNativeRaw(int(fd), &request)
	})
	controlSocketNativeRefusal(t, returned, errors.Join(nativeErr, controlErr), unix.ESTALE)
	useful := controlSocketNativeOpen(t, original, pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRequireSocket(t, useful, pppkernel.IPv6ControlSocketUDP6, int(original.Identity().NativeIndex))
}

// TestCreatedUnitControlSocketNativeMalformed proves each exact input field
// refusal, native errno and input-only ABI, paired with a useful real request.
func TestCreatedUnitControlSocketNativeMalformed(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	base := controlSocketNativeRequest{abi: 1, kind: 2,
		unit: int32(unit.UnitNumber()), index: int32(unit.Identity().NativeIndex)}
	for _, entry := range []struct {
		name   string
		change func(*controlSocketNativeRequest)
		wanted error
	}{
		{"ABI", func(r *controlSocketNativeRequest) { r.abi++ }, unix.EINVAL},
		{"flags", func(r *controlSocketNativeRequest) { r.flags = 1 }, unix.EINVAL},
		{"reserved0", func(r *controlSocketNativeRequest) { r.reserved[0] = 1 }, unix.EINVAL},
		{"reserved1", func(r *controlSocketNativeRequest) { r.reserved[1] = 1 }, unix.EINVAL},
		{"reserved2", func(r *controlSocketNativeRequest) { r.reserved[2] = 1 }, unix.EINVAL},
		{"negative unit", func(r *controlSocketNativeRequest) { r.unit = -1 }, unix.EINVAL},
		{"zero index", func(r *controlSocketNativeRequest) { r.index = 0 }, unix.EINVAL},
		{"wrong unit", func(r *controlSocketNativeRequest) { r.unit++ }, unix.ESTALE},
		{"wrong index", func(r *controlSocketNativeRequest) { r.index++ }, unix.ESTALE},
		{"zero kind", func(r *controlSocketNativeRequest) { r.kind = 0 }, unix.EPROTONOSUPPORT},
		{"unknown kind", func(r *controlSocketNativeRequest) { r.kind = 3 }, unix.EPROTONOSUPPORT},
	} {
		t.Logf("actual ioctl43 malformed control: %s", entry.name)
		request := base
		entry.change(&request)
		before := request
		var returned *os.File
		var nativeErr error
		borrowErr := unit.WithDescriptor(func(fd int) error {
			returned, nativeErr = controlSocketNativeRaw(fd, &request)
			return nativeErr
		})
		controlSocketNativeRefusal(t, returned, errors.Join(nativeErr, borrowErr), entry.wanted)
		if request != before {
			t.Fatal("input-only ioctl43 modified a malformed request")
		}
	}
	useful := controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRequireSocket(t, useful, pppkernel.IPv6ControlSocketUDP6, int(unit.Identity().NativeIndex))
}

// TestCreatedUnitControlSocketNativePermission checks the actual target-netns
// CAP_NET_ADMIN refusal and errno, then restores permission and proves use.
func TestCreatedUnitControlSocketNativePermission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	var returned *os.File
	err := nativePPPGlobalSupportWithoutAdmin(t, func() error {
		var nativeErr error
		returned, nativeErr = unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketUDP6)
		return nativeErr
	})
	controlSocketNativeRefusal(t, returned, err, unix.EPERM)
	useful := controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	controlSocketNativeRequireSocket(t, useful, pppkernel.IPv6ControlSocketUDP6, int(unit.Identity().NativeIndex))
}

// TestCreatedUnitControlSocketNativeFDZero proves a NEW owned result at zero.
// userns.Enter owns this whole child process. Only its stdin is saved/restored;
// the actual PPP creator is never duplicated or wrapped by descriptor number.
func TestCreatedUnitControlSocketNativeFDZero(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcctrl")
	if err := unit.WithDescriptor(func(fd int) error {
		if fd == 0 {
			return errors.New("dedicated child stdin unexpectedly is the actual PPP creator")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := unix.FcntlInt(0, unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(saved); err != nil {
			t.Errorf("close saved child stdin: %v", err)
		}
	}()
	flags, err := unix.FcntlInt(0, unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	restoreFlags := 0
	if flags&unix.FD_CLOEXEC != 0 {
		restoreFlags = unix.O_CLOEXEC
	}
	defer func() {
		if err := unix.Dup3(saved, 0, restoreFlags); err != nil {
			t.Errorf("restore dedicated child stdin: %v", err)
		}
	}()
	if err := unix.Close(0); err != nil {
		t.Fatal(err)
	}
	file, err := unit.OpenIPv6ControlSocket(context.Background(), pppkernel.IPv6ControlSocketUDP6)
	if file != nil {
		// Function defers run before stdin restoration. A cleanup that ran
		// afterward could close the restored stdin through the stale fd0.
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("close actual new fd0 control file: %v", err)
			}
		}()
	}
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Fatal("native fd0 constructor returned no owned file")
	}
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	zero := false
	if err := raw.Control(func(fd uintptr) { zero = fd == 0 }); err != nil {
		t.Fatal(err)
	}
	if !zero {
		t.Fatal("test did not receive actual NEW constructor result fd0")
	}
	controlSocketNativeRequireSocket(t, file, pppkernel.IPv6ControlSocketUDP6, int(unit.Identity().NativeIndex))
}
