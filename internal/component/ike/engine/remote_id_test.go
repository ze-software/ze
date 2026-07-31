package engine

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"log/slog"
	"net"
	"strings"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// ridAnchorName is the PKI store name the fixtures give the certificate authority.
const ridAnchorName = "rid-ca"

// ridAnchor loads one certificate authority into the PKI store and removes it when the
// test ends. It returns the name the peer configuration must carry.
func ridAnchor(t *testing.T) (anchorName string, sign func(t *testing.T, cn string, ips []net.IP, dns []string) ([]byte, *ecdsa.PrivateKey)) {
	t.Helper()
	name := ridAnchorName
	caCert, caDER, caKey := rctCA(t, name)
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			name: {Name: name, Certificate: caCert, Raw: caDER},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})
	return name, func(t *testing.T, cn string, ips []net.IP, dns []string) ([]byte, *ecdsa.PrivateKey) {
		t.Helper()
		return rctLeafWithSAN(t, cn, ips, dns, caCert, caKey)
	}
}

// ridCertSA builds an SA that authenticates the remote party by certificate.
func ridCertSA(t *testing.T, anchor, remoteID string) *SA {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.CACertificate = anchor
	sa.PeerCfg.Auth.RemoteID = remoteID
	return sa
}

// ridAssert records the identity the remote party asserts on the wire.
func ridAssert(sa *SA, idType uint8, data []byte) {
	sa.RemoteIDPayload = &wire.PayloadID{
		IDPayloadType: wire.PayloadTypeIDr,
		IDType:        idType,
		IDData:        data,
	}
}

// ridPSKAuth builds a valid shared-key AUTH for the identity the peer asserted, so a
// refusal comes from the identity check and never from the arithmetic.
func ridPSKAuth(t *testing.T, sa *SA) *wire.PayloadAUTH {
	t.Helper()
	return eapmSharedKeyAuth(t, sa)
}

// ridCaptureLog swaps in a logger that records what the engine writes.
func ridCaptureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := getLogger()
	setLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(previous) })
	return &buf
}

// VALIDATES: a certificate that chains to the configured anchor does not authenticate a
// peer it was never issued to. The asserted identity must equal the configured remote-id,
// and the certificate must carry that identity.
// PREVENTS: the authentication bypass a shared certificate authority opens. gen-pki.sh
// mints client.pem (CN ze-test-client) and server.pem (CN and SAN 172.28.0.3) from one
// authority. A peer configured remote-id "172.28.0.3" accepted an IKE_AUTH from the
// holder of client.pem. The chain verified, and the signature covered the identity the
// holder chose.
func TestRidRefusesACertificateIssuedToAnotherIdentity(t *testing.T) {
	anchor, sign := ridAnchor(t)
	otherDER, otherKey := sign(t, "ze-test-client", nil, nil)

	// The holder of the wrong certificate claims the configured identity.
	claimed := ridCertSA(t, anchor, "172.28.0.3")
	claimed.RemoteCertRaw = otherDER
	ridAssert(claimed, wire.IDTypeIPv4Addr, net.IPv4(172, 28, 0, 3).To4())
	err := verifyRemoteAuth(claimed, rctDigitalSigAuth(t, claimed, otherKey))
	if err == nil {
		t.Fatal("a certificate issued to another identity authenticated as the configured peer")
	}
	if !strings.Contains(err.Error(), "172.28.0.3") {
		t.Errorf("the refusal %q does not name the expected identity", err)
	}

	// The same holder asserts its own identity instead. Policy refuses it.
	honest := ridCertSA(t, anchor, "172.28.0.3")
	honest.RemoteCertRaw = otherDER
	ridAssert(honest, wire.IDTypeFQDN, []byte("ze-test-client"))
	if err := verifyRemoteAuth(honest, rctDigitalSigAuth(t, honest, otherKey)); err == nil {
		t.Error("a peer asserting an identity remote-id does not name authenticated")
	}

	// The certificate the anchor issued for that identity still authenticates.
	right := ridCertSA(t, anchor, "172.28.0.3")
	rightDER, rightKey := sign(t, "172.28.0.3", []net.IP{net.IPv4(172, 28, 0, 3)}, nil)
	right.RemoteCertRaw = rightDER
	ridAssert(right, wire.IDTypeIPv4Addr, net.IPv4(172, 28, 0, 3).To4())
	if err := verifyRemoteAuth(right, rctDigitalSigAuth(t, right, rightKey)); err != nil {
		t.Fatalf("the certificate issued for the configured identity was refused: %v", err)
	}
}

