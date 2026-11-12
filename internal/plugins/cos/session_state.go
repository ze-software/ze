// Design: docs/architecture/traffic/cos-dynamic.md -- per-session CoS state for revert
// Related: handler.go -- cosHandler uses sessionStore for revert
//
// The build constraint matches handler.go and session_transaction.go, the
// writers of sessionStore. Without ze_l2tp there are no subscriber sessions,
// no state to keep, and no reachable accessInterface field.

//go:build ze_l2tp

package cos

import "sync"

type sessionCoSState struct {
	accessInterface string
	profileName     string
	// commandToken identifies the successful owner apply that wrote this
	// profile. One matching notification consumes it without a second write.
	commandToken  string
	staticIngress map[uint32]uint32
	staticEgress  map[uint32]uint32
}

var sessionStore sync.Map

type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}
