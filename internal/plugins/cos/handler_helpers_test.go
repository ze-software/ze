// Design: docs/architecture/traffic/cos-dynamic.md -- test helpers for CoS handler tests

//go:build ze_l2tp

package cos

import (
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

func storeMetadataForTest(tunnelID, sessionID uint16, cosProfile string) {
	l2tp.StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(tunnelID), sessionID), &l2tp.AuthMetadata{
		CoSProfile: cosProfile,
	})
}

func clearMetadataForTest(tunnelID, sessionID uint16) {
	l2tp.ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(tunnelID), sessionID))
}
