// Design: docs/architecture/fleet-config.md -- reporting a hub session that cannot open
// Design: docs/architecture/doctor-and-health-checks.md -- hub-session runtime check
// Related: client.go -- runConnection classifies and reports each failed attempt here
// Related: doctor.go -- checkHubSession renders the last failure for doctor

package managed

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// sessionFailureErrorMax bounds the error text kept on one HubSessionFailure.
const sessionFailureErrorMax = 1024

// HubSessionStep identifies where an attempt to open the hub session failed.
// The zero value means no failure is recorded.
type HubSessionStep uint8

const (
	// HubSessionStepNone is the zero value: no failed attempt is recorded.
	HubSessionStepNone HubSessionStep = iota
	// HubSessionStepAnchor means building the TLS trust anchor failed before dialing.
	HubSessionStepAnchor
	// HubSessionStepConnect means dialing the hub's TCP address failed.
	HubSessionStepConnect
	// HubSessionStepHandshake means the TLS handshake did not complete.
	HubSessionStepHandshake
	// HubSessionStepAuth means the hub refused authentication after TLS completed.
	HubSessionStepAuth
)

// HubSessionCause refines why an attempt to open the hub session failed.
type HubSessionCause uint8

const (
	// HubSessionCauseNone is the zero value.
	HubSessionCauseNone HubSessionCause = iota
	// HubSessionCauseAnchorUndefined means the configured pki ca name does not exist.
	HubSessionCauseAnchorUndefined
	// HubSessionCauseUnknownAuthority means the trust anchor did not issue the hub certificate.
	HubSessionCauseUnknownAuthority
	// HubSessionCauseHostnameMismatch means the hub certificate does not carry the dialed host.
	HubSessionCauseHostnameMismatch
	// HubSessionCauseValidity means the hub certificate is outside its validity dates.
	HubSessionCauseValidity
	// HubSessionCauseCertOther means certificate verification failed for another reason.
	HubSessionCauseCertOther
	// HubSessionCauseHandshakeOther means the TLS handshake failed without a verification error.
	HubSessionCauseHandshakeOther
	// HubSessionCauseAuthRejected means the hub answered authentication with anything other than ok.
	HubSessionCauseAuthRejected
)

// HubTrustAnchor names which trust anchor clientTLSConfig selected for the attempt.
type HubTrustAnchor uint8

const (
	// HubTrustAnchorUnspecified is the zero value.
	HubTrustAnchorUnspecified HubTrustAnchor = iota
	// HubTrustAnchorPKICA is the named pki ca block in the device's configuration.
	HubTrustAnchorPKICA
	// HubTrustAnchorStoredRoot is the root stored by ze init or trust-on-first-use.
	HubTrustAnchorStoredRoot
	// HubTrustAnchorInsecure means TLS verification was switched off.
	HubTrustAnchorInsecure
	// HubTrustAnchorSystem is the operating system trust store.
	HubTrustAnchorSystem
)

// HubSessionFailure is the last attempt of the running hub client that failed
// before its session opened. Live only, never persisted, copied by value.
type HubSessionFailure struct {
	Time          time.Time
	Server        string
	Step          HubSessionStep
	Cause         HubSessionCause
	Anchor        HubTrustAnchor
	CAName        string
	Error         string
	CertNames     []string
	CertNotBefore time.Time
	CertNotAfter  time.Time
}

// clone returns an independent copy of f so callers cannot alias CertNames.
func (f HubSessionFailure) clone() HubSessionFailure {
	f.CertNames = slices.Clone(f.CertNames)
	return f
}

// sessionAttemptError carries the classified failure alongside the raw error
// returned by runConnection so RunManagedClient can log its doctor code and fix.
type sessionAttemptError struct {
	failure HubSessionFailure
	err     error
}

func (e sessionAttemptError) Error() string { return e.err.Error() }
func (e sessionAttemptError) Unwrap() error { return e.err }

// clientTrustAnchor returns the trust anchor cfg asks clientTLSConfig to use,
// in the exact precedence order of clientTLSConfig.
func clientTrustAnchor(cfg *ClientConfig) (HubTrustAnchor, string) {
	if cfg.CA != "" {
		return HubTrustAnchorPKICA, cfg.CA
	}
	if cfg.HubRoot != nil {
		return HubTrustAnchorStoredRoot, ""
	}
	if cfg.TLSInsecure {
		return HubTrustAnchorInsecure, ""
	}
	return HubTrustAnchorSystem, ""
}

