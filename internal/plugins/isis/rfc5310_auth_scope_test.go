// Design: docs/architecture/isis/isis-10-auth.md -- which authentication string each PDU class uses.
// Related: auth_wiring_test.go -- the authTestConfig fixture and the engine signers.
// Related: auth_rfc1195_test.go -- the authTestSNP builder.
//
// Goal: prove that Ze signs and accepts a Level 1 Sequence Number PDU with the
// Area Authentication string and an IS-IS Hello with the Link Level string, and
// that it keeps using a key it no longer sends with.
// Method: sign through the engine signers, check the bytes against each string,
// and dispatch PDUs signed with the wrong string through verifyFrame. A wrong
// string is signed under the Key ID the receiver expects, so a Key ID lookup
// cannot be what refuses it.

package isis

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// scopeKey is an HMAC-SHA-256 key carrying one of authTestConfig's secrets
// under the given Key ID.
func scopeKey(secret string, keyID uint16) packet.Key {
	return packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, Secret: []byte(secret), KeyID: keyID}
}

// scopeSign signs pdu with key, failing the test on a signing error.
func scopeSign(t *testing.T, pdu []byte, key packet.Key) []byte {
	t.Helper()
	signed, err := packet.SignPDU(pdu, key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// scopeEngine returns an engine keyed with authTestConfig and eth0 at ifindex 10.
func scopeEngine(t *testing.T) *engine {
	t.Helper()
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	e.registerTestCircuit(t, "eth0", 10)
	return e
}

// RFC requirement: RFC5310-3.2-1 positive -- signLevelPDU signs a Level 1 CSNP and a Level 1 PSNP with the area string (Key ID 1 verifies with "areasecret", not with "domainsecret" under the same Key ID), and verifyFrame accepts both.
func TestRFC5310Level1SNPUsesAreaString(t *testing.T) {
	e := scopeEngine(t)
	for _, complete := range []bool{true, false} {
		signed := e.signLevelPDU(authTestSNP(levelOne, complete))
		if err := packet.VerifyPDU(signed, []packet.Key{scopeKey("areasecret", 1)}); err != nil {
			t.Fatalf("complete=%v: L1 SNP not signed with the area string: %v", complete, err)
		}
		if err := packet.VerifyPDU(signed, []packet.Key{scopeKey("domainsecret", 1)}); err == nil {
			t.Fatalf("complete=%v: L1 SNP verifies with the domain string", complete)
		}
		if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signed}) {
			t.Fatalf("complete=%v: area-signed L1 SNP rejected", complete)
		}
	}
}

// RFC requirement: RFC5310-3.2-1 negative -- a Level 1 CSNP or PSNP signed with the domain string or the link string is rejected by verifyFrame, both under the area Key ID 1 and under that string's own configured Key ID, while the same PDU signed with the area string is accepted.
func TestRFC5310Level1SNPRefusesOtherStrings(t *testing.T) {
	e := scopeEngine(t)
	others := []packet.Key{
		scopeKey("domainsecret", 1), scopeKey("iihsecret", 1),
		scopeKey("domainsecret", 2), scopeKey("iihsecret", 3),
	}
	for _, complete := range []bool{true, false} {
		snp := authTestSNP(levelOne, complete)
		if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: scopeSign(t, snp, scopeKey("areasecret", 1))}) {
			t.Fatalf("complete=%v: area-signed L1 SNP rejected", complete)
		}
		for _, key := range others {
			if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: scopeSign(t, snp, key)}) {
				t.Fatalf("complete=%v: L1 SNP signed with %q (Key ID %d) accepted", complete, key.Secret, key.KeyID)
			}
		}
	}
}

// RFC requirement: RFC5310-3.2-3 positive -- signHelloPDU signs a LAN IIH and a point-to-point IIH with the link string (Key ID 3 verifies with "iihsecret", not with "areasecret" under the same Key ID), and verifyFrame accepts both.
func TestRFC5310HelloUsesLinkString(t *testing.T) {
	e := scopeEngine(t)
	for name, hello := range map[string][]byte{"lan": authTestLANHello(), "p2p": authTestP2PHello()} {
		signed := e.signHelloPDU("eth0", adjacency.Level1, hello)
		if err := packet.VerifyPDU(signed, []packet.Key{scopeKey("iihsecret", 3)}); err != nil {
			t.Fatalf("%s IIH not signed with the link string: %v", name, err)
		}
		if err := packet.VerifyPDU(signed, []packet.Key{scopeKey("areasecret", 3)}); err == nil {
			t.Fatalf("%s IIH verifies with the area string", name)
		}
		if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signed}) {
			t.Fatalf("link-signed %s IIH rejected", name)
		}
	}
}

// RFC requirement: RFC5310-3.2-3 negative -- a LAN IIH or point-to-point IIH signed with the area string or the domain string is rejected by verifyFrame, both under the link Key ID 3 and under that string's own configured Key ID, while the same IIH signed with the link string is accepted.
func TestRFC5310HelloRefusesAreaAndDomainStrings(t *testing.T) {
	e := scopeEngine(t)
	others := []packet.Key{
		scopeKey("areasecret", 3), scopeKey("domainsecret", 3),
		scopeKey("areasecret", 1), scopeKey("domainsecret", 2),
	}
	for name, hello := range map[string][]byte{"lan": authTestLANHello(), "p2p": authTestP2PHello()} {
		if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: scopeSign(t, hello, scopeKey("iihsecret", 3))}) {
			t.Fatalf("link-signed %s IIH rejected", name)
		}
		for _, key := range others {
			if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: scopeSign(t, hello, key)}) {
				t.Fatalf("%s IIH signed with %q (Key ID %d) accepted", name, key.Secret, key.KeyID)
			}
		}
	}
}

// RFC requirement: RFC5310-4-2 negative -- after the send key moves to key 2, the store still uses key 1: signLevelPDU signs with key 2 alone, and verifyFrame accepts an LSP signed with key 1 as well as one signed with key 2.
func TestRFC5310NonSendingKeyStillUsed(t *testing.T) {
	now := time.Now()
	rolled := now.Add(-time.Minute).Format(time.RFC3339)
	start := now.Add(-time.Hour).Format(time.RFC3339)
	end := now.Add(time.Hour).Format(time.RFC3339)
	cfg := Config{
		Level1AuthKeyChain: "rot",
		KeyChains: []KeyChainConfig{{
			Name: "rot",
			Keys: []KeyConfig{
				{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "oldkey",
					SendStart: start, SendEnd: rolled, AcceptStart: start, AcceptEnd: end},
				{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "newkey",
					SendStart: rolled, SendEnd: end, AcceptStart: start, AcceptEnd: end},
			},
		}},
	}
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(cfg)
	e.registerTestCircuit(t, "eth0", 10)
	lsp := authTestLSP(levelOne)
	sent := e.signLevelPDU(lsp)
	if err := packet.VerifyPDU(sent, []packet.Key{scopeKey("newkey", 2)}); err != nil {
		t.Fatalf("send key did not move to key 2: %v", err)
	}
	for _, key := range []packet.Key{scopeKey("oldkey", 1), scopeKey("newkey", 2)} {
		if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: scopeSign(t, lsp, key)}) {
			t.Fatalf("LSP signed with key %d rejected while both keys are stored", key.KeyID)
		}
	}
}
