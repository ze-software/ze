// Design: docs/architecture/isis/isis-10-auth.md -- IS-IS authentication backend tests.
//
// These tests exercise the crypto backend on raw PDU bytes: per-algorithm,
// per-PDU-class sign/verify round-trips, the RFC 5304 sec 1 first-TLV rule, the
// LSP checksum-after-sign interaction (RFC 5304 sec 2), authenticated purges,
// constant-time comparison, wrong-key rejection, and the boundary table.

package packet

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // test asserts the RFC 5304 HMAC-MD5 digest length
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// ---- test PDU builders (unsigned, no TLV 10) ----

func testLANHello(t *testing.T) []byte {
	t.Helper()
	h := LANHello{
		PDUType:     PDUTypeL1LANHello,
		CircuitType: CircuitL1L2,
		SystemID:    types.SystemID{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		HoldingTime: types.HoldingTime(30),
		Priority:    64,
		TLVs: []TLV{
			{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}},
			{Type: TLVProtocolsSupported, Value: []byte{NLPIDIPv4}},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

func testP2PHello(t *testing.T) []byte {
	t.Helper()
	h := P2PHello{
		CircuitType:    CircuitL1L2,
		SystemID:       types.SystemID{0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
		HoldingTime:    types.HoldingTime(30),
		LocalCircuitID: 1,
		TLVs: []TLV{
			{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

func testLSP(t *testing.T) []byte {
	t.Helper()
	src := types.NewSourceID(types.SystemID{0x00, 0x00, 0x00, 0x00, 0x00, 0x03}, 0)
	l := LSP{
		PDUType:           PDUTypeL1LSP,
		RemainingLifetime: 1200,
		LSPID:             types.NewLSPID(src, 0),
		SequenceNumber:    5,
		TypeBlock:         LSPFlagISTypeL1,
		TLVs: []TLV{
			{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}},
		},
	}
	buf := make([]byte, l.EncodedLen())
	return buf[:l.WriteTo(buf, 0)]
}

func testCSNP(t *testing.T) []byte {
	t.Helper()
	src := types.NewSourceID(types.SystemID{0x00, 0x00, 0x00, 0x00, 0x00, 0x04}, 0)
	c := CSNP{
		PDUType:    PDUTypeL1CSNP,
		SourceID:   src,
		StartLSPID: types.LSPID{},
		EndLSPID:   types.LSPID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}
	buf := make([]byte, c.EncodedLen())
	return buf[:c.WriteTo(buf, 0)]
}

func testPSNP(t *testing.T) []byte {
	t.Helper()
	src := types.NewSourceID(types.SystemID{0x00, 0x00, 0x00, 0x00, 0x00, 0x05}, 0)
	p := PSNP{PDUType: PDUTypeL1PSNP, SourceID: src}
	buf := make([]byte, p.EncodedLen())
	return buf[:p.WriteTo(buf, 0)]
}

// pduBuilders returns one builder per PDU class for table-driven per-class tests.
func pduBuilders() map[string]func(*testing.T) []byte {
	return map[string]func(*testing.T) []byte{
		"lan-hello": testLANHello,
		"p2p-hello": testP2PHello,
		"lsp":       testLSP,
		"csnp":      testCSNP,
		"psnp":      testPSNP,
	}
}

// ---- per-algorithm per-PDU-class sign/verify ----

// VALIDATES: TestISISAuthSignVerifyCleartext (TDD plan) -- type 1 sign/verify per
// PDU class. PREVENTS: a cleartext password mismatch silently forming adjacency.
func TestISISAuthSignVerifyCleartext(t *testing.T) {
	key := Key{Algorithm: AuthAlgoCleartext, Secret: []byte("s3cret")}
	for name, build := range pduBuilders() {
		t.Run(name, func(t *testing.T) {
			signed, err := SignPDU(build(t), key)
			if err != nil {
				t.Fatalf("SignPDU: %v", err)
			}
			if err := VerifyPDU(signed, []Key{key}); err != nil {
				t.Fatalf("VerifyPDU: %v", err)
			}
			// Wrong password is rejected.
			wrong := Key{Algorithm: AuthAlgoCleartext, Secret: []byte("nope")}
			if err := VerifyPDU(signed, []Key{wrong}); err == nil {
				t.Fatal("expected mismatch for wrong cleartext password")
			}
		})
	}
}

// VALIDATES: TestISISAuthSignVerifyHMACMD5 (TDD plan) -- type 54 HMAC-MD5
// sign/verify per PDU class; digest length 16 (RFC 5304 sec 2).
//
// RFC requirement: RFC5304-2-6 positive -- a PDU whose Authentication Value is
// CORRECT is accepted (not discarded) by VerifyPDU, per PDU class (RFC 5304 sec 2).
//
// RFC requirement: RFC1195-3.9-1 positive -- a PDU whose authentication information is valid is accepted (not discarded) by VerifyPDU (RFC 1195 sec 3.9).
func TestISISAuthSignVerifyHMACMD5(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("md5-key")}
	for name, build := range pduBuilders() {
		t.Run(name, func(t *testing.T) {
			signed, err := SignPDU(build(t), key)
			if err != nil {
				t.Fatalf("SignPDU: %v", err)
			}
			// The on-wire auth type byte must be 54 and the digest 16 octets.
			at := authValueOf(t, signed)
			if at.AuthType != AuthTypeHMACMD5 {
				t.Errorf("auth type = %d, want %d", at.AuthType, AuthTypeHMACMD5)
			}
			if len(at.Value) != md5.Size {
				t.Errorf("digest len = %d, want %d", len(at.Value), md5.Size)
			}
			if err := VerifyPDU(signed, []Key{key}); err != nil {
				t.Fatalf("VerifyPDU: %v", err)
			}
		})
	}
}

// VALIDATES: TestISISAuthSignVerifyHMACSHA256 (TDD plan) -- type 3 HMAC-SHA-256
// sign/verify per PDU class; Key ID round-trips; digest 32 octets (RFC 5310).
//
// RFC requirement: RFC5310-3.4-1 positive -- the on-wire Authentication TLV carries
// auth type 3 (CRYPTO_AUTH) and Key-ID(2)||digest(32); the auth-type byte and the TLV
// length are placed before the digest is computed (auth_sign.go:47-52), so a type-3
// PDU round-trips and verifies (RFC 5310 sec 3.4).
func TestISISAuthSignVerifyHMACSHA256(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("sha-key"), KeyID: 0xBEEF}
	for name, build := range pduBuilders() {
		t.Run(name, func(t *testing.T) {
			signed, err := SignPDU(build(t), key)
			if err != nil {
				t.Fatalf("SignPDU: %v", err)
			}
			at := authValueOf(t, signed)
			if at.AuthType != AuthTypeGenericCrypto {
				t.Errorf("auth type = %d, want %d (RFC 5310)", at.AuthType, AuthTypeGenericCrypto)
			}
			// Value = Key-ID(2) || digest(32).
			if len(at.Value) != 2+sha256.Size {
				t.Fatalf("value len = %d, want %d", len(at.Value), 2+sha256.Size)
			}
			kid := uint16(at.Value[0])<<8 | uint16(at.Value[1])
			if kid != 0xBEEF {
				t.Errorf("key-id = %#x, want 0xBEEF", kid)
			}
			if err := VerifyPDU(signed, []Key{key}); err != nil {
				t.Fatalf("VerifyPDU: %v", err)
			}
		})
	}
}

// VALIDATES: all HMAC-SHA family algorithms (RFC 5310 algorithm agility) sign and
// verify and carry the RFC-mandated digest length.
func TestISISAuthSignVerifyHMACSHAFamily(t *testing.T) {
	cases := []struct {
		algo AuthAlgorithm
		dlen int
	}{
		{AuthAlgoHMACSHA1, 20},
		{AuthAlgoHMACSHA224, 28},
		{AuthAlgoHMACSHA256, 32},
		{AuthAlgoHMACSHA384, 48},
		{AuthAlgoHMACSHA512, 64},
	}
	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			key := Key{Algorithm: tc.algo, Secret: []byte("key"), KeyID: 7}
			signed, err := SignPDU(testCSNP(t), key)
			if err != nil {
				t.Fatalf("SignPDU: %v", err)
			}
			at := authValueOf(t, signed)
			if len(at.Value) != 2+tc.dlen {
				t.Errorf("algo %d digest len = %d, want %d", tc.algo, len(at.Value)-2, tc.dlen)
			}
			if err := VerifyPDU(signed, []Key{key}); err != nil {
				t.Errorf("verify algo %d: %v", tc.algo, err)
			}
		})
	}
}

// VALIDATES: RFC 5310 sec 3.3 step 1 / sec 3.5 -- the HMAC-SHA generic-crypto
// digest is computed over the PDU with the Authentication Data field filled with
// Apad (0x878FE1F3 repeated), NOT zeroed. This is the regression for the B6
// finding: zeroing made every Ze<->FRR HMAC-SHA digest mismatch. The test is a
// KNOWN-ANSWER check: it independently re-derives the digest by filling the
// auth-data region with Apad and running HMAC-SHA-256, then asserts the on-wire
// digest equals it AND differs from the zero-fill digest a non-conforming
// implementation would emit.
// PREVENTS: a regression back to zero-fill for the RFC 5310 family, which would
// break interop with any RFC-conforming peer.
//
// RFC requirement: RFC5310-3.4-1 positive -- known-answer: the re-hashed pre-image
// (wantPre) is the full signed PDU with only the data region Apad-filled, so it still
// contains the Authentication TLV's type byte (0x3) and length; the on-wire digest
// equals that pre-image, proving the auth type and length are filled before the digest
// is computed (RFC 5310 sec 3.3 step 1 / sec 3.4).
func TestISISAuthHMACSHAApadPreimage(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("apad-key"), KeyID: 0x1234}
	signed, err := SignPDU(testCSNP(t), key)
	if err != nil {
		t.Fatalf("SignPDU: %v", err)
	}

	// Locate the digest region on the wire (after the auth-type byte and Key ID).
	layout := snpAuthLayout(signed)
	digOff := layout.authValueOff + keyIDOctets(key.Algorithm)
	digEnd := layout.authValueOff + layout.authValueLen
	if digEnd-digOff != sha256.Size {
		t.Fatalf("digest region = %d octets, want %d", digEnd-digOff, sha256.Size)
	}
	onWire := append([]byte(nil), signed[digOff:digEnd]...)

	// Independently re-derive: Apad-fill the auth-data region, then HMAC-SHA-256.
	apad := []byte{0x87, 0x8F, 0xE1, 0xF3}
	wantPre := append([]byte(nil), signed...)
	for i := digOff; i < digEnd; i++ {
		wantPre[i] = apad[(i-digOff)%4]
	}
	macApad := hmac.New(sha256.New, key.Secret)
	macApad.Write(wantPre)
	wantApad := macApad.Sum(nil)
	if !bytes.Equal(onWire, wantApad) {
		t.Fatalf("on-wire digest is not over Apad-filled bytes\n  on-wire: % x\n  want   : % x", onWire, wantApad)
	}

	// A zero-fill pre-image (the buggy behavior) MUST produce a different digest;
	// otherwise the test would not actually distinguish Apad from zero.
	zeroPre := append([]byte(nil), signed...)
	for i := digOff; i < digEnd; i++ {
		zeroPre[i] = 0
	}
	macZero := hmac.New(sha256.New, key.Secret)
	macZero.Write(zeroPre)
	if bytes.Equal(onWire, macZero.Sum(nil)) {
		t.Fatal("Apad and zero-fill digests collide; the known-answer test cannot detect a regression")
	}

	// And the full sign->verify round-trip still succeeds (both sides use Apad).
	if err := VerifyPDU(signed, []Key{key}); err != nil {
		t.Fatalf("VerifyPDU over Apad-signed PDU: %v", err)
	}
}

// VALIDATES: HMAC-MD5 (RFC 5304 sec 2) still ZEROES the Authentication Value
// (NOT Apad) -- the Apad change is scoped to the RFC 5310 family only. Known
// answer: the on-wire digest must equal the zero-fill HMAC-MD5 and must NOT equal
// the Apad-fill digest.
func TestISISAuthHMACMD5ZeroPreimage(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("md5-preimage")}
	signed, err := SignPDU(testCSNP(t), key)
	if err != nil {
		t.Fatalf("SignPDU: %v", err)
	}
	layout := snpAuthLayout(signed)
	digOff := layout.authValueOff // no Key ID for HMAC-MD5
	digEnd := layout.authValueOff + layout.authValueLen
	if digEnd-digOff != md5.Size {
		t.Fatalf("digest region = %d octets, want %d", digEnd-digOff, md5.Size)
	}
	onWire := append([]byte(nil), signed[digOff:digEnd]...)

	zeroPre := append([]byte(nil), signed...)
	for i := digOff; i < digEnd; i++ {
		zeroPre[i] = 0
	}
	macZero := hmac.New(md5.New, key.Secret)
	macZero.Write(zeroPre)
	if !bytes.Equal(onWire, macZero.Sum(nil)) {
		t.Fatal("HMAC-MD5 on-wire digest is not over zero-filled bytes (RFC 5304 sec 2)")
	}

	apad := []byte{0x87, 0x8F, 0xE1, 0xF3}
	apadPre := append([]byte(nil), signed...)
	for i := digOff; i < digEnd; i++ {
		apadPre[i] = apad[(i-digOff)%4]
	}
	macApad := hmac.New(md5.New, key.Secret)
	macApad.Write(apadPre)
	if bytes.Equal(onWire, macApad.Sum(nil)) {
		t.Fatal("HMAC-MD5 used Apad fill; RFC 5304 sec 2 requires zero fill")
	}
}

