// Design: docs/architecture/testing/interop.md -- original-session EOR reception.
package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: the actual no-family checker attributes FRR's receive decode to
// the exact sender, family, VRF and original daemon/session, not route arrival.
// PREVENTS: a stale or foreign EOR certifying Ze's implicit-family EOR output.
func TestNoFamilyEORCurrentSessionConsumer(t *testing.T) {
	const startup = "2026/11/04 12:00:00 BGP: [START-00000] bgpd 10.7.1 starting: vty@2605\n"
	const decode = "2026/11/04 12:00:01 BGP: [EOR00-00000] bgp_update_receive_eor: rcvd End-of-RIB for IPv4 Unicast from 172.30.0.2 in vrf default\n"
	for _, test := range []struct {
		name                                   string
		log                                    string
		generation, dropped                    uint64
		changeDaemon, changeSocket, replaceLog bool
		wantOK                                 bool
	}{
		{name: "current decode", log: startup + decode, generation: 1, wantOK: true},
		{name: "route without EOR", log: startup + "2026/11/04 12:00:01 BGP: [ROUTE-00000] 172.30.0.2 rcvd UPDATE w/ attr: nexthop 172.30.0.2\n", generation: 1},
		{name: "address substring", log: startup + strings.ReplaceAll(decode, "172.30.0.2", "172.30.0.20"), generation: 1},
		{name: "address elsewhere", log: startup + strings.Replace(decode, "from 172.30.0.2", "from 172.30.0.9", 1) + "172.30.0.2 connected\n", generation: 1},
		{name: "address embedded elsewhere same line", log: startup + strings.Replace(decode, "from 172.30.0.2", "from 172.30.0.9", 1)[:len(decode)-1] + " note 172.30.0.2\n", generation: 1},
		{name: "wrong VRF", log: startup + strings.Replace(decode, "vrf default", "vrf blue", 1), generation: 1},
		{name: "wrong family", log: startup + strings.Replace(decode, "IPv4 Unicast", "IPv6 Unicast", 1), generation: 1},
		{name: "send direction", log: startup + strings.Replace(decode, "rcvd End-of-RIB", "send End-of-RIB", 1), generation: 1},
		{name: "wrong producer", log: startup + strings.Replace(decode, "bgp_update_receive_eor:", "other_function:", 1), generation: 1},
		{name: "previous daemon EOR", log: decode + startup, generation: 1},
		{name: "lab image build suffix", log: strings.Replace(startup, "bgpd 10.7.1 ", "bgpd 10.7.1_git ", 1) + decode, generation: 1, wantOK: true},
		{name: "another release with a build suffix", log: strings.Replace(startup, "bgpd 10.7.1 ", "bgpd 10.6.0_git ", 1) + decode, generation: 1},
		{name: "replaced session", log: startup + decode, generation: 2, dropped: 1},
		{name: "daemon replaced after decode", log: startup + decode, generation: 1, changeDaemon: true},
		{name: "socket replaced after decode", log: startup + decode, generation: 1, changeSocket: true},
		{name: "log replaced after decode", log: startup + decode, generation: 1, replaceLog: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			lab := &noFamilyEORConsumerLab{log: test.log, generation: test.generation, dropped: test.dropped,
				changeDaemon: test.changeDaemon, changeSocket: test.changeSocket, replaceLog: test.replaceLog, cancel: cancel}
			err := checkNoFamilyEndOfRIB(ctx, &interoplab.CheckContext{Lab: lab,
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}})
			if (err == nil) != test.wantOK {
				t.Fatalf("actual checker error = %v, want success %t (route answers=%d, log answers=%d)", err, test.wantOK, lab.routeReads, lab.logReads)
			}
			if test.wantOK && (lab.routeReads == 0 || lab.logReads == 0) {
				t.Fatalf("checker bypassed route/EOR evidence: route=%d log=%d", lab.routeReads, lab.logReads)
			}
		})
	}
}

