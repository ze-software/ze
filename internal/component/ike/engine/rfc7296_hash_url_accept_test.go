// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- certificate payloads, Hash and URL
// Related: rfc7296_cert_chain_test.go -- the wpc fixtures, the send half and the OFF default
//
// VALIDATES: RFC 7296 Section 3.6, the accept half: with hash-and-url configured, a
// received encoding 12 and a received encoding 13 are each resolved over HTTP and accepted.
// PREVENTS: the row going green on the send half while one received format is dropped.

package engine

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.6 names SHA-1 for Hash and URL
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestRFC7296ReceivedHashAndURLFormatsAreAcceptedWhenConfigured proves Ze accepts both
// Hash and URL formats with HTTP URLs once configured to.
//
// RFC 7296 Section 3.6: "Implementations MUST be capable of being configured to send and
// accept up to four X.509 certificates in support of authentication, and also MUST be
// capable of being configured to send and accept the two Hash and URL formats (with HTTP
// URLs)."
//
// Method: one SA per format with hash-and-url ON. Encoding 12 names the leaf DER and
// encoding 13 names a DER CertificateBundle of the leaf and one intermediate, each served
// by a loopback HTTP server and hashed with SHA-1. storeRemoteCerts runs twice, as for a
// retransmission: the first delivery starts the lookup, the second finds it cached. The
// stored peer certificate is the leaf, and the bundle's intermediate reaches the chain.
// The negative polarity (a received Hash and URL payload is dropped when not configured)
// is TestChuHashAndURLIsOffByDefault.
func TestRFC7296ReceivedHashAndURLFormatsAreAcceptedWhenConfigured(t *testing.T) {
	chain := wpcChain(t, 1)
	leaf := chain[0]
	bundle, err := encodeCertBundle(chain)
	if err != nil {
		t.Fatalf("encodeCertBundle: %v", err)
	}
	loopback := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

	cases := []struct {
		name          string
		encoding      uint8
		body          []byte
		intermediates int
	}{
		{"encoding 12 certificate", wire.CertEncodingHashURL, leaf, 0},
		{"encoding 13 bundle", wire.CertEncodingHashURLBundle, bundle, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wpcFreshCache(t)
			srv, hits := wpcServer(t, tc.body)
			sum := sha1.Sum(tc.body) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
			payload := &wire.PayloadCERT{CertEncoding: tc.encoding, CertData: wpcHashAndURL(sum[:], srv.URL)}

			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth = wpcAuth()
			sa.PeerCfg.Auth.HashAndURL = true
			sa.PeerCfg.Auth.CertificateURL = "http://pki.example/device.der"
			sa.PeerCfg.Auth.CertificateURLAllow = loopback

			// RFC requirement: RFC7296-3.6-2 positive -- configured for hash-and-url, ze
			// accepts a received encoding 12 and a received encoding 13 with an http URL:
			// each is fetched, its SHA-1 matches, and the leaf becomes the peer certificate.
			if !acceptedCertEncoding(sa, payload) {
				t.Fatalf("encoding %d was not accepted with hash-and-url configured", tc.encoding)
			}
			first := storeRemoteCerts(sa, []*wire.PayloadCERT{payload}, wpcQuietLogger())
			if !errors.Is(first, errCertURLPending) {
				t.Fatalf("first delivery returned %v, want errCertURLPending", first)
			}
			wpcAwaitCached(t, sum[:])
			if err := storeRemoteCerts(sa, []*wire.PayloadCERT{payload}, wpcQuietLogger()); err != nil {
				t.Fatalf("encoding %d with an http URL was refused: %v", tc.encoding, err)
			}
			if *hits == 0 {
				t.Fatal("the http server was never contacted")
			}
			if !bytes.Equal(sa.RemoteCertRaw, leaf) {
				t.Error("the stored peer certificate is not the leaf the URL resolved to")
			}
			if len(sa.RemoteCertChainRaw) != tc.intermediates {
				t.Fatalf("stored chain holds %d intermediates, want %d", len(sa.RemoteCertChainRaw), tc.intermediates)
			}
			if tc.intermediates == 1 && !bytes.Equal(sa.RemoteCertChainRaw[0], chain[1]) {
				t.Error("the bundle's intermediate is not the one stored")
			}
		})
	}
}