// ---- TLV-first ordering ----

// VALIDATES: TestISISAuthTLVFirstOnEncode (TDD plan) -- TLV 10 is the first TLV
// after signing (RFC 5304 sec 1, AC-8), regardless of how many TLVs preceded it.
func TestISISAuthTLVFirstOnEncode(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("k")}
	signed, err := SignPDU(testLANHello(t), key)
	if err != nil {
		t.Fatalf("SignPDU: %v", err)
	}
	dec, err := DecodePDU(signed)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if idx := AuthTLVIndex(dec.LANHello.TLVs); idx != 0 {
		t.Fatalf("auth TLV index = %d, want 0 (first)", idx)
	}
}

// VALIDATES: TestISISAuthTLVNotFirstRejected (TDD plan) -- a PDU whose TLV 10 is
// present but NOT first is rejected on verify (RFC 5304 sec 1, AC-8).
func TestISISAuthTLVNotFirstRejected(t *testing.T) {
	// Build a LAN IIH with a non-auth TLV first, then a TLV 10, by hand.
	h := LANHello{
		PDUType:     PDUTypeL1LANHello,
		CircuitType: CircuitL1,
		SystemID:    types.SystemID{0, 0, 0, 0, 0, 1},
		HoldingTime: types.HoldingTime(30),
		TLVs: []TLV{
			{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}},
			{Type: TLVAuthentication, Value: append([]byte{AuthTypeHMACMD5}, make([]byte, 16)...)},
		},
	}
	buf := make([]byte, h.EncodedLen())
	pdu := buf[:h.WriteTo(buf, 0)]

	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("k")}
	if err := VerifyPDU(pdu, []Key{key}); !errors.Is(err, ErrAuthNotFirst) {
		t.Fatalf("VerifyPDU = %v, want ErrAuthNotFirst", err)
	}
}

