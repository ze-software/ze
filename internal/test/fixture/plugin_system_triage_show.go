// Design: docs/architecture/diagnostics/production-diagnostics.md -- functional fixture for show system triage
// Related: register_system_triage_show.go -- fixture registration
// Related: plugin_fixture_16.go -- sibling show system observer fixtures

package fixture

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func pluginSystemTriage(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system triage", "system-triage")
	if err != nil {
		return err
	}

	sections := []string{
		"version",
		"health",
		"doctor",
		"errors",
		"warnings",
		"crashes",
		"plugins",
		"reload",
		"recent-logs",
		"runtime",
	}
	objs := make(map[string]map[string]any, len(sections))
	for _, key := range sections {
		raw, exists := data[key]
		if !exists {
			return fmt.Errorf("system-triage: missing section %q", key)
		}
		sec, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("system-triage: section %q is %T, want object", key, raw)
		}
		objs[key] = sec
	}

	goVer, ok := objs["version"]["go"].(string)
	if !ok {
		return fmt.Errorf("system-triage: version.go is %T, want string", objs["version"]["go"])
	}
	if !strings.HasPrefix(goVer, "go") {
		return fmt.Errorf("system-triage: version.go=%q does not start with go", goVer)
	}

	healthStatus, ok := objs["health"]["status"].(string)
	if !ok {
		return fmt.Errorf("system-triage: health.status is %T, want string", objs["health"]["status"])
	}
	if healthStatus == "" {
		return fmt.Errorf("system-triage: health.status is empty")
	}

	goroutines, err := plugin16RequireNumber(objs["runtime"], "num-goroutines", 1, "system-triage runtime")
	if err != nil {
		return err
	}

	for _, name := range []string{"errors", "warnings", "crashes", "recent-logs"} {
		if err := requireTriageListSection(objs[name], name); err != nil {
			return err
		}
	}

	var tb textbuf.Buffer
	tb.Reset().Str("OK: show system triage go=").Str(goVer).Str(" health=").Str(healthStatus).Str(" goroutines=").Int(goroutines).Byte('\n')
	_ = tb.StdErr() //nolint:errcheck // best-effort diagnostic progress output
	return nil
}

func requireTriageListSection(sec map[string]any, name string) error {
	if _, err := plugin16RequireNumber(sec, "total-count", 0, "system-triage "+name); err != nil {
		return err
	}
	if _, ok := sec["truncated"].(bool); !ok {
		return fmt.Errorf("system-triage %s: truncated is %T, want bool", name, sec["truncated"])
	}
	for _, val := range sec {
		if _, ok := val.([]any); ok {
			return nil
		}
	}
	return fmt.Errorf("system-triage %s: missing list array field in %v", name, sec)
}
