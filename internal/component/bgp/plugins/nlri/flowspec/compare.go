// Design: docs/architecture/wire/nlri-flowspec.md -- packet filter precedence
package flowspec

import (
	"bytes"
	"cmp"
)

// Compare returns a negative value when a has higher packet-filter precedence.
// RFC 8955 Section 5.1: "The relative order of two Flow Specifications is
// determined by comparing their respective components."
// This comparison does not use peer identity, arrival time, or action values.
func Compare(a, b *FlowSpec) int {
	for typ := FlowDestPrefix; typ <= FlowFlowLabel; typ++ {
		left, right := a.componentOfType(typ), b.componentOfType(typ)
		if left == nil && right == nil {
			continue
		}
		if left == nil {
			return 1
		}
		if right == nil {
			return -1
		}
		if lp, ok := left.(*prefixComponent); ok {
			rp := right.(*prefixComponent)
			// RFC 8956 Section 4: lower offsets have higher precedence.
			if order := cmp.Compare(lp.offset, rp.offset); order != 0 {
				return order
			}
			l, r := lp.prefix.Masked(), rp.prefix.Masked()
			if l.Overlaps(r) {
				if order := cmp.Compare(r.Bits(), l.Bits()); order != 0 {
					return order
				}
			} else if order := l.Addr().Compare(r.Addr()); order != 0 {
				return order
			}
			continue
		}
		l, r := comparisonData(left, a.family), comparisonData(right, b.family)
		common := min(len(l), len(r))
		if order := bytes.Compare(l[:common], r[:common]); order != 0 {
			return order
		}
		if order := cmp.Compare(len(r), len(l)); order != 0 {
			return order
		}
	}
	return 0
}

func comparisonData(component FlowComponent, fam Family) []byte {
	if numeric, ok := component.(*numericComponent); ok {
		if numeric.wireData != nil {
			return numeric.wireData
		}
		data := make([]byte, numeric.Len())
		numeric.writeToAFI(data, 0, fam.AFI)
		return data[1:]
	}
	return component.Bytes()[1:]
}