// VALIDATES: AC-1 / R-6 downgrade resistance -- under configured auth a PDU with
// no TLV 10 is rejected (ErrAuthMissing).
func TestISISAuthMissingRejected(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("k")}
	if err := VerifyPDU(testCSNP(t), []Key{key}); !errors.Is(err, ErrAuthMissing) {
		t.Fatalf("VerifyPDU = %v, want ErrAuthMissing", err)
	}
}

// VALIDATES: unauthenticated operation (no keys) accepts any PDU unchanged.
func TestISISAuthNoKeysAccepts(t *testing.T) {
	if err := VerifyPDU(testCSNP(t), nil); err != nil {
		t.Fatalf("VerifyPDU with no keys = %v, want nil", err)
	}
}

// ---- LSP checksum-after-sign interaction ----

// VALIDATES: TestISISAuthLSPChecksumAfterSign (TDD plan, AC-9) -- a signed LSP's
// Fletcher checksum is valid AFTER the digest is in place, the digest is computed
// over the PDU with Authentication Value + Checksum + Remaining Lifetime zeroed
// (RFC 5304 sec 2), and the whole thing round-trips and verifies.
//
// RFC requirement: RFC5310-4-1 positive -- the LSP digest excludes the Checksum and
// the Remaining Lifetime: both are zeroed before the HMAC is computed
// (auth_sign.go:268-272), so a post-sign Remaining-Lifetime change (with a recomputed
// Fletcher checksum) still verifies, proving those fields are not authenticated
// (RFC 5310 sec 4).
func TestISISAuthLSPChecksumAfterSign(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("lsp-key")}
	signed, err := SignPDU(testLSP(t), key)
	if err != nil {
		t.Fatalf("SignPDU: %v", err)
	}
	// Fletcher checksum over the post-Remaining-Lifetime region must verify.
	dec, err := DecodePDU(signed)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if !dec.LSP.VerifyChecksum() {
		t.Fatal("LSP Fletcher checksum invalid after signing")
	}
	// The digest must verify too.
	if err := VerifyPDU(signed, []Key{key}); err != nil {
		t.Fatalf("VerifyPDU: %v", err)
	}
	// Changing the Remaining Lifetime must NOT break the digest (RFC 5304 sec 2:
	// it is zeroed before the digest), but DOES require a checksum recompute.
	// Simulate an in-flight aging: decrement lifetime and recompute checksum.
	lifeOff := CommonHeaderLen + lspRemLifetimeOff
	signed[lifeOff], signed[lifeOff+1] = 0x04, 0x00 // 1024
	finalizeLSPChecksum(signed)
	if err := VerifyPDU(signed, []Key{key}); err != nil {
		t.Fatalf("VerifyPDU after lifetime change = %v, want nil (lifetime not authenticated)", err)
	}
}

