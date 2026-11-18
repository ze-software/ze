// Design: docs/architecture/api/architecture.md -- a daemon owns metadata and parameter construction.
// Related: plugin_fixture_typed_selector.go -- the separate SDK process being observed.
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

	"github.com/ze-software/ze/internal/test/externalyang"
)

const typedSelectorPollDelay = 100 * time.Millisecond
const typedSelectorResponseMax = 64 * 1024

func init() {
	Register("plugin/typed-selector-observer", typedSelectorObserver)
}

type typedSelectorClient struct {
	endpoint string
	client   http.Client
}

type typedSelectorEnvelope struct {
	Status string          `json:"status"`
	Error  string          `json:"error"`
	Data   json.RawMessage `json:"data"`
}

type typedSelectorObserved struct {
	Record   string               `json:"record"`
	ID       uint16               `json:"id"`
	Listener string               `json:"listener"`
	Count    uint8                `json:"count"`
	Enabled  *bool                `json:"enabled"`
	InlineID bool                 `json:"inline-id"`
	Counts   *typedSelectorCounts `json:"counts"`
}

type typedSelectorMetadata struct {
	Params []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Required bool   `json:"required"`
		Flag     bool   `json:"flag"`
	} `json:"params"`
}

// typedSelectorObserver proves boot-published YANG metadata, authenticated HTTP,
// engine construction and real forked dispatch in one daemon. Its bounded
// requests share the runner budget; no fixed sleep decides readiness or success.
func typedSelectorObserver(parent context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("typed-selector-observer requires the runner's REST port")
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return err
	}
	if port == 0 {
		return errors.New("typed selector REST port must be nonzero")
	}
	ctx, cancel := context.WithTimeout(parent, WaitBudget(80, time.Minute))
	defer cancel()
	client := &typedSelectorClient{
		endpoint: "http://127.0.0.1:" + strconv.FormatUint(port, 10),
		client:   http.Client{Timeout: WaitBudget(5, 3*time.Second)},
	}
	defer client.client.CloseIdleConnections()
	ready, readyCancel := context.WithTimeout(ctx, WaitBudget(15, 5*time.Second))
	var metadata typedSelectorMetadata
	metadataErr := errors.New("REST command metadata has not answered")
	found := Poll(ready, WaitAttempts(15, typedSelectorPollDelay, 40), typedSelectorPollDelay, func() bool {
		metadataErr = client.request(ready, http.MethodGet, "/commands/request/typed-selector/id", nil, &metadata)
		return metadataErr == nil
	})
	readyCancel()
	if !found {
		return fmt.Errorf("typed selector command metadata did not become ready: %w", metadataErr)
	}
	if err := typedSelectorCheckMetadata(metadata); err != nil {
		return err
	}
	counts, err := client.state(ctx)
	if err != nil {
		return err
	}
	if counts != (typedSelectorCounts{}) {
		return fmt.Errorf("fixture already received requests: %+v", counts)
	}
	refused := 0
	for _, step := range []struct {
		name     string
		command  string
		params   map[string]any
		listener string
		good     bool
		force    bool
	}{
		{name: "raw-useful", command: externalyang.TypedSelectorCommand + " 7 listener edge count 3 enabled false", listener: "edge", good: true},
		{name: "raw-forced-useful", command: externalyang.TypedSelectorCommand + " 7 listener edge count 3 enabled false force", listener: "edge", good: true, force: true},
		{name: "structured-listener", command: externalyang.TypedSelectorCommand, params: typedSelectorParams(7, "listener", false), listener: "listener", good: true},
		{name: "structured-tunnel-keyword", command: externalyang.TypedSelectorCommand, params: typedSelectorParams(7, "tunnel-id", false), listener: "tunnel-id", good: true},
		{name: "structured-guard-keyword", command: externalyang.TypedSelectorCommand, params: typedSelectorParams(7, "expected-id", false), listener: "expected-id", good: true},
		{name: "mixed-false", command: externalyang.TypedSelectorCommand + " 7", params: typedSelectorParams(nil, "listener", false), listener: "listener", good: true},
		{name: "mixed-true", command: externalyang.TypedSelectorCommand + " 7", params: typedSelectorParams(nil, "listener", true), listener: "listener", good: true, force: true},
		{name: "malformed-flag", command: externalyang.TypedSelectorCommand + " 7", params: typedSelectorParams(nil, "listener", "maybe")},
		{name: "zero-id", command: externalyang.TypedSelectorCommand, params: typedSelectorParams(0, "listener", false)},
		{name: "wide-id", command: externalyang.TypedSelectorCommand, params: typedSelectorParams(65536, "listener", false)},
	} {
		var result typedSelectorEnvelope
		payload := struct {
			Command string         `json:"command"`
			Params  map[string]any `json:"params,omitempty"`
		}{Command: step.command, Params: step.params}
		if err := client.request(ctx, http.MethodPost, "/execute", payload, &result); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		if !step.good {
			if result.Status != statusError {
				return fmt.Errorf("%s returned %s instead of error: %s", step.name, result.Status, result.Error)
			}
			if result.Error == "" {
				return fmt.Errorf("%s refused without a cause", step.name)
			}
			if step.name == "malformed-flag" {
				for _, part := range []string{"force", "No command ran", "ze help command"} {
					if !strings.Contains(result.Error, part) {
						return fmt.Errorf("malformed flag refusal omits %q: %s", part, result.Error)
					}
				}
			}
			after, err := client.state(ctx)
			if err != nil {
				return err
			}
			if after != counts {
				return fmt.Errorf("%s reached the external provider: before=%+v after=%+v", step.name, counts, after)
			}
			refused++
			continue
		}
		if result.Status != statusDone {
			return fmt.Errorf("%s returned %s: %s", step.name, result.Status, result.Error)
		}
		counts.Received++
		counts.Completed++
		if step.force {
			counts.Forced++
		}
		var observed typedSelectorObserved
		if err := json.Unmarshal(result.Data, &observed); err != nil {
			return err
		}
		if err := typedSelectorCheckReply(observed, step.listener, counts); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	_, err = fmt.Fprintf(os.Stdout, "typed-selector: received=%d completed=%d forced=%d refused=%d record=%s\n",
		counts.Received, counts.Completed, counts.Forced, refused, typedSelectorRecord)
	return err
}

