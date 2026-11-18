// Design: docs/architecture/traffic/cos-dynamic.md -- admitted legacy map users
// Related: shared_access.go -- one original native baseline per map lifetime
//
// The build constraint matches handler.go and session_transaction.go, the
// writers of sessionStore. Without ze_l2tp there are no subscriber sessions,
// no state to keep, and no reachable accessInterface field.

//go:build ze_l2tp

package cos

import (
	"sync"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

// sessionCoSState is a published profile observation backed by its initialized
// shared owner. Its zero value has no installed policy. Display-only tests may
// construct observations, but native mutations require the exact retained owner.
type sessionCoSState struct {
	accessInterface string
	profileName     string
	// One matching notification consumes the token; the completed transaction
	// itself remains compensatable until the next accepted prepare.
	commandToken string
	owner        *sharedCoSAccess
}

var sessionStore sync.Map

type sessionKey struct {
	birth subscriberid.L2TPBirth
}

// sessionDiagnostic retains the original wire labels for reports only. It is
// never an ownership key or native capability. Zero means no labels were supplied.
type sessionDiagnostic struct {
	tunnelID  uint16
	sessionID uint16
}
