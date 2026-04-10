package httpclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gokrazy/internal/config"
	"github.com/gokrazy/internal/tlsflag"
	"github.com/gokrazy/internal/updateflag"
)

func For(updateVal updateflag.Value, cfg *config.Struct) (_ *http.Client, foundMatchingCertificate bool, updateBaseURL *url.URL, _ error) {
	schema := "http"
	certPath, _, err := tlsflag.CertificatePathsFor(cfg.Update.UseTLS, cfg.Hostname)
	if err != nil {
		return nil, false, nil, err
	}
	if certPath != "" {
		schema = "https"
	}

	update, err := cfg.Update.WithFallbackToHostSpecific(cfg.Hostname)
	if err != nil {
		return nil, false, nil, err
	}

	if update.HTTPPort == "" {
		update.HTTPPort = "80"
	}

	if update.HTTPSPort == "" {
		update.HTTPSPort = "443"
	}

	if update.Hostname == "" {
		update.Hostname = cfg.Hostname
	}

	updateBaseURL, err = updateVal.BaseURL(update.HTTPPort, update.HTTPSPort, schema, update.Hostname, update.HTTPPassword)
	if err != nil {
		return nil, false, nil, err
	}

	hc, fmc, err := GetTLSHttpClientByTLSFlag(update.UseTLS, cfg.InternalCompatibilityFlags.Insecure, updateBaseURL)
	if err != nil {
		return nil, false, nil, fmt.Errorf("getting http client by tls flag: %v", err)
	}
	return hc, fmc, updateBaseURL, nil
}
