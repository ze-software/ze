// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- Flowtriq cloud API reporter

package flowtriq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type client struct {
	base     string
	apiKey   string
	nodeUUID string
	http     *http.Client
	cb       circuitBreaker
}

// newClient builds the client that posts to the Flowtriq API at base. Its HTTP
// client follows no redirect (refuseFlowtriqRedirect).
func newClient(base, apiKey, nodeUUID string) *client {
	return &client{
		base:     base,
		apiKey:   apiKey,
		nodeUUID: nodeUUID,
		http:     &http.Client{Timeout: 10 * time.Second, CheckRedirect: refuseFlowtriqRedirect},
	}
}

// refuseFlowtriqRedirect answers http.ErrUseLastResponse, so the client hands
// back a 3xx answer unfollowed and post refuses it. Every request carries the
// api-key as a bearer token and the incident data. Go's default client
// re-sends a POST body on a 307 or 308 to any host and any scheme, and keeps
// the Authorization header on a redirect to the same host over http, so a
// misconfigured endpoint, or anyone able to inject a redirect, would receive
// the token or the data. A same-origin redirect is refused too: the operator
// configures the final api-base instead, so a request goes only where the
// configuration names. The config archive upload and the appliance push refuse
// every redirect the same way (internal/component/config/archive/archive.go,
// refuseArchiveRedirect; internal/appliance/cmd_push.go, refusePushRedirect).
func refuseFlowtriqRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// The incident payload field names the FlowTriq API reads. The server ignores a
// field it does not recognize, so a misspelling here is silent: the incident is
// accepted with the value missing.
const (
	fieldPeakPPS         = "peak_pps"
	fieldPeakBPS         = "peak_bps"
	fieldAttackFamily    = "attack_family"
	fieldConfidence      = "confidence"
	fieldTopSourceIPs    = "top_src_ips"
	fieldDurationSeconds = "duration_seconds"
)

func (c *client) openIncident(e *ddosevent.AttackDetected) (string, error) {
	sources := make([]map[string]string, 0, len(e.TopSources))
	for _, src := range e.TopSources {
		sources = append(sources, map[string]string{"ip": src.String()})
	}
	body := map[string]any{
		fieldPeakPPS:      e.PeakRxPps,
		fieldPeakBPS:      e.PeakRxBps,
		fieldAttackFamily: string(e.Family),
		fieldTopSourceIPs: sources,
	}

	var resp struct {
		UUID string `json:"uuid"`
	}
	if err := c.post("/agent/incidents", body, &resp); err != nil {
		return "", err
	}
	return resp.UUID, nil
}

func (c *client) updateIncident(uuid string, pps, bps float64, family ddosevent.AttackFamily, confidence int) error {
	var tb textbuf.Buffer
	path := tb.Str("/agent/incidents/").Str(uuid).String()
	body := map[string]any{
		fieldPeakPPS:      pps,
		fieldPeakBPS:      bps,
		fieldAttackFamily: string(family),
		fieldConfidence:   confidence,
	}
	return c.post(path, body, nil)
}

func (c *client) resolveIncident(uuid string, duration, peakPPS, peakBPS float64, confidence int) error {
	var tb textbuf.Buffer
	path := tb.Str("/agent/incidents/").Str(uuid).Str("/resolve").String()
	body := map[string]any{
		fieldDurationSeconds: duration,
		fieldPeakPPS:         peakPPS,
		fieldPeakBPS:         peakBPS,
		fieldConfidence:      confidence,
	}
	return c.post(path, body, nil)
}

func (c *client) heartbeat(ready bool, avg, p99 float64) error {
	body := map[string]any{
		"version":          "ze",
		"baseline_ready":   ready,
		"baseline_avg_pps": avg,
		"baseline_p99_pps": p99,
	}
	return c.post("/agent/heartbeat", body, nil)
}

