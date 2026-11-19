// Design: docs/architecture/l2tp/bng-5-pppoe.md -- interrupted native receive
// Related: socket_linux.go -- EINTR retries inside the same poll callback

//go:build linux

package vpp

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// An interrupted receive has not consumed readiness. Polling again can block
// indefinitely, even though retrying the same native receive would succeed.
func TestVPPNativeSocketRetriesInterruptedReceiveBeforePolling(t *testing.T) {
	original := receiveControlDatagram
	t.Cleanup(func() { receiveControlDatagram = original })
	calls := 0
	frame := []byte{0xc0, 0x21, 9, 1, 0, 4}
	receiveControlDatagram = func(fd int, buf, control []byte, flags int) (int, int, int, unix.Sockaddr, error) {
		calls++
		if fd != 73 || len(control) != 0 || flags != unix.MSG_DONTWAIT|unix.MSG_TRUNC {
			t.Fatalf("receive arguments: fd=%d, control=%d, flags=%d", fd, len(control), flags)
		}
		if calls == 1 {
			return 0, 0, 0, nil, unix.EINTR
		}
		return copy(buf, frame), 0, 0, nil, nil
	}
	raw := &singleCallbackRawConn{}
	socket := &socketFile{raw: raw}
	var buf [32]byte
	n, err := socket.Read(buf[:])
	if err != nil || n != len(frame) || string(buf[:n]) != string(frame) {
		t.Fatalf("interrupted receive n=%d, frame=%x, err=%v", n, buf[:n], err)
	}
	if calls != 2 || raw.polls != 0 {
		t.Fatalf("receive calls=%d, polls=%d", calls, raw.polls)
	}
}

// EAGAIN does require a readiness wait. Retrying that error in the callback
// would spin and deny the runtime poller its chance to wait for fresh data.
func TestVPPNativeSocketWouldBlockWaitsForReadiness(t *testing.T) {
	original := receiveControlDatagram
	t.Cleanup(func() { receiveControlDatagram = original })
	calls := 0
	receiveControlDatagram = func(_ int, buf, _ []byte, _ int) (int, int, int, unix.Sockaddr, error) {
		calls++
		if calls == 1 {
			return 0, 0, 0, nil, unix.EAGAIN
		}
		buf[0] = 0xc0
		return 1, 0, 0, nil, nil
	}
	raw := &readinessCallbackRawConn{}
	var buf [32]byte
	n, err := (&socketFile{raw: raw}).Read(buf[:])
	if err != nil || n != 1 || buf[0] != 0xc0 || calls != 2 || raw.polls != 1 {
		t.Fatalf("would-block receive n=%d, frame=%x, calls=%d, polls=%d, err=%v", n, buf[:n], calls, raw.polls, err)
	}
}

type readinessCallbackRawConn struct{ singleCallbackRawConn }

func (r *readinessCallbackRawConn) Read(callback func(uintptr) bool) error {
	if callback(73) {
		return errors.New("would-block receive did not wait for readiness")
	}
	r.polls++
	if !callback(73) {
		return errors.New("ready receive waited again")
	}
	return nil
}

type singleCallbackRawConn struct{ polls int }

func (r *singleCallbackRawConn) Control(callback func(uintptr)) error {
	callback(73)
	return nil
}
func (r *singleCallbackRawConn) Read(callback func(uintptr) bool) error {
	if !callback(73) {
		r.polls++
		return errors.New("interrupted receive attempted another readiness wait")
	}
	return nil
}
func (r *singleCallbackRawConn) Write(callback func(uintptr) bool) error {
	if !callback(73) {
		return errors.New("unexpected blocked write")
	}
	return nil
}
