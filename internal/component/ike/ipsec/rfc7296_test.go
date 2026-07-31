// VALIDATES: the RFC 7296 Section 2.15 obligations on the management interface that supplies
// a pre-shared secret: it accepts ASCII strings of at least 64 octets, and it does not add a
// null terminator before the secret is used. Each test carries an `RFC requirement:` tag
// binding it to its checklist id.
// PREVENTS: a config-parse change that caps, pads, or terminates an operator's pre-shared
// secret, which would silently break authentication against a conforming peer.
package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/secret"
)

// pskPeerTree builds a site-to-site peer whose authentication mode is pre-shared-secret.
func pskPeerTree(t *testing.T, psk string) *IPsecConfig {
	t.Helper()
	tree := makePeerTree("psk-length", peerOpts{
		ikeGroupName: "IKE-RW",
		espGroupName: "ESP-RW",
		ikeGroup:     "IKE-RW",
		espGroup:     "ESP-RW",
		authMode:     "pre-shared-secret",
		psk:          psk,
	})
	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	return cfg
}

// RFC requirement: RFC7296-2.15-1 positive -- the management interface accepts an ASCII shared
// secret of at least 64 octets. The `pre-shared-secret` YANG leaf is an unconstrained
// `type string` (ipsec/yang/ze-ipsec-conf.yang:344-348). parseAuthConfig assigns that
// value through unchanged (config.go:544-553). A 64-octet secret and a 200-octet secret
// both survive the parse byte for byte.
// RFC requirement: RFC7296-2.15-1 negative -- acceptance follows the operator's input and Ze
// never invents a secret. A peer that supplies no `pre-shared-secret` leaf leaves the
// secret empty.
func TestPSKAcceptsAtLeast64ASCIIOctets(t *testing.T) {
	for _, n := range []int{64, 65, 128, 200} {
		want := strings.Repeat("a", n-1) + "Z"
		cfg := pskPeerTree(t, want)
		peer, ok := cfg.Peers["psk-length"]
		if !ok {
			t.Fatal("peer psk-length is absent from the parsed config")
		}
		if peer.Auth.PSK != want {
			t.Errorf("a %d-octet ASCII secret parsed to %d octets; want it accepted unchanged",
				n, len(peer.Auth.PSK))
		}
		if len(peer.Auth.PSK) != n {
			t.Errorf("secret length = %d octets, want %d", len(peer.Auth.PSK), n)
		}
	}

	// The full printable-ASCII range is accepted, not just letters.
	var ascii strings.Builder
	for c := byte(0x20); c < 0x7f; c++ {
		ascii.WriteByte(c)
	}
	full := ascii.String()
	if len(full) < 64 {
		t.Fatalf("printable ASCII sample is %d octets, want at least 64", len(full))
	}
	cfg := pskPeerTree(t, full)
	if got := cfg.Peers["psk-length"].Auth.PSK; got != full {
		t.Errorf("a %d-octet printable-ASCII secret did not survive the parse", len(full))
	}

	// Negative: no leaf, no secret.
	none := pskPeerTree(t, "")
	if got := none.Peers["psk-length"].Auth.PSK; got != "" {
		t.Errorf("a peer with no pre-shared-secret leaf parsed to secret %q; the management "+
			"interface must not invent one", got)
	}
}

