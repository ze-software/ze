// Design: docs/research/l2tpv2-ze-integration.md -- native capture control boundary

//go:build integration && linux

package kernel

import (
	"errors"
	"os"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// Two genuine live transports and a fresh unbound file exercise the actual
// kernel capture. Replacing only the query's display label with a real foreign
// channel label MUST still capture the exact original socket's channel. This
// uses no fabricated public owner or successful model authority.
func TestOriginalPPPChannelSupplierNativeForeignQueryLabelIsDisplayOnly(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original, foreign, target, _ := supplierCaptureNativeFiles(t)
	var originalLabel, foreignLabel int32
	if err := supplierCaptureFileLease(original, func(fd int) error {
		owner := pppTransportOwner{abi: 1}
		if err := queryPPPTransportOwner(fd, &owner); err != nil {
			return err
		}
		originalLabel = owner.channel
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := supplierCaptureFileLease(foreign, func(fd int) error {
		owner := pppTransportOwner{abi: 1}
		if err := queryPPPTransportOwner(fd, &owner); err != nil {
			return err
		}
		foreignLabel = owner.channel
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if originalLabel == foreignLabel {
		t.Fatal("foreign native control is not distinct")
	}
	err := supplierCaptureFileLease(original, func(sourceFD int) error {
		return supplierCaptureFileLease(target, func(targetFD int) error {
			return captureOriginalPPPChannel(sourceFD, targetFD,
				func(fd int, owner *pppTransportOwner) error {
					if err := queryPPPTransportOwner(fd, owner); err != nil {
						return err
					}
					owner.channel = foreignLabel
					return nil
				}, capturePPPTransportOwner)
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := supplierCaptureNativeLabel(t, target); got != originalLabel {
		t.Fatalf("display label redirected capture to%d; original%d foreign%d", got, originalLabel, foreignLabel)
	}
}

// Unregister the actual original device after a successful actual query but
// before capture45. A foreign transport stays live and supplies a tempting
// channel label; capture MUST return ENOTCONN and leave its target unbound.
func TestOriginalPPPChannelSupplierNativeUnregisterBetweenQueryAndCapture(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original, foreign, target, unregister := supplierCaptureNativeFiles(t)
	var foreignLabel int32
	if err := supplierCaptureFileLease(foreign, func(fd int) error {
		owner := pppTransportOwner{abi: 1}
		if err := queryPPPTransportOwner(fd, &owner); err != nil {
			return err
		}
		foreignLabel = owner.channel
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	err := supplierCaptureFileLease(original, func(sourceFD int) error {
		return supplierCaptureFileLease(target, func(targetFD int) error {
			return captureOriginalPPPChannel(sourceFD, targetFD,
				func(fd int, owner *pppTransportOwner) error {
					if err := queryPPPTransportOwner(fd, owner); err != nil {
						return err
					}
					if err := unregister(); err != nil {
						return err
					}
					owner.channel = foreignLabel
					return nil
				}, capturePPPTransportOwner)
		})
	})
	if !errors.Is(err, unix.ENOTCONN) {
		t.Fatalf("unregistered original captured another channel: %v", err)
	}
	if err := supplierCaptureFileLease(target, func(fd int) error {
		request := [4]int32{1}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc010742e, uintptr(unsafe.Pointer(&request))) //nolint:gosec // native channel query.
		if errno != unix.EBADF {
			t.Fatalf("failed capture target became bound: %v", errno)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	other, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := supplierCaptureFileLease(foreign, func(sourceFD int) error {
		return supplierCaptureFileLease(other, func(targetFD int) error {
			return captureOriginalPPPChannel(sourceFD, targetFD, queryPPPTransportOwner, capturePPPTransportOwner)
		})
	}); err != nil {
		t.Fatalf("foreign survivor positive capture failed: %v", err)
	}
	if got := supplierCaptureNativeLabel(t, other); got != foreignLabel {
		t.Fatalf("foreign survivor captured%d want%d", got, foreignLabel)
	}
}

func supplierCaptureNativeFiles(t *testing.T) (*os.File, *os.File, *os.File, func() error) {
	t.Helper()
	links := []*netlink.Dummy{{Name: "zc-capture-src"}, {Name: "zc-capture-other"}}
	owned := [2]bool{}
	for index, link := range links {
		if err := netlink.LinkAdd(link); err != nil {
			t.Fatal(err)
		}
		owned[index] = true
		t.Cleanup(func() {
			if owned[index] {
				if err := netlink.LinkDel(link); err != nil {
					t.Error(err)
				}
			}
		})
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
	}
	original := supplierCaptureSocket(t, links[0].Name, 101)
	foreign := supplierCaptureSocket(t, links[1].Name, 102)
	// A missing native ABI is an explicit unexecuted prerequisite, never a pass.
	if err := supplierCaptureFileLease(original, func(fd int) error { owner := pppTransportOwner{abi: 1}; return queryPPPTransportOwner(fd, &owner) }); err != nil {
		if errors.Is(err, unix.ENOTTY) {
			t.Skipf("runtime capture ABI prerequisite absent: %v", err)
		}
		t.Fatal(err)
	}
	target, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("native /dev/ppp prerequisite absent: %v", err)
	}
	t.Cleanup(func() {
		if err := target.Close(); err != nil {
			t.Error(err)
		}
	})
	return original, foreign, target, func() error {
		if err := netlink.LinkDel(links[0]); err != nil {
			return err
		}
		owned[0] = false
		return nil
	}
}

func supplierCaptureSocket(t *testing.T, name string, sid uint16) *os.File {
	t.Helper()
	fd, err := unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Skipf("native PPPoX prerequisite absent: %v", err)
	}
	file := os.NewFile(uintptr(fd), "ppp.supplier.actual.socket")
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Connect(fd, &unix.SockaddrPPPoE{SID: sid, Dev: name, Remote: []byte{2, 0, 0, 0, 0, byte(sid)}}); err != nil {
		t.Fatal(err)
	}
	return file
}

func supplierCaptureFileLease(file *os.File, operation func(int) error) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	leaseErr := connection.Control(func(fd uintptr) { operationErr = operation(int(fd)) })
	return errors.Join(operationErr, leaseErr)
}

func supplierCaptureNativeLabel(t *testing.T, file *os.File) int32 {
	t.Helper()
	request := [4]int32{1}
	if err := supplierCaptureFileLease(file, func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc010742e, uintptr(unsafe.Pointer(&request))) //nolint:gosec // native channel query.
		if errno != 0 {
			return errno
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return request[2]
}
