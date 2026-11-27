// Design: docs/architecture/diagnostics/production-diagnostics.md -- functional fixture for request/show system trace
// Related: register_system_trace_show.go -- fixture registration
// Related: plugin_fixture_16.go -- sibling show system observer fixtures

package fixture

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func pluginSystemTrace(ctx context.Context, p *sdk.Plugin) error {
	snap, err := plugin16DoneMap(ctx, p, "request system trace snapshot", "request-system-trace-snapshot")
	if err != nil {
		return err
	}
	snapID, ok := snap["id"].(string)
	if !ok {
		return fmt.Errorf("request-system-trace-snapshot: id is %T, want string", snap["id"])
	}
	if snapID == "" {
		return fmt.Errorf("request-system-trace-snapshot: empty id in %v", snap)
	}

	list, err := plugin16DoneMap(ctx, p, "show system trace", "show-system-trace")
	if err != nil {
		return err
	}
	enabled, ok := list["enabled"].(bool)
	if !ok {
		return fmt.Errorf("show-system-trace: enabled is %T, want bool", list["enabled"])
	}
	if !enabled {
		return fmt.Errorf("show-system-trace: enabled=false, want true")
	}
	snapshots, ok := list["snapshots"].([]any)
	if !ok {
		return fmt.Errorf("show-system-trace: snapshots is %T, want array", list["snapshots"])
	}
	found := false
	for _, entry := range snapshots {
		row, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if row["id"] == snapID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("show-system-trace: snapshots %v missing captured id %q", snapshots, snapID)
	}

	var tb textbuf.Buffer
	fetchCmd := tb.Reset().Str("show system trace id ").Str(snapID).String()
	fetch, err := plugin16DoneMap(ctx, p, fetchCmd, "show-system-trace-id")
	if err != nil {
		return err
	}
	if fetch["format"] != "go-trace-base64" {
		return fmt.Errorf("show-system-trace-id: format=%v, want go-trace-base64", fetch["format"])
	}
	encoded, ok := fetch["data"].(string)
	if !ok {
		return fmt.Errorf("show-system-trace-id: data is %T, want string", fetch["data"])
	}
	if encoded == "" {
		return fmt.Errorf("show-system-trace-id: data is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("show-system-trace-id: invalid base64 data: %w", err)
	}
	if len(decoded) == 0 {
		return fmt.Errorf("show-system-trace-id: decoded trace payload is empty")
	}

	tb.Reset().Str("OK: system-trace id=").Str(snapID).Str(" bytes=").Int(int64(len(decoded))).Byte('\n')
	_ = tb.StdErr() //nolint:errcheck // best-effort diagnostic progress output
	return nil
}