// RFC requirement: RFC7296-2.15-2 positive -- Ze adds no null terminator. parseAuthConfig assigns
// the leaf value to AuthConfig.PSK as a Go string with no padding step. The parsed secret
// is therefore exactly as long as the operator's value, and it ends with the operator's
// last octet, never 0x00. computePSKAuth then feeds []byte(psk) straight into the PRF
// (engine/auth.go:245), so the AUTH key is derived over those octets alone.
// RFC requirement: RFC7296-2.15-2 negative -- the no-terminator rule is not a stripping rule.
// A secret the operator deliberately ends with a NUL keeps that octet, so the parser
// neither adds nor removes trailing bytes.
func TestPSKHasNoNullTerminatorAdded(t *testing.T) {
	const plain = "correct horse battery staple correct horse battery staple 1234567"
	cfg := pskPeerTree(t, plain)
	got := cfg.Peers["psk-length"].Auth.PSK
	if len(got) != len(plain) {
		t.Fatalf("secret length = %d octets, want %d; a terminator was added or removed",
			len(got), len(plain))
	}
	if strings.ContainsRune(got, 0) {
		t.Error("the parsed secret contains a NUL octet; no terminator may be added")
	}
	if got[len(got)-1] != plain[len(plain)-1] {
		t.Errorf("last octet = %q, want %q", got[len(got)-1], plain[len(plain)-1])
	}

	// Negative: a NUL the operator supplied is preserved, so the parser is not trimming.
	withNUL := plain + "\x00"
	cfgNUL := pskPeerTree(t, withNUL)
	gotNUL := cfgNUL.Peers["psk-length"].Auth.PSK
	if len(gotNUL) != len(withNUL) {
		t.Errorf("an operator-supplied trailing NUL changed the length: %d octets, want %d",
			len(gotNUL), len(withNUL))
	}
	if gotNUL[len(gotNUL)-1] != 0 {
		t.Error("an operator-supplied trailing NUL was stripped; the parser must neither add " +
			"nor remove trailing octets")
	}
}

// makeSuitePolicyTree builds an ike-group carrying the named proposals. It exists so the
// management-facility tests can vary every transform independently, which makeIKETree's
// caller in config_test.go does not need and makePeerTree hardcodes.
func makeSuitePolicyTree(t *testing.T, props map[string]ikeProposalOpts) (*IPsecConfig, error) {
	t.Helper()
	return ParseIPsecConfig(makeIKETree("IKE-POLICY", ikeOpts{proposals: props}))
}

// RFC requirement: RFC7296-3.3.4-4 positive -- "All implementations of IKEv2 MUST include a
// management facility that enables a user or system administrator to specify the suites
// that are acceptable for use with IKE" (rfc/full/rfc7296.txt:4771-4773). The facility is
// the `ike-group / proposal` YANG list (ipsec/yang/ze-ipsec-conf.yang, `list proposal`
// under `list ike-group`), parsed by parseIKEProposal. This asserts an administrator's
// choice of suite reaches the parsed policy exactly as written, and that a DIFFERENT
// choice produces a DIFFERENT policy. The two-tree differential is what makes this a
// facility test rather than a parser test.
//
// The trailing sentence of the paragraph bounds all three §3.3.4 rows: "Note that
// cryptographic suites that MUST be implemented need not be configured as acceptable to
// local policy" (rfc/full/rfc7296.txt:4778-4780). So the facility is not required to
// pre-admit anything, which is why an empty policy is not asserted here.
//
// The comparison this policy drives, and the rejection of a proposal it does not
// authorize, are RFC7296-3.3.4-2 and -3, already proven against NegotiateIKE in
// internal/component/ike/crypto/rfc7296_proposal_test.go. This row is the facility's
// EXISTENCE and operator-controllability, which those two presuppose.
func TestIKESuitePolicyIsOperatorSpecified(t *testing.T) {
	cfg, err := makeSuitePolicyTree(t, map[string]ikeProposalOpts{
		"1": {encryption: "aes128gcm", hash: "sha256", dhGroup: "14"},
		"2": {encryption: "aes256", hash: "sha512", dhGroup: "20"},
	})
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	group, ok := cfg.IKEGroups["IKE-POLICY"]
	if !ok {
		t.Fatal("ike-group IKE-POLICY is absent from the parsed config")
	}
	if len(group.Proposals) != 2 {
		t.Fatalf("parsed %d proposals, want 2", len(group.Proposals))
	}
	// Priority order is the list key, so proposal 1 precedes proposal 2.
	if group.Proposals[0].Number != 1 || group.Proposals[1].Number != 2 {
		t.Errorf("proposals are not in priority order: got %d then %d",
			group.Proposals[0].Number, group.Proposals[1].Number)
	}
	// Every transform the administrator named survives the parse exactly.
	if got := group.Proposals[0].DHGroup; got != DHGroup(14) {
		t.Errorf("proposal 1 dh-group = %d, want 14", got)
	}
	if got := group.Proposals[1].DHGroup; got != DHGroup(20) {
		t.Errorf("proposal 2 dh-group = %d, want 20", got)
	}
	if group.Proposals[0].Encryption == group.Proposals[1].Encryption {
		t.Error("both proposals parsed to the same encryption transform; the facility is " +
			"not carrying the administrator's per-proposal choice")
	}
	if group.Proposals[0].Hash == group.Proposals[1].Hash {
		t.Error("both proposals parsed to the same hash transform")
	}

	// The differential: a different configuration yields a different policy. Without
	// this, every assertion above would pass against a parser returning a constant.
	other, err := makeSuitePolicyTree(t, map[string]ikeProposalOpts{
		"1": {encryption: "aes256", hash: "sha512", dhGroup: "19"},
	})
	if err != nil {
		t.Fatalf("ParseIPsecConfig (second tree): %v", err)
	}
	otherGroup := other.IKEGroups["IKE-POLICY"]
	if len(otherGroup.Proposals) == len(group.Proposals) {
		t.Error("two different suite configurations produced the same proposal count; " +
			"the policy is not read from the tree")
	}
	if otherGroup.Proposals[0].DHGroup == group.Proposals[0].DHGroup {
		t.Errorf("two different suite configurations produced the same dh-group (%d); "+
			"the policy is not read from the tree", otherGroup.Proposals[0].DHGroup)
	}
}