// VALIDATES: a certificate whose only identity is a subject common name still binds. The
// repository mints client.pem with no subject alternative name, and so do many
// deployments.
// PREVENTS: the entitlement check turning into a blanket refusal of every certificate
// that predates subject alternative names.
func TestRidAcceptsACommonNameWhenNoSubjectAltNameExists(t *testing.T) {
	anchor, sign := ridAnchor(t)
	der, key := sign(t, "ze-test-client", nil, nil)

	sa := ridCertSA(t, anchor, "ze-test-client")
	sa.RemoteCertRaw = der
	ridAssert(sa, wire.IDTypeFQDN, []byte("ze-test-client"))
	if err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key)); err != nil {
		t.Fatalf("a common-name-only certificate was refused: %v", err)
	}
}

// VALIDATES: the subject common name binds only when the certificate carries no subject
// alternative name extension. A certificate that carries one is bound by it alone.
// PREVENTS: the common name escaping the X.509 name constraints the authority set.
// crypto/x509 checkChainConstraints (constraints.go) applies a permitted or excluded
// subtree to DNSNames, URIs, EmailAddresses and IPAddresses, and never to the subject
// distinguished name. A corporate authority that permits only dNSName .branch.example.com
// therefore constrains the alternative name and leaves the common name free. Consulting
// the common name after a non-empty alternative name authenticates the holder as the very
// identity the authority constrained it away from.
func TestRidCommonNameNeverOverridesASubjectAltName(t *testing.T) {
	anchor, sign := ridAnchor(t)

	// The authority issued this certificate for the branch and wrote the headquarters
	// name into the subject common name.
	der, key := sign(t, "hq.example.com", nil, []string{"branch.example.com"})

	claimed := ridCertSA(t, anchor, "hq.example.com")
	claimed.RemoteCertRaw = der
	ridAssert(claimed, wire.IDTypeFQDN, []byte("hq.example.com"))
	err := verifyRemoteAuth(claimed, rctDigitalSigAuth(t, claimed, key))
	if err == nil {
		t.Fatal("a common name outranked a subject alternative name the authority constrained")
	}
	if !strings.Contains(err.Error(), "hq.example.com") {
		t.Errorf("the refusal %q does not name the asserted identity", err)
	}

	// The alternative name the authority did issue still authenticates.
	// The refusal above is therefore about the common name.
	// It is not about certificates that carry an alternative name.
	honest := ridCertSA(t, anchor, "branch.example.com")
	honest.RemoteCertRaw = der
	ridAssert(honest, wire.IDTypeFQDN, []byte("branch.example.com"))
	if err := verifyRemoteAuth(honest, rctDigitalSigAuth(t, honest, key)); err != nil {
		t.Fatalf("the issued alternative name was refused: %v", err)
	}

	// An address alternative name blocks the common name for an address identity too.
	ipDER, ipKey := sign(t, "10.0.0.9", []net.IP{net.IPv4(10, 0, 0, 1)}, nil)
	ipSA := ridCertSA(t, anchor, "10.0.0.9")
	ipSA.RemoteCertRaw = ipDER
	ridAssert(ipSA, wire.IDTypeIPv4Addr, net.IPv4(10, 0, 0, 9).To4())
	if err := verifyRemoteAuth(ipSA, rctDigitalSigAuth(t, ipSA, ipKey)); err == nil {
		t.Error("a common name outranked an address alternative name")
	}
}

