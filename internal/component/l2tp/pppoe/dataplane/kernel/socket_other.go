// Design: docs/architecture/l2tp/bng-5-pppoe.md -- explicit native-platform refusal
// Related: socket_linux.go -- Linux PPPoX socket implementation

//go:build !linux

package kernel

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

func requirePlatform() error {
	return errors.New("pppoe: netlink sessions require Linux")
}

func openTransport(context.Context, dataplane.Request) (*pppoxTransport, error) {
	return nil, requirePlatform()
}
