package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin01APIBGPSummary(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	data, err := plugin01RequireDone(ctx, plugin, "show bgp")
	if err != nil {
		return err
	}
	configured, exists := data["peers-configured"]
	if !exists {
		return fmt.Errorf("no peers-configured in %v", data)
	}
	found := false
	for _, value := range plugin01Array(data["peers"]) {
		if plugin01Object(value)["address"] == addrLoopback {
			found = true
		}
	}
	if !found {
		return errors.New("127.0.0.1 not in summary peers")
	}
	fmt.Fprintf(os.Stderr, "OK: summary has %.0f peer(s) configured\n", plugin01Number(configured))
	return nil
}

func plugin01APICacheForward(ctx context.Context, plugin *sdk.Plugin) error {
	_, status, err := plugin01DispatchMap(ctx, plugin, "request cache forward 999 127.0.0.1")
	if err != nil {
		if (status != "" && status != rpc.StatusError) || !strings.HasPrefix(err.Error(), "rpc error:") {
			return fmt.Errorf("cache forward: status=%q: %w", status, err)
		}
		status = rpc.StatusError
	}
	if status != rpc.StatusDone && status != rpc.StatusError {
		return fmt.Errorf("cache forward: unexpected status=%q", status)
	}
	fmt.Fprintf(os.Stderr, "cache forward status=%s\nOK: cache forward dispatched\n", status)
	if !plugin01WaitCounter(ctx, plugin, "*", "eor-sent", 1, 40) {
		return errors.New("ze did not send the End-of-RIB to the peer before shutdown")
	}
	return nil
}

func plugin01APICacheOps(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := plugin01RequireDone(ctx, plugin, "show cache"); err != nil {
		return err
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: cache list dispatched")
	return nil
}

func plugin01WaitEORAndQuiesce(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitCounter(ctx, plugin, "peer1", "eor-sent", 1, 40) {
		return errors.New("peer1 initial-sync EOR never reached the wire")
	}
	return plugin01Quiesce(ctx, plugin)
}

func plugin01APICommitLifecycle(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	for _, command := range []string{"request commit rollback-test start", "request commit rollback-test rollback", "request commit list"} {
		if _, err := plugin01RequireDone(ctx, plugin, command); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "OK: commit start/rollback/list lifecycle completed")
	return nil
}

func plugin01APICommitWorkflow(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	for _, command := range []string{"request commit test-commit start", "request commit test-commit show", "request commit test-commit end"} {
		if _, err := plugin01RequireDone(ctx, plugin, command); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "OK: commit start/show/end workflow completed")
	return nil
}

type plugin01HTTPError struct {
	status int
	body   []byte
}

func (e *plugin01HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.status, e.body) }

func plugin01REST(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:18088/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close() //nolint:errcheck // the fixture only reads the body, so a close failure changes no assertion
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &plugin01HTTPError{status: response.StatusCode, body: raw}
	}
	result := map[string]any{}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func plugin01RESTReady(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:18088/api/v1/commands", http.NoBody)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer secret")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	defer response.Body.Close() //nolint:errcheck // the fixture only reads the body, so a close failure changes no assertion
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func plugin01APIConfigCommitReject(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 20, 500*time.Millisecond, func() bool {
		return plugin01RESTReady(ctx)
	}) {
		return errors.New("REST API did not respond")
	}
	created, err := plugin01REST(ctx, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return err
	}
	sessionID, _ := created["session-id"].(string)
	if sessionID == "" {
		return fmt.Errorf("create session response=%v", created)
	}
	if _, err := plugin01REST(ctx, http.MethodPut, "/config/sessions/"+sessionID,
		map[string]string{fieldPath: "bgp.peer.peer1.timer.receive-hold-time", fieldValue: "2"}); err != nil {
		return err
	}
	_, err = plugin01REST(ctx, http.MethodPost, "/config/sessions/"+sessionID+"/commit", nil)
	var httpErr *plugin01HTTPError
	if err == nil {
		return errors.New("invalid commit unexpectedly succeeded")
	}
	if !errors.As(err, &httpErr) {
		return err
	}
	body := string(httpErr.body)
	if !strings.Contains(body, "receive-hold-time") && !strings.Contains(body, "reload") && !strings.Contains(body, "commit") {
		return fmt.Errorf("unexpected commit rejection status=%d body=%s", httpErr.status, body)
	}
	diff, err := plugin01REST(ctx, http.MethodGet, "/config/sessions/"+sessionID+"/diff", nil)
	if err != nil {
		return err
	}
	diffText, _ := diff["diff"].(string)
	if !strings.Contains(diffText, "receive-hold-time") {
		return fmt.Errorf("rejected session was not kept open: %v", diff)
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: REST config commit rejected and session kept open")
	return nil
}

