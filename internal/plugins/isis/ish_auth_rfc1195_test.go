// Design: docs/architecture/isis/isis-10-auth.md -- authenticated ISO 9542 discovery.
// Goal: verify configured passwords on actual emitted and dispatched ISH PDUs.
// Method: use the running engine and capture backend, not local authentication substitutes.

package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

const ishPasswordConfig = `{"isis":{"net":"49.0001.0000.0000.0001.00",
"key-chains":{"link":{"key":{"1":{"algorithm":"cleartext","secret":"link-password"}}}},
"interfaces":{"interface":{"eth0":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point",
"level-1":{"auth-key-chain":"link"}}}}}}`

// RFC requirement: RFC1195-4.4-2 positive -- the configured P2P ISO 9542
// exchange transmits a per-link password and accepts a peer ISH carrying it.
func TestRFC1195ISHConfiguredPassword(t *testing.T) {
	eng, backend := protocolEngine(t, ishPasswordConfig)
	c := protocolLiveCircuit(t, eng, "eth0")
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	capture := protocolCaptureFor(t, backend, "eth0")
	capture.mu.Lock()
	var pdu, iih []byte
	for _, sent := range capture.sent {
		if len(sent) < 5 {
			continue
		}
		if sent[0] == packet.ESISProtocolDiscriminator {
			pdu = bytes.Clone(sent)
		} else if packet.PDUType(sent[4]&0x1f) == packet.PDUTypeP2PHello {
			iih = bytes.Clone(sent)
		}
	}
	capture.mu.Unlock()
	keys := []packet.Key{{Algorithm: packet.AuthAlgoCleartext, Secret: []byte("link-password")}}
	if err := packet.VerifyISH(pdu, keys); err != nil {
		t.Fatalf("transmitted ISH lacks the configured password: %v", err)
	}
	if len(iih)+transport.LLCHeaderLen != capture.MTU() {
		t.Fatalf("authenticated IIH does not fit the link MTU: PDU=%d MTU=%d", len(iih), capture.MTU())
	}
	if err := packet.VerifyPDU(iih, keys); err != nil {
		t.Fatalf("IIH authentication did not cover final padding: %v", err)
	}
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerISH(t, "link-password")})
	rows := c.Table().Snapshot()
	if len(rows) != 1 {
		t.Fatalf("authenticated discovery missing: %+v", rows)
	}
	if rows[0].State != "initializing" {
		t.Fatalf("ISH bypassed IIH establishment: %+v", rows[0])
	}
}

// RFC requirement: RFC1195-4.4-2 negative -- configured per-link authentication
// rejects missing and wrong passwords before an ISH can initialize the circuit.
func TestRFC1195ISHWrongPassword(t *testing.T) {
	for _, password := range []string{"", "wrong-password"} {
		t.Run(password, func(t *testing.T) {
			eng, _ := protocolEngine(t, ishPasswordConfig)
			c := protocolLiveCircuit(t, eng, "eth0")
			eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerISH(t, password)})
			if rows := c.Table().Snapshot(); len(rows) != 0 {
				t.Fatalf("unauthenticated ISH created state: %+v", rows)
			}
		})
	}
}

// Expired or undecodable configured passwords must not become unauthenticated
// discovery. Both receive and send remain closed through real engine hooks.
func TestRFC1195ISHUnusablePasswordDoesNotDowngrade(t *testing.T) {
	for _, key := range []string{
		`"algorithm":"cleartext","secret":""`,
		`"algorithm":"cleartext","secret":"link-password","send-lifetime":{"end":"2000-01-01T00:00:00Z"},"accept-lifetime":{"end":"2000-01-01T00:00:00Z"}`,
	} {
		data := `{"isis":{"net":"49.0001.0000.0000.0001.00","key-chains":{"link":{"key":{"1":{` + key + `}}}},` +
			`"interfaces":{"interface":{"eth0":{"level":"l1","circuit-type":"point-to-point","level-1":{"auth-key-chain":"link"}}}}}}`
		eng, _ := protocolEngine(t, data)
		c := protocolLiveCircuit(t, eng, "eth0")
		if err := c.SendHello(adjacency.Level1); err == nil {
			t.Fatal("unusable configured ISH password sent an unsigned discovery")
		}
		eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerISH(t, "")})
		if rows := c.Table().Snapshot(); len(rows) != 0 {
			t.Fatalf("unusable password accepted unsigned discovery: %+v", rows)
		}
	}
}

// Crypto-only ISH discovery cannot bypass authenticated IIHs or refresh a live
// adjacency. ISO 9542 has no RFC 5304/5310 HMAC encoding to invent here.
func TestRFC1195ISHCannotBypassIIHAuthentication(t *testing.T) {
	data := `{"isis":{"net":"49.0001.0000.0000.0001.00","key-chains":{"link":{"key":{"1":{"algorithm":"hmac-sha-256","secret":"crypto-key"}}}},` +
		`"interfaces":{"interface":{"eth0":{"level":"l1","circuit-type":"point-to-point","level-1":{"auth-key-chain":"link"}}}}}}`
	eng, _ := protocolEngine(t, data)
	c := protocolLiveCircuit(t, eng, "eth0")
	dispatch := func(pdu []byte) {
		eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: pdu})
	}
	dispatch(protocolPeerISH(t, ""))
	peer := types.SystemID{0, 0, 0, 0, 0, 2}
	iih := p2pHelloPDU(t, peer, eng.cfg.NETs[0].AreaID())
	dispatch(iih)
	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].State != "initializing" {
		t.Fatalf("unsigned IIH bypassed authentication after ISH: %+v", rows)
	}
	signed, err := packet.SignPDU(iih, packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, KeyID: 1, Secret: []byte("crypto-key")})
	if err != nil {
		t.Fatal(err)
	}
	dispatch(signed)
	before := c.Table().Snapshot()
	if len(before) != 1 || before[0].State != "up" {
		t.Fatalf("authenticated IIH did not establish adjacency: %+v", before)
	}
	dispatch(protocolPeerISH(t, ""))
	after := c.Table().Snapshot()
	if len(after) != 1 || after[0] != before[0] {
		t.Fatalf("ISH changed authenticated adjacency: before=%+v after=%+v", before, after)
	}
}
