// Design: docs/architecture/pki/tls-listeners.md -- doctor check for environment.looking-glass.certificate
// Related: server.go -- the listener whose TLS material this check validates

package lg

import (
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkLGTLSCertificate validates that environment.looking-glass.certificate
// names a PKI store entry that can actually serve TLS, reading the pki block of
// the SAME tree so the answer is available before the config is committed.
func checkLGTLSCertificate(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*zeconfig.Tree)
	if !ok {
		return nil
	}
	return lgTLSDiagnostic(tree, time.Now())
}

// lgTLSDiagnostic is the pure decision function. No reference configured means
// the self-signed certificate serves, which is not a problem to report.
//
// Settings, not addresses: the check reads ExtractLGSettings, which returns on
// the PRESENCE of the block, rather than ExtractLGConfig, which gates on
// `enabled true`. ze.looking-glass.listen and ze.looking-glass.enabled both
// start the listener with no `enabled` leaf in the block, and that listener
// would serve this certificate. Gating on the leaf would stay silent for
// exactly those deployments.
func lgTLSDiagnostic(tree *zeconfig.Tree, now time.Time) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	cfg, ok := zeconfig.ExtractLGSettings(tree)
	if !ok || cfg.Certificate == "" {
		return nil
	}

	// A pki block that does not parse is reported by the pki component's own
	// startup path. Leaving pkiCfg nil here makes CheckCertReference say the
	// name resolves to nothing, which is the operator-visible consequence.
	pkiCfg, _ := pki.ParseConfig(tree)

	problems := pki.CheckCertReference(pkiCfg, cfg.Certificate, now)
	out := make([]diagnostic.Diagnostic, 0, len(problems))
	var tb textbuf.Buffer
	for _, p := range problems {
		tb.Reset()
		out = append(out, diagnostic.Diagnostic{
			Code:     p.Code,
			Severity: diagnostic.Severity(p.Severity),
			Message:  tb.Str("environment.looking-glass.certificate: ").Str(p.Message).String(),
		})
	}
	return out
}
