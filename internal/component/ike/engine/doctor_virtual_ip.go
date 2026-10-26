// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- virtual-IP pool utilization
// RFC: rfc/short/rfc7296.md -- INTERNAL_ADDRESS_FAILURE (Section 3.15.4)
// Related: doctor.go -- the sibling ike readiness checks and their registration
// Related: virtual_ip.go -- the lease registry and the pool generation this check reads
// Related: metrics.go -- publishVirtualIPGauges reports the same counts

package engine

import (
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// virtualIPPoolWarnPercent is the share of a family's range held at which the doctor
// warns: the spec's 90%, high enough to stay quiet on a busy pool and early enough to
// widen the range before the next client is refused.
const virtualIPPoolWarnPercent = 90

// poolUsage is one family of one pool: how many leases it holds and can hold.
type poolUsage struct {
	pool     string
	family   eap.Family
	held     uint64
	capacity uint64
}

// virtualIPPoolUsage lists the usage of every served family of every pool of the running
// engine, sorted by pool name, IPv4 first. It answers ok false when no engine runs, so a
// caller can tell "no engine" from "no pool".
func virtualIPPoolUsage() ([]poolUsage, bool) {
	table := ActiveTable()
	if table == nil {
		return nil, false
	}
	gen := table.virtualIP.pools.Load()
	if gen == nil {
		return nil, true
	}
	names := make([]string, 0, len(gen.byName))
	for name := range gen.byName {
		names = append(names, name)
	}
	slices.Sort(names)
	var out []poolUsage
	for _, name := range names {
		for _, family := range [...]eap.Family{eap.FamilyIPv4, eap.FamilyIPv6} {
			held, capacity, ok := table.virtualIP.leases.HeldIn(gen.byName[name], family)
			if !ok {
				continue
			}
			out = append(out, poolUsage{pool: name, family: family, held: held, capacity: capacity})
		}
	}
	return out, true
}

// checkIPsecVirtualIPPool warns for each pool family at or above 90% held. A full pool
// refuses the next client with INTERNAL_ADDRESS_FAILURE (RFC 7296 Section 3.15.4), and
// nothing on the operator's side says so until a client complains.
//
// It reads the running engine, as checkIPsecUDPEncap does, because utilization is runtime
// state the config tree does not hold. With no engine running there is nothing held and
// the check reports nothing.
func checkIPsecVirtualIPPool(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	usage, running := virtualIPPoolUsage()
	if !running {
		return nil
	}
	var out []rpc.DoctorCheckDiagnostic
	for _, u := range usage {
		if u.held*100 < u.capacity*virtualIPPoolWarnPercent {
			continue
		}
		leaf := "range"
		if u.family == eap.FamilyIPv6 {
			leaf = "range6"
		}
		var tb textbuf.Buffer
		tb.Str("ipsec remote-access pool ").Str(u.pool).Str(" holds ").Int(int64(u.held)).
			Str(" of the ").Int(int64(u.capacity)).Str(" addresses its ").Str(leaf).
			Str(" can lease, so the next client bound to it may be refused with " +
				"INTERNAL_ADDRESS_FAILURE and get no tunnel. Widen ").Str(leaf).
			Str(" under vpn ipsec remote-access pool ").Str(u.pool).
			Str(", or bind some peers to another pool with virtual-ip-pool.")
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecVirtualIPPool,
			Severity: severityWarning,
			Message:  tb.String(),
		})
	}
	return out
}