// VALIDATES: an empty asserted identity never binds to a certificate, whatever the
// certificate carries.
// PREVENTS: the guard relying on its caller. asciiEqualFold("", "") is true, so an empty
// common name and an empty asserted value agreed. Only the want != "" gate in
// getRemoteCert kept that unreachable (ai/rules/fail-closed-guards.md).
func TestRidEmptyAssertedIdentityNeverBinds(t *testing.T) {
	_, sign := ridAnchor(t)
	der, _ := sign(t, "", nil, nil)
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse the leaf certificate: %v", err)
	}
	for _, idType := range []uint8{wire.IDTypeFQDN, wire.IDTypeRFC822Addr} {
		p := &wire.PayloadID{IDPayloadType: wire.PayloadTypeIDr, IDType: idType}
		if certificateCarriesIdentity(cert, p, "", 0) {
			t.Errorf("an empty %s bound to a certificate with an empty common name",
				idTypeName(idType))
		}
	}
}

// VALIDATES: an address-valued remote-id binds only against an address alternative name.
// PREVENTS: the peer choosing which certificate field the check reads. The binding read
// the ASSERTED type, so remote-id "172.28.0.3" was satisfiable as ID_IPV4_ADDR against
// cert.IPAddresses.
// It was equally satisfiable as ID_FQDN, bound against cert.DNSNames.
// One configured value reached two certificate fields, and the peer chose.
// An authority issues an address alternative name under a tighter policy than a name, so
// the peer chose the weaker field.
func TestRidAddressRemoteIDBindsOnlyToAnAddressAltName(t *testing.T) {
	anchor, sign := ridAnchor(t)

	// The authority issued a name alternative name whose text is an address literal.
	der, key := sign(t, "172.28.0.3", nil, []string{"172.28.0.3"})

	named := ridCertSA(t, anchor, "172.28.0.3")
	named.RemoteCertRaw = der
	ridAssert(named, wire.IDTypeFQDN, []byte("172.28.0.3"))
	err := verifyRemoteAuth(named, rctDigitalSigAuth(t, named, key))
	if err == nil {
		t.Fatal("an address-valued remote-id bound to a name alternative name")
	}
	if !strings.Contains(err.Error(), "172.28.0.3") {
		t.Errorf("the refusal %q does not name the identity", err)
	}

	// The address type still authenticates against an address alternative name.
	addrDER, addrKey := sign(t, "172.28.0.3", []net.IP{net.IPv4(172, 28, 0, 3)}, nil)
	addressed := ridCertSA(t, anchor, "172.28.0.3")
	addressed.RemoteCertRaw = addrDER
	ridAssert(addressed, wire.IDTypeIPv4Addr, net.IPv4(172, 28, 0, 3).To4())
	if err := verifyRemoteAuth(addressed, rctDigitalSigAuth(t, addressed, addrKey)); err != nil {
		t.Fatalf("an address identity against an address-valued remote-id was refused: %v", err)
	}
}

// VALIDATES: an address-valued remote-id refuses a text identity that spells the same
// address. The refusal names the type the peer asserted.
// PREVENTS: the policy half disagreeing with the binding half. certificateCarriesIdentity
// picks the certificate field from the CONFIGURED class, so remote-id "10.0.0.1" reaches
// cert.IPAddresses alone. remoteIDMatches read the ASSERTED type instead, so ID_FQDN
// carrying the text 10.0.0.1 satisfied it. For X.509 and EAP the certificate half then
// denied the peer. For pre-shared-secret no certificate exists, so nothing denied it
// (ai/rules/fail-closed-guards.md).
func TestRidAddressRemoteIDRefusesATextIdentity(t *testing.T) {
	// The text types a peer can spell an address literal into. Each one is refused.
	for _, tc := range []struct {
		name   string
		idType uint8
	}{
		{"fqdn", wire.IDTypeFQDN},
		{"rfc822", wire.IDTypeRFC822Addr},
		{"keyid", wire.IDTypeKeyID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
			sa.PeerCfg.Auth.PSK = "shared-secret"
			sa.PeerCfg.Auth.RemoteID = "10.0.0.1"
			ridAssert(sa, tc.idType, []byte("10.0.0.1"))

			err := verifyRemoteAuth(sa, ridPSKAuth(t, sa))
			if err == nil {
				t.Fatalf("%s carrying an address literal satisfied an address remote-id",
					idTypeName(tc.idType))
			}
			// The two sides render alike, so the refusal has to name the type.
			if !strings.Contains(err.Error(), idTypeName(tc.idType)) {
				t.Errorf("the refusal %q does not name the asserted type", err)
			}
		})
	}

	// The address types the operator did name still authenticate.
	ok := testSAWithKeys(t)
	ok.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	ok.PeerCfg.Auth.PSK = "shared-secret"
	ok.PeerCfg.Auth.RemoteID = "10.0.0.1"
	ridAssert(ok, wire.IDTypeIPv4Addr, net.IPv4(10, 0, 0, 1).To4())
	if err := verifyRemoteAuth(ok, ridPSKAuth(t, ok)); err != nil {
		t.Fatalf("the configured address identity was refused: %v", err)
	}
}

