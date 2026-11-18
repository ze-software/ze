// Design: docs/architecture/l2tp/followup-l2tp-call.md -- exact original relay peers.
// Related: kernel_ppp_owners_linux.go -- worker cleanup retains uncertain peers.

//go:build linux

package l2tp

import (
	"errors"
	"os"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/callsink"
)

// maybeBridgePPPoE consumes fresh attached channel owners only after the access
// side released its local PPP pair. Nonrelay calls keep their ordinary setup.
// The worker MUST retain owners until bridge.Close has settled both originals.
func maybeBridgePPPoE(ev kernelSetupEvent, owners *kernelPPPOwners) (bool, error) {
	return maybeBridgePPPoEWith(ev, owners, pppkernel.CreatePPPChannel,
		func(first, second *pppkernel.CreatedChannel) (kernelPPPBridge, error) {
			return pppkernel.NewOriginalChannelBridge(first, second)
		})
}

// These private ports model capture failure without minting public native
// owners. Only the production constructor can grant an original bridge.
func maybeBridgePPPoEWith(ev kernelSetupEvent, owners *kernelPPPOwners,
	capture func(*os.File) (*pppkernel.CreatedChannel, error),
	construct func(*pppkernel.CreatedChannel, *pppkernel.CreatedChannel) (kernelPPPBridge, error),
) (requested bool, err error) {
	if ev.lnsMode {
		return false, nil
	}
	if ev.channel == nil {
		return false, nil
	}
	original, supported := ev.channel.(callsink.RelayOriginalLifetime)
	if !supported {
		return true, errors.New("l2tp: relay transport cannot retain its original PPPoX file")
	}
	if owners == nil {
		return true, errors.New("l2tp: bridge has no original resources")
	}
	lease, err := original.RetainOriginalFile()
	if err != nil {
		return true, err
	}
	owners.relayFile = lease
	if lease == nil {
		return true, errors.New("l2tp: relay transport returned no original file lease")
	}
	var peer *pppkernel.CreatedChannel
	defer func() {
		failure := recover()
		// Constructor entry consumes both owners even on error or panic. Until
		// then this local is the only owner of a partially captured peer.
		var cleanupErr error
		if peer != nil {
			cleanupErr = closeKernelPPPOperation(peer.Close)
		}
		if failure != nil {
			if cleanupErr != nil {
				panic(pppkernel.RecordSetupCleanup(kernelPPPFailure(failure), cleanupErr))
			}
			panic(failure)
		}
		err = pppkernel.RecordSetupCleanup(err, cleanupErr)
	}()
	err = lease.WithOriginalFile(func(file *os.File) error {
		if file == nil {
			return errors.New("l2tp: relay supplied no original PPPoX file")
		}
		var captureErr error
		peer, captureErr = capture(file)
		return captureErr
	})
	if err != nil {
		return true, err
	}
	if peer == nil {
		return true, errors.New("l2tp: relay capture returned no original peer channel")
	}
	if owners.channel == nil {
		return true, errors.New("l2tp: bridge has no fresh original local channel")
	}
	first, second := owners.channel, peer
	owners.channel, peer = nil, nil
	bridge, err := construct(first, second)
	// Root any returned owner before observing error or calling native Bridge.
	owners.bridge = bridge
	if err != nil {
		return true, err
	}
	if bridge == nil {
		return true, errors.New("l2tp: original bridge constructor returned no owner")
	}
	return true, bridge.Bridge()
}
