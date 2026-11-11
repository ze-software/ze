// Design: docs/guide/command-reference.md -- NTP answers come from its runtime owner.
// Related: register.go, register_commands.go -- provider projection and command forwarding.

package ntp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestNTPDateMetadataReadsRuntimeOwner proves an opposite daemon-local snapshot
// cannot replace the runtime owner's published false synchronization and zero.
func TestNTPDateMetadataReadsRuntimeOwner(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(&syncState{Enabled: true, Synced: true, Source: "stale.example", Offset: time.Second})
	calls := 0
	data, refusal := ntpSyncInfo(func(name string) (map[string]any, error) {
		calls++
		if name != commandNTPStatus {
			t.Errorf("queried %q, want %q", name, commandNTPStatus)
		}
		return map[string]any{fieldEnabled: true, "synced": false, "source": "worker.example", "offset": "0s"}, nil
	})
	if refusal != nil {
		t.Fatal(refusal)
	}
	if calls != 1 {
		t.Fatalf("runtime queries=%d, want one", calls)
	}
	for field, want := range map[string]any{
		"ntp-synced": false, "ntp-source": "worker.example", "ntp-offset": "0s",
	} {
		got, present := data[field]
		if !present {
			t.Errorf("runtime answer omitted %q", field)
		}
		if got != want {
			t.Errorf("%s=%v, want runtime value %v", field, got, want)
		}
	}
}

// TestNTPDateMetadataRefusesUnknownReply checks absence and malformed values do
// not turn into disabled, synchronized or zero-offset answers.
func TestNTPDateMetadataRefusesUnknownReply(t *testing.T) {
	for _, current := range []struct {
		name string
		data map[string]any
		err  error
	}{
		{name: "transport failure", err: errors.New("worker disconnected")},
		{name: "no document"},
		{name: "no enabled", data: map[string]any{"synced": false}},
		{name: "unreadable enabled", data: map[string]any{fieldEnabled: "false"}},
		{name: "no synchronization", data: map[string]any{fieldEnabled: true, "source": "", "offset": "0s"}},
		{name: "unreadable synchronization", data: map[string]any{fieldEnabled: true, "synced": "false", "source": "", "offset": "0s"}},
		{name: "no source", data: map[string]any{fieldEnabled: true, "synced": false, "offset": "0s"}},
		{name: "no offset", data: map[string]any{fieldEnabled: true, "synced": false, "source": ""}},
		{name: "unreadable offset", data: map[string]any{fieldEnabled: true, "synced": false, "source": "", "offset": "missing"}},
	} {
		t.Run(current.name, func(t *testing.T) {
			data, refusal := ntpSyncInfo(func(string) (map[string]any, error) {
				return current.data, current.err
			})
			if refusal == nil {
				t.Fatalf("unknown runtime reply produced %v without refusal", data)
			}
			if data != nil {
				t.Fatalf("unknown runtime reply produced success metadata %v", data)
			}
			if !strings.Contains(refusal.Error(), "ze show system ntp") {
				t.Errorf("refusal has no NTP recovery command: %s", refusal)
			}
		})
	}
}

// TestNTPPublicCommandsReadAcceptedConfiguration keeps a known inactive client
// useful without mistaking an unrelated local snapshot for its active worker.
func TestNTPPublicCommandsReadAcceptedConfiguration(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(&syncState{Enabled: true, Synced: true, Source: "stale.example"})
	for _, tree := range []map[string]any{
		{},
		{"environment": map[string]any{"ntp": map[string]any{fieldEnabled: "false"}}},
	} {
		ctx := ntpCommandTestContext(t, tree)
		response, err := forwardShowSystemNTP(ctx, commandtest.Args())
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != plugin.StatusDone {
			t.Fatalf("known inactive status=%q: %s", response.Status, response.Error)
		}
		data, ok := response.Data.(plugin.Map)
		if !ok {
			t.Fatalf("known inactive data has type %T", response.Data)
		}
		if data[fieldEnabled] != false {
			t.Fatalf("known inactive enabled=%v, want false", data[fieldEnabled])
		}
		if _, present := data["synced"]; present {
			t.Fatal("inactive answer leaked the daemon-local synchronization snapshot")
		}
		peers, err := forwardShowSystemNTPPeers(ctx, commandtest.Args())
		if err != nil {
			t.Fatal(err)
		}
		if peers.Status != plugin.StatusDone {
			t.Fatalf("known inactive peers status=%q: %s", peers.Status, peers.Error)
		}
		peerData, ok := peers.Data.(plugin.Map)
		if !ok {
			t.Fatalf("known inactive peer data has type %T", peers.Data)
		}
		if peerData["count"] != 0 {
			t.Errorf("known inactive peer count=%v, want zero", peerData["count"])
		}
	}
}

