// Design: docs/research/l2tpv2-ze-integration.md -- NCP coordinator, IPCP DNS options
// Related: ncp.go -- buildNakOrReject, ipcpOptionServed

// VALIDATES: an IPCP Configure-Request asking for a DNS server the session holds
// none for draws a Configure-Reject of that option alone, while a slot the pool
// filled is still Nak'd with its server and then Acked.
// PREVENTS: the IPCP loop where Ze Nak'd the request without the option it ruled
// unacceptable, so the subscriber resent the same Configure-Request forever.
// METHOD: drives a real session through ncpTestDriver with an IP responder that
// fills only the DNS slots each case names, then plays the subscriber.
package ppp

import (
	"bytes"
	"net/netip"
	"testing"
)

// acceptIPWithDNS returns an IP responder that assigns the stock IPv4 addresses
// and exactly the DNS servers given. An invalid netip.Addr leaves that slot empty,
// as a pool with no server for it does.
func acceptIPWithDNS(primary, secondary netip.Addr) func(*Driver) {
	return func(d *Driver) {
		for ev := range d.IPEventsOut() {
			req, ok := ev.(EventIPRequest)
			if !ok {
				continue
			}
			args := IPResponseArgs{Accept: true, Family: req.Family}
			switch req.Family {
			case AddressFamilyIPv4:
				args.Local = ipcpTestLocal
				args.Peer = ipcpTestPeer
				args.DNSPrimary = primary
				args.DNSSecondary = secondary
			case AddressFamilyIPv6:
			default:
				panic("BUG: unknown NCP request address family")
			}
			_ = d.IPResponse(req.TunnelID, req.SessionID, args) //nolint:errcheck // ignore teardown race
		}
	}
}

// readIPCPReply reads IPCP packets until one is not a Configure-Request, so a
// legitimate resend of Ze's own request does not stand in for its answer. The
// first answer is returned whatever its code, so a Nak where a Reject is owed
// fails the caller's assertion instead of being skipped.
func readIPCPReply(t *testing.T, td *ncpTestDriver) LCPPacket {
	t.Helper()
	for range 6 {
		pkt := td.readPeerNCPPacket(t, ProtoIPCP)
		if pkt.Code != LCPConfigureRequest {
			return pkt
		}
	}
	t.Fatal("no IPCP reply after 6 reads")
	return LCPPacket{}
}

// ipcpOpt encodes one IPv4-valued IPCP option.
func ipcpOpt(optType uint8, addr netip.Addr) []byte {
	a4 := addr.As4()
	return []byte{optType, 6, a4[0], a4[1], a4[2], a4[3]}
}

// startIPCPPeer opens a session whose pool filled the given DNS slots and Acks
// Ze's own IPCP Configure-Request, leaving the peer's half to the caller.
func startIPCPPeer(t *testing.T, primary, secondary netip.Addr) *ncpTestDriver {
	t.Helper()
	td := newNCPTestDriverIP(t, &StartSession{DisableIPv6CP: true}, acceptIPWithDNS(primary, secondary))
	cr := td.readPeerNCPPacket(t, ProtoIPCP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("got code %d, want Ze's Configure-Request", cr.Code)
	}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)
	return td
}

