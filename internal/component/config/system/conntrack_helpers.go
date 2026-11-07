// Design: docs/guide/configuration.md -- conntrack helper assignment
// Related: conntrack.go -- the module leaf-list this table answers for
// Related: conntrack_helpers_test.go -- ties this table to the YANG enumeration

package system

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConntrackHelperFamily says which address families the kernel helper
// registers. The kernel registers an IPv4-only helper (AF_INET) for some
// protocols, and a rule assigning it in an IPv6 path would name an object with
// no IPv6 helper behind it, so the family decides the nft table the
// assignment goes in.
type ConntrackHelperFamily uint8

const (
	conntrackHelperFamilyUnspecified ConntrackHelperFamily = iota
	// ConntrackHelperBothFamilies is a helper registered for IPv4 and IPv6
	// (NFPROTO_UNSPEC, or one helper per family).
	ConntrackHelperBothFamilies
	// ConntrackHelperIPv4Only is a helper registered for AF_INET alone.
	ConntrackHelperIPv4Only
)

// ConntrackHelperRule is one kernel helper of a configured helper module, with
// the transport and the ports it is assigned on.
//
// Ze's assignment rule is the only place the port lives: since Linux 6.0 no
// helper module carries a ports parameter and nf_ct_helper_init takes none, so
// the kernel never assigns a helper by port on its own.
type ConntrackHelperRule struct {
	KernelHelper string   // the name nf_ct_helper_init registers ("ftp", "RAS", "Q.931")
	Protocol     string   // "tcp" or "udp"
	Ports        []uint16 // destination ports, from VyOS module_map or the IANA registry
}

// ConntrackHelper is one value of `system conntrack module`.
type ConntrackHelper struct {
	Name   string
	Family ConntrackHelperFamily
	Rules  []ConntrackHelperRule
}

// The transports a kernel helper registers for, and the one helper whose
// operator name and kernel name are used for two rules.
const (
	conntrackTCP = "tcp"
	conntrackUDP = "udp"
	conntrackSIP = "sip"
)

// conntrackHelpers is the single declaration of what each helper module
// assigns. The YANG enumeration at system/conntrack/module is checked against
// it (TestHelperTableMatchesModel), and the nft lowering, the guide and
// `show system conntrack` read it.
//
// Kernel helper names, families and transports are read from Linux v7.3,
// the nf_ct_helper_init calls in net/netfilter/nf_conntrack_*.c. Ports for
// ftp, h323, pptp, sip and tftp come from VyOS src/conf_mode/system_conntrack.py
// module_map; the rest from the IANA service registry.
var conntrackHelpers = []ConntrackHelper{
	{Name: "amanda", Family: ConntrackHelperBothFamilies, Rules: []ConntrackHelperRule{
		{KernelHelper: "amanda", Protocol: conntrackUDP, Ports: []uint16{10080}},
	}},
	{Name: "ftp", Family: ConntrackHelperBothFamilies, Rules: []ConntrackHelperRule{
		{KernelHelper: "ftp", Protocol: conntrackTCP, Ports: []uint16{21}},
	}},
	// H.245 is reached through the expectation Q.931 opens, never by port.
	{Name: "h323", Family: ConntrackHelperBothFamilies, Rules: []ConntrackHelperRule{
		{KernelHelper: "RAS", Protocol: conntrackUDP, Ports: []uint16{1719}},
		{KernelHelper: "Q.931", Protocol: conntrackTCP, Ports: []uint16{1720}},
	}},
	{Name: "irc", Family: ConntrackHelperIPv4Only, Rules: []ConntrackHelperRule{
		{KernelHelper: "irc", Protocol: conntrackTCP, Ports: []uint16{6667}},
	}},
	{Name: "netbios-ns", Family: ConntrackHelperIPv4Only, Rules: []ConntrackHelperRule{
		{KernelHelper: "netbios-ns", Protocol: conntrackUDP, Ports: []uint16{137}},
	}},
	{Name: "pptp", Family: ConntrackHelperIPv4Only, Rules: []ConntrackHelperRule{
		{KernelHelper: "pptp", Protocol: conntrackTCP, Ports: []uint16{1723}},
	}},
	{Name: "sane", Family: ConntrackHelperBothFamilies, Rules: []ConntrackHelperRule{
		{KernelHelper: "sane", Protocol: conntrackTCP, Ports: []uint16{6566}},
	}},
	// 5061 is SIP over TLS, which the helper cannot read; kept for VyOS parity.
	{Name: conntrackSIP, Family: ConntrackHelperBothFamilies, Rules: []ConntrackHelperRule{
		{KernelHelper: conntrackSIP, Protocol: conntrackUDP, Ports: []uint16{5060, 5061}},
		{KernelHelper: conntrackSIP, Protocol: conntrackTCP, Ports: []uint16{5060, 5061}},
	}},
	{Name: "snmp", Family: ConntrackHelperIPv4Only, Rules: []ConntrackHelperRule{
		{KernelHelper: "snmp", Protocol: conntrackUDP, Ports: []uint16{161}},
	}},
	{Name: "tftp", Family: ConntrackHelperBothFamilies, Rules: []ConntrackHelperRule{
		{KernelHelper: "tftp", Protocol: conntrackUDP, Ports: []uint16{69}},
	}},
}

// ConntrackHelperByName answers the declaration of the helper module name, and
// an error naming the valid helpers when no row declares it. The returned
// value shares its Rules with the table, which no caller may modify.
func ConntrackHelperByName(name string) (ConntrackHelper, error) {
	i := slices.IndexFunc(conntrackHelpers, func(h ConntrackHelper) bool { return h.Name == name })
	if i < 0 {
		return ConntrackHelper{}, fmt.Errorf("conntrack: no helper declaration for module %q (valid: %s)",
			name, conntrackHelperNames())
	}
	return conntrackHelpers[i], nil
}

// conntrackHelperNames lists the declared helper names, for error text.
func conntrackHelperNames() string {
	names := make([]string, len(conntrackHelpers))
	for i := range conntrackHelpers {
		names[i] = conntrackHelpers[i].Name
	}
	return textbuf.Join(names, ", ")
}
