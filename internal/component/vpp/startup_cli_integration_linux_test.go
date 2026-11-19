//go:build integration && linux

// Design: docs/guide/vpp.md -- original managed bootstrap failure cleanup
// Related: vpp.go -- actual runOnce binding, barrier, original Wait and joins
// This lifecycle model owns a real child and real Unix API transport. Its
// typed CLI response is fixture data; it proves no VPP plugin or forwarding.
package vpp

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
	"go.fd.io/govpp/codec"
	"golang.org/x/sys/unix"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 3 {
		if os.Args[1] == "-c" {
			if strings.HasSuffix(os.Args[2], "managed-bootstrap-child.pid") {
				if err := startupCLIChild(os.Args[2]); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
	}
	os.Exit(m.Run())
}

// The same compiled test executable creates the listener inside the child
// whose atomic birth descriptor runOnce retains. No shell or extra executable
// is required. Every request/reply uses the existing typed native codec.
func startupCLIChild(path string) (failure error) {
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: "api.sock", Net: "unix"})
	if err != nil {
		return err
	}
	defer func() { failure = errors.Join(failure, listener.Close()) }()
	output, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { failure = errors.Join(failure, output.Close()) }()
	if _, err := fmt.Fprintln(output, os.Getpid()); err != nil {
		return err
	}
	connection, err := listener.AcceptUnix()
	if err != nil {
		return err
	}
	defer func() { failure = errors.Join(failure, connection.Close()) }()
	createFrame, err := readSocketTransportFrame(connection)
	if err != nil {
		return err
	}
	if binary.BigEndian.Uint16(createFrame[:2]) != 15 {
		return errors.New("bootstrap fixture expected native create")
	}
	create := new(memclnt.SockclntCreate)
	if err := codec.DefaultCodec.DecodeMsg(createFrame, create); err != nil {
		return err
	}
	table := []memclnt.MessageTableEntry{
		{Index: 42, Name: "sockclnt_delete_8ac76db6"},
		{Index: 43, Name: (&memclnt.ControlPing{}).GetMessageName() + "_" + (&memclnt.ControlPing{}).GetCrcString()},
		{Index: 44, Name: (&memclnt.ControlPingReply{}).GetMessageName() + "_" + (&memclnt.ControlPingReply{}).GetCrcString()},
		{Index: 45, Name: (&startupCLIRequest{}).GetMessageName() + "_" + (&startupCLIRequest{}).GetCrcString()},
		{Index: 46, Name: (&startupCLIReply{}).GetMessageName() + "_" + (&startupCLIReply{}).GetCrcString()},
	}
	if err := writeStartupCLIFrame(connection, &memclnt.SockclntCreateReply{Index: 123, MessageTable: table}, 16, binary.BigEndian.Uint32(createFrame[2:6])); err != nil {
		return err
	}
	release, err := os.Open(path + ".stop")
	if err != nil {
		return err
	}
	defer func() { failure = errors.Join(failure, release.Close()) }()
	for {
		request, err := readSocketTransportFrame(connection)
		if err != nil {
			return err
		}
		if len(request) < 10 {
			return errors.New("bootstrap fixture short native request")
		}
		context := binary.BigEndian.Uint32(request[6:10])
		switch binary.BigEndian.Uint16(request[:2]) {
		case 43:
			ping := new(memclnt.ControlPing)
			if err := codec.DefaultCodec.DecodeMsg(request, ping); err != nil {
				return err
			}
			if err := writeStartupCLIFrame(connection, &memclnt.ControlPingReply{VpePID: uint32(os.Getpid())}, 44, context); err != nil {
				return err
			}
		case 45:
			command := new(startupCLIRequest)
			if err := codec.DefaultCodec.DecodeMsg(request, command); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(output, "BOOTSTRAP "+command.Cmd); err != nil {
				return err
			}
			// Every duplicate command independently requires a parent receipt. A replay
			// blocks the actual runOnce result and fails its unchanged three-second bound.
			var receipt [1]byte
			if _, err := io.ReadFull(release, receipt[:]); err != nil {
				return err
			}
			if err := writeStartupCLIFrame(connection, &startupCLIReply{Retval: -7, Reply: "original startup CLI receive failed"}, 46, context); err != nil {
				return err
			}
		case 42:
			return nil
		default:
			return fmt.Errorf("bootstrap fixture unexpected native ID %d", binary.BigEndian.Uint16(request[:2]))
		}
	}
}
func writeStartupCLIFrame(connection *net.UnixConn, message api.Message, id uint16, context uint32) error {
	body, err := codec.DefaultCodec.EncodeMsg(message, id)
	if err != nil {
		return err
	}
	if len(body) < 6 {
		return errors.New("bootstrap fixture short native reply")
	}
	binary.BigEndian.PutUint32(body[2:6], context)
	var header [16]byte
	binary.BigEndian.PutUint32(header[8:12], uint32(len(body)))
	frame := append(header[:], body...)
	n, err := connection.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

// Exact kernel-confirmed peer death authorizes the asynchronous notification.
// Separately, the original cmd.Wait/reap and all runOnce-owned joins MUST finish
// before runOnce returns. ECHILD in the callback would overconstrain the approved
// physical observer; ECHILD at the joined result discriminates a missing Wait.
func TestVPPManagedBootstrapRunOnceFailureWaitsBeforeRetirement(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	pidPath := filepath.Join(root, "managed-bootstrap-child.pid")
	stopPath := pidPath + ".stop"
	for _, path := range []string{pidPath, stopPath} {
		if err := unix.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fd, err := unix.Open(pidPath, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	pidFile := os.NewFile(uintptr(fd), pidPath)
	if pidFile == nil {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
		t.Fatal("PID FIFO returned no file")
	}
	t.Cleanup(func() {
		if err := pidFile.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := pidFile.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	releaseFD, err := unix.Open(stopPath, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	releaseFile := os.NewFile(uintptr(releaseFD), stopPath)
	if releaseFile == nil {
		if err := unix.Close(releaseFD); err != nil {
			t.Error(err)
		}
		t.Fatal("release FIFO returned no file")
	}
	t.Cleanup(func() {
		if err := releaseFile.Close(); err != nil {
			t.Error(err)
		}
	})
	child, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := filepath.Join(root, "original.cli")
	if err := os.WriteFile(bootstrap, []byte("show version\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := defaultTestSettings()
	settings.APISocket = "api.sock"
	settings.StartupCLIFile = bootstrap
	settings.Stats.SocketPath = filepath.Join(root, "stats-must-not-be-opened")
	statsPath := settings.Stats.SocketPath
	// Preserve W01: the ordinary sentinel reaches a real immediate Unixpacket dial
	// refusal if bootstrap is bypassed, without the stats client's missing-file wait.
	const statsSentinel = "not a native statistics socket\n"
	if err := os.WriteFile(statsPath, []byte(statsSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if settings.Stats.SocketPath != statsPath {
			t.Error("original statistics pathname changed", settings.Stats.SocketPath)
		}
		contents, err := os.ReadFile(statsPath)
		if err != nil {
			t.Error("original statistics fixture disappeared", err)
			return
		}
		if string(contents) != statsSentinel {
			t.Error("original statistics fixture changed", string(contents))
		}
	})
	manager := NewVPPManager(settings, root, child)
	owner := manager.connector
	startup := make(chan error, 1)
	manager.startup = startup
	identity := make(chan int, 1)
	entered := make(chan string, 1)
	feedbackDone := make(chan error, 1)
	var nativeCommands atomic.Int32
	readFeedback := func() {
		defer close(feedbackDone)
		reader := bufio.NewScanner(pidFile)
		if !reader.Scan() {
			feedbackDone <- fmt.Errorf("original child PID: %w", reader.Err())
			return
		}
		pid, err := strconv.Atoi(reader.Text())
		if err != nil {
			feedbackDone <- fmt.Errorf("original child PID %q: %w", reader.Text(), err)
			return
		}
		if pid <= 0 {
			feedbackDone <- fmt.Errorf("original child PID %q must be positive", reader.Text())
			return
		}
		identity <- pid
		for reader.Scan() {
			line := reader.Text()
			if !strings.HasPrefix(line, "BOOTSTRAP ") {
				feedbackDone <- fmt.Errorf("unexpected original child receipt %q", line)
				return
			}
			if nativeCommands.Add(1) == 1 {
				entered <- strings.TrimPrefix(line, "BOOTSTRAP ")
			}
		}
		feedbackDone <- reader.Err()
	}
	var custody sync.Mutex
	handle := -1
	closed := false
	var peerLease *ProcessLease
	var generation atomic.Uint64
	var retirements atomic.Int32
	notified := make(chan struct{}, 1)
	observer := fmt.Sprintf("bootstrap-order-%d", nextConnectorIdentity())
	if err := RegisterProcessExitObserver(observer, func(proof ProcessExit) {
		expected := generation.Load()
		if expected == 0 || !proof.Covers(owner.Identity(), expected) {
			return
		}
		custody.Lock()
		defer custody.Unlock()
		if closed {
			return
		}
		retirements.Add(1)
		if peerLease == nil {
			t.Error("physical notification preceded original retained peer")
			return
		}
		if err := peerLease.ValidateRetirement(proof); err != nil {
			t.Error("notification had foreign physical proof", err)
		}
		observed, err := peerLease.Exit()
		if err != nil || observed == nil {
			t.Error("notification preceded kernel-confirmed original peer death", err)
		}
		if handle < 0 {
			t.Error("notification has no original child handle")
			return
		}
		descriptors := []unix.PollFd{{Fd: int32(handle), Events: unix.POLLIN}}
		if _, err := unix.Poll(descriptors, 0); err != nil {
			t.Error(err)
		}
		if descriptors[0].Revents&unix.POLLIN == 0 {
			t.Error("notification preceded original physical child death")
		}
		select {
		case notified <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { processObservers.Lock(); delete(processObservers.byName, observer); processObservers.Unlock() })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { defer close(result); result <- manager.runOnce(ctx, pidPath) }()
	t.Cleanup(func() {
		cancel()
		custody.Lock()
		if handle >= 0 {
			if err := unix.PidfdSendSignal(handle, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
				t.Error(err)
			}
		}
		custody.Unlock()
		joined := false
		select {
		case <-result:
			joined = true
		case <-time.After(3 * time.Second):
			t.Error("original runOnce worker did not join")
		}
		custody.Lock()
		closed = true
		original := handle
		handle = -1
		lease := peerLease
		peerLease = nil
		if original >= 0 {
			// Sole fallback reap custody transfers only AFTER actual runOnce joins.
			if joined {
				var status unix.Siginfo
				if err := unix.Waitid(unix.P_PIDFD, original, &status, unix.WEXITED|unix.WNOHANG, nil); err != nil && !errors.Is(err, unix.ECHILD) {
					t.Error(err)
				}
			}
			if err := unix.Close(original); err != nil {
				t.Error(err)
			}
		}
		custody.Unlock()
		owner.Close()
		if lease != nil {
			if err := lease.Close(); err != nil {
				t.Error(err)
			}
		}
		// End and join the sole FIFO reader on success and every assertion failure.
		if err := pidFile.SetReadDeadline(time.Now()); err != nil {
			t.Error(err)
		}
		select {
		case err := <-feedbackDone:
			if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("original feedback reader did not join")
		}
	})
	go readFeedback()
	select {
	case command := <-entered:
		if command != "exec "+bootstrap {
			t.Fatal("original bootstrap path changed", command)
		}
	case err := <-startup:
		t.Fatal("actual runOnce bypassed the managed bootstrap: early startup receipt", err)
	case err := <-result:
		t.Fatal("actual runOnce bypassed the managed bootstrap", err)
	case err := <-feedbackDone:
		t.Fatal("original native peer did not reach bootstrap", err)
	case <-time.After(3 * time.Second):
		t.Fatal("actual runOnce never reached bootstrap")
	}
	select {
	case err := <-startup:
		t.Fatal("actual runOnce published readiness before native completion", err)
	default:
	}
	if retirements.Load() != 0 {
		t.Fatal("held native startup retired its still-live original child")
	}
	select {
	case pid := <-identity:
		if pid <= 0 {
			t.Fatal("invalid original child PID")
		}
	default:
		t.Fatal("bootstrap preceded original child diagnostic identity")
	}
	originalGeneration, err := owner.ConnectionGeneration()
	if err != nil {
		t.Fatal(err)
	}
	lease, err := owner.RetainProcessForGeneration(originalGeneration)
	if err != nil {
		t.Fatal(err)
	}
	// Duplicate the retained actual SO_PEERPIDFD handle under its original owner
	// mutex. No reported PID lookup can race cmd.Wait or confer identity here.
	witness := lease.witness
	witness.mu.Lock()
	descriptor, ok := witness.descriptor.(*linuxProcessDescriptor)
	if !ok {
		witness.mu.Unlock()
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("original peer is not a genuine Linux descriptor")
	}
	originalHandle, err := unix.Dup(descriptor.fd)
	witness.mu.Unlock()
	if err != nil {
		if closeErr := lease.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	unix.CloseOnExec(originalHandle)
	custody.Lock()
	peerLease = lease
	handle = originalHandle
	generation.Store(originalGeneration)
	custody.Unlock()
	custody.Lock()
	aliveErr := unix.PidfdSendSignal(handle, 0, nil, 0)
	custody.Unlock()
	if aliveErr != nil {
		t.Fatal("held original child was not alive", aliveErr)
	}
	if _, err := releaseFile.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "native CLI result -7: original startup CLI receive failed") || !strings.Contains(err.Error(), bootstrap) {
			t.Fatal("actual runOnce lost original native bootstrap result/output/path", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("failed native bootstrap did not reap original child")
	}
	custody.Lock()
	var status unix.Siginfo
	reapErr := unix.Waitid(unix.P_PIDFD, handle, &status, unix.WEXITED|unix.WNOHANG|unix.WNOWAIT, nil)
	custody.Unlock()
	if !errors.Is(reapErr, unix.ECHILD) {
		t.Fatal("runOnce returned before original child's completed Wait", reapErr)
	}
	select {
	case <-notified:
	case <-time.After(3 * time.Second):
		t.Fatal("original physical peer death was not notified")
	}
	if retirements.Load() != 1 {
		t.Fatal("original bootstrap failure did not notify exact physical death once", retirements.Load())
	}
	if nativeCommands.Load() != 1 {
		t.Fatal("actual runOnce duplicated startup CLI", nativeCommands.Load())
	}
	if owner.IsConnected() {
		t.Fatal("failed original bootstrap retained its connector")
	}
	if manager.ready {
		t.Fatal("failed original bootstrap set readiness")
	}
}