// RFC requirement: RFC7296-3.3.4-4 negative -- the facility specifies the suites that are
// ACCEPTABLE, so it must refuse a suite this build cannot honor. A facility that accepts
// a suite the negotiator cannot key is not specifying what is acceptable. It is recording
// a wish, and the operator learns of that wish only when a peer fails to establish.
//
// Row 3 is the one that discriminates. `dh-group 5` sits inside the YANG `range "1..31"`
// and passes ValidDHGroup (ipsec/types.go, ValidDHGroup). But dhGroupRegistry
// (internal/component/ike/crypto/transform.go, dhGroupRegistry) holds only 14, 19 and 20.
//
// Before DHGroupImplemented was added to parseIKEProposal it committed.
// crypto.LookupDHGroup then returned the ZERO DHGroupTransform, which is Transform ID 0.
// RFC 7296 Section 3.3.2 reserves that ID, and it reached the wire as a valid-looking
// answer. That is the zero-value trap of ai/rules/fail-closed-guards.md, and the
// encryption and hash leaves were already gated against it (EncryptionImplemented,
// HashImplemented).
//
// Row 4 keeps the new gate a filter rather than a blanket refusal.
func TestIKESuitePolicyRejectsAnUnhonourableSuite(t *testing.T) {
	// 1. dh-group is mandatory: a proposal omitting it is refused.
	if _, err := makeSuitePolicyTree(t, map[string]ikeProposalOpts{
		"1": {encryption: "aes128gcm", hash: "sha256"},
	}); err == nil {
		t.Error("a proposal with no dh-group was accepted; the leaf is mandatory")
	}

	// 2. Outside the range from RFC 7296 Section 3.3.2 that the data model assigns.
	for _, group := range []string{"0", "32"} {
		if _, err := makeSuitePolicyTree(t, map[string]ikeProposalOpts{
			"1": {encryption: "aes128gcm", hash: "sha256", dhGroup: group},
		}); err == nil {
			t.Errorf("dh-group %s was accepted; it is outside the valid range 1-31", group)
		}
	}

	// 3. Inside the range, absent from the registry. This is the defect the gate closes.
	for _, group := range []string{"1", "2", "5", "15", "31"} {
		_, err := makeSuitePolicyTree(t, map[string]ikeProposalOpts{
			"1": {encryption: "aes128gcm", hash: "sha256", dhGroup: group},
		})
		if err == nil {
			t.Errorf("dh-group %s was accepted, and this build implements no transform for it; "+
				"the negotiator would carry the reserved Transform ID 0", group)
			continue
		}
		// The error must name the group and the implemented set, per
		// ai/rules/error-messages.md: what failed, why, and what to do next.
		if !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("dh-group %s refusal does not read as an implementability failure: %v",
				group, err)
		}
		if !strings.Contains(err.Error(), "14") {
			t.Errorf("dh-group %s refusal does not name the implemented set: %v", group, err)
		}
	}

	// 4. The implemented groups are still accepted, so the gate filters rather than blocks.
	for _, group := range []string{"14", "19", "20"} {
		cfg, err := makeSuitePolicyTree(t, map[string]ikeProposalOpts{
			"1": {encryption: "aes128gcm", hash: "sha256", dhGroup: group},
		})
		if err != nil {
			t.Errorf("dh-group %s was refused, and this build implements it: %v", group, err)
			continue
		}
		if len(cfg.IKEGroups["IKE-POLICY"].Proposals) != 1 {
			t.Errorf("dh-group %s parsed to no proposal", group)
		}
	}
}

