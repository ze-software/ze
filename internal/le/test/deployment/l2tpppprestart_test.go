package testdeployment

import (
	"strings"
	"testing"
)

func TestLCPRestartProofRejectsEarlyIPCPInEitherDirection(t *testing.T) {
	log := "sent [LCP ConfReq id=0x7 <magic 0x1234>]\n" +
		"rcvd [LCP ConfAck id=0x7 <magic 0x1234>]\n" +
		"rcvd [LCP ConfReq id=0x8 <auth chap MD5>]\n" +
		"sent [LCP ConfAck id=0x8 <auth chap MD5>]\n" +
		"rcvd [CHAP Challenge id=0x9 <abcd>, name = ze]\n" +
		"sent [CHAP Response id=0x9 <abcd>, name = alice]\n" +
		"rcvd [CHAP Success id=0x9]\n" +
		"sent [IPCP ConfReq id=0xa <addr 10.100.0.2>]\n" +
		"rcvd [IPCP ConfAck id=0xa <addr 10.100.0.2>]\n" +
		"rcvd [IPCP ConfReq id=0xb <addr 10.100.0.1>]\n" +
		"sent [IPCP ConfAck id=0xb <addr 10.100.0.1>]\n" +
		"local  IP address " + L2TPPPPPeerAddr + "\n" +
		"remote IP address " + L2TPPPPLocalAddr + "\n"
	if missing := l2tpPPPRestartProgress(log, "chap-md5"); missing != "" {
		t.Fatalf("complete post-authentication exchange rejected: %s", missing)
	}
	for _, direction := range []string{"sent", "rcvd"} {
		t.Run(direction, func(t *testing.T) {
			// A valid later exchange must not hide the earlier phase violation.
			early := direction + " [IPCP ConfReq id=0x4 <addr 10.100.0.1>]\n"
			bad := strings.Replace(log, "rcvd [CHAP Challenge", early+"rcvd [CHAP Challenge", 1)
			if missing := l2tpPPPRestartProgress(bad, "chap-md5"); missing == "" {
				t.Fatalf("proof accepted %s IPCP before fresh authentication", direction)
			}
		})
	}
}

const l2tpPPPTunnelListing = "Tunnel 19329, encap UDP\n" +
	"  From 172.30.0.2 to 172.30.0.1\n" +
	"  Peer tunnel 1702\n" +
	"  UDP source / dest ports: 1702/1701\n"

const l2tpPPPDataListing = "Session 2562 in tunnel 19329\n" +
	"  Peer session 62969, tunnel 1702\n"

const l2tpPPPManagementListing = "Session 0 in tunnel 19329\n" +
	"  Peer session 0, tunnel 1702\n"

// Linux enumerates xl2tpd's zero-ID management context alongside its data
// session. Neither it nor a second data session may supply the injection IDs.
func TestLCPRestartKernelTransportIdentity(t *testing.T) {
	want := l2tpPPPTransport{
		Tunnel: 19329, PeerTunnel: 1702, Session: 2562, PeerSession: 62969,
		Local: "172.30.0.2", Peer: "172.30.0.1", LocalPort: 1702, PeerPort: 1701,
	}
	for _, tt := range []struct {
		name       string
		sessions   string
		management bool
	}{
		{"data-only", l2tpPPPDataListing, false},
		{"management-first", l2tpPPPManagementListing + l2tpPPPDataListing, true},
		{"management-last", l2tpPPPDataListing + l2tpPPPManagementListing, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseL2TPPPPTransport(l2tpSnapshot{tunnel: l2tpPPPTunnelListing, session: tt.sessions})
			expected := want
			expected.Management = tt.management
			if err != nil || got != expected {
				t.Fatalf("live transport = %+v, %v; want %+v", got, err, expected)
			}
			packet := l2tpPPPRestartDatagram(got)
			if packet[12] != 0x06 || packet[13] != 0xa6 || packet[14] != 0xf5 || packet[15] != 0xf9 {
				t.Fatalf("injected packet does not address the live recipient: %x", packet)
			}
		})
	}
}

func TestLCPRestartRejectsAmbiguousKernelState(t *testing.T) {
	for name, sessions := range map[string]string{
		"empty":                  "",
		"management-only":        l2tpPPPManagementListing,
		"two-data":               l2tpPPPDataListing + strings.Replace(l2tpPPPDataListing, "2562", "2563", 1),
		"duplicate-management":   l2tpPPPDataListing + l2tpPPPManagementListing + l2tpPPPManagementListing,
		"wrong-data-tunnel":      strings.Replace(l2tpPPPDataListing, "19329", "19330", 1),
		"wrong-data-peer-tunnel": strings.Replace(l2tpPPPDataListing, "1702", "1703", 1),
		"wrong-management-tunnel": l2tpPPPDataListing +
			strings.Replace(l2tpPPPManagementListing, "19329", "19330", 1),
		"wrong-management-peer-tunnel": l2tpPPPDataListing +
			strings.Replace(l2tpPPPManagementListing, "1702", "1703", 1),
		"zero-local-data": strings.Replace(l2tpPPPDataListing, "2562", "0", 1),
		"zero-peer-data":  strings.Replace(l2tpPPPDataListing, "62969", "0", 1),
		"sequenced":       l2tpPPPDataListing + "  sequence numbering: send\n",
		"malformed":       l2tpPPPDataListing + "Session broken\n",
		"indented-extra":  l2tpPPPDataListing + "  " + l2tpPPPManagementListing,
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := parseL2TPPPPTransport(l2tpSnapshot{tunnel: l2tpPPPTunnelListing, session: sessions}); err == nil {
				t.Fatalf("accepted ambiguous kernel state: %+v", got)
			}
		})
	}
	for name, tunnel := range map[string]string{
		"empty":       "",
		"two-tunnels": l2tpPPPTunnelListing + l2tpPPPTunnelListing,
		"zero-id":     strings.Replace(l2tpPPPTunnelListing, "Tunnel 19329", "Tunnel 0", 1),
		"zero-port":   strings.Replace(l2tpPPPTunnelListing, "1702/1701", "0/1701", 1),
		"unspecified": strings.Replace(l2tpPPPTunnelListing, "172.30.0.2", "0.0.0.0", 1),
		"malformed":   "Tunnel broken\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := parseL2TPPPPTransport(l2tpSnapshot{tunnel: tunnel, session: l2tpPPPDataListing}); err == nil {
				t.Fatalf("accepted invalid tunnel: %+v", got)
			}
		})
	}
}

func TestRejectedPPPRequiresDataSessionRemoval(t *testing.T) {
	for _, tt := range []struct {
		name     string
		sessions string
		clean    bool
		wantErr  bool
	}{
		{"empty", "", true, false},
		{"management-only", l2tpPPPManagementListing, true, false},
		{"data-only", l2tpPPPDataListing, false, false},
		{"data-and-management", l2tpPPPDataListing + l2tpPPPManagementListing, false, false},
		{"duplicate-management", l2tpPPPManagementListing + l2tpPPPManagementListing, false, true},
		{"wrong-tunnel", strings.Replace(l2tpPPPManagementListing, "19329", "19330", 1), false, true},
		{"one-zero-id", strings.Replace(l2tpPPPDataListing, "2562", "0", 1), false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clean, err := l2tpPPPRejectedSessions(l2tpSnapshot{tunnel: l2tpPPPTunnelListing, session: tt.sessions}, l2tpSnapshot{})
			if clean != tt.clean || (err != nil) != tt.wantErr {
				t.Fatalf("rejected session cleanup = %t, %v; want clean=%t, error=%t", clean, err, tt.clean, tt.wantErr)
			}
		})
	}
}
