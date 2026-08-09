// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE engine readiness checks
// RFC: rfc/short/rfc7296.md -- Hash and URL certificate lookup (Section 3.6)
// Related: doctor.go -- the sibling ike readiness checks and their registration
// Related: certurl.go -- the fetcher whose bound this check reports against

package engine

import (
	"net/netip"
	"net/url"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// checkIPsecCertURL reports a peer whose hash-and-url configuration cannot work.
//
// Hash and URL is a RUNTIME NETWORK DEPENDENCY. With it enabled ze publishes its own
// certificate at an operator-named http URL and expects the peer to fetch it. Ze fetches
// the peer's certificate in turn.
//
// ai/rules/repo-maintenance.md requires a readiness check for a dependency of that shape.
// The failure is otherwise invisible from ze's side. The peer refuses ze's certificate,
// and the reason lives in the peer's log.
//
// It is a readiness check rather than a config-verify rejection, for the same reason
// checkIPsecInterface is. Reachability is a property of the HOST and its position in the
// network. It is not a property of the config being judged. The two structural errors
// that ARE properties of the config alone (hash-and-url with no certificate-url, and a
// malformed prefix in certificate-url-allow) are refused at commit by
// parseCertificatePolicy instead.
func checkIPsecCertURL(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil || cfg == nil {
		// checkIPsecInterface already reports an unparseable vpn ipsec section, with
		// the same tree and in the same phase. Reporting it twice would make one
		// config error read as two unrelated faults.
		return nil
	}

	var out []rpc.DoctorCheckDiagnostic
	for name := range cfg.Peers {
		peer := cfg.Peers[name]
		if !peer.Auth.HashAndURL {
			continue
		}
		out = append(out, certURLDiagnostics(name, peer.Auth)...)
	}
	return out
}

// certURLDiagnostics reports what is wrong with one peer's certificate-url.
func certURLDiagnostics(peerName string, auth ipsec.AuthConfig) []rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer

	u, err := url.Parse(auth.CertificateURL)
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-cert-url",
			Severity: "error",
			Message: tb.Str("ipsec peer ").Str(peerName).
				Str(" certificate-url is not a URL: ").Err(err).String(),
		}}
	}
	if u.Scheme != "http" {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-cert-url",
			Severity: "error",
			Message: tb.Str("ipsec peer ").Str(peerName).
				Str(" certificate-url uses the ").Str(u.Scheme).
				Str(" scheme, and RFC 7296 Section 3.6 makes http the scheme an implementation " +
					"must support for hash-and-url lookup. Ze refuses every other scheme").String(),
		}}
	}
	host := u.Hostname()
	if host == "" {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-cert-url",
			Severity: "error",
			Message: tb.Str("ipsec peer ").Str(peerName).
				Str(" certificate-url names no host, so no peer can fetch the certificate").String(),
		}}
	}

	// A literal address is judged against the same deny list the fetcher applies. A NAME
	// is not resolved here. Doctor must not make a network call on the operator's behalf.
	// And a name that resolves differently later is exactly what the fetcher re-checks at
	// dial time.
	addr, addrErr := netip.ParseAddr(host)
	if addrErr != nil {
		return nil
	}
	if allowedByOperator(addr, auth.CertificateURLAllow) {
		return nil
	}
	if certURLDenied(addr) {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-cert-url-denied",
			Severity: "warning",
			Message: tb.Str("ipsec peer ").Str(peerName).
				Str(" certificate-url names ").Addr(addr).
				Str(", which the hash-and-url fetcher refuses. The URL must be reachable by the " +
					"PEER, so a loopback or private address is usually a mistake. Name the prefix " +
					"in certificate-url-allow when it is deliberate").String(),
		}}
	}
	return nil
}

// allowedByOperator reports whether an address falls inside a configured allowance. It is
// the same widening the fetcher's dial control applies, read from the same leaf. Doctor
// and the fetcher therefore cannot disagree about what is permitted.
func allowedByOperator(addr netip.Addr, allow []netip.Prefix) bool {
	for _, p := range allow {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}
