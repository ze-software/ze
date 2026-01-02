package main

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// TestCmdEncode_BasicUnicast verifies basic IPv4 unicast encoding.
//
// VALIDATES: encode command produces valid BGP UPDATE hex for unicast routes.
// PREVENTS: Regression in encode command functionality.
func TestCmdEncode_BasicUnicast(t *testing.T) {
	// Capture output
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{"route 10.0.0.0/24 next-hop 192.168.1.1"}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Byte-level assertions:
	// [0:16]  - BGP marker (16x 0xFF)
	// [16:18] - Length (2 bytes)
	// [18]    - Type (0x02 = UPDATE)
	// [19:21] - Withdrawn routes length (0x0000)
	// [21:23] - Path attributes length

	if len(decoded) < 23 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}

	// Check BGP marker
	for i := 0; i < 16; i++ {
		if decoded[i] != 0xFF {
			t.Errorf("byte %d: expected 0xFF, got 0x%02X", i, decoded[i])
		}
	}

	// Check message type (UPDATE = 2)
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}

	// Check withdrawn routes length = 0
	if decoded[19] != 0x00 || decoded[20] != 0x00 {
		t.Errorf("withdrawn length: expected 0x0000, got 0x%02X%02X", decoded[19], decoded[20])
	}

	// Check NLRI contains prefix 10.0.0.0/24 (0x180A0000 = /24 + 10.0.0)
	// NLRI is at the end after path attributes
	nlriPattern := []byte{0x18, 0x0A, 0x00, 0x00} // /24 prefix, 10.0.0
	if !bytes.Contains(decoded, nlriPattern) {
		t.Errorf("NLRI should contain 10.0.0.0/24 (180A0000)")
	}

	// Check next-hop 192.168.1.1 (0xC0A80101) in attributes
	nextHopPattern := []byte{0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(decoded, nextHopPattern) {
		t.Errorf("attributes should contain next-hop 192.168.1.1 (C0A80101)")
	}
}

// TestCmdEncode_IPv6Unicast verifies IPv6 unicast encoding.
//
// VALIDATES: encode command handles IPv6 family correctly.
// PREVENTS: IPv6 encoding bugs.
func TestCmdEncode_IPv6Unicast(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{"-f", "ipv6 unicast", "route 2001:db8::/32 next-hop 2001:db8::1"}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}
}

// TestCmdEncode_NoHeader verifies --no-header flag.
//
// VALIDATES: --no-header flag excludes 19-byte BGP header.
// PREVENTS: Header stripping regression.
func TestCmdEncode_NoHeader(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{"--no-header", "route 10.0.0.0/24 next-hop 192.168.1.1"}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Should NOT start with BGP marker
	if strings.HasPrefix(output, "FFFFFFFF") {
		t.Error("output should not contain BGP marker when --no-header is used")
	}
}

// TestCmdEncode_NLRIOnly verifies -n flag for NLRI-only output.
//
// VALIDATES: -n flag outputs only NLRI bytes.
// PREVENTS: NLRI extraction regression.
func TestCmdEncode_NLRIOnly(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{"-n", "route 10.0.0.0/24 next-hop 192.168.1.1"}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// NLRI for 10.0.0.0/24 should be: 18 0A 00 00 (prefix-len + prefix bytes)
	// 24 bits = 0x18, then 10.0.0 = 0x0A 0x00 0x00
	expected := "180A0000"
	if output != expected {
		t.Errorf("expected NLRI %s, got %s", expected, output)
	}
}

// TestCmdEncode_MissingRoute verifies error handling for missing route.
//
// VALIDATES: Error returned when route argument is missing.
// PREVENTS: Silent failure on invalid input.
func TestCmdEncode_MissingRoute(t *testing.T) {
	var stderr bytes.Buffer
	oldStderr := encodeStderr
	encodeStderr = &stderr
	defer func() { encodeStderr = oldStderr }()

	args := []string{}
	exitCode := cmdEncode(args)

	if exitCode == 0 {
		t.Error("expected non-zero exit code for missing route")
	}
}

