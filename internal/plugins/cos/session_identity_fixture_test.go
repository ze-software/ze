// Design: docs/architecture/traffic/cos-dynamic.md -- explicit synthetic fixture births
//go:build ze_l2tp

package cos

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

// Existing synthetic fixtures each declare one original owner by these labels.
// This deterministic fixture constructor is not used by production admission.
// Distinct-birth/equal-label regressions provide their explicit values directly.
func fixtureCoSKey(tunnelID, sessionID uint16) sessionKey {
	birth, err := subscriberid.ParseL2TPBirth(fmt.Sprintf("l2tp-000000000000000100000000%04x%04x", tunnelID, sessionID))
	if err != nil {
		panic(err)
	}
	return sessionKey{birth: birth}
}

func fixtureCoSID(tunnelID, sessionID uint16) string {
	return fixtureCoSKey(tunnelID, sessionID).birth.String()
}
