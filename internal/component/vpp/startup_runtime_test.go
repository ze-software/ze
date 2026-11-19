// Design: docs/research/vpp-deployment-reference.md -- registered runtime prerequisites

package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// A declaration captures message identity rather than retaining mutable
// caller objects. Compatibility must ask VPP for the originally declared CRC.
func TestStartupRuntimeOwnsMessageIdentity(t *testing.T) {
	message := &startupProbeMessage{name: "probe_request", crc: "1234abcd"}
	feature := startupRuntimeTestFeature(t, message)
	message.crc = "00000000"
	var checked int
	channel := startupProbeChannel{check: func(messages ...api.Message) error {
		checked++
		if len(messages) != 1 {
			t.Fatalf("checked %d messages", len(messages))
		}
		if messages[0].GetCrcString() != "1234abcd" {
			t.Fatalf("checked mutated CRC %s", messages[0].GetCrcString())
		}
		return nil
	}}
	settings := VPPSettings{startup: []startupFeature{feature}}
	if err := settings.checkStartupAPI(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	if checked != 1 {
		t.Fatalf("compatibility checks=%d", checked)
	}
}

// A stock or missing plugin cannot pass merely because the API connection is
// usable. Its exact name-and-CRC refusal must survive the readiness gate.
func TestStartupRuntimeRejectsIncompatiblePlugin(t *testing.T) {
	feature := startupRuntimeTestFeature(t, &startupProbeMessage{name: "probe_request", crc: "1234abcd"})
	want := &api.CompatibilityError{IncompatibleMessages: []string{"probe_request_1234abcd"}}
	channel := startupProbeChannel{check: func(...api.Message) error { return want }}
	settings := VPPSettings{startup: []startupFeature{feature}}
	if err := settings.checkStartupAPI(context.Background(), channel); !errors.Is(err, want) {
		t.Fatalf("readiness refusal=%v", err)
	}
}

// Cancellation must prevent the compatibility query, while an unselected
// feature must add no requirement to a different forwarding configuration.
func TestStartupRuntimeCancellationAndUnselectedFeature(t *testing.T) {
	feature := startupRuntimeTestFeature(t, &startupProbeMessage{name: "probe_request", crc: "1234abcd"})
	channel := startupProbeChannel{check: func(...api.Message) error {
		t.Fatal("canceled or unselected prerequisite was queried")
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	settings := VPPSettings{startup: []startupFeature{feature}}
	if err := settings.checkStartupAPI(ctx, channel); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled readiness=%v", err)
	}
	settings.startup = nil
	if err := settings.checkStartupAPI(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
}

// Doctor distinguishes an answered schema mismatch from a socket that could
// not be queried, and uses the configured API socket for both outcomes.
func TestStartupRuntimeDoctorDistinguishesUnknownAndIncompatible(t *testing.T) {
	feature := startupRuntimeTestFeature(t, &startupProbeMessage{name: "probe_request", crc: "1234abcd"})
	tree, err := config.TreeFromPluginMap(map[string]any{
		"vpp": map[string]any{"enabled": "true", "api-socket": "/run/probe/api.sock"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []struct {
		name     string
		failure  error
		severity diagnostic.Severity
	}{
		{name: "unavailable", failure: errors.New("socket unavailable"), severity: diagnostic.SeverityWarning},
		{name: "incompatible", failure: &api.CompatibilityError{IncompatibleMessages: []string{"probe_request_1234abcd"}}, severity: diagnostic.SeverityError},
		{name: "available"},
	} {
		t.Run(point.name, func(t *testing.T) {
			findings := checkStartupDoctor(tree, feature, func(_ context.Context, settings *VPPSettings) error {
				if settings.APISocket != "/run/probe/api.sock" {
					t.Fatalf("probe socket=%s", settings.APISocket)
				}
				if len(settings.startup) != 1 {
					t.Fatalf("selected requirements=%d", len(settings.startup))
				}
				return point.failure
			})
			if point.failure == nil {
				if len(findings) != 0 {
					t.Fatalf("healthy findings=%v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("findings=%v", findings)
			}
			if findings[0].Severity != point.severity {
				t.Fatalf("severity=%s", findings[0].Severity)
			}
			if findings[0].Code != feature.runtime.code {
				t.Fatalf("diagnostic code=%s", findings[0].Code)
			}
			if findings[0].Fix != "ze doctor" {
				t.Fatalf("repair=%s", findings[0].Fix)
			}
		})
	}
}

// Malformed declarations cannot publish a readiness check which proves no
// message layout, or silently omits the operator's diagnosis and repair.
func TestStartupRuntimeRejectsMalformedDeclaration(t *testing.T) {
	for _, point := range []string{"empty", "nil", "crc length", "crc encoding", "name", "code", "repair"} {
		t.Run(point, func(t *testing.T) {
			message := &startupProbeMessage{name: "probe_request", crc: "1234abcd"}
			input := &StartupRuntime{Messages: []api.Message{message}, Code: "doctor-vpp-probe-plugin", Fix: "ze doctor"}
			switch point {
			case "empty":
				input.Messages = nil
			case "nil":
				input.Messages = []api.Message{nil}
			case "crc length":
				message.crc = "1234"
			case "crc encoding":
				message.crc = "notacrc!"
			case "name":
				message.name = "bad name"
			case "code":
				input.Code = ""
			case "repair":
				input.Fix = ""
			}
			if _, err := newStartupRuntime(input); err == nil {
				t.Fatal("malformed runtime requirement accepted")
			}
		})
	}
}

func startupRuntimeTestFeature(t *testing.T, message api.Message) startupFeature {
	t.Helper()
	feature, err := newStartupFeature("probe", StartupFeature{
		Plugin: "probe_plugin.so", Section: "probe",
		Enabled: func(*config.Tree) (bool, error) { return true, nil },
		Runtime: &StartupRuntime{Messages: []api.Message{message},
			Code: "doctor-vpp-probe-plugin", Fix: "ze doctor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return feature
}

type startupProbeMessage struct{ name, crc string }

func (m *startupProbeMessage) GetMessageName() string        { return m.name }
func (m *startupProbeMessage) GetCrcString() string          { return m.crc }
func (*startupProbeMessage) GetMessageType() api.MessageType { return api.RequestMessage }

type startupProbeChannel struct {
	api.Channel
	check func(...api.Message) error
}

func (c startupProbeChannel) CheckCompatiblity(messages ...api.Message) error {
	return c.check(messages...)
}
