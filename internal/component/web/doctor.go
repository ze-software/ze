// Design: plan/spec-pki-full-chain.md -- doctor check for environment.web.certificate
// Related: server.go -- the listener whose TLS material this check validates

package web

import (
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkWebTLSCertificate validates that environment.web.certificate names a PKI
// store entry that can actually serve TLS, reading the pki block of the SAME
// tree so the answer is available before the config is committed.
func checkWebTLSCertificate(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*zeconfig.Tree)
	if !ok {
		return nil
	}
	return webTLSDiagnostic(tree, time.Now())
}

// webTLSDiagnostic is the pure decision function. No reference configured means
// the self-signed certificate serves, which is not a problem to report.
//
// Settings, not addresses: the check does NOT require the block to say
// `enabled true`, because --web, ze.web.listen, and ze.web.enabled all start the
// listener without it, and each of those would still serve this certificate
// (plan/learned/1327-enabled-gate-discards-service-settings.md).
func webTLSDiagnostic(tree *zeconfig.Tree, now time.Time) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	cfg, ok := zeconfig.ExtractWebSettings(tree)
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
			Message:  tb.Str("environment.web.certificate: ").Str(p.Message).String(),
		})
	}
	return out
}
