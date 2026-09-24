//go:build integration && linux

package flowspecfirewall

import (
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft"
)

// RFC requirement: RFC8955-5.1-2 positive -- the kernel applies a more-specific FlowSpec before an overlapping covering rule (§5.1).
// RFC requirement: RFC8955-5.1-2 negative -- inserting the covering discard first cannot shadow the more-specific terminal rule (§5.1).
// RFC requirement: RFC8955-7.3-2 positive -- T-set continues and applies a later matching discard, while T-clear stops (§7.3).
// RFC requirement: RFC8955-7.3-2 negative -- a continuing marking action cannot change the header used by a later DSCP predicate (§7.3).
func TestSelectedFlowSpecKernelPacketSemantics(t *testing.T) {
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("network namespace unavailable: %v", err)
	}
	namespace, err := netns.New()
	if err != nil {
		_ = original.Close()
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_SYS_ADMIN and CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables("flowspec", nil)
		_ = firewall.CloseBackend()
		if err := netns.Set(original); err != nil {
			t.Errorf("restore network namespace: %v", err)
		}
		_ = namespace.Close()
		_ = original.Close()
		runtime.UnlockOSThread()
	})
	lo, err := netlink.LinkByName("lo")
	require.NoError(t, err)
	require.NoError(t, netlink.LinkSetUp(lo))
	require.NoError(t, firewall.LoadBackend("nft"))
	family.RegisterTestFamilies()

	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = receiver.Close() })
	raw, err := receiver.SyscallConn()
	require.NoError(t, err)
	var sockerr error
	require.NoError(t, raw.Control(func(fd uintptr) { sockerr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_RECVTOS, 1) }))
	require.NoError(t, sockerr)
	sender, err := net.DialUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: receiver.LocalAddr().(*net.UDPAddr).Port}, receiver.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sender.Close() })
	port := receiver.LocalAddr().(*net.UDPAddr).Port
	components := []byte{3, 0x81, 17, 4, 0x91, byte(port >> 8), byte(port)}
	narrow := append([]byte{13, 1, 32, 127, 0, 0, 1}, components...)
	broad := append([]byte{10, 1, 8, 127}, components...)
	drop := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
	mark := []byte{0x80, 9, 0, 0, 0, 0, 0, 10}
	cont := []byte{0x80, 7, 0, 0, 0, 0, 0, 1}
	b := testBridge(t)
	t.Cleanup(func() { b.stopped = true })
	install := func(nlri, communities []byte) {
		b.handleSelected(&ribevents.FlowSpecChange{Family: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, NLRI: nlri, ExtendedCommunities: communities})
	}
	withdraw := func(nlri []byte) {
		b.handleSelected(&ribevents.FlowSpecChange{Family: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, NLRI: nlri, Withdraw: true})
	}
	probe := func(wantReceive bool, dscp byte) {
		t.Helper()
		_, err := sender.Write([]byte{0x42})
		require.NoError(t, err)
		require.NoError(t, receiver.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
		var data [8]byte
		var ancillary [128]byte
		n, oobn, _, _, err := receiver.ReadMsgUDP(data[:], ancillary[:])
		if !wantReceive {
			var timeout net.Error
			require.ErrorAs(t, err, &timeout)
			require.True(t, timeout.Timeout())
			return
		}
		require.NoError(t, err)
		require.Equal(t, []byte{0x42}, data[:n])
		messages, err := unix.ParseSocketControlMessage(ancillary[:oobn])
		require.NoError(t, err)
		for _, msg := range messages {
			if msg.Header.Level == unix.IPPROTO_IP && msg.Header.Type == unix.IP_TOS {
				require.Equal(t, dscp, msg.Data[0]>>2)
				return
			}
		}
		t.Fatal("kernel supplied no received TOS control message")
	}

	probe(true, 0)
	install(broad, drop)
	probe(false, 0)
	install(narrow, mark) // T clear: specific terminal marking wins.
	probe(true, 10)
	install(narrow, append(append([]byte(nil), mark...), cont...))
	probe(false, 0) // T set: the covering discard also applies.

	withdraw(broad)
	// The original packet's DSCP is zero. Marking to ten must not make this
	// later DSCP=zero rule cease matching during the matching phase.
	broadDSCP := append(append([]byte(nil), broad...), 11, 0x81, 0)
	broadDSCP[0] += 3
	install(broadDSCP, drop)
	probe(false, 0)
	withdraw(broadDSCP)
	probe(true, 10)
	withdraw(narrow)
	probe(true, 0)

	// A positive packet rate permits conforming traffic and drops excess.
	// Sending the complete burst before reading avoids counting receive
	// deadline waits as token replenishment.
	install(narrow, []byte{0x80, 0x0c, 0, 0, 0x3f, 0x80, 0, 0, 0x80, 7, 0, 0, 0, 0, 0, 2}) // one packet/second, sample
	for range 32 {
		_, err := sender.Write([]byte{0x42})
		require.NoError(t, err)
	}
	require.NoError(t, receiver.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	received := 0
	for {
		var data [8]byte
		_, _, err := receiver.ReadFromUDP(data[:])
		if err != nil {
			var timeout net.Error
			require.ErrorAs(t, err, &timeout)
			require.True(t, timeout.Timeout())
			break
		}
		received++
	}
	require.Greater(t, received, 0, "the rate is not a discard-all action")
	require.Less(t, received, 32, "excess packets must not fall through to accept")
	counters, err := firewall.GetBackend().GetCounters(tableName)
	require.NoError(t, err)
	var sampled uint64
	for _, chain := range counters {
		for _, term := range chain.Terms {
			if strings.HasSuffix(term.Name, "-sample") {
				sampled += term.Packets
			}
		}
	}
	require.Equal(t, uint64(32), sampled, "both port alternatives match, but every packet is sampled once, including excess packets")
	withdraw(narrow)
	probe(true, 0)

	// A restarted bridge owns no selected route yet. It must remove a rule
	// that survived the previous process, even when replay is empty.
	install(narrow, drop)
	probe(false, 0)
	require.NoError(t, firewall.CloseBackend())
	require.NoError(t, firewall.LoadBackend("nft"))
	probe(false, 0)
	require.NoError(t, clearStaleRules())
	probe(true, 0)
}
