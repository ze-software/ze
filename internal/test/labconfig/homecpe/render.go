// Design: docs/architecture/testing/interop.md -- the home-CPE lab.
// Package homecpe renders the converted CPE for the lab and config validation.
package homecpe

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The converted CPE's inputs, under the checkout: the model's output with
// placeholders, the VyOS source it came from, the record of the conversion,
// and the lab values for the placeholders.
const (
	Directory     = "test/interop-topology/cpe"
	ConvertedFile = "ze-cpe.conf"
	LabValuesFile = "lab-values"
)

// The two placeholder kinds of the conversion: a secret the owner fills on the
// router, and a value the lab issues at run time.
const (
	placeholderOpen   = "<<"
	placeholderClose  = ">>"
	placeholderSecret = "SECRET:"
	placeholderLab    = "LAB:"
)

// The names the issuer reads from lab-values and the names it issues. Each
// issued name is a placeholder of ze-cpe.conf; wgLaptopPrivate is not, it is
// the wg-client's own key.
const (
	wgCPEPrivate      = "wg-wg0-private-key"
	wgLaptopPSK       = "wg-wg0-laptop-psk"
	wgLaptopPrivate   = "wg-laptop-private-key"
	wgLaptopPublic    = "wg-wg0-laptop-public-key"
	pkiCACertificate  = "pki-LAB-CA-certificate"
	pkiCPECertificate = "pki-CPE-certificate"
	pkiCPEKey         = "pki-CPE-key"
)

// The identities of the lab PKI: the responder vpn.example.net and the CPE's
// local-id, both in ze-cpe.conf and the responder's swanctl.conf.
const (
	responderName = "vpn.example.net"
	CPEIdentity   = "cpe.lab.example.net"
)

// The files the issuer writes for the peers, relative to the prepared
// directory, named by the node declarations that mount them.
const (
	LaptopKeyFile    = "wireguard/laptop.key"
	CPEPublicFile    = "wireguard/cpe.pub"
	LaptopPSKFile    = "wireguard/psk"
	CAFile           = "swanctl/x509ca/lab-ca.pem"
	ResponderFile    = "swanctl/x509/vpn.pem"
	ResponderKeyFile = "swanctl/private/vpn-key.pem"
)

// labValues maps a placeholder name to its value.
type labValues map[string]string

// ParseValues reads lab-values: one "name value" pair per line, with blank
// lines and lines opening with "#" skipped. A line with no value and a name
// given twice are refused, so a typo never leaves a placeholder unfilled in
// silence.
func ParseValues(text string) (labValues, error) {
	values := labValues{}
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, " ")
		value = strings.TrimSpace(value)
		if !found {
			return nil, fmt.Errorf("lab-values line %d names %q with no value; write \"<name> <value>\"", number+1, name)
		}
		if value == "" {
			return nil, fmt.Errorf("lab-values line %d names %q with no value; write \"<name> <value>\"", number+1, name)
		}
		if _, seen := values[name]; seen {
			return nil, fmt.Errorf("lab-values line %d names %q a second time; keep one value per name", number+1, name)
		}
		values[name] = value
	}
	return values, nil
}

// substitutePlaceholders replaces every <<SECRET:name>> and <<LAB:name>> of
// template with its value and changes nothing else. A placeholder of another
// kind, one with no value, and an unterminated one are refused: a file that
// still holds a placeholder is not the configuration the lab means to run.
func substitutePlaceholders(template string, values labValues) (string, error) {
	var out strings.Builder
	rest := template
	for {
		before, after, opened := strings.Cut(rest, placeholderOpen)
		out.WriteString(before)
		if !opened {
			return out.String(), nil
		}
		body, tail, closed := strings.Cut(after, placeholderClose)
		if !closed {
			return "", fmt.Errorf("the configuration holds an unterminated placeholder at %q; close it with %q", clip(placeholderOpen+after), placeholderClose)
		}
		name, err := placeholderName(body)
		if err != nil {
			return "", err
		}
		value, found := values[name]
		if !found {
			return "", fmt.Errorf("placeholder %q has no lab value; add \"%s <value>\" to %s/%s or issue it", body, name, Directory, LabValuesFile)
		}
		out.WriteString(value)
		rest = tail
	}
}