func plugin01APIDoctorProvider(ctx context.Context, _ []string) error {
	plugin, err := newObserver("doctor-provider")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	plugin.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
		if name != "test-cache-reachable" {
			return nil, nil
		}
		return []rpc.DoctorCheckDiagnostic{{
			Code: codeDoctorTestCacheDown, Severity: severityWarning, Message: "test cache not responding",
		}}, nil
	})
	return plugin.Run(ctx, sdk.Registration{DoctorChecks: []sdk.DoctorCheckDecl{{
		Name: "test-cache-reachable", Phase: sdk.DoctorPhasePostConfig, Codes: []string{codeDoctorTestCacheDown},
	}}})
}

func plugin01APIDoctorChecker(ctx context.Context, plugin *sdk.Plugin) error {
	var data map[string]any
	ok := Poll(ctx, 40, plugin01PollDelay, func() bool {
		var status string
		var err error
		data, status, err = plugin01DispatchMap(ctx, plugin, "show doctor")
		if err != nil || status != rpc.StatusDone {
			return false
		}
		for _, value := range plugin01Array(data["diagnostics"]) {
			if plugin01Object(value)["code"] == codeDoctorTestCacheDown {
				return true
			}
		}
		return false
	})
	if !ok {
		return fmt.Errorf("plugin doctor check diagnostic missing: %v", data)
	}
	for _, value := range plugin01Array(data["diagnostics"]) {
		diagnostic := plugin01Object(value)
		if diagnostic["code"] != codeDoctorTestCacheDown {
			continue
		}
		if diagnostic["severity"] != severityWarning || !strings.Contains(fmt.Sprint(diagnostic["message"]), "test cache not responding") {
			return fmt.Errorf("wrong diagnostic: %v", diagnostic)
		}
		return nil
	}
	return errors.New("plugin doctor check diagnostic disappeared")
}

