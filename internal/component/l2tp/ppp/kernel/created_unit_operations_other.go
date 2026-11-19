// Design: docs/research/l2tpv2-ze-integration.md -- Linux original-unit operations

//go:build !linux

package kernel

import (
	"context"
	"errors"
)

func (unit *CreatedUnit) WithOriginalNamespace(context.Context, func() error) error {
	return errors.New("ppp: original native unit namespace operations require Linux")
}
