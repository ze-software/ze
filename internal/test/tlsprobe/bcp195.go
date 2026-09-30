// Design: docs/architecture/dns/secure-transports.md -- BCP 195 cipher-suite probe
// Related: tlsprobe.go -- the ClientHello the assertions send
// RFC: rfc/short/rfc7858.md -- RFC7858-8-1; rfc/short/rfc9728.md -- RFC9728-7.1-2 (BCP 195, RFC 9325)

package tlsprobe

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"
)

// KeyKind names the key of the certificate a listener serves. A cipher suite
// authenticates with one key kind, so the suites a probe can meaningfully offer
// depend on it: an RSA suite refused by an ECDSA server proves nothing.
type KeyKind uint8

// The certificate key kinds a probe knows.
const (
	KeyUnspecified KeyKind = iota
	KeyECDSA
	KeyRSA
)

type suite struct {
	id   uint16
	name string
	// rule names the RFC 9325 section 4.1 statement the suite breaks.
	rule string
}

// RFC 9325 Section 4.1 statements the forbidden tables below encode:
// "Implementations MUST NOT negotiate the cipher suites with NULL encryption."
// "Implementations MUST NOT negotiate RC4 cipher suites."
// "Implementations MUST NOT negotiate cipher suites offering less than 112 bits
// of security, including so-called "export-level" encryption"
// "Implementations SHOULD NOT negotiate cipher suites that use algorithms
// offering less than 128 bits of security."
// "Implementations SHOULD NOT negotiate cipher suites based on RSA key
// transport, a.k.a. "static RSA"."
// "Implementations SHOULD NOT negotiate non-ephemeral Elliptic Curve DH key
// agreement."
// "TLS 1.2 implementations SHOULD NOT negotiate cipher suites based on
// ephemeral finite-field Diffie-Hellman key agreement".
const (
	ruleNull       = "NULL encryption (MUST NOT)"
	ruleRC4        = "RC4 (MUST NOT)"
	ruleBelow112   = "below 112 bits, export or single DES (MUST NOT)"
	ruleBelow128   = "112-bit 3DES (SHOULD NOT)"
	ruleStaticRSA  = "static RSA (SHOULD NOT)"
	ruleStaticECDH = "static ECDH (SHOULD NOT)"
	ruleDHE        = "TLS_DHE_* in TLS 1.2 (SHOULD NOT)"
)

var (
	forbiddenECDSA = []suite{
		{0xc006, "TLS_ECDHE_ECDSA_WITH_NULL_SHA", ruleNull},
		{0xc007, "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA", ruleRC4},
		{0xc008, "TLS_ECDHE_ECDSA_WITH_3DES_EDE_CBC_SHA", ruleBelow128},
		{0xc001, "TLS_ECDH_ECDSA_WITH_NULL_SHA", ruleNull},
		{0xc004, "TLS_ECDH_ECDSA_WITH_AES_128_CBC_SHA", ruleStaticECDH},
	}
	forbiddenRSA = []suite{
		{0x0001, "TLS_RSA_WITH_NULL_MD5", ruleNull},
		{0x0002, "TLS_RSA_WITH_NULL_SHA", ruleNull},
		{0x0003, "TLS_RSA_EXPORT_WITH_RC4_40_MD5", ruleBelow112},
		{0x0008, "TLS_RSA_EXPORT_WITH_DES40_CBC_SHA", ruleBelow112},
		{0x0009, "TLS_RSA_WITH_DES_CBC_SHA", ruleBelow112},
		{0x0005, "TLS_RSA_WITH_RC4_128_SHA", ruleRC4},
		{0xc011, "TLS_ECDHE_RSA_WITH_RC4_128_SHA", ruleRC4},
		{0xc012, "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", ruleBelow128},
		{0x000a, "TLS_RSA_WITH_3DES_EDE_CBC_SHA", ruleBelow128},
		{0x002f, "TLS_RSA_WITH_AES_128_CBC_SHA", ruleStaticRSA},
		{0x009c, "TLS_RSA_WITH_AES_128_GCM_SHA256", ruleStaticRSA},
		{0x0033, "TLS_DHE_RSA_WITH_AES_128_CBC_SHA", ruleDHE},
	}
)

