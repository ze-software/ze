// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle.
package server

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSourcePolicyUnsupportedOwnerRefusedOnBothTransports proves a plugin that
// declares source-policy-owner in its Stage 1 registration and registers no
// handler refuses the capture before policy publication, on DirectBridge and on
// the wire. Goal: a route server or reflector that holds a peer's routes is
// never skipped as an observer, and the operator is told what to do. Method:
// start the plugin through the production startup with the declaration the SDK
// sends, then capture it through CaptureSourcePolicyOwners.
func TestSourcePolicyUnsupportedOwnerRefusedOnBothTransports(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			s, _ := newLifecycleStartupServer(t)
			const name = "source-policy-unsupported-owner"
			registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
				if wire {
					conn = struct{ net.Conn }{conn}
				}
				p := sdk.NewWithConn(name, conn)
				p.OnConfigure(func([]sdk.ConfigSection) error { return nil })
				defer p.Close() //nolint:errcheck // test plugin teardown
				if err := p.Run(s.Context(), &sdk.Registration{SourcePolicyOwner: true}); err != nil {
					return 1
				}
				return 0
			})
			if err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}); err != nil {
				t.Fatal(err)
			}
			reg := s.procManager.Load().GetProcess(name).Registration()
			if !reg.SourcePolicyOwner || reg.SourcePolicy {
				t.Fatalf("Stage 1 carried owner=%t support=%t, want owner without support", reg.SourcePolicyOwner, reg.SourcePolicy)
			}
			owners, err := s.CaptureSourcePolicyOwners([]string{name})
			if err == nil {
				t.Fatalf("an unsupported owner was captured as %d owners", len(owners))
			}
			for _, want := range []string{"plugin " + name + " holds routes received from this peer",
				"disable the peer", "re-enable the peer"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not say %q", err, want)
				}
			}
		})
	}
}
