// MEASURES: what a REAL VPP does with a cleartext packet that arrives on an
// interface whose SPD holds inbound PROTECT, DISCARD and BYPASS entries, and with
// ESP the inbound SA decrypts. This test only installs that SPD and SA through this
// backend and asserts nothing about VPP's verdict; the deployment run
// (internal/le/test/deployment/vppevidencecleartext.go, `./le test deployment vpp`)
// then injects the packets with VPP's packet generator, reads VPP's own counters
// and trace, and holds them to RFC 4301 Section 5.2. Both read the SA and the
// selectors from internal/test/vppcleartext, so neither holds a copy.
// PREVENTS: an RFC 4301 Section 5.2 claim about VPP inbound processing that rests on
// a reading of VPP's source alone.

//go:build ze_vpp && integration

package dataplane

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	"github.com/ze-software/ze/internal/test/vppcleartext"
)

// TestVPPRealCleartextProbe installs, on the interface ZE_VPP_IPSEC_SW_IF_INDEX
// names, one inbound tunnel-mode SA and three inbound SPD entries: PROTECT for
// 100.64.46.0/24, DISCARD for 100.64.47.0/24 and BYPASS for 100.64.48.0/24, each
// toward 100.64.45.0/24 (internal/test/vppcleartext). The selectors are written the
// way the IKE engine writes an inbound policy (childPolicy, ike/engine/child.go): Src
// is the packet's source, the remote side. DISCARD and BYPASS take testOperatorOrder,
// as an operator entry ranked ahead of the Child SA's would; their remote ranges are
// disjoint, so the rank decides nothing between them.
//
// The PROTECT entry also brings the backend's four IKE BYPASS entries for its local
// tunnel end, 100.64.45.1, UDP 500 and 4500 (ensureIKEBypass). VPP's inbound ESP
// lookup tries BYPASS before DISCARD and compares addresses alone, so the deployment
// run aims the ESP frame the DISCARD entry is owed at another host of
// 100.64.45.0/24.
//
// It leaves everything installed and prints the SPD id, because the measurement runs
// against VPP after this process exits. It builds the backend directly, as
// TestVPPRealDataplaneInstalls does and for the same reason.
func TestVPPRealCleartextProbe(t *testing.T) {
	socket := os.Getenv(realVPPSocketEnv)
	if socket == "" {
		t.Skipf("%s is unset: this probe programs a running VPP (`./le test deployment vpp`)", realVPPSocketEnv)
	}
	swIfIndex, err := strconv.Atoi(os.Getenv(realVPPIfIndexEnv))
	if err != nil {
		t.Fatalf("%s must be the VPP interface index to bind the SPD to: %v", realVPPIfIndexEnv, err)
	}

	conn := vppcomp.NewConnector(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := conn.Connect(ctx, 10, time.Second); err != nil {
		t.Fatalf("connect to VPP at %s: %v", socket, err)
	}
	defer conn.Close()
	ch, err := conn.NewChannel()
	if err != nil {
		t.Fatalf("VPP api channel: %v", err)
	}
	// Close is deliberately not called: it would delete the SPD the measurement reads.
	b := &vppBackend{conn: conn, ch: ch}

	encKey := vppcleartext.Key()
	tunnelRemote, tunnelLocal := net.ParseIP(vppcleartext.TunnelRemote), net.ParseIP(vppcleartext.Host)
	sa := SAParams{
		SPI:       vppcleartext.SPI,
		Src:       tunnelRemote,
		Dst:       tunnelLocal,
		Dir:       SADirIn,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReplayWin: 32,
		EncAlgo:   algoAES256GCM,
		EncKey:    encKey[:],
		IsAEAD:    true,
	}
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA inbound against a real VPP: %v", err)
	}

	local := realVPPCleartextNet(t, vppcleartext.Local)
	protect := SPParams{
		Src:       realVPPCleartextNet(t, vppcleartext.Protect),
		Dst:       local,
		Dir:       SADirIn,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		IfIndex:   swIfIndex,
		Action:    SPActionProtect,
		Priority:  PriorityChildSA,
		TunnelSrc: tunnelRemote,
		TunnelDst: tunnelLocal,
		SAID:      vppcleartext.SPI,
	}
	if err := b.InstallPolicy(protect); err != nil {
		t.Fatalf("InstallPolicy inbound protect against a real VPP: %v", err)
	}
	discard := SPParams{
		Src:      realVPPCleartextNet(t, vppcleartext.Discard),
		Dst:      local,
		Dir:      SADirIn,
		IfIndex:  swIfIndex,
		Action:   SPActionDiscard,
		Priority: testOperatorOrder,
	}
	if err := b.InstallPolicy(discard); err != nil {
		t.Fatalf("InstallPolicy inbound discard against a real VPP: %v", err)
	}
	bypass := SPParams{
		Src:      realVPPCleartextNet(t, vppcleartext.Bypass),
		Dst:      local,
		Dir:      SADirIn,
		IfIndex:  swIfIndex,
		Action:   SPActionBypass,
		Priority: testOperatorOrder,
	}
	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("InstallPolicy inbound bypass against a real VPP: %v", err)
	}
	fmt.Printf("%s spd-id=%d\n", vppcleartext.ReportPrefix, b.spdID)
}

func realVPPCleartextNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR %s: %v", cidr, err)
	}
	return network
}
