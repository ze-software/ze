// Design: docs/architecture/pki/pki-store.md -- the local CA that issues leaves
// Related: ca_test.go -- the serial, backdating and validity tests of the same CA
//
// VALIDATES: the RFC 5280 profile of the two certificates Ze's local CA writes. The
// root generateRoot mints is a marked CA, and the leaf Root.IssueLeafFor signs is a
// marked end entity whose identities sit in the subject alternative name.
// PREVENTS: a root whose key usage or basic constraints a verifier refuses, a leaf that
// claims to sign certificates, and a leaf that shares the CA's own distinguished name.

package pki

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"
)

// oidBasicConstraints is id-ce-basicConstraints, RFC 5280 Section 4.2.1.9.
var oidBasicConstraints = asn1.ObjectIdentifier{2, 5, 29, 19}

// rfc5280Root mints a root and parses one leaf it signs for a DNS name and an address.
func rfc5280Root(t *testing.T) (root *Root, leaf *x509.Certificate) {
	t.Helper()
	root, _, _, err := generateRoot(time.Hour)
	if err != nil {
		t.Fatalf("generateRoot: %v", err)
	}
	return root, rfc5280Leaf(t, root, "ze-test")
}

// rfc5280Leaf issues and parses a leaf for one DNS name and one address.
func rfc5280Leaf(t *testing.T, root *Root, commonName string) *x509.Certificate {
	t.Helper()
	issued, err := root.IssueLeafFor(commonName, []string{"node.example.net", "192.0.2.1"}, time.Hour)
	if err != nil {
		t.Fatalf("IssueLeafFor(%s): %v", commonName, err)
	}
	leaf, err := x509.ParseCertificate(issued.Certificate[0])
	if err != nil {
		t.Fatalf("parse the leaf: %v", err)
	}
	return leaf
}

// basicConstraintsExtension answers the certificate's basicConstraints extension, and
// fails the test when it has none.
func basicConstraintsExtension(t *testing.T, cert *x509.Certificate) []byte {
	t.Helper()
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidBasicConstraints) {
			continue
		}
		if !ext.Critical {
			t.Errorf("%s: the basicConstraints extension is not marked critical", cert.Subject)
		}
		return ext.Value
	}
	t.Fatalf("%s carries no basicConstraints extension", cert.Subject)
	return nil
}

// TestRFC5280RootIsAMarkedCertificationAuthority checks the root's subject, key usage
// and basic constraints.
//
// Method: generateRoot mints a root, the root signs one leaf, and both are read back
// from DER. Each requirement is asserted on the field the verifier reads.
func TestRFC5280RootIsAMarkedCertificationAuthority(t *testing.T) {
	root, leaf := rfc5280Root(t)
	ca := root.cert

	// RFC requirement: RFC5280-4.1.2.6-1 positive -- the root is a CA (cA TRUE), its
	// subject is a non-empty DN, and the issuer of the leaf it signs is byte-equal to it.
	if !ca.IsCA {
		t.Fatal("the root does not assert cA")
	}
	if len(ca.Subject.Names) == 0 {
		t.Error("the root's subject is an empty DN")
	}
	if !bytes.Equal(leaf.RawIssuer, ca.RawSubject) {
		t.Error("the leaf's issuer field does not match the root's subject")
	}

	// RFC requirement: RFC5280-4.1.2.6-2 positive -- the root asserts cRLSign, and its
	// subject is the non-empty DN "Ze Local CA".
	if ca.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Error("the root does not assert cRLSign")
	}
	if ca.Subject.CommonName != rootSubjectCN {
		t.Errorf("the root's subject is %q, want CN=%s", ca.Subject, rootSubjectCN)
	}

	// RFC requirement: RFC5280-4.2.1.3-1 positive -- the root, whose key verifies the
	// signature on every leaf, carries the keyUsage extension with keyCertSign set.
	if err := leaf.CheckSignatureFrom(ca); err != nil {
		t.Fatalf("the root's key does not verify the leaf it signed: %v", err)
	}
	if ca.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("the root does not assert keyCertSign")
	}

	// RFC requirement: RFC5280-4.2.1.3-2 positive -- the root asserts keyCertSign, and
	// its basic constraints extension asserts cA.
	if ca.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Fatal("the root asserts no keyCertSign")
	}
	if !ca.BasicConstraintsValid || !ca.IsCA {
		t.Error("the root asserts keyCertSign without asserting cA")
	}

	// RFC requirement: RFC5280-4.2.1.3-4 positive -- the keyUsage extension of the root
	// and of the leaf each has at least one bit set.
	if ca.KeyUsage == 0 {
		t.Error("the root's keyUsage has no bit set")
	}
	if leaf.KeyUsage == 0 {
		t.Error("the leaf's keyUsage has no bit set")
	}

	// RFC requirement: RFC5280-4.2.1.9-3 positive -- the root's pathLenConstraint is
	// present and is zero, which is greater than or equal to zero.
	if ca.MaxPathLen != 0 || !ca.MaxPathLenZero {
		t.Errorf("the root's pathLenConstraint is %d (zero flag %v), want a present 0", ca.MaxPathLen, ca.MaxPathLenZero)
	}

	// RFC requirement: RFC5280-4.2.1.9-4 positive -- the root's basicConstraints
	// extension is present and marked critical.
	basicConstraintsExtension(t, ca)
}

