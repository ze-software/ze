// Design: docs/architecture/isis/isis-11-redistribution.md -- export chain on reload and rollback.

package isis

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// isisRollbackEngine is the engine side of one IS-IS plugin connection. It
// answers every request the plugin sends and reports each method on seen, so
// the test can follow the five-stage startup.
type isisRollbackEngine struct {
	mux  *rpc.MuxConn
	seen chan string
}

// serve answers each plugin request until the connection closes: runtime state
// is absent and a boot count increments to one, and every other request is
// acknowledged.
func (e *isisRollbackEngine) serve(ctx context.Context) {
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
func (e *isisRollbackEngine) waitFor(t *testing.T, ctx context.Context, method string) {
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
func (e *isisRollbackEngine) callback(t *testing.T, ctx context.Context, method string, input any) json.RawMessage {
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

// advertises reports whether an own LSP carries a TLV 135 entry for the packed
// IPv4 prefix hex, read through `show isis database detail`.
func (e *isisRollbackEngine) advertises(t *testing.T, ctx context.Context, packed string) bool {
	t.Helper()
	answer, err := e.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Serial: "1", Command: cmdShowDatabaseDetail})
	if err != nil {
		t.Fatalf("show isis database detail: %v", err)
	}
	// One document answer: the head, one record carrying the whole reply, the
	// terminator (pkg/plugin/sdk/sdk_callbacks.go, executeCommandAnswer).
	var document json.RawMessage
	for record := range answer.Records {
		document = record.Item
	}
	if answerErr := answer.Err(); answerErr != nil {
		t.Fatalf("show isis database detail answer: %v", answerErr)
	}
	var lsps []struct {
		Own  bool `json:"own"`
		TLVs []struct {
			Type  uint8  `json:"type"`
			Value string `json:"value"`
		} `json:"tlvs"`
	}
	if err := json.Unmarshal(document, &lsps); err != nil {
		t.Fatalf("show isis database detail data %s: %v", document, err)
	}
	for _, lsp := range lsps {
		if !lsp.Own {
			continue
		}
		for _, tlv := range lsp.TLVs {
			if tlv.Type == 135 && strings.Contains(strings.ToLower(tlv.Value), packed) {
				return true
			}
		}
	}
	return false
}

// TestISISConfigRollbackRestoresExportChain proves the plugin's registered
// rollback callback takes the export chain back. Method: run the real IS-IS
// plugin over a pipe, start it with config A (export final accept), offer a
// redistributed route, apply config B (export final reject) through
// config-verify and config-apply, then send config-rollback. The installed
// chain must accept the route again and the own LSP must advertise it again,
// read through `show isis database detail`. Without the OnConfigRollback
// registration the SDK answers the rollback as a no-op and B's chain stays.
func TestISISConfigRollbackRestoresExportChain(t *testing.T) {
	const (
		cfgA = `{"isis":{"net":"49.0001.0000.0000.0001.00","filter":{"export":{"final":"accept"}}}}`
		cfgB = `{"isis":{"net":"49.0001.0000.0000.0001.00","filter":{"export":{"final":"reject"}}}}`
		// TLV 135 packs 192.0.2.1/32 as its length octet 0x20, then the address.
		packed = "20c0000201"
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Cleanup(func() { isisredistribute.SetExportChain(nil) })
	previous, hadPrevious := configredist.LookupConsumer(isisredistribute.ConsumerName)
	t.Cleanup(func() {
		if hadPrevious {
			configredist.ReregisterConsumer(previous)
		}
	})

	pluginEnd, engineEnd := net.Pipe()
	eng := &isisRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	go eng.serve(ctx)
	done := make(chan struct{})
	go func() {
		runISISEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = eng.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("IS-IS engine did not exit after its transport closed")
		}
	})

	eng.waitFor(t, ctx, "ze-plugin-engine:declare-registration")
	eng.callback(t, ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: cfgA}}})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")

	var consumer configredist.RedistConsumer
	for consumer == nil {
		if c, ok := configredist.LookupConsumer(isisredistribute.ConsumerName); ok && (!hadPrevious || c != previous) {
			consumer = c
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("OnStarted never registered the IS-IS redistribution consumer")
		case <-time.After(10 * time.Millisecond):
		}
	}

	prefix := netip.MustParsePrefix("192.0.2.1/32")
	entry := configredist.RouteEntry{Prefix: prefix.String(), Source: "static"}
	entry.Route.Prefix = prefix
	consumer.InjectRoute(ctx, family.IPv4Unicast, entry)
	if !eng.advertises(t, ctx, packed) {
		t.Fatal("config A: chain X (final accept) did not advertise the offered route")
	}

	eng.callback(t, ctx, "ze-plugin-callback:config-verify",
		rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: cfgB}}})
	eng.callback(t, ctx, "ze-plugin-callback:config-apply",
		rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: cfgB}}})
	if eng.advertises(t, ctx, packed) {
		t.Fatal("config B: chain Y (final reject) still advertises the route")
	}

	eng.callback(t, ctx, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-b"})
	chain := isisredistribute.ExportChain()
	if chain == nil {
		t.Fatal("after rollback no export chain is installed, want chain X")
	}
	if got := chain.Evaluate(entry.Route).Action; got != ribpolicy.ActionAccept {
		t.Fatalf("after rollback the installed chain answers %v for the route, want accept (chain X)", got)
	}
	if !eng.advertises(t, ctx, packed) {
		t.Fatal("after rollback the route chain X accepts is not advertised again")
	}
}
