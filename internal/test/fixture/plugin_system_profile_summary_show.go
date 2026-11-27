// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- functional fixture for show system profile | summary
// Related: register_system_profile_summary_show.go -- fixture registration
// Related: plugin_fixture_16.go -- sibling show system observer fixtures

package fixture

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func pluginSystemProfileSummary(ctx context.Context, p *sdk.Plugin) error {
	types := []string{"heap", "mutex", "block"}
	var tb textbuf.Buffer
	for _, profileType := range types {
		cmd := tb.Reset().Str("show system profile type ").Str(profileType).Str(" | summary").String()
		label := tb.Reset().Str("system-profile-summary-").Str(profileType).String()
		data, err := plugin16DoneMap(ctx, p, cmd, label)
		if err != nil {
			return err
		}
		if data["format"] != "summary" {
			return fmt.Errorf("%s: format=%v, want summary", label, data["format"])
		}
		if data["type"] != profileType {
			return fmt.Errorf("%s: type=%v, want %s", label, data["type"], profileType)
		}
		top, ok := data["top-functions"].([]any)
		if !ok {
			return fmt.Errorf("%s: top-functions is %T, want array", label, data["top-functions"])
		}
		if _, ok := data["truncated"].(bool); !ok {
			return fmt.Errorf("%s: truncated is %T, want bool", label, data["truncated"])
		}
		if profileType == "mutex" || profileType == "block" {
			num, ok := data["sampling-rate"].(json.Number)
			if !ok {
				return fmt.Errorf("%s: sampling-rate is %T, want json.Number", label, data["sampling-rate"])
			}
			rate, err := num.Int64()
			if err != nil {
				return fmt.Errorf("%s: sampling-rate=%q invalid int: %w", label, num, err)
			}
			if rate <= 0 {
				return fmt.Errorf("%s: sampling-rate=%d, want > 0", label, rate)
			}
		}
		tb.Reset().Str("OK: ").Str(cmd).Str(" top-functions=").Int(int64(len(top))).Byte('\n')
		_ = tb.StdErr() //nolint:errcheck // best-effort diagnostic progress output
	}
	return nil
}
