// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane
// Related: doctor_label.go -- the fib-vpp-label-entries check and its code
// Related: doctor_route.go -- the fib-vpp-stale-routes check and its code
// Related: doctor_srv6.go -- the fib-vpp-srv6 check and its code

package fibvpp

import "github.com/ze-software/ze/internal/core/diagnostic"

// exampleShowDoctor is the command every fib-vpp doctor diagnostic cites as
// its first example.
const exampleShowDoctor = "ze show doctor"

// init registers the explanation of each code the fib-vpp-label-entries,
// fib-vpp-stale-routes, and fib-vpp-srv6 checks report, so `ze show diagnostic`
// describes it.
func init() {
	for _, meta := range []diagnostic.CodeMeta{
		pathMTUCodeMeta,
		staleLabelCodeMeta,
		staleRouteCodeMeta,
		srv6RefusedCodeMeta,
	} {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: fib-vpp: doctor code registration refused: " + err.Error())
		}
	}
}
