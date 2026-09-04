// Design: docs/architecture/pki/pki-store.md -- PKI show command handlers

package pki

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	registerHealth()

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificates",
			Handler:    handleShowPKICertificates,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificate",
			Handler:    handleShowPKICertificate,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificate-pem",
			Handler:    handleShowPKICertificatePEM,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificate-bundle-pem",
			Handler:    handleShowPKICertificateBundlePEM,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificate-fingerprint",
			Handler:    handleShowPKICertificateFingerprint,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-local-ca-pem",
			Handler:    handleShowPKILocalCAPEM,
		},
	)
}

// afterLocalCAPEM names the export keyword in an unexpected-argument answer.
const afterLocalCAPEM = "the local certificate authority export"

// handleShowPKILocalCAPEM answers the local certificate authority root in PEM,
// which an operator pastes into a client's `pki ca <name> certificate` block so
// that client trusts the ISSUER rather than a copy of one leaf.
//
// The private key is not reachable from here. Root exposes no accessor for it,
// by design: it leaves the pki package only into a signing operation.
//
// The subject and the expiry travel with the PEM because an operator
// distributing a root has to know which root they copied and how long it lasts,
// and reading either out of the PEM needs a second tool.
func handleShowPKILocalCAPEM(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if extra := unexpectedAfter(args, afterLocalCAPEM); extra != nil {
		return extra, nil
	}

	root := loadedRoot()
	if root == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: this daemon has loaded no local certificate authority root",
		}, nil
	}

	cert := root.Certificate()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldPEM:      string(root.CertificatePEM()),
			"subject":     cert.Subject.String(),
			fieldNotAfter: cert.NotAfter.UTC().Format(time.RFC3339),
		},
	}, nil
}

func handleShowPKICertificates(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	now := time.Now()
	s := get()

	rows := make([]CertSummary, 0, len(s.caCerts)+len(s.certificates))

	for _, ca := range s.caCerts {
		rows = append(rows, certSummary(ca.Name, "ca", ca.Certificate, now.Before(ca.Certificate.NotAfter)))
	}
	for _, entry := range s.certificates {
		rows = append(rows, certSummary(entry.Name, "device", entry.Certificate, now.Before(entry.Certificate.NotAfter)))
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type != rows[j].Type {
			return rows[i].Type < rows[j].Type
		}
		return rows[i].Name < rows[j].Name
	})

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"certificates": rows,
			"count":        len(rows),
		},
	}, nil
}

// selectorName is the keyword that carries the certificate name in
// `show pki certificate name <name> ...`. An operator types it, so it is
// separate from the payload keys in types.go.
const selectorName = "name"

// certSelected answers the certificate the operator named, and the arguments
// left after the name.
//
// The name arrives as the typed `name <name>` selector, which the dispatcher
// extracts before the handler runs. A caller holding no command context reaches
// the same handler with the name as its first argument.
func certSelected(ctx *pluginserver.CommandContext, args []string) (string, []string) {
	if ctx != nil {
		if name := ctx.Selector(selectorName); name != "" {
			return name, args
		}
	}
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

// selectedCert is what one output form acts on: the certificate the operator
// named, the store entry it resolved to, and the arguments left after the name.
// Exactly one of ca and entry is set, because a name reaches one store or the
// other.
type selectedCert struct {
	name  string
	ca    *CACertEntry
	entry *CertificateEntry
	rest  []string
}

// certNamed resolves the certificate the operator named. It answers the error
// response for a missing name and for a name the store does not hold, so each
// form handler starts from a certificate or from an answer to give back.
func certNamed(ctx *pluginserver.CommandContext, args []string) (selectedCert, *plugin.Response) {
	certName, rest := certSelected(ctx, args)
	if certName == "" {
		return selectedCert{}, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: no certificate named, supply one as name <name>",
		}
	}
	name, ca, entry, errResp := lookupCert(certName)
	if errResp != nil {
		return selectedCert{}, errResp
	}
	return selectedCert{name: name, ca: ca, entry: entry, rest: rest}, nil
}

