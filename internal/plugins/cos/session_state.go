// Design: docs/architecture/traffic/cos-dynamic.md -- per-session CoS state for revert
// Related: handler.go -- cosHandler uses sessionStore for revert
//
// The build constraint is handler.go's. That file is the only writer of
// sessionStore, so without ze_l2tp there are no subscriber sessions, no state
// to keep, and accessInterface is a field nothing in the binary can reach.

//go:build ze_l2tp

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