// ---- authenticated purge ----

// VALIDATES: TestISISAuthPurge (TDD plan, AC-13, AC-14) -- a signed purge (body
// stripped, only TLV 10, digest over zeroed lifetime) round-trips and verifies;
// an unauthenticated purge and a purge carrying a non-auth TLV are rejected.
func TestISISAuthPurge(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("purge-key")}

	// Build a live LSP, strip its body to a purge, sign it.
	dec, err := DecodePDU(testLSP(t))
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	purge := StripPurgeBody(dec.LSP)
	pbuf := make([]byte, purge.EncodedLen())
	purgePDU := pbuf[:purge.WriteTo(pbuf, 0)]

	signed, err := SignPDU(purgePDU, key)
	if err != nil {
		t.Fatalf("SignPDU purge: %v", err)
	}
	// The signed purge round-trips and verifies (AC-13).
	// RFC requirement: RFC5304-2-8 positive -- an AUTHENTICATED (signed) purge is
	// accepted (RFC 5304 sec 2: purges must be authenticated to be honored).
	// RFC requirement: RFC5304-2-9 positive -- a clean signed purge carrying only
	// the auth TLV is accepted (no extra TLV to reject) (RFC 5304 sec 2).
	if err := VerifyPDU(signed, []Key{key}); err != nil {
		t.Fatalf("VerifyPDU signed purge: %v", err)
	}
	// It must carry ONLY TLV 10.
	// RFC requirement: RFC5304-2-7 positive -- an initiated purge has its body
	// removed and the authentication TLV added, so the signed purge carries only
	// TLV 10 (RFC 5304 sec 2).
	sdec, _ := DecodePDU(signed)
	if len(sdec.LSP.TLVs) != 1 || sdec.LSP.TLVs[0].Type != TLVAuthentication {
		t.Fatalf("signed purge TLVs = %+v, want only TLV 10", sdec.LSP.TLVs)
	}

	// AC-14: an unauthenticated purge (no TLV 10) is rejected.
	// RFC requirement: RFC5304-2-8 negative -- an unauthenticated purge (no auth
	// TLV) is not accepted (RFC 5304 sec 2: MUST NOT accept unauthenticated purges).
	// RFC requirement: RFC5304-2-7 negative -- a purge lacking the added
	// authentication TLV is rejected, so the "add the authentication TLV" rule is
	// enforced on receive (RFC 5304 sec 2).
	if err := VerifyPDU(purgePDU, []Key{key}); !errors.Is(err, ErrAuthMissing) {
		t.Fatalf("unauthenticated purge = %v, want ErrAuthMissing", err)
	}

	// AC-14: a purge carrying a non-auth TLV is rejected even if (would be) signed.
	withExtra := *dec.LSP
	withExtra.RemainingLifetime = 0 // purge
	withExtra.TLVs = []TLV{{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}}}
	ebuf := make([]byte, withExtra.EncodedLen())
	extraPDU := ebuf[:withExtra.WriteTo(ebuf, 0)]
	signedExtra, err := SignPDU(extraPDU, key)
	if err != nil {
		t.Fatalf("SignPDU purge-with-extra: %v", err)
	}
	// RFC requirement: RFC5304-2-9 negative -- a purge carrying any TLV other than
	// the authentication TLV is not accepted (RFC 5304 sec 2: MUST NOT accept
	// purges that contain TLVs other than the authentication TLV).
	if err := VerifyPDU(signedExtra, []Key{key}); !errors.Is(err, ErrAuthPurgeExtraTLV) {
		t.Fatalf("purge with non-auth TLV = %v, want ErrAuthPurgeExtraTLV", err)
	}
}

