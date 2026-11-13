// Design: docs/architecture/doctor-and-health-checks.md -- runtime routing-view health.
// Related: sysrib_execution_fixture.go -- actual placement and useful kernel controls.

package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func sysribHealthDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	return Observe(ctx, "sysrib-health-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		for _, prefix := range []string{sysribExecutionTagged, sysribExecutionPlain} {
			if err := sysribExecutionCandidates(ctx, p, "show rib candidates prefix "+prefix); err != nil {
				return err
			}
			if err := waitRIBWinner(ctx, p, prefix, protocolStatic); err != nil {
				return err
			}
		}
		if err := sysribExecutionCandidates(ctx, p, "show rib candidates"); err != nil {
			return err
		}
		if err := sysribExecutionKernel(ctx); err != nil {
			return err
		}
		if err := sysribHealthReady(ctx, p); err != nil {
			return err
		}
		return importSay("actual system RIB is healthy through doctor and ready telemetry after both useful kernel controls")
	})
}

func sysribHealthReady(ctx context.Context, p *sdk.Plugin) error {
	var last string
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
		data, err := requireDone02(ctx, p, "show metrics name ze_rib_view_ready")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var metrics struct {
			Series []struct {
				Line string `json:"line"`
			} `json:"series"`
		}
		if err := decode02(data, &metrics); err != nil {
			last = err.Error()
			return false
		}
		for _, series := range metrics.Series {
			if series.Line == "ze_rib_view_ready 1" {
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("routing-view ready gauge did not become one: %s", last)
	}
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		data, err := requireDone02(ctx, p, "show doctor")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var report struct {
			SchemaVersion int `json:"schema-version"`
			Diagnostics   []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Fix     string `json:"fix"`
			} `json:"diagnostics"`
		}
		if err := decode02(data, &report); err != nil {
			last = err.Error()
			return false
		}
		if report.SchemaVersion < 1 {
			return false
		}
		for _, finding := range report.Diagnostics {
			if finding.Code == "doctor-rib-routing-view-unavailable" {
				return false
			}
			if finding.Code == "doctor-plugin-unreachable" {
				if strings.Contains(finding.Message, sysribExecutionOwner) {
					return false
				}
			}
		}
		return true
	}) {
		return fmt.Errorf("healthy kernel controls retained a routing-view doctor refusal: %s", last)
	}
	return nil
}
