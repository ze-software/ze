// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// RFC: rfc/short/rfc7296.md -- Certificate payloads, chain bound and Hash and URL (Section 3.6)
// Overview: auth.go -- the IKE_AUTH assembly and verification these payloads serve
// Related: certurl.go -- the bounded fetcher a Hash and URL payload is resolved through
// Related: certbundle.go -- the X.509 bundle codec the encoding-13 form carries
// Related: remote_id.go -- the identity binding applied to the certificate this returns

package engine

import (
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.6 mandates SHA-1 as the hash-and-url object identifier
	"crypto/x509"
	"fmt"
	"log/slog"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// hashAndURLNotify returns the HTTP_CERT_LOOKUP_SUPPORTED notification when the operator
// enabled hash-and-url for this peer, and nil otherwise.
//
// RFC 7296 Section 3.10: "The HTTP_CERT_LOOKUP_SUPPORTED notification MAY be included in
// any message that can include a CERTREQ payload and indicates that the sender is capable
// of looking up certificates based on an HTTP-based URL". Section 3.7 puts CERTREQ in the
// IKE_SA_INIT response and the IKE_AUTH request, and ze sends it from both.
//
// The notification is what makes the default safe. RFC 7296 Section 3.6 gates the Hash
// and URL formats on an HTTP_CERT_LOOKUP_SUPPORTED Notify payload from the receiver.
// A conforming peer therefore sends ze such a payload only after ze asks for one.
// With the leaf off ze never asks. acceptedCertEncoding drops a non-conforming payload.
func hashAndURLNotify(sa *SA) *wire.PayloadNotify {
	if !sa.PeerCfg.Auth.HashAndURL {
		return nil
	}
	return &wire.PayloadNotify{NotifyMsgType: wire.NotifyHTTPCertLookupSupported}
}

// acceptedCertEncoding reports whether a received CERT payload is one ze collects.
//
// Both IKE_AUTH walks call it. The two roles therefore cannot drift into accepting
// different encodings. A payload that fails it is DROPPED rather than refused. RFC 7296
// Section 3.6 assigns encodings ze never asked for. A peer that includes one beside a
// usable certificate has not malformed its message.
//
// Ze collects the Hash and URL encodings only when the operator sets hash-and-url for this
// peer. That is the outer half of the gate. resolveCertPayloads holds the inner half,
// because it is the single funnel every collected payload passes through. A gate that
// lived only here would be one edit away from a fetch an unauthenticated peer names.
func acceptedCertEncoding(sa *SA, p *wire.PayloadCERT) bool {
	if len(p.CertData) == 0 {
		return false
	}
	switch p.CertEncoding {
	case wire.CertEncodingX509Sig:
		return true
	case wire.CertEncodingHashURL, wire.CertEncodingHashURLBundle:
		return sa.PeerCfg.Auth.HashAndURL
	}
	return false
}

// storeRemoteCerts records the CERT payloads of one IKE_AUTH message in wire order.
//
// RFC 7296 Section 3.6 states the rule.
// "If multiple certificates are sent, the first certificate MUST contain the public key
// associated with the private key used to sign the AUTH payload."
// The first payload is therefore the peer certificate. Every later one is a link on the
// path toward a trust anchor, and getRemoteCert offers those to x509 as intermediates.
//
// Both IKE_AUTH walks call this. Each one used to assign the peer certificate on every
// CERT payload, so the LAST certificate won. A conformant peer sent its leaf and then the
// issuing intermediate. Ze then verified AUTH against the intermediate.
//
// It is also the ONE place a received chain is bounded. RFC 7296 Section 3.6 requires an
// implementation be "capable of being configured to send and accept up to four X.509
// certificates". Ze had no bound at all on this side. That is not the same property. Ze
// accepted four because it had no limit.
//
// It would have parsed a thousand into an x509.CertPool as readily. certificate-count is
// that bound. Ze reads it from sa.PeerCfg, so the answer is the operator's.
//
// On overflow the message is REFUSED, never truncated. A truncating cap passes any count
// assertion. It also hides from the operator that the limit was reached. And it makes the
// surviving certificates depend on the order an unauthenticated peer chose.
//
// Ze writes nothing to the SA until it accepts the whole chain. A refused message
// therefore leaves RemoteCertRaw and RemoteCertChainRaw empty rather than half-filled
// (ai/rules/fail-closed-guards.md, ai/rules/exact-or-reject.md).
func storeRemoteCerts(sa *SA, certs []*wire.PayloadCERT, log *slog.Logger) error {
	if len(certs) == 0 {
		return nil
	}
	limit := int(sa.PeerCfg.Auth.CertificateChainLimit())
	if len(certs) > limit {
		return fmt.Errorf(
			"ike auth: peer %q sent %d certificate payloads and certificate-count allows %d. "+
				"Raise certificate-count for this peer, or configure the peer to send a shorter chain",
			sa.PeerName, len(certs), limit)
	}

	chain, err := resolveCertPayloads(sa, certs, log)
	if err != nil {
		return err
	}
	if len(chain) > limit {
		return fmt.Errorf(
			"ike auth: peer %q sent a certificate chain of %d after hash-and-url resolution "+
				"and certificate-count allows %d", sa.PeerName, len(chain), limit)
	}
	if len(chain) == 0 {
		return fmt.Errorf("ike auth: peer %q sent certificate payloads carrying no certificate", sa.PeerName)
	}

	sa.RemoteCertRaw = chain[0]
	sa.RemoteCertChainRaw = chain[1:]
	return nil
}

// resolveCertPayloads turns the received CERT payloads into certificate DER, in wire
// order. It resolves the Hash and URL encodings of RFC 7296 Section 3.6 on the way.
//
// The hash-and-url leaf gates the resolution here, and it gates the two collection loops
// as well. The loops decide what ze COLLECTS. This is the single funnel every collected
// payload passes through. A gate that lived only in the loops would be one edit away from
// a fetch an unauthenticated peer names.
func resolveCertPayloads(sa *SA, certs []*wire.PayloadCERT, log *slog.Logger) ([][]byte, error) {
	chain := make([][]byte, 0, len(certs))
	// pending records that a background lookup is running for at least one object.
	//
	// The walk CONTINUES past a miss. Every uncached object in this message then gets a
	// worker from ONE delivery. A return at the first miss starts one worker per delivery.
	// A chain of four needs four retransmissions in that case, and a responder's half-open
	// timeout can expire first.
	pending := false
	for _, c := range certs {
		switch c.CertEncoding {
		case wire.CertEncodingX509Sig:
			chain = append(chain, c.CertData)

		case wire.CertEncodingHashURL, wire.CertEncodingHashURLBundle:
			if !sa.PeerCfg.Auth.HashAndURL {
				return nil, fmt.Errorf(
					"ike auth: peer %q sent a hash-and-url certificate payload and hash-and-url "+
						"is not set for it, so ze performs no lookup. Set hash-and-url to true for "+
						"this peer, or configure the peer to send its certificate inline",
					sa.PeerName)
			}
			// A malformed payload is REFUSED here rather than routed to the fetcher. It
			// can never become cached, so treating it as a pending lookup would make the
			// peer retransmit until the half-open reaper fires.
			hash, _, err := splitHashAndURL(c.CertData)
			if err != nil {
				return nil, fmt.Errorf("ike auth: peer %q hash-and-url payload: %w", sa.PeerName, err)
			}
			// THE LOOKUP DOES NOT RUN ON THIS GOROUTINE. This code path is reached from a
			// half-open handshake, which routeInbound drives inline on the ONE dispatch
			// goroutine that serves every IKE session (register.go). A network fetch here
			// is one unauthenticated peer stopping every other peer's IKE for as long as
			// its chosen server stalls. certURLFetcher moves it to a worker.
			//
			// The cached bytes were stored only after their SHA-1 matched the hash the
			// peer sent, and that comparison ran before any parser saw them. What comes
			// back therefore reaches x509 by the path an inline CERT payload takes.
			der, cached := lookupHashAndURL(hash)
			if !cached {
				certURLFetches.start(c.CertData, sa.PeerCfg.Auth.CertificateURLAllow, log)
				pending = true
				continue
			}
			if c.CertEncoding == wire.CertEncodingHashURL {
				chain = append(chain, der)
				continue
			}
			bundle, err := decodeCertBundle(der)
			if err != nil {
				return nil, fmt.Errorf("ike auth: peer %q hash-and-url bundle: %w", sa.PeerName, err)
			}
			log.Debug("ike: resolved a hash-and-url certificate bundle",
				"peer", sa.PeerName, "certificates", len(bundle))
			chain = append(chain, bundle...)

		default:
			// An encoding ze does not implement is dropped rather than refused. RFC 7296
			// Section 3.6 assigns encodings ze never asked for, and a peer including one
			// beside a usable certificate has not malformed the message.
			log.Debug("ike: ignoring a certificate payload encoding ze does not implement",
				"peer", sa.PeerName, "encoding", c.CertEncoding)
		}
	}
	// A partially resolved chain is never returned. The caller drops the message and the
	// peer sends it again in full. Half a chain has no use. It would also install a peer
	// identity built from the certificates that the cache happened to hold.
	if pending {
		return nil, fmt.Errorf("ike auth: peer %q hash-and-url lookup: %w", sa.PeerName, errCertURLPending)
	}
	return chain, nil
}

func getRemoteCert(sa *SA) (*x509.Certificate, error) {
	if len(sa.RemoteCertRaw) == 0 {
		return nil, fmt.Errorf("ike auth: no remote certificate received in IKE_AUTH")
	}

	cert, err := x509.ParseCertificate(sa.RemoteCertRaw)
	if err != nil {
		return nil, fmt.Errorf("ike auth: parse remote certificate: %w", err)
	}

	// A certificate with no trust anchor authenticates nobody. Ze holds one anchor
	// per peer, ca-certificate. An empty value leaves nothing to chain to, and every
	// self-signed certificate passes.
	//
	// Refuse it here. The caller treats what this function returns as the peer's
	// identity (ai/rules/fail-closed-guards.md). ValidatePKIRefs refuses the same
	// config at verify time. A peer that reaches this error changed its mode on the
	// wire, or was never meant to send one.
	caName := sa.PeerCfg.Auth.CACertificate
	if caName == "" {
		return nil, fmt.Errorf(
			"ike auth: peer %q sent a certificate and no ca-certificate is configured, "+
				"so nothing can validate it", sa.PeerName)
	}
	ca := pki.GetCA(caName)
	if ca == nil {
		return nil, fmt.Errorf("ike auth: CA %q not found in PKI store", caName)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca.Certificate)

	// The peer's own CERT payloads carry the path from its certificate toward the
	// anchor. ca-certificate holds ONE certificate (pki.CACertEntry), so without this
	// pool a leaf signed by an intermediate has no path and a two-level authority
	// authenticates nobody. A supplied intermediate grants no trust on its own: x509
	// still requires a signature chain that ends at the anchor above.
	intermediates := x509.NewCertPool()
	supplied := 0
	for _, der := range sa.RemoteCertChainRaw {
		inter, interErr := x509.ParseCertificate(der)
		if interErr != nil {
			getLogger().Warn("ike: remote sent an unparsable intermediate certificate",
				"peer", sa.PeerName, "error", interErr)
			continue
		}
		intermediates.AddCert(inter)
		supplied++
	}

	// KeyUsages is stated rather than inherited. An empty field means
	// ExtKeyUsageServerAuth in crypto/x509, and an IKE peer is not a TLS server. That
	// default accepts a certificate with no extension, which is what gen-pki.sh mints,
	// and refuses one that names clientAuth alone or strongSwan's id-kp-ipsecIKE from
	// "pki --issue --flag ike". RFC 7296 puts no extended key usage rule on a peer
	// certificate, so ze imposes none. internal/component/pki/store.go states the same
	// value for the same reason.
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:         pool,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		// The count is the actionable part. "signed by unknown authority" with no
		// intermediate offered means the peer must send its chain. The same text with
		// intermediates offered means the chain does not reach ca-certificate
		// (ai/rules/error-messages.md).
		return nil, fmt.Errorf(
			"ike auth: remote certificate validation against ca-certificate %q, "+
				"with %d intermediate certificate(s) supplied by peer %q: %w",
			caName, supplied, sa.PeerName, err)
	}

	// The chain proves the authority issued this certificate. It does not prove the
	// certificate speaks for the peer ze configured. An authority that issues to many
	// clients therefore authenticates every one of them as this peer, because the peer
	// chooses the identity its signature covers. remote-id closes that, and this is the
	// certificate half of it (ai/rules/fail-closed-guards.md).
	asserted, _ := assertedIdentity(sa.RemoteIDPayload)
	want := sa.PeerCfg.Auth.RemoteID
	if want == "" {
		// The guard cannot deny, because the operator stated no expectation. It says so
		// instead. Silence here reads as a check that passed.
		getLogger().Warn(
			"ike: remote-id is not set, so every certificate this authority issued authenticates as this peer",
			"peer", sa.PeerName,
			"ca-certificate", caName,
			"asserted-identity", asserted)
		return cert, nil
	}
	if !certificateCarriesIdentity(cert, sa.RemoteIDPayload, want, sa.PeerCfg.Auth.RemoteIDType) {
		return nil, fmt.Errorf(
			"ike auth: peer %q asserted identity %q, and its certificate carries no such identity. "+
				"Issue a certificate whose subject alternative name, common name or subject is %q, "+
				"or correct remote-id. An opaque ID_KEY_ID corresponds to no certificate field, so "+
				"accepting one needs remote-id-type key-id set for this peer",
			sa.PeerName, asserted, want)
	}

	return cert, nil
}

