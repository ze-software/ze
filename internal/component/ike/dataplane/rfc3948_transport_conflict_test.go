// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- policy ownership
// Related: policy_owner.go -- policyOwners.claim, the refusal these tests drive

package dataplane

import (
	"errors"
	"net"
	"strings"
	"testing"
)

// transportClient builds the outbound or inbound transport-mode policy the engine
// installs for one client behind the RFC 3948 Section 5.2 NAT (childPolicyParams in
// internal/component/ike/engine/child.go): the server's address and the NAT's external
// address as the host selector pair, no tunnel endpoints, one owner per client.
func transportClient(t *testing.T, owner string, dir SADir, upperProto uint8, serverPort PortMatch) SPParams {
	t.Helper()
	_, server, err := net.ParseCIDR("203.0.113.1/32")
	if err != nil {
		t.Fatalf("parse server address: %v", err)
	}
	_, nat, err := net.ParseCIDR("198.51.100.7/32")
	if err != nil {
		t.Fatalf("parse NAT address: %v", err)
	}
	p := SPParams{
		Src:        server,
		Dst:        nat,
		Dir:        dir,
		Action:     SPActionProtect,
		Owner:      owner,
		Proto:      ProtoESP,
		Mode:       ModeTransport,
		ReqID:      1,
		Priority:   PriorityChildSA,
		UpperProto: upperProto,
		SrcPort:    serverPort,
		DstPort:    AnyPortMatch(),
	}
	if dir == SADirIn {
		p.Src, p.Dst = nat, server
		p.SrcPort, p.DstPort = AnyPortMatch(), serverPort
	}
	return p
}

// VALIDATES: a transport-mode claim whose traffic description overlaps a different
// client's transport-mode claim is refused, in either order and either direction, and
// the refusal names both clients and hands nothing over.
// PREVENTS: the RFC 3948 Section 5.2 conflict: two transport SAs from the server to one
// NAT address whose descriptions overlap, where the kernel's ordered policy search sends
// the overlap to whichever client's SA ranks first.
//
// RFC requirement: RFC3948-5.2-1 negative -- a second client's transport-mode claim whose protocol and port description overlaps the first client's (any against TCP 80, TCP 80 against any, TCP any port against TCP 80) is refused with TransportSelectorConflictError naming both clients, and the second client holds no record.
func TestRFC3948TransportConflictingConnectionIsDisallowed(t *testing.T) {
	cases := []struct {
		name  string
		first SPParams
		later SPParams
	}{
		{"any then tcp/80", transportClient(t, "ari", SADirOut, 0, AnyPortMatch()),
			transportClient(t, "bob", SADirOut, 6, ExactPortMatch(80))},
		{"tcp/80 then any", transportClient(t, "ari", SADirOut, 6, ExactPortMatch(80)),
			transportClient(t, "bob", SADirOut, 0, AnyPortMatch())},
		{"tcp any port then tcp/80 inbound", transportClient(t, "ari", SADirIn, 6, AnyPortMatch()),
			transportClient(t, "bob", SADirIn, 6, ExactPortMatch(80))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var owners policyOwners
			if _, err := owners.claim(tc.first); err != nil {
				t.Fatalf("first client could not claim an unheld selector: %v", err)
			}
			_, err := owners.claim(tc.later)
			if err == nil {
				t.Fatal("a second client's overlapping transport description was admitted: the overlap goes to whichever SA the kernel ranks first")
			}
			var conflict *TransportSelectorConflictError
			if !errors.As(err, &conflict) {
				t.Fatalf("error is %T (%v), want *TransportSelectorConflictError", err, err)
			}
			if conflict.HeldBy != "ari" || conflict.Wanted != "bob" {
				t.Errorf("refusal names held-by %q wanted %q, want ari / bob", conflict.HeldBy, conflict.Wanted)
			}
			for _, want := range []string{"ari", "bob"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal text does not name %q: %s", want, err)
				}
			}
			if held, ok := owners.ownerOf(tc.later); ok {
				t.Errorf("refused client holds a record (owner %q): the refusal must record nothing", held)
			}
		})
	}
}