func (c *client) post(path string, payload, out any) error {
	if c.cb.tripped() {
		return fmt.Errorf("flowtriq: circuit breaker open")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("flowtriq: marshal: %w", err)
	}

	var tb textbuf.Buffer
	endpoint := tb.Str(c.base).Str(path).String()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("flowtriq: request: %w", err)
	}
	tb.Reset()
	req.Header.Set("Authorization", tb.Str("Bearer ").Str(c.apiKey).String())
	req.Header.Set("X-Node-UUID", c.nodeUUID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ze-ddos-flowtriq/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("flowtriq: %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A redirect is a failure the breaker counts like any other: the request
	// reached no Flowtriq API, and an endpoint that keeps redirecting trips it.
	if redirect, ok := redirectOf(resp); ok {
		c.cb.recordFailure()
		return redirect.refusal(resp.Request.URL.Redacted(), path)
	}

	// Only a 2xx delivered the request: a 1xx, an unfollowed 3xx with no
	// Location, a 4xx and a 5xx are failures (docs/integrations/flowtriq-api.md).
	if !statusSuccess(resp.StatusCode) {
		c.cb.recordFailure()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("flowtriq: %s: status %d: %s", path, resp.StatusCode, respBody)
	}

	c.cb.recordSuccess()

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("flowtriq: %s: decode response: %w", path, err)
		}
	}
	return nil
}

// statusSuccess answers whether status is a 2xx, the only answer that
// delivered a request.
func statusSuccess(status int) bool {
	if status < 200 {
		return false
	}
	return status < 300
}

// apiRedirect is a 3xx answer from the Flowtriq API that names another
// location.
type apiRedirect struct {
	status int
	target *url.URL // the Location the server answered with, resolved against the request
}

// redirectOf answers the redirect resp carries. A 3xx with no Location, or
// with one that does not parse, names nowhere to configure, so it is not a
// redirect and the caller's status check refuses it as a failure.
func redirectOf(resp *http.Response) (apiRedirect, bool) {
	if resp.StatusCode < 300 {
		return apiRedirect{}, false
	}
	if resp.StatusCode > 399 {
		return apiRedirect{}, false
	}
	target, err := resp.Location()
	if err != nil {
		return apiRedirect{}, false
	}
	return apiRedirect{status: resp.StatusCode, target: target}, true
}

// refusal is the operator error for a post to endpoint (the redacted request
// URL, api-base followed by path) that the server answered with r.
func (r apiRedirect) refusal(endpoint, path string) error {
	return opererr.New(
		"the Flowtriq API at "+endpoint+" answered HTTP "+strconv.Itoa(r.status)+
			", a redirect to "+r.target.Redacted()+", and Ze did not follow it",
		"each request carries the api-key as a bearer token, so Ze sends it only to the api-base you configured, "+
			"never to a location a server names",
		"set ddos flowtriq api-base "+r.apiBase(path))
}

// apiBase answers the api-base the redirect points at: its https Location with
// the request path removed, which is what Ze appends to api-base. A Location
// that is not https names no api-base verify accepts, and one that does not
// end with the path names no api-base at all, so for either the answer is the
// default api-base, the Flowtriq API itself.
func (r apiRedirect) apiBase(path string) string {
	if r.target.Scheme != "https" {
		return defaultAPIBase
	}
	if !strings.HasSuffix(r.target.Path, path) {
		return defaultAPIBase
	}
	base := *r.target
	base.Path = strings.TrimSuffix(r.target.Path, path)
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	return base.Redacted()
}

type circuitBreaker struct {
	mu           sync.Mutex
	failures     int
	tripAt       int
	trippedUntil time.Time
	recoveryWait time.Duration
}

func (cb *circuitBreaker) init() {
	if cb.tripAt == 0 {
		cb.tripAt = 5
		cb.recoveryWait = 60 * time.Second
	}
}

func (cb *circuitBreaker) tripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.init()
	if cb.failures >= cb.tripAt {
		if time.Now().Before(cb.trippedUntil) {
			return true
		}
		cb.failures = 0
	}
	return false
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.init()
	cb.failures++
	if cb.failures >= cb.tripAt {
		cb.trippedUntil = time.Now().Add(cb.recoveryWait)
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
}
