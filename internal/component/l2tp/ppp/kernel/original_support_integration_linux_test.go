//go:build integration && linux

// Design: docs/architecture/doctor-and-health-checks.md -- running kernel declaration.
// Related: client_interleave_integration_linux_test.go -- real producer/control fixture.

package kernel

import (
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// Real query42 succeeds twice on one fresh unbound file, leaves it unbound,
// and is not a receipt on which an uninstantiated pair can operate.
func TestOriginalSupportNativeFreshUnbound(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	file, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	for range 2 {
		reply := pppSupportReply{abi: pppSupportABI}
		if err := inspectOriginalAttachmentFile(file, &reply); err != nil {
			t.Fatalf("query42 prerequisite: %v", err)
		}
		if err := validateOriginalSupportReply(reply); err != nil {
			t.Fatal(err)
		}
		if reply.features&pppSupportSession != pppSupportSession {
			t.Fatalf("missing base declaration %#x", reply.features)
		}
		var unitErr error
		connection, err := file.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		if err := connection.Control(func(fd uintptr) { _, unitErr = ioctlGetInt(int(fd), pppiocGUnit) }); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(unitErr, unix.ENOTTY) {
			t.Fatalf("support query bound its fresh file: %v", unitErr)
		}
	}
	result := inspectOriginalAttachment(pppSupportSession)
	if result.status != originalAttachmentSupported {
		t.Fatalf("real declaration %v: %v", result.status, result.cause)
	}
	provenance, err := originalAttachmentKernel()
	if err != nil {
		t.Fatal(err)
	}
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(provenance, "release="+unix.ByteSliceToString(name.Release[:])) {
		t.Fatal(provenance)
	}
	if !strings.Contains(provenance, "machine="+unix.ByteSliceToString(name.Machine[:])) {
		t.Fatal(provenance)
	}
}

// The actual kernel rejects malformed input rather than creating or attaching
// anything; a subsequent valid query on the same original still succeeds.
func TestOriginalSupportNativeMalformedInput(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	file, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, reply := range []pppSupportReply{{abi: 2}, {abi: 1, features: 1}, {abi: 1, reserved0: 1}, {abi: 1, reserved1: 1}} {
		if err := inspectOriginalAttachmentFile(file, &reply); !errors.Is(err, unix.EINVAL) {
			t.Fatalf("malformed input: %v", err)
		}
	}
	reply := pppSupportReply{abi: 1}
	if err := inspectOriginalAttachmentFile(file, &reply); err != nil {
		t.Fatal(err)
	}
	if err := validateOriginalSupportReply(reply); err != nil {
		t.Fatal(err)
	}
}

// Permission refusal is a measured Unknown from the exact production probe,
// followed by useful support after restoring the same thread's capability.
func TestOriginalSupportNativePermissionUnknown(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var original [2]unix.CapUserData
	if err := unix.Capget(&header, &original[0]); err != nil {
		t.Fatal(err)
	}
	reduced := original
	reduced[unix.CAP_NET_ADMIN/32].Effective &^= uint32(1) << (unix.CAP_NET_ADMIN % 32)
	if err := unix.Capset(&header, &reduced[0]); err != nil {
		t.Fatal(err)
	}
	result := inspectOriginalAttachment(pppSupportSession)
	if err := unix.Capset(&header, &original[0]); err != nil {
		t.Fatalf("restore native capability: %v", err)
	}
	if result.status != originalAttachmentUnknown {
		t.Fatalf("permission became absence/support: %v %v", result.status, result.cause)
	}
	if !errors.Is(result.cause, unix.EPERM) {
		t.Fatal(result.cause)
	}
	if err := CheckOriginalPPPSupport(); err != nil {
		t.Fatalf("restored capability cannot inspect: %v", err)
	}
}

// A valid declaration and real original-file receipts are followed by useful
// Connect/MRU/mode/disconnect. Kernel producer cuts must make THIS test red.
func TestOriginalSupportNativeUsefulOriginalPair(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if err := CheckOriginalPPPSupport(); err != nil {
		t.Fatal(err)
	}
	client, _, channel, _, _ := clientNativeInterleaveInputs(t, "zsupport-pair", 192)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if err := client.SetIPv4Mode(NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := client.SetIPv6Mode(NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := clientNativeAttachmentWitness(channel); err != nil {
		t.Fatal(err)
	}
}

// The same real pair passes declaration and live-owner admission before a
// thread-local kernel syscall filter refuses only Connect50 with ENOTTY. The
// caller retains that late cause and never enables the unit receive lane.
func TestOriginalSupportNativeLateENOTTY(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	client, _, channel, _, _ := clientNativeInterleaveInputs(t, "zsupport-late", 193)
	if err := CheckOriginalPPPSupport(); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	// No Unlock: this dedicated test goroutine exits and retires its filtered
	// thread. The filter cannot be removed and MUST NOT reach another test.
	var native [8]byte
	binary.NativeEndian.PutUint64(native[:], 1)
	argumentOffset := uint32(24)
	if native[0] == 0 {
		argumentOffset += 4
	}
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.SYS_IOCTL, Jf: 3},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: argumentOffset},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: pppPairConnect, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.ENOTTY)},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER,
		uintptr(unsafe.Pointer(&program)), 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := client.Connect(); !errors.Is(err, unix.ENOTTY) {
		t.Fatalf("late refusal cause: %v", err)
	}
	if client.input.readUnit() {
		t.Fatal("late refusal enabled unit input")
	}
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if client.input.readUnit() {
		t.Fatal("MRU after refused connect enabled unit input")
	}
	if err := clientNativeAttachmentWitness(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("late refusal attached numerically: %v", err)
	}
	if err := CheckOriginalPPPSupport(); err != nil {
		t.Fatalf("connect refusal changed compiled declaration: %v", err)
	}
}