// acceptedSuite is an RFC 9325 section 4.2 recommended suite for the key kind:
// ECDHE key agreement, AES-128-GCM.
func acceptedSuite(key KeyKind) (suite, bool) {
	switch key {
	case KeyECDSA:
		return suite{0xc02b, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", "recommended"}, true
	case KeyRSA:
		return suite{0xc02f, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "recommended"}, true
	case KeyUnspecified:
	}
	return suite{}, false
}

func forbiddenFor(key KeyKind) []suite {
	if key == KeyRSA {
		return forbiddenRSA
	}
	return forbiddenECDSA
}

// compressionNull and compressionDeflate are the RFC 3749 method numbers.
const (
	compressionNull    uint8 = 0
	compressionDeflate uint8 = 1
)

// AssertBCP195 probes the TLS listener at address, which serves a certificate
// of kind key, and fails t on each departure from the RFC 9325 cipher-suite,
// compression and renegotiation guidance:
//   - the recommended ECDHE AES-GCM suite is selected, with null compression
//     and a renegotiation_info extension in the ServerHello (section 3.5);
//   - every suite of the forbidden table for the key kind, offered alone, is
//     never selected: the server answers with an alert or closes;
//   - offered after every forbidden suite, the recommended one is selected;
//   - DEFLATE is never selected: offered before null the server picks null,
//     offered alone the server refuses (section 3.3).
func AssertBCP195(t testing.TB, address string, key KeyKind) {
	t.Helper()
	good, ok := acceptedSuite(key)
	if !ok {
		t.Fatalf("tlsprobe: no recommended suite for key kind %d", key)
	}
	forbidden := forbiddenFor(key)

	answer := offer(t, address, []uint16{good.id}, []uint8{compressionNull})
	if answer.Outcome != OutcomeServerHello || answer.Suite != good.id {
		t.Fatalf("%s offered alone: %+v, want a ServerHello selecting it", good.name, answer)
	}
	if !answer.SecureRenegotiation {
		t.Errorf("%s: ServerHello carries no renegotiation_info (RFC 9325 section 3.5)", good.name)
	}

	for _, bad := range forbidden {
		answer := offer(t, address, []uint16{bad.id}, []uint8{compressionNull})
		if answer.Outcome == OutcomeServerHello {
			t.Errorf("%s (%#04x, %s) offered alone was selected: %+v", bad.name, bad.id, bad.rule, answer)
		}
	}

	all := make([]uint16, 0, len(forbidden)+1)
	for _, bad := range forbidden {
		all = append(all, bad.id)
	}
	all = append(all, good.id)
	answer = offer(t, address, all, []uint8{compressionNull})
	if answer.Outcome != OutcomeServerHello || answer.Suite != good.id {
		t.Errorf("forbidden suites offered before %s: %+v, want %s selected", good.name, answer, good.name)
	}

	answer = offer(t, address, []uint16{good.id}, []uint8{compressionDeflate, compressionNull})
	if answer.Outcome != OutcomeServerHello || answer.Compression != compressionNull {
		t.Errorf("DEFLATE offered before null: %+v, want null compression selected", answer)
	}
	answer = offer(t, address, []uint16{good.id}, []uint8{compressionDeflate})
	if answer.Outcome == OutcomeServerHello {
		t.Errorf("DEFLATE offered alone was accepted: %+v", answer)
	}
}

func offer(t testing.TB, address string, suites []uint16, compression []uint8) Answer {
	t.Helper()
	answer, err := Offer(t.Context(), address, suites, compression)
	if err != nil {
		t.Fatalf("probe %s: %v", address, err)
	}
	if answer.Outcome == OutcomeUnspecified {
		t.Fatalf("probe %s: no outcome", address)
	}
	return answer
}

// SelfSignedRSA returns a PEM certificate and PKCS#1 key for a 2048-bit RSA
// server valid for one hour for the IP host and the name "localhost".
func SelfSignedRSA(host string) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("tlsprobe: rsa key: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, nil, fmt.Errorf("tlsprobe: host %q is not an IP address", host)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{ip},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("tlsprobe: certificate: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, nil
}
