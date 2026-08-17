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
	})
}