// VALIDATES: a text-valued remote-id refuses an address identity. The class gate runs in
// both directions.
// PREVENTS: the gate reading as one-sided. An address type against a name was already
// refused, because netip.ParseAddr rejects the configured name. That refusal came from a
// parse failure rather than from policy, so a later edit to the parse would lose it.
func TestRidTextRemoteIDRefusesAnAddressIdentity(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "shared-secret"
	sa.PeerCfg.Auth.RemoteID = "vpn.example.com"
	ridAssert(sa, wire.IDTypeIPv4Addr, net.IPv4(10, 0, 0, 1).To4())

	if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err == nil {
		t.Fatal("an address identity satisfied a name-valued remote-id")
	}
}

// VALIDATES: a name that differs from remote-id only by a trailing dot is refused, and the
// refusal names the trailing dot.
// PREVENTS: an operator reading "asserted vpn.example.com., expects vpn.example.com" and
// seeing no difference. RFC 7296 Section 3.5 puts the A-label on the wire, so the strict
// comparison is correct and only the message needed the hint
// (ai/rules/error-messages.md, leg 3).
func TestRidTrailingDotRefusalNamesTheTrailingDot(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "shared-secret"
	sa.PeerCfg.Auth.RemoteID = "vpn.example.com"
	ridAssert(sa, wire.IDTypeFQDN, []byte("vpn.example.com."))

	err := verifyRemoteAuth(sa, ridPSKAuth(t, sa))
	if err == nil {
		t.Fatal("a trailing-dot name matched a remote-id without one")
	}
	if !strings.Contains(err.Error(), "trailing dot") {
		t.Errorf("the refusal %q does not name the trailing dot", err)
	}
}

// VALIDATES: an identity type ze cannot compare denies the peer and names the type, and a
// malformed distinguished name denies rather than rendering to something comparable.
// PREVENTS: the guard falling through. A check that cannot run is a check that denies,
// never one that passes (ai/rules/fail-closed-guards.md).
//
// ID_DER_ASN1_DN moved OUT of this test when RFC7296-4-4 made it comparable, and the two
// cases below are what remains genuinely incomparable. ID_DER_ASN1_GN is still assigned by
// RFC 7296 Section 3.5 and still has no comparison. And a DN whose octets are not valid
// DER cannot be rendered at all. Both must deny by the not-comparable branch rather than
// by a value mismatch, so the refusal names the TYPE.
func TestRidDeniesAnIdentityTypeItCannotCompare(t *testing.T) {
	for _, tc := range []struct {
		name   string
		idType uint8
		data   []byte
		names  string
	}{
		{"general name", wire.IDTypeDERASN1GN, []byte{0x30, 0x00}, "ID_DER_ASN1_GN"},
		{"malformed distinguished name", wire.IDTypeDERASN1DN, []byte{0xff, 0xff, 0xff}, "ID_DER_ASN1_DN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anchor, sign := ridAnchor(t)
			der, key := sign(t, "ze-test-client", nil, nil)

			sa := ridCertSA(t, anchor, "CN=ze-test-client")
			sa.RemoteCertRaw = der
			ridAssert(sa, tc.idType, tc.data)
			err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key))
			if err == nil {
				t.Fatal("an identity type ze cannot compare authenticated")
			}
			if !strings.Contains(err.Error(), tc.names) {
				t.Errorf("the refusal %q does not name the identity type %s", err, tc.names)
			}
			if !strings.Contains(err.Error(), "cannot compare") {
				t.Errorf("the refusal %q is a value mismatch, not the not-comparable branch, "+
					"so the type check did not run", err)
			}
		})
	}
}

