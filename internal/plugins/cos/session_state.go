// Design: docs/architecture/traffic/cos-dynamic.md -- per-session CoS state for revert
// Related: handler.go -- cosHandler uses sessionStore for revert

package cos

import "sync"

type sessionCoSState struct {
	accessInterface string
	profileName     string
	staticIngress   map[uint32]uint32
	staticEgress    map[uint32]uint32
}

var sessionStore sync.Map

type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}
