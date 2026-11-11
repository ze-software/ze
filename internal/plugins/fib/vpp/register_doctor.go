// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane
// Related: doctor_label.go -- the fib-vpp-label-entries check and its code

package fibvpp

import "github.com/ze-software/ze/internal/core/diagnostic"

// init registers the explanation of each code the fib-vpp-label-entries check
// reports, so `ze show diagnostic` describes it.
func init() {
	for _, meta := range []diagnostic.CodeMeta{pathMTUCodeMeta, staleLabelCodeMeta} {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: fib-vpp: doctor code registration refused: " + err.Error())
		}
	}
}
