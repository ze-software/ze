// Design: docs/architecture/l2tp/bng-5-pppoe.md -- AC policy to PPP driver.
//go:build linux

package pppoe

import (
	"encoding/binary"
	"log/slog"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestPPPoEAuthAllowReachesDriver proves a configured PAP-only policy reaches
// handlePADR's production StartSession: the driver must advertise PAP, not its
// implicit CHAP-first default. It also checks the unchanged PPPoE MRU ceiling.
func TestPPPoEAuthAllowReachesDriver(t *testing.T) {
	p := extractFromConfigText(t, "pppoe { auth-method pap; authentication-allow [pap]; interface eth0 {} }")
	driver := ppp.NewProductionDriver(slog.Default(), mruCeilingBackend{})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	channel := socketPair(t)
	unit := socketPair(t)
	key := CookieKey{}
	hwAddr := [EthALen]byte{2, 0, 0, 0, 0, 1}
	srcMAC := [EthALen]byte{2, 0, 0, 0, 0, 2}
	srv := &InterfaceServer{
		ifName: "eth0", hwAddr: hwAddr, sessions: newSessionTable("eth0", 8),
		cookieKey: key, logger: slog.Default(), pppDriver: driver,
		sendFrameFn: func([]byte) {},
		pppoeCreateFn: func(string, uint16, [EthALen]byte) (int, error) {
			return unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
		},
		devPPPSetupFn: func(int) (int, int, int, error) {
			return channel[0], unit[0], 1, nil
		},
	}
	applyInterfaceConfig(srv, &p.Interfaces[0], &p)
	srv.handlePADR(&Packet{Code: CodePADR, SrcMAC: srcMAC, Tags: []Tag{
		{Type: TagACCookie, Value: GenerateCookie(key, hwAddr[:], srcMAC[:], nil)},
		{Type: TagServiceName},
	}})
	request := readLCP(t, channel[1], ppp.LCPConfigureRequest)
	opts, err := ppp.ParseLCPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, opt := range opts {
		if opt.Type == ppp.LCPOptAuthProto {
			if len(opt.Data) != 2 {
				t.Fatalf("PAP option = %x", opt.Data)
			}
			if got := binary.BigEndian.Uint16(opt.Data); got != ppp.ProtoPAP {
				t.Fatalf("Auth-Protocol = %#x, want PAP", got)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no Auth-Protocol in AC Configure-Request")
	}
	if got, ok := mruOf(t, request.Data); !ok || got != PPPoEMaxMTU {
		t.Fatalf("MRU = %d (present %t), want %d", got, ok, PPPoEMaxMTU)
	}
	// Reconfigure the server after StartSession has transferred its policy.
	// An MRU-only Nak makes that existing session produce another request,
	// which must retain PAP rather than the new server generation's CHAP.
	next := extractFromConfigText(t, "pppoe { auth-method chap-md5; interface eth0 {} }")
	applyInterfaceConfig(srv, &next.Interfaces[0], &next)
	options := []byte{ppp.LCPOptMRU, 4, 0x05, 0xc8}
	var frame [64]byte
	n := ppp.WriteFrame(frame[:], 0, ppp.ProtoLCP, nil)
	n += ppp.WriteLCPPacket(frame[:], n, ppp.LCPConfigureNak, request.Identifier, options)
	if _, err := unix.Write(channel[1], frame[:n]); err != nil {
		t.Fatal(err)
	}
	retry := readLCP(t, channel[1], ppp.LCPConfigureRequest)
	retryOpts, err := ppp.ParseLCPOptions(retry.Data)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, opt := range retryOpts {
		if opt.Type == ppp.LCPOptAuthProto {
			if len(opt.Data) != 2 {
				t.Fatalf("existing session auth option changed: %x", opt.Data)
			}
			if binary.BigEndian.Uint16(opt.Data) != ppp.ProtoPAP {
				t.Fatalf("existing session adopted reloaded policy: %x", opt.Data)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("existing session lost its original authentication policy")
	}
}
