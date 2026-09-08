// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate revocation
// Detail: established.go -- runEstablished, which owns this check; fsm.go -- buildPeerTLSConfig
// RFC: rfc/short/rfc9190.md -- Section 5.4; rfc/short/rfc5216.md -- Section 5.4

package engine

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/ze-software/ze/internal/core/eap"
)

// The bounds on one post-authentication status fetch.
//
// The request goes to a responder named by a certificate ze has already
// path-validated against its own trust anchor, so the destination is not
// attacker-chosen the way a Hash and URL fetch is (certurl.go). It still travels
// a network the peer sits on, so every answer is bounded.
const (
	// ocspFetchTimeout bounds connect, headers and body together. The tunnel is
	// already up and nothing waits on this check, so the budget is generous
	// rather than tight; what it must do is END.
	ocspFetchTimeout = 10 * time.Second

	// ocspMaxBytes caps the response body. A single-certificate OCSP response is
	// a few hundred octets, and one carrying the responder's certificate is a
	// few thousand, so 64 KiB is far above any real answer.
	ocspMaxBytes = 64 << 10

	// ocspMaxHeaderBytes caps the RESPONSE HEADER, which Go otherwise bounds at
	// 10 MiB. Header bytes are read before the body cap governs anything.
	ocspMaxHeaderBytes = 8 << 10

	// ocspContentType is the media type RFC 6960 Appendix A.1 fixes for a request
	// posted over HTTP.
	ocspContentType = "application/ocsp-request"
)

var (
	// errOCSPScheme refuses a responder URL that is not https.
	//
	// RFC 9190 Section 5.4: "An EAP peer MUST use a secure transport to verify
	// the revocation status of the server certificate." An OCSP response is
	// signed, so plain http would still be authentic, and the sentence asks for
	// more than authenticity: over http a network that can see the request learns
	// which certificate this peer is asking about, and one that can drop it
	// chooses when the answer never arrives.
	errOCSPScheme = errors.New(
		"ike: the certificate names an OCSP responder that is not https, and RFC 9190 Section 5.4 requires a " +
			"secure transport to verify the revocation status of the server certificate")

	// errOCSPNoResponder names a certificate that carries no responder ze may
	// use. It is a reason the check produced no answer, never a refusal.
	errOCSPNoResponder = errors.New("ike: no https OCSP responder is named by the authenticator's certificate chain")

	// errOCSPTooLarge refuses a response body past the cap.
	errOCSPTooLarge = errors.New("ike: the OCSP responder returned more than the response cap")
)

// serverCertStatus is what one post-authentication check concluded about the
// authenticator's certificate chain.
//
// The zero value is "no answer", which is a NAMED outcome rather than a silent
// default: a responder that cannot be reached and a responder that reports the
// certificate good lead to different lines in the log and different actions on
// the SA (ai/rules/principles.md).
type serverCertStatus uint8

const (
	// statusUnchecked says the check obtained no answer: no https responder is
	// named, or none answered. The SA is left alone.
	statusUnchecked serverCertStatus = iota

	// statusGood says a responder answered, and every certificate it answered
	// about is still valid.
	statusGood

	// statusRevoked says a responder answered and its answer does not say the
	// certificate is good. The SA comes down.
	statusRevoked
)

// serverCertRecheck is the post-authentication revocation check of one IKE SA.
//
// RFC 9190 Section 5.4: "To enable revocation checking in situations where
// EAP-TLS peers do not implement or use OCSP stapling, and where network
// connectivity is not available prior to authentication completion, EAP-TLS peer
// implementations MUST also support checking for certificate revocation after
// authentication completes and network connectivity is available."
//
// RFC 5216 Section 5.4 asks for the same thing with no TLS version attached, so
// this check runs whichever version the EAP-TLS exchange negotiated.
//
// THE ORDER IS THE POINT, and it is why this lives in the engine rather than in
// the EAP peer. The EAP-TLS peer has no network of its own: the tunnel it
// authenticated is what gives it one. So the check cannot run inside the
// handshake, and it MUST NOT hold the Child SA up either, because the Child SA is
// the connectivity the check needs. runEstablished starts it after the Child SA
// is installed and the routes are announced, and the check then runs beside the
// SA it may bring down.
//
// The caller MUST call stop when it is done with the check, and stop returns only
// after the worker has ended.
type serverCertRecheck struct {
	// revoked carries the one verdict that acts on the SA. It is buffered so the
	// worker never blocks on an owner loop that has already left.
	revoked chan error

	cancel context.CancelFunc
	done   chan struct{}
}