// placeholderName answers the name of one placeholder's body, and refuses a
// kind that is neither SECRET nor LAB.
func placeholderName(body string) (string, error) {
	if name, found := strings.CutPrefix(body, placeholderSecret); found {
		return name, nil
	}
	if name, found := strings.CutPrefix(body, placeholderLab); found {
		return name, nil
	}
	return "", fmt.Errorf("placeholder %q is neither %s nor %s; the conversion writes only those two kinds", body, placeholderSecret, placeholderLab)
}

// clip shortens a fragment quoted in an error to one line of context.
func clip(fragment string) string {
	const contextOctets = 40
	if len(fragment) > contextOctets {
		return fragment[:contextOctets]
	}
	return fragment
}

// issuedLab is what one run issues: the placeholder values only the run can
// know, and the files the peers mount, by path relative to the prepared
// directory.
type issuedLab struct {
	values labValues
	files  map[string][]byte
}

// KeyAlgorithm is the algorithm of the key the lab issues the CPE, which
// is the key Ze signs its IKE AUTH with. The zero value is no choice, and
// issuing refuses it.
type KeyAlgorithm uint8

const (
	KeyUnspecified KeyAlgorithm = iota
	// KeyP256 is an ECDSA P-256 key: Ze signs with ECDSA under RFC 7427.
	KeyP256
	// KeyEd25519 is an Ed25519 key: Ze signs with pure Ed25519 under
	// RFC 8420 (mgmt-tunnel-ed25519, spec-home-cpe-11 AC-9).
	KeyEd25519
)

// errCPEKeyUnspecified says a scenario declared no CPE key algorithm.
var errCPEKeyUnspecified = errors.New("the scenario names no CPE key algorithm; declare KeyP256 or KeyEd25519")

// issueLab derives the WireGuard public keys from the lab private keys and
// issues a fresh PKI: a P-256 CA with path length 0, the responder's P-256
// certificate for vpn.example.net, and the CPE's certificate for
// cpe.lab.example.net with a key of algorithm, the one AC-9 variant the
// scenario runs.
func issueLab(values labValues, now time.Time, algorithm KeyAlgorithm) (issuedLab, error) {
	issued := issuedLab{values: labValues{}, files: map[string][]byte{}}
	if err := issueWireGuard(values, &issued); err != nil {
		return issuedLab{}, err
	}
	if err := issuePKI(now, algorithm, &issued); err != nil {
		return issuedLab{}, err
	}
	return issued, nil
}

// issueWireGuard derives each X25519 public key from its lab private key
// (RFC 7748 Section 6.1), so no public key is written by hand beside the
// private key it must match.
func issueWireGuard(values labValues, issued *issuedLab) error {
	laptopPublic, err := wireGuardPublic(values, wgLaptopPrivate)
	if err != nil {
		return err
	}
	cpePublic, err := wireGuardPublic(values, wgCPEPrivate)
	if err != nil {
		return err
	}
	psk, found := values[wgLaptopPSK]
	if !found {
		return fmt.Errorf("lab-values names no %s; the wg-client needs the preshared key the CPE holds", wgLaptopPSK)
	}
	issued.values[wgLaptopPublic] = laptopPublic
	issued.files[LaptopKeyFile] = []byte(values[wgLaptopPrivate] + "\n")
	issued.files[CPEPublicFile] = []byte(cpePublic + "\n")
	issued.files[LaptopPSKFile] = []byte(psk + "\n")
	return nil
}

// wireGuardPublic answers the base64 public key of the lab private key name.
func wireGuardPublic(values labValues, name string) (string, error) {
	encoded, found := values[name]
	if !found {
		return "", fmt.Errorf("lab-values names no %s; add a base64 X25519 private key for it", name)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("lab value %s is not base64: %w", name, err)
	}
	private, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", fmt.Errorf("lab value %s is not a 32-octet X25519 private key: %w", name, err)
	}
	return base64.StdEncoding.EncodeToString(private.PublicKey().Bytes()), nil
}

