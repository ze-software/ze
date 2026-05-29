package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gokrazy/internal/config"
)

func GetTLSHttpClientByTLSFlag(tlsFlag string, tlsInsecure bool, baseUrl *url.URL) (*http.Client, bool, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		log.Printf("initializing x509 system cert pool failed (%v), falling back to empty cert pool", err)
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if tlsFlag == "off" {
		return getTLSHTTPClient(rootCAs, tlsInsecure), false, nil
	}

	foundMatchingCertificate := false
	// Append user specified certificate(s)
	if tlsFlag != "self-signed" && tlsFlag != "" {
		usrCert := strings.Split(tlsFlag, ",")[0]
		certBytes, err := ioutil.ReadFile(usrCert)
		if err != nil {
			return nil, false, fmt.Errorf("reading user specified certificate %s: %v", usrCert, err)
		}
		rootCAs.AppendCertsFromPEM(certBytes)
	} else {
		// Try to find a certificate in the local host config
		hostConfig := config.HostnameSpecific(baseUrl.Hostname())
		certPath := filepath.Join(string(hostConfig), "cert.pem")
		if _, err := os.Stat(certPath); !os.IsNotExist(err) {
			foundMatchingCertificate = true
			log.Printf("Using certificate %s", certPath)
			certBytes, err := ioutil.ReadFile(certPath)
			if err != nil {
				return nil, false, fmt.Errorf("reading certificate %s: %v", certPath, err)
			}
			rootCAs.AppendCertsFromPEM(certBytes)
		}
	}

	return getTLSHTTPClient(rootCAs, tlsInsecure), foundMatchingCertificate, nil
}

func getTLSHTTPClient(trustStore *x509.CertPool, tlsInsecure bool) *http.Client {
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.TLSClientConfig = &tls.Config{
		RootCAs:            trustStore,
		InsecureSkipVerify: tlsInsecure,
	}

	return &http.Client{
		Transport: httpTransport,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}

			last := via[len(via)-1]
			if last.URL.Host != r.URL.Host {
				// Do not send credentials to other targets
				return nil
			}
			if u := last.URL.User; u != nil {
				if pass, ok := u.Password(); ok {
					// Carry over basic authentication across redirects:
					r.SetBasicAuth(u.Username(), pass)
				}
			}
			return nil
		},
	}
}

func GetRemoteScheme(baseUrl *url.URL) (string, error) {
	// probe for https redirect, before sending credentials via http
	probeClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // do not follow redirects
		},
	}
	probeResp, err := probeClient.Get("http://" + baseUrl.Host)
	if err != nil {
		return "", fmt.Errorf("probing url for https: %v", err)
	}
	probeLocation, err := probeResp.Location()
	if err != nil {
		// remote did not upgrade us to HTTPS
		return "http", nil
	}
	return probeLocation.Scheme, nil
}
