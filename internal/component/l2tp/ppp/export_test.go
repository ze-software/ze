package ppp

import (
	"io"
	"net"
	"os"
	"testing"
)

// These wrappers are protocol-test fixtures only. Production ownership and
// descriptor construction live in ppp/kernel; synthetic protocol descriptors
// resolve to their registered net.Pipe without entering the kernel adapter.
var newChanFileFn = func(fd int, name string) io.ReadWriteCloser {
	return os.NewFile(uintptr(fd), name)
}

var newUnitFileFn = func(fd int) io.ReadCloser {
	if fd <= 0 {
		return nil
	}
	return newChanFileFn(fd, "ppp.unit")
}

// SetNewChanFileForTest replaces the channel wrapper used by testSessionStart.
// Tests use it to substitute a net.Pipe end (or any io.ReadWriteCloser)
// so the Driver can be exercised without /dev/ppp. Returns the previous
// function so the test can restore via defer. A session started with
// CaptureProxyLCP gets the same file: a net.Pipe end has read deadlines,
// which QuiesceSession uses to wake its reader.
func SetNewChanFileForTest(fn func(fd int, name string) io.ReadWriteCloser) func(int, string) io.ReadWriteCloser {
	prev := newChanFileFn
	newChanFileFn = fn
	return prev
}

// RestoreNewChanFile restores the previous test wrapper.
func RestoreNewChanFile(prev func(fd int, name string) io.ReadWriteCloser) {
	newChanFileFn = prev
}

// SetNewUnitFileForTest replaces the unit fd wrapper. Returns previous.
func SetNewUnitFileForTest(fn func(int) io.ReadCloser) func(int) io.ReadCloser {
	prev := newUnitFileFn
	newUnitFileFn = fn
	return prev
}

// RestoreNewUnitFile restores the previous test wrapper.
func RestoreNewUnitFile(prev func(int) io.ReadCloser) {
	newUnitFileFn = prev
}

// NewPipeDriverForTest returns a started Driver with no auth responder, whose
// session on chanFD reads and writes the driver end of a net.Pipe, and the
// peer end of that pipe. An external test package (ppp_test) uses it to drive
// a session through a real auth handler, which package ppp cannot import
// without a cycle. The Driver is stopped and the peer end closed at cleanup.
func NewPipeDriverForTest(t *testing.T, chanFD int) (*Driver, net.Conn) {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, chanFD)
	t.Cleanup(func() { closeConn(pair.peerEnd) })

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(d.Stop)
	return d, pair.peerEnd
}

// BindSessionForTest gives an external-package protocol test the same created
// session resources as NewPipeDriverForTest. It keeps the actual driver queue
// and authentication boundary, without exposing the fixture's private recorder.
func BindSessionForTest(driver *Driver, start StartSession, channel, unit, number int) StartSession {
	return testStartForDriver(driver, start, channel, unit, number)
}
