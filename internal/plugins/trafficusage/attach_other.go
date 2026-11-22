//go:build !linux

// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage non-Linux stub attacher

package trafficusage

import (
	"errors"
	"fmt"
)

// errUnsupportedPlatform is returned by every attacher operation on non-Linux
// builds. eBPF TCX exists only in the Linux kernel.
var errUnsupportedPlatform = errors.New("traffic-usage: eBPF TCX accounting is supported on Linux only")

// unsupportedAttacher is the non-Linux attacher: it never attaches and always
// reports the platform as unsupported, so the plugin degrades to a clean no-op
// rather than misreporting zero traffic.
type unsupportedAttacher struct{}

func newAttacher() attacher { return unsupportedAttacher{} }

// errNoTCXHere is every answer of this attacher: the build holds no TCX
// attach point, which classifyAttachError reads as absent.
var errNoTCXHere = fmt.Errorf("%w: %w", errNoTCX, errUnsupportedPlatform)

func (unsupportedAttacher) Available() error { return errNoTCXHere }

func (unsupportedAttacher) Attach(int, string, uint32, bool) (attachment, error) {
	return nil, errNoTCXHere
}
