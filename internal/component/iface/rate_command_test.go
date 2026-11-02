package iface

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestRateCommandAnswersFromThePluginTracker checks the plugin side of the
// rate command: it answers from the tracker of the process it runs in, every
// interface sorted by name or one by name, in a document that decodes to the
// type the daemon handlers read. Method: a stub backend feeds one collect.
func TestRateCommandAnswersFromThePluginTracker(t *testing.T) {
	_, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth1", Stats: &InterfaceStats{RxBytes: 200}},
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100}},
	})
	defer cleanup()

	tracker := newRateTracker()
	globalTracker.Store(tracker)
	defer globalTracker.Store(nil)
	tracker.collect()

	status, data, err := executeCommand(RateCommand, nil)
	if err != nil || status != "done" {
		t.Fatalf("status %q err %v", status, err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rates []InterfaceRate
	if err := json.Unmarshal(b, &rates); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	if len(rates) != 2 || rates[0].Name != "eth0" || rates[1].Name != "eth1" {
		t.Fatalf("rates %+v, want eth0 then eth1", rates)
	}

	_, one, err := executeCommand(RateCommand, []string{"eth1"})
	if err != nil {
		t.Fatalf("one: %v", err)
	}
	if rate, ok := one.(InterfaceRate); !ok || rate.Name != "eth1" {
		t.Fatalf("one answered %+v", one)
	}

	if _, _, err := executeCommand(RateCommand, []string{"eth9"}); err == nil || err.Error() != "interface not found: eth9" {
		t.Fatalf("unknown interface: %v", err)
	}
	if _, _, err := executeCommand("show interface brief", nil); err == nil {
		t.Fatal("a command the plugin does not serve was answered")
	}
}

// TestRateCommandRefusesWithoutTracker checks that a process with no tracker
// refuses instead of answering an empty list.
func TestRateCommandRefusesWithoutTracker(t *testing.T) {
	globalTracker.Store(nil)
	if _, _, err := executeCommand(RateCommand, nil); !errors.Is(err, errRateTrackerNotRunning) {
		t.Fatalf("err %v, want errRateTrackerNotRunning", err)
	}
}
