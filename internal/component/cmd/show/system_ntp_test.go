package show

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestSystemDateRefusesAbsentNTPProvider proves an unavailable state reader
// cannot look like a clock whose NTP client is simply disabled.
//
// VALIDATES: the owned NTP omission row of spec-refuse-instead-of-defaulting.
// PREVENTS: a successful date answer silently omitting unreadable sync state.
func TestSystemDateRefusesAbsentNTPProvider(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	registry.SetNTPSyncProvider(nil)
	response, err := handleShowSystemDate(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("handler transport error: %v", err)
	}
	if response.Status != plugin.StatusError {
		t.Fatalf("absent NTP provider answered status %q, want error", response.Status)
	}
	if response.Data != nil {
		t.Fatalf("refusal carried success data: %#v", response.Data)
	}
	for _, text := range []string{"NTP", "show system date", "fix:"} {
		if !strings.Contains(response.Error, text) {
			t.Errorf("refusal %q does not name %q", response.Error, text)
		}
	}
}

// TestSystemDateAcceptsInactiveNTPProvider proves the registered client's
// deliberate no-sync-metadata answer remains a successful date reading.
//
// VALIDATES: ntpSyncInfo's unconfigured/disabled contract.
// PREVENTS: conflating an inactive client with an absent state reader.
func TestSystemDateAcceptsInactiveNTPProvider(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	registry.SetNTPSyncProvider(func(registry.CommandQuery) (map[string]any, *opererr.Error) { return nil, nil })
	response, err := handleShowSystemDate(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("handler transport error: %v", err)
	}
	if response.Status != plugin.StatusDone {
		t.Fatalf("inactive NTP provider answered %q: %s", response.Status, response.Error)
	}
	data, ok := response.Data.(plugin.Map)
	if !ok {
		t.Fatalf("date answer has type %T, want plugin.Map", response.Data)
	}
	if _, present := data["time"]; !present {
		t.Fatal("inactive NTP client removed the date reading")
	}
	if _, present := data["ntp-synced"]; present {
		t.Fatal("inactive NTP client invented a synchronization measurement")
	}
}

// TestSystemDateCarriesMeasuredNTPState proves an available provider's
// unsynchronized reading survives enrichment, including its zero offset.
//
// VALIDATES: the successful provider-to-command path alongside refusal.
// PREVENTS: replacing missing state with a constant synchronized answer.
func TestSystemDateCarriesMeasuredNTPState(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	registry.SetNTPSyncProvider(func(registry.CommandQuery) (map[string]any, *opererr.Error) {
		return map[string]any{"ntp-synced": false, "ntp-source": "clock.example", "ntp-offset": "0s"}, nil
	})
	response, err := handleShowSystemDate(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("handler transport error: %v", err)
	}
	if response.Status != plugin.StatusDone {
		t.Fatalf("measured NTP provider answered %q: %s", response.Status, response.Error)
	}
	data, ok := response.Data.(plugin.Map)
	if !ok {
		t.Fatalf("date answer has type %T, want plugin.Map", response.Data)
	}
	for key, want := range map[string]any{"ntp-synced": false, "ntp-source": "clock.example", "ntp-offset": "0s"} {
		if got := data[key]; got != want {
			t.Errorf("%s = %v, want measured %v", key, got, want)
		}
	}
}
