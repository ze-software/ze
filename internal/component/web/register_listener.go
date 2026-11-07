// Design: docs/architecture/config/syntax.md -- listener conflict detection at config parse time
// Related: server.go -- listenAddr, the socket code whose behavior this registers

package web

import "github.com/ze-software/ze/internal/component/config"

// listenAddr binds a bracketed address with network "tcp6", and Go then sets
// IPV6_V6ONLY on the socket, so the web server's `::` accepts IPv6 alone and
// `0.0.0.0` on the same port binds beside it. Conflict validation must know it,
// or it refuses that pair as a dual-stack overlap.
func init() {
	config.RegisterListenerV6Only("web")
}
