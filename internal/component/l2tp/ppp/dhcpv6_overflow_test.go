// Design: docs/research/l2tpv2-ze-integration.md -- DHCPv6-PD codec for BNG PPP sessions
// RFC: rfc/short/rfc3633.md -- DHCPv6 Prefix Delegation (IA_PD / IA_Prefix)
//
// VALIDATES: the DHCPv6 reply builders are capacity-checked -- a reply that would
// overflow the caller's fixed 512-byte buffer is dropped (error, not truncated,
// no panic) and the checked path is byte-for-byte identical to the raw builder
// for a fitting reply.
// PREVENTS: an oversized DHCPv6 reply panicking with a slice-bounds error inside
// BuildDHCPv6Reply (spec-fixit-fixed-buffer-overflow, AC-2/AC-3).
package ppp

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
)

// TestDHCPv6ReplyOversizedRejected drives the real service entry point
// (HandleDHCPv6 -> handleSolicit) with a server DUID far larger than the fixed
// 512-byte reply buffer. Before the bound this panics inside BuildDHCPv6Reply;
// after it, the service returns an error and no bytes, and the DHCPv6 server
// loop logs the error and skips the send.
func TestDHCPv6ReplyOversizedRejected(t *testing.T) {
	svc := newIPv6Service(iPv6ServiceConfig{})

	// A server DUID whose ID dwarfs the 512-byte reply buffer.
	hugeServerID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: make([]byte, 600)}
	msg := &dHCPv6Message{
		Type:     DHCPv6Solicit,
		ClientID: &DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{1, 2, 3, 4, 5, 6}},
		IAPD:     &DHCPv6IAPD{IAID: 0x11223344},
	}
	alloc := func() (netip.Prefix, bool) { return netip.MustParsePrefix("2001:db8:abcd::/56"), true }

	var (
		resp     []byte
		err      error
		didPanic bool
	)
	func() {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()
		resp, err = svc.handleDHCPv6(msg, hugeServerID, alloc)
	}()

	if didPanic {
		t.Fatal("HandleDHCPv6 panicked on an oversized reply; the bound is missing")
	}
	if err == nil {
		t.Fatal("oversized reply must return an error, got nil")
	}
	if resp != nil {
		t.Fatalf("oversized reply must not be returned, got %d bytes", len(resp))
	}
	if !strings.Contains(err.Error(), "needs") {
		t.Errorf("error must name the required length, got %v", err)
	}
}

// TestDHCPv6ReplyBytesUnchanged proves the checked builders are byte-for-byte
// identical to the raw builders for fitting replies, and that the length
// helpers match what the raw builders write (AC-3 + drift guard).
func TestDHCPv6ReplyBytesUnchanged(t *testing.T) {
	replyCfg := dHCPv6ReplyConfig{
		Type:          DHCPv6Reply,
		TransactionID: [3]byte{0x01, 0x02, 0x03},
		ServerID:      DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}},
		ClientID:      &DHCPv6DUID{Type: DUIDTypeLLT, HWType: 1, Time: 0x12345678, ID: []byte{1, 2, 3, 4, 5, 6}},
		IAID:          0x11223344,
		Prefix:        netip.MustParsePrefix("2001:db8:1234::/56"),
		PrefLifetime:  3600,
		ValidLifetime: 7200,
		T1:            1800,
		T2:            2880,
	}

	raw := make([]byte, 512)
	nRaw := buildDHCPv6Reply(raw, replyCfg)
	chk := make([]byte, 512)
	nChk, err := checkedBuildDHCPv6Reply(chk, replyCfg)
	if err != nil {
		t.Fatalf("CheckedBuildDHCPv6Reply on a fitting reply errored: %v", err)
	}
	if nRaw != nChk || !bytes.Equal(raw[:nRaw], chk[:nChk]) {
		t.Fatalf("checked reply bytes differ from raw (raw=%d checked=%d)", nRaw, nChk)
	}
	if got := dhcpv6ReplyLen(replyCfg); got != nRaw {
		t.Fatalf("dhcpv6ReplyLen=%d but BuildDHCPv6Reply wrote %d", got, nRaw)
	}

	statusCfg := dHCPv6StatusReplyConfig{
		TransactionID: [3]byte{0x09, 0x08, 0x07},
		ServerID:      DHCPv6DUID{Type: DUIDTypeEN, EnterpriseNum: 0xdeadbeef, ID: []byte{1, 2, 3}},
		ClientID:      &DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{4, 5, 6}},
		StatusCode:    D6StatusNoPrefixAvail,
		StatusMessage: "pool exhausted",
	}

	sRaw := make([]byte, 512)
	snRaw := buildDHCPv6StatusReply(sRaw, statusCfg)
	sChk := make([]byte, 512)
	snChk, err := checkedBuildDHCPv6StatusReply(sChk, statusCfg)
	if err != nil {
		t.Fatalf("CheckedBuildDHCPv6StatusReply on a fitting reply errored: %v", err)
	}
	if snRaw != snChk || !bytes.Equal(sRaw[:snRaw], sChk[:snChk]) {
		t.Fatalf("checked status bytes differ from raw (raw=%d checked=%d)", snRaw, snChk)
	}
	if got := dhcpv6StatusReplyLen(statusCfg); got != snRaw {
		t.Fatalf("dhcpv6StatusReplyLen=%d but BuildDHCPv6StatusReply wrote %d", got, snRaw)
	}
}

// TestDHCPv6LenMatchesBuildAllDUIDTypes pins the length helpers to the raw
// builders across every DUID type, since duidLen mirrors writeDUID's type
// switch and a mismatch would let CheckedBuild mis-size the buffer.
func TestDHCPv6LenMatchesBuildAllDUIDTypes(t *testing.T) {
	duids := map[string]DHCPv6DUID{
		"LLT":     {Type: DUIDTypeLLT, HWType: 1, Time: 42, ID: []byte{1, 2, 3, 4}},
		"EN":      {Type: DUIDTypeEN, EnterpriseNum: 99, ID: []byte{5, 6}},
		"LL":      {Type: DUIDTypeLL, HWType: 1, ID: []byte{7, 8, 9, 10, 11, 12}},
		"unknown": {Type: 99, ID: []byte{13, 14, 15}},
	}
	for name, duid := range duids {
		t.Run(name, func(t *testing.T) {
			cfg := dHCPv6ReplyConfig{
				Type:          DHCPv6Reply,
				TransactionID: [3]byte{1, 2, 3},
				ServerID:      duid,
				ClientID:      &duid,
				IAID:          7,
				Prefix:        netip.MustParsePrefix("2001:db8::/48"),
			}
			buf := make([]byte, 1024)
			if got, want := dhcpv6ReplyLen(cfg), buildDHCPv6Reply(buf, cfg); got != want {
				t.Fatalf("dhcpv6ReplyLen=%d but BuildDHCPv6Reply wrote %d", got, want)
			}
		})
	}
}