// TestCmdEncode_WithAttributes verifies encoding with path attributes.
//
// VALIDATES: Path attributes are correctly encoded.
// PREVENTS: Attribute encoding bugs.
func TestCmdEncode_WithAttributes(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"route 10.0.0.0/24 next-hop 192.168.1.1 origin igp local-preference 200 community [65000:100]",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex and longer than basic route
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}

	// Should be longer than header (19 bytes) - attributes add length
	if len(decoded) < 30 {
		t.Errorf("output too short for route with attributes: %d bytes", len(decoded))
	}
}

// TestCmdEncode_EVPN_Type2 verifies EVPN Type 2 (MAC/IP) encoding.
//
// VALIDATES: EVPN MAC/IP routes are correctly encoded.
// PREVENTS: EVPN encoding regression.
func TestCmdEncode_EVPN_Type2(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn evpn",
		"mac-ip rd 100:1 esi 0 etag 0 mac 00:11:22:33:44:55 label 100 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Byte-level assertions
	if len(decoded) < 23 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}

	// Check message type (UPDATE = 2)
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}

	// Check MAC address 00:11:22:33:44:55 appears in output
	macPattern := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	if !bytes.Contains(decoded, macPattern) {
		t.Errorf("NLRI should contain MAC 00:11:22:33:44:55")
	}

	// Check RD 100:1 (Type 0: 0x0000 + ASN 0x0064 + value 0x00000001)
	rdPattern := []byte{0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0x01}
	if !bytes.Contains(decoded, rdPattern) {
		t.Errorf("NLRI should contain RD 100:1")
	}
}

// TestCmdEncode_EVPN_Type5 verifies EVPN Type 5 (IP Prefix) encoding.
//
// VALIDATES: EVPN IP Prefix routes are correctly encoded.
// PREVENTS: EVPN Type 5 encoding bugs.
func TestCmdEncode_EVPN_Type5(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn evpn",
		"ip-prefix rd 100:1 esi 0 etag 0 prefix 10.0.0.0/24 gateway 0.0.0.0 label 100 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}
}

// TestCmdEncode_Stdin verifies reading route command from stdin.
//
// VALIDATES: encode reads from stdin when no args provided.
// PREVENTS: Stdin support regression.
func TestCmdEncode_Stdin(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	// Set up stdin
	stdinContent := "route 10.0.0.0/24 next-hop 192.168.1.1\n"
	oldStdin := encodeStdin
	encodeStdin = strings.NewReader(stdinContent)
	defer func() { encodeStdin = oldStdin }()

	// Mock TTY check to return false (stdin is a pipe)
	oldIsTTY := encodeStdinIsTTY
	encodeStdinIsTTY = func() bool { return false }
	defer func() { encodeStdinIsTTY = oldIsTTY }()

	args := []string{} // No args - should read from stdin
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex with BGP marker
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Should contain BGP marker
	if !strings.HasPrefix(output, strings.Repeat("FF", 16)) {
		t.Error("expected BGP marker at start of output")
	}
}

// TestCmdEncode_TTYShowsUsage verifies that usage is shown when stdin is TTY and no args.
//
// VALIDATES: Usage is shown instead of blocking when stdin is a terminal.
// PREVENTS: Blocking forever waiting for terminal input.
func TestCmdEncode_TTYShowsUsage(t *testing.T) {
	var stderr bytes.Buffer
	oldStderr := encodeStderr
	encodeStderr = &stderr
	defer func() { encodeStderr = oldStderr }()

	// Mock TTY check to return true (stdin is a terminal)
	oldIsTTY := encodeStdinIsTTY
	encodeStdinIsTTY = func() bool { return true }
	defer func() { encodeStdinIsTTY = oldIsTTY }()

	args := []string{} // No args, stdin is TTY - should show usage
	exitCode := cmdEncode(args)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "missing route command") {
		t.Errorf("expected 'missing route command' in stderr, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "Usage:") {
		t.Errorf("expected usage in stderr, got: %s", errOutput)
	}
}

