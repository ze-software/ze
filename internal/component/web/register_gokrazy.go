// Design: ai/rules/plugins.md -- gokrazy portal web route self-registration
// Related: webroute.go -- WebRoute.Enabled/Portal fields this route uses
//
// The gokrazy appliance UI is embedded via an iframe portal. Unlike the L2TP/
// IS-IS/OSPF views it is gated on ze.gokrazy.enabled and carries a portal menu
// entry, so it uses the WebRoute.Enabled and Portal fields. The route is only
// wired when the socket-backed gokrazy service is enabled.

package web

import (
	"net/http"

	zegokrazy "github.com/ze-software/ze/internal/component/gokrazy"
	"github.com/ze-software/ze/internal/core/env"
)

func init() {
	RegisterWebRoute(WebRoute{
		Pattern: "/gokrazy/",
		Wrap:    WrapAuth,
		Enabled: func() bool { return env.IsEnabled("ze.gokrazy.enabled") },
		Build: func(RouteDeps) http.Handler {
			return zegokrazy.Handler(env.Get("ze.gokrazy.socket"))
		},
		Portal: &PortalService{
			Key: "gokrazy", Title: "Gokrazy", Path: "/gokrazy/",
			Icon: "/gokrazy/assets/gokrazy-logo.svg",
		},
	})
}