func typedSelectorParams(id any, listener string, force any) map[string]any {
	values := map[string]any{typedSelectorLeafListener: listener, fieldCount: 3, typedSelectorLeafEnabled: false, typedSelectorLeafForce: force}
	if id != nil {
		values[typedSelectorLeafID] = id
	}
	return values
}

func typedSelectorCheckMetadata(metadata typedSelectorMetadata) error {
	if len(metadata.Params) != 5 {
		return fmt.Errorf("typed selector metadata has %d parameters, want five", len(metadata.Params))
	}
	wanted := map[string]string{typedSelectorLeafID: "original-number", typedSelectorLeafListener: "string", fieldCount: "uint8", typedSelectorLeafEnabled: "boolean", typedSelectorLeafForce: "empty"}
	for _, parameter := range metadata.Params {
		kind, found := wanted[parameter.Name]
		if !found {
			return fmt.Errorf("unexpected or repeated parameter %q", parameter.Name)
		}
		delete(wanted, parameter.Name)
		if parameter.Type != kind {
			return fmt.Errorf("%s metadata type=%q, want %q", parameter.Name, parameter.Type, kind)
		}
		if parameter.Flag != (parameter.Name == typedSelectorLeafForce) {
			return fmt.Errorf("%s metadata misstates flag presence", parameter.Name)
		}
		if parameter.Required != (parameter.Name != typedSelectorLeafForce) {
			return fmt.Errorf("%s metadata misstates the mandatory leaf", parameter.Name)
		}
	}
	return nil
}

func typedSelectorCheckReply(observed typedSelectorObserved, listener string, counts typedSelectorCounts) error {
	if observed.Record != typedSelectorRecord {
		return fmt.Errorf("reply selected record %q", observed.Record)
	}
	if observed.ID != typedSelectorID {
		return fmt.Errorf("reply selected ID %d", observed.ID)
	}
	if observed.Listener != listener {
		return fmt.Errorf("reply listener=%q, want %q", observed.Listener, listener)
	}
	if observed.Count != 3 {
		return fmt.Errorf("ordinary count=%d, want three", observed.Count)
	}
	if observed.Enabled == nil {
		return errors.New("ordinary boolean value was omitted")
	}
	if *observed.Enabled {
		return errors.New("ordinary false boolean became true")
	}
	if !observed.InlineID {
		return errors.New("structured ID was not written bare after the path keyword")
	}
	if observed.Counts == nil {
		return errors.New("measured counters are absent")
	}
	if *observed.Counts != counts {
		return fmt.Errorf("action counters=%+v, want %+v", *observed.Counts, counts)
	}
	return nil
}

func (client *typedSelectorClient) state(ctx context.Context) (typedSelectorCounts, error) {
	var result typedSelectorEnvelope
	if err := client.request(ctx, http.MethodPost, "/execute", map[string]string{"command": externalyang.TypedSelectorState}, &result); err != nil {
		return typedSelectorCounts{}, err
	}
	if result.Status != statusDone {
		return typedSelectorCounts{}, fmt.Errorf("state returned %s: %s", result.Status, result.Error)
	}
	var reply struct {
		Record string               `json:"record"`
		Counts *typedSelectorCounts `json:"counts"`
	}
	if err := json.Unmarshal(result.Data, &reply); err != nil {
		return typedSelectorCounts{}, err
	}
	if reply.Record != typedSelectorRecord {
		return typedSelectorCounts{}, fmt.Errorf("state selected %q", reply.Record)
	}
	if reply.Counts == nil {
		return typedSelectorCounts{}, errors.New("state omitted measured counters")
	}
	return *reply.Counts, nil
}

// request bounds every response independently of what the server sends.
func (client *typedSelectorClient) request(ctx context.Context, method, path string, payload, result any) error {
	var body io.Reader = http.NoBody
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.endpoint+"/api/v1"+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer admin:testpass")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // ReadAll owns the useful read result.
	data, err := io.ReadAll(io.LimitReader(response.Body, typedSelectorResponseMax+1))
	if err != nil {
		return err
	}
	if len(data) > typedSelectorResponseMax {
		return errors.New("typed selector HTTP response exceeds 64 KiB")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, data)
	}
	if err := json.Unmarshal(data, result); err != nil {
		return err
	}
	return nil
}