// localCertChain returns the DER of the certificates ze sends for this SA, leaf first.
//
// RFC 7296 Section 3.6 states the order requirement:
//
// "If multiple certificates are sent, the first certificate MUST contain the public key
// associated with the private key used to sign the AUTH payload."
//
// computeX509Auth signs with entry.PrivateKey. The certificate of that key is entry.Raw.
// Ze therefore appends the leaf first, and it does so unconditionally. The intermediates follow it. The same
// section puts no order on them, so ze keeps the order from the config.
//
// Ze does not truncate the chain here. ValidateCertificateChains refuses at commit a peer
// whose certificate-count is smaller than the chain its PKI entry holds. The operator
// therefore learns the bound is too small. Without that check the operator would watch a
// peer fail to build a path (ai/rules/exact-or-reject.md).
func localCertChain(entry *pki.CertificateEntry) [][]byte {
	chain := make([][]byte, 0, 1+len(entry.RawIntermediates))
	chain = append(chain, entry.Raw)
	chain = append(chain, entry.RawIntermediates...)
	return chain
}

func buildCertPayloads(sa *SA) ([]wire.PayloadEntry, error) {
	certName := sa.PeerCfg.Auth.Certificate
	if certName == "" {
		return nil, nil
	}
	entry := pki.GetCertificate(certName)
	if entry == nil {
		return nil, nil
	}
	chain := localCertChain(entry)

	if sa.PeerCfg.Auth.HashAndURL {
		return buildHashAndURLPayloads(sa, chain)
	}

	payloads := make([]wire.PayloadEntry, 0, len(chain))
	for _, der := range chain {
		payloads = append(payloads, wire.PayloadEntry{
			Payload: &wire.PayloadCERT{
				CertEncoding: wire.CertEncodingX509Sig,
				CertData:     der,
			},
		})
	}
	return payloads, nil
}