// TestCmdEncode_StdinWithFamily verifies stdin with family flag.
//
// VALIDATES: Family flag works with stdin input.
// PREVENTS: Flag parsing issues with stdin.
func TestCmdEncode_StdinWithFamily(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	// Set up stdin
	stdinContent := "route 2001:db8::/32 next-hop 2001:db8::1\n"
	oldStdin := encodeStdin
	encodeStdin = strings.NewReader(stdinContent)
	defer func() { encodeStdin = oldStdin }()

	// Mock TTY check to return false (stdin is a pipe)
	oldIsTTY := encodeStdinIsTTY
	encodeStdinIsTTY = func() bool { return false }
	defer func() { encodeStdinIsTTY = oldIsTTY }()

	args := []string{"-f", "ipv6 unicast"} // Family flag but route from stdin
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}
}

// TestCmdEncode_LabeledUnicast verifies labeled unicast (nlri-mpls) encoding.
//
// VALIDATES: Labeled unicast routes are correctly encoded.
// PREVENTS: Labeled unicast encoding regression.
func TestCmdEncode_LabeledUnicast(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 nlri-mpls",
		"10.0.0.0/24 next-hop 192.168.1.1 label 100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Should contain BGP header
	if len(decoded) < 19 {
		t.Fatalf("output too short for BGP header: %d bytes", len(decoded))
	}
}

// TestCmdEncode_L3VPN verifies L3VPN (mpls-vpn) encoding.
//
// VALIDATES: L3VPN routes are correctly encoded.
// PREVENTS: L3VPN encoding regression.
func TestCmdEncode_L3VPN(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 mpls-vpn",
		"10.0.0.0/24 rd 100:1 next-hop 192.168.1.1 label 100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Should contain BGP header
	if len(decoded) < 19 {
		t.Fatalf("output too short for BGP header: %d bytes", len(decoded))
	}
}

// TestCmdEncode_L3VPN_IPv6 verifies IPv6 L3VPN encoding.
//
// VALIDATES: IPv6 VPN routes are correctly encoded.
// PREVENTS: IPv6 VPN encoding bugs.
func TestCmdEncode_L3VPN_IPv6(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv6 mpls-vpn",
		"2001:db8::/32 rd 100:1 next-hop 2001:db8::1 label 100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}
}

// TestCmdEncode_EVPN_Type1 verifies EVPN Type 1 (Ethernet Auto-Discovery) encoding.
//
// VALIDATES: EVPN Type 1 routes are correctly encoded.
// PREVENTS: EVPN Type 1 encoding regression.
func TestCmdEncode_EVPN_Type1(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn evpn",
		"ethernet-ad rd 100:1 esi 0 etag 0 label 100 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}
}

// TestCmdEncode_EVPN_Type3 verifies EVPN Type 3 (Inclusive Multicast) encoding.
//
// VALIDATES: EVPN Type 3 routes are correctly encoded.
// PREVENTS: EVPN Type 3 encoding regression.
func TestCmdEncode_EVPN_Type3(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn evpn",
		"multicast rd 100:1 etag 0 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}
}

// TestCmdEncode_EVPN_Type4 verifies EVPN Type 4 (Ethernet Segment) encoding.
//
// VALIDATES: EVPN Type 4 routes are correctly encoded.
// PREVENTS: EVPN Type 4 encoding regression.
func TestCmdEncode_EVPN_Type4(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn evpn",
		"ethernet-segment rd 100:1 esi 0 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}
}

