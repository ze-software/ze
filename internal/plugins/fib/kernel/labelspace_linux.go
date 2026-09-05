//go:build linux

// Design: docs/guide/mpls.md -- the label space ze writes before it programs a label
// Related: kernelcap_linux.go -- the capability enrolment that reports the table's absence
// Related: mplsentry_linux.go, nexthop_linux.go -- the two paths that program a label
//
// net.mpls.platform_labels is the size of the kernel's label table and it
// defaults to 0, which disables MPLS entirely. A kernel that builds MPLS in
// therefore boots with the table present and the label space empty, which is
// the state every ze appliance starts in.
//
// A label space of 0 is a dead configuration ze can repair rather than report
// (owner decision 5, 2026-08-14), so ze writes it here, once, immediately
// before it programs its first label. Writing at the point of use rather than
// at startup is what guarantees the ordering the operator needs: the space
// exists before the route that needs it, on every path that programs one.

package fibkernel

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

// labelSpaceMax is the whole 20-bit label space (RFC 3032 Section 2.1), which
// is also the maximum internal/core/sysctl/known_linux.go accepts for
// net.mpls.platform_labels. Ze cannot know the highest label a peer will send
// it, and a table shorter than the label the peer chose drops that traffic with
// nothing said, so the default ze writes covers every legal label. The kernel
// allocates one pointer for each entry, which is 8 MiB on a 64-bit host. An
// operator who wants a smaller table sets net.mpls.platform_labels in the
// sysctl {} block, and the write below leaves that value alone.
const labelSpaceMax = 1048575

// labelSpaceOnce holds the repair to one attempt for each process. The label
// space is one global kernel property, so a second read on every route would
// buy nothing.
var labelSpaceOnce sync.Once

// writeFile is the repair's write, a var so a unit test drives it without root.
var writeFile = os.WriteFile

// readLabelSpaceFile is the repair's read, a var for the same reason.
var readLabelSpaceFile = os.ReadFile

// ensureLabelSpace writes a non-zero label space when the kernel holds an
// AF_MPLS table whose label space is still the disabling default.
//
// An operator's own value is never overwritten: only a table reading exactly 0
// is repaired. A kernel with no table at all is left alone too, because the
// capability gate has already refused the start for it, and a `ze doctor` run
// that reached here must not create the file.
//
// A failure is logged and not returned. The caller is about to program a label
// through netlink and that call reports its own failure with the route it was
// installing, which is a better message than this one could give.
func ensureLabelSpace() {
	labelSpaceOnce.Do(repairLabelSpace)
}

// repairLabelSpace is the repair itself, separated from the once so a test can
// run it for each case. Every caller in production goes through ensureLabelSpace.
func repairLabelSpace() {
	path := kernelcap.MPLSPlatformLabelsPath()
	current, err := readLabelSpaceFile(path)
	if err != nil {
		logger().Warn("fib-kernel: cannot read the MPLS label space", "path", path, "error", err)
		return
	}
	if strings.TrimSpace(string(current)) != "0" {
		return
	}
	size := strconv.Itoa(labelSpaceMax)
	if err := writeFile(path, []byte(size), 0o644); err != nil {
		logger().Warn("fib-kernel: cannot enable the MPLS label space",
			"path", path, "size", size, "error", err)
		return
	}
	logger().Info("fib-kernel: MPLS label space enabled", "path", path, "size", size)
}
