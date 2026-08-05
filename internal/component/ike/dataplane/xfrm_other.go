// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- non-Linux dataplane stub

//go:build !linux

package dataplane

import (
	"fmt"
	"net"
	"runtime"
)

type xfrmBackend struct{}

func newXFRMBackend() (Dataplane, error) {
	return &xfrmBackend{}, nil
}

func (b *xfrmBackend) InstallSA(_ SAParams) error {
	return fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) RemoveSA(_ uint32, _ net.IP, _ uint8) error {
	return fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) InstallPolicy(_ SPParams) error {
	return fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) RemovePolicy(_, _ *net.IPNet, _ SADir) error {
	return fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) RemovePolicyParams(_ SPParams) error {
	return fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) ListSAs(_ uint32) ([]SAInfo, error) {
	return nil, fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) ListPolicies() ([]PolicyInfo, error) {
	return nil, fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

func (b *xfrmBackend) Close() error { return nil }
