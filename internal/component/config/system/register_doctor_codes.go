// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/config/system.

package system

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorDNSSourceEmpty is the doctor diagnostic code "doctor-dns-source-empty".
	CodeDoctorDNSSourceEmpty = "doctor-dns-source-empty"
	// CodeDoctorDNSSourceInterfaceSilent is the doctor diagnostic code "doctor-dns-source-interface-silent".
	CodeDoctorDNSSourceInterfaceSilent = "doctor-dns-source-interface-silent"
	// CodeDoctorDNSSourceSelfUnserved is the doctor diagnostic code "doctor-dns-source-self-unserved".
	CodeDoctorDNSSourceSelfUnserved = "doctor-dns-source-self-unserved"
	exampleDoctorJSON               = "ze doctor --json"
	exampleShowDoctor               = "ze show doctor"
	exampleShowSystemDNS            = "ze show system dns"
)

// codeConntrackProcfs names a netfilter sysctl tree that is absent or not
// writable. internal/core/diagnostic/codes.go declares it, so `ze show diagnostic
// doctor-conntrack-procfs` answers.
const codeConntrackProcfs = "doctor-conntrack-procfs"

// codeUpdateHistoryPersistFailed names a failure to persist the self-update
// history ring to the daemon's zefs store under meta/config/update-history.
const codeUpdateHistoryPersistFailed = "doctor-update-history-persist-failed"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        CodeDoctorDNSSourceEmpty,
		Title:       "A dns-source list selected no resolver",
		Description: "A consumer's dns-source list (written, or its built-in default) names received, and the list selected no server, so that consumer offers no DNS resolver: no lease, router advertisement or PPP session has supplied one, and no later class in the list holds a usable server. Run show system dns to see the records and each selection. Add a fallback, for example set system name-server <address> and the source configured, or check why no interface learned a resolver.",
		Examples:    []string{exampleShowDoctor, exampleShowSystemDNS},
	},
	{
		Code:        CodeDoctorDNSSourceInterfaceSilent,
		Title:       "A received-interface holds no learned resolver",
		Description: "A dns-source list narrows received to interfaces named under received-interface, and one of them holds no learned resolver record, so the list never selects from it. The interface may not have learned yet (no DHCP lease, no RA, no PPP session), or the name is wrong. Run show system dns to see which interfaces hold records, then correct the received-interface entry or bring the interface up.",
		Examples:    []string{exampleShowDoctor, exampleShowSystemDNS},
	},
	{
		Code:        CodeDoctorDNSSourceSelfUnserved,
		Title:       "A self DNS server is not served",
		Description: "A consumer hands Ze's own address to clients through the self source of its dns-source list (for a DHCP subnet, Ze's IPv4 address inside the subnet on the interface that serves it, which can differ from a configured server identifier; for an RA unit, the first global or unique-local IPv6 address Ze holds on the advertising unit, else the one a live prefix delegation gave it, never its link-local source; for an L2TP pool, the LNS's local IPCP address on the subscriber's PPP link), and no service dns-resolver listener answers on that address, so clients time out. Add a listener on that address, for example set service dns-resolver listener lan ip <address>, or remove self from that consumer's dns-source list.",
		Examples:    []string{exampleShowDoctor, exampleShowSystemDNS},
	},
	{
		Code:        codeConntrackProcfs,
		Title:       "Conntrack procfs unavailable",
		Description: "Conntrack tuning is configured but /proc/sys/net/netfilter/nf_conntrack_max is unavailable or not writable.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-conntrack-procfs"},
	},
	{
		Code:        codeSystemHostNotApplied,
		Title:       "Kernel host name differs from system host",
		Description: "On the appliance, Ze sets the kernel host name from system host at start and on every reload. The kernel still carries another name, so setting it failed: the warn line in the log names the error and the fix.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-system-host-not-applied"},
	},
	{
		Code:        codeUpdateCheckUnreachable,
		Title:       "Update check URL unreachable",
		Description: "The configured firmware update-check URL did not respond to an HTTP HEAD probe within the timeout.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-update-check-unreachable"},
	},
	{
		Code:        codeUpdateHistoryPersistFailed,
		Title:       "Update history failed to persist",
		Description: "The self-updater could not write the update event history to the daemon store under meta/config/update-history, so recent update events stay in memory and will not survive a daemon restart. Check store health and disk space with ze show data check.",
		Examples:    []string{exampleShowDoctor, "ze show data check", "ze show diagnostic doctor-update-history-persist-failed"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: system: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeUpdateHistoryPersistFailed}); err != nil {
		panic("BUG: system: doctor event code registration refused: " + err.Error())
	}
}
