//go:build integration && linux

// VALIDATES: both trafficusage programs answer TCX_NEXT on every path, so the
// hook goes on to the next TCX program and to the legacy clsact filters; a
// clsact filter on a link traffic-usage counts still runs, on ingress and on
// egress.
// PREVENTS: traffic-usage turning off the traffic-control ingress policer, the
// flow-export sFlow sampler and the interface mirror on every link it attaches
// to, which a TCX_PASS verdict does (sch_handle_ingress, sch_handle_egress).

package trafficusage

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// TestProgram_ReturnsTCXNext runs each program, with per-IP accounting on and
// off, over a frame it counts and a frame it does not parse, through
// BPF_PROG_TEST_RUN, and reads the verdict.
//
// MUTATION: return 0 (TCX_PASS) from ret0: every row goes red.
func TestProgram_ReturnsTCXNext(t *testing.T) {
	counted := ethIPv4(protoUDP, [4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, udpHdr(4000, 53))
	notIPv4 := make([]byte, 64)
	copy(notIPv4[12:14], []byte{0x08, 0x06}) // ARP
	for _, trackIP := range []bool{true, false} {
		coll := loadColl(t, trackIP)
		for _, name := range []string{progIngress, progEgress} {
			for _, frame := range []struct {
				what string
				pkt  []byte
			}{{"a counted UDP frame", counted}, {"an ARP frame", notIPv4}} {
				ret, _, err := coll.Programs[name].Test(frame.pkt)
				if err != nil {
					t.Fatalf("%s, track-ip %v, %s: BPF_PROG_TEST_RUN: %v", name, trackIP, frame.what, err)
				}
				if got := tcxVerdict(ret); got != tcxNext {
					t.Errorf("%s, track-ip %v, %s: verdict %d, want %d (TCX_NEXT)", name, trackIP, frame.what, got, tcxNext)
				}
			}
		}
		coll.Close()
	}
}

// TestProgram_LetsAClsactFilterRun attaches traffic-usage to one end of a veth
// pair, puts a clsact matchall filter that mirrors every frame to a second pair
// on the same hook, and sends a frame through it. The mirror is the same filter
// shape the interface mirror installs (mirrorFilter, iface/netlink/mirror_linux.go).
//
// Ingress: a frame sent out tuv3b reaches tuv3a's ingress hook, where the
// trafficusage program runs first and the clsact filter after it; the mirror
// copies it out tuv4a, so it arrives on tuv4b only when the filter ran. Egress:
// a frame sent out tuv3a meets the egress program, then the egress filter, the
// same way.
//
// MUTATION: return 0 (TCX_PASS) from ret0: sch_handle_ingress and
// sch_handle_egress skip tc_run, no frame reaches tuv4b, and both legs go red.
func TestProgram_LetsAClsactFilterRun(t *testing.T) {
	requireBPF(t)
	countedIndex := createVethPair(t, "tuv3a", "tuv3b")
	mirrorIndex := createVethPair(t, "tuv4a", "tuv4b")

	att, err := tcxAttacher{}.Attach(countedIndex, "tuv3a", 1024, false)
	if err != nil {
		t.Fatalf("AttachTCX: %v", err)
	}
	defer func() {
		if err := att.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	qdisc := &netlink.Clsact{QdiscAttrs: netlink.QdiscAttrs{
		LinkIndex: countedIndex,
		Handle:    netlink.MakeHandle(0xffff, 0),
		Parent:    netlink.HANDLE_CLSACT,
	}}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		t.Fatalf("add the clsact qdisc to tuv3a: %v", err)
	}
	defer func() {
		if err := netlink.QdiscDel(qdisc); err != nil && !errors.Is(err, unix.ENOENT) {
			t.Errorf("remove the clsact qdisc from tuv3a: %v", err)
		}
	}()

	for _, leg := range []struct {
		name   string
		parent uint32
		sendOn string
		dst    [4]byte
	}{
		{name: "ingress", parent: netlink.HANDLE_MIN_INGRESS, sendOn: "tuv3b", dst: [4]byte{10, 9, 3, 1}},
		{name: "egress", parent: netlink.HANDLE_MIN_EGRESS, sendOn: "tuv3a", dst: [4]byte{10, 9, 3, 2}},
	} {
		t.Run(leg.name, func(t *testing.T) {
			filter := &netlink.MatchAll{
				FilterAttrs: netlink.FilterAttrs{
					LinkIndex: countedIndex,
					Parent:    leg.parent,
					Priority:  1,
					Protocol:  unix.ETH_P_ALL,
				},
				Actions: []netlink.Action{&netlink.MirredAction{
					ActionAttrs:  netlink.ActionAttrs{Action: netlink.TC_ACT_PIPE},
					MirredAction: netlink.TCA_EGRESS_MIRROR,
					Ifindex:      mirrorIndex,
				}},
			}
			if err := netlink.FilterAdd(filter); err != nil {
				t.Fatalf("add the %s mirror filter to tuv3a: %v", leg.name, err)
			}
			defer func() {
				if err := netlink.FilterDel(filter); err != nil {
					t.Errorf("remove the %s mirror filter: %v", leg.name, err)
				}
			}()

			peer := packetReceiver(t, "tuv4b")
			defer func() {
				if err := unix.Close(peer); err != nil {
					t.Errorf("close the tuv4b receiver: %v", err)
				}
			}()
			frame := ethIPv4(protoUDP, [4]byte{10, 9, 3, 9}, leg.dst, udpHdr(4000, 9))
			injectFrame(t, leg.sendOn, frame)
			awaitFrames(t, peer, [][]byte{frame})
		})
	}
}