// TestNTPPublicCommandsRefuseUnavailableOwner distinguishes an enabled but
// unreadable client from a known inactive configuration on both public commands.
func TestNTPPublicCommandsRefuseUnavailableOwner(t *testing.T) {
	ctx := ntpCommandTestContext(t, map[string]any{
		"environment": map[string]any{"ntp": map[string]any{fieldEnabled: "true"}},
	})
	for _, handler := range []pluginserver.Handler{forwardShowSystemNTP, forwardShowSystemNTPPeers} {
		response, err := handler(ctx, commandtest.Args())
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != plugin.StatusError {
			t.Fatalf("missing active owner status=%q data=%v", response.Status, response.Data)
		}
		if response.Data != nil {
			t.Errorf("missing owner carried success data %v", response.Data)
		}
		for _, text := range []string{"NTP", "fix:", "ze show system ntp"} {
			if !strings.Contains(response.Error, text) {
				t.Errorf("refusal %q omits %q", response.Error, text)
			}
		}
	}
}

// TestNTPPublicCommandsRefuseUnknownConfiguration proves a missing accepted
// configuration is never taken as the empty, inactive configuration.
func TestNTPPublicCommandsRefuseUnknownConfiguration(t *testing.T) {
	for _, ctx := range []*pluginserver.CommandContext{nil, ntpCommandTestContext(t, nil)} {
		response, err := forwardShowSystemNTP(ctx, commandtest.Args())
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != plugin.StatusError {
			t.Fatalf("missing accepted configuration status=%q data=%v", response.Status, response.Data)
		}
	}
}

// TestNTPRuntimeCommandsReadTheirWorker checks the SDK command callback carries
// current state to both command paths rather than a daemon-side substitute.
func TestNTPRuntimeCommandsReadTheirWorker(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(&syncState{Enabled: true, Synced: false, Offset: 0,
		Servers: []serverState{{Address: "worker.example", Offset: 0, Stratum: 2}}})
	status, value, err := executeNTPCommand("1", commandNTPStatus, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if status != plugin.StatusDone {
		t.Fatalf("runtime summary status=%q", status)
	}
	data, ok := value.(plugin.Map)
	if !ok {
		t.Fatalf("runtime summary has type %T", value)
	}
	if data["synced"] != false {
		t.Errorf("runtime synced=%v, want false", data["synced"])
	}
	if data["offset"] != "0s" {
		t.Errorf("runtime offset=%v, want 0s", data["offset"])
	}
	status, value, err = executeNTPCommand("2", commandNTPPeers, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if status != plugin.StatusDone {
		t.Fatalf("runtime peers status=%q", status)
	}
	data, ok = value.(plugin.Map)
	if !ok {
		t.Fatalf("runtime peers have type %T", value)
	}
	rows, ok := data["peers"].([]map[string]any)
	if !ok {
		t.Fatalf("runtime peer rows have type %T", data["peers"])
	}
	if len(rows) != 1 {
		t.Fatalf("runtime peer count=%d, want one", len(rows))
	}
	if rows[0]["address"] != "worker.example" {
		t.Errorf("runtime peer=%v, want worker.example", rows[0]["address"])
	}
}

// TestNTPRuntimeCommandRefusals keeps unsupported calls and unpublished state
// explicit when a request reaches the SDK callback.
func TestNTPRuntimeCommandRefusals(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(nil)
	for _, current := range []struct {
		name string
		args []string
	}{
		{name: "show system unknown"},
		{name: commandNTPStatus, args: []string{"unexpected"}},
		{name: commandNTPStatus},
		{name: commandNTPPeers},
	} {
		status, data, err := executeNTPCommand("1", current.name, current.args, "")
		if err == nil {
			t.Errorf("runtime call %q %v returned no error", current.name, current.args)
		}
		if status != plugin.StatusError {
			t.Errorf("runtime refusal status=%q, want error", status)
		}
		if data != nil {
			t.Errorf("runtime refusal carried data %v", data)
		}
	}
	storeState(&syncState{Enabled: false})
	status, data, err := executeNTPCommand("2", commandNTPPeers, nil, "")
	if err == nil {
		t.Fatal("disabled worker answered an active client's peers request without error")
	}
	if status != plugin.StatusError {
		t.Errorf("disabled worker peers status=%q, want error", status)
	}
	if data != nil {
		t.Errorf("disabled worker peers carried data %v", data)
	}
}

// ntpCommandTestContext gives the public handlers a real daemon dispatcher and
// a coordinator holding the accepted config, without starting a time worker.
func ntpCommandTestContext(t *testing.T, tree map[string]any) *pluginserver.CommandContext {
	t.Helper()
	server, err := pluginserver.NewServer(nil, plugin.NewCoordinator(tree))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Stop)
	return &pluginserver.CommandContext{Server: server, Sender: plugin.OperatorSender()}
}
