//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the test override
//
// Two native probes, one for each enrolled capability. Both ask whether the
// CAPABILITY exists, never how the kernel was PACKAGED: /proc/modules lists
// loaded modules only, so a kernel with CONFIG_XFRM_USER=y or
// CONFIG_MPLS_ROUTING=y reads as absent there. Under a refusal that misreading
// stops a working router, which is why neither probe touches the module list.
//
// Neither probe executes a binary. An external program is a second dependency
// that can be absent for its own reasons, which is the fault this package
// removes rather than a way to detect it.

package kernelcap

import (
	"errors"
	"io/fs"
	"os"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// xfrmOpen is the raw socket open, a var so a unit test drives every errno
// without a kernel that changes.
var xfrmOpen = openXFRMNetlink

// readFile is the probe's file read, a var for the same reason.
var readFile = os.ReadFile

// XFRM reports whether the kernel holds the IPsec dataplane ze installs every
// Child SA through.
//
// Opening NETLINK_XFRM is the honest test, and it is the one a dump of the
// Security Policy Database cannot make: the dump needs CAP_NET_ADMIN, so an
// unprivileged reader gets EPERM and cannot tell a kernel without XFRM from a
// process without the capability. The open needs no CAP_NET_ADMIN, and on a
// modular kernel it loads xfrm_user the same way any XFRM user would.
func XFRM() Result {
	if forced, ok := forcedXFRM(); ok {
		return forced
	}
	return classifyXFRM(xfrmOpen())
}

func openXFRMNetlink() error {
	handle, err := netlink.NewHandle(unix.NETLINK_XFRM)
	if err != nil {
		return err
	}
	handle.Close()
	return nil
}

// classifyXFRM turns the socket open's outcome into a verdict.
//
// EPROTONOSUPPORT is the kernel saying it carries no XFRM netlink protocol,
// which is absence. EAFNOSUPPORT is the same answer from a kernel with no
// AF_NETLINK support for the family. Every other errno, EPERM included, means
// the question was not answered: reporting absence there would rebuild a kernel
// for a capability the host already has.
func classifyXFRM(err error) Result {
	if err == nil {
		return Result{State: StatePresent}
	}
	if errors.Is(err, unix.EPROTONOSUPPORT) || errors.Is(err, unix.EAFNOSUPPORT) {
		return Result{State: StateAbsent, Reason: err}
	}
	return Result{State: StateUnknown, Reason: err}
}

// MPLS reports whether the kernel holds an AF_MPLS forwarding table.
//
// af_mpls creates net.mpls.platform_labels when MPLS routing is available,
// built in or loaded as a module, so one probe answers for both packagings.
//
// Only EXISTENCE is read. The sysctl is the size of the label space and it
// defaults to 0, which disables MPLS; ze WRITES a label space when it programs
// its first label (internal/plugins/fib/kernel/labelspace_linux.go), so a zero
// here is a dead configuration ze repairs rather than a fault it reports
// (owner decision 5, 2026-08-14).
func MPLS() Result {
	path := MPLSPlatformLabelsPath()
	_, err := readFile(path)
	if err == nil {
		return Result{State: StatePresent}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return Result{State: StateAbsent, Reason: err}
	}
	// The probe could not be READ, which is not the same as "MPLS is absent". A
	// guard that cannot reach its evidence says so rather than reporting the
	// answer it did not get (ai/rules/evidence.md).
	return Result{State: StateUnknown, Reason: err}
}
