// Design: docs/architecture/policyroute/netlink-int-field-truncation.md -- netlink int width

//go:build linux && !amd64 && !arm64

package policyroute

import (
	"fmt"
	"math"
)

// netlinkTableInt converts a kernel routing table ID to the Go int that
// netlink.Rule.Table is typed as, on a target whose int width is not known
// here to be 64 bits.
//
// The encoder emits FRA_TABLE only for Table >= 256 and the compat byte only
// for 0 <= Table < 256 (vendor/github.com/vishvananda/netlink/rule_linux.go:57,126),
// so a table ID that turns negative in the conversion is not rejected: the rule
// is installed with RT_TABLE_UNSPEC and the operator's steering is silently
// dropped. Refuse it instead.
//
// The bound is this build's own int maximum, so it is exact on a 32-bit target
// (where it is MaxInt32) and on any 64-bit target not named by a sibling file.
// Ze does not ship such a target -- mk/build-appliance.mk:103-104 builds linux/amd64
// and linux/arm64 only -- so this path exists to keep `go build` honest
// elsewhere, not to bound what operators can configure.
func netlinkTableInt(v uint32) (int, error) {
	if uint64(v) > uint64(math.MaxInt) {
		return 0, fmt.Errorf("table %d exceeds %d, the largest this build can program through netlink", v, uint64(math.MaxInt))
	}
	return int(v), nil
}
