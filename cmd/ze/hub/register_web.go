//go:build ze_web

// Design: ai/rules/plugins.md -- ze_web service registration
package hub

func init() {
	registerService("web", buildWebService, func(lm *ListenerMigrator, svc Service) {
		lm.SetWeb(svc)
		// The same handle also carries the certificate-rotation seam, so a
		// reload that rotates the PKI material reaches the running listener.
		if updatable, ok := svc.(TLSUpdatable); ok {
			lm.SetWebTLS(updatable)
		}
	})
	setWebStandalone(runWebOnly)
}