// TestCmdEncode_EVPN_WithESI verifies EVPN encoding with non-zero ESI.
//
// VALIDATES: Non-zero ESI values are correctly encoded.
// PREVENTS: ESI encoding regression.
func TestCmdEncode_EVPN_WithESI(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn evpn",
		"mac-ip rd 100:1 esi 00:11:22:33:44:55:66:77:88:99 etag 0 mac 00:11:22:33:44:55 label 100 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Output should contain ESI bytes (0011223344556677889)
	if !strings.Contains(output, "00112233445566778899") {
		t.Error("output should contain ESI bytes")
	}
}

// Note: ESI parsing tests are in pkg/bgp/nlri/evpn_test.go (TestParseESIString)

// TestCmdEncode_ASN4False verifies 2-byte ASN encoding with --asn4=false.
//
// VALIDATES: AS_PATH uses 2-byte encoding when ASN4 is disabled.
// PREVENTS: ASN4 flag regression.
func TestCmdEncode_ASN4False(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"--asn4=false",
		"-a", "65001",
		"-z", "65002",
		"route 10.0.0.0/24 next-hop 192.168.1.1 as-path [65001 65002]",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}

	// With 2-byte ASN encoding:
	// AS_PATH segment: type(1) + count(1) + ASNs(2 bytes each)
	// Local AS (65001) is prepended, so path is [65001, 65001, 65002]
	// AS 65001 = 0xFDE9, AS 65002 = 0xFDEA
	// Segment: 0x02 (AS_SEQUENCE) + 0x03 (count) + FDE9 + FDE9 + FDEA
	asPath2Byte := []byte{0x02, 0x03, 0xFD, 0xE9, 0xFD, 0xE9, 0xFD, 0xEA}
	if !bytes.Contains(decoded, asPath2Byte) {
		t.Errorf("expected 2-byte AS_PATH encoding (02 03 FDE9 FDE9 FDEA), not found in output")
	}

	// With 4-byte ASN, each AS would be 4 bytes: 0000FDE9, 0000FDE9, 0000FDEA
	// Verify 4-byte pattern is NOT present
	asPath4Byte := []byte{0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA}
	if bytes.Contains(decoded, asPath4Byte) {
		t.Errorf("found 4-byte AS_PATH encoding when --asn4=false")
	}
}

// TestCmdEncode_AddPath verifies ADD-PATH encoding with -i flag.
//
// VALIDATES: Path ID is included in NLRI when ADD-PATH is enabled.
// PREVENTS: ADD-PATH flag regression.
func TestCmdEncode_AddPath(t *testing.T) {
	// First, encode without ADD-PATH
	var stdoutWithout bytes.Buffer
	encodeStdout = &stdoutWithout
	argsWithout := []string{"route 10.0.0.0/24 next-hop 192.168.1.1"}
	if code := cmdEncode(argsWithout); code != 0 {
		t.Fatalf("encode without ADD-PATH failed with code %d", code)
	}
	outputWithout := strings.TrimSpace(stdoutWithout.String())
	decodedWithout, _ := hex.DecodeString(outputWithout)

	// Now encode with ADD-PATH
	var stdoutWith bytes.Buffer
	encodeStdout = &stdoutWith
	argsWith := []string{"-i", "route 10.0.0.0/24 next-hop 192.168.1.1"}
	if code := cmdEncode(argsWith); code != 0 {
		t.Fatalf("encode with ADD-PATH failed with code %d", code)
	}
	outputWith := strings.TrimSpace(stdoutWith.String())
	decodedWith, err := hex.DecodeString(outputWith)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}

	// With ADD-PATH, the NLRI should be 4 bytes longer (path-id prefix)
	// Without ADD-PATH: prefix-len(1) + prefix-bytes(3) = 4 bytes for /24
	// With ADD-PATH: path-id(4) + prefix-len(1) + prefix-bytes(3) = 8 bytes
	lenDiff := len(decodedWith) - len(decodedWithout)
	if lenDiff != 4 {
		t.Errorf("expected ADD-PATH to add 4 bytes (path-id), got diff of %d bytes", lenDiff)
	}

	// Without ADD-PATH, NLRI is: 18 0A 00 00 (prefix-len + 10.0.0)
	// With ADD-PATH, NLRI is: 00 00 00 00 18 0A 00 00 (path-id=0 + prefix)
	nlriWithAddPath := []byte{0x00, 0x00, 0x00, 0x00, 0x18, 0x0A, 0x00, 0x00}
	if !bytes.Contains(decodedWith, nlriWithAddPath) {
		t.Errorf("expected ADD-PATH NLRI (00000000 18 0A0000), not found in output")
	}
}

