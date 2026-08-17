// VALIDATES: the vendored nclient4 raw receive path still refuses an IPv4
// frame whose payload cannot hold a UDP header.
// PREVENTS: a downgrade of github.com/insomniacslk/dhcp, or a re-vendor from an
// older pin, silently restoring the negative slice bound that panicked the DHCP
// receive loop and killed the process.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	nclient4ConnPath = "vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/conn_unix.go"
	nclient4Recovery = "go get github.com/insomniacslk/dhcp@latest && go mod vendor"
)

// nclient4GuardLines are the statements that make the DHCP length non-negative.
// BroadcastRawUDPConn.ReadFrom subtracts the UDP header length from the IPv4
// payload length, and ipv4.isValid accepts a payload of 0 to 7 bytes, so
// without these the subtraction goes negative.
var nclient4GuardLines = []string{
	"ipPayloadLen := int(ipHdr.payloadLength())",
	"if ipPayloadLen < udpHdrLen {",
	"dhcpLen := ipPayloadLen - udpHdrLen",
	"if !buf.Has(dhcpLen) {",
}

// nclient4PanicLine is the pre-fix subtraction. uio.Lexer.Consume reaches
// Buffer.ReadN with the result, whose Has check passes for a negative n, and
// the slice bound then panics on a goroutine nclient4 owns, which no recover in
// ze can catch.
const nclient4PanicLine = "dhcpLen := int(ipHdr.payloadLength()) - udpHdrLen"

// TestNclient4ShortIPv4PayloadGuard keeps the vendored guard in the tree ze
// compiles. The behavior it protects is asserted against a running client by
// TestDHCPv4SurvivesShortIPv4Payload
// (internal/plugins/iface/dhcp/dhcp_shortframe_integration_linux_test.go).
func TestNclient4ShortIPv4PayloadGuard(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, filepath.FromSlash(nclient4ConnPath))
	data, err := os.ReadFile(path) //nolint:gosec // a fixed in-repo path
	if err != nil {
		t.Fatalf("read %s: %v", nclient4ConnPath, err)
	}

	lines := make(map[string]bool, 64)
	for line := range strings.SplitSeq(string(data), "\n") {
		lines[strings.TrimSpace(line)] = true
	}

	for _, want := range nclient4GuardLines {
		if !lines[want] {
			t.Fatalf("%s no longer carries %q: the vendored pin predates the "+
				"BroadcastRawUDPConn.ReadFrom short-payload fix, so a frame "+
				"declaring fewer than eight IPv4 payload bytes panics the DHCP "+
				"receive loop.\nRecovery: %s",
				nclient4ConnPath, want, nclient4Recovery)
		}
	}
	if lines[nclient4PanicLine] {
		t.Fatalf("%s carries the pre-fix line %q, which computes a negative "+
			"DHCP length.\nRecovery: %s",
			nclient4ConnPath, nclient4PanicLine, nclient4Recovery)
	}
}
