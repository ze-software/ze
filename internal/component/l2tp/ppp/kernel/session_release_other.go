//go:build !linux

// Design: docs/research/l2tpv2-ze-integration.md -- native PPP requires Linux

package kernel

import (
	"errors"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func duplicateSessionDescriptor(int) (int, error) {
	return -1, errors.New("ppp: native unit retention requires Linux")
}

func stampSessionBirth(ppp.SessionIdentity, string) error {
	return errors.New("ppp: native unit birth stamping requires Linux")
}
