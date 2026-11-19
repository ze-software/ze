// Design: docs/research/l2tpv2-ze-integration.md -- factory original mutation owner

package kernel

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// The backend argument is captured by preparedFactoryOps. It is neither a
// current-backend lookup nor a wrapper replacing SessionConfig provenance.
func setupOriginalNativePPP(backend sessionBackend, creator *pppkernel.UnitCreator, transport *os.File, name string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
	provider, ok := backend.(iface.NativePPPBackend)
	if !ok {
		return nil, nil, errors.Join(iface.ErrNativePPPUnavailable, errors.New("pppoe: original backend has no original-unit mutation binding"))
	}
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
}

// Failed partial binding still owns its exact writer. Unit closure starts
// before settlement, and the original backend retains any pending writer.
func closeOriginalNativePPP(unit *pppkernel.CreatedUnit) error {
	if unit == nil {
		return nil
	}
	physical := unit.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	retired := unit.WaitNativePPPUnitRetired(ctx)
	native := unit.CloseNativeOperations(ctx)
	return errors.Join(physical, retired, native)
}
