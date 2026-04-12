package bmp

import "testing"

func TestRiboutShouldSendNewPath(t *testing.T) {
	// VALIDATES: AC-35 -- new NLRI is sent
	r := newRibout()
	if !r.shouldSend("10.0.0.0/24", 12345) {
		t.Error("new NLRI should be sent")
	}
}

func TestRiboutShouldSendDuplicate(t *testing.T) {
	// VALIDATES: AC-35 -- identical path not re-sent
	r := newRibout()
	r.shouldSend("10.0.0.0/24", 12345)
	if r.shouldSend("10.0.0.0/24", 12345) {
		t.Error("duplicate path should be suppressed")
	}
}

func TestRiboutShouldSendChanged(t *testing.T) {
	// VALIDATES: AC-35 -- changed path is sent
	r := newRibout()
	r.shouldSend("10.0.0.0/24", 12345)
	if !r.shouldSend("10.0.0.0/24", 67890) {
		t.Error("changed path should be sent")
	}
}

func TestRiboutWithdrawKnown(t *testing.T) {
	// VALIDATES: AC-35 -- withdraw for known NLRI is sent
	r := newRibout()
	r.shouldSend("10.0.0.0/24", 12345)
	if !r.shouldSend("10.0.0.0/24", 0) {
		t.Error("withdraw for known NLRI should be sent")
	}
}

func TestRiboutWithdrawUnknown(t *testing.T) {
	// VALIDATES: AC-35 -- withdraw for unknown NLRI is suppressed
	r := newRibout()
	if r.shouldSend("10.0.0.0/24", 0) {
		t.Error("withdraw for unknown NLRI should be suppressed")
	}
}

func TestRiboutClear(t *testing.T) {
	r := newRibout()
	r.shouldSend("10.0.0.0/24", 12345)
	r.clear()
	// After clear, same hash should be sent again.
	if !r.shouldSend("10.0.0.0/24", 12345) {
		t.Error("after clear, path should be sent again")
	}
}

func TestFnvHash(t *testing.T) {
	h1 := fnvHash([]byte("hello"))
	h2 := fnvHash([]byte("hello"))
	h3 := fnvHash([]byte("world"))

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if h1 == 0 {
		t.Error("hash should not be zero for non-empty input")
	}
}
