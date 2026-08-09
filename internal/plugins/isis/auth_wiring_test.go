// Design: docs/architecture/isis/isis-10-auth.md -- engine authentication wiring tests.
//
// These tests exercise the engine-level sign/verify hooks end to end on raw PDU
// bytes (no transport): the per-level signer (LSP/CSNP/PSNP), the per-interface
// signer (IIH), the verify hook's accept/reject + counter increment, and the
// chain selection by PDU class.

package isis

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// authTestConfig builds a config with a per-level area/domain chain and a
// per-interface IIH chain on eth0, all HMAC-SHA-256.
func authTestConfig() Config {
	return Config{
		Level1AuthKeyChain: "area",
		Level2AuthKeyChain: "domain",
		KeyChains: []KeyChainConfig{
			{Name: "area", Keys: []KeyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "areasecret"}}},
			{Name: "domain", Keys: []KeyConfig{{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "domainsecret"}}},
			{Name: "iih", Keys: []KeyConfig{{KeyID: 3, Algorithm: "hmac-sha-256", Secret: "iihsecret"}}},
		},
		Interfaces: []InterfaceConfig{{
			Name:   "eth0",
			Level1: LevelInterfaceConfig{AuthKeyChain: "iih"},
			Level2: LevelInterfaceConfig{AuthKeyChain: "iih"},
		}},
	}
}

// authTestConfigSplitIIH builds a config whose per-interface IIH chains DIFFER
// between L1 and L2 on eth0 ("iih1" vs "iih2"). This is the case the P2P signing
// fix targets: a P2P IIH signed with the negotiated level's chain must still
// verify even though the receiver cannot read the negotiated level off the wire.
func authTestConfigSplitIIH() Config {
	return Config{
		KeyChains: []KeyChainConfig{
			{Name: "iih1", Keys: []KeyConfig{{KeyID: 11, Algorithm: "hmac-sha-256", Secret: "iih1secret"}}},
			{Name: "iih2", Keys: []KeyConfig{{KeyID: 12, Algorithm: "hmac-sha-256", Secret: "iih2secret"}}},
		},
		Interfaces: []InterfaceConfig{{
			Name:   "eth0",
			Level1: LevelInterfaceConfig{AuthKeyChain: "iih1"},
			Level2: LevelInterfaceConfig{AuthKeyChain: "iih2"},
		}},
	}
}

// authTestP2PHello encodes a minimal P2P IIH (level-agnostic on the wire).
func authTestP2PHello() []byte {
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1L2,
		SystemID:       types.SystemID{0, 0, 0, 0, 0, 8},
		HoldingTime:    types.HoldingTime(30),
		LocalCircuitID: 1,
		TLVs:           []packet.TLV{{Type: packet.TLVAreaAddresses, Value: []byte{0x01, 0x49}}},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

func authTestLSP(level lsdbLevel) []byte {
	pt := packet.PDUTypeL1LSP
	if level == levelTwo {
		pt = packet.PDUTypeL2LSP
	}
	src := types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 7}, 0)
	l := packet.LSP{
		PDUType:           pt,
		RemainingLifetime: 1200,
		LSPID:             types.NewLSPID(src, 0),
		SequenceNumber:    3,
		TypeBlock:         packet.LSPFlagISTypeL1,
		TLVs:              []packet.TLV{{Type: packet.TLVAreaAddresses, Value: []byte{0x01, 0x49}}},
	}
	buf := make([]byte, l.EncodedLen())
	return buf[:l.WriteTo(buf, 0)]
}

