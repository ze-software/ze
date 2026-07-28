// Design: plan/learned/1274-netlink-int-field-truncation.md -- netlink int width

package policyroute

// netlinkTableInt converts a kernel routing table ID to the Go int that
// netlink.Rule.Table is typed as. int is 64 bits on this target, so every
// uint32 converts exactly. See netlinkint_linux_amd64.go for why the
// conversion is split per architecture.
func netlinkTableInt(v uint32) (int, error) {
	return int(v), nil
}
