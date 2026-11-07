// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client IPCP DNS options.
package pppoeclient

import (
	"bytes"
	"encoding/hex"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// ipcpAddressOption is an IP-Address option (RFC 1332 Section 3.3) carrying addr.
func ipcpAddressOption(addr netip.Addr) []byte {
	a := addr.As4()
	return []byte{ppp.IPCPOptIPAddress, 6, a[0], a[1], a[2], a[3]}
}

// ipcpRequestFor is the option list of a client request whose IP-Address
// option is address and whose two DNS options still ask with 0.0.0.0.
func ipcpRequestFor(address []byte) []byte {
	return append(bytes.Clone(address),
		ppp.IPCPOptPrimaryDNS, 6, 0, 0, 0, 0,
		ppp.IPCPOptSecondaryDNS, 6, 0, 0, 0, 0)
}

// dnsOption is one RFC 1877 DNS option of type optType carrying addr.
func dnsOption(optType uint8, addr string) []byte {
	a := netip.MustParseAddr(addr).As4()
	return []byte{optType, 6, a[0], a[1], a[2], a[3]}
}

type ipcpOutcome struct {
	result ipcpResult
	err    error
}

// startIPCP runs negotiateIPCP against frames and answers its log and outcome.
func startIPCP(t *testing.T, frames chan readFrame, stop chan struct{}) (*frameLog, chan ipcpOutcome) {
	t.Helper()
	w := &frameLog{}
	done := make(chan ipcpOutcome, 1)
	reader := testFrameReader(t, frames, nil)
	go func() {
		var buf [ppp.MaxFrameBufLen]byte
		result, err := negotiateIPCP(w, reader, buf[:], 1, 0, nil, new(uint8), stop, slog.Default(), new(uint8))
		done <- ipcpOutcome{result, err}
	}()
	return w, done
}

// finishIPCP Acknowledges the latest request and opens the peer direction.
func finishIPCP(t *testing.T, w *frameLog, frames chan readFrame, done chan ipcpOutcome) ipcpOutcome {
	t.Helper()
	synctest.Wait()
	last := recordedIPCP(t, w, len(w.snapshot())-1)
	frames <- ipcpFrame(ppp.LCPConfigureAck, last.Identifier, last.Data)
	frames <- ipcpFrame(ppp.LCPConfigureRequest, 90, ipcpAddressOption(netip.MustParseAddr("192.0.2.1")))
	return <-done
}

// TestRFC1877ClientRequestsDNSOptions: the first IPCP Configure-Request asks
// for an address and both DNS servers, each with 0.0.0.0, in that order.
//
// VALIDATES: AC-1, RFC 1877 Sections 1.1 and 1.3 request form.
// PREVENTS: a client that never asks and so never learns a resolver.
func TestRFC1877ClientRequestsDNSOptions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		w, _ := startIPCP(t, frames, stop)
		synctest.Wait()
		got := hex.EncodeToString(w.snapshot()[0])
		want := "8021" + "0101" + "0016" + "0306" + "00000000" + "8106" + "00000000" + "8306" + "00000000"
		if got != want {
			t.Fatalf("first IPCP request = %s, want %s", got, want)
		}
	})
}

// TestRFC1877ClientTakesDNSFromNak: the servers a Configure-Nak names are
// requested next with a new Identifier and recorded once Acknowledged, and a
// Nak naming 0.0.0.0 records nothing.
//
// VALIDATES: AC-2, AC-10.
// PREVENTS: learned servers lost between Nak and Ack, and an unusable
// 0.0.0.0 server reaching resolv.conf.
func TestRFC1877ClientTakesDNSFromNak(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		w, done := startIPCP(t, frames, stop)
		synctest.Wait()
		first := recordedIPCP(t, w, 0)
		nak := append(ipcpAddressOption(netip.MustParseAddr("192.0.2.2")),
			append(dnsOption(ppp.IPCPOptPrimaryDNS, "192.0.2.53"), dnsOption(ppp.IPCPOptSecondaryDNS, "192.0.2.54")...)...)
		frames <- ipcpFrame(ppp.LCPConfigureNak, first.Identifier, nak)
		synctest.Wait()
		next := recordedIPCP(t, w, 1)
		if next.Identifier == first.Identifier {
			t.Fatalf("request after Nak kept Identifier %d", next.Identifier)
		}
		if !bytes.Equal(next.Data, nak) {
			t.Fatalf("request after Nak = % x, want % x", next.Data, nak)
		}
		got := finishIPCP(t, w, frames, done)
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.result.primaryDNS != netip.MustParseAddr("192.0.2.53") {
			t.Fatalf("primary DNS = %v", got.result.primaryDNS)
		}
		if got.result.secondaryDNS != netip.MustParseAddr("192.0.2.54") {
			t.Fatalf("secondary DNS = %v", got.result.secondaryDNS)
		}
	})
	t.Run("zero value", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			w, done := startIPCP(t, frames, stop)
			synctest.Wait()
			first := recordedIPCP(t, w, 0)
			nak := append(ipcpAddressOption(netip.MustParseAddr("192.0.2.2")),
				append(dnsOption(ppp.IPCPOptPrimaryDNS, "0.0.0.0"), dnsOption(ppp.IPCPOptSecondaryDNS, "192.0.2.54")...)...)
			frames <- ipcpFrame(ppp.LCPConfigureNak, first.Identifier, nak)
			got := finishIPCP(t, w, frames, done)
			if got.err != nil {
				t.Fatal(got.err)
			}
			if got.result.primaryDNS.IsValid() {
				t.Fatalf("a Nak of 0.0.0.0 recorded primary DNS %v", got.result.primaryDNS)
			}
			if got.result.secondaryDNS != netip.MustParseAddr("192.0.2.54") {
				t.Fatalf("secondary DNS = %v", got.result.secondaryDNS)
			}
		})
	})
}