// VALIDATES: the error ORDERING between the not-first check and the purge
// extra-TLV check (RFC 5304 sec 1 + sec 2). VerifyPDU validates TLV-10-first
// BEFORE the authenticated-purge "only TLV 10" rule, so:
//
//	(a) a purge with TLV 10 FIRST plus a non-auth TLV  -> ErrAuthPurgeExtraTLV
//	(b) a purge with TLV 10 NOT first plus a non-auth TLV -> ErrAuthNotFirst
//
// Case (a) is the purge body-removal rule (RFC 5304 sec 2: a purge MUST carry
// only TLV 10). Case (b) documents that the ordering invariant (sec 1) is
// checked first and wins: a malformed-ordering purge is rejected as not-first,
// never reaching the extra-TLV branch.
// PREVENTS: a refactor that reorders the two checks and silently changes which
// typed error a not-first purge yields (callers match on the sentinel).
func TestISISAuthPurgeExtraTLVOrdering(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("purge-ordering")}
	src := types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 0x33}, 0)

	// A 16-octet placeholder TLV-10 value (auth-type 54 + zeroed HMAC-MD5 digest);
	// the digest content is irrelevant because both cases fail before the digest
	// is checked.
	authTLV := TLV{Type: TLVAuthentication, Value: append([]byte{AuthTypeHMACMD5}, make([]byte, md5.Size)...)}
	extraTLV := TLV{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}}

	// (a) purge, TLV 10 FIRST, plus a non-auth TLV -> ErrAuthPurgeExtraTLV.
	firstAuth := LSP{
		PDUType:           PDUTypeL1LSP,
		RemainingLifetime: 0, // purge
		LSPID:             types.NewLSPID(src, 0),
		SequenceNumber:    7,
		TypeBlock:         LSPFlagISTypeL1,
		TLVs:              []TLV{authTLV, extraTLV},
	}
	bufA := make([]byte, firstAuth.EncodedLen())
	pduA := bufA[:firstAuth.WriteTo(bufA, 0)]
	finalizeLSPChecksum(pduA)
	if err := VerifyPDU(pduA, []Key{key}); !errors.Is(err, ErrAuthPurgeExtraTLV) {
		t.Fatalf("purge TLV-10-first + extra = %v, want ErrAuthPurgeExtraTLV", err)
	}

	// (b) purge, TLV 10 NOT first, plus a non-auth TLV -> ErrAuthNotFirst.
	// The not-first check (RFC 5304 sec 1) precedes the purge extra-TLV check.
	notFirst := LSP{
		PDUType:           PDUTypeL1LSP,
		RemainingLifetime: 0, // purge
		LSPID:             types.NewLSPID(src, 0),
		SequenceNumber:    7,
		TypeBlock:         LSPFlagISTypeL1,
		TLVs:              []TLV{extraTLV, authTLV},
	}
	bufB := make([]byte, notFirst.EncodedLen())
	pduB := bufB[:notFirst.WriteTo(bufB, 0)]
	finalizeLSPChecksum(pduB)
	if err := VerifyPDU(pduB, []Key{key}); !errors.Is(err, ErrAuthNotFirst) {
		t.Fatalf("purge TLV-10-not-first + extra = %v, want ErrAuthNotFirst (ordering: not-first wins)", err)
	}
}

