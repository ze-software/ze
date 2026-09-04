// Design: docs/architecture/pki/pki-store.md -- the local certificate authority
// Related: store.go -- the config-driven half of this package, which holds CA
// certificates with no key and therefore cannot issue
// Related: tls.go -- ServerTLSMaterial, the named-certificate path that outranks
// issuance whenever an operator configured a certificate

package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/zefs"
)

// Issuance parameters. Each is stated here because a certificate a peer refuses
// is indistinguishable from a peer that is down, and the reader needs the number
// to judge a refusal.
const (
	// rootValidity is how long the root lives. It is long because rotating it
	// means redistributing it to every peer by hand, and short enough that a
	// compromise is not unbounded.
	rootValidity = 10 * 365 * 24 * time.Hour

	// leafValidity is how long an issued leaf lives. A component reissues on
	// every start, so a day is ample, and it bounds the exposure of a leaf that
	// must be distrusted: Ze has no revocation.
	leafValidity = 24 * time.Hour

	// clockSkewMargin backdates NotBefore on every certificate this package
	// issues. A router that has not yet reached its NTP server can be minutes
	// ahead of the daemon that issued the certificate, and such a peer refuses a
	// freshly issued leaf as not yet valid. Five minutes covers the skew a
	// booting router shows before its first NTP correction.
	clockSkewMargin = 5 * time.Minute

	// serialBits is the width of the random serial drawn for every certificate.
	// Uniqueness per issuer is the whole requirement, and 128 random bits meets
	// it with no persistent ledger. selfcert.GenerateWebCertWithNames draws the
	// same width for the same reason.
	serialBits = 128
)

// rootSubjectCN names the issuer in every chain Ze validates. It is not a
// hostname and is never matched against one.
const rootSubjectCN = "Ze Local CA"

var (
	errRootStoreMissing  = errors.New("pki: certificate authority needs a store")
	errRootCertNotPEM    = errors.New("pki: stored root certificate is not PEM")
	errRootKeyNotPEM     = errors.New("pki: stored root private key is not PEM")
	errRootNotCA         = errors.New("pki: stored root certificate is not a CA")
	errRootKeyMismatched = errors.New("pki: stored root key does not match the stored root certificate")

	errIssueLeafNoCommonName = errors.New("pki: leaf certificate needs a common name")
	errIssueLeafNoHosts      = errors.New("pki: a leaf with no subject alternative name identifies nothing")

	errRootNoValidity  = errors.New("pki: root certificate needs a validity longer than zero")
	errIssueNoValidity = errors.New("pki: leaf certificate needs a validity longer than zero")
)

var caLog = slogutil.LazyLogger("pki.ca")

// rootGenerationMu serializes root generation inside this process, so two
// goroutines racing to start a listener end with one root rather than two.
//
// It is deliberately IN-PROCESS only. zefs takes no file lock (pkg/zefs, which
// TestBlobStoreNoFlock asserts), so a second daemon sharing one blob already
// replaces arbitrary state rather than only the root, and a lock here would
// suggest a protection it does not give.
var rootGenerationMu sync.Mutex

// RootStore is the persistence the certificate authority needs: three
// operations over the two registered zefs keys. storage.Storage satisfies it, so
// the daemon passes its own store handle with no adapter.
//
// This package takes the narrow interface rather than storage.Storage itself
// because a certificate authority has no business reaching a config path, a
// write lock, or a version history.
type RootStore interface {
	// ReadFile returns the value stored under name, or an error when there is
	// none.
	ReadFile(name string) ([]byte, error)

	// WriteFile stores data under name. The filesystem backend applies perm;
	// the ZeFS backend accepts and ignores it, and the mode that protects the
	// key there is the blob file's own 0600.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// Exists reports whether name holds a value.
	Exists(name string) bool
}

// Root is the certificate authority Ze issues its own components' certificates
// from: the hub acceptor's leaf, and every other internal listener that has no
// operator-named certificate.
//
// LoadOrGenerateRoot is the only constructor. Every field is written before the
// value is returned and is read-only after, so a Root is safe for concurrent
// use.
type Root struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// Certificate returns the root certificate. It is public material: a peer needs
// it to validate a leaf this root issued, and it carries no private key.
func (r *Root) Certificate() *x509.Certificate {
	return r.cert
}

