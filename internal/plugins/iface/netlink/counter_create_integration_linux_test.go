// Design: docs/features/interfaces.md -- original counter-source namespace
// Related: counter_linux.go -- the retained subscribed socket under test

//go:build integration && linux

package ifacenetlink

import (
	"errors"
	"runtime"
	"testing"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestCounterCreatorNamespacePrimitive compares the real subscribed socket's
// namespace with its creator and then a different caller namespace. This proves
// the primitive only; it neither creates a device nor implements the capability.
func TestCounterCreatorNamespacePrimitive(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	runtime.LockOSThread()

	original, err := netns.Get()
	if err != nil {
		// No namespace change was attempted, so this thread is still safe.
		runtime.UnlockOSThread()
		t.Fatalf("open creator namespace: %v", err)
	}
	defer counterCreatorCloseFD(t, int(original))
	// Restore before closing the original namespace or releasing the thread,
	// including when a prerequisite refusal or assertion ends the test early.
	// A failed restore leaves the thread locked so the runtime retires it.
	defer func() {
		if err := netns.Set(original); err != nil {
			t.Errorf("restore creator namespace: %v", err)
			return
		}
		runtime.UnlockOSThread()
	}()

	var source counterSource
	defer source.close()
	source.mu.Lock()
	err = source.open()
	source.mu.Unlock()
	if err != nil {
		counterCreatorPrimitiveError(t, "open subscribed counter socket", err)
	}
	socketNamespace := counterCreatorSocketNamespace(t, source.socket.GetFd())
	defer counterCreatorCloseFD(t, socketNamespace)
	socketIdentity := counterCreatorNamespaceStat(t, socketNamespace)
	originalIdentity := counterCreatorNamespaceStat(t, int(original))
	if socketIdentity.Dev != originalIdentity.Dev {
		t.Fatal("counter socket namespace device differs from its creator")
	}
	if socketIdentity.Ino != originalIdentity.Ino {
		t.Fatal("counter socket namespace inode differs from its creator")
	}

	other, err := netns.New()
	if err != nil {
		counterCreatorPrimitiveError(t, "create another caller namespace", err)
	}
	defer counterCreatorCloseFD(t, int(other))
	otherIdentity := counterCreatorNamespaceStat(t, int(other))
	if socketIdentity.Dev == otherIdentity.Dev {
		if socketIdentity.Ino == otherIdentity.Ino {
			t.Fatal("different caller namespace retained the creator's identity")
		}
	}
	afterNamespace := counterCreatorSocketNamespace(t, source.socket.GetFd())
	defer counterCreatorCloseFD(t, afterNamespace)
	afterIdentity := counterCreatorNamespaceStat(t, afterNamespace)
	if afterIdentity.Dev != socketIdentity.Dev {
		t.Fatal("counter socket namespace device followed the caller")
	}
	if afterIdentity.Ino != socketIdentity.Ino {
		t.Fatal("counter socket namespace inode followed the caller")
	}
	t.Logf("socket namespace dev=%d inode=%d; different caller dev=%d inode=%d",
		socketIdentity.Dev, socketIdentity.Ino, otherIdentity.Dev, otherIdentity.Ino)
}

func counterCreatorSocketNamespace(t *testing.T, socket int) int {
	t.Helper()
	fd, err := unix.IoctlRetInt(socket, unix.SIOCGSKNS)
	if err != nil {
		counterCreatorPrimitiveError(t, "read counter socket namespace with SIOCGSKNS", err)
	}
	return fd
}

func counterCreatorNamespaceStat(t *testing.T, fd int) unix.Stat_t {
	t.Helper()
	var identity unix.Stat_t
	if err := unix.Fstat(fd, &identity); err != nil {
		t.Fatalf("read namespace descriptor identity: %v", err)
	}
	return identity
}

func counterCreatorCloseFD(t *testing.T, fd int) {
	t.Helper()
	if err := unix.Close(fd); err != nil {
		t.Errorf("close owned namespace descriptor: %v", err)
	}
}

func counterCreatorPrimitiveError(t *testing.T, operation string, err error) {
	t.Helper()
	if errors.Is(err, unix.EPERM) {
		t.Skipf("namespace primitive prerequisite unavailable (%s): %v", operation, err)
	}
	if errors.Is(err, unix.EACCES) {
		t.Skipf("namespace primitive prerequisite unavailable (%s): %v", operation, err)
	}
	if errors.Is(err, unix.ENOSYS) {
		t.Skipf("namespace primitive prerequisite unavailable (%s): %v", operation, err)
	}
	if errors.Is(err, unix.ENOTTY) {
		t.Skipf("namespace primitive prerequisite unavailable (%s): %v", operation, err)
	}
	t.Fatalf("namespace primitive failed (%s): %v", operation, err)
}