// TestRFC5280LeafIsAMarkedEndEntity checks that the leaf asserts neither cA nor
// keyCertSign, and that its basicConstraints carry no pathLenConstraint.
//
// Method: one root and one leaf, as above. Each MUST NOT is asserted on the leaf, and
// the contrasting root proves the same assertion sees the field when it is set.
func TestRFC5280LeafIsAMarkedEndEntity(t *testing.T) {
	root, leaf := rfc5280Root(t)

	// RFC requirement: RFC5280-4.2.1.9-1 positive -- the leaf does not assert cA, and its
	// keyUsage does not assert keyCertSign.
	if leaf.IsCA {
		t.Fatal("the leaf asserts cA")
	}
	if leaf.KeyUsage&x509.KeyUsageCertSign != 0 {
		t.Error("a leaf that is not a CA asserts keyCertSign")
	}
	// RFC requirement: RFC5280-4.2.1.9-1 negative -- the root, which asserts cA, does
	// assert keyCertSign, so the check above reads a bit that can be set.
	if root.cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("the root asserts no keyCertSign, so the leaf check above reads nothing")
	}

	// RFC requirement: RFC5280-4.2.1.9-5 positive -- the leaf carries a basicConstraints
	// extension whose value is the empty SEQUENCE: no cA, no pathLenConstraint.
	if value := basicConstraintsExtension(t, leaf); !bytes.Equal(value, []byte{0x30, 0x00}) {
		t.Errorf("the leaf's basicConstraints is % x, want the empty SEQUENCE 30 00", value)
	}
	if leaf.MaxPathLen != -1 {
		t.Errorf("the leaf carries pathLenConstraint %d", leaf.MaxPathLen)
	}
	// RFC requirement: RFC5280-4.2.1.9-5 negative -- the root, which asserts cA and
	// keyCertSign, does carry pathLenConstraint 0 (30 06 01 01 ff 02 01 00), so the leaf
	// check above can see the field when it is present.
	want := []byte{0x30, 0x06, 0x01, 0x01, 0xff, 0x02, 0x01, 0x00}
	if value := basicConstraintsExtension(t, root.cert); !bytes.Equal(value, want) {
		t.Errorf("the root's basicConstraints is % x, want % x", value, want)
	}
}

