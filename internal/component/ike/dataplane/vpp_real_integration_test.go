// VALIDATES: this backend against a REAL VPP. It connects to a running VPP's binary
// API socket, installs one AEAD SA and two policies, and reports the ids VPP gave
// them. internal/le/deployment/vppevidence.go runs it inside a privileged container and
// then asserts through vppctl that VPP holds what was asked for (make
// ze-deployment-vpp-test). It is the spec's AC-7.
// PREVENTS: a green unit suite standing in for a working backend. Every unit test
// here agrees with the generated binapi by construction, and the divergence that
// made this backend inoperable was between the message and what VPP accepts. Only a
// running VPP can see across that boundary (ai/rules/testing.md).

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

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ipsec"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// The socket of the VPP to program, and the VPP interface index to bind the SPD to.
// Both are absent outside the deployment run, and the test skips.
const (
	realVPPSocketEnv    = "ZE_VPP_IPSEC_API_SOCKET"
	realVPPIfIndexEnv   = "ZE_VPP_IPSEC_SW_IF_INDEX"
	realVPPReportPrefix = "ze-vpp-ipsec:"
)

// The values the vppctl assertions look for. The salt is the last four octets of the
// AEAD key material, so a backend that sent the whole 36 octets as the key, or a zero
// salt, reports a different number.
var (
	realVPPSPI        = uint32(0x11223344)
	realVPPInboundSPI = uint32(0x55667788)
	realVPPSalt       = []byte{0xde, 0xad, 0xbe, 0xef}
	realVPPLocal      = net.ParseIP("192.0.2.1")
	realVPPRemote     = net.ParseIP("198.51.100.1")
)

