//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- firewall observation in the captured namespace under the unit's capabilities.
// Related: espform_firewall_kernel_linux.go -- real reader and lasting watch sockets.
// Related: unit_capabilities_integration_linux_test.go -- exact capability assertions.
package dataplane

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/google/nftables"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/privilege"
)

const (
	espFormNetNSTestModeEnv       = "ZE_TEST_ESP_FORM_NETNS_MODE"
	espFormNetNSTestReader        = "current-reader"
	espFormNetNSTestWatch         = "current-watch"
	espFormNetNSTestCurrentZero   = "current-zero-watch"
	espFormNetNSTestDifferent     = "different-watch"
	espFormNetNSTestDifferentZero = "different-zero-watch"
	espFormNetNSTestClosed        = "closed-watch"
	espFormNetNSTestClosedLow     = "closed-low-watch"
	espFormNetNSTestPassed        = "esp-form-netns: proved "
)

// TestESPFormNFTReaderUnderUnitCapabilities reads the helper's own table through
// the real netfilter socket, as a non-root process with exactly the unit's capabilities.
//
//ze:guest-root
func TestESPFormNFTReaderUnderUnitCapabilities(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestReader)
}

// TestESPFormNFTWatchUnderUnitCapabilities opens the lasting socket for a second
// descriptor of the current namespace without requiring CAP_SYS_ADMIN.
//
//ze:guest-root
func TestESPFormNFTWatchUnderUnitCapabilities(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestWatch)
}

// TestESPFormNFTWatchAcceptsCurrentNamespaceAtDescriptorZero preserves a valid
// namespace descriptor even when its number is the vendor's no-namespace sentinel.
//
//ze:guest-root
func TestESPFormNFTWatchAcceptsCurrentNamespaceAtDescriptorZero(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestCurrentZero)
}

// TestESPFormNFTWatchRefusesDifferentNamespaceUnderUnitCapabilities prevents a
// namespace error from silently opening a watch in the caller's own namespace.
//
//ze:guest-root
func TestESPFormNFTWatchRefusesDifferentNamespaceUnderUnitCapabilities(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestDifferent)
}

// TestESPFormNFTWatchRefusesDifferentNamespaceAtDescriptorZero prevents the
// vendor's zero sentinel from silently watching the caller's namespace instead.
//
//ze:guest-root
func TestESPFormNFTWatchRefusesDifferentNamespaceAtDescriptorZero(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestDifferentZero)
}

// TestESPFormNFTWatchRefusesClosedNamespace prevents a failed identity read from
// being treated as equality, including when both operations receive one descriptor.
//
//ze:guest-root
func TestESPFormNFTWatchRefusesClosedNamespace(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestClosed)
}

// TestESPFormNFTWatchRefusesClosedLowNamespace keeps a closed target from being
// mistaken for the current namespace when opening another handle reuses its number.
//
//ze:guest-root
func TestESPFormNFTWatchRefusesClosedLowNamespace(t *testing.T) {
	runESPFormNetNSHelper(t, espFormNetNSTestClosedLow)
}

func runESPFormNetNSHelper(t *testing.T, mode string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU to create a network namespace and start a non-root helper with the unit's capabilities")
	}
	origin, err := os.Open("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("open the parent network namespace: %v", err)
	}
	defer func() {
		if err := origin.Close(); err != nil {
			t.Errorf("close the parent namespace descriptor: %v", err)
		}
	}()
	capabilities := privilege.DaemonCapabilities()
	ambient := make([]uintptr, 0, len(capabilities))
	for _, capability := range capabilities {
		ambient = append(ambient, uintptr(capability.Bit))
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0],
		"-test.run=^TestESPFormNetNSHelperProcess$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), espFormNetNSTestModeEnv+"="+mode)
	cmd.ExtraFiles = []*os.File{origin}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWNET,
		Credential:  &syscall.Credential{Uid: unitCapsUnprivilegedID, Gid: unitCapsUnprivilegedID},
		AmbientCaps: ambient,
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the %s namespace helper failed under the unit's capabilities: %v\n%s", mode, err, output)
	}
	if !strings.Contains(string(output), espFormNetNSTestPassed+mode) {
		t.Fatalf("the %s namespace helper exited without its proof marker:\n%s", mode, output)
	}
	t.Logf("helper output:\n%s", output)
}

