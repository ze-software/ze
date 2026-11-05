// Design: docs/labs/l2tp-interop.md -- real kernel proxy MRU enforcement proof.
// Related: internal/test/fixture/tunnel_fixture_l2tp_proxy_mru.go -- registered LAC fixture.

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const proxyMRUFixture = "l2tp/proxy-mru-transmit-cap"

// checkProxyMRU reuses the compiled functional fixture inside the daemon's
// kernel namespace. The fixture drives real proxy AVPs, CHAP, IPCP, kernel
// interface measurement and an ICMP round trip before removing each session.
func checkProxyMRU(ctx context.Context, lab labOperations, timeout time.Duration) error {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for _, peerMRU := range []string{"1500", "9000", "0"} {
		result, err := lab.Exec(probeCtx, peerZe,
			[]string{"le", "test", "fixture", proxyMRUFixture, "1701", peerMRU}, nil)
		if err != nil {
			return interoplab.WithSearchedOutput(fmt.Errorf("accepted proxy MRU %s fixture: %w", peerMRU, err), result.Stdout+"\n"+result.Stderr)
		}
		if result.ExitCode != 0 {
			return interoplab.WithSearchedOutput(fmt.Errorf("accepted proxy MRU %s fixture exited %d", peerMRU, result.ExitCode), result.Stdout+"\n"+result.Stderr)
		}
		if err := proxyMRUFixtureEvidence(result.Stdout, peerMRU); err != nil {
			return interoplab.WithSearchedOutput(err, result.Stdout+"\n"+result.Stderr)
		}
		if _, err := fmt.Fprintln(os.Stdout, "Proxy MRU fixture evidence:\n"+result.Stdout); err != nil {
			return fmt.Errorf("retain proxy MRU fixture evidence: %w", err)
		}
	}
	return nil
}

// A successful process exit alone can be a usage page or the wrong helper.
// Require the registered fixture's measured-state and packet assertions too.
func proxyMRUFixtureEvidence(output, peerMRU string) error {
	for _, expected := range []string{
		"OK: CHAP Success received",
		"OK: IPCP opened, ze assigned ",
		"OK: proxy peer MRU " + peerMRU + " installed kernel MTU 1492 on ",
		"OK: proxy peer MRU " + peerMRU + " passed an ICMP round trip with 1492 Information-field octets",
		"OK: proxy PPP interface removed after StopCCN",
	} {
		if !strings.Contains(output, expected) {
			return fmt.Errorf("accepted proxy MRU %s fixture lacks %q", peerMRU, expected)
		}
	}
	if strings.Contains(output, "ZE-OBSERVER-FAIL") {
		return errors.New("accepted proxy fixture reported an observer failure")
	}
	return nil
}