// boundSessionError cuts text to at most sessionFailureErrorMax bytes on a
// UTF-8 rune boundary.
func boundSessionError(text string) string {
	if len(text) <= sessionFailureErrorMax {
		return text
	}
	cut := sessionFailureErrorMax
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// classifySessionFailure turns a pre-authentication failure from runConnection
// into a typed HubSessionFailure.
func classifySessionFailure(cfg *ClientConfig, step HubSessionStep, err error, now time.Time) HubSessionFailure {
	anchor, caName := clientTrustAnchor(cfg)
	failure := HubSessionFailure{
		Time:   now,
		Server: cfg.Server,
		Step:   step,
		Anchor: anchor,
		CAName: caName,
		Error:  boundSessionError(err.Error()),
	}
	switch step {
	case HubSessionStepNone, HubSessionStepConnect:
		// Connect failures are owned by doctor-hub-unreachable; leave Cause unset.
	case HubSessionStepAnchor:
		if errors.Is(err, errCANotConfigured) {
			failure.Cause = HubSessionCauseAnchorUndefined
		} else {
			failure.Cause = HubSessionCauseCertOther
		}
	case HubSessionStepHandshake:
		failure.Cause, failure.CertNames, failure.CertNotBefore, failure.CertNotAfter = classifyHandshakeError(err)
	case HubSessionStepAuth:
		if errors.Is(err, errAuthRejected) {
			failure.Cause = HubSessionCauseAuthRejected
		} else {
			failure.Cause = HubSessionCauseHandshakeOther
		}
	}
	return failure
}

// classifyHandshakeError distinguishes certificate verification failures by
// their crypto/x509 and crypto/tls error types and extracts the refused
// certificate's names and validity dates when present.
func classifyHandshakeError(err error) (HubSessionCause, []string, time.Time, time.Time) {
	var cert *x509.Certificate
	if verErr, ok := errors.AsType[*tls.CertificateVerificationError](err); ok && len(verErr.UnverifiedCertificates) > 0 {
		cert = verErr.UnverifiedCertificates[0]
	}

	if hostErr, ok := errors.AsType[x509.HostnameError](err); ok {
		if hostErr.Certificate != nil {
			cert = hostErr.Certificate
		}
		names, nb, na := certSummary(cert)
		return HubSessionCauseHostnameMismatch, names, nb, na
	}
	if invalidErr, ok := errors.AsType[x509.CertificateInvalidError](err); ok {
		if invalidErr.Cert != nil {
			cert = invalidErr.Cert
		}
		names, nb, na := certSummary(cert)
		if invalidErr.Reason == x509.Expired {
			return HubSessionCauseValidity, names, nb, na
		}
		return HubSessionCauseCertOther, names, nb, na
	}
	if authErr, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		if authErr.Cert != nil {
			cert = authErr.Cert
		}
		names, nb, na := certSummary(cert)
		return HubSessionCauseUnknownAuthority, names, nb, na
	}
	if _, ok := errors.AsType[x509.SystemRootsError](err); ok {
		names, nb, na := certSummary(cert)
		return HubSessionCauseUnknownAuthority, names, nb, na
	}
	if _, ok := errors.AsType[x509.ConstraintViolationError](err); ok {
		names, nb, na := certSummary(cert)
		return HubSessionCauseCertOther, names, nb, na
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		names, nb, na := certSummary(cert)
		return HubSessionCauseCertOther, names, nb, na
	}
	return HubSessionCauseHandshakeOther, nil, time.Time{}, time.Time{}
}

// certSummary extracts the SAN hostnames and IP addresses plus validity dates
// from cert.
func certSummary(cert *x509.Certificate) ([]string, time.Time, time.Time) {
	if cert == nil {
		return nil, time.Time{}, time.Time{}
	}
	names := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses))
	names = append(names, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}
	return names, cert.NotBefore, cert.NotAfter
}

const sessionImpactClause = "so this device holds no hub session, receives no configuration change and keeps retrying while it keeps running the configuration it has"

// anchorDescription describes the trust anchor used by f.
func (f HubSessionFailure) anchorDescription() string {
	switch f.Anchor {
	case HubTrustAnchorPKICA:
		return "pki ca " + f.CAName
	case HubTrustAnchorStoredRoot:
		return "the root ze init stored"
	case HubTrustAnchorInsecure:
		return "verification switched off"
	case HubTrustAnchorSystem:
		return "the system trust store"
	case HubTrustAnchorUnspecified:
		return "its trust anchor"
	default:
		return "its trust anchor"
	}
}

// unknownIssuerFix returns the operator fix tailored to f's trust anchor,
// always naming the hub-side option too.
func (f HubSessionFailure) unknownIssuerFix() string {
	const hubSide = "or have the hub present a certificate this anchor verifies (bus hub server <block> certificate)"
	switch f.Anchor {
	case HubTrustAnchorPKICA:
		return "correct pki ca " + f.CAName + " in this device's configuration on the hub, then run ze init --managed --force --hub-ca <file>, " + hubSide
	case HubTrustAnchorStoredRoot:
		return "ze init --managed --force --hub-ca <file> (holding the root from ze show pki local-ca pem on the hub), " + hubSide
	case HubTrustAnchorSystem, HubTrustAnchorInsecure, HubTrustAnchorUnspecified:
		return "name a ca in this device's configuration on the hub, then run ze init --managed --force --hub-ca <file>, " + hubSide
	default:
		return "name a ca in this device's configuration on the hub, then run ze init --managed --force --hub-ca <file>, " + hubSide
	}
}

