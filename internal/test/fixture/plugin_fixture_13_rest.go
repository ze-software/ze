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
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rest-api-commands", restAPICommands13)
	Register("plugin/rest-execute", restExecute13)
	Register("plugin/rest-no-auth-readonly", restNoAuthReadonly13)
	Register("plugin/rest-peer-set-delete-lifecycle", restPeerLifecycle13)
}

type httpResult13 struct {
	status int
	body   []byte
}

func httpRequest13(ctx context.Context, method, url string, headers map[string]string, payload any) (httpResult13, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return httpResult13{}, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return httpResult13{}, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return httpResult13{}, err
	}
	defer response.Body.Close() //nolint:errcheck
	raw, err := io.ReadAll(response.Body)
	return httpResult13{status: response.StatusCode, body: raw}, err
}

func waitHTTP13(ctx context.Context, url string, headers map[string]string, attempts int, delay time.Duration) (httpResult13, bool) {
	var result httpResult13
	ok := Poll(ctx, attempts, delay, func() bool {
		requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var err error
		result, err = httpRequest13(requestCtx, http.MethodGet, url, headers, nil)
		return err == nil && result.status >= 200 && result.status < 300
	})
	return result, ok
}

func decodeObject13(raw []byte) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func restAPICommands13(ctx context.Context, args []string) error {
	return observe13(ctx, "rest-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		const base = "http://127.0.0.1:18081/api/v1"
		commandsResult, ok := waitHTTP13(ctx, base+"/commands", nil, 20, 500*time.Millisecond)
		if !ok {
			return errors.New("REST API did not respond within timeout")
		}
		var commands []map[string]any
		if err := json.Unmarshal(commandsResult.body, &commands); err != nil {
			return fmt.Errorf("decode commands: %w", err)
		}
		if len(commands) == 0 {
			return errors.New("empty command list")
		}
		advertised := false
		for _, command := range commands {
			if command["Name"] == "show bgp" {
				advertised = true
				break
			}
		}
		if !advertised {
			return fmt.Errorf("command list does not advertise show bgp (%d commands)", len(commands))
		}
		fmt.Fprintf(os.Stderr, "OK: REST /api/v1/commands returned %d commands\n", len(commands))
		result, err := httpRequest13(ctx, http.MethodPost, base+"/execute", map[string]string{"Content-Type": "application/json"}, map[string]any{"command": "show bgp"})
		if err != nil {
			return err
		}
		answer, err := decodeObject13(result.body)
		if err != nil {
			return fmt.Errorf("decode execute response (HTTP %d): %w: %.200s", result.status, err, result.body)
		}
		if answer["status"] != "done" {
			return fmt.Errorf("execute status=%v error=%v data=%v", answer["status"], answer["error"], answer["data"])
		}
		if err := requireBGPPeerSummary13(answer["data"], "127.0.0.1"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: REST /api/v1/execute returned peer data")
		return nil
	})
}

func requireBGPPeerSummary13(value any, address string) error {
	var summary map[string]any
	switch data := value.(type) {
	case map[string]any:
		summary = data
	case string:
		if err := json.Unmarshal([]byte(data), &summary); err != nil {
			return err
		}
	default:
		return fmt.Errorf("execute returned non-object data: %v", value)
	}
	if _, ok := summary["peers-configured"]; !ok {
		return fmt.Errorf("execute returned no peers-configured: %v", summary)
	}
	peers, _ := summary["peers"].([]any)
	for _, value := range peers {
		peer, _ := value.(map[string]any)
		if peer["address"] == address {
			return nil
		}
	}
	return fmt.Errorf("execute did not list peer %s: %v", address, summary)
}

func restExecute13(ctx context.Context, args []string) error {
	return observe13(ctx, "rest-exec-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		const base = "http://127.0.0.1:18082/api/v1"
		if _, ok := waitHTTP13(ctx, base+"/commands", nil, 20, 500*time.Millisecond); !ok {
			return errors.New("REST API did not respond within timeout")
		}
		headers := map[string]string{"Content-Type": "application/json"}
		result, err := httpRequest13(ctx, http.MethodPost, base+"/execute", headers, map[string]any{"command": "show version"})
		if err != nil {
			return err
		}
		version, err := decodeObject13(result.body)
		if err != nil || version["status"] != "done" {
			return fmt.Errorf("show version status=%v HTTP=%d err=%v", version["status"], result.status, err)
		}
		fmt.Fprintln(os.Stderr, "OK: REST execute returned status=done")
		result, err = httpRequest13(ctx, http.MethodPost, base+"/execute", headers, map[string]any{"command": "show bgp", "params": map[string]any{}})
		if err != nil {
			return err
		}
		answer, err := decodeObject13(result.body)
		if err != nil || answer["status"] != "done" {
			return fmt.Errorf("show bgp status=%v HTTP=%d err=%v", answer["status"], result.status, err)
		}
		if err := requireBGPPeerSummary13(answer["data"], "127.0.0.1"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: REST execute with params returned peer data")
		return nil
	})
}