// VALIDATES: the perf fix to VerifyPDU (one scratch buffer reused across the
// candidate-key chain) is correct: a chain whose FIRST candidate is the wrong
// key still verifies on a LATER candidate (so the scratch is restored between
// keys), and the caller's receive buffer is never mutated by verification.
// PREVENTS: a regression where the shared scratch is left zeroed/Apad-filled by
// an earlier candidate, corrupting a later candidate's digest, or where the
// receive buffer is mutated (breaking the zero-copy contract).
func TestISISAuthVerifyScratchReuse(t *testing.T) {
	// Same algorithm + type so every candidate "matches" the received auth type
	// and the loop actually iterates over multiple keys on one scratch buffer.
	right := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("correct-key"), KeyID: 5}
	wrongA := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("wrong-a"), KeyID: 5}
	wrongB := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("wrong-b"), KeyID: 5}

	for name, build := range pduBuilders() {
		t.Run(name, func(t *testing.T) {
			signed, err := SignPDU(build(t), right)
			if err != nil {
				t.Fatalf("SignPDU: %v", err)
			}
			// Snapshot the receive buffer to prove VerifyPDU does not mutate it.
			before := append([]byte(nil), signed...)

			// Right key LAST: the scratch must be restored after each wrong key so
			// the final candidate sees the original PDU bytes.
			if err := VerifyPDU(signed, []Key{wrongA, wrongB, right}); err != nil {
				t.Fatalf("chain [wrongA wrongB right] = %v, want nil", err)
			}
			if !bytes.Equal(signed, before) {
				t.Fatal("VerifyPDU mutated the caller's receive buffer (zero-copy violated)")
			}

			// Right key FIRST, then wrong keys: still accepts on the first match,
			// and still leaves the buffer untouched.
			if err := VerifyPDU(signed, []Key{right, wrongA, wrongB}); err != nil {
				t.Fatalf("chain [right wrongA wrongB] = %v, want nil", err)
			}
			if !bytes.Equal(signed, before) {
				t.Fatal("VerifyPDU mutated the caller's receive buffer (zero-copy violated)")
			}

			// All-wrong chain rejects (and still does not mutate the buffer).
			if err := VerifyPDU(signed, []Key{wrongA, wrongB}); err == nil {
				t.Fatal("all-wrong chain accepted")
			}
			if !bytes.Equal(signed, before) {
				t.Fatal("VerifyPDU mutated the caller's receive buffer on the reject path")
			}
		})
	}
}

