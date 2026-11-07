package ppp

import (
	"io"
	"net"
	"testing"
)

// SetNewChanFileForTest replaces the chan fd wrapper used by spawnSession.
// Tests use it to substitute a net.Pipe end (or any io.ReadWriteCloser)
// so the Driver can be exercised without /dev/ppp. Returns the previous
// function so the test can restore via defer. A session started with
// CaptureProxyLCP gets the same file: a net.Pipe end has read deadlines,
// which QuiesceSession uses to wake its reader.
func SetNewChanFileForTest(fn func(fd int, name string) io.ReadWriteCloser) func(int, string) io.ReadWriteCloser {
	prev := newChanFileFn
	newChanFileFn = fn
	newQuiescableChanFileFn = func(fd int, name string) (io.ReadWriteCloser, error) { return fn(fd, name), nil }
	return prev
}

// RestoreNewChanFile resets the wrapper to production.
func RestoreNewChanFile(prev func(fd int, name string) io.ReadWriteCloser) {
	newChanFileFn = prev
	newQuiescableChanFileFn = newInterruptibleFDFile
}

// SetNewUnitFileForTest replaces the unit fd wrapper. Returns previous.
func SetNewUnitFileForTest(fn func(int) io.ReadCloser) func(int) io.ReadCloser {
	prev := newUnitFileFn
	newUnitFileFn = fn
	return prev
}

// RestoreNewUnitFile resets the unit fd wrapper to production.
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