// TestCmdEncode_FlowSpec_Discard verifies FlowSpec encoding with discard action.
//
// VALIDATES: FlowSpec routes with discard action are correctly encoded.
// PREVENTS: FlowSpec encoding regression.
func TestCmdEncode_FlowSpec_Discard(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then discard",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check message type (UPDATE = 2)
	if len(decoded) < 19 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}

	// FlowSpec NLRI should contain:
	// - Component type 1 (destination prefix)
	// - Prefix length 24 (0x18)
	// - Prefix bytes 10.0.0 (0x0A, 0x00, 0x00)
	destPrefixPattern := []byte{0x01, 0x18, 0x0A, 0x00, 0x00}
	if !bytes.Contains(decoded, destPrefixPattern) {
		t.Errorf("NLRI should contain destination 10.0.0.0/24 component (01 18 0A 00 00)")
	}
}

// TestCmdEncode_FlowSpec_DestPort verifies FlowSpec encoding with destination port.
//
// VALIDATES: FlowSpec routes with port matching are correctly encoded.
// PREVENTS: FlowSpec port component encoding bugs.
func TestCmdEncode_FlowSpec_DestPort(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 destination-port 80 then discard",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// FlowSpec NLRI should contain:
	// - Component type 5 (destination port)
	// - Port 80 (0x50)
	// The exact encoding depends on operator flags
	if len(decoded) < 19 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}
}

// TestCmdEncode_FlowSpec_IPv6 verifies IPv6 FlowSpec encoding.
//
// VALIDATES: IPv6 FlowSpec routes are correctly encoded.
// PREVENTS: IPv6 FlowSpec encoding bugs.
func TestCmdEncode_FlowSpec_IPv6(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv6 flowspec",
		"match destination 2001:db8::/32 then discard",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}
}

// TestCmdEncode_LabeledUnicast_IPv6 verifies IPv6 labeled unicast encoding.
//
// VALIDATES: IPv6 labeled unicast routes are correctly encoded.
// PREVENTS: IPv6 labeled unicast encoding bugs.
func TestCmdEncode_LabeledUnicast_IPv6(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv6 nlri-mpls",
		"2001:db8::/32 next-hop 2001:db8::1 label 100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}
}

// TestCmdEncode_L3VPN_RDType1 verifies L3VPN encoding with RD Type 1 (IP:Local).
//
// VALIDATES: RD Type 1 (IP address:local value) is correctly parsed and encoded.
// PREVENTS: RD Type 1 parsing bugs in full encode flow.
func TestCmdEncode_L3VPN_RDType1(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 mpls-vpn",
		"10.0.0.0/24 rd 1.2.3.4:100 next-hop 192.168.1.1 label 100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// RD Type 1 format: 0x0001 + 4-byte IP + 2-byte local
	// 1.2.3.4:100 = 0x0001 + 0x01020304 + 0x0064
	rdType1Pattern := []byte{0x00, 0x01, 0x01, 0x02, 0x03, 0x04, 0x00, 0x64}
	if !bytes.Contains(decoded, rdType1Pattern) {
		t.Errorf("UPDATE should contain RD Type 1 (1.2.3.4:100)")
	}
}

