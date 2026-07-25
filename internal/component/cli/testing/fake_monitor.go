// Design: ai/rules/pipe-completeness.md -- functional coverage for | log data-transform pipes
// Related: runner.go -- option=monitor:ping=fake wires these fakes into the headless model
//
// fake_monitor.go provides deterministic, offline stand-ins for the monitor
// ping streaming factory and the | resolve / | origin lookups so .et tests can
// exercise the piped monitor render paths without a daemon, ICMP sockets, or
// DNS. The ping channel is pre-filled and closed, so a single poll drains the
// whole session and the test completes deterministically (no sleeps).

package testing

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/command"
)

var errNoFakeRecord = errors.New("no fake record")

// fakeMonitorPTRResolver answers | resolve lookups with fixed PTR names.
type fakeMonitorPTRResolver struct{}

func (fakeMonitorPTRResolver) ResolvePTR(address string) ([]string, error) {
	if address == "192.0.2.1" {
		return []string{"ping-target.test."}, nil
	}
	return nil, errNoFakeRecord
}

// fakeMonitorOriginResolver answers | origin lookups with fixed ASN data.
type fakeMonitorOriginResolver struct{}

func (fakeMonitorOriginResolver) LookupOrigin(_ context.Context, ip string) (command.OriginResult, error) {
	if ip == "192.0.2.1" {
		return command.OriginResult{ASN: 64500, Prefix: "192.0.2.0/24", Name: "TEST-NET-AS"}, nil
	}
	return command.OriginResult{}, errNoFakeRecord
}

// fakePingFactory returns a reply channel pre-filled with three deterministic
// replies (two ok, one timeout) and already closed: the first poll drains the
// session and the monitor stops on its own.
func fakePingFactory(ctx context.Context, _ string, _, _ time.Duration, _, _ int) (<-chan map[string]any, context.CancelFunc, error) {
	ch := make(chan map[string]any, 3)
	ch <- map[string]any{"seq": 0, "status": "ok", "rtt-ms": 1.234}
	ch <- map[string]any{"seq": 1, "status": "ok", "rtt-ms": 2.345}
	ch <- map[string]any{"seq": 2, "status": "timeout"}
	close(ch)
	_, cancel := context.WithCancel(ctx)
	return ch, cancel, nil
}

// wireFakePingMonitor installs the fake ping factory on the model and the
// deterministic resolvers used by | resolve and | origin. The resolvers are
// process-global and intentionally left in place: tests run in parallel in
// one process and the fakes are idempotent.
func wireFakePingMonitor(hm *HeadlessModel) {
	hm.Model().SetPingFactory(fakePingFactory)
	command.SetPTRResolver(fakeMonitorPTRResolver{})
	command.SetOriginResolver(fakeMonitorOriginResolver{})
}
