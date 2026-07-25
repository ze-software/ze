// Design: ai/rules/feature-gate-registration.md -- ze_ike hub registration gating
//
// IKE engine + IPsec config-plumbing registration for the hub composition
// root, gated on ze_ike. These were side-effect blank imports in main.go; the
// hub never calls an ike symbol directly, so moving them here is the whole
// hub-side gate. The other roots are the generated all_ze_ike.go group file
// (schema + command surface). internal/component/ike/dataplane stays always-on
// as the shared XFRM seam OSPF also programs through.

//go:build ze_ike

package hub

import (
	_ "github.com/ze-software/ze/internal/component/ike/engine"
	_ "github.com/ze-software/ze/internal/component/ike/ipsec"
)
