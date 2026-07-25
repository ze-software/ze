// VALIDATES: show debug RPC returns live slogutil state (levels, flags, scopes).
// PREVENTS: show debug returning stale profile data instead of runtime state.

package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestHandleDebugState(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	_ = slogutil.Logger("test.show.debug")
	_ = slogutil.SetLevel("test.show.debug", "debug")
	slogutil.ConfigureFilter("test.show.debug", []string{"update"}, map[string]string{"neighbor": "192.0.2.1"})

	resp, err := handleDebugState(nil, nil)
	if err != nil {
		t.Fatalf("handleDebugState: %v", err)
	}
	if resp.Status != "done" {
		t.Fatalf("status = %q, want done", resp.Status)
	}

	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("data type = %T, want plugin.Map", resp.Data)
	}
	subsystems, ok := data["subsystems"].([]map[string]any)
	if !ok {
		t.Fatalf("subsystems type = %T, want []map[string]any", data["subsystems"])
	}

	var found bool
	for _, s := range subsystems {
		if s["name"] != "test.show.debug" {
			continue
		}
		found = true
		if s["level"] != "debug" {
			t.Errorf("level = %v, want debug", s["level"])
		}
		flags, _ := s["flags"].([]string)
		if len(flags) != 1 || flags[0] != "update" {
			t.Errorf("flags = %v, want [update]", flags)
		}
		scopes, _ := s["scopes"].(map[string]string)
		if scopes["neighbor"] != "192.0.2.1" {
			t.Errorf("scopes = %v, want neighbor=192.0.2.1", scopes)
		}
		break
	}
	if !found {
		t.Error("test.show.debug not found in response")
	}
}

func TestHandleDebugStateNoFilters(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	_ = slogutil.Logger("test.show.nofilter")
	_ = slogutil.SetLevel("test.show.nofilter", "info")

	resp, err := handleDebugState(nil, nil)
	if err != nil {
		t.Fatalf("handleDebugState: %v", err)
	}

	data, _ := resp.Data.(plugin.Map)
	subsystems, _ := data["subsystems"].([]map[string]any)
	for _, s := range subsystems {
		if s["name"] == "test.show.nofilter" {
			if s["level"] != "info" {
				t.Errorf("level = %v, want info", s["level"])
			}
			if s["flags"] != nil {
				t.Errorf("flags should be nil, got %v", s["flags"])
			}
			if s["scopes"] != nil {
				t.Errorf("scopes should be nil, got %v", s["scopes"])
			}
			return
		}
	}
	t.Error("test.show.nofilter not found in response")
}
