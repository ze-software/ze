package ppp

import "io"

// SetNewChanFileForTest replaces the chan fd wrapper used by spawnSession.
// Tests use it to substitute a net.Pipe end (or any io.ReadWriteCloser)
// so the Driver can be exercised without /dev/ppp. Returns the previous
// function so the test can restore via defer.
func SetNewChanFileForTest(fn func(fd int, name string) io.ReadWriteCloser) func(int, string) io.ReadWriteCloser {
	prev := newChanFileFn
	newChanFileFn = fn
	return prev
}

// RestoreNewChanFile resets the wrapper to production.
func RestoreNewChanFile(prev func(fd int, name string) io.ReadWriteCloser) {
	newChanFileFn = prev
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
