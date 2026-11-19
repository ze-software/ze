// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- both actual original closes
// Related: client_native_owner.go -- close both before waiting or writer settlement

package pppoeclient

import (
	"context"
	"errors"
)

// waitNativeOriginals MUST follow advancing BOTH present original closes, including unit
// Close when channel Close failed. It never initiates either close and MUST NOT
// run under a registry/status mutex or descriptor borrow. Every error is joined;
// failure grants no writer/backend release even if the unit alone retired.
func (owner *clientNativeOwner) waitNativeOriginals(ctx context.Context) error {
	var channelErr, unitErr error
	if owner.channel != nil {
		channelErr = owner.channel.WaitNativePPPChannelRetired(ctx)
	}
	if owner.unit != nil {
		unitErr = owner.unit.WaitNativePPPUnitRetired(ctx)
	}
	return errors.Join(channelErr, unitErr)
}
