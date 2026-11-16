// Design: docs/functional-tests.md -- native process fixtures retain kernel ownership.

package teststressrepro

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// stressDescriptor has one owned received descriptor and its private control
// connection. Its zero and nil values are invalid. Test cleanup closes both;
// acknowledge releases the helper only after its kernel handle is accepted.
type stressDescriptor struct {
	file   *os.File
	leader int
	conn   *net.UnixConn
}

func stressDescriptorSocket(t *testing.T) (*net.UnixListener, string) {
	t.Helper()
	dir, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dir.Close(); err != nil {
			t.Errorf("close fixture socket directory: %v", err)
		}
	})
	// Keep the private directory open and use its procfs handle so a long
	// session TMPDIR cannot exceed sockaddr_un's pathname limit.
	path := fmt.Sprintf("/proc/%d/fd/%d/s", os.Getpid(), dir.Fd())
	listener, err := net.ListenUnix("unixpacket", &net.UnixAddr{Name: path, Net: "unixpacket"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close fixture descriptor listener: %v", err)
		}
	})
	return listener, path
}

func receiveStressDescriptor(t *testing.T, ctx context.Context, listener *net.UnixListener) *stressDescriptor {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("descriptor fixture requires a bounded context")
	}
	if err := listener.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close fixture descriptor connection: %v", err)
		}
	})
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	file, leader, err := readStressDescriptor(conn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close received fixture descriptor: %v", err)
		}
	})
	return &stressDescriptor{file: file, leader: leader, conn: conn}
}

func readStressDescriptor(conn *net.UnixConn) (_ *os.File, _ int, resultErr error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, 0, err
	}
	data := make([]byte, 64)
	control := make([]byte, unix.CmsgSpace(16*4))
	var size, controlSize, flags int
	var receiveErr error
	err = raw.Read(func(fd uintptr) bool {
		size, controlSize, flags, _, receiveErr = unix.Recvmsg(int(fd), data, control, unix.MSG_CMSG_CLOEXEC|unix.MSG_DONTWAIT)
		if errors.Is(receiveErr, unix.EAGAIN) {
			return false
		}
		if errors.Is(receiveErr, unix.EINTR) {
			return false
		}
		return true
	})
	if err != nil {
		return nil, 0, err
	}
	if receiveErr != nil {
		return nil, 0, receiveErr
	}
	messages, err := unix.ParseSocketControlMessage(control[:controlSize])
	if err != nil {
		return nil, 0, err
	}
	var descriptors []int
	defer func() {
		if resultErr == nil {
			return
		}
		for _, fd := range descriptors {
			if err := unix.Close(fd); err != nil {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}()
	var controlErr error
	for i := range messages {
		rights, err := unix.ParseUnixRights(&messages[i])
		if err != nil {
			controlErr = errors.Join(controlErr, err)
			continue
		}
		descriptors = append(descriptors, rights...)
	}
	if controlErr != nil {
		return nil, 0, controlErr
	}
	if flags&unix.MSG_CTRUNC != 0 {
		return nil, 0, errors.New("fixture descriptor control message was truncated")
	}
	if flags&unix.MSG_TRUNC != 0 {
		return nil, 0, errors.New("fixture descriptor identity was truncated")
	}
	if len(descriptors) != 1 {
		return nil, 0, fmt.Errorf("fixture sent %d descriptors, want one", len(descriptors))
	}
	leader, err := strconv.Atoi(string(data[:size]))
	if err != nil {
		return nil, 0, fmt.Errorf("read fixture leader identity: %w", err)
	}
	if leader <= 1 {
		return nil, 0, fmt.Errorf("invalid fixture leader PID %d", leader)
	}
	file := os.NewFile(uintptr(descriptors[0]), "stress fixture descriptor")
	if file == nil {
		return nil, 0, errors.New("fixture sent an invalid descriptor")
	}
	return file, leader, nil
}

func (d *stressDescriptor) acknowledge(t *testing.T) {
	t.Helper()
	if _, err := d.conn.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
}

func sendStressDescriptor(path string, fd int) error {
	conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: path, Net: "unixpacket"})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // The caller checks the complete transfer and acknowledgement.
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	data := []byte(strconv.Itoa(os.Getpid()))
	rights := unix.UnixRights(fd)
	n, controlN, err := conn.WriteMsgUnix(data, rights, nil)
	if err != nil {
		return err
	}
	if n != len(data) {
		return errors.New("fixture descriptor identity transfer was incomplete")
	}
	if controlN != len(rights) {
		return errors.New("fixture descriptor rights transfer was incomplete")
	}
	var ack [1]byte
	n, err = conn.Read(ack[:])
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("fixture descriptor acknowledgement was incomplete")
	}
	if ack[0] != 1 {
		return errors.New("fixture descriptor acknowledgement was invalid")
	}
	return nil
}