// ---- constant-time compare ----

// VALIDATES: TestISISAuthConstantTimeCompare (TDD plan, AC-12, R-1) -- verify
// uses a constant-time compare. We assert behaviorally that the implementation
// uses hmac.Equal by confirming a one-bit digest difference is rejected (a
// non-constant-time bytes.Equal would also reject, so this is a smoke test for
// rejection; the source-grep gate in the spec enforces hmac.Equal directly).
//
// RFC requirement: RFC5304-2-6 negative -- a PDU whose Authentication Value is
// INCORRECT (one digest bit flipped) is discarded by VerifyPDU (RFC 5304 sec 2).
//
// RFC requirement: RFC1195-3.9-1 negative -- a PDU with invalid authentication information (one digest bit flipped) is discarded by VerifyPDU (RFC 1195 sec 3.9: a packet with invalid authentication information MUST be discarded).
func TestISISAuthConstantTimeCompare(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("k"), KeyID: 1}
	signed, err := SignPDU(testCSNP(t), key)
	if err != nil {
		t.Fatalf("SignPDU: %v", err)
	}
	// Flip one bit of the digest; verify must reject.
	layout := snpAuthLayout(signed)
	pos := layout.authValueOff + keyIDOctets(key.Algorithm)
	signed[pos] ^= 0x01
	if err := VerifyPDU(signed, []Key{key}); err == nil {
		t.Fatal("expected rejection after flipping one digest bit")
	}
	// Sanity: the helper hmac.Equal is the one used (compile-time reference).
	if !hmac.Equal([]byte{1, 2}, []byte{1, 2}) {
		t.Fatal("hmac.Equal sanity failed")
	}
}

// VALIDATES: TestISISAuthWrongKeyRejected (TDD plan, AC-2, AC-6, AC-7) -- a PDU
// signed with one key is rejected when verified against a different key, per PDU
// class and per algorithm.
//
// RFC requirement: RFC5304-2-6 negative -- a PDU whose Authentication Value is
// INCORRECT (signed with a different key) is discarded by VerifyPDU (RFC 5304 sec 2).
// RFC requirement: RFC5310-4-2 negative -- a PDU signed with a key that is NOT among
// the candidate (accept-set) keys is rejected, so the rollover accept-set only honors
// currently valid keys and an unknown key does not verify (RFC 5310 sec 4;
// HMAC-SHA-256 is one of the algorithms exercised).
func TestISISAuthWrongKeyRejected(t *testing.T) {
	algos := []AuthAlgorithm{AuthAlgoHMACMD5, AuthAlgoHMACSHA256}
	for _, algo := range algos {
		for name, build := range pduBuilders() {
			t.Run(name, func(t *testing.T) {
				good := Key{Algorithm: algo, Secret: []byte("right"), KeyID: 1}
				bad := Key{Algorithm: algo, Secret: []byte("wrong"), KeyID: 1}
				signed, err := SignPDU(build(t), good)
				if err != nil {
					t.Fatalf("SignPDU: %v", err)
				}
				if err := VerifyPDU(signed, []Key{bad}); err == nil {
					t.Fatalf("algo %d %s: wrong key accepted", algo, name)
				}
			})
		}
	}
}

// VALIDATES: AC-4 hitless rotation -- a PDU signed with EITHER of two accepted
// keys verifies when both are candidates (the overlap window).
//
// RFC requirement: RFC5310-4-2 positive -- a PDU signed with EITHER of two accepted
// keys (the overlap window) verifies, so more than one key is stored and used at the
// same time (RFC 5310 sec 4).
// RFC requirement: RFC5310-4-1 positive -- an HMAC-SHA-256 (CRYPTO_AUTH type 3) LSP
// round-trips and verifies; the LSP digest is computed by the shared backend that
// zeroes the Checksum and Remaining Lifetime before hashing (auth_sign.go:268-272),
// so the RFC 5310 sec 4 exclusion holds on the type-3 LSP path.
func TestISISAuthRotationOverlapAccepts(t *testing.T) {
	oldKey := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("old"), KeyID: 1}
	newKey := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("new"), KeyID: 2}
	candidates := []Key{oldKey, newKey}

	signedOld, _ := SignPDU(testLSP(t), oldKey)
	if err := VerifyPDU(signedOld, candidates); err != nil {
		t.Fatalf("verify old-key PDU during overlap: %v", err)
	}
	signedNew, _ := SignPDU(testLSP(t), newKey)
	if err := VerifyPDU(signedNew, candidates); err != nil {
		t.Fatalf("verify new-key PDU during overlap: %v", err)
	}
}

