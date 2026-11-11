// Design: docs/architecture/config/syntax.md -- inactive subtrees are absent from effective configuration.
// Related: register_commands.go -- accepted configuration classifies NTP availability.

package ntp

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	ntpyang "github.com/ze-software/ze/internal/plugins/ntp/yang"
)

// TestNTPPublicCommandsRespectInactiveSubtree follows the loader's real pruning
// boundary. An inactive subtree can retain enabled=true in the candidate while
// its accepted effective config establishes inactivity without a running owner.
// ActiveConfigMap projects retained leaf-list intent; PruneInactive removes
// whole inactive containers before lowering, as the production loader does.
func TestNTPPublicCommandsRespectInactiveSubtree(t *testing.T) {
	schema, err := config.YANGSchemaWithPlugins(map[string]string{"ze-ntp-conf.yang": ntpyang.ZeNTPConfYANG})
	if err != nil {
		t.Fatal(err)
	}
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(&syncState{Enabled: true, Synced: true, Source: "stale.example"})
	for _, current := range []struct {
		name       string
		prefix     string
		wantStatus string
	}{
		{name: "inactive", prefix: "inactive: ", wantStatus: plugin.StatusDone},
		{name: "active owner unavailable", wantStatus: plugin.StatusError},
	} {
		t.Run(current.name, func(t *testing.T) {
			candidate, err := config.NewParser(schema).Parse("environment { " + current.prefix + "ntp { enabled true } }")
			if err != nil {
				t.Fatal(err)
			}
			node := candidate.GetContainer("environment").GetContainer("ntp")
			if enabled, _ := node.Get(fieldEnabled); enabled != "true" {
				t.Fatalf("candidate enabled=%q, want true", enabled)
			}
			accepted := candidate.Clone()
			config.PruneInactive(accepted, schema)
			running := accepted.ToRunningMap()
			before, err := json.Marshal(running)
			if err != nil {
				t.Fatal(err)
			}
			ctx := ntpCommandTestContext(t, running)
			for index, handler := range []pluginserver.Handler{forwardShowSystemNTP, forwardShowSystemNTPPeers} {
				response, err := handler(ctx, commandtest.Args())
				if err != nil {
					t.Fatal(err)
				}
				if response.Status != current.wantStatus {
					t.Fatalf("public command status=%q, want %q: %s", response.Status, current.wantStatus, response.Error)
				}
				if current.wantStatus == plugin.StatusError {
					if response.Data != nil {
						t.Errorf("unavailable owner carried success data %v", response.Data)
					}
					continue
				}
				data, ok := response.Data.(plugin.Map)
				if !ok {
					t.Fatalf("inactive command data has type %T", response.Data)
				}
				if index == 0 {
					if enabled, present := data[fieldEnabled]; !present || enabled != false {
						t.Errorf("inactive status enabled=%v present=%t, want false", enabled, present)
					}
				} else {
					if count, present := data["count"]; !present || count != 0 {
						t.Errorf("inactive peers count=%v present=%t, want zero", count, present)
					}
					peers, ok := data["peers"].([]map[string]any)
					if !ok || len(peers) != 0 {
						t.Errorf("inactive peers=%v, want an explicit empty array", data["peers"])
					}
				}
				if _, present := data["synced"]; present {
					t.Error("inactive command leaked stale synchronization state")
				}
			}
			after, err := json.Marshal(running)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Errorf("public reads changed accepted running configuration: before=%s after=%s", before, after)
			}
		})
	}
}