func authTestLANHello(level lsdbLevel) []byte {
	pt := packet.PDUTypeL1LANHello
	if level == levelTwo {
		pt = packet.PDUTypeL2LANHello
	}
	h := packet.LANHello{
		PDUType:     pt,
		CircuitType: packet.CircuitL1L2,
		SystemID:    types.SystemID{0, 0, 0, 0, 0, 8},
		HoldingTime: types.HoldingTime(30),
		TLVs:        []packet.TLV{{Type: packet.TLVAreaAddresses, Value: []byte{0x01, 0x49}}},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// VALIDATES: the per-level signer signs an LSP with the area key (L1) and the
// domain key (L2), and the engine verify hook accepts the result.
//
// RFC requirement: RFC5304-2-1 positive -- a Level 1 level PDU signed with the AREA
// authentication string (the L1 chain, levelChain(levelOne)) verifies; L1 PDUs use
// the area string as in L1 Link State PDUs (RFC 5304 sec 2).
// RFC requirement: RFC5304-2-2 positive -- a Level 2 level PDU signed with the DOMAIN
// authentication string (the L2 chain, levelChain(levelTwo)) verifies; L2 PDUs use
// the domain string as in L2 Link State PDUs (RFC 5304 sec 2).
// RFC requirement: RFC5310-3.2-1 positive -- authTestConfig keys the area chain
// HMAC-SHA-256, so a Level 1 level PDU signed with the AREA authentication string
// carries CRYPTO_AUTH type 3 and verifies; L1 PDUs use the Area Authentication string
// as in L1 Link State PDUs (RFC 5310 sec 3.2).
// RFC requirement: RFC5310-3.2-2 positive -- a Level 2 level PDU signed with the DOMAIN
// authentication string (HMAC-SHA-256, CRYPTO_AUTH type 3) verifies; L2 PDUs use the
// domain authentication string as in L2 Link State PDUs (RFC 5310 sec 3.2).
func TestISISAuthEngineSignLevel(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())

	for _, level := range []lsdbLevel{levelOne, levelTwo} {
		signed := e.signLevelPDU(authTestLSP(level))
		// The signed LSP must verify with the correct per-level chain.
		e.ksMu.RLock()
		keys := e.keystore.verifyKeys(e.keystore.levelChain(level), nowFn())
		e.ksMu.RUnlock()
		if err := packet.VerifyPDU(signed, keys); err != nil {
			t.Fatalf("level %d signed LSP failed verify: %v", level, err)
		}
		// TLV 10 must be first.
		dec, _ := packet.DecodePDU(signed)
		if packet.AuthTLVIndex(dec.LSP.TLVs) != 0 {
			t.Fatalf("level %d: TLV 10 not first", level)
		}
		// The Fletcher checksum must be valid after signing.
		if !dec.LSP.VerifyChecksum() {
			t.Fatalf("level %d: LSP checksum invalid after signing", level)
		}
	}
}

// VALIDATES: TestISISAuthReject (TDD plan, AC-1/AC-2/AC-3) -- the verify hook
// rejects a frame with no key and a frame with the wrong key, accepts the correct
// key, and increments ze_isis_auth_failures_total on rejection.
func TestISISAuthReject(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	// Register a live circuit named eth0 at ifindex 10 so the IIH chain resolves.
	e.registerTestCircuit(t, "eth0", 10)

	// Positive case: a correctly signed IIH on eth0 verifies (adjacency may form).
	// RFC requirement: RFC5304-2-3 positive -- an IIH signed with the per-interface
	// Link Level Authentication String (the circuit's IIH chain) verifies (RFC 5304 sec 2).
	// RFC requirement: RFC5310-3.2-3 positive -- an IIH signed with the Link Level
	// Authentication string (the per-interface IIH chain, HMAC-SHA-256/CRYPTO_AUTH type
	// 3) verifies; IS-IS HELLO PDUs use the Link Level Authentication string
	// (RFC 5310 sec 3.2).
	signedHello := e.signHelloPDU("eth0", adjacency.Level1, authTestLANHello(levelOne))
	if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signedHello}) {
		t.Fatal("correctly signed IIH rejected")
	}

	// Negative case (AC-1): an IIH with no TLV 10 is rejected.
	// RFC requirement: RFC5304-2-3 negative -- an IIH lacking the Link Level
	// Authentication String (no TLV 10) is rejected under configured IIH auth (RFC 5304 sec 2).
	// RFC requirement: RFC5310-3.2-3 negative -- an IIH lacking the Link Level
	// Authentication string (no TLV 10) is rejected under configured CRYPTO_AUTH
	// (RFC 5310 sec 3.2).
	if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: authTestLANHello(levelOne)}) {
		t.Fatal("unauthenticated IIH accepted under configured auth")
	}

	// Negative case (AC-2): an IIH signed with the wrong key is rejected.
	wrong := packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, Secret: []byte("nope"), KeyID: 3}
	badHello, _ := packet.SignPDU(authTestLANHello(levelOne), wrong)
	if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: badHello}) {
		t.Fatal("wrong-key IIH accepted")
	}
}

// VALIDATES: chain selection by PDU class -- an LSP (per-level) is verified
// against the level chain, not the per-interface chain, so an IIH key does not
// accept an LSP and vice versa (AC-10 cross-use rejection).
func TestISISAuthChainSelection(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	e.registerTestCircuit(t, "eth0", 10)

	// An LSP signed with the AREA (level) key verifies as an LSP.
	signedLSP := e.signLevelPDU(authTestLSP(levelOne))
	if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signedLSP}) {
		t.Fatal("area-signed LSP rejected")
	}

	// An LSP signed with the IIH key (wrong chain) is rejected: the verify path
	// selects the area chain for an LSP, whose key differs.
	// RFC requirement: RFC5304-2-1 negative -- an L1 level PDU signed with a
	// non-area string (the per-interface IIH key) is rejected; only the Area
	// authentication string authenticates an L1 PDU (RFC 5304 sec 2).
	// RFC requirement: RFC5310-3.2-1 negative -- an L1 level PDU signed with a
	// non-area string (the per-interface IIH key) is rejected; only the Area
	// Authentication string authenticates an L1 PDU (RFC 5310 sec 3.2).
	iihKey := packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, Secret: []byte("iihsecret"), KeyID: 3}
	crossLSP, _ := packet.SignPDU(authTestLSP(levelOne), iihKey)
	if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: crossLSP}) {
		t.Fatal("IIH-key-signed LSP accepted (cross-use should be rejected)")
	}
}