func TestIPCPDNSRequestWithoutServer(t *testing.T) {
	zero := netip.IPv4Unspecified()
	none := netip.Addr{}
	primaryReq := ipcpOpt(IPCPOptPrimaryDNS, zero)
	secondaryReq := ipcpOpt(IPCPOptSecondaryDNS, zero)
	request := bytes.Join([][]byte{ipcpOpt(IPCPOptIPAddress, ipcpTestPeer), primaryReq, secondaryReq}, nil)

	cases := []struct {
		name       string
		primary    netip.Addr
		secondary  netip.Addr
		wantReject []byte
	}{
		{"secondary not configured", ipcpTestDNS1, none, secondaryReq},
		{"primary not configured", none, ipcpTestDNS2, primaryReq},
		{"neither configured", none, none, bytes.Join([][]byte{primaryReq, secondaryReq}, nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := startIPCPPeer(t, tc.primary, tc.secondary)

			td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x40, request)
			reply := readIPCPReply(t, td)
			if reply.Code != LCPConfigureReject {
				t.Fatalf("reply code %d (% x), want Configure-Reject", reply.Code, reply.Data)
			}
			if !bytes.Equal(reply.Data, tc.wantReject) {
				t.Fatalf("Configure-Reject Data = % x, want only the unserved DNS options % x", reply.Data, tc.wantReject)
			}

			// The subscriber drops the rejected options. What is left draws
			// the Nak for each served slot, then the Ack.
			retry := ipcpOpt(IPCPOptIPAddress, ipcpTestPeer)
			final := ipcpOpt(IPCPOptIPAddress, ipcpTestPeer)
			if tc.primary.IsValid() {
				retry = append(retry, primaryReq...)
				final = append(final, ipcpOpt(IPCPOptPrimaryDNS, tc.primary)...)
			}
			if tc.secondary.IsValid() {
				retry = append(retry, secondaryReq...)
				final = append(final, ipcpOpt(IPCPOptSecondaryDNS, tc.secondary)...)
			}
			if !bytes.Equal(retry, final) {
				td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x41, retry)
				nak := readIPCPReply(t, td)
				if nak.Code != LCPConfigureNak {
					t.Fatalf("retry reply code %d (% x), want Configure-Nak", nak.Code, nak.Data)
				}
				opts, err := ParseIPCPOptions(nak.Data)
				if err != nil {
					t.Fatalf("Nak options: %v", err)
				}
				if opts.HasPrimary != tc.primary.IsValid() || opts.PrimaryDNS != tc.primary {
					t.Errorf("Nak Primary-DNS = %v (present %v), want %v", opts.PrimaryDNS, opts.HasPrimary, tc.primary)
				}
				if opts.HasSecondary != tc.secondary.IsValid() || opts.SecondaryDNS != tc.secondary {
					t.Errorf("Nak Secondary-DNS = %v (present %v), want %v", opts.SecondaryDNS, opts.HasSecondary, tc.secondary)
				}
			}
			td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x42, final)
			ack := readIPCPReply(t, td)
			if ack.Code != LCPConfigureAck {
				t.Fatalf("final reply code %d (% x), want Configure-Ack", ack.Code, ack.Data)
			}
			if !bytes.Equal(ack.Data, final) {
				t.Errorf("Configure-Ack Data = % x, want % x", ack.Data, final)
			}
		})
	}
}

// TestIPCPDNSRequestBothServed is the control: with both slots filled, the same
// request draws a Nak carrying both servers, and the corrected request an Ack.
func TestIPCPDNSRequestBothServed(t *testing.T) {
	zero := netip.IPv4Unspecified()
	td := startIPCPPeer(t, ipcpTestDNS1, ipcpTestDNS2)

	request := bytes.Join([][]byte{
		ipcpOpt(IPCPOptIPAddress, ipcpTestPeer),
		ipcpOpt(IPCPOptPrimaryDNS, zero),
		ipcpOpt(IPCPOptSecondaryDNS, zero),
	}, nil)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x50, request)
	nak := readIPCPReply(t, td)
	if nak.Code != LCPConfigureNak {
		t.Fatalf("reply code %d (% x), want Configure-Nak", nak.Code, nak.Data)
	}
	opts, err := ParseIPCPOptions(nak.Data)
	if err != nil {
		t.Fatalf("Nak options: %v", err)
	}
	if !opts.HasPrimary || opts.PrimaryDNS != ipcpTestDNS1 {
		t.Errorf("Nak Primary-DNS = %v (present %v), want %v", opts.PrimaryDNS, opts.HasPrimary, ipcpTestDNS1)
	}
	if !opts.HasSecondary || opts.SecondaryDNS != ipcpTestDNS2 {
		t.Errorf("Nak Secondary-DNS = %v (present %v), want %v", opts.SecondaryDNS, opts.HasSecondary, ipcpTestDNS2)
	}

	final := bytes.Join([][]byte{
		ipcpOpt(IPCPOptIPAddress, ipcpTestPeer),
		ipcpOpt(IPCPOptPrimaryDNS, ipcpTestDNS1),
		ipcpOpt(IPCPOptSecondaryDNS, ipcpTestDNS2),
	}, nil)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x51, final)
	ack := readIPCPReply(t, td)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("final reply code %d (% x), want Configure-Ack", ack.Code, ack.Data)
	}
	if !bytes.Equal(ack.Data, final) {
		t.Errorf("Configure-Ack Data = % x, want % x", ack.Data, final)
	}
}
