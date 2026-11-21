// Design: docs/architecture/l2tp/bng-5-pppoe.md -- namespaced LCP readback
// Related: prepare_linux.go -- readControlLink

//go:build integration && linux

package vpp

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// The native reader must reach a real device in a different namespace and
// leave the calling thread in its original namespace, including on refusal.
func TestNativeVPPAccessControlReadbackKeepsCallerNamespace(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := os.MkdirAll("/run/netns", 0o755); err != nil {
		t.Fatal(err)
	}
	name := "ze-pppoe-access-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	path := "/run/netns/" + name
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	mounted := false
	t.Cleanup(func() {
		if mounted {
			if err := unix.Unmount(path, unix.MNT_DETACH); err != nil {
				t.Error(err)
			}
		}
		if err := os.Remove(path); err != nil {
			t.Error(err)
		}
	})
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	type createdNamespace struct {
		handle  netns.NsHandle
		pair    iface.LCPPair
		mounted bool
		err     error
	}
	created := make(chan createdNamespace, 1)
	go func() {
		runtime.LockOSThread()
		handle, err := netns.New()
		if err != nil {
			created <- createdNamespace{handle: netns.None(), err: err}
			return
		}
		source := fmt.Sprintf("/proc/self/task/%d/ns/net", unix.Gettid())
		if err := unix.Mount(source, path, "none", unix.MS_BIND, ""); err != nil {
			created <- createdNamespace{handle: handle, err: err}
			return
		}
		link := &netlink.Dummy{Name: "cp-proof", MTU: 1500,
			HardwareAddr: net.HardwareAddr{2, 3, 4, 5, 6, 7}}
		if err := netlink.LinkAdd(link); err != nil {
			created <- createdNamespace{handle: handle, mounted: true, err: err}
			return
		}
		actual, err := netlink.LinkByName("cp-proof")
		if err != nil {
			created <- createdNamespace{handle: handle, mounted: true, err: err}
			return
		}
		created <- createdNamespace{handle: handle, mounted: true, pair: iface.LCPPair{
			LinuxIndex: actual.Attrs().Index, ControlDevice: "cp-proof", Namespace: name}}
		// Namespace creation changed this worker; exit locked to discard it.
	}()
	result := <-created
	mounted = result.mounted
	if result.handle.IsOpen() {
		t.Cleanup(func() {
			if err := result.handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if result.err != nil {
		if errors.Is(result.err, unix.EPERM) {
			t.Skip("CAP_SYS_ADMIN and CAP_NET_ADMIN are required for the owned network namespace")
		}
		t.Fatal(result.err)
	}
	link, err := readControlLink(t.Context(), result.pair)
	if err != nil {
		t.Fatal(err)
	}
	if link.name != "cp-proof" {
		t.Fatalf("actual name=%q", link.name)
	}
	if link.index != result.pair.LinuxIndex {
		t.Fatalf("actual index=%d", link.index)
	}
	if link.mac != [6]byte{2, 3, 4, 5, 6, 7} {
		t.Fatalf("actual MAC=%v", link.mac)
	}
	if link.mtu != 1500 {
		t.Fatalf("actual MTU=%d", link.mtu)
	}
	wrong := result.pair
	wrong.ControlDevice = "wrong-device"
	if _, err := readControlLink(t.Context(), wrong); err == nil {
		t.Fatal("mismatched Linux identity accepted")
	}
	after, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := after.Close(); err != nil {
			t.Error(err)
		}
	}()
	if !after.Equal(original) {
		t.Fatal("control-link readback changed the caller namespace")
	}
}