// TestRFC5280LeafIdentitiesSitInTheSubjectAltName checks where IssueLeafFor binds the
// hosts it is given, and that the leaf's DN is the one its caller named.
//
// Method: leaves for two component names, each for one DNS name and one address, are
// read back. A call with no host and a call naming the CA itself are refused.
func TestRFC5280LeafIdentitiesSitInTheSubjectAltName(t *testing.T) {
	root, leaf := rfc5280Root(t)
	address := net.ParseIP("192.0.2.1")

	// RFC requirement: RFC5280-4.2.1.6-1 positive -- both hosts are bound into the subject
	// alternative name extension, and neither is written into the subject DN.
	if len(leaf.DNSNames) != 1 || len(leaf.IPAddresses) != 1 {
		t.Fatalf("the leaf binds DNS %v and IP %v, want one of each", leaf.DNSNames, leaf.IPAddresses)
	}
	if leaf.Subject.CommonName != "ze-test" || len(leaf.Subject.Names) != 1 {
		t.Errorf("the leaf's subject is %q, want CN=ze-test alone", leaf.Subject)
	}

	// RFC requirement: RFC5280-4.2.1.6-2 positive -- what Ze's CA verifies before it
	// binds a name: each host given is classified by parsing, an address becomes an
	// iPAddress entry and every other host a dNSName, each appears once and nothing is
	// added. A call with no host is refused rather than issued.
	if leaf.DNSNames[0] != "node.example.net" || !leaf.IPAddresses[0].Equal(address) {
		t.Errorf("the leaf binds DNS %v and IP %v, want [node.example.net] and [192.0.2.1]", leaf.DNSNames, leaf.IPAddresses)
	}
	if len(leaf.EmailAddresses) != 0 || len(leaf.URIs) != 0 {
		t.Errorf("the leaf binds names nobody asked for: email %v, URI %v", leaf.EmailAddresses, leaf.URIs)
	}

	// RFC requirement: RFC5280-4.2.1.6-7 positive -- the domain name is stored as a
	// dNSName.
	if leaf.DNSNames[0] != "node.example.net" {
		t.Errorf("the domain name is not a dNSName: %v", leaf.DNSNames)
	}
	// RFC requirement: RFC5280-4.2.1.6-7 negative -- the address, which is no domain
	// name, is never stored as a dNSName.
	for _, name := range leaf.DNSNames {
		if net.ParseIP(name) != nil {
			t.Errorf("the address %s is stored as a dNSName", name)
		}
	}

	// RFC requirement: RFC5280-4.2.1.6-10 positive -- the subjectAltName extension the
	// leaf carries holds at least one entry.
	if len(leaf.DNSNames)+len(leaf.IPAddresses) == 0 {
		t.Error("the leaf's subjectAltName holds no entry")
	}
	// RFC requirement: RFC5280-4.2.1.6-10 negative -- a leaf with no host would carry an
	// empty sequence, so the CA refuses to issue it.
	if _, err := root.IssueLeafFor("ze-test", nil, time.Hour); !errors.Is(err, errIssueLeafNoHosts) {
		t.Errorf("IssueLeafFor with no host returned %v, want errIssueLeafNoHosts", err)
	}

	// RFC requirement: RFC5280-4.1.2.6-5 positive -- two components get two DNs, each
	// exactly the name its caller gave, and neither is the root's DN.
	other := rfc5280Leaf(t, root, "ze-other")
	if bytes.Equal(leaf.RawSubject, other.RawSubject) {
		t.Error("two components were issued the same DN")
	}
	for _, cert := range []*x509.Certificate{leaf, other} {
		if bytes.Equal(cert.RawSubject, root.cert.RawSubject) {
			t.Errorf("the leaf %q shares the root's DN", cert.Subject)
		}
	}
	// RFC requirement: RFC5280-4.1.2.6-5 negative -- a leaf asking for the root's own
	// name, exactly or differing only in letter case or in leading, trailing or inner
	// space, would share the CA's DN under RFC 4518 comparison, so it is refused.
	for _, name := range []string{rootSubjectCN, "ze local ca", "ZE LOCAL CA", "Ze  Local CA", " Ze Local CA", "Ze Local CA\t"} {
		if _, err := root.IssueLeafFor(name, []string{"node.example.net"}, time.Hour); !errors.Is(err, ErrIssueLeafNamesTheRoot) {
			t.Errorf("IssueLeafFor(%q) returned %v, want ErrIssueLeafNamesTheRoot", name, err)
		}
	}
}

