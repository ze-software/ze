// Design: docs/labs/l2tp-interop.md -- independent kernel address and route evidence.
// Related: scale_kernel.go -- point-to-point JSON boundary.
package l2tp

import (
	"fmt"
	"strings"
	"testing"
)

func TestScaleKernelEvidence(t *testing.T) {
	population := scaleTestPopulation(t, scaleTestSessions()[:1])
	addresses := `[{"ifname":"ppp7","flags":["UP","POINTOPOINT"],"addr_info":[{"family":"inet","local":"10.100.0.1","address":"10.100.0.2","prefixlen":32}]}]`
	routes := `[{"dst":"10.100.0.2","dev":"ppp7"}]`
	links, err := requireScaleKernel([]byte(addresses), []byte(routes), population)
	if err != nil {
		t.Fatal(err)
	}
	if links["scale-1-01"] != "ppp7" {
		t.Fatal("subscriber interface identity lost")
	}
	for _, bad := range []string{`[]`, strings.Replace(addresses, `"address":`, `"peer":`, 1), strings.Replace(addresses, "10.100.0.2", "10.100.0.3", 1)} {
		if _, err := requireScaleKernel([]byte(bad), []byte(routes), population); err == nil {
			t.Fatal("missing or wrong PPP remote accepted")
		}
	}
	if _, err := requireScaleKernel([]byte(addresses), []byte(strings.Replace(routes, "ppp7", "ppp8", 1)), population); err == nil {
		t.Fatal("wrong subscriber route accepted")
	}
}

func TestScaleIPCPEvidence(t *testing.T) {
	session := scaleTestPopulation(t, scaleTestSessions()[:1]).byUser["scale-1-01"]
	log := fmt.Sprintf("sent [IPCP ConfAck id=0x1 <addr %s>]\nrcvd [IPCP ConfAck id=0x2 <addr %s>]\nlocal  IP address %s\nremote IP address %s\n", localPPPAddress, session.address, session.address, localPPPAddress)
	if err := requireScaleIPCP(log, session); err != nil {
		t.Fatal(err)
	}
	if err := requireScaleIPCP(strings.Replace(log, "rcvd [IPCP ConfAck", "rcvd [IPCP ConfNak", 1), session); err == nil {
		t.Fatal("IPCP without acknowledged local address accepted")
	}
}
