// Design: docs/architecture/pki/pki-store.md -- PKI config parser
// RFC: rfc/short/rfc7296.md -- the certificate payloads a peer entry supplies (Section 3.6)
// RFC: rfc/short/rfc9190.md -- Section 5.4, the crl leaf-list this parser reads

package pki

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ocsp"

	"github.com/ze-software/ze/internal/component/config"
)

const maxNameLen = 255

var (
	errPKICertEmpty      = errors.New("pki: certificate leaf is empty")
	errPKICertDecode     = errors.New("pki: certificate leaf is neither a PEM certificate nor base64 DER")
	errPKICertNotPEM     = errors.New("pki: certificate leaf opens a PEM block that does not decode")
	errPKICertPEMBlock   = errors.New("pki: certificate leaf holds a PEM block that is not a CERTIFICATE")
	errPKICertPEMExtra   = errors.New("pki: certificate leaf holds more than one PEM block, and this leaf names one certificate")
	errPKIKeyEmpty       = errors.New("pki: private key leaf is empty")
	errPKIKeyDecode      = errors.New("pki: private key leaf is neither a PEM key nor base64 DER")
	errPKIKeyNotPEM      = errors.New("pki: private key leaf opens a PEM block that does not decode")
	errPKIKeyPEMExtra    = errors.New("pki: private key leaf holds more than one PEM block, and this leaf names one key")
	errPKIKeyMismatch    = errors.New("pki: private key does not match certificate public key")
	errPKIKeyUnsupported = errors.New("pki: unsupported private key type")
	errPKICRLDecode      = errors.New("pki: crl leaf is neither a PEM X509 CRL nor base64 DER")
	errPKICRLNotPEM      = errors.New("pki: crl leaf opens a PEM block that does not decode")
	errPKICRLPEMBlock    = errors.New("pki: crl leaf holds a PEM block that is not an X509 CRL")
	errPKICRLPEMExtra    = errors.New("pki: crl leaf holds more than one PEM block, and this leaf names one revocation list")
	errPKICRLIssuer      = errors.New("pki: crl is not signed by this ca, so it cannot answer for the certificates this ca issued")
	errPKIOCSPDecode     = errors.New("pki: ocsp-response leaf is neither a PEM OCSP RESPONSE nor base64 DER")
	errPKIOCSPNotPEM     = errors.New("pki: ocsp-response leaf opens a PEM block that does not decode")
	errPKIOCSPPEMBlock   = errors.New("pki: ocsp-response leaf holds a PEM block that is not an OCSP RESPONSE")
	errPKIOCSPPEMExtra   = errors.New("pki: ocsp-response leaf holds more than one PEM block, and this leaf names one response")
	errPKIOCSPParse      = errors.New("pki: ocsp-response leaf does not decode as an OCSP response")
	errPKIOCSPSerial     = errors.New("pki: ocsp-response answers about another certificate, so it says nothing about this one")
	errPKINameInvalid    = errors.New("pki: name contains invalid characters (allowed: alphanumeric, dash, underscore, dot)")
	errPKINameTooLong    = errors.New("pki: name exceeds 255 characters")
)

func validateName(name string) error {
	if name == "" {
		return errPKINameInvalid
	}
	if len(name) > maxNameLen {
		return errPKINameTooLong
	}
	for _, c := range name {
		if !isNameChar(c) {
			return errPKINameInvalid
		}
	}
	return nil
}

func isNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.'
}

