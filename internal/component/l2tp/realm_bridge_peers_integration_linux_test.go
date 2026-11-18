//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- fresh post-release realm peers.
package l2tp_test

import (
	"errors"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// Both earlier channels were consumed by NewSession. The rig follows the
// worker's release-before-capture order and never replays either wrapper.
func (rig *realmBridgeRig) bridgeOriginalPeers() error {
	if err := errors.Join(rig.driverOwner.Close(), rig.l2tpOwner.Close()); err != nil {
		return err
	}
	local, err := pppkernel.CreatePPPChannel(rig.l2tpFile)
	if err != nil {
		if local != nil {
			err = errors.Join(err, local.Close())
		}
		return err
	}
	peer, err := pppkernel.CreatePPPChannel(rig.pppoeFile)
	if err != nil {
		if peer != nil {
			err = errors.Join(err, peer.Close())
		}
		return errors.Join(err, local.Close())
	}
	// Constructor consumes both fresh owners on every outcome.
	rig.bridgeOwner, err = pppkernel.NewOriginalChannelBridge(local, peer)
	if err != nil {
		return err
	}
	return rig.bridgeOwner.Bridge()
}