// TestESPFormNetNSHelperProcess is the isolated non-root subprocess. Its parent
// checks the exit status and a mode-specific marker after all real socket assertions.
// test-asserts-nothing: subprocess entry point; the parent checks its result.
func TestESPFormNetNSHelperProcess(t *testing.T) {
	mode := os.Getenv(espFormNetNSTestModeEnv)
	if mode == "" {
		return
	}
	requireExactlyTheUnitCapabilities(t)
	// Keep the current namespace stable through the socket's actual creation.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	switch mode {
	case espFormNetNSTestReader:
		espFormNetNSReadOwnTable(t)
	case espFormNetNSTestWatch, espFormNetNSTestCurrentZero:
		ns, err := netns.Get()
		if err != nil {
			t.Fatalf("open another descriptor of the current namespace: %v", err)
		}
		defer func() {
			if err := ns.Close(); err != nil {
				t.Errorf("close the current namespace descriptor: %v", err)
			}
		}()
		target := ns
		if mode == espFormNetNSTestCurrentZero {
			if err := unix.Dup3(int(ns), 0, unix.O_CLOEXEC); err != nil {
				t.Fatalf("place the current namespace at descriptor zero: %v", err)
			}
			target = netns.NsHandle(0)
		}
		watch, _, err := startESPFormNFTWatch(target)
		if err != nil {
			t.Fatalf("open the current-namespace nftables watch without CAP_SYS_ADMIN: %v", err)
		}
		watch.Close()
	case espFormNetNSTestDifferent:
		// The parent descriptor is fd 3; CLONE_NEWNET put this process elsewhere.
		espFormNetNSRequireRefusal(t, netns.NsHandle(3), unix.EPERM)
	case espFormNetNSTestDifferentZero:
		if err := unix.Dup3(3, 0, unix.O_CLOEXEC); err != nil {
			t.Fatalf("place the different namespace at descriptor zero: %v", err)
		}
		espFormNetNSRequireRefusal(t, netns.NsHandle(0), unix.EPERM)
	case espFormNetNSTestClosed:
		// A high closed descriptor cannot be reused by the opener's own handles.
		fd, err := unix.FcntlInt(3, unix.F_DUPFD_CLOEXEC, 256)
		if err != nil {
			t.Fatalf("duplicate a descriptor for the closed-handle control: %v", err)
		}
		if err := unix.Close(fd); err != nil {
			t.Fatalf("close the negative-control descriptor: %v", err)
		}
		espFormNetNSRequireRefusal(t, netns.NsHandle(fd), unix.EBADF)
	case espFormNetNSTestClosedLow:
		// Validate this target before opening any handle that could reuse fd 3.
		if err := unix.Close(3); err != nil {
			t.Fatalf("close the inherited low descriptor: %v", err)
		}
		espFormNetNSRequireRefusal(t, netns.NsHandle(3), unix.EBADF)
	default:
		t.Fatalf("unknown namespace helper mode %q", mode)
	}
	t.Log(espFormNetNSTestPassed + mode)
}

func espFormNetNSRequireRefusal(t *testing.T, ns netns.NsHandle, want error) {
	t.Helper()
	watch, _, err := startESPFormNFTWatch(ns)
	if watch != nil {
		watch.Close()
	}
	if !errors.Is(err, want) {
		t.Fatalf("watch for the refused namespace returned %v, want %v", err, want)
	}
}

func espFormNetNSReadOwnTable(t *testing.T) {
	t.Helper()
	conn, err := nftables.New(nftables.AsLasting())
	if err != nil {
		t.Fatalf("open a control connection in the helper namespace: %v", err)
	}
	defer func() {
		if err := conn.CloseLasting(); err != nil {
			t.Errorf("close the control connection: %v", err)
		}
	}()
	table := conn.AddTable(&nftables.Table{Family: nftables.TableFamilyINet, Name: "ze_netns_reader_test"})
	if err := conn.Flush(); err != nil {
		t.Fatalf("create the helper's own control table: %v", err)
	}
	// This private namespace and its table disappear with the helper process.
	reader, closeReader, err := openESPFormNFTReader()
	if err != nil {
		t.Fatalf("open the current-namespace nftables reader: %v", err)
	}
	defer func() {
		if err := closeReader(); err != nil {
			t.Errorf("close the current-namespace reader: %v", err)
		}
	}()
	tables, err := reader.ListTables()
	if err != nil {
		t.Fatalf("read the helper's namespace tables: %v", err)
	}
	for _, observed := range tables {
		if observed.Family == table.Family {
			if observed.Name == table.Name {
				return
			}
		}
	}
	t.Fatal("the reader did not observe its own namespace's control table")
}