// startServerCertRecheck starts the post-authentication revocation check for one
// IKE SA, and answers nil when there is nothing to check.
//
// There is nothing to check when this SA did not authenticate with EAP-TLS as
// the peer, or when the EAP-TLS exchange accepted no certificate chain. Neither
// is an error: the responder role runs no such check (it validated the client
// chain inside the handshake and needed no network for it), and a password EAP
// method presents no certificate at all.
//
// The caller MUST call stop on the value returned.
func startServerCertRecheck(sa *SA, client *http.Client, log *slog.Logger) *serverCertRecheck {
	session, ok := sa.EAPSession.(*eap.PeerSession)
	if !ok || session == nil {
		return nil
	}
	chains := session.ServerChains()
	if len(chains) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &serverCertRecheck{
		revoked: make(chan error, 1),
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	go r.run(ctx, chains, sa.PeerName, client, log)
	return r
}

// verdict answers the channel the owner loop selects on. It carries at most one
// value, the reason the authenticator's certificate must no longer be trusted.
//
// A nil receiver answers a nil channel, which blocks forever, so an owner loop
// selecting on it needs no branch for an SA that has nothing to check.
func (r *serverCertRecheck) verdict() <-chan error {
	if r == nil {
		return nil
	}
	return r.revoked
}

// stop ends the check and waits for its goroutine.
//
// It MUST be called by whoever called startServerCertRecheck, and it is safe on
// a nil receiver so the caller needs no branch. A second call is safe too:
// context.CancelFunc is idempotent and the wait on a closed channel returns at
// once.
func (r *serverCertRecheck) stop() {
	if r == nil {
		return
	}
	r.cancel()
	<-r.done
}

// run performs the check once and reports what it found.
//
// ONCE is deliberate. RFC 9190 Section 5.4 asks for a check "after
// authentication completes and network connectivity is available", which is a
// single moment, and a periodic re-check is a different feature with its own
// cost on every responder Ze talks to. The next authentication runs the check
// again from the start.
func (r *serverCertRecheck) run(
	ctx context.Context,
	chains [][]*x509.Certificate,
	peerName string,
	client *http.Client,
	log *slog.Logger,
) {
	defer close(r.done)

	status, err := checkServerChainStatus(ctx, chains, client, time.Now())
	switch status {
	case statusRevoked:
		log.Warn("ike: the authenticator certificate was revoked after authentication, closing the SA",
			"peer", peerName, "error", err)
		r.revoked <- err
	case statusGood:
		log.Info("ike: the authenticator certificate is still valid after authentication",
			"peer", peerName)
	case statusUnchecked:
		// The SA stays up. RFC 9190 Section 5.4's own answer to an unverified
		// status is the SHOULD NOT pair that follows it (an EAP peer "SHOULD NOT
		// trust the network ... until it has verified the revocation status"),
		// and ze does not implement those, so this line is what an operator has.
		// Tearing a working tunnel down because a responder is unreachable would
		// turn one responder outage into an outage of every tunnel.
		log.Warn("ike: the authenticator certificate revocation status was not re-checked after the tunnel came up",
			"peer", peerName, "reason", err)
	}
}

// checkServerChainStatus asks each certificate's own responder about it, over
// the secure transport RFC 9190 Section 5.4 requires, and answers what the
// responders said.
//
// EVERY certificate on the chain except the trust anchor is asked about, which
// is the population Section 5.4's first sentence names. A chain that
// x509.Certificate.Verify built runs leaf first and trust anchor last, so
// chain[i+1] issued chain[i] and the last element is the anchor.
//
// A responder that ANSWERS anything other than "good" refuses the certificate,
// and that includes an expired answer, an answer about the wrong certificate and
// an answer signed by nobody this issuer delegated to (eap.CheckCertificateStatus).
// A responder that gives no answer at all leaves the status unchecked: an
// attacker on the path can drop packets, and dropping them MUST NOT be a way to
// have a revoked certificate accepted -- but it is also not evidence of
// revocation, so the two outcomes are kept apart.
func checkServerChainStatus(
	ctx context.Context,
	chains [][]*x509.Certificate,
	client *http.Client,
	now time.Time,
) (serverCertStatus, error) {
	if len(chains) == 0 {
		// The chain is what the check is ABOUT, so an empty set is a failure to
		// check and never a clean result (ai/rules/principles.md).
		return statusUnchecked, errors.New("ike: no verified certificate chain to re-check")
	}

	answered := 0
	var unreachable error

	for _, chain := range chains {
		for i := 0; i+1 < len(chain); i++ {
			cert := chain[i]
			issuer := chain[i+1]

			der, err := fetchCertificateStatus(ctx, cert, issuer, client)
			if err != nil {
				// Kept rather than returned: another certificate on the chain, or
				// another chain, may still carry a responder that answers, and a
				// revocation found there is the answer that matters.
				if unreachable == nil {
					unreachable = err
				}
				continue
			}
			answered++
			if err := eap.CheckCertificateStatus(der, cert, issuer, now); err != nil {
				return statusRevoked, err
			}
		}
	}

	if answered == 0 {
		return statusUnchecked, unreachable
	}
	return statusGood, nil
}

// fetchCertificateStatus asks one certificate's OCSP responder about it and
// answers the DER response.
//
// THE SCHEME IS CHECKED BEFORE ANY CONNECTION, so an http responder costs no
// I/O and leaks no request. RFC 9190 Section 5.4: "An EAP peer MUST use a secure
// transport to verify the revocation status of the server certificate." https is
// that transport, and the responder's own certificate is verified by the
// system trust store the appliance image carries.
//
// The loop is bounded by the certificate's authority information access
// extension, which x509 parsed from a certificate ze already validated.
func fetchCertificateStatus(
	ctx context.Context,
	cert, issuer *x509.Certificate,
	client *http.Client,
) ([]byte, error) {
	request, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return nil, fmt.Errorf("ike: build the OCSP request for %q: %w", cert.Subject, err)
	}

	var refused error
	for _, raw := range cert.OCSPServer {
		responder, pErr := url.Parse(raw)
		if pErr != nil {
			refused = fmt.Errorf("ike: the OCSP responder url of %q does not parse: %w", cert.Subject, pErr)
			continue
		}
		if responder.Scheme != "https" || responder.Host == "" {
			refused = fmt.Errorf("%w: %q names %q", errOCSPScheme, cert.Subject, raw)
			continue
		}
		return postOCSPRequest(ctx, responder, request, client)
	}

	if refused != nil {
		return nil, refused
	}
	return nil, fmt.Errorf("%w: %q names none", errOCSPNoResponder, cert.Subject)
}

