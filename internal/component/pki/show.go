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
	RegisterHealth()

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificates",
			Handler:    handleShowPKICertificates,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:pki-certificate",
			Handler:    handleShowPKICertificate,
		},
	)
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

func handleShowPKICertificate(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const usage = "usage: show pki certificate name <name> [pem | bundle pem | fingerprint [sha256|sha384|sha512]]"
	// The certificate name is the typed `name <name>` selector
	// (`show pki certificate name <name> ...`). The remaining args select the
	// output form (pem, bundle pem, fingerprint). A bare positional name is
	// accepted as a fallback for programmatic callers.
	certName := ""
	if ctx != nil {
		certName = ctx.Selector("name")
	}
	if certName == "" {
		if len(args) == 0 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  usage,
			}, nil
		}
		certName = args[0]
		args = args[1:]
	}

	name, ca, entry, errResp := lookupCert(certName)
	if errResp != nil {
		return errResp, nil
	}

	sub := args
	switch {
	case len(sub) == 0:
		return certDetail(name, ca, entry)
	case len(sub) == 1 && sub[0] == "pem":
		return certPEM(ca, entry)
	case len(sub) == 2 && sub[0] == "bundle" && sub[1] == "pem":
		return certBundlePEM(entry)
	case len(sub) >= 1 && sub[0] == "fingerprint":
		algo := algoSHA256
		if len(sub) > 1 {
			algo = strings.ToLower(sub[1])
		}
		return certFingerprint(name, ca, entry, algo)
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  usage,
		}, nil
	}
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
	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})

	if entry != nil {
		for _, inter := range entry.RawIntermediates {
			out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: inter})...)
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"pem": string(out)},
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

	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.Raw})
	for _, inter := range entry.RawIntermediates {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: inter})...)
	}

	keyPEM, keyErr := marshalPrivateKeyPEM(entry.PrivateKey)
	if keyErr != nil {
		return keyErr, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"pem": string(out) + string(keyPEM)},
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
			"name":        name,
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
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
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
		"name":            name,
		"type":            typ,
		"subject":         cert.Subject.String(),
		"issuer":          cert.Issuer.String(),
		"serial":          cert.SerialNumber.String(),
		"not-before":      cert.NotBefore.UTC().Format(time.RFC3339),
		"not-after":       cert.NotAfter.UTC().Format(time.RFC3339),
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
