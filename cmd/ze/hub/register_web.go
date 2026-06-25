//go:build ze_web

// Design: ai/rules/feature-gate-registration.md -- ze_web service registration
package hub

func init() {
	registerService("web", buildWebService, func(lm *ListenerMigrator, svc Service) {
		lm.SetWeb(svc)
	})
	setWebStandalone(runWebOnly)
}
