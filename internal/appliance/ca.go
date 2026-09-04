// Design: docs/architecture/appliance/builder.md -- the appliance certificate authority
// Related: cmd_init.go -- writeTLSSecrets, the one write path for the serving material
// Related: cmd_push.go -- loadDeviceTLS, which trusts the root this file writes

package appliance

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	// caCertFileName and caKeyFileName hold the appliance's own certificate
	// authority beside the material it signs. The root persists for the life of
	// the appliance: a leaf reissued by `ze appliance replace-cert` has to come
	// from the root the device's trust file already carries, or every later
	// push fails to verify the device.
	caCertFileName = "ca-cert.pem"
	caKeyFileName  = "ca-key.pem"

	// applianceRootMargin is how much longer the root lives than the leaf it
	// signs at initialization. `tls.validity-years` is the life the operator
	// asked the SERVING certificate to have, and a chain stops verifying the
	// moment its issuer expires, so the root takes that life plus this margin
	// and a leaf can never outlive the root that signed it.
	//
	// One year also covers a certificate reissued during the appliance's first
	// year, which still expires inside the root's window. A leaf reissued later
	// is bounded by the root instead, which is what a certificate authority
	// does to everything it signed.
	applianceRootMargin = 365 * 24 * time.Hour

	// yearDuration is the year `tls.validity-years` counts. It is the same
	// 365-day year the appliance has always used for that leaf.
	yearDuration = 365 * 24 * time.Hour

	pemBlockCertificate = "CERTIFICATE"
	pemBlockPrivateKey  = "PRIVATE KEY"
)

var errCARootStoreKeyUnknown = errors.New("appliance: the certificate authority store holds the root certificate and the root key, and nothing else")

// applianceRootStore keeps the appliance's certificate authority in two files
// beside cert.pem, and satisfies pki.RootStore. The build host runs no daemon
// and has no zefs blob, so the two registered zefs key names address files here
// instead.
//
// The certificate is public material and is stored as it is. The key goes
// through the appliance passphrase, exactly as key.pem does, so the root key is
// encrypted at rest whenever the appliance's other secrets are.
type applianceRootStore struct {
	certPath   string
	keyPath    string
	passphrase []byte
}

func newApplianceRootStore(baseDir, name string, passphrase []byte) *applianceRootStore {
	return &applianceRootStore{
		certPath:   filepath.Join(tLSDir(baseDir, name), caCertFileName),
		keyPath:    filepath.Join(tLSDir(baseDir, name), caKeyFileName),
		passphrase: passphrase,
	}
}

// path maps a registered zefs key onto the file that holds it, and reports
// whether that file is the private half. A name that is neither key is an error
// rather than a path: a caller asking for something this store does not hold
// must not be answered with a file the next write would create.
func (s *applianceRootStore) path(name string) (path string, private bool, err error) {
	switch name {
	case zefs.KeyCACert.Pattern:
		return s.certPath, false, nil
	case zefs.KeyCAKey.Pattern:
		return s.keyPath, true, nil
	}
	return "", false, fmt.Errorf("%w: %q is neither", errCARootStoreKeyUnknown, name)
}

// ReadFile returns the stored value for a registered key name.
func (s *applianceRootStore) ReadFile(name string) ([]byte, error) {
	path, private, err := s.path(name)
	if err != nil {
		return nil, err
	}
	if !private {
		return os.ReadFile(path) //nolint:gosec // path built from the appliance directory
	}
	return readSecret(path, s.passphrase)
}

// WriteFile stores data under a registered key name. perm is accepted and
// ignored: WriteSecret writes 0600 itself, which is the mode both files need.
func (s *applianceRootStore) WriteFile(name string, data []byte, _ fs.FileMode) error {
	path, private, err := s.path(name)
	if err != nil {
		return err
	}
	if !private {
		return WriteSecret(path, data, nil)
	}
	return WriteSecret(path, data, s.passphrase)
}

// Exists reports whether the file behind a registered key name is there. An
// unknown name holds nothing, which is the answer that keeps the caller from
// reading it.
func (s *applianceRootStore) Exists(name string) bool {
	path, _, err := s.path(name)
	if err != nil {
		return false
	}
	_, statErr := os.Stat(path)
	return statErr == nil
}

// issueWebLeaf returns the appliance's serving material: a certificate file
// holding the LEAF first and the ROOT second, and the leaf's private key.
//
// The root is what makes the device trustable after its certificate changes.
// `loadDeviceTLS` (cmd_push.go) puts every certificate in the file into its
// pool, so a push validates a leaf the file has never seen as long as this root
// signed it.
//
// Leaf first is required, not cosmetic. `certExpiry` (cmd_show.go),
// `validateTLSPair` (cmd_cert.go), `checkCertExpiry`
// (internal/component/doctor/checks_tls.go) and `selfcert.NewTLSConfig` on the
// device's own boot path each read the FIRST block only, and the serving
// certificate is the answer every one of them wants.
func issueWebLeaf(baseDir, name string, cfg *applianceConfig, passphrase []byte) (certPEM, keyPEM []byte, err error) {
	leafValidity := time.Duration(cfg.TLS.ValidityYears) * yearDuration

	store := newApplianceRootStore(baseDir, name, passphrase)
	root, err := zepki.LoadOrGenerateRootFor(store, leafValidity+applianceRootMargin)
	if err != nil {
		return nil, nil, fmt.Errorf("appliance certificate authority: %w", err)
	}

	var extraNames []string
	if cfg.TLS.CertName != "" {
		extraNames = []string{cfg.TLS.CertName}
	}

	// The common name names the device in the subject and is never matched
	// against a hostname; the SANs are what a client verifies.
	commonName := cfg.Identity.Hostname
	if commonName == "" {
		commonName = cfg.Identity.Name
	}

	leaf, err := root.IssueLeafFor(commonName, selfcert.WebCertHosts(cfg.SSH.Host, extraNames), leafValidity)
	if err != nil {
		return nil, nil, fmt.Errorf("issue appliance certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(leaf.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal appliance private key: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: leaf.Certificate[0]})
	certPEM = append(certPEM, root.CertificatePEM()...)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlockPrivateKey, Bytes: keyDER})
	return certPEM, keyPEM, nil
}
