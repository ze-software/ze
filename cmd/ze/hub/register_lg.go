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

		lgSvc, ok := svc.(lgService)
		if !ok {
			panic("BUG: the looking-glass migrator hook got a service buildLGService did not build")
		}
		// The named next-hop ranges are replaced on reload whatever each
		// listener serves.
		lm.setLGNextHopRanges(lgSvc)
		// The same handle carries the protocol and certificate seam, so a
		// reload that rotates the PKI material, or changes the tls leaf of a
		// server entry, reaches the running listeners.
		//
		// It is installed whatever the looking glass serves at start. A
		// listener that serves HTTP today, because its entry says `tls false`
		// or because Ze had no certificate to present, serves HTTPS after a
		// reload whose config asks for it, and the reverse
		// (listener_migrate.go, updateLGTransport). A handle withheld from an
		// all-HTTP looking glass would leave that reload accepted and applied
		// to nothing.
		lm.setLGTransport(lgSvc)
	})
}
