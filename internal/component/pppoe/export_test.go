package pppoe

// SetNowUnix overrides the clock for testing. Caller must restore
// the original value after the test.
func SetNowUnix(fn func() int64) (restore func()) {
	orig := nowUnix
	nowUnix = fn
	return func() { nowUnix = orig }
}
