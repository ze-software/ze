// MEASURES: what a REAL VPP does with a cleartext packet that arrives on an
// interface whose SPD holds inbound PROTECT, DISCARD and BYPASS entries, and with
// ESP the inbound SA decrypts. This test only installs that SPD and SA through this
// backend and asserts nothing about VPP's verdict; the deployment run
// (internal/le/test/deployment/vppevidencecleartext.go, `./le test deployment vpp`)
// then injects the packets with VPP's packet generator, reads VPP's own counters
// and trace, and holds them to RFC 4301 Section 5.2. The SA's SPI, tunnel ends and
// key are twins of the vppDecrypt constants there: change both together.
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
)

// The selectors of the cleartext probe. Their twins, which the deployment run builds
// its frames from, are the vppCleartext constants in
// internal/le/test/deployment/vppevidencecleartext.go: change both together.
//
// 100.64.45.0/24 is the probe interface's own subnet (the local side). Each inbound
// entry names a different remote /24, so no frame matches two entries and the
// priorities decide nothing.
const (
	realVPPCleartextSPI     = uint32(0x45450001)
	realVPPCleartextLocal   = "100.64.45.0/24"
	realVPPCleartextProtect = "100.64.46.0/24"
	realVPPCleartextDiscard = "100.64.47.0/24"
	realVPPCleartextBypass  = "100.64.48.0/24"
	realVPPCleartextPrefix  = "ze-vpp-cleartext:"
)

var (
	realVPPCleartextTunnelLocal  = net.ParseIP("100.64.45.1")
	realVPPCleartextTunnelRemote = net.ParseIP("100.64.46.1")
)

// TestVPPRealCleartextProbe installs, on the interface ZE_VPP_IPSEC_SW_IF_INDEX
// names, one inbound tunnel-mode SA and three inbound SPD entries: PROTECT for
// 100.64.46.0/24, DISCARD for 100.64.47.0/24 and BYPASS for 100.64.48.0/24, each
// toward 100.64.45.0/24. The selectors are written the way the IKE engine writes an
// inbound policy (childPolicy, ike/engine/child.go): Src is the packet's source, the
// remote side.
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

	encKey := make([]byte, 36)
	for i := range encKey {
		encKey[i] = byte(0x40 + i)
	}
	sa := SAParams{
		SPI:       realVPPCleartextSPI,
		Src:       realVPPCleartextTunnelRemote,
		Dst:       realVPPCleartextTunnelLocal,
		Dir:       SADirIn,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReplayWin: 32,
		EncAlgo:   algoAES256GCM,
		EncKey:    encKey,
		IsAEAD:    true,
	}
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA inbound against a real VPP: %v", err)
	}

	local := realVPPCleartextNet(t, realVPPCleartextLocal)
	protect := SPParams{
		Src:       realVPPCleartextNet(t, realVPPCleartextProtect),
		Dst:       local,
		Dir:       SADirIn,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		IfIndex:   swIfIndex,
		Action:    SPActionProtect,
		Priority:  PriorityChildSA,
		TunnelSrc: realVPPCleartextTunnelRemote,
		TunnelDst: realVPPCleartextTunnelLocal,
		SAID:      realVPPCleartextSPI,
	}
	if err := b.InstallPolicy(protect); err != nil {
		t.Fatalf("InstallPolicy inbound protect against a real VPP: %v", err)
	}
	discard := SPParams{
		Src:      realVPPCleartextNet(t, realVPPCleartextDiscard),
		Dst:      local,
		Dir:      SADirIn,
		IfIndex:  swIfIndex,
		Action:   SPActionDiscard,
		Priority: testBypassPriority,
	}
	if err := b.InstallPolicy(discard); err != nil {
		t.Fatalf("InstallPolicy inbound discard against a real VPP: %v", err)
	}
	bypass := SPParams{
		Src:      realVPPCleartextNet(t, realVPPCleartextBypass),
		Dst:      local,
		Dir:      SADirIn,
		IfIndex:  swIfIndex,
		Action:   SPActionBypass,
		Priority: testBypassPriority,
	}
	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("InstallPolicy inbound bypass against a real VPP: %v", err)
	}
	fmt.Printf("%s spd-id=%d\n", realVPPCleartextPrefix, b.spdID)
}

func realVPPCleartextNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR %s: %v", cidr, err)
	}
	return network
}
