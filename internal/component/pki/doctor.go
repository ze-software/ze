// Design: docs/architecture/pki/pki-store.md -- the readiness check over the
// local certificate authority root
// Related: ca.go -- LoadOrGenerateRoot, which writes the pair this check reads
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// The root is a runtime dependency of every internal listener that has no
// operator-named certificate, so the pki component owns its doctor check
// (ai/rules/repo-maintenance.md, ownership). `ze doctor` runs as a separate
// process from the daemon, so this check reads the two stored keys rather than
// the root the daemon loaded in memory.

package pki

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	// codeCARootMissing says no root is stored. It is distinct from the expiry
	// code because the operator answer differs: a missing root is regenerated at
	// the next start and then redistributed, where an expiring one is still
	// serving.
	codeCARootMissing = "doctor-pki-ca-root-missing"

	// codeCARootExpiry says the stored root is near its NotAfter, or past it.
	codeCARootExpiry = "doctor-pki-ca-root-expiry"

	// codeCARootInvalid is what every Ze surface already reports for certificate
	// material that will not load: internal/core/dnsserver/certcheck.go for a
	// file pair, and the as112 listener check for a store entry. A stored root
	// that is not PEM, is not a CA, or does not match its key is the same
	// finding, so it takes the same code rather than a third spelling of one
	// concept (ai/rules/writing.md, habit 1).
	codeCARootInvalid = "doctor-tls-invalid"
)

// caRootExpiryWarnWindow is how far ahead of NotAfter the root is reported.
//
// It is 90 days where certExpiryWarnWindow is 30. A configured certificate is
// replaced on the router that serves it, so a month of notice is enough. The
// root has to be replaced on every peer that trusts it, by hand, because Ze
// distributes it manually and holds no revocation. Ninety days is the runway
// that visit needs.
const caRootExpiryWarnWindow = 90 * 24 * time.Hour

// caRootNow is the clock this check reads. It is a var so the expiry boundary
// can be driven in a test without minting a certificate the production path
// would never issue. It is not a config surface.
var caRootNow = time.Now

// caRootDoctorCheck is the registration installed from register.go. It reads the
// store and no config, so it runs before the config is loaded: a daemon whose
// config is broken still owes the operator the state of its certificate
// authority.
var caRootDoctorCheck = diagnostic.DoctorCheck{
	Name:         "pki-ca-root",
	Phase:        diagnostic.DoctorPhasePreConfig,
	Order:        731,
	Component:    "pki",
	Dependencies: []string{"storage"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeCARootMissing, codeCARootExpiry, codeCARootInvalid},
	Check:        checkCARoot,
}

// checkCARoot reports the state of the stored root: absent, unloadable, or near
// expiry. A root that loads and has more than caRootExpiryWarnWindow left
// reports nothing.
func checkCARoot(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Store == nil {
		return []diagnostic.Diagnostic{{
			Code:     codeCARootInvalid,
			Severity: diagnostic.SeverityError,
			Message:  "cannot read the local certificate authority root: this doctor run resolved no storage",
			Help:     "check the storage diagnostics reported above this one",
		}}
	}

	certKey := zefs.KeyCACert.Pattern
	keyKey := zefs.KeyCAKey.Pattern
	certStored := ctx.Store.Exists(certKey)
	keyStored := ctx.Store.Exists(keyKey)

	if !certStored && !keyStored {
		return []diagnostic.Diagnostic{{
			Code:     codeCARootMissing,
			Severity: diagnostic.SeverityWarning,
			Message:  "no local certificate authority root is stored",
			Help:     "the daemon generates one at its next start. Every copy of the previous root stops working, so export the new one with `show pki local-ca pem` and give it to each client that trusts this node",
		}}
	}
	if !certStored || !keyStored {
		return []diagnostic.Diagnostic{caRootHalfWritten(certStored)}
	}

	root, err := loadRoot(ctx.Store, certKey, keyKey)
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codeCARootInvalid,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("the stored local certificate authority root does not load: ").Err(err).String(),
			Help:     "the daemon refuses to start on this root. Remove both stored halves so the next start generates a root, then redistribute it",
		}}
	}

	return caRootExpiry(root.Certificate().NotAfter)
}

// caRootHalfWritten answers a store holding one half of the pair. The daemon
// treats it as no root at all and generates a replacement, so the operator is
// told which half survived before that happens.
func caRootHalfWritten(certStored bool) diagnostic.Diagnostic {
	// The two halves the root is stored as, named for a reader rather than by
	// their zefs key.
	const (
		halfCert = "certificate"
		halfKey  = "private key"
	)

	present, absent := halfCert, halfKey
	if !certStored {
		present, absent = halfKey, halfCert
	}
	var tb textbuf.Buffer
	return diagnostic.Diagnostic{
		Code:     codeCARootInvalid,
		Severity: diagnostic.SeverityError,
		Message: tb.Str("the stored local certificate authority root holds a ").Str(present).
			Str(" and no ").Str(absent).String(),
		Help: "the next start generates a new root and replaces the surviving half, so every distributed copy of the old root stops working",
	}
}

// caRootExpiry reports the root as expired, as near expiry, or not at all.
func caRootExpiry(notAfter time.Time) []diagnostic.Diagnostic {
	now := caRootNow()
	remaining := notAfter.Sub(now)

	if remaining <= 0 {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codeCARootExpiry,
			Severity: diagnostic.SeverityError,
			Message: tb.Str("the local certificate authority root expired on ").
				Str(notAfter.UTC().Format(time.RFC3339)).String(),
			Help: "every leaf it issued is refused. Remove both stored halves so the next start generates a root, then redistribute it",
		}}
	}
	if remaining >= caRootExpiryWarnWindow {
		return nil
	}

	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     codeCARootExpiry,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("the local certificate authority root expires in ").
			Int(int64(daysUntil(now, notAfter))).Str(" days").String(),
		Help: "each client trusting this node needs the replacement root before then, so plan the visit now",
	}}
}
