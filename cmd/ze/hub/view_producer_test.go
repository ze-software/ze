//go:build ze_ssh

package hub

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
	pingcmd "github.com/ze-software/ze/internal/component/ping/cmd"
	traceroutecmd "github.com/ze-software/ze/internal/component/traceroute/cmd"
)

// TestMonitorViewsUseTheOneProducer proves AC-14 for monitor ping and monitor
// traceroute on the SSH session route: the live views run NewPingSession and
// NewTracerouteSession, the producers the local `ze monitor` loop
// (monitorPingLocal, monitorTracerouteLocal) and `ze cli`
// (cli/client TestMonitorViewsUseTheOneProducer) run too.
//
// Method: inject the view factories into a model and compare each stored
// factory's code pointer with the producer's. A route that wrapped or rebuilt
// the session would store another function and fail.
func TestMonitorViewsUseTheOneProducer(t *testing.T) {
	m := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
	injectViewFactories(&m, nil)
	assertViewProducer(t, &m, cli.ViewKeyPing, pingcmd.NewPingSession)
	assertViewProducer(t, &m, cli.ViewKeyTraceroute, traceroutecmd.NewTracerouteSession)
}

func assertViewProducer(t *testing.T, m *cli.Model, key string, producer any) {
	t.Helper()
	factory, ok := m.ViewFactory(key)
	if !ok {
		t.Fatalf("view %q: no factory injected", key)
	}
	if reflect.ValueOf(factory).Pointer() != reflect.ValueOf(producer).Pointer() {
		t.Errorf("view %q: injected factory is not the producer", key)
	}
}