// CertificatePEM returns the root certificate in PEM, which is the form every
// consumer of a trust anchor takes: the environment slot an external plugin
// process reads, the text the export command prints for an operator to paste
// into a client's pki ca block, and the second block of the appliance device
// certificate file.
//
// The private key has no such accessor, and MUST NOT gain one. It leaves this
// package only into a signing operation (`ai/rules/principles.md`, and the
// spec's own security review: the root key reaches a signer and nothing else).
func (r *Root) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: r.cert.Raw})
}

// LoadOrGenerateRoot returns the daemon's certificate authority at the default
// root lifetime, reading the root from the store when one is present and
// generating and persisting it once when none is. A daemon that restarts
// presents the same root, so every copy an operator already distributed keeps
// working.
//
// This is the constructor every component takes. A caller that must choose the
// lifetime itself calls LoadOrGenerateRootFor, and the appliance build host is
// the one such caller: its root has to outlive the leaf it signs, whose life an
// operator sets.
func LoadOrGenerateRoot(store RootStore) (*Root, error) {
	return LoadOrGenerateRootFor(store, rootValidity)
}

// LoadOrGenerateRootFor is LoadOrGenerateRoot with the root's lifetime named at
// the call site. The validity applies to a root this call GENERATES; a root the
// store already holds is returned with the lifetime it was created with, so
// changing the value never shortens or extends a root already distributed.
//
// It reads before it writes, and generation is serialized in-process, so
// concurrent callers agree on one root.
func LoadOrGenerateRootFor(store RootStore, validity time.Duration) (*Root, error) {
	if store == nil {
		return nil, errRootStoreMissing
	}
	if validity <= 0 {
		return nil, errRootNoValidity
	}

	rootGenerationMu.Lock()
	defer rootGenerationMu.Unlock()

	certKey := zefs.KeyCACert.Pattern
	keyKey := zefs.KeyCAKey.Pattern

	// Both halves must be present. One half alone is a store that was written
	// part-way, and reading it would produce a CA that cannot sign.
	if store.Exists(certKey) && store.Exists(keyKey) {
		root, err := loadRoot(store, certKey, keyKey)
		if err != nil {
			return nil, err
		}
		currentRoot.Store(root)
		return root, nil
	}

	root, certPEM, keyPEM, err := generateRoot(validity)
	if err != nil {
		return nil, err
	}

	if err := store.WriteFile(certKey, certPEM, 0o600); err != nil {
		return nil, fmt.Errorf("pki: store root certificate: %w", err)
	}
	if err := store.WriteFile(keyKey, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("pki: store root private key: %w", err)
	}

	caLog().Info("generated local CA root",
		"subject", root.cert.Subject.CommonName,
		"serial", root.cert.SerialNumber.String(),
		"expires", root.cert.NotAfter.Format(time.RFC3339))
	currentRoot.Store(root)
	return root, nil
}

// currentRoot is the authority this process loaded, published by
// LoadOrGenerateRoot for a surface that reads the root and holds no store
// handle. The export command is that surface: it answers inside the daemon, and
// a command context carries no storage.
//
// It is the same shape store.go uses for the config-driven half of this package,
// and it is nil until a root is loaded rather than an empty Root nobody can tell
// from a real one.
var currentRoot atomic.Pointer[Root]

// loadedRoot returns the certificate authority this process loaded, or nil when
// LoadOrGenerateRoot has not run here. A caller MUST branch on nil: there is no
// certificate to answer with before a root exists.
//
// It is unexported because the only surface that needs it is the export command
// in this package. A caller outside pki holds the store handle and calls
// LoadOrGenerateRoot, which is the constructor and never a second lookup.
//
// Safe for concurrent use.
func loadedRoot() *Root {
	return currentRoot.Load()
}

// loadRoot reads and validates the persisted pair. A stored root that does not
// parse, is not a CA, or does not match its key is an error: a CA that cannot
// sign, or one that signs with the wrong key, produces certificates no peer
// accepts, and the operator needs to be told which.
func loadRoot(store RootStore, certKey, keyKey string) (*Root, error) {
	certPEM, err := store.ReadFile(certKey)
	if err != nil {
		return nil, fmt.Errorf("pki: read root certificate: %w", err)
	}
	keyPEM, err := store.ReadFile(keyKey)
	if err != nil {
		return nil, fmt.Errorf("pki: read root private key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, errRootCertNotPEM
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse root certificate: %w", err)
	}
	if !cert.IsCA {
		return nil, errRootNotCA
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, errRootKeyNotPEM
	}
	parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse root private key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("pki: root private key is %T, want an ECDSA key", parsed)
	}

	public, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("pki: root certificate carries a %T public key, want an ECDSA key", cert.PublicKey)
	}
	if !public.Equal(&key.PublicKey) {
		return nil, errRootKeyMismatched
	}

	return &Root{cert: cert, key: key}, nil
}

