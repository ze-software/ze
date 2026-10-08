//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the test override
//
// Three native probes, one for each enrolled capability. Each asks whether the
// CAPABILITY exists, never how the kernel was PACKAGED: /proc/modules lists
// loaded modules only, so a kernel with CONFIG_XFRM_USER=y or
// CONFIG_MPLS_ROUTING=y reads as absent there. Under a refusal that misreading
// stops a working router, which is why no probe touches the module list.
//
// No probe executes a binary. An external program is a second dependency
// that can be absent for its own reasons, which is the fault this package
// removes rather than a way to detect it.

package kernelcap

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
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

const (
	// mplsProbeLabel is the label the transit MTU probe addresses: the first
	// label Linux lets a route use (MPLS_LABEL_FIRST_UNRESERVED). The probe
	// never creates it, and a route a peer already holds there is left alone.
	mplsProbeLabel = 16
	// mplsProbeMTU is any frame budget the patched kernel accepts (at least 68).
	mplsProbeMTU = 1500
)

// mplsRouteProbe sends one probe request, a var so a unit test drives every
// errno without a kernel that changes.
var mplsRouteProbe = sendMPLSRouteProbe

var (
	errMPLSLabelSpaceSmall = errors.New("the MPLS label space holds no unreserved label to address, " +
		"so the probe cannot be asked; ze enables one when it programs its first label")
	errMPLSProbeCreated  = errors.New("the kernel accepted a probe that may not create a route")
	errMPLSMetricRefused = errors.New("the kernel refuses RTA_METRICS on an AF_MPLS route " +
		"(no CONFIG_MPLS_IP_MTU: gokrazy/kernel/patches/0002-mpls-ip-mtu.patch is not applied)")
)

// MPLSIPMTU reports whether the kernel carries Ze's MPLS IP MTU patch, which
// lets an AF_MPLS route hold a path MTU (RTA_METRICS/RTAX_MTU) and enforces it
// on transit. Upstream Linux rejects the attribute with EINVAL.
//
// The probe asks without changing anything: an RTM_NEWROUTE with NLM_F_EXCL
// and no NLM_F_CREATE can only fail, with ENOENT when the label is free and
// EEXIST when it is taken, once the kernel has parsed every attribute. A
// control request without the metric must reach that answer first, so the
// probe never reads an EINVAL from its own malformed request, a label outside
// the label space or a missing AF_MPLS table as the unpatched kernel. Asking
// needs CAP_NET_ADMIN; an unprivileged reader is told the answer is unknown.
// Both requests run in the calling thread's network namespace.
func MPLSIPMTU() Result {
	data, err := readFile(MPLSPlatformLabelsPath())
	if err != nil {
		return Result{State: StateUnknown, Reason: fmt.Errorf("read the MPLS label space: %w", err)}
	}
	labels, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return Result{State: StateUnknown, Reason: fmt.Errorf("read the MPLS label space: %w", err)}
	}
	if labels <= mplsProbeLabel {
		return Result{State: StateUnknown, Reason: errMPLSLabelSpaceSmall}
	}
	return classifyMPLSIPMTU(mplsRouteProbe(false), mplsRouteProbe(true))
}

// classifyMPLSIPMTU turns the control answer and the metric answer into a
// verdict. Only the metric being refused where the control was not is absence.
func classifyMPLSIPMTU(control, metric error) Result {
	if control == nil {
		return Result{State: StateUnknown, Reason: errMPLSProbeCreated}
	}
	if !mplsProbeReachedLookup(control) {
		return Result{State: StateUnknown, Reason: fmt.Errorf("the probe without a metric was refused: %w", control)}
	}
	if metric == nil {
		return Result{State: StateUnknown, Reason: errMPLSProbeCreated}
	}
	if mplsProbeReachedLookup(metric) {
		return Result{State: StatePresent}
	}
	if errors.Is(metric, unix.EINVAL) {
		return Result{State: StateAbsent, Reason: errMPLSMetricRefused}
	}
	return Result{State: StateUnknown, Reason: fmt.Errorf("the probe with a metric was refused: %w", metric)}
}

// mplsProbeReachedLookup reports whether the kernel parsed the request and
// answered from the label table, which only happens after every attribute was
// accepted.
func mplsProbeReachedLookup(err error) bool {
	if errors.Is(err, unix.ENOENT) {
		return true
	}
	return errors.Is(err, unix.EEXIST)
}

// sendMPLSRouteProbe sends one non-creating AF_MPLS RTM_NEWROUTE for the probe
// label, carrying RTA_METRICS/RTAX_MTU when withMetric is set.
func sendMPLSRouteProbe(withMetric bool) error {
	request := nl.NewNetlinkRequest(unix.RTM_NEWROUTE, unix.NLM_F_ACK|unix.NLM_F_EXCL)
	message := nl.NewRtMsg()
	message.Family = unix.AF_MPLS
	message.Dst_len = 20 // An MPLS route is keyed by one 20-bit label.
	request.AddData(message)
	request.AddData(nl.NewRtAttr(unix.RTA_DST, nl.EncodeMPLSStack(mplsProbeLabel)))
	if withMetric {
		metrics := nl.NewRtAttr(unix.RTA_METRICS, nil)
		metrics.AddRtAttr(unix.RTAX_MTU, nl.Uint32Attr(mplsProbeMTU))
		request.AddData(metrics)
	}
	_, err := request.Execute(unix.NETLINK_ROUTE, 0)
	return err
}
