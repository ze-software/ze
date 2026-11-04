// Design: docs/architecture/testing/runner-architecture.md -- native lifecycle proof.

package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

const ownedPeerOpen = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D01040001005A0A141E2800"
const ownedPeerKeepalive = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"
const ownedRunnerStop = "stop"
const ownedPeerWireName = "wire"

// ownedPeerLinger drives the real runner in this process so wait4 can distinguish
// a reaped child from an exited zombie. Each route completes an OPEN expectation,
// answers two later wire challenges, then ends only when named stop reaches it.
func ownedPeerLinger(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("owned-peer-linger takes no arguments")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(sessionpath.EnsureScratchRoot(root), "owned-peer-linger-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture-owned scratch only
	for _, route := range []string{fieldFile, pipeCommand} {
		if err := ownedPeerRoute(ctx, base, route); err != nil {
			return fmt.Errorf("%s route: %w", route, err)
		}
	}
	fmt.Println("file-and-pipe: BGP live until stop; children reaped; lease released")
	return nil
}

func ownedPeerRoute(ctx context.Context, base, route string) error {
	tests := runner.NewEncodingTests(base)
	rec := tests.Add("owned-peer-" + route)
	rec.Activate()
	rec.Extra = map[string]string{fieldTimeout: "20s"}
	rec.Conf = map[string]any{}
	rec.StdinBlocks = map[string][]byte{fieldPeer: []byte("option=open:value=inspect-open-message\noption=linger:value=true\nexpect=bgp:conn=1:seq=1:hex=" + ownedPeerOpen + "\n")}
	peerExec := "le test peer --port $PORT"
	if route == pipeCommand {
		peerExec += " -"
	}
	marker := filepath.Join(base, route+"-closed")
	zero := 0
	rec.ExpectExitCode = &zero
	rec.RunCommands = []runner.RunCommand{
		{Mode: "background", Seq: 1, Name: fieldPeer, Ready: "listening on", Exec: peerExec, Stdin: fieldPeer},
		{Mode: "background", Seq: 2, Name: ownedPeerWireName, Ready: "wire-live", Exec: "le test fixture runner/owned-peer-wire $PORT " + marker},
		{Mode: "stop", Seq: 3, Name: fieldPeer, Signal: "term"},
		{Mode: "foreground", Seq: 4, Exec: "le test fixture runner/owned-peer-closed " + marker, ExitCode: &zero},
		{Mode: "stop", Seq: 5, Name: ownedPeerWireName, Signal: "term"},
	}
	r, err := runner.NewRunner(tests, base)
	if err != nil {
		return err
	}
	defer r.Cleanup()
	if !r.Run(ctx, &runner.RunOptions{Timeout: 25 * time.Second, Parallel: 1, Quiet: true, SkipTimings: true}) {
		return fmt.Errorf("native run failed: %w; peer output: %s", rec.Error, rec.PeerOutput)
	}
	if !strings.Contains(rec.PeerOutput, "lingering: holding the session open") {
		return fmt.Errorf("peer did not enter completed linger: %s", rec.PeerOutput)
	}
	// The runner's trace records only successful starts, including their PID.
	pidPattern := regexp.MustCompile(`s pid ([0-9]+),`)
	children := 0
	for _, step := range rec.StepTrace {
		match := pidPattern.FindStringSubmatch(step.Assert)
		if match == nil {
			continue
		}
		pid, err := strconv.Atoi(match[1])
		if err != nil {
			return err
		}
		var status syscall.WaitStatus
		got, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if !errors.Is(err, syscall.ECHILD) {
			return fmt.Errorf("child %d not reaped at return: wait4=%d, error=%v", pid, got, err)
		}
		children++
	}
	if children != 3 {
		return fmt.Errorf("checked %d started children, want 3", children)
	}
	if _, err := os.Stat(rec.WorkDir); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("work directory not released: %v", err)
	}
	lease, err := runner.LeaseTestPorts(rec.Port)
	if err != nil {
		return err
	}
	defer lease.Release()
	if lease.Start != rec.Port {
		return fmt.Errorf("lease %d still held; got %d", rec.Port, lease.Start)
	}
	return nil
}

// ownedPeerWire consumes the handshake before challenging the completed peer.
// The second reply cannot be a buffered handshake KEEPALIVE. Readiness permits
// the runner to stop the peer; EOF then proves that stop closed the same socket.
func ownedPeerWire(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("owned-peer-wire needs port and closure marker")
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", args[0]))
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // the assertion reads closure before returning
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	open, err := hex.DecodeString(ownedPeerOpen)
	if err != nil {
		return err
	}
	if _, err := conn.Write(open); err != nil {
		return err
	}
	if err := ownedPeerFrame(conn, 1); err != nil {
		return fmt.Errorf("OPEN reply: %w", err)
	}
	if err := ownedPeerFrame(conn, 4); err != nil {
		return fmt.Errorf("handshake KEEPALIVE: %w", err)
	}
	keepalive, err := hex.DecodeString(ownedPeerKeepalive)
	if err != nil {
		return err
	}
	for range 2 {
		if _, err := conn.Write(keepalive); err != nil {
			return fmt.Errorf("linger challenge: %w", err)
		}
		if err := ownedPeerFrame(conn, 4); err != nil {
			return fmt.Errorf("linger reply: %w", err)
		}
	}
	fmt.Println("wire-live")
	var one [1]byte
	n, err := conn.Read(one[:])
	if n != 0 {
		return fmt.Errorf("unexpected byte after stop: %x", one[:n])
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("peer did not close at stop: %w", err)
	}
	return os.WriteFile(args[1], []byte("peer-closed\n"), 0o600)
}

func ownedPeerFrame(conn net.Conn, kind byte) error {
	var header [19]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return err
	}
	if !bytes.Equal(header[:16], bytes.Repeat([]byte{255}, 16)) {
		return fmt.Errorf("invalid BGP marker: %x", header[:16])
	}
	if header[18] != kind {
		return fmt.Errorf("BGP kind %d, want %d", header[18], kind)
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	if length < len(header) {
		return fmt.Errorf("short BGP frame: %d", length)
	}
	if kind == 4 {
		if length != len(header) {
			return fmt.Errorf("KEEPALIVE length %d, want 19", length)
		}
	}
	_, err := io.CopyN(io.Discard, conn, int64(length-len(header)))
	return err
}

func ownedPeerClosed(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("owned-peer-closed needs the closure marker")
	}
	if !Poll(ctx, 100, 20*time.Millisecond, func() bool {
		data, err := os.ReadFile(args[0])
		return err == nil && string(data) == "peer-closed\n"
	}) {
		return errors.New("wire client did not observe peer closure after named stop")
	}
	return nil
}