// generateRoot mints a new self-signed ECDSA P-256 root that lives for validity
// and returns it with the PEM material to persist. The key is marshaled PKCS#8,
// which is what loadRoot parses back and what every other Ze surface writes.
func generateRoot(validity time.Duration) (root *Root, certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pki: generate root key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: rootSubjectCN},
		NotBefore:             now.Add(-clockSkewMargin),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		// The root signs leaves directly, so no other CA may sit under it.
		MaxPathLen:     0,
		MaxPathLenZero: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pki: create root certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pki: parse new root certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pki: marshal root private key: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlockPrivateKey, Bytes: keyDER})
	return &Root{cert: cert, key: key}, certPEM, keyPEM, nil
}

// IssueLeaf signs a server certificate for one of Ze's own components, valid
// for the default leaf lifetime. The commonName names the component in the
// subject and is never matched against a hostname; hosts are the SANs a peer
// verifies, where a value that parses as an IP address becomes an IP SAN and
// every other value a DNS SAN.
//
// This is the method a component takes. Such a component reissues at every
// start, so it MUST NOT each pick a lifetime of its own; a caller that does not
// reissue at every start calls IssueLeafFor and names one.
//
// Safe for concurrent use.
func (r *Root) IssueLeaf(commonName string, hosts []string) (tls.Certificate, error) {
	return r.IssueLeafFor(commonName, hosts, leafValidity)
}

// IssueLeafFor is IssueLeaf with the leaf's lifetime named at the call site. A
// leaf that is minted once and then copied into an image lives as long as the
// operator asked for, and the caller MUST keep it inside the root's own
// validity: a chain stops verifying the moment its issuer expires.
//
// The returned certificate carries the leaf alone. A peer that validates it
// holds the root as its trust anchor already, so sending the root back adds
// bytes and no information.
//
// Safe for concurrent use.
func (r *Root) IssueLeafFor(commonName string, hosts []string, validity time.Duration) (tls.Certificate, error) {
	// A leaf with no SAN identifies nothing, so every peer refuses it at the
	// handshake, one layer and one process away from the caller that asked for
	// it. Refuse here, where the caller can be named.
	if commonName == "" {
		return tls.Certificate{}, errIssueLeafNoCommonName
	}
	if len(hosts) == 0 {
		return tls.Certificate{}, fmt.Errorf("pki: leaf for %s needs at least one host: %w", commonName, errIssueLeafNoHosts)
	}
	// A leaf that is already expired when it is issued is refused by the same
	// peers, and reads to the operator as a daemon that minted a certificate
	// wrong rather than as a caller that asked for nothing.
	if validity <= 0 {
		return tls.Certificate{}, fmt.Errorf("pki: leaf for %s: %w", commonName, errIssueNoValidity)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("pki: generate leaf key for %s: %w", commonName, err)
	}

	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-clockSkewMargin),
		NotAfter:     now.Add(validity),
		// DigitalSignature alone. The key is ECDSA, so it never transports a
		// key, and KeyEncipherment would claim a use it cannot serve.
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, host)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, r.cert, &key.PublicKey, r.key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("pki: issue leaf for %s: %w", commonName, err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

// randomSerial draws a serial from the full serialBits range. There is no
// ledger: uniqueness per issuer is the requirement, and the collision
// probability over any number of certificates one router issues is negligible.
func randomSerial() (*big.Int, error) {
	one := big.NewInt(1)
	// rand.Int draws from [0, max). Drawing from one below 2^serialBits and
	// adding one gives [1, 2^serialBits - 1]: never zero, which is not a valid
	// serial, and never wider than serialBits.
	max := new(big.Int).Lsh(one, serialBits)
	max.Sub(max, one)

	serial, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, fmt.Errorf("pki: draw certificate serial: %w", err)
	}
	return serial.Add(serial, one), nil
}