// TestVPPRealDataplaneInstalls programs a running VPP.
//
// It builds the backend directly rather than through newVPPBackend, because that
// reads the connector the VPP component publishes while it runs (GetActiveConnector,
// component/vpp/vpp.go) and no VPP component runs here. Everything below that seam is
// the shipped path.
func TestVPPRealDataplaneInstalls(t *testing.T) {
	socket := os.Getenv(realVPPSocketEnv)
	if socket == "" {
		t.Skipf("%s is unset: this test programs a running VPP (`./le deployment vpp-test`)", realVPPSocketEnv)
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
	// Connector.Close returns nothing (component/vpp/conn.go), so there is
	// no error to assert here. The assertion below it, on the backend's own Close,
	// stays.
	defer conn.Close()
	ch, err := conn.NewChannel()
	if err != nil {
		t.Fatalf("VPP api channel: %v", err)
	}
	// The CLEANUP half runs FIRST, and against the same VPP, so the state it leaves
	// behind is none. Everything the install half programs below must survive this
	// process, because the vppctl assertions in effective-vpp.py read VPP after it
	// exits.
	realVPPCloseRemovesWhatItInstalled(t, conn, swIfIndex)

	b := &vppBackend{conn: conn, ch: ch}
	// the deferred `b.Close()` assertion that stood here is MOVED, not
	// dropped. Close now DELETES the SAs and the SPD this backend installed, so
	// closing here would leave the AC-7 assertions nothing to read: they run against
	// VPP after this process exits. realVPPCloseRemovesWhatItInstalled above asserts
	// the same Close error AND, additionally, that VPP no longer holds what it
	// removed. A ze daemon closes its backend on the way out; this probe deliberately
	// does not.

	// One AEAD SA. AES-GCM-256 KEYMAT is the 32 octet cipher key followed by the four
	// octet salt (RFC 4106 Section 8.1), and VPP takes the two in separate fields.
	encKey := make([]byte, 32, 36)
	for i := range encKey {
		encKey[i] = byte(i)
	}
	sa := SAParams{
		SPI:       realVPPSPI,
		Src:       realVPPLocal,
		Dst:       realVPPRemote,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReplayWin: 32,
		EncAlgo:   algoAES256GCM,
		EncKey:    append(encKey, realVPPSalt...),
		IsAEAD:    true,
	}
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA against a real VPP: %v", err)
	}
	identity, err := saIdentityOf(sa.SPI, sa.Dst, sa.Proto)
	if err != nil {
		t.Fatalf("saIdentityOf: %v", err)
	}
	fmt.Printf("%s sad-id=%d\n", realVPPReportPrefix, b.sadIDs[identity])

	// The INBOUND half of the same pair, so `show ipsec sa` reports what VPP does with
	// IPSEC_API_SAD_FLAG_IS_INBOUND (vppSAFlags, vpp.go). Its SPI is this node's own
	// choice, and its endpoints are the outbound pair reversed.
	inbound := sa
	inbound.SPI = realVPPInboundSPI
	inbound.Src, inbound.Dst = realVPPRemote, realVPPLocal
	inbound.Dir = SADirIn
	if err := b.InstallSA(inbound); err != nil {
		t.Fatalf("InstallSA inbound against a real VPP: %v", err)
	}

	// Two OUTBOUND policies whose Ze priorities differ. VPP holds both in one chain,
	// so the order `show ipsec spd` prints them in is what the SPD sort order IS. This
	// run is the measurement vppPriority (vpp_policy.go) cites: the chain is DESCENDING,
	// which is what its negation assumes.
	_, protectSrc, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	_, protectDst, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	protect := SPParams{
		Src:       protectSrc,
		Dst:       protectDst,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		IfIndex:   swIfIndex,
		Action:    SPActionProtect,
		Priority:  PriorityChildSA,
		TunnelSrc: realVPPLocal,
		TunnelDst: realVPPRemote,
		SAID:      realVPPSPI,
	}
	if err := b.InstallPolicy(protect); err != nil {
		t.Fatalf("InstallPolicy protect against a real VPP: %v", err)
	}

	_, bypassSrc, err := net.ParseCIDR("203.0.113.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	_, bypassDst, err := net.ParseCIDR("203.0.113.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	bypass := SPParams{
		Src:        bypassSrc,
		Dst:        bypassDst,
		Dir:        SADirOut,
		IfIndex:    swIfIndex,
		Action:     SPActionBypass,
		Priority:   PriorityIKEBypass,
		UpperProto: 17, // UDP
		DstPort:    ExactPortMatch(500),
	}
	if err := b.InstallPolicy(bypass); err != nil {
		t.Fatalf("InstallPolicy bypass against a real VPP: %v", err)
	}
	fmt.Printf("%s spd-id=%d\n", realVPPReportPrefix, b.spdID)
}

// The SPI of the SA the cleanup half installs. It differs from both SPIs above, so
// the two halves never read each other's state.
const realVPPClosedSPI = uint32(0x0badcafe)

// realVPPCloseRemovesWhatItInstalled proves against a REAL VPP that Close deletes the
// SAD entry and the SPD this backend created, and unbinds the SPD from the interface.
//
// Nothing in VPP expires either one. Before this, a ze that exited left the SAs of a
// dead IKE session installed and its SPD still bound to the interface, enforcing
// PROTECT entries that named those SAs, and the next run stepped over all of it
// (firstFreeSadID, freeSPDID). Whether VPP ACCEPTS the removals, and in this order,
// is a property of VPP rather than of this code, so only a running VPP settles it.
//
// The readback uses a SECOND channel on the same connection, because Close closes the
// backend's own.
func realVPPCloseRemovesWhatItInstalled(t *testing.T, conn *vppcomp.Connector, swIfIndex int) {
	t.Helper()

	ch, err := conn.NewChannel()
	if err != nil {
		t.Fatalf("VPP api channel for the cleanup half: %v", err)
	}
	b := &vppBackend{conn: conn, ch: ch}

	sa := SAParams{
		SPI:       realVPPClosedSPI,
		Src:       realVPPLocal,
		Dst:       realVPPRemote,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReplayWin: 32,
		EncAlgo:   "aes256",
		EncKey:    make([]byte, 32),
		AuthAlgo:  "sha256",
		AuthKey:   make([]byte, 32),
	}
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("cleanup half InstallSA: %v", err)
	}
	_, protectSrc, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	_, protectDst, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	if err := b.InstallPolicy(SPParams{
		Src:       protectSrc,
		Dst:       protectDst,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		IfIndex:   swIfIndex,
		Action:    SPActionProtect,
		Priority:  PriorityChildSA,
		TunnelSrc: realVPPLocal,
		TunnelDst: realVPPRemote,
		SAID:      realVPPClosedSPI,
	}); err != nil {
		t.Fatalf("cleanup half InstallPolicy: %v", err)
	}
	spdID := b.spdID

	// The reader channel is opened BEFORE Close, so the assertions below cannot be
	// answered by a connection Close took down.
	reader, err := conn.NewChannel()
	if err != nil {
		t.Fatalf("VPP api channel for the readback: %v", err)
	}
	if !realVPPHoldsSPI(t, reader, realVPPClosedSPI) {
		t.Fatalf("VPP does not hold the SA spi=%#x this half just installed, so its absence after Close proves nothing", realVPPClosedSPI)
	}
	if !realVPPHoldsSPD(t, reader, spdID) {
		t.Fatalf("VPP does not hold SPD %d this half just created, so its absence after Close proves nothing", spdID)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("close backend against a real VPP: %v", err)
	}
	if realVPPHoldsSPI(t, reader, realVPPClosedSPI) {
		t.Errorf("VPP still holds SA spi=%#x after Close: a restarted ze would leave a dead session's SA installed", realVPPClosedSPI)
	}
	if realVPPHoldsSPD(t, reader, spdID) {
		t.Errorf("VPP still holds SPD %d after Close, bound to interface %d and enforcing entries that name a deleted SA", spdID, swIfIndex)
	}
	fmt.Printf("%s close-removed-spi=%#x\n", realVPPReportPrefix, realVPPClosedSPI)
	fmt.Printf("%s close-removed-spd-id=%d\n", realVPPReportPrefix, spdID)
}

func realVPPHoldsSPI(t *testing.T, ch api.Channel, spi uint32) bool {
	t.Helper()
	req := ch.SendMultiRequest(&ipsec.IpsecSaV3Dump{})
	for {
		details := &ipsec.IpsecSaV3Details{}
		stop, err := req.ReceiveReply(details)
		if err != nil {
			t.Fatalf("ipsec sa dump: %v", err)
		}
		if stop {
			return false
		}
		if details.Entry.Spi == spi {
			return true
		}
	}
}

func realVPPHoldsSPD(t *testing.T, ch api.Channel, spdID uint32) bool {
	t.Helper()
	req := ch.SendMultiRequest(&ipsec.IpsecSpdsDump{})
	for {
		details := &ipsec.IpsecSpdsDetails{}
		stop, err := req.ReceiveReply(details)
		if err != nil {
			t.Fatalf("ipsec spds dump: %v", err)
		}
		if stop {
			return false
		}
		if details.SpdID == spdID {
			return true
		}
	}
}
