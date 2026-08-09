// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS shared constants, errors, validation
// Related: nexthop_linux.go -- buildMPLSEncap programs the netlink MPLS encap
// Related: fibkernel.go -- addChange/replaceChange dispatch labeled routes here

package fibkernel

import (
	"errors"
	"fmt"
)

var errMPLSEmptyLabelStack = errors.New("mpls: empty label stack")

const (
	// RFC 3032 Section 2.1: 20-bit label field.
	maxMPLSLabel  = 1048575
	maxLabelStack = 16
)

// validateMPLSLabels enforces the RFC 3032 label-stack invariants. Validation
// is platform-independent (the 20-bit range and stack-depth limit do not depend
// on the kernel); only the netlink programming in nexthop_linux.go is Linux-only.
func validateMPLSLabels(labels []uint32) error {
	if len(labels) == 0 {
		return errMPLSEmptyLabelStack
	}
	if len(labels) > maxLabelStack {
		return fmt.Errorf("mpls: label stack depth %d exceeds limit %d", len(labels), maxLabelStack)
	}
	for _, l := range labels {
		if l > maxMPLSLabel {
			return fmt.Errorf("mpls: label %d exceeds 20-bit maximum %d", l, maxMPLSLabel)
		}
	}
	return nil
}
