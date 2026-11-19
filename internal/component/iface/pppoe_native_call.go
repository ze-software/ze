// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- guarded result application
// Related: pppoe_admission.go -- no admission gap around native outcomes

package iface

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
)

func invokePPPoENativeWrite(ctx context.Context, write func(context.Context) error) (result error) {
	defer func() {
		if failure := recover(); failure != nil {
			cause, ok := failure.(error)
			if !ok {
				cause = fmt.Errorf("pppoe-client: native writer panicked: %v", failure)
			}
			result = errors.Join(cause, ErrNativePPPPending)
		}
	}()
	return write(ctx)
}

func (c *PPPoEClient) applyNativeLinkLocal(applied, next netip.Addr) (netip.Addr, error) {
	result := applied
	err := c.nativeWrite(func(context.Context) error {
		var err error
		result, err = applyPPPoELinkLocal(c.native, applied, next)
		return err
	})
	return result, err
}