// ParseConfig extracts PKI certificates and keys from the config tree.
// Returns a zero-value PKIConfig if no pki {} block is present.
func ParseConfig(tree *config.Tree) (*PKIConfig, error) {
	if tree == nil {
		return &PKIConfig{
			CACerts:      make(map[string]*CACertEntry),
			Certificates: make(map[string]*CertificateEntry),
		}, nil
	}

	pkiRoot := tree.GetContainer("pki")
	if pkiRoot == nil {
		return &PKIConfig{
			CACerts:      make(map[string]*CACertEntry),
			Certificates: make(map[string]*CertificateEntry),
		}, nil
	}

	cfg := &PKIConfig{
		CACerts:      make(map[string]*CACertEntry),
		Certificates: make(map[string]*CertificateEntry),
	}

	caList := pkiRoot.GetList("ca")
	for name, caTree := range caList {
		if err := validateName(name); err != nil {
			return nil, fmt.Errorf("pki ca %q: %w", name, err)
		}
		entry, err := parseCACert(name, caTree)
		if err != nil {
			return nil, fmt.Errorf("pki ca %q: %w", name, err)
		}
		cfg.CACerts[name] = entry
	}

	certList := pkiRoot.GetList("certificate")
	for name, certTree := range certList {
		if err := validateName(name); err != nil {
			return nil, fmt.Errorf("pki certificate %q: %w", name, err)
		}
		entry, err := parseDeviceCert(name, certTree)
		if err != nil {
			return nil, fmt.Errorf("pki certificate %q: %w", name, err)
		}
		cfg.Certificates[name] = entry
	}

	return cfg, nil
}

// pemPreamble opens every PEM document. RFC 7468 Section 2 gives the encoding
// as "-----BEGIN " followed by the label, so the marker is present in a PEM
// value and absent from base64 DER, which carries no dash at all.
//
// Its presence decides which decoder runs, and there is no second attempt. A
// value that opens a PEM block is held to PEM rules, so a truncated paste is
// reported as a broken PEM rather than as a base64 error about a payload the
// operator never wrote.
const pemPreamble = "-----BEGIN"

// certificateDER decodes what a pki `certificate` or `intermediate` leaf holds.
//
// Two forms are accepted, because the operator's clipboard carries whichever
// one the tool they copied from printed. PEM is what `ze show pki local-ca pem`
// answers, what a `.crt` file holds, and what every other X.509 tool emits.
// Bare base64 DER is what this leaf took before and what existing configs hold,
// so it keeps working unchanged.
func certificateDER(value string) ([]byte, error) {
	if !strings.Contains(value, pemPreamble) {
		der, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errPKICertDecode, err)
		}
		return der, nil
	}

	block, rest := pem.Decode([]byte(value))
	if block == nil {
		return nil, errPKICertNotPEM
	}
	if block.Type != pemBlockCertificate {
		return nil, fmt.Errorf("%w: %s", errPKICertPEMBlock, block.Type)
	}
	// A second block is refused rather than ignored. An appliance cert.pem holds
	// the leaf followed by its root, and pasting the whole file into a `ca`
	// leaf would otherwise store the LEAF as the trust anchor and fail later, at
	// a handshake, with an error naming neither the paste nor this leaf.
	if extra, _ := pem.Decode(rest); extra != nil {
		return nil, fmt.Errorf("%w: the second is %s", errPKICertPEMExtra, extra.Type)
	}
	return block.Bytes, nil
}

// privateKeyDER decodes what a pki `private key` leaf holds, in the same two
// forms certificateDER takes: an operator holding a certificate in PEM holds
// the key beside it in PEM too.
//
// The block label is not checked here. The three encodings parsePrivateKey
// reads write three different labels (PRIVATE KEY for PKCS8, EC PRIVATE KEY for
// SEC1, RSA PRIVATE KEY for PKCS1), and the parser is what decides which one
// the bytes are.
func privateKeyDER(value string) ([]byte, error) {
	if !strings.Contains(value, pemPreamble) {
		der, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errPKIKeyDecode, err)
		}
		return der, nil
	}

	block, rest := pem.Decode([]byte(value))
	if block == nil {
		return nil, errPKIKeyNotPEM
	}
	if extra, _ := pem.Decode(rest); extra != nil {
		return nil, fmt.Errorf("%w: the second is %s", errPKIKeyPEMExtra, extra.Type)
	}
	return block.Bytes, nil
}

