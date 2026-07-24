// Design: ai/rules/feature-gate-registration.md -- ze_ike-off VPN page stub
// Related: page_vpn_ipsec.go -- the real IKE-backed page (ze_ike builds)

//go:build !ze_ike

package web

import (
	"html/template"
	"net/http"
)

// renderVPNPageContent is the ze_ike-off counterpart of the IKE-backed VPN
// page: the workbench "vpn" route stays valid in every ze_web build, but a
// build without the IKE engine states plainly that IPsec is not included
// instead of importing internal/component/ike/engine (which would pin the
// whole IKE subtree into the binary).
func renderVPNPageContent(_ *Renderer, _ *http.Request, _ []string) (template.HTML, bool) {
	return template.HTML(`<div class="workbench-empty">IKEv2/IPsec is not included in this build (ze_ike off).</div>`), true
}
