// Design: register.go -- DoctorChecks registration (flow-export-conntrack-tracking).
// Related: conntrack_worker.go -- the periodic dump that feeds the recent-flow ring.
//
// flow-export conntrack export reads the kernel nf_conntrack table over ctnetlink.
// If nf_conntrack is not loaded (nothing else pulled it in -- no firewall/NAT rule,
// no explicit modprobe), the table read returns nothing, the recent-flow ring
// stays empty, and the DDoS characterizer silently degrades to generic-flood with
// no discriminating vector. `ze doctor` surfaces that instead of leaving it silent.

package flowexport

import (
	"os"
	"runtime"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// nfConntrackAvailable reports whether the kernel conntrack table is available to
// read. A package var so the doctor test can stub it.
var nfConntrackAvailable = nfConntrackAvailableImpl

// nfConntrackAvailableImpl reports whether nf_conntrack is loaded:
// /proc/sys/net/netfilter/nf_conntrack_max exists only once the module is loaded
// (a firewall/NAT rule, or an explicit modprobe, pulls it in). Its absence means
// the ctnetlink dump the conntrack worker uses returns an empty table. The check
// is registered for linux only; on other platforms /proc is absent so it reports
// false, but the check never runs there.
func nfConntrackAvailableImpl() bool {
	if runtime.GOOS != "linux" {
		return true // nf_conntrack is a linux concept; do not warn elsewhere
	}
	_, err := os.Stat("/proc/sys/net/netfilter/nf_conntrack_max")
	return err == nil
}

// checkConntrackTracking warns when flow-export conntrack export is enabled but
// nf_conntrack is not available, so operators learn why DDoS characterization
// degrades to generic-flood (empty recent-flow ring) rather than hitting it blind.
func checkConntrackTracking(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	fe := tree.GetContainer(configRootFlowExport)
	if fe == nil {
		return nil
	}
	ct := fe.GetContainer("conntrack")
	if ct == nil {
		return nil
	}
	if enabled, ok := ct.Get("enabled"); !ok || enabled != "true" {
		return nil
	}
	if nfConntrackAvailable() {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-flowexport-conntrack-unavailable",
		Severity: "warning",
		Message:  "flow-export conntrack export is enabled but the kernel nf_conntrack table is unavailable (module not loaded -- e.g. no firewall/NAT rule pulled it in); the recent-flow ring stays empty and DDoS characterization degrades to generic-flood with no target vector",
	}}
}