// hexPSKPeer parses a peer whose shared secret is supplied under the named encoding.
// An empty encoding leaves the leaf absent, which is the deployed-operator case.
func hexPSKPeer(t *testing.T, value, encoding string) (*IPsecConfig, error) {
	t.Helper()
	return ParseIPsecConfig(makePeerTree("psk-hex", peerOpts{
		ikeGroupName: "IKE-RW",
		espGroupName: "ESP-RW",
		ikeGroup:     "IKE-RW",
		espGroup:     "ESP-RW",
		authMode:     "pre-shared-secret",
		psk:          value,
		pskEncoding:  encoding,
	}))
}

// RFC requirement: RFC7296-2.15-3 positive -- "It MUST also accept a hex encoding of the
// shared secret" (rfc/full/rfc7296.txt:2878-2879). The management interface is
// parseAuthConfig, and parsePreSharedSecret is the one producer all three §2.15 rows share.
//
// The decoded value must be a binary octet string, not text. The fixture therefore decodes
// to octets containing NUL and 0xFF. RFC7296-2.15-2 guarantees no terminator is added, and
// a hex secret is exactly where a NUL legitimately appears mid-value.
//
// The MAY at :2879-2881 ("MAY accept other encodings if the algorithm for translating the
// encoding to a binary string is specified") is not implemented and is not claimed.
func TestPSKAcceptsHexEncoding(t *testing.T) {
	// Decodes to 00 ff 41 0a 1b 2c -- a NUL, a high octet, and a printable one.
	cfg, err := hexPSKPeer(t, "00ff410a1b2c", "hex")
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	got := cfg.Peers["psk-hex"].Auth.PSK
	want := string([]byte{0x00, 0xff, 0x41, 0x0a, 0x1b, 0x2c})
	if got != want {
		t.Fatalf("hex secret decoded to %q, want %q", got, want)
	}
	if len(got) != 6 {
		t.Errorf("decoded secret is %d octets, want 6 (half the 12 input digits)", len(got))
	}

	// Upper-case and mixed-case digits decode identically.
	upper, err := hexPSKPeer(t, "00FF410A1B2C", "hex")
	if err != nil {
		t.Fatalf("ParseIPsecConfig (upper case): %v", err)
	}
	if upper.Peers["psk-hex"].Auth.PSK != want {
		t.Error("an upper-case hex secret decoded differently from its lower-case form")
	}

	// An explicit ascii encoding is the same as the leaf being absent.
	explicit, err := hexPSKPeer(t, "00ff410a1b2c", "ascii")
	if err != nil {
		t.Fatalf("ParseIPsecConfig (explicit ascii): %v", err)
	}
	if explicit.Peers["psk-hex"].Auth.PSK != "00ff410a1b2c" {
		t.Errorf("with encoding ascii the value was transformed: %q",
			explicit.Peers["psk-hex"].Auth.PSK)
	}
}