// issuePKI issues the CA, the responder's certificate and the CPE's, the
// CPE's with a key of algorithm. Ze's pki leaves take the base64 DER without
// PEM armour (a certificate) and a key in SEC 1 or PKCS #8 DER; strongSwan
// reads PEM.
func issuePKI(now time.Time, algorithm KeyAlgorithm, issued *issuedLab) error {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate the lab CA key: %w", err)
	}
	caTemplate, err := certificateTemplate(now, "ze-home-lab-ca")
	if err != nil {
		return err
	}
	caTemplate.IsCA = true
	caTemplate.BasicConstraintsValid = true
	caTemplate.MaxPathLen = 0
	caTemplate.MaxPathLenZero = true
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("issue the lab CA: %w", err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return fmt.Errorf("read back the lab CA: %w", err)
	}
	responderDER, responderKey, err := issueLeaf(now, ca, caKey, responderName, KeyP256)
	if err != nil {
		return err
	}
	cpeDER, cpeKey, err := issueLeaf(now, ca, caKey, CPEIdentity, algorithm)
	if err != nil {
		return err
	}
	issued.values[pkiCACertificate] = base64.StdEncoding.EncodeToString(caDER)
	issued.values[pkiCPECertificate] = base64.StdEncoding.EncodeToString(cpeDER)
	issued.values[pkiCPEKey] = base64.StdEncoding.EncodeToString(cpeKey)
	issued.files[CAFile] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issued.files[ResponderFile] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: responderDER})
	issued.files[ResponderKeyFile] = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: responderKey})
	return nil
}

// issueLeaf issues one end-entity certificate for name with a key of
// algorithm, signed by ca, and answers its DER and its key's DER: SEC 1 for a
// P-256 key, which the responder's "EC PRIVATE KEY" PEM block carries, and
// PKCS #8 for an Ed25519 key, which SEC 1 cannot hold.
func issueLeaf(now time.Time, ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string, algorithm KeyAlgorithm) ([]byte, []byte, error) {
	public, keyDER, err := leafKey(algorithm)
	if err != nil {
		return nil, nil, fmt.Errorf("the key of %s: %w", name, err)
	}
	template, err := certificateTemplate(now, name)
	if err != nil {
		return nil, nil, err
	}
	template.DNSNames = []string{name}
	template.KeyUsage = x509.KeyUsageDigitalSignature
	der, err := x509.CreateCertificate(rand.Reader, template, ca, public, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("issue the certificate of %s: %w", name, err)
	}
	return der, keyDER, nil
}

// leafKey generates a key of algorithm and answers its public half and the
// DER of its private half.
func leafKey(algorithm KeyAlgorithm) (any, []byte, error) {
	switch algorithm {
	case KeyUnspecified:
		return nil, nil, errCPEKeyUnspecified
	case KeyP256:
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("generate a P-256 key: %w", err)
		}
		der, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, nil, fmt.Errorf("encode a P-256 key: %w", err)
		}
		return &key.PublicKey, der, nil
	case KeyEd25519:
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("generate an Ed25519 key: %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			return nil, nil, fmt.Errorf("encode an Ed25519 key: %w", err)
		}
		return public, der, nil
	}
	return nil, nil, fmt.Errorf("unknown CPE key algorithm %d", algorithm)
}

// certificateTemplate is a certificate valid for one day around now, with a
// random 128-bit serial number.
func certificateTemplate(now time.Time, commonName string) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("draw a serial number for %s: %w", commonName, err)
	}
	return &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: commonName},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}, nil
}

// Rendered is the converted configuration with every placeholder filled,
// and the files the run issued for the peers.
type Rendered struct {
	Config string
	Files  map[string][]byte
}

// Render reads the converted configuration and the lab values under
// root, issues the run's values with a CPE key of algorithm, and substitutes
// both. A name both listed and issued is refused, so lab-values can never
// shadow what the run derived.
func Render(root string, now time.Time, algorithm KeyAlgorithm) (Rendered, error) {
	directory := filepath.Join(root, filepath.FromSlash(Directory))
	template, err := os.ReadFile(filepath.Join(directory, ConvertedFile)) //nolint:gosec // the checkout root joined with a fixed name
	if err != nil {
		return Rendered{}, fmt.Errorf("read the converted CPE configuration: %w", err)
	}
	listed, err := os.ReadFile(filepath.Join(directory, LabValuesFile)) //nolint:gosec // the checkout root joined with a fixed name
	if err != nil {
		return Rendered{}, fmt.Errorf("read the lab values: %w", err)
	}
	values, err := ParseValues(string(listed))
	if err != nil {
		return Rendered{}, err
	}
	issued, err := issueLab(values, now, algorithm)
	if err != nil {
		return Rendered{}, err
	}
	for name, value := range issued.values {
		if _, both := values[name]; both {
			return Rendered{}, fmt.Errorf("lab-values lists %s, which the run issues; remove it from %s/%s", name, Directory, LabValuesFile)
		}
		values[name] = value
	}
	config, err := substitutePlaceholders(string(template), values)
	if err != nil {
		return Rendered{}, err
	}
	return Rendered{Config: config, Files: issued.files}, nil
}
