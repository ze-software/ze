//go:build !linux

package transport

import "testing"

// VALIDATES: the fail-closed half of the RFC 5880 Section 6.8.6 demux input on a
// platform with no IP_PKTINFO. The receiver cannot learn which of its own
// addresses a packet reached, and it says so by answering with an INVALID
// address rather than a plausible one.
// PREVENTS: a later edit substituting the socket's bind address here, which is
// the wildcard and would read as a real local address at every call site. That
// is the silently-wrong value ai/rules/principles.md forbids, and it is exactly
// the defect this whole path was fixed for on Linux.
func TestParseReceivedPktinfoAnswersUnknownOffLinux(t *testing.T) {
	addr, ifindex := parseReceivedPktinfo([]byte{1, 2, 3, 4})
	if addr.IsValid() {
		t.Errorf("local = %v, want an invalid address: this platform cannot know it", addr)
	}
	if ifindex != 0 {
		t.Errorf("ifindex = %d, want 0", ifindex)
	}
}