// VALIDATES: a window that closes with no decode reports the not-ready
// verdict and FRR's last log, even when the closing deadline cuts the next
// identity read short.
// PREVENTS: the expired context of that cut read being reported as the
// assertion-4 verdict, which hides the log a red must show.
// Method: the lab ends the wait on its second log read and refuses every
// identity read made after that, as a docker exec under a done context does.
func TestNoFamilyEORWindowVerdict(t *testing.T) {
	const routeOnly = "2026/11/04 12:00:00 BGP: [START-00000] bgpd 10.7.1_git starting: vty@2605\n" +
		"2026/11/04 12:00:01 BGP: [ROUTE-00000] 172.30.0.2 rcvd UPDATE w/ attr: nexthop 172.30.0.2\n"
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lab := &noFamilyEORConsumerLab{log: routeOnly, generation: 1, cancel: cancel, honorDeadline: true}
	err := checkNoFamilyEndOfRIB(ctx, &interoplab.CheckContext{Lab: lab,
		Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}})
	if err == nil {
		t.Fatal("a window with no End-of-RIB decode passed")
	}
	if lab.logReads < 2 {
		t.Fatalf("the wait ended before the deadline cut an identity read: log reads=%d", lab.logReads)
	}
	for _, want := range []string{
		"FRR current-session decode of the End-of-RIB marker timed out before the peer became ready",
		"rcvd UPDATE w/ attr: nexthop 172.30.0.2",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("assertion-4 verdict = %v\nwant it to carry %q", err, want)
		}
	}
}

type noFamilyEORConsumerLab struct {
	recordingLab
	log                                    string
	generation, dropped                    uint64
	changeDaemon, changeSocket, replaceLog bool
	// honorDeadline refuses an identity read made under a done context, as
	// a docker exec does once the wait's deadline has passed.
	honorDeadline        bool
	cancel               context.CancelFunc
	logReads, routeReads int
}

func (l *noFamilyEORConsumerLab) Query(ctx context.Context, _ string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	joined := strings.Join(command, " ")
	if l.honorDeadline && joined != cmdCat+" "+frrLogPath && ctx.Err() != nil {
		return "", fmt.Errorf("query %q cut short by its caller's deadline: %w", joined, ctx.Err())
	}
	switch {
	case joined == "cat /tmp/frr.log":
		l.logReads++
		if l.logReads > 1 {
			l.cancel()
		}
		return l.log, nil
	case joined == "cat /var/run/frr/bgpd.pid":
		if l.changeDaemon && l.logReads > 0 {
			return "43\n", nil
		}
		return "42\n", nil
	case strings.HasPrefix(joined, "cat /proc/"):
		// Linux /proc/<pid>/stat: PID, comm, state, then fields 4..22.
		return "42 (bgpd) S 1 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 12345\n", nil
	case strings.HasPrefix(joined, "stat "):
		if l.replaceLog && l.logReads > 0 {
			return "7:999\n", nil
		}
		return "7:123\n", nil
	case strings.Contains(joined, "show bgp neighbor 172.30.0.2 json"):
		port := 42001
		if l.changeSocket && l.logReads > 0 {
			port++
		}
		return fmt.Sprintf(`{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":%d,"connectionsDropped":%d,"portLocal":179,"portForeign":%d,"neighborCapabilities":{"multiprotocolExtensions":{"ipv4Unicast":{"advertised":true}}}}}`, l.generation, l.dropped, port), nil
	case strings.Contains(joined, "show bgp neighbor"):
		return "BGP state = Established\n", nil
	case strings.Contains(joined, "show bgp ipv4 unicast"):
		l.routeReads++
		return `{"prefix":"10.10.0.0/24","paths":[{"valid":true,"bestpath":{"overall":true},"peer":{"peerId":"172.30.0.2","type":"external"}}]}`, nil
	default:
		return "", fmt.Errorf("unexpected no-family consumer query %q", joined)
	}
}
