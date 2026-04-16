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
