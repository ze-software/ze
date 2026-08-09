// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- Flowtriq cloud API reporter

package flowtriq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type client struct {
	base     string
	apiKey   string
	nodeUUID string
	http     *http.Client
	cb       circuitBreaker
}

func newClient(base, apiKey, nodeUUID string) *client {
	return &client{
		base:     base,
		apiKey:   apiKey,
		nodeUUID: nodeUUID,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *client) openIncident(e *ddosevent.AttackDetected) (string, error) {
	sources := make([]map[string]string, 0, len(e.TopSources))
	for _, src := range e.TopSources {
		sources = append(sources, map[string]string{"ip": src.String()})
	}
	body := map[string]any{
		"peak_pps":      e.PeakRxPps,
		"peak_bps":      e.PeakRxBps,
		"attack_family": string(e.Family),
		"top_src_ips":   sources,
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
		"peak_pps":      pps,
		"peak_bps":      bps,
		"attack_family": string(family),
		"confidence":    confidence,
	}
	return c.post(path, body, nil)
}

func (c *client) resolveIncident(uuid string, duration, peakPPS, peakBPS float64, confidence int) error {
	var tb textbuf.Buffer
	path := tb.Str("/agent/incidents/").Str(uuid).Str("/resolve").String()
	body := map[string]any{
		"duration_seconds": duration,
		"peak_pps":         peakPPS,
		"peak_bps":         peakBPS,
		"confidence":       confidence,
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
	url := tb.Str(c.base).Str(path).String()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(data))
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

	if resp.StatusCode >= 400 {
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
