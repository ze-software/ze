// Design: docs/architecture/traffic/cos-dynamic.md -- warning ownership transfer
// Related: shared_access_restart_test.go -- joined registered runner lifetimes

//go:build ze_l2tp

package cos

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: joined internal runner replacement retains only the original failed
// warning clearance after native restoration has already completed.
// Method: fail real Down, recover its modeled native maps but fail CLEAR,
// join that SDK runner, then retry through the replacement's real SDK route.
// This proves diagnostic custody over modeled native seams, not native grants,
// physical predecessor retirement, external process replacement or G3.
func TestCoSLegacySharedRegisteredRestartKeepsReportOnlyWarning(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	write := f.handler.updateQoS
	read := f.handler.readLink
	previousHandler := dynamicHandler
	t.Cleanup(func() { dynamicHandler = previousHandler })
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	registered := registry.Lookup(Name)
	if registered == nil {
		t.Fatal("the class-of-service registration is absent")
	}
	start := func(t *testing.T) *pluginengine.Engine {
		t.Helper()
		registered.ConfigureEventBus(bus)
		if dynamicHandler == nil {
			t.Fatal("the registered runner did not publish its owner")
		}
		f.handler = dynamicHandler
		f.handler.updateQoS = write
		f.handler.readLink = read
		f.handler.captureTarget = captureModeledOriginalCoSTarget(f.handler)
		return pluginengine.Start(t, pluginengine.TransportBridge, registered.RunEngine)
	}
	var subject string
	if !t.Run("original runner", func(t *testing.T) {
		engine := start(t)
		registeredCoSCleanupReviewUp(t, bus, 63237)
		f.failWrites = 1
		registeredCoSCleanupReviewDown(t, bus, 63237)
		subject = requireCoSCommandWarning(t, "", "native write outcome is not confirmed")
		f.handler.mu.Lock()
		publish := f.handler.reportIssue
		f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
			if input.Action == rpc.ReportClear {
				return errors.New("original warning clearance acknowledgement is unavailable")
			}
			return publish(input)
		}
		f.handler.mu.Unlock()
		writes := f.writes
		cleanupErr := executeCoSCleanupForReview(t, engine, nil)
		if cleanupErr == nil {
			t.Fatal("original cleanup acknowledged failed warning clearance")
		}
		if !strings.Contains(cleanupErr.Error(), "original warning clearance acknowledgement is unavailable") {
			t.Fatalf("original cleanup lost its publication cause: %v", cleanupErr)
		}
		if f.writes != writes+1 {
			t.Fatal("original cleanup did not confirm native restoration once")
		}
		f.requireMaps(f.original)
		requireCoSCommandWarning(t, subject, "native write outcome is not confirmed")
		if _, retained := sessionStore.Load(fixtureCoSKey(sharedCoSTestTunnel, 63237)); retained {
			t.Fatal("native-confirmed report-only custody retained a departed subscriber")
		}
	}) {
		return
	}
	writes := f.writes
	reads := f.reads
	if !t.Run("replacement runner", func(t *testing.T) {
		engine := start(t)
		clears := 0
		f.handler.mu.Lock()
		publish := f.handler.reportIssue
		f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
			if input.Action == rpc.ReportRaise {
				return errors.New("replacement manufactured a new warning lifetime")
			}
			if input.Action == rpc.ReportClear {
				clears++
				if input.Subject != subject {
					return errors.New("replacement tried to clear another original owner")
				}
			}
			return publish(input)
		}
		f.handler.mu.Unlock()
		if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
			t.Fatalf("replacement could not clear the transferred original warning: %v", err)
		}
		if clears != 1 {
			t.Fatalf("replacement made %d CLEAR attempts, want one original lifetime", clears)
		}
		if f.writes != writes {
			t.Fatal("replacement replayed confirmed native cleanup")
		}
		if f.reads != reads {
			t.Fatal("replacement captured a new baseline for report-only custody")
		}
		if warnings := report.Warnings(); len(warnings) != 0 {
			t.Fatalf("replacement abandoned the transferred original warning: %+v", warnings)
		}
		if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
			t.Fatal(err)
		}
		if clears != 1 {
			t.Fatal("replacement repeated acknowledged original warning clearance")
		}
	}) {
		return
	}
	f.requireMaps(f.original)
}
