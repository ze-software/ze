// Design: docs/architecture/api/process-protocol.md -- accepted loser results.
package redistribute

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type retainedResultDispatcher struct {
	fakeDispatcher
	output *rpc.UpdateRouteOutput
	cause  error
}

func (d *retainedResultDispatcher) UpdateRouteResult(context.Context, string, string) (*rpc.UpdateRouteOutput, error) {
	return d.output, d.cause
}

// Useful retained admission must not become a no-peer warning, while missing
// results, explicit refusal, real transport errors and actual no-peer answers do.
func TestBGPConsumerRetainedResultIsNotNoPeerFailure(t *testing.T) {
	retained := rpc.RouteAdmission{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Destination: "192.0.2.7", PeerOwner: 17,
		Generation: 3, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}
	refused := retained
	refused.Generation = 0
	refused.Outcome = rpc.RouteAdmissionRefused
	refused.Disposition = rpc.RouteAdmissionDispositionUnspecified
	refused.Reason = "origin denied by admission"
	for _, test := range []struct {
		name   string
		output *rpc.UpdateRouteOutput
		cause  error
		want   string
		noPeer bool
	}{
		{name: "retained", output: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{retained}}},
		{name: "refused", output: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{refused}}, want: refused.Reason},
		{name: "missing", want: "update-route result is missing"},
		{name: "transport", cause: errors.New("origin transport unavailable"), want: "origin transport unavailable"},
		{name: "zero-with-warning", output: &rpc.UpdateRouteOutput{Warnings: []string{"family not negotiated"}}, want: "family not negotiated", noPeer: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := logger
			t.Cleanup(func() { logger = original })
			var buffer bytes.Buffer
			capture := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
			logger = func() *slog.Logger { return capture }
			consumer := NewBGPConsumer(&retainedResultDispatcher{output: test.output, cause: test.cause})
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "10.0.0.0/24"})
			got := buffer.String()
			if test.want == "" {
				if got != "" {
					t.Fatalf("retained origin was reported as failure: %s", got)
				}
			} else if !strings.Contains(got, test.want) {
				t.Fatalf("failure reason missing: %s", got)
			}
			if strings.Contains(got, "reached no BGP peer") != test.noPeer {
				t.Fatalf("no-peer classification changed: %s", got)
			}
		})
	}
}
