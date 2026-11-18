//go:build linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- bridge role dependency.
// Related: original_bridge_linux.go -- positive original channel receipts.

package kernel

import "errors"

// Global declaration executes in the retained original namespace before file
// leases. It never replaces either positive channel46 BRIDGE receipt.
func checkOriginalBridgeSupport(bridge *OriginalChannelBridge) error {
	first, ok := bridge.first.(*claimedCreatedChannel)
	if !ok {
		return errors.New("ppp: native bridge support requires the original first channel claim")
	}
	result := runOriginalPPPNamespace(first.channel.namespace, func() error {
		return checkOriginalAttachmentRequirements(pppSupportSession | pppSupportBridge)
	})
	return result.err()
}
