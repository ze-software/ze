// Design: docs/architecture/config/transaction-protocol.md -- startup publication belongs to undo.before.
package server

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A newly started in-process runtime can publish its selected configuration in
// OnConfigure, before ordinary verify/apply. This is the actual authority write
// a protocol factory's attach performs; cleanup cannot guess the prior accepted
// map. The existing reload acceptance owner must already own undo.before.
func TestReloadStartupPublicationOwnsCompensationBeforeActivation(t *testing.T) {
	for _, tc := range []struct {
		name              string
		activationFailure bool
		verifyRefusal     bool
	}{
		{"activation-failure", true, false},
		{"later-verify-refusal", false, true},
		{"successful-activation", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			guardName := "authority-guard-" + tc.name
			ownerName := "authority-runtime-" + tc.name
			guardRoot := "authority-guard-root-" + tc.name
			ownerRoot := "authority-runtime-root-" + tc.name
			before := map[string]any{guardRoot: map[string]any{"value": "before"}}
			candidate := map[string]any{guardRoot: map[string]any{"value": "candidate"}, ownerRoot: map[string]any{"enabled": "true"}}
			coord := plugin.NewCoordinator(before)
			s, spawner := newLifecycleStartupServer(t)
			s.reactor = coord
			var published, verified atomic.Int64
			var active, refuse atomic.Bool
			if err := registry.Register(registry.Registration{
				Name: guardName, Description: "real startup authority verify gate", CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					p := sdk.NewWithConn(guardName, conn)
					p.DeclareAppliesNothing("test verify gate: owns no reload-applied resource")
					defer func() { _ = p.Close() }()
					p.OnConfigVerify(func([]sdk.ConfigSection) error {
						verified.Add(1)
						if refuse.Load() {
							return errors.New("authority guard refused candidate")
						}
						return nil
					})
					_ = p.Run(context.Background(), &sdk.Registration{WantsConfig: []string{guardRoot}})
					return 0
				},
			}); err != nil {
				t.Fatal(err)
			}
			if err := registry.Register(registry.Registration{
				Name: ownerName, Description: "real startup-time running authority owner", ConfigRoots: []string{ownerRoot}, CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					p := sdk.NewWithConn(ownerName, conn)
					p.DeclareAppliesNothing("test runtime publishes constructor input at configure and reads no public leaf")
					defer func() { _ = p.Close(); active.Store(false) }()
					p.OnConfigure(func([]sdk.ConfigSection) error {
						selected, err := s.ConfigurationFor(conn)
						if err != nil {
							return err
						}
						if !reflect.DeepEqual(selected, candidate) {
							return errors.New("constructor received another Configure lifetime's authority")
						}
						// Deliberately retain the published map on cleanup, just as
						// detach must retain the exiting runtime's actual map.
						coord.SetConfigTree(selected)
						published.Add(1)
						active.Store(true)
						if tc.activationFailure {
							return errors.New("runtime failed after publishing authority")
						}
						return nil
					})
					_ = p.Run(context.Background(), &sdk.Registration{WantsConfig: []string{ownerRoot}})
					return 0
				},
			}); err != nil {
				t.Fatal(err)
			}
			if err := s.runPluginPhase([]plugin.PluginConfig{{Name: guardName, Internal: true, Encoder: plugin.EncodingJSON}}); err != nil {
				t.Fatal(err)
			}
			refuse.Store(tc.verifyRefusal)
			err := s.ReloadConfig(t.Context(), candidate)
			if got := published.Load(); got != 1 {
				t.Fatalf("actual auto-loaded configure published %d times, want once", got)
			}
			if tc.activationFailure || tc.verifyRefusal {
				if err == nil {
					t.Fatal("reload accepted deliberately failed runtime activation/verification")
				}
				if got := coord.GetConfigTree(); !reflect.DeepEqual(got, before) {
					t.Errorf("rejected startup left candidate authority published: got %v, want %v", got, before)
				}
				if active.Load() {
					t.Error("failed candidate runtime remained active")
				}
				if spawner.pm.GetProcess(ownerName) != nil {
					t.Error("failed auto-loaded runtime remained in process manager")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(coord.GetConfigTree(), candidate) || !active.Load() || spawner.pm.GetProcess(ownerName) == nil {
					t.Error("successful activation did not retain the candidate authority and actual runtime")
				}
			}
			if tc.verifyRefusal && verified.Load() == 0 {
				t.Error("later refusal did not traverse the actual SDK verification callback")
			}
		})
	}
}