func plugin01APIPeerCapabilities(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitPeers(ctx, plugin, 1, 40) {
		return errors.New("no peer reached established")
	}
	data, err := plugin01RequireDone(ctx, plugin, "show bgp peer peer1 capabilities")
	if err != nil {
		return err
	}
	rows := plugin01Array(data["peers"])
	if len(rows) != 1 {
		return fmt.Errorf("expected one peer row, got %v", rows)
	}
	row := plugin01Object(rows[0])
	negotiated := plugin01Object(row["negotiated"])
	if row["peer"] != addrLoopback || len(negotiated) == 0 {
		return fmt.Errorf("unexpected capability row: %v", row)
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: capabilities has %d entries for 127.0.0.1\n", len(negotiated))
	return nil
}

func plugin01APIPeerList(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show bgp peer list")
	if err != nil {
		return err
	}
	if _, ok := plugin01PeerRows(data)["127.0.0.1"]; !ok {
		return errors.New("127.0.0.1 not in peers")
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: peer list contains 127.0.0.1")
	return nil
}

func plugin01APIPeerPauseResume(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "request peer peer1 pause"); err != nil {
		return err
	}
	data, err := plugin01RequireDone(ctx, plugin, "show bgp peer peer1 detail")
	if err != nil {
		return err
	}
	state := plugin01Object(plugin01PeerRows(data)["127.0.0.1"])["state"]
	fmt.Fprintf(os.Stderr, "peer state after pause: %v\n", state)
	if _, err := plugin01RequireDone(ctx, plugin, "request peer peer1 resume"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: pause/resume cycle completed")
	return nil
}

func plugin01APIPeerPrefixUpdate(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "update bgp peer * prefix")
	if err != nil {
		return err
	}
	results := plugin01Array(data["results"])
	if plugin01Number(data["updated"]) != 1 || len(results) != 1 {
		return fmt.Errorf("unexpected prefix update result: %v", data)
	}
	result := plugin01Object(results[0])
	changes := plugin01Object(result["changes"])
	if result["status"] != statusUpdated || plugin01Number(changes["ipv4/unicast"]) != 71501 {
		return fmt.Errorf("unexpected peer prefix changes: %v", result)
	}
	fmt.Fprintln(os.Stderr, "OK: prefix update succeeded, ipv4=71501")
	return nil
}

func plugin01APIPeerRemove(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show bgp peer list")
	if err != nil {
		return err
	}
	if _, ok := plugin01PeerRows(data)["127.0.0.1"]; !ok {
		return errors.New("config peer 127.0.0.1 not in initial peer list")
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "delete bgp peer 127.0.0.1"); err != nil {
		return err
	}
	if !Poll(ctx, 40, plugin01PollDelay, func() bool {
		current, status, dispatchErr := plugin01DispatchMap(ctx, plugin, "show bgp peer list")
		if dispatchErr != nil || status != rpc.StatusDone {
			return false
		}
		_, exists := plugin01PeerRows(current)["127.0.0.1"]
		return !exists
	}) {
		return errors.New("127.0.0.1 still in peers after delete")
	}
	fmt.Fprintln(os.Stderr, "OK: peer delete verified")
	return nil
}

func plugin01APIPeerShow(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show bgp peer peer1 detail")
	if err != nil {
		return err
	}
	peer := plugin01Object(plugin01PeerRows(data)["127.0.0.1"])
	if _, ok := peer["state"]; !ok {
		return errors.New("no state in peer detail")
	}
	if _, ok := peer["remote-as"]; !ok {
		return errors.New("no remote-as in peer detail")
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: peer detail has state=%v remote-as=%v\n", peer["state"], peer["remote-as"])
	return nil
}

func plugin01APIRaw(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitPeers(ctx, plugin, 1, 40) {
		return errors.New("no peer reached established")
	}
	if _, err := plugin01RequireDone(ctx, plugin, "peer peer1 raw hex FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"); err != nil {
		return err
	}
	// The raw socket write exposes no per-message delivery signal. Retain the
	// original settle before shutdown so the peer can consume the just-written frame.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	fmt.Fprintln(os.Stderr, "OK: raw keepalive sent")
	return nil
}

func plugin01APIRIBInClear(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "clear bgp rib in *"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: rib clear in dispatched")
	return nil
}

// plugin01APIAnnounceUnicast dispatches the operator's own `announce unicast`
// command and lets the peer block judge what reached the wire.
//
// The command answers before the UPDATE is encoded, so the dispatch status only
// proves the handler accepted the words. The proof that the route was
// originated is the peer's expectation of the UPDATE that follows the EOR.
func plugin01APIAnnounceUnicast(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "announce unicast 198.51.100.0/24 next-hop 10.0.1.254 community 65001:666"); err != nil {
		return err
	}
	// The peer-scoped form is the same wire method at a second command path. It
	// proves the dispatcher fills the selector slot the model declares on the
	// peer container.
	if _, err := plugin01RequireDone(ctx, plugin, "peer 127.0.0.1 announce unicast 203.0.113.0/24 next-hop 10.0.1.254"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: announce unicast dispatched, bare and peer-scoped")
	return nil
}

func plugin01APIRIBInShow(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "show bgp rib received"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: show bgp rib received dispatched")
	return nil
}