// VALIDATES: two clients behind one NAT whose transport-mode descriptions do NOT overlap
// are both admitted, and one client's own overlapping re-claim (a rekey, or a second
// Child SA of the same peer) is admitted.
// PREVENTS: the refusal above being bounded by the shared NAT address rather than by
// the overlap, which would refuse every second client behind a NAT.
//
// RFC requirement: RFC3948-5.2-1 positive -- transport-mode clients behind one NAT address with disjoint descriptions (TCP 80 against TCP 443, TCP against UDP) are both admitted, and the refusal is between clients: the same client's overlapping claim is admitted.
func TestRFC3948TransportDisjointConnectionsAreAdmitted(t *testing.T) {
	cases := []struct {
		name  string
		first SPParams
		later SPParams
	}{
		{"tcp/80 and tcp/443", transportClient(t, "ari", SADirOut, 6, ExactPortMatch(80)),
			transportClient(t, "bob", SADirOut, 6, ExactPortMatch(443))},
		{"tcp and udp", transportClient(t, "ari", SADirIn, 6, AnyPortMatch()),
			transportClient(t, "bob", SADirIn, 17, AnyPortMatch())},
		{"same client overlapping", transportClient(t, "ari", SADirOut, 0, AnyPortMatch()),
			transportClient(t, "ari", SADirOut, 6, ExactPortMatch(80))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var owners policyOwners
			if _, err := owners.claim(tc.first); err != nil {
				t.Fatalf("first claim: %v", err)
			}
			if _, err := owners.claim(tc.later); err != nil {
				t.Fatalf("a non-conflicting transport claim was refused: %v", err)
			}
			if held, _ := owners.ownerOf(tc.later); held != tc.later.Owner {
				t.Errorf("owner of the admitted claim = %q, want %q", held, tc.later.Owner)
			}
		})
	}
}

// VALIDATES: two transport-mode clients at DIFFERENT addresses (two NATs, or two
// un-NATed hosts) whose protocol and port descriptions overlap exactly are both
// admitted, in either direction.
// PREVENTS: the RFC 3948 Section 5.2 refusal being decided by the protocol and port
// description alone: the conflict the RFC names needs the SAs to share the address,
// so a prefix comparison that always answered "overlap" would refuse every second
// transport client on the server.
//
// RFC requirement: RFC3948-5.2-1 positive -- transport-mode clients at different addresses with overlapping descriptions (any against TCP 80 outbound, TCP any port against TCP 80 inbound) are both admitted, each owning its own claim.
func TestRFC3948TransportClientsAtDifferentAddressesAreAdmitted(t *testing.T) {
	_, otherNAT, err := net.ParseCIDR("192.0.2.9/32")
	if err != nil {
		t.Fatalf("parse second NAT address: %v", err)
	}
	elsewhere := func(p SPParams) SPParams {
		if p.Dir == SADirIn {
			p.Src = otherNAT
			return p
		}
		p.Dst = otherNAT
		return p
	}
	cases := []struct {
		name  string
		first SPParams
		later SPParams
	}{
		{"any then tcp/80 outbound", transportClient(t, "ari", SADirOut, 0, AnyPortMatch()),
			elsewhere(transportClient(t, "bob", SADirOut, 6, ExactPortMatch(80)))},
		{"tcp any port then tcp/80 inbound", transportClient(t, "ari", SADirIn, 6, AnyPortMatch()),
			elsewhere(transportClient(t, "bob", SADirIn, 6, ExactPortMatch(80)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var owners policyOwners
			if _, err := owners.claim(tc.first); err != nil {
				t.Fatalf("first claim: %v", err)
			}
			if _, err := owners.claim(tc.later); err != nil {
				t.Fatalf("a transport client at a different address was refused: %v", err)
			}
			if held, _ := owners.ownerOf(tc.first); held != "ari" {
				t.Errorf("owner of the first claim = %q, want ari", held)
			}
			if held, _ := owners.ownerOf(tc.later); held != "bob" {
				t.Errorf("owner of the second claim = %q, want bob", held)
			}
		})
	}
}
