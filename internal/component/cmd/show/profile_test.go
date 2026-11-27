package show

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

const profileTypeLeaf = "show/system/profile/type"

func TestShowSystemProfile_Wiring(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, commandtest.Args(profileTypeHeap))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("expected StatusDone, got %v", resp.Status)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if data["type"] != profileTypeHeap {
		t.Errorf("expected type=heap, got %v", data["type"])
	}
	if _, exists := data["data"]; !exists {
		t.Error("missing data field")
	}
}

func TestProfileCPUMutex(t *testing.T) {
	cpuProfileMu.Lock()

	resp, err := handleShowSystemProfile(nil, commandtest.Args(profileTypeCPU))
	if err != nil {
		cpuProfileMu.Unlock()
		t.Fatal(err)
	}

	cpuProfileMu.Unlock()

	if resp.Status != plugin.StatusError {
		t.Errorf("expected StatusError when mutex held, got %v", resp.Status)
	}
}

func TestProfileCPUDurationBoundary(t *testing.T) {
	tests := []struct {
		name string
		dur  string
	}{
		{"too long", "61s"},
		{"too short", "500ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleShowSystemProfile(nil, commandtest.Args(profileTypeCPU, "duration", tt.dur))
			if err != nil {
				t.Fatal(err)
			}
			if resp.Status != plugin.StatusError {
				t.Errorf("expected StatusError for duration %s, got %v", tt.dur, resp.Status)
			}
		})
	}
}

func TestProfileCPUDurationInvalid(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, commandtest.Args(profileTypeCPU, "duration", "notaduration"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("expected StatusError for invalid duration, got %v", resp.Status)
	}
}

func TestProfileUnknownArgIgnored(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, commandtest.Args("invalid-type"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusDone {
		t.Errorf("expected StatusDone (unknown args ignored, defaults to heap), got %v", resp.Status)
	}
}

// TestProfileTypesMatchTheModel holds show system profile's YANG enum and
// command-owned pipe filters to the handler in both directions.
//
// VALIDATES: AC-1 -- YANG show/system/profile/type declares all six profile
// types handled by handleShowSystemProfile and "summary" is registered as a
// no-argument pipe filter on "show system profile".
// PREVENTS: model/handler drift where mutex or block silently falls back to
// heap or | summary disappears from command-owned pipe filters.
func TestProfileTypesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(profileTypeLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", profileTypeLeaf, err)
	}

	want := []string{"allocs", "block", "cpu", "goroutine", "heap", "mutex"}
	slices.Sort(want)
	if !slices.Equal(model, want) {
		t.Errorf("model enum at %s = %v, want %v", profileTypeLeaf, model, want)
	}

	filters := command.PipeFiltersForCommand("show system profile")
	foundSummary := false
	for _, f := range filters {
		if f.Name == "summary" && f.Argument == command.ArgNone {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Errorf("command.PipeFiltersForCommand(\"show system profile\") = %+v, missing summary (ArgNone)", filters)
	}

	for _, profileType := range model {
		if profileType == profileTypeCPU {
			continue
		}
		resp, err := handleShowSystemProfile(nil, commandtest.Args("type", profileType))
		if err != nil {
			t.Fatalf("type %q: %v", profileType, err)
		}
		if resp.Status != plugin.StatusDone {
			t.Fatalf("type %q: status = %v, error = %q", profileType, resp.Status, resp.Error)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("type %q answered %T, not plugin.Map", profileType, resp.Data)
		}
		if data[keyType] != profileType {
			t.Errorf("type %q is offered at %s but handler answered type %v", profileType, profileTypeLeaf, data[keyType])
		}
	}
}

// TestShowSystemProfileSummaryPipe exercises handleShowSystemProfile with the
// command-owned | summary pipe operation across all instant profile types and
// verifies mutex sampling-rate metadata on both bare and summary responses.
//
// VALIDATES: AC-1 -- show system profile type <t> | summary returns structured
// summary data with format "summary", top-functions, and truncated flag, and
// mutex responses report sampling-rate without modifying the runtime fraction.
// PREVENTS: pipe operation wiring regressions where | summary is ignored and
// raw pprof-base64 is returned instead.
func TestShowSystemProfileSummaryPipe(t *testing.T) {
	pipeOps := []command.PipeOperation{{Name: pipeOpSummary}}
	for _, profileType := range []string{profileTypeHeap, profileTypeGoroutine, profileTypeAllocs, profileTypeMutex, profileTypeBlock} {
		t.Run(profileType, func(t *testing.T) {
			args := commandtest.Args(keyType, profileType).WithPipeOperations(pipeOps)
			resp, err := handleShowSystemProfile(nil, args)
			if err != nil {
				t.Fatalf("type %q | summary: %v", profileType, err)
			}
			if resp.Status != plugin.StatusDone {
				t.Fatalf("type %q | summary: status = %v, error = %q", profileType, resp.Status, resp.Error)
			}
			data, ok := resp.Data.(plugin.Map)
			if !ok {
				t.Fatalf("type %q | summary answered %T, not plugin.Map", profileType, resp.Data)
			}
			assertProfileSummaryShape(t, data, profileType)
			if _, ok := data["profile-count"].(int); !ok {
				t.Errorf("type %q | summary: missing int profile-count, got %T", profileType, data["profile-count"])
			}
			if profileType == profileTypeMutex || profileType == profileTypeBlock {
				if _, ok := data[keySamplingRate].(int); !ok {
					t.Errorf("%s | summary: missing int sampling-rate, got %T", profileType, data[keySamplingRate])
				}
			}
		})
	}

	for _, profileType := range []string{profileTypeMutex, profileTypeBlock} {
		rawResp, err := handleShowSystemProfile(nil, commandtest.Args(keyType, profileType))
		if err != nil {
			t.Fatalf("%s bare: %v", profileType, err)
		}
		if rawResp.Status != plugin.StatusDone {
			t.Fatalf("%s bare: status = %v, error = %q", profileType, rawResp.Status, rawResp.Error)
		}
		rawData, ok := rawResp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("%s bare answered %T, not plugin.Map", profileType, rawResp.Data)
		}
		if rawData[keyFormat] != profileFormatPprofBase64 {
			t.Errorf("%s bare format = %v, want %s", profileType, rawData[keyFormat], profileFormatPprofBase64)
		}
		if _, ok := rawData[keySamplingRate].(int); !ok {
			t.Errorf("%s bare: missing int sampling-rate, got %T", profileType, rawData[keySamplingRate])
		}
	}
}
