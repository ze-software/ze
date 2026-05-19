// Design: plan/spec-ipsec-1-pki-store.md -- PKI show command handlers

package pki

import (
	"crypto/x509"
	"sort"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
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
		Data: map[string]any{
			"certificates": rows,
			"count":        len(rows),
		},
	}, nil
}

func handleShowPKICertificate(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: show pki certificate <name>",
		}, nil
	}
	name := args[0]

	s := get()
	now := time.Now()

	if ca, ok := s.caCerts[name]; ok {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   certDetailMap(name, "ca", ca.Certificate, false, now, s),
		}, nil
	}

	if entry, ok := s.certificates[name]; ok {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   certDetailMap(name, "device", entry.Certificate, entry.PrivateKey != nil, now, s),
		}, nil
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
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   map[string]any{"error": "pki: certificate " + name + " not found", "available": names},
		}, nil
	}
	return &plugin.Response{Status: plugin.StatusError, Data: "pki: certificate " + name + " not found"}, nil
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