func stressPidfdLive(t *testing.T, file *os.File) bool {
	t.Helper()
	if err := unix.PidfdSendSignal(int(file.Fd()), 0, nil, 0); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return false
		}
		t.Fatalf("validate controlled descendant pidfd: %v", err)
	}
	poll := []unix.PollFd{{Fd: int32(file.Fd()), Events: unix.POLLIN}}
	for {
		_, err := unix.Poll(poll, 0)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if poll[0].Revents&unix.POLLIN != 0 {
			return false
		}
		if poll[0].Revents != 0 {
			t.Fatalf("unexpected fixture pidfd poll status %#x", poll[0].Revents)
		}
		return true
	}
}

func stopStressPidfd(t *testing.T, file *os.File) {
	t.Helper()
	if err := unix.PidfdSendSignal(int(file.Fd()), unix.SIGKILL, nil, 0); err != nil {
		if !errors.Is(err, unix.ESRCH) {
			t.Errorf("stop controlled descendant through pidfd: %v", err)
			return
		}
	}
	poll := []unix.PollFd{{Fd: int32(file.Fd()), Events: unix.POLLIN}}
	deadline := time.Now().Add(2 * time.Second)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Error("controlled descendant did not exit after pidfd cleanup")
			return
		}
		_, err := unix.Poll(poll, max(1, int(remaining.Milliseconds())))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Errorf("join controlled descendant through pidfd: %v", err)
			return
		}
		if poll[0].Revents&unix.POLLIN != 0 {
			return
		}
		if poll[0].Revents != 0 {
			t.Errorf("unexpected cleanup pidfd poll status %#x", poll[0].Revents)
			return
		}
	}
}

// TestStressProcessHelper transfers a handle before the parent acknowledges
// readiness. A direct unreaped child pins its PID until its pidfd is opened;
// the transferred handle, rather than a borrowed number, owns later cleanup.
func TestStressProcessHelper(t *testing.T) {
	if len(os.Args) < 4 {
		return
	}
	if os.Args[len(os.Args)-3] != "stress-process-fixture" {
		return
	}
	role, socket := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if role == "output" {
		if err := sendStressDescriptor(socket, int(os.Stdout.Fd())); err != nil {
			t.Fatal(err)
		}
		return
	}
	if role != "exit" {
		if role != "wait" {
			t.Fatalf("unknown process fixture role %q", role)
		}
	}
	cmd := exec.Command("sleep", "30") //nolint:noctx // The owning native runner stops this same-group child and the parent test retains its pidfd.
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.PidfdOpen(cmd.Process.Pid, 0)
	if err == nil {
		defer unix.Close(fd) //nolint:errcheck // The transferred duplicate has separate ownership in the parent test.
		err = sendStressDescriptor(socket, fd)
	}
	if err != nil {
		if killErr := cmd.Process.Kill(); killErr != nil {
			t.Errorf("stop unpublished child: %v", killErr)
		}
		if waitErr := cmd.Wait(); waitErr != nil {
			t.Logf("unpublished child exit: %v", waitErr)
		}
		t.Fatal(err)
	}
	if role == "wait" {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("controlled child ended before its invocation was canceled: %v", err)
		}
		return
	}
	if err := cmd.Process.Release(); err != nil {
		t.Fatal(err)
	}
}

func stressFixtureProgram(t *testing.T) string {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(program)
}