func restNoAuthReadonly13(ctx context.Context, args []string) error {
	return observe13(ctx, "rest-readonly-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		const base = "http://127.0.0.1:18086/api/v1"
		if _, ok := waitHTTP13(ctx, base+"/commands", nil, 20, 500*time.Millisecond); !ok {
			return errors.New("REST API did not respond")
		}
		headers := map[string]string{"Content-Type": "application/json"}
		result, err := httpRequest13(ctx, http.MethodPost, base+"/execute", headers, map[string]any{"command": "show version"})
		if err != nil {
			return err
		}
		body, err := decodeObject13(result.body)
		if err != nil || body["status"] != "done" {
			return fmt.Errorf("read command status=%v HTTP=%d err=%v", body["status"], result.status, err)
		}
		fmt.Fprintln(os.Stderr, "OK: no-auth REST read command allowed")
		result, err = httpRequest13(ctx, http.MethodPost, base+"/execute", headers, map[string]any{"command": "request reload"})
		if err != nil {
			return err
		}
		if result.status != http.StatusForbidden {
			return fmt.Errorf("no-auth REST write command status=%d, want 403", result.status)
		}
		fmt.Fprintln(os.Stderr, "OK: no-auth REST write command denied")
		result, err = httpRequest13(ctx, http.MethodPost, base+"/config/sessions", nil, nil)
		if err != nil {
			return err
		}
		if result.status != http.StatusForbidden {
			return fmt.Errorf("no-auth REST config session status=%d, want 403", result.status)
		}
		fmt.Fprintln(os.Stderr, "OK: no-auth REST config session denied")
		return waitEORSent13(ctx, plugin, "peer1", 1)
	})
}

type restLifecycle13 struct {
	base     string
	peerPort string
	headers  map[string]string
}

func (r restLifecycle13) request(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	result, err := httpRequest13(ctx, method, r.base+path, r.headers, payload)
	if err != nil {
		return nil, err
	}
	if result.status < 200 || result.status >= 300 {
		return nil, fmt.Errorf("HTTP %d %s %s: %s", result.status, method, path, result.body)
	}
	if len(result.body) == 0 {
		return map[string]any{}, nil
	}
	return decodeObject13(result.body)
}

func (r restLifecycle13) createSession(ctx context.Context) (string, error) {
	created, err := r.request(ctx, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return "", err
	}
	sid, _ := created["session-id"].(string)
	if sid == "" {
		return "", fmt.Errorf("create session failed: %v", created)
	}
	return sid, nil
}

func (r restLifecycle13) set(ctx context.Context, sid, path, value string) error {
	_, err := r.request(ctx, http.MethodPut, "/config/sessions/"+sid, map[string]any{"path": path, "value": value})
	return err
}

func (r restLifecycle13) diff(ctx context.Context, sid string) (string, error) {
	result, err := r.request(ctx, http.MethodGet, "/config/sessions/"+sid+"/diff", nil)
	if err != nil {
		return "", err
	}
	diff, _ := result["diff"].(string)
	return diff, nil
}

func (r restLifecycle13) addPeer(ctx context.Context, sid, name, remoteIP string, remoteAS int) error {
	prefix := "bgp.peer." + name
	values := [][2]string{
		{prefix + ".connection.remote.ip", remoteIP},
		{prefix + ".connection.remote.port", r.peerPort},
		{prefix + ".connection.local.ip", remoteIP},
		{prefix + ".connection.local.accept", "false"},
		{prefix + ".session.asn.local", "65000"},
		{prefix + ".session.asn.remote", strconv.Itoa(remoteAS)},
		{prefix + ".session.router-id", "1.2.3.4"},
		{prefix + ".session.family.ipv4/unicast.prefix.maximum", "10000"},
		{prefix + ".behavior.group-updates", "false"},
	}
	for _, value := range values {
		if err := r.set(ctx, sid, value[0], value[1]); err != nil {
			return err
		}
	}
	return nil
}

func (r restLifecycle13) commit(ctx context.Context, sid, label string) error {
	result, err := r.request(ctx, http.MethodPost, "/config/sessions/"+sid+"/commit", nil)
	if err != nil {
		return err
	}
	if result["status"] != "committed" {
		return fmt.Errorf("%s commit status: %v", label, result)
	}
	return nil
}

