// Design: docs/architecture/traffic/fw-7-traffic-vpp.md -- Backend registration

package trafficvpp

import "github.com/ze-software/ze/internal/component/traffic"

func init() {
	if err := traffic.RegisterBackend("vpp", newBackend); err != nil {
		panic("BUG: trafficvpp: registration failed: " + err.Error())
	}
	if err := traffic.RegisterVerifier("vpp", Verify); err != nil {
		panic("BUG: trafficvpp: verifier registration failed: " + err.Error())
	}
}
