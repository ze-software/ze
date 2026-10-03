// Design: docs/architecture/isis/isis-10-auth.md -- RFC 1195 Annex D password scoping for ISO 9542 IS Hellos.
// Related: ish_auth_rfc1195_test.go -- the single-password ISH tests.
//
// Goal: prove that an ISO 9542 IS Hello carries the per-link password and no
// other scope's password, when per-link, per-area and per-domain passwords are
// all configured and all differ.
// Method: run the engine over the capture backend, verify the transmitted ISH
// against each scope's password, and dispatch peer ISHs carrying each password.

package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// ishScopeConfig configures three distinct cleartext passwords: per-link on
// eth0, per-area at Level 1 and per-domain at Level 2.
const ishScopeConfig = `{"isis":{"net":"49.0001.0000.0000.0001.00",
"key-chains":{"link":{"key":{"1":{"algorithm":"cleartext","secret":"link-password"}}},
"area":{"key":{"1":{"algorithm":"cleartext","secret":"area-password"}}},
"domain":{"key":{"1":{"algorithm":"cleartext","secret":"domain-password"}}}},
"level-1":{"auth-key-chain":"area"},"level-2":{"auth-key-chain":"domain"},
"interfaces":{"interface":{"eth0":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point",
"level-1":{"auth-key-chain":"link"}}}}}}`

// cleartextKey is the cleartext key carrying password.
func cleartextKey(password string) []packet.Key {
	return []packet.Key{{Algorithm: packet.AuthAlgoCleartext, Secret: []byte(password)}}
}

// RFC requirement: RFC1195-7-7 positive -- with per-link, per-area and per-domain passwords all configured, the transmitted ISO 9542 IS Hello verifies with the per-link password and with neither other one, and a peer ISH carrying the per-link password initializes the adjacency.
func TestRFC1195ISHCarriesPerLinkPassword(t *testing.T) {
	eng, backend := protocolEngine(t, ishScopeConfig)
	c := protocolLiveCircuit(t, eng, "eth0")
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	capture := protocolCaptureFor(t, backend, "eth0")
	capture.mu.Lock()
	var ish []byte
	for _, sent := range capture.sent {
		if len(sent) > 0 && sent[0] == packet.ESISProtocolDiscriminator {
			ish = bytes.Clone(sent)
		}
	}
	capture.mu.Unlock()
	if ish == nil {
		t.Fatal("no ISH transmitted")
	}
	if err := packet.VerifyISH(ish, cleartextKey("link-password")); err != nil {
		t.Fatalf("transmitted ISH lacks the per-link password: %v", err)
	}
	for _, other := range []string{"area-password", "domain-password"} {
		if err := packet.VerifyISH(ish, cleartextKey(other)); err == nil {
			t.Fatalf("transmitted ISH verifies with %q", other)
		}
	}
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerISH(t, "link-password")})
	if rows := c.Table().Snapshot(); len(rows) != 1 || rows[0].State != "initializing" {
		t.Fatalf("per-link ISH did not initialize the adjacency: %+v", rows)
	}
}

// RFC requirement: RFC1195-7-7 negative -- a peer ISO 9542 IS Hello carrying the per-area or the per-domain password, both configured on this router, creates no adjacency state on a circuit whose per-link password differs.
func TestRFC1195ISHRefusesAreaAndDomainPasswords(t *testing.T) {
	for _, password := range []string{"area-password", "domain-password"} {
		t.Run(password, func(t *testing.T) {
			eng, _ := protocolEngine(t, ishScopeConfig)
			c := protocolLiveCircuit(t, eng, "eth0")
			eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerISH(t, password)})
			if rows := c.Table().Snapshot(); len(rows) != 0 {
				t.Fatalf("ISH with the %s created state: %+v", password, rows)
			}
		})
	}
}