// TestCmdEncode_StdinEmpty verifies error handling for empty stdin input.
//
// VALIDATES: Empty stdin produces error, not hang.
// PREVENTS: Silent failure on empty piped input.
func TestCmdEncode_StdinEmpty(t *testing.T) {
	var stderr bytes.Buffer
	oldStderr := encodeStderr
	encodeStderr = &stderr
	defer func() { encodeStderr = oldStderr }()

	// Set up empty stdin
	oldStdin := encodeStdin
	encodeStdin = strings.NewReader("")
	defer func() { encodeStdin = oldStdin }()

	// Mock TTY check to return false (stdin is a pipe)
	oldIsTTY := encodeStdinIsTTY
	encodeStdinIsTTY = func() bool { return false }
	defer func() { encodeStdinIsTTY = oldIsTTY }()

	args := []string{} // No args - should read from stdin
	exitCode := cmdEncode(args)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1 for empty stdin, got %d", exitCode)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "missing route command") {
		t.Errorf("expected 'missing route command' error, got: %s", errOutput)
	}
}

// TestCmdEncode_StdinInvalid verifies error handling for invalid stdin input.
//
// VALIDATES: Invalid stdin input produces clear error.
// PREVENTS: Confusing error messages for bad piped input.
func TestCmdEncode_StdinInvalid(t *testing.T) {
	var stderr bytes.Buffer
	oldStderr := encodeStderr
	encodeStderr = &stderr
	defer func() { encodeStderr = oldStderr }()

	// Set up invalid stdin
	oldStdin := encodeStdin
	encodeStdin = strings.NewReader("not-a-valid-command\n")
	defer func() { encodeStdin = oldStdin }()

	// Mock TTY check to return false (stdin is a pipe)
	oldIsTTY := encodeStdinIsTTY
	encodeStdinIsTTY = func() bool { return false }
	defer func() { encodeStdinIsTTY = oldIsTTY }()

	args := []string{} // No args - should read from stdin
	exitCode := cmdEncode(args)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1 for invalid stdin, got %d", exitCode)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "error") {
		t.Errorf("expected error message, got: %s", errOutput)
	}
}

// TestCmdEncode_MUP_ISD verifies MUP ISD (Interwork Segment Discovery) encoding.
//
// VALIDATES: MUP ISD routes are correctly encoded.
// PREVENTS: MUP ISD encoding regression.
func TestCmdEncode_MUP_ISD(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 mup",
		"mup-isd 10.0.0.0/24 rd 100:1 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check message type (UPDATE = 2)
	if len(decoded) < 19 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}
}

// TestCmdEncode_MUP_T1ST verifies MUP T1ST (Type 1 Session Transformed) encoding.
//
// VALIDATES: MUP T1ST routes are correctly encoded.
// PREVENTS: MUP T1ST encoding regression.
func TestCmdEncode_MUP_T1ST(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv6 mup",
		"mup-t1st 2001:db8::/32 rd 100:1 next-hop 2001:db8::1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	_, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}
}

// TestCmdEncode_VPLS verifies VPLS encoding.
//
// VALIDATES: VPLS routes are correctly encoded.
// PREVENTS: VPLS encoding regression.
func TestCmdEncode_VPLS(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "l2vpn vpls",
		"rd 100:1 ve-block-offset 0 ve-block-size 10 label 100 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	// Output should be valid hex
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check message type (UPDATE = 2)
	if len(decoded) < 19 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}
}

// TestCmdEncode_MUP_DSD verifies MUP DSD (Direct Segment Discovery) encoding.
//
// VALIDATES: MUP DSD routes are correctly encoded.
// PREVENTS: MUP DSD encoding regression.
func TestCmdEncode_MUP_DSD(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 mup",
		"mup-dsd 192.168.1.100 rd 100:1 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check message type (UPDATE = 2)
	if len(decoded) < 19 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}

	// Check DSD address 192.168.1.100 (0xC0A80164) appears in output
	dsdAddr := []byte{0xC0, 0xA8, 0x01, 0x64}
	if !bytes.Contains(decoded, dsdAddr) {
		t.Errorf("NLRI should contain DSD address 192.168.1.100 (C0A80164)")
	}
}

