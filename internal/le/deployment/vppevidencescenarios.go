// Design: docs/architecture/testing/interop.md -- the eight real VPP proof scenarios
// Overview: vppevidence.go -- the run that drives these configurations
//
// The constants and configurations are the data shared with effective-vpp.py.
// The parity test compares every value and every written file until step 14.
package deployment

import "github.com/ze-software/ze/internal/core/textbuf"

// The eight producer scenarios, in execution order.
const (
	VPPScenarioIPsec             = "ipsec"
	VPPScenarioIPv4FIB           = "ipv4-fib"
	VPPScenarioMPLSFIB           = "mpls-fib"
	VPPScenarioTrafficInterface  = "traffic-interface-class"
	VPPScenarioTrafficProtocol   = "traffic-protocol-class"
	VPPScenarioTrafficDSCP       = "traffic-dscp-class"
	VPPScenarioTrafficMultiClass = "traffic-multi-class"
	VPPScenarioFirewall          = "firewall-acl"
)

// Values both the producer and the Go runner send to ze and VPP.
const (
	VPPFIBPrefix                    = "10.20.0.0/24"
	VPPNextHop                      = "10.0.0.1"
	VPPMPLSPrefix                   = "10.30.0.0/24"
	VPPMPLSLabel                    = 100
	VPPTrafficPolicerClass          = "default"
	VPPTrafficProtocolClass         = "tcp"
	VPPTrafficProtocolNumber        = 6
	VPPTrafficDSCPClass             = "cs6"
	VPPTrafficDSCPValue             = 48
	VPPTrafficMultiClassA           = "web"
	VPPTrafficMultiProtocolA        = 6
	VPPTrafficMultiClassB           = "dns"
	VPPTrafficMultiProtocolB        = 17
	VPPIPsecReportPrefix            = "ze-vpp-ipsec:"
	VPPIPsecSPI              uint64 = 0x11223344
	VPPIPsecInboundSPI       uint64 = 0x55667788
	VPPIPsecSalt                    = "0xdeadbeef"
	VPPIPsecCipherKey               = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" //nolint:gosec // G101: a public fixed test vector
	VPPFirewallACLTag               = "ze/wan/input"
)

const vppExternalConfig = `vpp {
    enabled true;
    external true;
    api-socket /run/vpp/api.sock;
    stats { socket-path /run/vpp/stats.sock; }
}
`

func vppFIBConfig(mpls bool) string {
	var tb textbuf.Buffer
	if mpls {
		tb.Str(`bgp {
    peer peer1 {
        connection {
            remote { ip 127.0.0.1; }
            local  { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 1; remote 1; }
            router-id 1.2.3.4;
            family {
                ipv4/unicast { prefix { maximum 10000; } }
                ipv4/mpls-label { prefix { maximum 10000; } }
            }
            capability { graceful-restart disable; }
        }
        behavior { group-updates disable; }
    }
}

`)
	} else {
		tb.Str(`bgp {
    peer peer1 {
        connection {
            remote { ip 127.0.0.1; }
            local  { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 1; remote 1; }
            router-id 1.2.3.4;
            family { ipv4/unicast { prefix { maximum 10000; } } }
            capability { graceful-restart disable; }
        }
        behavior { group-updates disable; }
    }
}

`)
	}
	tb.Str(vppExternalConfig).Str(`fib {
    vpp { enabled true; }
}
`)
	return tb.String()
}

func vppTrafficConfig(iface, kind string, withInterface bool) string {
	var tb textbuf.Buffer
	tb.Str(vppExternalConfig).Byte('\n').Str("traffic {\n    control {\n        backend vpp;\n")
	if withInterface {
		tb.Str("        interface ").Str(iface).Str(" {\n            qdisc {\n                type htb;\n")
		switch kind {
		case VPPScenarioTrafficInterface:
			tb.Str("                default-class ").Str(VPPTrafficPolicerClass).Str(";\n").
				Str("                class ").Str(VPPTrafficPolicerClass).Str(` {
                    rate 1mbit;
                    ceil 2mbit;
                }
`)
		case VPPScenarioTrafficProtocol:
			tb.Str("                default-class ").Str(VPPTrafficProtocolClass).Str(";\n").
				Str("                class ").Str(VPPTrafficProtocolClass).Str(` {
                    rate 1mbit;
                    ceil 2mbit;
                    match protocol { value `).Int(VPPTrafficProtocolNumber).Str(`; }
                }
`)
		case VPPScenarioTrafficDSCP:
			tb.Str("                default-class ").Str(VPPTrafficDSCPClass).Str(";\n").
				Str("                class ").Str(VPPTrafficDSCPClass).Str(` {
                    rate 1mbit;
                    ceil 2mbit;
                    match dscp { value `).Int(VPPTrafficDSCPValue).Str(`; }
                }
`)
		case VPPScenarioTrafficMultiClass:
			tb.Str("                class ").Str(VPPTrafficMultiClassA).Str(` {
                    rate 10mbit;
                    ceil 100mbit;
                    match protocol { value `).Int(VPPTrafficMultiProtocolA).Str(`; }
                }
`).Str("                class ").Str(VPPTrafficMultiClassB).Str(` {
                    rate 1mbit;
                    ceil 100mbit;
                    match protocol { value `).Int(VPPTrafficMultiProtocolB).Str(`; }
                }
`)
		}
		tb.Str("            }\n        }\n")
	}
	tb.Str("    }\n}\n")
	return tb.String()
}

func vppFirewallConfig(withRules bool) string {
	var tb textbuf.Buffer
	tb.Str(vppExternalConfig).Byte('\n').Str("firewall {\n    backend vpp;\n")
	if withRules {
		tb.Str(`    table wan {
        family inet;
        chain input {
            type filter;
            hook input;
            priority 0;
            policy drop;
            term allow-ssh {
                from {
                    protocol tcp;
                    destination-port 22;
                }
                then {
                    accept;
                }
            }
            term drop-all {
                then {
                    drop;
                }
            }
        }
    }
`)
	}
	tb.Str("}\n")
	return tb.String()
}

func vppPolicerName(iface, class string) string {
	var tb textbuf.Buffer
	return tb.Str("ze/").Str(iface).Byte('/').Str(class).String()
}
