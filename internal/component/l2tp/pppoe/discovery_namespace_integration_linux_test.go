// Design: docs/architecture/l2tp/bng-5-pppoe.md -- real discovery namespace isolation

//go:build integration && linux

package pppoe

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// Goal: the production wrapper opens its AF_PACKET socket in the named
// namespace while leaving the caller in its original namespace. Independent
// sockets opened in each namespace provide positive SO_NETNS_COOKIE controls.
// The returned raw socket must also be marked close-on-exec.
func TestDiscoverySocketUsesPreparedNamespace(t *testing.T) {
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		current, readErr := netns.Get()
		restored := false
		if readErr != nil {
			t.Error(readErr)
		} else {
			restored = current.Equal(original)
			if err := current.Close(); err != nil {
				t.Error(err)
			}
			if !restored {
				if err := netns.Set(original); err != nil {
					t.Error(err)
				} else {
					restored = true
				}
			}
		}
		if err := original.Close(); err != nil {
			t.Error(err)
		}
		if restored {
			runtime.UnlockOSThread()
		}
	}()
	originalCookie := discoverySocketNamespaceCookie(t)
	name := "ze-pppoe-disc-" + strconv.Itoa(os.Getpid())
	namespacePath := "/run/netns/" + name
	if _, err := os.Lstat(namespacePath); err == nil {
		t.Skip("test namespace name is already owned by another run")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	// NewNamed can fail after creating the mountpoint. Own the initially
	// absent path before that call so partial namespace creation is cleaned too.
	defer func() {
		if _, err := os.Lstat(namespacePath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				t.Error(err)
			}
			return
		}
		if err := unix.Unmount(namespacePath, unix.MNT_DETACH); err != nil {
			if !errors.Is(err, unix.EINVAL) {
				t.Error(err)
				return
			}
		}
		if err := os.Remove(namespacePath); err != nil {
			t.Error(err)
		}
	}()
	target, err := netns.NewNamed(name)
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skip("CAP_SYS_ADMIN is required to create the test namespace")
		}
		t.Fatal(err)
	}
	defer func() {
		if err := target.Close(); err != nil {
			t.Error(err)
		}
	}()
	targetCookie := discoverySocketNamespaceCookie(t)
	if targetCookie == originalCookie {
		t.Fatal("independent namespace controls have the same cookie")
	}
	if err := netns.Set(original); err != nil {
		t.Fatal(err)
	}
	fd, err := openDiscoverySocketInNamespace(name)
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skip("CAP_NET_RAW is required to open the discovery socket")
		}
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	cookie, err := unix.GetsockoptUint64(fd, unix.SOL_SOCKET, unix.SO_NETNS_COOKIE)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != targetCookie {
		t.Fatalf("discovery namespace cookie=%d,want target %d", cookie, targetCookie)
	}
	current, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	if !current.Equal(original) {
		t.Error("discovery creation changed the caller namespace")
	}
	if err := current.Close(); err != nil {
		t.Fatal(err)
	}
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.FD_CLOEXEC == 0 {
		t.Fatal("discovery descriptor is inherited across exec")
	}
}

func discoverySocketNamespaceCookie(t *testing.T) uint64 {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	cookie, err := unix.GetsockoptUint64(fd, unix.SOL_SOCKET, unix.SO_NETNS_COOKIE)
	if err != nil {
		t.Fatal(err)
	}
	return cookie
}