// postOCSPRequest performs the one bounded HTTP exchange this check makes.
//
// RFC 6960 Appendix A.1: a request is sent as an HTTP POST whose body is the DER
// request and whose Content-Type is "application/ocsp-request". POST rather than
// the base64 GET form, because a GET url is cached by intermediaries and the
// answer to "is this certificate revoked" is exactly the answer that must not
// come from a cache.
func postOCSPRequest(ctx context.Context, responder *url.URL, request []byte, client *http.Client) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, ocspFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responder.String(), bytes.NewReader(request))
	if err != nil {
		return nil, fmt.Errorf("ike: build the OCSP request to %s: %w", responder.Redacted(), err)
	}
	req.Header.Set("Content-Type", ocspContentType)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ike: ask the OCSP responder %s: %w", responder.Redacted(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ike: the OCSP responder %s answered status %d", responder.Redacted(), resp.StatusCode)
	}

	// One octet beyond the cap is read so that hitting the cap is
	// distinguishable from a body that happens to be exactly the cap.
	body, err := io.ReadAll(io.LimitReader(resp.Body, ocspMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("ike: read the OCSP response from %s: %w", responder.Redacted(), err)
	}
	if len(body) > ocspMaxBytes {
		return nil, fmt.Errorf("%w: %s returned more than %d octets", errOCSPTooLarge, responder.Redacted(), ocspMaxBytes)
	}
	return body, nil
}

// ocspClient builds the bounded HTTP client the post-authentication check uses.
// Every control is set here so a reader can audit them in one place, and so no
// caller can get a client missing one.
//
// It sets no Control hook on its dialer, and that is the one difference from
// certURLClient (certurl.go). That fetcher resolves a URL an UNAUTHENTICATED
// peer chose, which is a server-side request forgery primitive and is why it
// denies loopback, private and metadata addresses outright. This one resolves a
// URL written into a certificate that ze has already chained to its own
// configured trust anchor, and an OCSP responder on a private address is a
// normal deployment for the enterprise CA an EAP-TLS network runs on.
func ocspClient() *http.Client {
	return &http.Client{
		Timeout: ocspFetchTimeout,
		Transport: &http.Transport{
			DisableKeepAlives:      true,
			MaxIdleConns:           1,
			ResponseHeaderTimeout:  ocspFetchTimeout,
			MaxResponseHeaderBytes: ocspMaxHeaderBytes,
		},
		// Zero redirects. A redirect re-opens the scheme check above against a
		// new location, and RFC 6960 needs none: the responder answers the POST.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("ike: the OCSP responder answered with a redirect, which ze does not follow")
		},
	}
}