// unexpectedAfter refuses a token the form declares no value for, naming what
// the token followed.
//
// The dispatcher passes an unmatched token through to the handler once every
// declared argument is filled, so a tail nobody declared reaches here. Answering
// it is what keeps `show pki certificate name dev-1 garbage` from reading as the
// detail of dev-1 (ai/rules/evidence.md: fail closed).
func unexpectedAfter(rest []string, after string) *plugin.Response {
	if len(rest) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  tb.Str("pki: unexpected argument ").Quoted(rest[0]).Str(" after ").Str(after).String(),
	}
}

// afterCertName names the certificate name in an unexpected-argument answer.
const afterCertName = "the certificate name"

// handleShowPKICertificate answers the detail form: everything the store knows
// about one certificate.
func handleShowPKICertificate(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	cert, errResp := certNamed(ctx, args)
	if errResp != nil {
		return errResp, nil
	}
	if extra := unexpectedAfter(cert.rest, afterCertName); extra != nil {
		return extra, nil
	}
	return certDetail(cert.name, cert.ca, cert.entry)
}

// handleShowPKICertificatePEM answers the `pem` form: the certificate and its
// intermediates, and never a private key.
func handleShowPKICertificatePEM(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	cert, errResp := certNamed(ctx, args)
	if errResp != nil {
		return errResp, nil
	}
	if extra := unexpectedAfter(cert.rest, afterCertName); extra != nil {
		return extra, nil
	}
	return certPEM(cert.ca, cert.entry)
}

// handleShowPKICertificateBundlePEM answers the `bundle pem` form: the
// certificate, its intermediates and its private key.
func handleShowPKICertificateBundlePEM(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	cert, errResp := certNamed(ctx, args)
	if errResp != nil {
		return errResp, nil
	}
	if extra := unexpectedAfter(cert.rest, afterCertName); extra != nil {
		return extra, nil
	}
	return certBundlePEM(cert.entry)
}

// handleShowPKICertificateFingerprint answers the `fingerprint` form. The
// algorithm is the one word the model offers after the keyword, and SHA-256
// when the operator types none.
func handleShowPKICertificateFingerprint(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	cert, errResp := certNamed(ctx, args)
	if errResp != nil {
		return errResp, nil
	}
	if len(cert.rest) > 1 {
		return unexpectedAfter(cert.rest[1:], "the hash algorithm"), nil
	}
	algo := algoSHA256
	if len(cert.rest) > 0 {
		algo = strings.ToLower(cert.rest[0])
	}
	return certFingerprint(cert.name, cert.ca, cert.entry, algo)
}

func lookupCert(name string) (string, *CACertEntry, *CertificateEntry, *plugin.Response) {
	s := get()

	if ca, ok := s.caCerts[name]; ok {
		return name, ca, nil, nil
	}
	if entry, ok := s.certificates[name]; ok {
		return name, nil, entry, nil
	}

	names := make([]string, 0, len(s.caCerts)+len(s.certificates))
	for n := range s.caCerts {
		names = append(names, n)
	}
	for n := range s.certificates {
		names = append(names, n)
	}
	sort.Strings(names)

	var tb textbuf.Buffer
	if len(names) > 0 {
		return "", nil, nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("pki: certificate ").Str(name).Str(" not found (available: ").Join(names, ", ").Byte(')').String(),
		}
	}
	return "", nil, nil, &plugin.Response{
		Status: plugin.StatusError,
		Error:  tb.Str("pki: certificate ").Str(name).Str(" not found").String(),
	}
}

func certRawDER(ca *CACertEntry, entry *CertificateEntry) []byte {
	if ca != nil {
		return ca.Raw
	}
	return entry.Raw
}

func certDetail(name string, ca *CACertEntry, entry *CertificateEntry) (*plugin.Response, error) {
	s := get()
	now := time.Now()

	if ca != nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map(certDetailMap(name, "ca", ca.Certificate, false, now, s)),
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map(certDetailMap(name, "device", entry.Certificate, entry.PrivateKey != nil, now, s)),
	}, nil
}