func peerState13(ctx context.Context, plugin *sdk.Plugin, address string) (string, bool) {
	result := command13(ctx, plugin, "show bgp peer * detail")
	if !done13(result) {
		return "", false
	}
	row, ok := peerRows13(result)[address].(map[string]any)
	if !ok {
		return "", false
	}
	state, _ := row["state"].(string)
	return state, true
}

func waitPeerPresent13(ctx context.Context, plugin *sdk.Plugin, address string) (string, bool) {
	var state string
	var present bool
	Poll(ctx, 24, 500*time.Millisecond, func() bool { state, present = peerState13(ctx, plugin, address); return present })
	return state, present
}

func restPeerLifecycle13(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("rest lifecycle fixture requires REST port and peer port")
	}
	driver := restLifecycle13{
		base:     "http://127.0.0.1:" + args[0] + "/api/v1",
		peerPort: args[1],
		headers:  map[string]string{"Authorization": "Bearer secret", "Content-Type": "application/json"},
	}
	return observe13(ctx, "lifecycle-driver", func(ctx context.Context, plugin *sdk.Plugin) error {
		if _, ok := waitHTTP13(ctx, driver.base+"/commands", driver.headers, 50, 300*time.Millisecond); !ok {
			return errors.New("REST API did not respond")
		}
		state, present := waitPeerPresent13(ctx, plugin, "127.0.0.1")
		if !present {
			return errors.New("peer-alpha not in show bgp peer")
		}
		fmt.Fprintf(os.Stderr, "OK: step 1 - peer-alpha state=%s\n", state)
		sid, err := driver.createSession(ctx)
		if err != nil {
			return err
		}
		if err = driver.addPeer(ctx, sid, "peer-beta", "127.0.0.2", 65002); err != nil {
			return err
		}
		diff, err := driver.diff(ctx, sid)
		if err != nil {
			return err
		}
		if !strings.Contains(diff, "peer-beta") {
			return fmt.Errorf("peer-beta absent from REST candidate diff: %q", diff)
		}
		if err = driver.commit(ctx, sid, "step 2"); err != nil {
			return err
		}
		beta, betaPresent := waitPeerPresent13(ctx, plugin, "127.0.0.2")
		alpha, alphaPresent := waitPeerPresent13(ctx, plugin, "127.0.0.1")
		if !betaPresent || !alphaPresent {
			snapshot := command13(ctx, plugin, "show bgp peer * detail")
			running, _ := driver.request(ctx, http.MethodGet, "/config/running", nil)
			return fmt.Errorf("after add: alpha present=%v beta present=%v peers=%s running=%v", alphaPresent, betaPresent, snapshot.text(), running)
		}
		fmt.Fprintf(os.Stderr, "OK: step 2 - peer-beta state=%s, peer-alpha state=%s\n", beta, alpha)
		sid, err = driver.createSession(ctx)
		if err != nil {
			return err
		}
		path := strings.ReplaceAll("bgp.peer.peer-alpha", ".", "/")
		if _, err = driver.request(ctx, http.MethodDelete, "/config/sessions/"+sid+"/"+path, nil); err != nil {
			return err
		}
		if err = driver.commit(ctx, sid, "step 3"); err != nil {
			return err
		}
		gone := Poll(ctx, 24, 500*time.Millisecond, func() bool { _, present := peerState13(ctx, plugin, "127.0.0.1"); return !present })
		beta, betaPresent = waitPeerPresent13(ctx, plugin, "127.0.0.2")
		if !gone || !betaPresent {
			return fmt.Errorf("after delete: alpha gone=%v beta present=%v", gone, betaPresent)
		}
		fmt.Fprintf(os.Stderr, "OK: step 3 - peer-alpha gone, peer-beta state=%s\n", beta)
		sid, err = driver.createSession(ctx)
		if err != nil {
			return err
		}
		if err = driver.addPeer(ctx, sid, "peer-alpha", "127.0.0.1", 65001); err != nil {
			return err
		}
		if err = driver.commit(ctx, sid, "step 4"); err != nil {
			return err
		}
		alpha, alphaPresent = waitPeerPresent13(ctx, plugin, "127.0.0.1")
		beta, betaPresent = waitPeerPresent13(ctx, plugin, "127.0.0.2")
		if !alphaPresent || !betaPresent {
			return fmt.Errorf("after re-add: alpha present=%v beta present=%v", alphaPresent, betaPresent)
		}
		fmt.Fprintf(os.Stderr, "OK: step 4 - peer-alpha state=%s, peer-beta state=%s\n", alpha, beta)
		fmt.Fprintln(os.Stderr, "OK: config set/delete/commit lifecycle completed")
		return nil
	})
}
