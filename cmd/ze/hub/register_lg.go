// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
//
// Build-tag-gated registration of the looking-glass service factory. Compiled
// only under //go:build ze_lg; absent the tag this init() does not exist, so
// the hub builds no lg service and the lg package is dropped from the binary.

//go:build ze_lg

package hub

func init() {
	registerService("looking-glass", buildLGService, func(lm *listenerMigrator, svc Service) {
		lm.setLG(svc)

		// The same handle also carries the certificate-rotation seam, so a
		// reload that rotates the PKI material reaches the running listener.
		//
		// A looking glass serving plaintext gets no such handle, and
		// updateLGCertificate then no-ops on the nil one. It holds no
		// certificate to replace, and its server refuses a rotation it can
		// never serve. That refusal would fail the WHOLE reload over a leaf
		// this listener never reads.
		//
		// lgCertificateName (main_reload.go) already reports no name for a
		// config that turns TLS off. This gate covers the config that leaves
		// TLS on and gets plaintext anyway, because the self-signed path found
		// no blob store (service_lg.go).
		lgSvc, ok := svc.(lgService)
		if !ok {
			panic("BUG: the looking-glass migrator hook got a service buildLGService did not build")
		}
		if !lgSvc.ServesTLS() {
			return
		}
		lm.setLGTLS(lgSvc)
	})
}
