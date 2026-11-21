// Design: docs/architecture/l2tp/bng-5-pppoe.md -- explicit socket platform refusal

//go:build !linux

package vpp

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

func openControlSocket(context.Context, dataplane.Request) (*controlSocket, error) {
	return nil, errors.New("pppoe-vpp: PPPoE control sockets require Linux")
}