// TestCmdEncode_MUP_T2ST verifies MUP T2ST (Type 2 Session Transformed) encoding.
//
// VALIDATES: MUP T2ST routes are correctly encoded.
// PREVENTS: MUP T2ST encoding regression.
func TestCmdEncode_MUP_T2ST(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 mup",
		"mup-t2st 192.168.1.100 rd 100:1 next-hop 192.168.1.1",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check message type (UPDATE = 2)
	if len(decoded) < 19 {
		t.Fatalf("output too short: %d bytes", len(decoded))
	}
	if decoded[18] != 0x02 {
		t.Errorf("message type: expected 0x02 (UPDATE), got 0x%02X", decoded[18])
	}

	// Check T2ST endpoint 192.168.1.100 (0xC0A80164) appears in output
	t2stAddr := []byte{0xC0, 0xA8, 0x01, 0x64}
	if !bytes.Contains(decoded, t2stAddr) {
		t.Errorf("NLRI should contain T2ST endpoint 192.168.1.100 (C0A80164)")
	}
}

// TestCmdEncode_FlowSpec_Redirect_2ByteASN verifies FlowSpec redirect with 2-byte ASN.
//
// VALIDATES: Redirect action with 2-byte ASN is correctly encoded.
// PREVENTS: Redirect encoding regression.
func TestCmdEncode_FlowSpec_Redirect_2ByteASN(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 65000:100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check for 2-byte ASN redirect extended community
	// Type 0x80, Subtype 0x08, ASN 65000 (0xFDE8), value 100 (0x00000064)
	redirectEC := []byte{0x80, 0x08, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	if !bytes.Contains(decoded, redirectEC) {
		t.Errorf("expected 2-byte ASN redirect extended community (80 08 FDE8 00000064)")
	}
}

// TestCmdEncode_FlowSpec_Redirect_4ByteASN verifies FlowSpec redirect with 4-byte ASN.
//
// VALIDATES: Redirect action with 4-byte ASN is correctly encoded per RFC 7674.
// PREVENTS: 4-byte ASN redirect encoding bugs.
func TestCmdEncode_FlowSpec_Redirect_4ByteASN(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 4200000000:100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())

	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v\noutput: %s", err, output)
	}

	// Check for 4-byte ASN redirect extended community (RFC 7674)
	// Type 0x82, Subtype 0x08, ASN 4200000000 (0xFA56EA00), value 100 (0x0064)
	redirectEC := []byte{0x82, 0x08, 0xFA, 0x56, 0xEA, 0x00, 0x00, 0x64}
	if !bytes.Contains(decoded, redirectEC) {
		t.Errorf("expected 4-byte ASN redirect extended community (82 08 FA56EA00 0064)")
	}
}

// TestCmdEncode_FlowSpec_Redirect_Boundary_65535 verifies ASN 65535 uses 2-byte format.
//
// VALIDATES: Maximum 2-byte ASN uses 2-byte format (Type 0x80).
// PREVENTS: Off-by-one error at ASN boundary.
func TestCmdEncode_FlowSpec_Redirect_Boundary_65535(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 65535:100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}

	// ASN 65535 (0xFFFF) should use 2-byte format
	// Type 0x80, Subtype 0x08, ASN 0xFFFF, value 100 (0x00000064)
	redirectEC := []byte{0x80, 0x08, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x64}
	if !bytes.Contains(decoded, redirectEC) {
		t.Errorf("ASN 65535 should use 2-byte format (80 08 FFFF 00000064)")
	}
}