// TestRFC1661ClientDropsRejectedDNSOptions: a Configure-Reject of DNS options
// removes exactly those options from the next request, and IPCP still opens.
//
// RFC requirement: RFC1661-5.4-5 positive -- after the server Rejects Primary-DNS-Address and Secondary-DNS-Address, the client's next IPCP Configure-Request carries IP-Address only, and after a Reject of Secondary-DNS-Address alone it carries IP-Address and Primary-DNS-Address; IPCP then opens with no server learned for a Rejected option.
// VALIDATES: AC-3, AC-4.
// PREVENTS: a session ended, or a Rejected option resent, against an access
// concentrator that offers no DNS.
func TestRFC1661ClientDropsRejectedDNSOptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rejected []byte
		want     []byte
	}{
		{"both", append(dnsOption(ppp.IPCPOptPrimaryDNS, "0.0.0.0"), dnsOption(ppp.IPCPOptSecondaryDNS, "0.0.0.0")...),
			ipcpAddressOption(netip.IPv4Unspecified())},
		{"secondary", dnsOption(ppp.IPCPOptSecondaryDNS, "0.0.0.0"),
			append(ipcpAddressOption(netip.IPv4Unspecified()), dnsOption(ppp.IPCPOptPrimaryDNS, "0.0.0.0")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				frames := make(chan readFrame)
				stop := make(chan struct{})
				defer close(stop)
				w, done := startIPCP(t, frames, stop)
				synctest.Wait()
				first := recordedIPCP(t, w, 0)
				frames <- ipcpFrame(ppp.LCPConfigureReject, first.Identifier, tc.rejected)
				synctest.Wait()
				next := recordedIPCP(t, w, 1)
				if next.Identifier == first.Identifier {
					t.Fatalf("request after Reject kept Identifier %d", next.Identifier)
				}
				if !bytes.Equal(next.Data, tc.want) {
					t.Fatalf("request after Reject = % x, want % x", next.Data, tc.want)
				}
				frames <- ipcpFrame(ppp.LCPConfigureNak, next.Identifier, ipcpAddressOption(netip.MustParseAddr("192.0.2.2")))
				got := finishIPCP(t, w, frames, done)
				if got.err != nil {
					t.Fatalf("IPCP failed after a DNS Reject: %v", got.err)
				}
				if got.result.localIP != netip.MustParseAddr("192.0.2.2") {
					t.Fatalf("local address = %v", got.result.localIP)
				}
				if got.result.primaryDNS.IsValid() {
					t.Fatalf("primary DNS = %v, want none", got.result.primaryDNS)
				}
				if got.result.secondaryDNS.IsValid() {
					t.Fatalf("secondary DNS = %v, want none", got.result.secondaryDNS)
				}
			})
		})
	}
}

// TestRFC1661ClientFailsOnRejectedIPAddress: a Reject of IP-Address, alone
// or beside a DNS option, ends IPCP with an error.
//
// VALIDATES: AC-5, the behaviour kept from before DNS was requested.
// PREVENTS: a session opened with no address.
func TestRFC1661ClientFailsOnRejectedIPAddress(t *testing.T) {
	for _, rejected := range [][]byte{
		ipcpAddressOption(netip.IPv4Unspecified()),
		append(ipcpAddressOption(netip.IPv4Unspecified()), dnsOption(ppp.IPCPOptSecondaryDNS, "0.0.0.0")...),
	} {
		synctest.Test(t, func(t *testing.T) {
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			_, done := startIPCP(t, frames, stop)
			synctest.Wait()
			frames <- ipcpFrame(ppp.LCPConfigureReject, 1, rejected)
			if got := <-done; got.err == nil {
				t.Fatalf("Reject % x opened IPCP: %+v", rejected, got.result)
			}
		})
	}
}
