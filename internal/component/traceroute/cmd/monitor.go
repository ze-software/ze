// Design: docs/architecture/api/commands.md -- monitor traceroute command handler
// Related: traceroute.go -- sequential show traceroute; this is the live monitor variant
// Related: stream.go -- NewTracerouteSession, the producer every route reads

package cmd

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// monitorTracerouteRoundGap is the pause between two probe rounds of the
// streaming routes, the cadence the local `ze monitor traceroute` always had.
const monitorTracerouteRoundGap = time.Second

// handleMonitorTraceroute is the RPC handler for `monitor traceroute`
// (ze-traceroute:monitor-traceroute), the one-shot answer for the surfaces
// that cannot show a stream: a single parallel probe round, identical to
// `show probe-round`, run on the leaves the model validated. The terminal
// routes stream rounds through streamMonitorTraceroute, and the TUI views
// drive NewTracerouteSession.
func handleMonitorTraceroute(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	req, err := monitorTracerouteArgsOf(validated)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return probeRoundAnswer(req), nil
}

// streamMonitorTraceroute is the streaming route of `monitor traceroute`
// (`ze cli -c`, SSH exec): it runs probe rounds on the daemon and writes each
// round until the caller disconnects.
func streamMonitorTraceroute(ctx context.Context, _ *pluginserver.Server, w io.Writer, _ string, validated command.ValidatedArgs) error {
	req, err := monitorTracerouteArgsOf(validated)
	if err != nil {
		return err
	}
	return writeTracerouteRounds(ctx, w, req)
}

// writeTracerouteRounds writes the header, then one block per probe round of
// NewTracerouteSession, a round each monitorTracerouteRoundGap, until ctx ends
// (nil) or a write fails (that error). A session that cannot start on the
// first round is that error; on a later round it ends the stream. Every
// terminal route of `monitor traceroute` prints through it.
func writeTracerouteRounds(ctx context.Context, w io.Writer, req tracerouteArgs) error {
	var tb textbuf.Buffer
	tb.Str("traceroute to ").Str(req.target.String()).Str(" (Ctrl-C to stop)\n")
	if _, err := w.Write(tb.Bytes()); err != nil {
		return err
	}

	gap := time.NewTimer(monitorTracerouteRoundGap)
	defer gap.Stop()

	// The loop has no count bound on purpose: it ends when ctx does, which is
	// when the caller disconnects or interrupts.
	for round := 1; ; round++ {
		ch, cancel, sessionErr := NewTracerouteSession(ctx, req.target.String(), req.maxHops)
		if sessionErr != nil {
			if round == 1 {
				return sessionErr
			}
			return nil
		}

		var hops []map[string]any
		for hop := range ch {
			hops = append(hops, hop)
		}
		cancel()

		if ctx.Err() != nil {
			return nil
		}
		if _, err := w.Write(appendTracerouteRound(&tb, round, hops)); err != nil {
			return err
		}

		gap.Reset(monitorTracerouteRoundGap)
		select {
		case <-gap.C:
		case <-ctx.Done():
			return nil
		}
	}
}

// appendTracerouteRound renders one round into tb (reset first) and answers
// its bytes, valid until tb's next use.
func appendTracerouteRound(tb *textbuf.Buffer, round int, hops []map[string]any) []byte {
	tb.Reset(0)
	tb.Str("--- round ").Int(int64(round)).Str(" ---\n")
	for _, hop := range hops {
		ttl, _ := hop[fieldTTL].(int)
		addr, _ := hop["addr"].(string)
		tb.Str("  ").Int(int64(ttl)).Str("  ").Str(addr)
		if rtt, ok := hop["rtt-ms"].(float64); ok {
			tb.Str("  ").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str(" ms")
		}
		tb.Byte('\n')
	}
	return tb.Bytes()
}
