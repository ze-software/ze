// Design: plan/learned/1274-netlink-int-field-truncation.md -- netlink int width

package policyroute

// netlinkTableInt converts a kernel routing table ID to the Go int that
// netlink.Rule.Table is typed as.
//
// int is 64 bits on this target, so every uint32 converts exactly and there is
// nothing to reject. The check that the generic build needs lives in
// netlinkint_linux_generic.go; keeping it out of here is what lets the full
// kernel-legal table range stay usable on the targets Ze actually ships
// (mk/appliance.mk:103-104 builds linux/amd64 and linux/arm64 only).
//
// The per-architecture split is not cosmetic. It is also what makes the
// conversion analysable: CodeQL derives a file's int width from its build
// constraints, including the _amd64 filename suffix
// (github/codeql go/ql/lib/semmle/go/Files.qll, implicitlyConstrainsIntBitSize),
// so in this file int is known to be 64-bit and a uint32 -> int conversion is
// not a narrowing one. Expressing the bound as this build's math.MaxInt in a
// file with no architecture constraint is what left
// go/incorrect-integer-conversion flagged as alert 171.
func netlinkTableInt(v uint32) (int, error) {
	return int(v), nil
}
