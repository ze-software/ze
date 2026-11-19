// Design: docs/architecture/appliance/kernel-profiles.md -- original PPP IPv6 control sockets

//go:build !linux

package kernel

import (
	"context"
	"errors"
	"os"
)

func (*CreatedUnit) openIPv6ControlSocket(context.Context, IPv6ControlSocketKind, int32, int32) (*os.File, error) {
	return nil, errors.New("ppp: original IPv6 control sockets require Linux")
}
