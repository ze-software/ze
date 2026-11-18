// Design: docs/architecture/l2tp/followup-l2tp-call.md -- exact bridge cleanup receipt.
package l2tp

// kernelPPPBridgePending is true only while the exact handle still owns its
// two originals. It never interprets a recyclable numeric label as ownership.
func kernelPPPBridgePending(owners *kernelPPPOwners) bool {
	if owners == nil {
		return false
	}
	if owners.bridge == nil {
		return false
	}
	return owners.bridge.ClosePending()
}
