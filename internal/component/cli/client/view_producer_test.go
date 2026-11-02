package client

import (
	"reflect"
	"testing"

	unicli "github.com/ze-software/ze/internal/component/cli"
	pingcmd "github.com/ze-software/ze/internal/component/ping/cmd"
	traceroutecmd "github.com/ze-software/ze/internal/component/traceroute/cmd"
)

// TestMonitorViewsUseTheOneProducer proves AC-14 for monitor ping and monitor
// traceroute on the `ze cli` route: the live views run NewPingSession and
// NewTracerouteSession, the producers the local `ze monitor` loop
// (monitorPingLocal, monitorTracerouteLocal) and the SSH session factory
// (cmd/ze/hub TestMonitorViewsUseTheOneProducer) run too.
//
// Method: inject the view factories into a model and compare each stored
// factory's code pointer with the producer's. A route that wrapped or rebuilt
// the session would store another function and fail.
func TestMonitorViewsUseTheOneProducer(t *testing.T) {
	m := unicli.NewCommandModel(unicli.FilesystemAuthorityOperatorLocal)
	injectViewFactories(&m, nil)
	assertViewProducer(t, &m, unicli.ViewKeyPing, pingcmd.NewPingSession)
	assertViewProducer(t, &m, unicli.ViewKeyTraceroute, traceroutecmd.NewTracerouteSession)
}

func assertViewProducer(t *testing.T, m *unicli.Model, key string, producer any) {
	t.Helper()
	factory, ok := m.ViewFactory(key)
	if !ok {
		t.Fatalf("view %q: no factory injected", key)
	}
	if reflect.ValueOf(factory).Pointer() != reflect.ValueOf(producer).Pointer() {
		t.Errorf("view %q: injected factory is not the producer", key)
	}
}
