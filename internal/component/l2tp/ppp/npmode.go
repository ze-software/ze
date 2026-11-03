// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared PPP network-protocol mode.
// Related: devppp_linux.go -- ioctl; devppp_other.go -- unsupported platform.
// Linux API: https://github.com/torvalds/linux/blob/v6.18/include/uapi/linux/ppp_defs.h

package ppp

import "fmt"

// NPMode selects whether an opened unit passes or discards IPv4 packets.
// Zero is invalid. SetIPv4Mode rejects unnamed values before any ioctl.
// These values are independent of the Linux ABI, whose PASS value is zero.
type NPMode uint8

const (
	NPModeUnspecified NPMode = iota
	NPModePass
	NPModeDrop
)

// kernelValue validates the requested mode and converts it to Linux enum NPmode.
func (mode NPMode) kernelValue() (int32, error) {
	// The set is open because a caller can convert any integer to NPMode.
	switch mode {
	case NPModePass:
		return 0, nil
	case NPModeDrop:
		return 1, nil
	case NPModeUnspecified:
		return 0, fmt.Errorf("ppp: IPv4 mode is unspecified; select NPModePass or NPModeDrop")
	default:
		return 0, fmt.Errorf("ppp: invalid IPv4 mode %d; select NPModePass or NPModeDrop", mode)
	}
}
