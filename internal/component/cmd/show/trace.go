// Design: docs/architecture/diagnostics/production-diagnostics.md -- flight-recorder trace inspection and on-demand capture
// Related: show.go -- RPC registration for show-system-trace and request-system-trace-snapshot

package show

import (
	"encoding/base64"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	argTraceID          = "id"
	keyTraceCapturedAt  = "captured-at"
	keyTraceTrigger     = "trigger"
	keyTraceDetail      = "detail"
	keyTraceBytes       = "bytes"
	traceFormatGoBase64 = "go-trace-base64"
)

func handleShowSystemTrace(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	id := extractTraceID(ctx, validated)
	if id != "" {
		return showSystemTraceSnapshot(id)
	}

	st := flightrec.Status()
	snaps := flightrec.Snapshots()
	rows := make([]map[string]any, 0, len(snaps))
	for i := range snaps {
		row := map[string]any{
			argTraceID:         snaps[i].ID,
			keyTraceCapturedAt: snaps[i].CapturedAt.UTC().Format(time.RFC3339),
			keyTraceTrigger:    snaps[i].Trigger,
			keyTraceBytes:      snaps[i].Bytes,
		}
		if snaps[i].Detail != "" {
			row[keyTraceDetail] = snaps[i].Detail
		}
		rows = append(rows, row)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"enabled":             st.Enabled,
			"min-age":             st.MinAge.String(),
			"max-bytes":           st.MaxBytes,
			"snapshot-count":      len(rows),
			"max-snapshots":       st.MaxSnapshots,
			"auto-capture-errors": st.AutoCaptureErrors,
			"snapshots":           rows,
		},
	}, nil
}

func showSystemTraceSnapshot(id string) (*plugin.Response, error) {
	snap, ok := flightrec.SnapshotByID(id)
	if !ok {
		return plugin.OperatorErrorResponse(opererr.New(
			"trace snapshot not found: "+id,
			"no captured flight-recorder snapshot has that identifier",
			"ze show system trace",
		)), nil
	}

	data := plugin.Map{
		argTraceID:         snap.Meta.ID,
		keyTraceCapturedAt: snap.Meta.CapturedAt.UTC().Format(time.RFC3339),
		keyTraceTrigger:    snap.Meta.Trigger,
		keyTraceBytes:      snap.Meta.Bytes,
		keyFormat:          traceFormatGoBase64,
		keyData:            base64.StdEncoding.EncodeToString(snap.Data),
	}
	if snap.Meta.Detail != "" {
		data[keyTraceDetail] = snap.Meta.Detail
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}

func handleRequestSystemTraceSnapshot(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	meta, err := flightrec.CaptureNow("operator-request", "manual snapshot")
	if err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(
			err,
			"cannot capture trace snapshot",
			"the daemon flight recorder is not enabled or could not write a snapshot",
			"ze show system trace",
		)), nil //nolint:nilerr // operator error in Response
	}

	data := plugin.Map{
		argTraceID:         meta.ID,
		keyTraceCapturedAt: meta.CapturedAt.UTC().Format(time.RFC3339),
		keyTraceTrigger:    meta.Trigger,
		keyTraceBytes:      meta.Bytes,
	}
	if meta.Detail != "" {
		data[keyTraceDetail] = meta.Detail
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}

func extractTraceID(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) string {
	if ctx != nil {
		if id := ctx.Selector(argTraceID); id != "" {
			return id
		}
	}
	args := validated.Tokens()
	for i := range args {
		if args[i] == argTraceID && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
