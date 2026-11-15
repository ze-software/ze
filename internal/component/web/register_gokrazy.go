// Design: ai/rules/plugins.md -- gokrazy portal web route self-registration
// Related: webroute.go -- WebRoute.Enabled/Portal fields this route uses
//
// The gokrazy appliance UI is embedded via an iframe portal. Unlike the L2TP/
// IS-IS/OSPF views it is gated on ze.gokrazy.enabled and carries a portal menu
// entry, so it uses the WebRoute.Enabled and Portal fields. The route is only
// wired when the socket-backed gokrazy service is enabled.
//
// The proxy carries gokrazy's update credential, so every request it forwards
// is first judged by the session's Ze authorization, the same check the edit
// gate makes (authorizeRequest): the proxy's gate names the Ze command each
// gokrazy page needs (internal/component/gokrazy/gate.go).

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
		Build: func(deps RouteDeps) http.Handler {
			authorize := func(r *http.Request, command string, isReadOnly bool) bool {
				return authorizeRequest(r, deps.Authorizer, command, isReadOnly)
			}
			return zegokrazy.Handler(env.Get("ze.gokrazy.socket"), authorize)
		},
		Portal: &PortalService{
			Key: "gokrazy", Title: "Gokrazy", Path: "/gokrazy/",
			Icon: "/gokrazy/assets/gokrazy-logo.svg",
		},
	})
}