func certPEM(ca *CACertEntry, entry *CertificateEntry) (*plugin.Response, error) {
	raw := certRawDER(ca, entry)
	out := pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: raw})

	if entry != nil {
		for _, inter := range entry.RawIntermediates {
			out = append(out, pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: inter})...)
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldPEM: string(out)},
	}, nil
}

func certBundlePEM(entry *CertificateEntry) (*plugin.Response, error) {
	if entry == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: bundle is only available for device certificates",
		}, nil
	}
	if entry.PrivateKey == nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("pki: certificate ").Str(entry.Name).Str(" has no private key").String(),
		}, nil
	}

	out := pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: entry.Raw})
	for _, inter := range entry.RawIntermediates {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: inter})...)
	}

	keyPEM, keyErr := marshalPrivateKeyPEM(entry.PrivateKey)
	if keyErr != nil {
		return keyErr, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldPEM: string(out) + string(keyPEM)},
	}, nil
}

const (
	algoSHA256 = "sha256"
	algoSHA384 = "sha384"
	algoSHA512 = "sha512"
)

func certFingerprint(name string, ca *CACertEntry, entry *CertificateEntry, algo string) (*plugin.Response, error) {
	raw := certRawDER(ca, entry)

	var fp string
	switch algo {
	case algoSHA256:
		sum := sha256.Sum256(raw)
		fp = hex.EncodeToString(sum[:])
	case algoSHA384:
		sum := sha512.Sum384(raw)
		fp = hex.EncodeToString(sum[:])
	case algoSHA512:
		sum := sha512.Sum512(raw)
		fp = hex.EncodeToString(sum[:])
	default:
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("pki: unsupported hash algorithm ").Str(algo).Str(" (use sha256, sha384, sha512)").String(),
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldName:     name,
			"algorithm":   algo,
			"fingerprint": formatFingerprint(fp),
		},
	}, nil
}

func marshalPrivateKeyPEM(key any) ([]byte, *plugin.Response) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		var tb textbuf.Buffer
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("pki: marshal private key: ").Err(err).String(),
		}
	}
	return pem.EncodeToMemory(&pem.Block{Type: pemBlockPrivateKey, Bytes: keyDER}), nil
}

func formatFingerprint(hexStr string) string {
	var b textbuf.Buffer
	for i, c := range hexStr {
		if i > 0 && i%2 == 0 {
			b.Byte(':')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func certDetailMap(name, typ string, cert *x509.Certificate, hasKey bool, now time.Time, s *storeState) map[string]any {
	sans := make([]string, 0, len(cert.DNSNames)+len(cert.EmailAddresses))
	sans = append(sans, cert.DNSNames...)
	sans = append(sans, cert.EmailAddresses...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}

	chainValid := true
	chainError := ""
	if typ == "device" {
		caPool := x509.NewCertPool()
		for _, ca := range s.caCerts {
			caPool.AddCert(ca.Certificate)
		}
		intermediatePool := x509.NewCertPool()
		if entry, ok := s.certificates[name]; ok {
			for _, inter := range entry.Intermediates {
				intermediatePool.AddCert(inter)
			}
		}
		_, err := cert.Verify(x509.VerifyOptions{
			Roots:         caPool,
			Intermediates: intermediatePool,
			CurrentTime:   now,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		})
		if err != nil {
			chainValid = false
			chainError = err.Error()
		}
	}

	m := map[string]any{
		fieldName:         name,
		fieldType:         typ,
		"subject":         cert.Subject.String(),
		"issuer":          cert.Issuer.String(),
		"serial":          cert.SerialNumber.String(),
		"not-before":      cert.NotBefore.UTC().Format(time.RFC3339),
		fieldNotAfter:     cert.NotAfter.UTC().Format(time.RFC3339),
		"key-algorithm":   cert.PublicKeyAlgorithm.String(),
		"key-size":        keySize(cert),
		"sans":            sans,
		"key-usage":       keyUsageStrings(cert.KeyUsage),
		"has-private-key": hasKey,
		"chain-valid":     chainValid,
	}
	if chainError != "" {
		m["chain-error"] = chainError
	}
	return m
}