func parseCACert(name string, tree *config.Tree) (*CACertEntry, error) {
	certValue, ok := tree.Get("certificate")
	if !ok || certValue == "" {
		return nil, errPKICertEmpty
	}

	der, err := certificateDER(certValue)
	if err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: x509 parse: %w", err)
	}

	entry := &CACertEntry{
		Name:        name,
		Certificate: cert,
		Raw:         der,
	}

	// crl is a leaf-list, so an operator can hold several lists for one CA: a
	// segmented CRL, or a fresh one published beside the outgoing one during a
	// rollover. GetSlice reads a single value and a bracket list alike.
	//
	// Each list is parsed HERE so a bad paste is refused at config load, where
	// the message names the CA. Left to the handshake it would surface as a
	// refused peer whose certificate is in fact valid.
	for _, crlValue := range tree.GetSlice("crl") {
		if crlValue == "" {
			continue
		}
		crlDER, cErr := revocationListDER(crlValue)
		if cErr != nil {
			return nil, cErr
		}
		list, cErr := x509.ParseRevocationList(crlDER)
		if cErr != nil {
			return nil, fmt.Errorf("pki: crl x509 parse: %w", cErr)
		}
		// The list must be one this CA signed. A list from another CA cannot
		// answer for the certificates this one issued, and storing it here would
		// let a paste into the wrong ca block read as a working revocation
		// source while answering nothing.
		if sErr := list.CheckSignatureFrom(cert); sErr != nil {
			return nil, fmt.Errorf("%w: %w", errPKICRLIssuer, sErr)
		}
		// The DER is what is stored, and the parsed list is discarded here. The
		// consumer hands the list on as PEM, so re-encoding the parsed structure
		// would not reproduce the bytes this CA signed (CACertEntry, types.go).
		entry.RawCRLs = append(entry.RawCRLs, crlDER)
	}

	return entry, nil
}

// ocspResponseDER decodes what a pki `ocsp-response` leaf holds, in the same two
// forms revocationListDER takes.
//
// An OCSP response has no PEM label of its own in RFC 7468, and `openssl ocsp
// -respout` writes raw DER, so base64 is the form an operator arrives with. The
// PEM arm is here because the two leaves beside it take PEM and an operator who
// wraps one wraps them all; `pemBlockOCSPResponse` is the label OpenSSL prints.
func ocspResponseDER(value string) ([]byte, error) {
	if !strings.Contains(value, pemPreamble) {
		der, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errPKIOCSPDecode, err)
		}
		return der, nil
	}

	block, rest := pem.Decode([]byte(value))
	if block == nil {
		return nil, errPKIOCSPNotPEM
	}
	if block.Type != pemBlockOCSPResponse {
		return nil, fmt.Errorf("%w: %s", errPKIOCSPPEMBlock, block.Type)
	}
	// A second block is refused rather than ignored, for the reason
	// certificateDER gives: a whole file pasted into one leaf would store its
	// first document and drop the rest with no message.
	if extra, _ := pem.Decode(rest); extra != nil {
		return nil, fmt.Errorf("%w: the second is %s", errPKIOCSPPEMExtra, extra.Type)
	}
	return block.Bytes, nil
}

// revocationListDER decodes what a pki `crl` leaf-list entry holds, in the same
// two forms certificateDER takes: an operator who holds certificates in PEM
// holds the revocation list beside them in PEM too.
func revocationListDER(value string) ([]byte, error) {
	if !strings.Contains(value, pemPreamble) {
		der, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errPKICRLDecode, err)
		}
		return der, nil
	}

	block, rest := pem.Decode([]byte(value))
	if block == nil {
		return nil, errPKICRLNotPEM
	}
	if block.Type != pemBlockCRL {
		return nil, fmt.Errorf("%w: %s", errPKICRLPEMBlock, block.Type)
	}
	// A second block is refused rather than ignored, for the reason
	// certificateDER gives: a whole file pasted into one leaf would store its
	// first document and drop the rest with no message. Each list gets its own
	// entry in the leaf-list.
	if extra, _ := pem.Decode(rest); extra != nil {
		return nil, fmt.Errorf("%w: the second is %s", errPKICRLPEMExtra, extra.Type)
	}
	return block.Bytes, nil
}

