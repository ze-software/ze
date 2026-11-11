// Design: docs/architecture/testing/interop.md -- original-session EOR reception.
package bgp

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestNoFamilyEORCanceledIdentityRetainsObservation checks both identity fences
// around the log read; TestNoFamilyEORWindowVerdict cuts only the second. An
// identity query canceled by the wait's end is no completed peer observation:
// it must not replace the last complete startup-only log.
func TestNoFamilyEORCanceledIdentityRetainsObservation(t *testing.T) {
	const startup = "2026/11/10 15:53:35 BGP: [T83RR-8SM5G] bgpd 10.7.1 starting: vty@2605, bgp@<all>:179\n"
	const complete = "first complete observation has no End-of-RIB\n"
	const incomplete = "later log read has no completed identity fence\n"
	for _, test := range []struct {
		name     string
		cutAtPID int
		logs     int
	}{
		{name: "identity before log", cutAtPID: 4, logs: 1},
		{name: "identity after log", cutAtPID: 5, logs: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			base := &noFamilyEORConsumerLab{log: startup + complete, generation: 1, cancel: func() {}}
			lab := &noFamilyEORCutIdentityLab{noFamilyEORConsumerLab: base, cancel: cancel,
				cutAtPID: test.cutAtPID, incompleteLog: startup + incomplete}
			err := checkNoFamilyEndOfRIB(ctx, &interoplab.CheckContext{Lab: lab,
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}})
			if err == nil {
				t.Fatal("checker accepted a session whose complete observations contain no End-of-RIB")
			}
			if lab.pidReads != test.cutAtPID || base.logReads != test.logs || base.routeReads == 0 {
				t.Fatalf("identity cut missed intended probe: PID=%d logs=%d routes=%d; want PID=%d logs=%d and route evidence",
					lab.pidReads, base.logReads, base.routeReads, test.cutAtPID, test.logs)
			}
			message := err.Error()
			if !strings.Contains(message, "scenario no-family-peer-eor-frr assertion 4: wait for FRR current-session decode of the End-of-RIB marker") {
				t.Fatalf("canceled identity query replaced the EOR wait failure: %v", err)
			}
			if !strings.Contains(message, startup+complete) {
				t.Fatalf("EOR wait lost its last complete startup-only log: %v", err)
			}
			if strings.Contains(message, incomplete) {
				t.Fatalf("EOR wait reported a log whose identity fence never completed: %v", err)
			}
		})
	}
}

type noFamilyEORCutIdentityLab struct {
	*noFamilyEORConsumerLab
	cancel        context.CancelFunc
	cutAtPID      int
	pidReads      int
	incompleteLog string
}

func (l *noFamilyEORCutIdentityLab) Query(ctx context.Context, peer string, command []string, environment []interoplab.EnvironmentVariable) (string, error) {
	joined := strings.Join(command, " ")
	if joined == "cat /var/run/frr/bgpd.pid" {
		l.pidReads++
		if l.pidReads == l.cutAtPID {
			l.cancel()
			return "", ctx.Err()
		}
	}
	if joined == "cat /tmp/frr.log" && l.logReads > 0 {
		l.log = l.incompleteLog
	}
	return l.noFamilyEORConsumerLab.Query(ctx, peer, command, environment)
}