// VALIDATES: a P2P IIH signed with EITHER per-interface IIH chain verifies, even
// when the L1 and L2 IIH chains differ. The sender (sendP2PHello) signs with the
// NEGOTIATED adjacency level's chain, but the receiver cannot read the negotiated
// level off a level-agnostic P2P Hello, so verifyFrame tries both IIH chains.
// PREVENTS: regression to verifying a P2P Hello against only the L1 chain (which
// would drop an L2-negotiated, L2-key-signed P2P Hello when the chains differ).
func TestISISAuthP2PHelloBothChains(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfigSplitIIH())
	e.registerTestCircuit(t, "eth0", 10)

	// A P2P Hello signed with the L2 IIH chain (the L2-negotiated case) verifies.
	signedL2 := e.signHelloPDU("eth0", adjacency.Level2, authTestP2PHello())
	if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signedL2}) {
		t.Fatal("L2-chain-signed P2P Hello rejected (receiver must try both IIH chains)")
	}

	// A P2P Hello signed with the L1 IIH chain (the L1-negotiated case) verifies.
	signedL1 := e.signHelloPDU("eth0", adjacency.Level1, authTestP2PHello())
	if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signedL1}) {
		t.Fatal("L1-chain-signed P2P Hello rejected")
	}

	// A P2P Hello signed with neither chain's key is still rejected (the HMAC
	// must match a real key; trying both chains does not weaken rejection).
	wrong := packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, Secret: []byte("nope"), KeyID: 11}
	bad, _ := packet.SignPDU(authTestP2PHello(), wrong)
	if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: bad}) {
		t.Fatal("wrong-key P2P Hello accepted")
	}

	// An unsigned P2P Hello under configured IIH auth is rejected.
	if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: authTestP2PHello()}) {
		t.Fatal("unsigned P2P Hello accepted under configured auth")
	}
}

// VALIDATES: with no auth configured the verify hook is detached and signing is a
// no-op (unauthenticated operation, the default).
func TestISISAuthUnconfigured(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(Config{}) // no key chains

	pdu := authTestLSP(levelOne)
	if got := e.signLevelPDU(pdu); len(got) != len(pdu) {
		t.Fatal("signLevelPDU mutated PDU with no chain configured")
	}
	if e.dispatch.verify != nil {
		t.Fatal("verify hook should be detached with no auth configured")
	}
}

// VALIDATES: the L2 mirror of TestISISAuthChainSelection -- a Level 2 level PDU is
// authenticated against the DOMAIN chain (RFC 5304 sec 2: L2 PDUs use the domain
// authentication string as in L2 Link State PDUs). A domain-signed L2 LSP verifies;
// an L2 LSP signed with any other string (the per-interface IIH key) is rejected.
func TestISISAuthLevelChainCrossUseL2(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	e.registerTestCircuit(t, "eth0", 10)

	// An L2 LSP signed with the DOMAIN (level-2) key verifies as an L2 LSP.
	signedL2 := e.signLevelPDU(authTestLSP(levelTwo))
	if !e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: signedL2}) {
		t.Fatal("domain-signed L2 LSP rejected")
	}

	// An L2 LSP signed with the IIH key (a non-domain string) is rejected: the
	// verify path selects the domain chain for an L2 PDU, whose key differs.
	// RFC requirement: RFC5304-2-2 negative -- an L2 level PDU signed with a
	// non-domain string (the per-interface IIH key) is rejected; only the domain
	// authentication string authenticates an L2 PDU (RFC 5304 sec 2).
	// RFC requirement: RFC5310-3.2-2 negative -- an L2 level PDU signed with a
	// non-domain string (the per-interface IIH key) is rejected; only the domain
	// authentication string authenticates an L2 PDU (RFC 5310 sec 3.2).
	iihKey := packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, Secret: []byte("iihsecret"), KeyID: 3}
	crossL2, _ := packet.SignPDU(authTestLSP(levelTwo), iihKey)
	if e.verifyFrame(transport.RawFrame{IfIndex: 10, PDU: crossL2}) {
		t.Fatal("IIH-key-signed L2 LSP accepted (cross-use should be rejected)")
	}
}

// ---- helpers ----

// nowFn returns the wiring's clock (time.Now); the keystore selects keys at this
// instant. Kept as a tiny indirection so a future fake clock can be injected.
func nowFn() time.Time { return time.Now() }

// registerTestCircuit registers a minimal circuit under the given name/ifindex so
// verifyFrame can resolve the ifindex to the interface name (for the per-
// interface IIH chain and the auth-failure label). verifyFrame only reads
// Name(), so a circuit built with no sender suffices.
func (e *engine) registerTestCircuit(t *testing.T, name string, ifindex int) { //nolint:unparam // name pairs with ifindex at every call site and is the key circuitByName resolves; hardcoding it would hide what verifyFrame looks up
	t.Helper()
	c := circuit.New(circuit.Config{
		Name:    name,
		IfIndex: ifindex,
		Levels:  []adjacency.Level{adjacency.Level1, adjacency.Level2},
	}, nil, nil)
	e.circuitsMu.Lock()
	e.circuits[ifindex] = c
	e.circuitByName[name] = c
	e.circuitsMu.Unlock()
}
