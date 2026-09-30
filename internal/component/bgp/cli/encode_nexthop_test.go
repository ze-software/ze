package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestCmdEncodeRefusesIPv4RouteWithIPv6NextHop proves `ze bgp encode` reports
// the builder's refusal of an IPv4 route with an IPv6 next hop.
//
// VALIDATES: exit code 1, nothing on stdout, and stderr carrying
// message.ErrUnicastNextHopUnusable's text.
// PREVENTS: the command printing an UPDATE whose NLRI has no NEXT_HOP attribute,
// which it did before BuildUnicast refused the input.
func TestCmdEncodeRefusesIPv4RouteWithIPv6NextHop(t *testing.T) {
	var stdout, stderr bytes.Buffer
	oldStdout, oldStderr := encodeStdout, encodeStderr
	encodeStdout, encodeStderr = &stdout, &stderr
	defer func() { encodeStdout, encodeStderr = oldStdout, oldStderr }()

	exitCode := cmdEncode([]string{"route 10.0.0.0/24 next-hop 2001:db8::1"})

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stdout %q", exitCode, stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("a refused route still printed %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), message.ErrUnicastNextHopUnusable.Error()) {
		t.Fatalf("stderr %q does not report %q", stderr.String(), message.ErrUnicastNextHopUnusable)
	}
}