// RFC requirement: RFC7296-2.15-3 negative -- the hex encoding is EXPLICIT and is never
// guessed from the value, and a malformed hex value is refused rather than read as ASCII.
//
// Step 1 is the assertion this row exists to protect. "abcdef0123456789" is valid
// printable ASCII, is accepted by RFC7296-2.15-1, and is also a valid hex string. An
// implementation that autodetected the encoding would reinterpret that deployed
// operator's sixteen-character secret as eight binary octets. It would break
// authentication that works today, and violate RFC7296-2.15-1 at the same time. With the
// leaf absent the value must survive byte for byte.
//
// Steps 2 and 3 are ai/rules/exact-or-reject.md: a value the interface cannot honor is
// refused at parse, and no peer is left holding an ASCII-interpreted fallback.
func TestHexEncodingIsExplicitAndNeverGuessed(t *testing.T) {
	// 1. The collision case, with the leaf absent and with it set to ascii.
	for _, encoding := range []string{"", "ascii"} {
		cfg, err := hexPSKPeer(t, "abcdef0123456789", encoding)
		if err != nil {
			t.Fatalf("ParseIPsecConfig (encoding %q): %v", encoding, err)
		}
		got := cfg.Peers["psk-hex"].Auth.PSK
		if got != "abcdef0123456789" {
			t.Errorf("with encoding %q a hex-shaped ASCII secret parsed to %q; it must "+
				"survive unchanged, or every deployed secret of this shape breaks",
				encoding, got)
		}
		if len(got) != 16 {
			t.Errorf("with encoding %q the secret is %d octets, want 16", encoding, len(got))
		}
	}

	// 2. Malformed hex is refused, and the error names the peer and the leaf.
	for _, bad := range []string{"abc", "zz", "00ff4", "gg11"} {
		_, err := hexPSKPeer(t, bad, "hex")
		if err == nil {
			t.Errorf("the malformed hex secret %q was accepted", bad)
			continue
		}
		if !strings.Contains(err.Error(), "psk-hex") {
			t.Errorf("the refusal of %q does not name the peer: %v", bad, err)
		}
		if !strings.Contains(err.Error(), "pre-shared-secret-encoding") {
			t.Errorf("the refusal of %q does not name the leaf: %v", bad, err)
		}
	}

	// 3. No silent fallback: the refusal leaves no peer holding the raw text.
	cfg, err := hexPSKPeer(t, "zz", "hex")
	if err == nil {
		t.Fatal("a non-hex value was accepted under encoding hex")
	}
	if cfg != nil {
		if _, present := cfg.Peers["psk-hex"]; present {
			t.Error("the peer is present in the parsed config after its secret was refused; " +
				"a refused value must not fall back to an ASCII reading")
		}
	}

	// 4. The $9$ wrapper is unwrapped BEFORE the hex decode, so the two compose.
	//    Without this the ordering can invert, and an obfuscated hex secret would fail.
	obfuscated, err := secret.Encode("00ff41")
	if err != nil {
		t.Fatalf("secret.Encode: %v", err)
	}
	wrapped, err := hexPSKPeer(t, obfuscated, "hex")
	if err != nil {
		t.Fatalf("a $9$-wrapped hex secret was refused: %v", err)
	}
	if got := wrapped.Peers["psk-hex"].Auth.PSK; got != string([]byte{0x00, 0xff, 0x41}) {
		t.Errorf("a $9$-wrapped hex secret decoded to %q, want the three raw octets; "+
			"the unwrap must run before the hex decode", got)
	}
}
