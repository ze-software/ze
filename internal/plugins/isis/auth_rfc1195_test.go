// Design: docs/architecture/isis/isis-10-auth.md -- RFC 1195 Annex D password
// scoping: per-link, per-area and per-domain, one transmit password and a set of
// receive passwords.
//
// Goal: prove that the key store holds the three password scopes the RFC names,
// that each PDU class is signed with the scope the RFC assigns it and with no
// other, and that a chain transmits with one key while it accepts a set. Method:
// resolve a config into the key store, sign one PDU of each class through the
// engine signers, and verify each against the right and the wrong key sets.

package isis

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// authTestSNP encodes a minimal CSNP or PSNP at the given level.
func authTestSNP(level lsdbLevel, complete bool) []byte {
	src := types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 7}, 0)
	if complete {
		pt := packet.PDUTypeL1CSNP
		if level == levelTwo {
			pt = packet.PDUTypeL2CSNP
		}
		c := packet.CSNP{PDUType: pt, SourceID: src, EndLSPID: types.LSPID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}}
		buf := make([]byte, c.EncodedLen())
		return buf[:c.WriteTo(buf, 0)]
	}
	pt := packet.PDUTypeL1PSNP
	if level == levelTwo {
		pt = packet.PDUTypeL2PSNP
	}
	p := packet.PSNP{PDUType: pt, SourceID: src}
	buf := make([]byte, p.EncodedLen())
	return buf[:p.WriteTo(buf, 0)]
}

// chainKeys returns the keys a chain accepts now, or nil for no chain.
func chainKeys(ks *keyStore, c *keyChain) []packet.Key {
	return ks.verifyKeys(c, time.Now())
}

// RFC requirement: RFC1195-7-6 positive -- one config resolves three password
// scopes: the per-link chain of interface eth0, the per-area chain (Level 1) and
// the per-domain chain (Level 2), each holding its own key.
func TestRFC1195PasswordScopesResolved(t *testing.T) {
	ks := newKeyStore(authTestConfig())

	link := ks.helloChain("eth0", levelOne)
	area := ks.levelChain(levelOne)
	domain := ks.levelChain(levelTwo)
	for name, c := range map[string]*keyChain{"per-link": link, "per-area": area, "per-domain": domain} {
		if c == nil || len(c.keys) != 1 {
			t.Fatalf("%s chain = %+v, want one resolved key", name, c)
		}
	}
	if link.name != "iih" || area.name != "area" || domain.name != "domain" {
		t.Fatalf("chains resolved as link=%q area=%q domain=%q, want iih/area/domain", link.name, area.name, domain.name)
	}
	if link.keys[0].key.KeyID == area.keys[0].key.KeyID || area.keys[0].key.KeyID == domain.keys[0].key.KeyID {
		t.Fatalf("scopes share a key: link %d area %d domain %d", link.keys[0].key.KeyID, area.keys[0].key.KeyID, domain.keys[0].key.KeyID)
	}
}

// RFC requirement: RFC1195-7-6 negative -- a per-link password is bound to its
// link and a per-area password to its level: interface eth1, which names no
// chain, resolves no per-link chain, and the Level 2 domain chain is not the
// Level 1 area chain.
func TestRFC1195PasswordScopesDoNotLeak(t *testing.T) {
	ks := newKeyStore(authTestConfig())

	if c := ks.helloChain("eth1", levelOne); c != nil {
		t.Fatalf("eth1 resolved the per-link chain %q configured on eth0", c.name)
	}
	if c := ks.helloChain("eth1", levelTwo); c != nil {
		t.Fatalf("eth1 resolved the per-link chain %q configured on eth0 at Level 2", c.name)
	}
	if ks.levelChain(levelTwo) == ks.levelChain(levelOne) {
		t.Fatal("the Level 2 domain chain is the Level 1 area chain")
	}
	if !ks.circuitAuthenticated("eth0") {
		t.Fatal("eth0 is not reported authenticated although it names a per-link chain")
	}
	if ks.circuitAuthenticated("eth1") {
		t.Fatal("eth1 is reported authenticated although it names no per-link chain")
	}
}

// RFC requirement: RFC1195-7-7 positive -- the engine signs an IS-IS Hello with
// the per-link password, a Level 1 LSP, CSNP and PSNP with the per-area password,
// and a Level 2 LSP, CSNP and PSNP with the per-domain password: each signed PDU
// verifies under the keys of that scope.
func TestRFC1195PDUClassSignedWithItsScope(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	ks := e.keystore

	hello := e.signHelloPDU("eth0", adjacency.Level1, authTestLANHello())
	if err := packet.VerifyPDU(hello, chainKeys(ks, ks.helloChain("eth0", levelOne))); err != nil {
		t.Fatalf("hello does not verify under the per-link password: %v", err)
	}

	cases := []struct {
		name  string
		pdu   []byte
		scope *keyChain
	}{
		{"L1 LSP", authTestLSP(levelOne), ks.levelChain(levelOne)},
		{"L1 CSNP", authTestSNP(levelOne, true), ks.levelChain(levelOne)},
		{"L1 PSNP", authTestSNP(levelOne, false), ks.levelChain(levelOne)},
		{"L2 LSP", authTestLSP(levelTwo), ks.levelChain(levelTwo)},
		{"L2 CSNP", authTestSNP(levelTwo, true), ks.levelChain(levelTwo)},
		{"L2 PSNP", authTestSNP(levelTwo, false), ks.levelChain(levelTwo)},
	}
	for _, tc := range cases {
		signed := e.signLevelPDU(tc.pdu)
		if packet.AuthTLVIndex(mustDecodeTLVs(t, signed)) != 0 {
			t.Fatalf("%s: no authentication TLV was written first", tc.name)
		}
		if err := packet.VerifyPDU(signed, chainKeys(ks, tc.scope)); err != nil {
			t.Fatalf("%s does not verify under its scope's password (%s): %v", tc.name, tc.scope.name, err)
		}
	}
}

