// Design: plan/spec-cos-dynamic.md -- test helpers for CoS handler tests

//go:build ze_l2tp

package cos

import "github.com/ze-software/ze/internal/component/l2tp"

func storeMetadataForTest(tunnelID, sessionID uint16, cosProfile string) {
	l2tp.StoreSessionMetadata(tunnelID, sessionID, &l2tp.AuthMetadata{
		CoSProfile: cosProfile,
	})
}

func clearMetadataForTest(tunnelID, sessionID uint16) {
	l2tp.ClearSessionMetadata(tunnelID, sessionID)
}
