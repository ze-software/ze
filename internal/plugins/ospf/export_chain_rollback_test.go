// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- export chain on reload and rollback.

package ospf

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ospfRollbackEngine is the engine side of one OSPF plugin connection. It
// answers every request the plugin sends and reports each method on seen, so
// the test can follow the five-stage startup.
type ospfRollbackEngine struct {
	mux  *rpc.MuxConn
	seen chan string
}

// serve answers each plugin request until the connection closes: runtime state
// is absent and a boot count increments to one, and every other request is
// acknowledged.
func (e *ospfRollbackEngine) serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-e.mux.Requests():
			if !ok || req == nil {
				return
			}
			switch req.Method {
			case rpc.MethodStateIncrement:
				_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateOK, Value: 1})
			case rpc.MethodStateGet:
				_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateAbsent})
			default:
				_ = e.mux.SendOK(ctx, req.ID)
			}
			select {
			case e.seen <- req.Method:
			default:
			}
		}
	}
}

// waitFor blocks until the plugin has sent method.
func (e *ospfRollbackEngine) waitFor(t *testing.T, ctx context.Context, method string) {
	t.Helper()
	for {
		select {
		case got := <-e.seen:
			if got == method {
				return
			}
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v", method, ctx.Err())
		}
	}
}

// callback sends one engine-to-plugin callback and fails on an error answer.
func (e *ospfRollbackEngine) callback(t *testing.T, ctx context.Context, method string, input any) json.RawMessage {
	t.Helper()
	raw, err := e.mux.CallRPC(ctx, method, input)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if len(raw) == 0 {
		return raw
	}
	var status struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("%s answer %s: %v", method, raw, err)
	}
	if status.Status == rpc.StatusError {
		t.Fatalf("%s refused: %s", method, status.Error)
	}
	return raw
}

// advertises reports whether the LSDB holds a live AS-External-LSA whose Link
// State ID is linkStateID, read through `show ospf database external`. A
// withdrawn LSA stays in the LSDB at MaxAge until it is flushed, so an LSA at
// MaxAge (RFC 2328 Appendix B, 3600 seconds) is not advertised.
func (e *ospfRollbackEngine) advertises(t *testing.T, ctx context.Context, linkStateID string) bool {
	t.Helper()
	answer, err := e.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Serial: "1", Command: cmdShowDatabaseExternal})
	if err != nil {
		t.Fatalf("show ospf database external: %v", err)
	}
	// One document answer: the head, one record carrying the whole reply, the
	// terminator (pkg/plugin/sdk/sdk_callbacks.go, executeCommandAnswer).
	var document json.RawMessage
	for record := range answer.Records {
		document = record.Item
	}
	if answerErr := answer.Err(); answerErr != nil {
		t.Fatalf("show ospf database external answer: %v", answerErr)
	}
	var snapshots []struct {
		ASExternal []struct {
			LinkStateID string `json:"link-state-id"`
			Age         uint16 `json:"age"`
		} `json:"as-external"`
	}
	if err := json.Unmarshal(document, &snapshots); err != nil {
		t.Fatalf("show ospf database external data %s: %v", document, err)
	}
	const maxAge = 3600
	for _, snapshot := range snapshots {
		for _, lsa := range snapshot.ASExternal {
			if lsa.LinkStateID == linkStateID && lsa.Age < maxAge {
				return true
			}
		}
	}
	return false
}

// TestOSPFConfigRollbackRestoresExportChain proves the plugin's registered
// rollback callback takes the export chain back. Method: run the real OSPF
// plugin over a pipe, start it with config A (export final accept), offer a
// redistributed route, apply config B (export final reject) through
// config-verify and config-apply, then send config-rollback. The installed
// chain must accept the route again and the LSDB must hold its
// AS-External-LSA live again, read through `show ospf database external`.
// Without the OnConfigRollback registration the SDK answers the rollback as a
// no-op and B's chain stays.
func TestOSPFConfigRollbackRestoresExportChain(t *testing.T) {
	const (
		// RFC 2328 Section 12.4 spaces two instances of one LSA by
		// MinLSInterval, so the rollback's re-origination of the LSA config B
		// purged would wait the default five seconds. One millisecond, waited
		// out before the rollback, lets it originate at once.
		cfgA          = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":1},"filter":{"export":{"final":"accept"}}}}`
		cfgB          = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":1},"filter":{"export":{"final":"reject"}}}}`
		minLSInterval = time.Millisecond
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
	previous, hadPrevious := configredist.LookupConsumer(ospfredistribute.ConsumerName)
	t.Cleanup(func() {
		if hadPrevious {
			configredist.ReregisterConsumer(previous)
		}
	})

	pluginEnd, engineEnd := net.Pipe()
	eng := &ospfRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	go eng.serve(ctx)
	done := make(chan struct{})
	go func() {
		runOSPFEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = eng.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("OSPF engine did not exit after its transport closed")
		}
	})

	eng.waitFor(t, ctx, "ze-plugin-engine:declare-registration")
	eng.callback(t, ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfgA}}})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")

	var consumer configredist.RedistConsumer
	for consumer == nil {
		if c, ok := configredist.LookupConsumer(ospfredistribute.ConsumerName); ok && (!hadPrevious || c != previous) {
			consumer = c
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("OnStarted never registered the OSPF redistribution consumer")
		case <-time.After(10 * time.Millisecond):
		}
	}

	prefix := netip.MustParsePrefix("192.0.2.1/32")
	entry := configredist.RouteEntry{Prefix: prefix.String(), Source: "static"}
	entry.Route.Prefix = prefix
	consumer.InjectRoute(ctx, family.IPv4Unicast, entry)
	if !eng.advertises(t, ctx, prefix.Addr().String()) {
		t.Fatal("config A: chain X (final accept) did not originate the offered route")
	}

	eng.callback(t, ctx, "ze-plugin-callback:config-verify",
		rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfgB}}})
	eng.callback(t, ctx, "ze-plugin-callback:config-apply",
		rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: Namespace, Changed: cfgB}}})
	if eng.advertises(t, ctx, prefix.Addr().String()) {
		t.Fatal("config B: chain Y (final reject) still advertises the route")
	}

	time.Sleep(2 * minLSInterval)
	eng.callback(t, ctx, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-b"})
	chain := ospfredistribute.ExportChain()
	if chain == nil {
		t.Fatal("after rollback no export chain is installed, want chain X")
	}
	if got := chain.Evaluate(entry.Route).Action; got != ribpolicy.ActionAccept {
		t.Fatalf("after rollback the installed chain answers %v for the route, want accept (chain X)", got)
	}
	if !eng.advertises(t, ctx, prefix.Addr().String()) {
		t.Fatal("after rollback the route chain X accepts is not advertised again")
	}
}
