//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay LAC bridge
// Related: bridge_realm_integration_linux_test.go -- the package l2tp_test tests these serve

package l2tp

import (
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"
)

// OpenKernelL2TPSession builds the L2TP half of a LAC session the way
// kernelWorker.setupSession and pppSetupReal do: a kernel tunnel on the LAC's
// listener (genlConn.tunnelCreate), a kernel session in it
// (genlConn.sessionCreate), and the pppol2tp socket connected to that session
// (pppoxCreate). It returns the pppol2tp descriptor. Cleanup closes it, then
// deletes the session and the tunnel. The package l2tp_test realm bridge tests
// use it because the PPPoE package they also drive imports this package.
func OpenKernelL2TPSession(t *testing.T, lacFD int, lns netip.AddrPort, localTID, localSID, remoteTID, remoteSID uint16) int {
	t.Helper()
	g, err := resolveGenlFamily()
	if err != nil {
		t.Fatalf("%v; the kernel needs l2tp_core, l2tp_netlink and l2tp_ppp", err)
	}
	connFD, err := g.tunnelCreate(localTID, remoteTID, lacFD, lns)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := g.tunnelDelete(localTID); err != nil {
			t.Errorf("delete kernel tunnel: %v", err)
		}
		if err := unix.Close(connFD); err != nil {
			t.Errorf("close the tunnel socket: %v", err)
		}
	})
	if err := g.sessionCreate(sessionCreateParams{tunnelID: localTID, localSID: localSID, remoteSID: remoteSID}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// The pppol2tp socket's cleanup, registered below, runs first, and
		// the kernel deletes a session together with its pppol2tp socket
		// (net/l2tp/l2tp_ppp.c pppol2tp_release calls l2tp_session_delete).
		// That delete can complete before or after this request arrives, so
		// both nil and ENODEV (the genl answer for a session it no longer
		// holds) are the expected end state. Any other error is a failure.
		err := g.sessionDelete(localTID, localSID)
		if err == nil {
			return
		}
		if errors.Is(err, unix.ENODEV) {
			return
		}
		t.Errorf("delete kernel session: %v", err)
	})
	pppoxFD, err := pppoxCreate(lacFD, lns, localTID, localSID, remoteTID, remoteSID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(pppoxFD); err != nil {
			t.Errorf("close the pppol2tp socket: %v", err)
		}
	})
	return pppoxFD
}

// BridgeChannels is bridgeChannels, the PPPIOCBRIDGECHAN pppSetupReal issues
// for a relayed call, for the package l2tp_test bridge tests.
func BridgeChannels(l2tpChanFD, pppoxFD int) error { return bridgeChannels(l2tpChanFD, pppoxFD) }
