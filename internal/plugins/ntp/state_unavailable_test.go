// Design: docs/guide/command-reference.md -- unreadable NTP state is refused.
// Related: register.go -- handleShowSystemNTP and handleShowSystemNTPPeers.

package ntp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestNTPStatusWithoutPublicationRefuses proves an uninitialized process is not
// an authoritative disabled client. An explicit disabled snapshot stays valid.
func TestNTPStatusWithoutPublicationRefuses(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(nil)
	response, err := handleShowSystemNTP(nil, commandtest.Args())
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != plugin.StatusError {
		t.Fatalf("unpublished NTP status=%q data=%v, want refusal", response.Status, response.Data)
	}
	storeState(&syncState{Enabled: false})
	response, err = handleShowSystemNTP(nil, commandtest.Args())
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != plugin.StatusDone {
		t.Fatalf("explicit disabled NTP status=%q: %s", response.Status, response.Error)
	}
	data, ok := response.Data.(plugin.Map)
	if !ok || data[fieldEnabled] != false {
		t.Fatalf("explicit disabled NTP answer=%v, want enabled=false", response.Data)
	}
}

// TestNTPPeersWithoutPublicationRefuse distinguishes an unreadable peer set
// from a measured empty set in an initialized NTP process.
func TestNTPPeersWithoutPublicationRefuse(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(nil)
	response, err := handleShowSystemNTPPeers(nil, commandtest.Args())
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != plugin.StatusError {
		t.Fatalf("unpublished NTP peers status=%q data=%v, want refusal", response.Status, response.Data)
	}
	storeState(&syncState{Enabled: true})
	response, err = handleShowSystemNTPPeers(nil, commandtest.Args())
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != plugin.StatusDone {
		t.Fatalf("measured empty NTP peers status=%q: %s", response.Status, response.Error)
	}
	data, ok := response.Data.(plugin.Map)
	if !ok || data["count"] != 0 {
		t.Fatalf("measured empty NTP peer answer=%v, want count=0", response.Data)
	}
}

// TestNTPPublicationReadsConfiguredEnablement proves a state snapshot reports
// the worker's accepted setting, rather than an unconditional enabled value.
func TestNTPPublicationReadsConfiguredEnablement(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	for _, enabled := range []bool{false, true} {
		cfg := defaultConfig()
		cfg.Enabled = enabled
		worker := newSyncWorker(cfg, nil)
		worker.publishState(false, "", 0, 0)
		state := loadState()
		if state == nil {
			t.Fatal("publication produced no state")
		}
		if state.Enabled != enabled {
			t.Fatalf("configured enabled=%v published enabled=%v", enabled, state.Enabled)
		}
	}
}
