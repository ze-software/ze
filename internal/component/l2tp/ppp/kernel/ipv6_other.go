// Design: docs/research/l2tpv2-ze-integration.md -- kernel IPv6 availability

//go:build !linux

package kernel

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func openIPv6Carrier(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
	return nil, errors.New("PPP kernel IPv6 carrier requires Linux")
}
