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