// Diagnostic renders f as a doctor Diagnostic for the hub-session runtime check.
// It returns false for HubSessionStepNone and HubSessionStepConnect (which
// doctor-hub-unreachable owns).
func (f HubSessionFailure) Diagnostic(now time.Time) (diagnostic.Diagnostic, bool) {
	if f.Step == HubSessionStepNone {
		return diagnostic.Diagnostic{}, false
	}
	if f.Step == HubSessionStepConnect {
		return diagnostic.Diagnostic{}, false
	}

	server := placeholderOr(f.Server, "<host:port>")
	switch f.Cause {
	case HubSessionCauseNone:
		return diagnostic.Diagnostic{}, false

	case HubSessionCauseAnchorUndefined:
		caName := placeholderOr(f.CAName, "<name>")
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubCertificateUnverified,
			Severity: diagnostic.SeverityWarning,
			Message: "this device did not dial its hub at " + server + " because its configuration names pki ca " + caName +
				", which pki does not define, " + sessionImpactClause,
			Fix: "define pki ca " + caName + " in this device's configuration on the hub with the root that issued the hub's certificate, " +
				"then run ze init --managed --force --hub-ca <file> on this device so that it fetches it",
		}, true

	case HubSessionCauseUnknownAuthority:
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubCertificateUnverified,
			Severity: diagnostic.SeverityWarning,
			Message: "this device refused the certificate its hub at " + server + " presents because " + f.anchorDescription() +
				" did not issue it (" + f.Error + "), " + sessionImpactClause,
			Fix: f.unknownIssuerFix(),
		}, true

	case HubSessionCauseHostnameMismatch:
		host := serverNameFromAddr(server)
		carried := "none"
		if len(f.CertNames) > 0 {
			carried = strings.Join(f.CertNames, ", ")
		}
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubCertificateUnverified,
			Severity: diagnostic.SeverityWarning,
			Message: "this device refused the certificate its hub at " + server + " presents because it does not carry the dialed host " +
				host + " (certificate names: " + carried + "), " + sessionImpactClause,
			Fix: "on the hub, give the serving block the ip devices dial (a block on 0.0.0.0 puts no host in the leaf ze issues) " +
				"or name a certificate that carries " + host + ", or correct host in this device's bus hub client block",
		}, true

	case HubSessionCauseValidity:
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubCertificateUnverified,
			Severity: diagnostic.SeverityWarning,
			Message: "this device refused the certificate its hub at " + server + " presents because it is outside its validity dates " +
				"(valid from " + f.CertNotBefore.UTC().Format(time.RFC3339) + " to " + f.CertNotAfter.UTC().Format(time.RFC3339) +
				", device time " + now.UTC().Format(time.RFC3339) + "), " + sessionImpactClause,
			Fix: "renew pki certificate <name> on the hub and commit (the next handshake presents it), or correct this device's clock",
		}, true

	case HubSessionCauseCertOther:
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubCertificateUnverified,
			Severity: diagnostic.SeverityWarning,
			Message: "this device refused the certificate its hub at " + server + " presents against " + f.anchorDescription() +
				" (" + f.Error + "), " + sessionImpactClause,
			Fix: f.unknownIssuerFix(),
		}, true

	case HubSessionCauseHandshakeOther:
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubHandshakeFailed,
			Severity: diagnostic.SeverityWarning,
			Message:  "the TLS handshake with the hub at " + server + " failed (" + f.Error + "), " + sessionImpactClause,
			Fix: "ze show doctor on the hub at " + server + " (check for a saturated handshake cap or a listener that did not bind); " +
				"correct host and port when that address is not a managed listener",
		}, true

	case HubSessionCauseAuthRejected:
		return diagnostic.Diagnostic{
			Code:     CodeDoctorHubAuthRejected,
			Severity: diagnostic.SeverityWarning,
			Message: "the hub at " + server + " refused this device's authentication (unknown name, wrong secret, or name already connected), " +
				sessionImpactClause,
			Fix: "ze show doctor on the hub at " + server + " to read doctor-managed-client-rejected, " +
				"then correct the client entry on the hub or this device's secret",
		}, true
	}

	return diagnostic.Diagnostic{}, false
}

// ReconnectMeta returns the doctor code and fix to attach to the client's
// reconnect Warn log line.
func (f HubSessionFailure) ReconnectMeta(now time.Time) (code, fix string) {
	if diag, ok := f.Diagnostic(now); ok {
		return diag.Code, diag.Fix
	}
	if f.Step == HubSessionStepConnect {
		server := placeholderOr(f.Server, "<host:port>")
		return codeHubUnreachable, "start the hub at " + server + ", or correct the host and port of this device's bus hub client block"
	}
	return codeHubUnreachable, "ze doctor"
}