// TestCmdEncode_FlowSpec_Redirect_Boundary_65536 verifies ASN 65536 uses 4-byte format.
//
// VALIDATES: First 4-byte ASN uses 4-byte format (Type 0x82).
// PREVENTS: Off-by-one error at ASN boundary.
func TestCmdEncode_FlowSpec_Redirect_Boundary_65536(t *testing.T) {
	var stdout bytes.Buffer
	oldStdout := encodeStdout
	encodeStdout = &stdout
	defer func() { encodeStdout = oldStdout }()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 65536:100",
	}
	exitCode := cmdEncode(args)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	decoded, err := hex.DecodeString(output)
	if err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}

	// ASN 65536 (0x00010000) should use 4-byte format
	// Type 0x82, Subtype 0x08, ASN 0x00010000, value 100 (0x0064)
	redirectEC := []byte{0x82, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64}
	if !bytes.Contains(decoded, redirectEC) {
		t.Errorf("ASN 65536 should use 4-byte format (82 08 00010000 0064)")
	}
}

// TestCmdEncode_FlowSpec_Redirect_4ByteASN_LargeTarget verifies error for invalid redirect.
//
// VALIDATES: 4-byte ASN redirect with target > 65535 returns error.
// PREVENTS: Silent failure on invalid redirect configuration.
func TestCmdEncode_FlowSpec_Redirect_4ByteASN_LargeTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	oldStdout := encodeStdout
	oldStderr := encodeStderr
	encodeStdout = &stdout
	encodeStderr = &stderr
	defer func() {
		encodeStdout = oldStdout
		encodeStderr = oldStderr
	}()

	// 4-byte ASN (4200000000) with target > 65535 (70000) should fail
	// RFC 7674 limits local admin to 16 bits for 4-byte ASN format
	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 4200000000:70000",
	}
	exitCode := cmdEncode(args)

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for invalid redirect")
	}

	// Verify error message mentions the limit
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "16-bit") && !strings.Contains(errOutput, "redirect") {
		t.Errorf("expected error about 16-bit limit for 4-byte ASN redirect, got: %s", errOutput)
	}
}

// TestCmdEncode_FlowSpec_Redirect_MalformedNoColon verifies error for redirect without colon.
//
// VALIDATES: Redirect without colon separator returns error.
// PREVENTS: Panic or silent failure on malformed input.
func TestCmdEncode_FlowSpec_Redirect_MalformedNoColon(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	oldStdout := encodeStdout
	oldStderr := encodeStderr
	encodeStdout = &stdout
	encodeStderr = &stderr
	defer func() {
		encodeStdout = oldStdout
		encodeStderr = oldStderr
	}()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 65000",
	}
	exitCode := cmdEncode(args)

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for malformed redirect")
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "invalid redirect") {
		t.Errorf("expected error about invalid redirect format, got: %s", errOutput)
	}
}

// TestCmdEncode_FlowSpec_Redirect_NegativeASN verifies error for negative ASN.
//
// VALIDATES: Negative ASN in redirect returns error.
// PREVENTS: Underflow or unexpected behavior.
func TestCmdEncode_FlowSpec_Redirect_NegativeASN(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	oldStdout := encodeStdout
	oldStderr := encodeStderr
	encodeStdout = &stdout
	encodeStderr = &stderr
	defer func() {
		encodeStdout = oldStdout
		encodeStderr = oldStderr
	}()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect -1:100",
	}
	exitCode := cmdEncode(args)

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for negative ASN")
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "invalid") {
		t.Errorf("expected error about invalid ASN, got: %s", errOutput)
	}
}

// TestCmdEncode_FlowSpec_Redirect_ASNOverflow verifies error for ASN > max uint32.
//
// VALIDATES: ASN exceeding 4294967295 returns error.
// PREVENTS: Integer overflow.
func TestCmdEncode_FlowSpec_Redirect_ASNOverflow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	oldStdout := encodeStdout
	oldStderr := encodeStderr
	encodeStdout = &stdout
	encodeStderr = &stderr
	defer func() {
		encodeStdout = oldStdout
		encodeStderr = oldStderr
	}()

	args := []string{
		"-f", "ipv4 flowspec",
		"match destination 10.0.0.0/24 then redirect 5000000000:100",
	}
	exitCode := cmdEncode(args)

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for ASN overflow")
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "invalid") {
		t.Errorf("expected error about invalid ASN, got: %s", errOutput)
	}
}