// buildHashAndURLPayloads replaces the certificate chain with one Hash and URL CERT
// payload. That payload holds a 20-octet SHA-1 of the replaced structure. The http URL
// that resolves to the structure follows the hash (RFC 7296 Section 3.6).
//
// The chain the operator configures picks which of the two formats goes out. Both are
// reachable. That is what "capable of being configured to send ... the two Hash and URL
// formats" asks for.
//
// Ze sends a lone certificate as encoding 12, "Hash and URL of X.509 certificate", because
// there is no bundle to name. Ze sends a chain as encoding 13, "Hash and URL of X.509
// bundle". One URL then resolves to the whole path. One payload per certificate would
// need one URL for each.
func buildHashAndURLPayloads(sa *SA, chain [][]byte) ([]wire.PayloadEntry, error) {
	url := sa.PeerCfg.Auth.CertificateURL
	if url == "" {
		// parseCertificatePolicy refuses this pairing at commit, so reaching it means
		// the SA was built from a config that never went through the parser.
		return nil, fmt.Errorf(
			"ike auth: peer %q has hash-and-url set and no certificate-url, so ze has no "+
				"URL to publish its certificate at", sa.PeerName)
	}

	encoding := wire.CertEncodingHashURL
	replaced := chain[0]
	if len(chain) > 1 {
		bundle, err := encodeCertBundle(chain)
		if err != nil {
			return nil, fmt.Errorf("ike auth: peer %q: %w", sa.PeerName, err)
		}
		encoding = wire.CertEncodingHashURLBundle
		replaced = bundle
	}

	sum := sha1.Sum(replaced) //nolint:gosec // RFC 7296 Section 3.6 mandates SHA-1 as the hash-and-url object identifier
	data := make([]byte, 0, wire.CertHashURLHashLen+len(url))
	data = append(data, sum[:]...)
	data = append(data, url...)

	return []wire.PayloadEntry{{
		Payload: &wire.PayloadCERT{CertEncoding: encoding, CertData: data},
	}}, nil
}