// VALIDATES: every identity type ze compares accepts the configured value and refuses a
// different one. The check runs on shared-key authentication too, where no certificate
// exists to bind.
// PREVENTS: a comparison that answers true for one type and silently passes the rest.
func TestRidComparesEveryIdentityTypeItSupports(t *testing.T) {
	cases := []struct {
		name     string
		idType   uint8
		remoteID string
		match    []byte
		differ   []byte
	}{
		{"ipv4", wire.IDTypeIPv4Addr, "172.28.0.3", net.IPv4(172, 28, 0, 3).To4(), net.IPv4(172, 28, 0, 4).To4()},
		{"ipv6", wire.IDTypeIPv6Addr, "2001:db8::3", net.ParseIP("2001:db8::3").To16(), net.ParseIP("2001:db8::4").To16()},
		{"fqdn", wire.IDTypeFQDN, "vpn.example.com", []byte("VPN.example.com"), []byte("other.example.com")},
		{"rfc822", wire.IDTypeRFC822Addr, "user@example.com", []byte("User@Example.com"), []byte("root@example.com")},
		{"keyid", wire.IDTypeKeyID, "branch-42", []byte("branch-42"), []byte("branch-43")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok := testSAWithKeys(t)
			ok.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
			ok.PeerCfg.Auth.PSK = "shared-secret"
			ok.PeerCfg.Auth.RemoteID = tc.remoteID
			ridAssert(ok, tc.idType, tc.match)
			if err := verifyRemoteAuth(ok, ridPSKAuth(t, ok)); err != nil {
				t.Fatalf("the configured identity was refused: %v", err)
			}

			bad := testSAWithKeys(t)
			bad.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
			bad.PeerCfg.Auth.PSK = "shared-secret"
			bad.PeerCfg.Auth.RemoteID = tc.remoteID
			ridAssert(bad, tc.idType, tc.differ)
			if err := verifyRemoteAuth(bad, ridPSKAuth(t, bad)); err == nil {
				t.Error("an identity remote-id does not name authenticated")
			}
		})
	}
}

// VALIDATES: a peer that sends no identification payload is denied when remote-id names
// one.
// PREVENTS: a nil payload reading as a match. RFC 7296 Section 1.2 puts IDi in every
// IKE_AUTH request, so an absent one is a peer ze cannot check.
func TestRidDeniesAPeerThatSendsNoIdentity(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "shared-secret"
	sa.PeerCfg.Auth.RemoteID = "172.28.0.3"
	sa.RemoteIDPayload = nil

	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the signed octets: %v", err)
	}
	derived, err := ikecrypto.PRF(sa.Proposal.PRF.ID, []byte("shared-secret"), []byte("Key Pad for IKEv2"))
	if err != nil {
		t.Fatalf("derive the key: %v", err)
	}
	data, err := ikecrypto.PRF(sa.Proposal.PRF.ID, derived, octets)
	if err != nil {
		t.Fatalf("compute the AUTH data: %v", err)
	}
	auth := &wire.PayloadAUTH{AuthMethod: wire.AuthMethodPSK, AuthData: data}

	if err := verifyRemoteAuth(sa, auth); err == nil {
		t.Fatal("a peer that asserted no identity authenticated against a configured remote-id")
	}
}

// VALIDATES: an unset remote-id leaves the identity unchecked, and the engine states that
// in the log rather than passing in silence.
// PREVENTS: the gap becoming invisible. A guard that cannot deny must say something
// (ai/rules/fail-closed-guards.md). Every certificate the authority issued authenticates
// as this peer while remote-id is empty.
func TestRidUnsetRemoteIDWarnsThatAnyIssuedCertificatePasses(t *testing.T) {
	anchor, sign := ridAnchor(t)
	der, key := sign(t, "ze-test-client", nil, nil)

	buf := ridCaptureLog(t)
	sa := ridCertSA(t, anchor, "")
	sa.RemoteCertRaw = der
	ridAssert(sa, wire.IDTypeFQDN, []byte("ze-test-client"))
	if err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key)); err != nil {
		t.Fatalf("an unset remote-id refused a trusted certificate: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "remote-id") {
		t.Errorf("the log is %q, want a warning that names remote-id", got)
	}
	if !strings.Contains(got, sa.PeerName) {
		t.Errorf("the log is %q, want the peer name", got)
	}
}