// RFC requirement: RFC1195-7-7 negative -- a PDU signed with one scope's password
// is refused under every other scope: the per-link hello fails under the area and
// domain keys, the Level 1 LSP fails under the domain and per-link keys, and the
// Level 2 CSNP fails under the area and per-link keys.
func TestRFC1195PDUClassRefusedUnderOtherScopes(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	ks := e.keystore
	link := ks.helloChain("eth0", levelOne)
	area := ks.levelChain(levelOne)
	domain := ks.levelChain(levelTwo)

	hello := e.signHelloPDU("eth0", adjacency.Level1, authTestLANHello())
	l1lsp := e.signLevelPDU(authTestLSP(levelOne))
	l2csnp := e.signLevelPDU(authTestSNP(levelTwo, true))

	cases := []struct {
		name  string
		pdu   []byte
		wrong *keyChain
	}{
		{"hello under area", hello, area},
		{"hello under domain", hello, domain},
		{"L1 LSP under domain", l1lsp, domain},
		{"L1 LSP under per-link", l1lsp, link},
		{"L2 CSNP under area", l2csnp, area},
		{"L2 CSNP under per-link", l2csnp, link},
	}
	for _, tc := range cases {
		if err := packet.VerifyPDU(tc.pdu, chainKeys(ks, tc.wrong)); err == nil {
			t.Fatalf("%s: accepted, want refused (the %s password is not this PDU class's scope)", tc.name, tc.wrong.name)
		}
	}
}

// mustDecodeTLVs decodes a signed PDU and returns its TLV list whatever the class.
func mustDecodeTLVs(t *testing.T, pdu []byte) []packet.TLV {
	t.Helper()
	dec, err := packet.DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	switch {
	case dec.LSP != nil:
		return dec.LSP.TLVs
	case dec.CSNP != nil:
		return dec.CSNP.TLVs
	case dec.PSNP != nil:
		return dec.PSNP.TLVs
	}
	t.Fatal("decoded PDU is not an LSP, CSNP or PSNP")
	return nil
}

// twoKeyChainConfig binds the Level 1 area scope to a chain holding two keys that
// both send and accept now, and a third key whose accept lifetime has ended.
func twoKeyChainConfig() Config {
	past := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	ended := time.Now().Add(-time.Hour).Format(time.RFC3339)
	return Config{
		Level1AuthKeyChain: "area",
		KeyChains: []KeyChainConfig{{
			Name: "area",
			Keys: []KeyConfig{
				{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "first"},
				{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "second"},
				{KeyID: 3, Algorithm: "hmac-sha-256", Secret: "retired", AcceptStart: past, AcceptEnd: ended},
			},
		}},
	}
}

// RFC requirement: RFC1195-7-8 positive -- a scope's Transmit Password is a
// single key (the chain signs with key 1 alone) while its Receive Passwords are a
// set (a PDU signed with key 1 and a PDU signed with key 2 both verify).
func TestRFC1195OneTransmitPasswordManyReceivePasswords(t *testing.T) {
	ks := newKeyStore(twoKeyChainConfig())
	area := ks.levelChain(levelOne)

	sk, ok := ks.signKey(area, time.Now())
	if !ok || sk.KeyID != 1 {
		t.Fatalf("transmit key = %+v ok=%v, want the single key 1", sk, ok)
	}
	receive := ks.verifyKeys(area, time.Now())
	if len(receive) != 2 {
		t.Fatalf("receive set holds %d keys, want 2 (keys 1 and 2)", len(receive))
	}
	for _, key := range area.keys[:2] {
		signed, err := packet.SignPDU(authTestLSP(levelOne), key.key)
		if err != nil {
			t.Fatalf("sign with key %d: %v", key.key.KeyID, err)
		}
		if err := packet.VerifyPDU(signed, receive); err != nil {
			t.Fatalf("PDU signed with receive key %d refused: %v", key.key.KeyID, err)
		}
	}
}

// RFC requirement: RFC1195-7-8 negative -- a key outside the Receive Passwords
// set (accept lifetime ended) does not authenticate a PDU, and the chain never
// transmits with a key other than its single Transmit Password: a PDU signed with
// key 2 is refused when only the transmit key is offered for verification.
func TestRFC1195KeyOutsideReceiveSetRefused(t *testing.T) {
	ks := newKeyStore(twoKeyChainConfig())
	area := ks.levelChain(levelOne)
	receive := ks.verifyKeys(area, time.Now())

	retired := area.keys[2].key
	signed, err := packet.SignPDU(authTestLSP(levelOne), retired)
	if err != nil {
		t.Fatalf("sign with the retired key: %v", err)
	}
	if err := packet.VerifyPDU(signed, receive); err == nil {
		t.Fatal("a PDU signed with the retired key (outside the receive set) was accepted")
	}

	sk, _ := ks.signKey(area, time.Now())
	withSecond, err := packet.SignPDU(authTestLSP(levelOne), area.keys[1].key)
	if err != nil {
		t.Fatalf("sign with key 2: %v", err)
	}
	if err := packet.VerifyPDU(withSecond, []packet.Key{sk}); err == nil {
		t.Fatal("the transmit key verified a PDU signed with key 2: the chain does not hold a single transmit password")
	}
}
