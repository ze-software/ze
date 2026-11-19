// Design: docs/research/l2tpv2-ze-integration.md -- worker original mutation owner

//go:build linux

package l2tp

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// prepareKernelPPPSetup chooses this complete producer before worker locking.
// The native provider and counter backend remain the SAME original instance.
func pppSetupWithNativeBackend(ev kernelSetupEvent, creator *pppkernel.UnitCreator, backend pppkernel.InterfaceOperations) (pppSessionFDs, error) {
	provider, ok := backend.(iface.NativePPPBackend)
	if !ok {
		return pppSessionFDs{}, errors.Join(iface.ErrNativePPPUnavailable, errors.New("l2tp: original backend has no original-unit mutation binding"))
	}
	return pppSetupWith(ev, creator, kernelPPPSetupOperations{
		create: openKernelPPPTransport,
		setup: func(creator *pppkernel.UnitCreator, transport *os.File, name string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
			channel, unit, err := pppkernel.DevPPPSetup(creator, transport, name)
			if err != nil {
				return channel, unit, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err = unit.BindNativeOperations(ctx, backend, func(original *pppkernel.CreatedUnit, index int, number int32) (pppkernel.NativeUnitOperations, error) {
				writer, err := provider.BindNativePPPUnit(original, index, number)
				return writer, err
			})
			return channel, unit, err
		},
		channel:  pppkernel.CreatePPPChannel,
		snapshot: snapshotOriginalKernelPPP,
		bridge:   bridgeOriginalKernelPPP,
		close: func(owners *kernelPPPOwners, transport *os.File, _ [3]int) [3]error {
			return closeOriginalKernelPPP(owners, transport, (*os.File).Close)
		},
	})
}

func closeOriginalKernelNativeUnit(unit *pppkernel.CreatedUnit) error {
	physical := unit.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	retired := unit.WaitNativePPPUnitRetired(ctx)
	native := unit.CloseNativeOperations(ctx)
	return errors.Join(physical, retired, native)
}
