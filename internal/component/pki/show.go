// Design: plan/spec-ipsec-1-pki-store.md -- PKI show command handlers

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

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
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

func handleShowPKICertificate(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: show pki certificate <name> [pem | bundle pem | fingerprint [sha256|sha384|sha512]]",
		}, nil
	}

	name, ca, entry, errResp := lookupCert(args[0])
	if errResp != nil {
		return errResp, nil
	}

	sub := args[1:]
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
			Error:  "usage: show pki certificate <name> [pem | bundle pem | fingerprint [sha256|sha384|sha512]]",
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

	if len(names) > 0 {
		return "", nil, nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: certificate " + name + " not found (available: " + strings.Join(names, ", ") + ")",
		}
	}
	return "", nil, nil, &plugin.Response{
		Status: plugin.StatusError,
		Error:  "pki: certificate " + name + " not found",
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

	if entry != nil && entry.RawInter != nil {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.RawInter})...)
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
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: certificate " + entry.Name + " has no private key",
		}, nil
	}

	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.Raw})
	if entry.RawInter != nil {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.RawInter})...)
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
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: unsupported hash algorithm " + algo + " (use sha256, sha384, sha512)",
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
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: marshal private key: " + err.Error(),
		}
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
}

func formatFingerprint(hexStr string) string {
	var b strings.Builder
	for i, c := range hexStr {
		if i > 0 && i%2 == 0 {
			b.WriteByte(':')
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
		if entry, ok := s.certificates[name]; ok && entry.Intermediate != nil {
			intermediatePool.AddCert(entry.Intermediate)
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
