// Design: docs/research/l2tpv2-ze-integration.md -- original PPP creation custody

//go:build !linux

package kernel

import "errors"

func NewUnitCreator(InterfaceOperations) (*UnitCreator, error) {
	return nil, errors.New("ppp: original unit creation requires Linux /dev/ppp and rtnetlink")
}