// TestIssueLeafDrawsAUniqueSerial issues 64 leaves from one root and reads back every
// serial and the root's own.
//
// RFC requirement: RFC5280-4.1.2.2-1 positive -- every leaf serial and the root serial is
// greater than zero.
// RFC requirement: RFC5280-4.1.2.2-2 positive -- no two of the 64 leaves one root issues
// share a serial.
// RFC requirement: RFC5280-4.1.2.2-3 positive -- no serial the CA writes is negative.
// RFC requirement: RFC5280-4.1.2.2-5 positive -- every leaf serial and the root serial,
// read from the encoded certificate, is a DER INTEGER whose content is at most 20 octets.
func TestIssueLeafDrawsAUniqueSerial(t *testing.T) {
	store, _ := newRootStore(t)

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	const issuances = 64
	// The bound is a literal, not serialBits: a test that reads the producer's own
	// width stays green when that width grows past what RFC 5280 permits.
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	seen := make(map[string]bool, issuances)
	wideSerials := 0

	for i := range issuances {
		cert, err := root.IssueLeaf("ze-test", []string{"127.0.0.1"})
		if err != nil {
			t.Fatalf("IssueLeaf %d: %v", i, err)
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			t.Fatalf("parse leaf %d: %v", i, err)
		}

		serial := leaf.SerialNumber
		if serial.Sign() <= 0 {
			t.Fatalf("serial %d is not positive: %s", i, serial)
		}
		if serial.Cmp(limit) >= 0 {
			t.Fatalf("serial %d needs %d bits, more than 128", i, serial.BitLen())
		}
		if octets := serialContentOctets(t, leaf.RawTBSCertificate); octets > 20 {
			t.Fatalf("serial %d is a DER INTEGER of %d content octets, more than 20", i, octets)
		}
		if seen[serial.String()] {
			t.Fatalf("serial %s was issued twice", serial)
		}
		seen[serial.String()] = true

		if serial.BitLen() > serialBits-8 {
			wideSerials++
		}
	}

	// A serial drawn from the full 128-bit range lands in the top 1/256 of it
	// about 255 times in 256. Sixty-four draws all landing below that point has
	// probability (1/256)^64, so this assertion fails only if the draw is not
	// 128 bits wide.
	if wideSerials == 0 {
		t.Fatalf("no serial out of %d needed more than %d bits: the draw is narrower than %d bits",
			issuances, serialBits-8, serialBits)
	}
	// The root draws from the same range, so it is not a counter either.
	rootSerial := root.cert.SerialNumber
	if rootSerial.Sign() <= 0 || rootSerial.Cmp(limit) >= 0 {
		t.Fatalf("root serial %s is outside the 128-bit range", rootSerial)
	}
	if octets := serialContentOctets(t, root.cert.RawTBSCertificate); octets > 20 {
		t.Fatalf("root serial is a DER INTEGER of %d content octets, more than 20", octets)
	}
}

// serialContentOctets answers how many content octets the serialNumber INTEGER of
// one encoded TBSCertificate holds. It reads the DER, not the parsed big.Int, so the
// count is the one a relying party meets on the wire.
func serialContentOctets(t *testing.T, tbs []byte) int {
	t.Helper()
	// TBSCertificate ::= SEQUENCE { version [0] EXPLICIT Version DEFAULT v1,
	// serialNumber CertificateSerialNumber, ... }. encoding/asn1 accepts the
	// trailing fields this struct does not name.
	var head struct {
		Version int `asn1:"optional,explicit,default:0,tag:0"`
		Serial  asn1.RawValue
	}
	if _, err := asn1.Unmarshal(tbs, &head); err != nil {
		t.Fatalf("parse TBSCertificate: %v", err)
	}
	if head.Serial.Class != asn1.ClassUniversal || head.Serial.Tag != asn1.TagInteger {
		t.Fatalf("serialNumber has class %d tag %d, want a universal INTEGER",
			head.Serial.Class, head.Serial.Tag)
	}
	return len(head.Serial.Bytes)
}
