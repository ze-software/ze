// Design: docs/architecture/api/commands.md -- monitor ping command handler
// Related: ping.go -- batch show ping; this is the live monitor variant
// Related: stream.go -- NewPingSession, the producer every route reads

package cmd

import (
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fieldReplies is the key of the per-probe rows in every ping answer.
const fieldReplies = "replies"

// errPingSessionShort is the one-shot refusal when the session ended before
// every requested probe produced a row: the ICMP socket did not open, or the
// session was canceled. A partial list read as the answer would hide that.
var errPingSessionShort = errors.New("the ping session ended before every probe answered: the daemon could not open its ICMP socket (it needs CAP_NET_RAW) or the request was canceled; the streaming form `ze cli -c \"monitor ping <dest>\"` shows each probe")

// handleMonitorPing is the one-shot answer of `monitor ping`
// (ze-ping:monitor-ping), for the surfaces that cannot show a stream (MCP,
// REST, gRPC, the web admin console). It is one sample of the stream: the rows
// NewPingSession produces for a bounded count (the requested count, else
// defaultPingCount), the same rows the streaming route writes one per line.
func handleMonitorPing(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	mp, err := monitorPingArgsOf(validated)
	if err != nil {
		return monitorPingRefusal(err), nil //nolint:nilerr // operational error in Response
	}
	count := mp.Count
	if count == 0 {
		count = defaultPingCount
	}

	ch, cancel, err := NewPingSession(ctx.Context(), mp.Dest.String(), mp.Interval, mp.Timeout, count, mp.Size)
	if err != nil {
		return monitorPingRefusal(err), nil //nolint:nilerr // operational error in Response
	}
	defer cancel()

	rows := make([]map[string]any, 0, count)
	for row := range ch {
		rows = append(rows, row)
	}
	if len(rows) < count {
		return monitorPingRefusal(errPingSessionShort), nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"destination": mp.Dest.String(),
			fieldReplies:  rows,
		},
	}, nil
}

// monitorPingRefusal is the one-shot refusal: an operational error travels in
// the Response, as every handler of this package answers one.
func monitorPingRefusal(err error) *plugin.Response {
	var tb textbuf.Buffer
	tb.Str("monitor ping: ").Err(err)
	return &plugin.Response{Status: plugin.StatusError, Error: tb.String()}
}

// streamMonitorPing is the streaming route of `monitor ping` (`ze cli -c`,
// SSH exec): it runs NewPingSession on the daemon and writes one line per
// probe until the caller disconnects or the requested count is reached.
func streamMonitorPing(ctx context.Context, _ *pluginserver.Server, w io.Writer, _ string, validated command.ValidatedArgs) error {
	mp, err := monitorPingArgsOf(validated)
	if err != nil {
		return err
	}
	ch, cancel, err := NewPingSession(ctx, mp.Dest.String(), mp.Interval, mp.Timeout, mp.Count, mp.Size)
	if err != nil {
		return err
	}
	defer cancel()

	return writePingRows(w, mp, ch)
}

// writePingRows writes the header and one line per row the session produces,
// the text every terminal route of `monitor ping` shows. It returns when the
// session closes ch, or with the first write error. It MUST be given the
// channel of a session the caller cancels, so a failed write ends the session.
func writePingRows(w io.Writer, mp monitorPingArgs, ch <-chan map[string]any) error {
	var tb textbuf.Buffer
	tb.Str("PING ").Str(mp.Dest.String()).Str(" (Ctrl-C to stop)\n")
	if _, err := w.Write(tb.Bytes()); err != nil {
		return err
	}
	for row := range ch {
		tb.Reset(0)
		seq, _ := row["seq"].(int)
		status, _ := row[fieldStatus].(string)
		tb.Str("  seq=").Int(int64(seq))
		if status == "ok" {
			rtt, _ := row["rtt-ms"].(float64)
			tb.Str("  rtt=").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str("ms\n")
		} else {
			tb.Str("  ").Str(status).Byte('\n')
		}
		if _, err := w.Write(tb.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