func parseDeviceCert(name string, tree *config.Tree) (*CertificateEntry, error) {
	certValue, ok := tree.Get("certificate")
	if !ok || certValue == "" {
		return nil, errPKICertEmpty
	}

	der, err := certificateDER(certValue)
	if err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: x509 parse: %w", err)
	}

	entry := &CertificateEntry{
		Name:        name,
		Certificate: cert,
		Raw:         der,
	}

	// intermediate is a leaf-list, so an operator can name every certificate on the path
	// from this one toward the trust anchor. RFC 7296 Section 3.6 requires ze be capable
	// of being configured to send up to four X.509 certificates, and the leaf plus three
	// intermediates is that maximum. GetSlice reads a single value and a bracket list
	// alike, so a config naming one intermediate parses exactly as it did before.
	for _, interValue := range tree.GetSlice("intermediate") {
		if interValue == "" {
			continue
		}
		interDER, iErr := certificateDER(interValue)
		if iErr != nil {
			return nil, fmt.Errorf("pki: intermediate: %w", iErr)
		}
		interCert, iErr := x509.ParseCertificate(interDER)
		if iErr != nil {
			return nil, fmt.Errorf("pki: intermediate x509 parse: %w", iErr)
		}
		entry.Intermediates = append(entry.Intermediates, interCert)
		entry.RawIntermediates = append(entry.RawIntermediates, interDER)
	}

	// RFC 9190 Section 5.4: "EAP-TLS servers supporting TLS 1.3 MUST implement
	// Certificate Status Requests (OCSP stapling)". The response is read HERE so
	// a paste that answers about another certificate is refused at config load,
	// where the message names the entry. Left to the handshake it would surface
	// as a peer refusing a certificate that is in fact valid, and only on a peer
	// strict enough to check.
	if ocspValue, oOk := tree.Get("ocsp-response"); oOk && ocspValue != "" {
		staple, oErr := ocspResponseDER(ocspValue)
		if oErr != nil {
			return nil, oErr
		}
		// The issuer is not resolved here, so the signature is not checked here:
		// ParseConfig reads the certificate list before the ca that issued this
		// entry is known to it, and the RELYING PARTY is what RFC 6960 Section
		// 3.2 makes responsible for judging a response anyway. Ze does that as a
		// peer (eap.CheckCertificateStatus). What this entry owes is that the
		// operator pasted a response about the certificate beside it.
		resp, oErr := ocsp.ParseResponse(staple, nil)
		if oErr != nil {
			return nil, fmt.Errorf("%w: %w", errPKIOCSPParse, oErr)
		}
		if resp.SerialNumber == nil || resp.SerialNumber.Cmp(cert.SerialNumber) != 0 {
			return nil, fmt.Errorf("%w: it answers about serial %s and this certificate is serial %s",
				errPKIOCSPSerial, resp.SerialNumber, cert.SerialNumber)
		}
		entry.RawOCSPResponse = staple
	}

	privContainer := tree.GetContainer("private")
	if privContainer != nil {
		keyValue, kOk := privContainer.Get("key")
		if kOk && keyValue != "" {
			privKey, kErr := parsePrivateKey(keyValue)
			if kErr != nil {
				return nil, kErr
			}
			if err := verifyKeyMatchesCert(privKey, cert); err != nil {
				return nil, err
			}
			entry.PrivateKey = privKey
		}
	}

	return entry, nil
}

func parsePrivateKey(value string) (any, error) {
	if value == "" {
		return nil, errPKIKeyEmpty
	}

	der, err := privateKeyDER(value)
	if err != nil {
		return nil, err
	}

	if key, pErr := x509.ParsePKCS8PrivateKey(der); pErr == nil {
		return key, nil
	}

	if key, pErr := x509.ParseECPrivateKey(der); pErr == nil {
		return key, nil
	}

	if key, pErr := x509.ParsePKCS1PrivateKey(der); pErr == nil {
		return key, nil
	}

	return nil, errPKIKeyUnsupported
}

func verifyKeyMatchesCert(privKey any, cert *x509.Certificate) error {
	switch k := privKey.(type) {
	case *ecdsa.PrivateKey:
		pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok || !k.PublicKey.Equal(pub) {
			return errPKIKeyMismatch
		}
	case *rsa.PrivateKey:
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok || !k.PublicKey.Equal(pub) {
			return errPKIKeyMismatch
		}
	case ed25519.PrivateKey:
		pub, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok || !pub.Equal(ed25519.PublicKey(k[32:])) {
			return errPKIKeyMismatch
		}
	default:
		return errPKIKeyUnsupported
	}
	return nil
}
