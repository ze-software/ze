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

func plugin01APISendCached(ctx context.Context, plugin *sdk.Plugin) error {
	_, status, err := plugin01DispatchMap(ctx, plugin, "send bgp 127.0.0.1 cached 999")
	if err != nil {
		if (status != "" && status != rpc.StatusError) || !strings.HasPrefix(err.Error(), "rpc error:") {
			return fmt.Errorf("send bgp cached: status=%q: %w", status, err)
		}
		status = rpc.StatusError
	}
	if status != rpc.StatusDone && status != rpc.StatusError {
		return fmt.Errorf("send bgp cached: unexpected status=%q", status)
	}
	fmt.Fprintf(os.Stderr, "send bgp cached status=%s\nOK: send bgp cached dispatched\n", status)
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
	return plugin01RESTAt(ctx, "http://127.0.0.1:18088", method, path, payload)
}

func plugin01RESTAt(ctx context.Context, endpoint, method, path string, payload any) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint+"/api/v1"+path, body)
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
	return plugin01RESTReadyAt(ctx, "http://127.0.0.1:18088")
}

func plugin01RESTReadyAt(ctx context.Context, endpoint string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/api/v1/commands", http.NoBody)
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

// plugin01RESTRefusalAt answers the error text of a REST call the daemon must
// refuse with HTTP 400, decoded from the JSON envelope, or an error naming
// what the call answered instead.
func plugin01RESTRefusalAt(ctx context.Context, endpoint, method, path string, body any) (string, error) {
	_, err := plugin01RESTAt(ctx, endpoint, method, path, body)
	if err == nil {
		return "", fmt.Errorf("%s %s was accepted, a refusal was expected", method, path)
	}
	var httpErr *plugin01HTTPError
	if !errors.As(err, &httpErr) {
		return "", err
	}
	if httpErr.status != http.StatusBadRequest {
		return "", fmt.Errorf("%s %s answered HTTP %d, want 400: %s", method, path, httpErr.status, httpErr.body)
	}
	var envelope struct {
		Error string `json:"error"`
	}
	if decodeErr := json.Unmarshal(httpErr.body, &envelope); decodeErr != nil {
		return "", fmt.Errorf("%s %s refusal is not the JSON envelope: %w: %s", method, path, decodeErr, httpErr.body)
	}
	return envelope.Error, nil
}

// plugin01RefusalHolds answers an error naming the first of want the refusal
// text does not hold, or the first of banned it holds.
func plugin01RefusalHolds(label, text string, want, banned []string) error {
	for _, piece := range want {
		if !strings.Contains(text, piece) {
			return fmt.Errorf("%s refusal lacks %q: %q", label, piece, text)
		}
	}
	for _, piece := range banned {
		if strings.Contains(text, piece) {
			return fmt.Errorf("%s refusal holds %q: %q", label, piece, text)
		}
	}
	return nil
}

// plugin01APIConfigEnumRefusal drives a REST config session through the two
// refusals a session meets (spec-config-enum-refusal-proposes-valid-values):
// a list key outside its enumeration, refused at set (AC-43), and a value
// outside an enumeration, refused at commit (AC-8). Each answers HTTP 400
// with the shared refusal text, whose fix is the root `set` form.
func plugin01APIConfigEnumRefusal(ctx context.Context, plugin *sdk.Plugin, endpoint string) error {
	if !Poll(ctx, 20, 500*time.Millisecond, func() bool {
		return plugin01RESTReadyAt(ctx, endpoint)
	}) {
		return errors.New("REST API did not respond")
	}
	created, err := plugin01RESTAt(ctx, endpoint, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return err
	}
	sessionID, _ := created["session-id"].(string)
	if sessionID == "" {
		return fmt.Errorf("create session response=%v", created)
	}
	session := "/config/sessions/" + sessionID

	keyText, err := plugin01RESTRefusalAt(ctx, endpoint, http.MethodPut, session,
		map[string]string{fieldPath: "ospf.redistribute.bogus.nssa-propagate", fieldValue: "true"})
	if err != nil {
		return err
	}
	if err := plugin01RefusalHolds("key", keyText,
		[]string{`"bogus" is not an accepted value`, configKeyRefusalFix},
		[]string{"line "}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: REST session set refused the list key bogus with HTTP 400 and the set fix")

	if _, err := plugin01RESTAt(ctx, endpoint, http.MethodPut, session,
		map[string]string{fieldPath: "bgp.peer.peer1.session.capability.route-refresh", fieldValue: "requre"}); err != nil {
		return err
	}
	valueText, err := plugin01RESTRefusalAt(ctx, endpoint, http.MethodPost, session+"/commit", nil)
	if err != nil {
		return err
	}
	if err := plugin01RefusalHolds("commit", valueText,
		[]string{
			`bgp peer peer1 session capability route-refresh: "requre" is not an accepted value`,
			"route-refresh accepts only these values:",
			"fix: set bgp peer peer1 session capability route-refresh require",
		},
		[]string{"line ", "route-refresh require;"}); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.TrimRight(valueText, "\n"), "fix: set bgp peer peer1 session capability route-refresh require") {
		return fmt.Errorf("commit refusal does not end with its fix: %q", valueText)
	}
	fmt.Fprintln(os.Stderr, "OK: REST session commit refused route-refresh requre with HTTP 400 and the set fix last")

	created, err = plugin01RESTAt(ctx, endpoint, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return err
	}
	sessionID, _ = created["session-id"].(string)
	if sessionID == "" {
		return fmt.Errorf("second create session response=%v", created)
	}
	session = "/config/sessions/" + sessionID
	for path, value := range map[string]string{
		"bgp.peer.peer1.session.capability.route-refresh": "requre",
		"ospf.interfaces.interface.eth0.cost":             "0",
	} {
		if _, err := plugin01RESTAt(ctx, endpoint, http.MethodPut, session,
			map[string]string{fieldPath: path, fieldValue: value}); err != nil {
			return err
		}
	}
	multiple, err := plugin01RESTRefusalAt(ctx, endpoint, http.MethodPost, session+"/commit", nil)
	if err != nil {
		return err
	}
	if err := plugin01RefusalHolds("two-value commit", multiple, []string{
		"config validation failed:\n",
		`bgp peer peer1 session capability route-refresh: "requre" is not an accepted value`,
		`ospf interfaces interface eth0 cost: "0" is outside the accepted range`,
		"  fix: set bgp peer peer1 session capability route-refresh require",
		"  fix: set ospf interfaces interface eth0 cost <1..65535>",
	}, []string{"line "}); err != nil {
		return err
	}
	if strings.Count(multiple, "\n  fix: ") != 2 {
		return fmt.Errorf("two-value commit does not hold two fix lines: %q", multiple)
	}
	for _, line := range strings.Split(multiple, "\n")[1:] {
		if !strings.HasPrefix(line, "  ") {
			return fmt.Errorf("two-value commit line lacks block indentation: %q", line)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: REST session commit returned both value refusals as indented blocks")

	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
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
	return plugin.Run(ctx, &sdk.Registration{DoctorChecks: []sdk.DoctorCheckDecl{{
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
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
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

// plugin01RawRefreshWire is one complete ROUTE-REFRESH for ipv4/unicast: marker,
// length 23, type 5, AFI 1, reserved 0, SAFI 1 (RFC 2918 Section 3). `peer <sel>
// raw hex <data>` with no type word carries a whole packet the caller built.
//
// ze builds a ROUTE-REFRESH only where a command asks for one
// (reactor/reactor_api.go and reactor/reactor_api_forward.go hold both
// producers), and test/plugin/api-raw.ci asks for none, so the frame the peer
// expects can only be this injection. A KEEPALIVE cannot carry that claim: ze
// sends its own and the two are byte-identical.
const plugin01RawRefreshWire = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170500010001"

func plugin01APIRaw(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitPeers(ctx, plugin, 1, 40) {
		return errors.New("no peer reached established")
	}
	if _, err := plugin01RequireDone(ctx, plugin, "send bgp peer1 raw hex "+plugin01RawRefreshWire); err != nil {
		return err
	}
	if !plugin01WaitSessionDown(ctx, plugin, "peer1") {
		return errors.New("peer1 stayed established, so the peer never completed on the injected ROUTE-REFRESH")
	}
	fmt.Fprintln(os.Stderr, "OK: raw route-refresh sent and consumed by the peer")
	return nil
}

// plugin01WaitSessionDown blocks until the named peer's session has left
// Established, and reports whether it did.
//
// It is the delivery barrier the raw write does not offer. ze-peer runs in check
// mode, so it returns the moment its expectation list is empty and the process
// exits; the .ci owes it a frame only the injection produces, so the session
// drops BECAUSE the peer read the injected bytes. Shutting the daemon down on a
// fixed sleep instead raced that read.
//
// A peer row that reports no state at all is not an answer: the poll keeps
// asking, and the caller names the failure when the attempts run out.
func plugin01WaitSessionDown(ctx context.Context, plugin *sdk.Plugin, peer string) bool {
	return Poll(ctx, 40, plugin01PollDelay, func() bool {
		data, _, err := plugin01DispatchMap(ctx, plugin, "show bgp peer "+peer+" detail")
		if err != nil {
			return false
		}
		for _, value := range plugin01PeerRows(data) {
			state, named := plugin01Object(value)["state"].(string)
			if !named {
				continue
			}
			if state != stateEstablished {
				return true
			}
		}
		return false
	})
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

// plugin01APIAnnounceUnicast dispatches the operator's own `send bgp * unicast`
// command and lets the peer block judge what reached the wire.
//
// The command answers before the UPDATE is encoded, so the dispatch status only
// proves the handler accepted the words. The proof that the route was
// originated is the peer's expectation of the UPDATE that follows the EOR.
func plugin01APIAnnounceUnicast(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "send bgp * unicast 198.51.100.0/24 next-hop 10.0.1.254 community 65001:666"); err != nil {
		return err
	}
	// The named selector is the same wire method reaching one session rather
	// than every one. It proves the dispatcher fills the selector slot the model
	// declares on the bgp container.
	//
	// The tag is what makes the announcement TRACKED, so the withdraw below has
	// something to find. An untagged announce is fire-and-forget and never
	// enters the registry.
	if _, err := plugin01RequireDone(ctx, plugin, "send bgp 127.0.0.1 unicast 203.0.113.0/24 next-hop 10.0.1.254 tag scope peer"); err != nil {
		return err
	}
	// The same prefix on the withdraw side. It withdraws the announcement made
	// to this fan-out and leaves the untagged one alone, which the peer block
	// asserts by expecting exactly one withdraw.
	if _, err := plugin01RequireDone(ctx, plugin, "send bgp 127.0.0.1 withdraw all"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: unicast and withdraw dispatched, wildcard and named selector")
	return nil
}

// plugin01APIAnnounceFlowspecExtraToken drives the reported defect of 2026-08-31
// over the daemon's own dispatcher: `send bgp * flowspec destination-ipv4
// 1.1.1.1/32 discard rate-limit 500` announced a plain discard and threw
// `rate-limit 500` away without a word.
//
// The command must answer an error naming the token, because the operator asked
// for a rate limit and a silent stop gives them a blackhole instead. The peer
// block asserts the other half: nothing reaches the wire.
func plugin01APIAnnounceFlowspecExtraToken(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	const command = "send bgp * flowspec destination-ipv4 1.1.1.1/32 discard rate-limit 500"
	data, status, err := plugin01DispatchMap(ctx, plugin, command)
	if err != nil {
		return err
	}
	if status != rpc.StatusError {
		return fmt.Errorf("%s: status=%s data=%v, want an error naming rate-limit", command, status, data)
	}
	message, _ := data[fieldError].(string)
	if !strings.Contains(message, "rate-limit") {
		return fmt.Errorf("%s: error %q does not name the token the operator typed", command, message)
	}
	fmt.Fprintln(os.Stderr, "OK: send bgp flowspec refused the word after the action")
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