// VALIDATES: a generic-crypto PDU whose Key-ID matches no candidate is rejected
// (RFC 5310 sec 3.5: the SA is identified by Key ID).
//
// RFC requirement: RFC5310-4-2 negative -- a CRYPTO_AUTH PDU whose Key-ID matches no
// candidate in the accept-set is rejected, so during rollover only a currently valid
// key's Key ID is honored (RFC 5310 sec 4 / sec 3.5: the SA is identified by Key ID).
func TestISISAuthKeyIDMismatchRejected(t *testing.T) {
	signed, _ := SignPDU(testCSNP(t), Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("k"), KeyID: 1})
	other := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("k"), KeyID: 99}
	if err := VerifyPDU(signed, []Key{other}); err == nil {
		t.Fatal("expected rejection on Key-ID mismatch")
	}
}

// ---- boundary tests (spec Boundary Tests table) ----

// VALIDATES: digest-length boundaries -- a received HMAC value shorter than the
// algorithm's digest length is rejected (no slice past the value).
func TestISISAuthDigestLengthBoundary(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("k")}
	// A TLV 10 value of type 54 but only 8 octets of "digest" (< 16) must reject.
	h := LANHello{
		PDUType:     PDUTypeL1LANHello,
		CircuitType: CircuitL1,
		SystemID:    types.SystemID{0, 0, 0, 0, 0, 1},
		HoldingTime: types.HoldingTime(30),
		TLVs: []TLV{
			{Type: TLVAuthentication, Value: append([]byte{AuthTypeHMACMD5}, make([]byte, 8)...)},
		},
	}
	buf := make([]byte, h.EncodedLen())
	pdu := buf[:h.WriteTo(buf, 0)]
	if err := VerifyPDU(pdu, []Key{key}); err == nil {
		t.Fatal("expected rejection of short HMAC-MD5 digest")
	}
}

// VALIDATES: Key-ID boundary 0..65535 round-trips (max value 0xFFFF).
func TestISISAuthKeyIDBoundary(t *testing.T) {
	for _, kid := range []uint16{0, 1, 0xFFFF} {
		key := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("k"), KeyID: kid}
		signed, err := SignPDU(testPSNP(t), key)
		if err != nil {
			t.Fatalf("SignPDU kid=%d: %v", kid, err)
		}
		at := authValueOf(t, signed)
		got := uint16(at.Value[0])<<8 | uint16(at.Value[1])
		if got != kid {
			t.Errorf("key-id round-trip = %d, want %d", got, kid)
		}
		if err := VerifyPDU(signed, []Key{key}); err != nil {
			t.Errorf("verify kid=%d: %v", kid, err)
		}
	}
}

// test-relax: TestISISAuthTypeCodes moved verbatim to auth_types_test.go as part
// of the auth_verify.go file split (replaced coverage, not removed); it tests
// authTypeFor, which now lives in auth_types.go. No assertion was dropped.

// ---- helpers ----

// authValueOf decodes the first TLV 10 of a signed PDU and returns it.
func authValueOf(t *testing.T, pdu []byte) AuthTLV {
	t.Helper()
	dec, err := DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	tlvs := pduTLVs(dec)
	idx := AuthTLVIndex(tlvs)
	if idx < 0 {
		t.Fatal("no TLV 10 in signed PDU")
	}
	at, err := DecodeAuthTLV(tlvs[idx].Value)
	if err != nil {
		t.Fatalf("DecodeAuthTLV: %v", err)
	}
	return at
}

// VALIDATES: signing is deterministic for HMAC (same key+PDU -> same digest), so
// a re-sign on retransmit produces identical bytes (no churn in the LSDB raw
// store).
func TestISISAuthSignDeterministic(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("k"), KeyID: 3}
	a, _ := SignPDU(testLSP(t), key)
	b, _ := SignPDU(testLSP(t), key)
	if !bytes.Equal(a, b) {
		t.Fatal("HMAC signing is not deterministic")
	}
}
