// Design: docs/architecture/pki/tls-listeners.md -- doctor check for environment.web.certificate
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
// listener without it, and each of those would still serve this certificate.
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
		tb.Str("environment.web.certificate: ").Str(p.Message)
		// An expired or soon-expiring certificate still serves: renewing it and
		// committing installs it without a restart.
		fix := "replace the material with `edit pki certificate " + cfg.Certificate + "`, then commit: the web server presents it at the next handshake"
		// A reference that cannot serve does not stop ze: the daemon starts
		// without the web server, so that the operator can still reach it and
		// correct the name (docs/architecture/core-design.md, "Design
		// decisions"). The finding says so, with the fix.
		if p.Code == diagnostic.CodeDoctorTLSReference {
			tb.Str(". ").Str(webReferenceEffect)
			fix = webReferenceFix(cfg.Certificate)
		}
		diag := diagnostic.Diagnostic{Code: p.Code, Severity: diagnostic.Severity(p.Severity), Message: tb.String(), Fix: fix}
		out = append(out, diag)
	}
	return out
}

// webReferenceEffect is what ze does with a web certificate reference that
// cannot serve TLS.
const webReferenceEffect = "ze starts without the web server and keeps its other services running, and a reload that names this certificate is refused"

// webReferenceFix is the fix of a web certificate reference that cannot serve
// TLS, naming the certificate the operator configured.
func webReferenceFix(name string) string {
	var tb textbuf.Buffer
	return tb.Str("Define it with `edit pki certificate ").Str(name).
		Str("`, or delete `environment web certificate`, then restart ze.").String()
}
