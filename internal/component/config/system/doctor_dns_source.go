// Design: docs/features/dns-resolver.md -- the doctor checks over every consumer's dns-source selection
// Overview: register.go -- the init() that registers these as a runtime doctor check
// Related: resolv.go -- Resolvers, the registry whose snapshot and publications these read
//
// These checks judge what the running daemon selected, so they read the
// registry the consumers publish into (Resolvers.Snapshot and
// Resolvers.Consumers). That registry is filled only in the daemon, so they
// register in the runtime doctor phase (diagnostic.DoctorPhaseRuntime):
// `show doctor` runs them, and the local `ze doctor`, whose registry is empty,
// does not.

package system

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// dnsSourceDoctorCheckName is the name the runtime doctor check registers as.
const dnsSourceDoctorCheckName = "dns-source-selection"

// dnsSourceDoctorCodes are the codes checkDNSSourceSelections emits.
var dnsSourceDoctorCodes = []string{
	diagnostic.CodeDoctorDNSSourceEmpty,
	diagnostic.CodeDoctorDNSSourceInterfaceSilent,
	diagnostic.CodeDoctorDNSSourceSelfUnserved,
}

// dnsSourceDoctorCheck is the runtime doctor check over every dns-source
// selection. It reads live daemon state, never the config tree, so the
// context it is handed carries nothing it needs.
var dnsSourceDoctorCheck = diagnostic.DoctorCheck{
	Name:         dnsSourceDoctorCheckName,
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        100,
	Component:    doctorComponentSystem,
	Dependencies: []string{"dns-source-registry"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        dnsSourceDoctorCodes,
	Check:        checkDNSSourceSelections,
}

// checkDNSSourceSelections is the runtime doctor check: the findings over the
// process registry's current state. It reads the live registry, never
// ctx.Tree, because the daemon route loads the config file rather than the
// running configuration.
func checkDNSSourceSelections(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	resolvers := ProcessResolvers()
	return dnsSourceFindings(resolvers.Snapshot(), resolvers.Consumers())
}

// dnsSourceFindings judges every selection, the router's and each consumer's
// published ones. It warns when a list naming received selected no server
// (spec-home-cpe-23b, Integration Checklist), when a received-interface entry
// names an interface that holds no record (D-11), and when a self address is
// served by no consumer's DNS listener (D-6, R-9).
func dnsSourceFindings(snapshot dnssource.Snapshot, consumers []dnssource.Publication) []diagnostic.Diagnostic {
	selections := make([]dnssource.ConsumerSelection, 0, 1+len(consumers))
	// A router selection with no path was never configured: the registry
	// holds no router list to judge yet.
	if snapshot.Router.Path != "" {
		selections = append(selections, snapshot.Router)
	}
	var served []netip.Addr
	for i := range consumers {
		for j := range consumers[i].Selections {
			selections = append(selections, consumers[i].Selections[j])
			served = append(served, consumers[i].Selections[j].Serves...)
		}
	}

	var diags []diagnostic.Diagnostic
	for i := range selections {
		selection := &selections[i]
		if len(selection.Servers) == 0 && slices.Contains(selection.Sources, dnssource.ClassReceived) {
			diags = append(diags, dnsSourceWarning(diagnostic.CodeDoctorDNSSourceEmpty, selection,
				"the list names received and selected no resolver, so this consumer offers none", dnsSourceFixShow))
		}
		for _, name := range selection.ReceivedInterfaces {
			if interfaceHoldsRecord(snapshot.Records, name) {
				continue
			}
			diags = append(diags, dnsSourceWarning(diagnostic.CodeDoctorDNSSourceInterfaceSilent, selection,
				"received-interface "+name+" holds no learned resolver, so the list never selects from it", dnsSourceFixShow))
		}
		for _, self := range selection.Self {
			if slices.Contains(served, self) {
				continue
			}
			diags = append(diags, dnsSourceWarning(diagnostic.CodeDoctorDNSSourceSelfUnserved, selection,
				"hands out "+self.String()+" through self, and no service dns-resolver listener answers on it",
				"set service dns-resolver listener <name> ip "+self.String()))
		}
	}
	return diags
}

// interfaceHoldsRecord reports whether a learned record names the interface.
func interfaceHoldsRecord(records []dnssource.Record, name string) bool {
	for i := range records {
		if records[i].Interface == name {
			return true
		}
	}
	return false
}

// dnsSourceFixShow is the fix of a finding whose cause is which resolvers
// were learned where: show system dns lists every learned record and each
// consumer's selection.
const dnsSourceFixShow = "show system dns"

// dnsSourceWarning builds one finding, naming the list's path and instance,
// with the command that fixes it on its own line (Diagnostic.Fix), never
// inside the message.
func dnsSourceWarning(code string, selection *dnssource.ConsumerSelection, reason, fix string) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str(selection.Path)
	if selection.Instance != "" {
		tb.Str(" (").Str(selection.Instance).Str(")")
	}
	tb.Str(": ").Str(reason)
	return diagnostic.Diagnostic{
		Code:     code,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Path:     selection.Path,
		Fix:      fix,
	}
}
