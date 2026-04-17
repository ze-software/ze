// Design: docs/architecture/testing/ci-format.md — result validation (JSON, logging, HTTP)
// Overview: runner.go — Runner struct and lifecycle
// Related: runner_exec.go — test execution that calls validation

package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/syslog"
)

// validateJSON validates JSON expectations against decoded messages.
// Returns nil if all validations pass or no JSON expectations exist.
// Skips tests with ExaBGP envelope format JSON (contains "exabgp" key).
// Matches by NLRI content, not position (ze may send routes in different order).
func (r *Runner) validateJSON(rec *Record) error {
	// Build cache of decoded received messages
	type decodedMsg struct {
		envelope map[string]any
		actual   map[string]any
		fam      string
		nlris    []string // for content matching
		action   string   // "add" or "del"
		used     bool     // track if already matched
	}
	decoded := make([]*decodedMsg, 0, len(rec.ReceivedRaw))

	for _, rawHex := range rec.ReceivedRaw {
		envelope, err := r.decodeToEnvelope(rawHex)
		if err != nil {
			continue // Skip unparseable messages
		}
		fam := extractFamily(envelope)
		actual, _ := transformEnvelopeToPlugin(envelope)
		nlris := extractNLRIs(actual)
		action := extractAction(actual)
		decoded = append(decoded, &decodedMsg{envelope, actual, fam, nlris, action, false})
	}

	// Find messages with JSON expectations
	for _, msg := range rec.Messages {
		if msg.JSON == "" {
			continue // No JSON expectation
		}

		// Check if JSON is in ExaBGP envelope format (contains "exabgp" key)
		if strings.Contains(msg.JSON, `"exabgp"`) {
			continue // Skip ExaBGP envelope format (not plugin format)
		}

		// Parse expected JSON to extract NLRIs and action for matching
		var expectedMap map[string]any
		if err := json.Unmarshal([]byte(msg.JSON), &expectedMap); err != nil {
			return fmt.Errorf("message %d: invalid expected JSON: %w", msg.Index, err)
		}
		expectedNLRIs := extractNLRIs(expectedMap)
		expectedAction := extractAction(expectedMap)

		if len(expectedNLRIs) == 0 {
			continue // No NLRI to match (e.g., EOR)
		}

		// Find received message with matching NLRI and action (not already used)
		found := false
		for _, dm := range decoded {
			if dm.used {
				continue // Already matched to another expected
			}
			if dm.fam != "" && !isSupportedFamily(dm.fam) {
				continue // Skip unsupported families
			}
			if nlrisMatch(expectedNLRIs, dm.nlris) && dm.action == expectedAction {
				// Compare full JSON
				if err := comparePluginJSON(dm.actual, msg.JSON); err != nil {
					return fmt.Errorf("message %d: %w", msg.Index, err)
				}
				dm.used = true
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("message %d: no received message with NLRI %v action %s", msg.Index, expectedNLRIs, expectedAction)
		}
	}

	return nil
}

// extractNLRIs extracts NLRI identifiers from plugin format JSON for content matching.
// For unicast: extracts prefix strings.
// For FlowSpec: extracts the "string" field from the nlri object (human-readable rule).
func extractNLRIs(m map[string]any) []string {
	var nlris []string
	families := []string{
		"ipv4/unicast", "ipv6/unicast", "ipv4 unicast", "ipv6 unicast",
		"ipv4/flow", "ipv6/flow", "ipv4 flow", "ipv6 flow",
	}
	for _, fam := range families {
		if arr, ok := m[fam].([]any); ok {
			for _, item := range arr {
				if entry, ok := item.(map[string]any); ok {
					nlris = append(nlris, extractNLRIFromEntry(entry)...)
				}
			}
		}
		// Also handle []map[string]any from transformAnnounce/transformFlowspecAnnounce
		if arr, ok := m[fam].([]map[string]any); ok {
			for _, entry := range arr {
				nlris = append(nlris, extractNLRIFromEntry(entry)...)
			}
		}
	}
	return nlris
}

// extractAction extracts the action (add/del) from plugin format JSON.
func extractAction(m map[string]any) string {
	families := []string{
		"ipv4/unicast", "ipv6/unicast", "ipv4 unicast", "ipv6 unicast",
		"ipv4/flow", "ipv6/flow", "ipv4 flow", "ipv6 flow",
	}
	for _, fam := range families {
		if arr, ok := m[fam].([]any); ok {
			for _, item := range arr {
				if entry, ok := item.(map[string]any); ok {
					if action, ok := entry["action"].(string); ok {
						return action
					}
				}
			}
		}
		if arr, ok := m[fam].([]map[string]any); ok {
			for _, entry := range arr {
				if action, ok := entry["action"].(string); ok {
					return action
				}
			}
		}
	}
	return ""
}

// extractNLRIFromEntry extracts NLRI identifiers from an entry map.
// For unicast: entry["nlri"] is []string of prefixes.
// For FlowSpec: entry["nlri"] is map with "string" field containing human-readable rule.
func extractNLRIFromEntry(entry map[string]any) []string {
	var nlris []string
	// Handle []any (from JSON unmarshal) - unicast format
	if nlriArr, ok := entry["nlri"].([]any); ok {
		for _, n := range nlriArr {
			if s, ok := n.(string); ok {
				nlris = append(nlris, s)
			}
		}
	}
	// Handle []string (from transformAnnounce) - unicast format
	if nlriArr, ok := entry["nlri"].([]string); ok {
		nlris = append(nlris, nlriArr...)
	}
	// Handle map[string]any (from transformFlowspecAnnounce/Withdraw) - FlowSpec format
	// Use the "string" field as the NLRI identifier for matching
	if nlriMap, ok := entry["nlri"].(map[string]any); ok {
		if s, ok := nlriMap["string"].(string); ok {
			nlris = append(nlris, s)
		}
	}
	return nlris
}

// nlrisMatch returns true if expected and actual NLRI lists have the same prefixes.
func nlrisMatch(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	// Sort both for comparison
	e := make([]string, len(expected))
	a := make([]string, len(actual))
	copy(e, expected)
	copy(a, actual)
	sort.Strings(e)
	sort.Strings(a)
	for i := range e {
		if e[i] != a[i] {
			return false
		}
	}
	return true
}

// checkObserverSentinel scans the given stderr for the ZE-OBSERVER-FAIL
// sentinel and returns a descriptive error if found, or nil if absent.
// Used by runner_exec.go at the start of the outcome-classification phase
// so an observer's runtime_fail takes precedence over timeout / exit-code
// / peer-output interpretation.
func checkObserverSentinel(stderr string) error {
	idx := strings.Index(stderr, observerFailSentinel)
	if idx < 0 {
		return nil
	}
	return fmt.Errorf("observer reported runtime failure: %s", extractObserverFailLine(stderr, idx))
}

// extractObserverFailLine returns the line in stderr that contains the
// ZE-OBSERVER-FAIL sentinel, trimmed of surrounding whitespace. Used by
// validateLogging to produce a focused error message pointing at the
// failing observer line rather than dumping the entire stderr buffer.
func extractObserverFailLine(stderr string, idx int) string {
	start := idx
	for start > 0 && stderr[start-1] != '\n' {
		start--
	}
	end := idx
	for end < len(stderr) && stderr[end] != '\n' {
		end++
	}
	return strings.TrimSpace(stderr[start:end])
}

// observerFailSentinel is the prefix that Python observer plugins emit on
// stderr via `ze_api.runtime_fail()` to signal a runtime assertion failure.
// The engine relays ERROR-level plugin stderr through its own stderr so the
// sentinel lands in the runner's captured output. validateLogging applies an
// implicit reject check for this sentinel on every test, so an observer's
// runtime_fail always surfaces as a test failure regardless of whether the
// test author added a dedicated reject= directive.
//
// Kept in sync with test/scripts/ze_api.py `_OBSERVER_FAIL_SENTINEL`.
// See plan/known-failures.md section 8 and plan/learned/550 for background.
const observerFailSentinel = "ZE-OBSERVER-FAIL"

// validateLogging validates logging expectations against stderr and syslog output.
// Returns nil if all validations pass or no logging expectations exist.
func (r *Runner) validateLogging(rec *Record, stderr string, syslogSrv *syslog.Server) error {
	// Implicit universal check: the observer-failure sentinel. Applies to every
	// test, even those without explicit expect/reject patterns, so the
	// runtime_fail helper in ze_api.py is not silently ignored when the
	// observer reports an assertion failure.
	if idx := strings.Index(stderr, observerFailSentinel); idx >= 0 {
		return fmt.Errorf("observer reported runtime failure: %s", extractObserverFailLine(stderr, idx))
	}

	// Check expected stderr patterns
	for _, pattern := range rec.ExpectStderr {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid expect=stderr pattern %q: %w", pattern, err)
		}
		if !re.MatchString(stderr) {
			return fmt.Errorf("expect=stderr pattern not found: %s", pattern)
		}
	}

	// Check rejected stderr patterns
	for _, pattern := range rec.RejectStderr {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid reject=stderr pattern %q: %w", pattern, err)
		}
		if re.MatchString(stderr) {
			return fmt.Errorf("reject=stderr pattern found: %s", pattern)
		}
	}

	// Check expected syslog patterns
	if syslogSrv != nil {
		for _, pattern := range rec.ExpectSyslog {
			if !syslogSrv.Match(pattern) {
				return fmt.Errorf("expect=syslog pattern not found: %s", pattern)
			}
		}
	}

	return nil
}

// decodeToEnvelope decodes a hex message using ze bgp decode and returns the envelope.
func (r *Runner) decodeToEnvelope(hexMsg string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.zePath, "bgp", "decode", "--json", "--update", hexMsg) //nolint:gosec // test runner
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ze bgp decode: %w: %s", err, string(output))
	}

	var envelope map[string]any
	if err := json.Unmarshal(output, &envelope); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return envelope, nil
}

// executeHTTPChecks runs HTTP assertions in seq order with retry+backoff.
// Returns nil if all checks pass, or the first error encountered.
func (r *Runner) executeHTTPChecks(ctx context.Context, rec *Record) error {
	checks := make([]HTTPCheck, len(rec.HTTPChecks))
	copy(checks, rec.HTTPChecks)
	sort.Slice(checks, func(i, j int) bool {
		return checks[i].Seq < checks[j].Seq
	})

	client := &http.Client{Timeout: 5 * time.Second}

	ciDir := filepath.Dir(rec.CIFile)
	for _, chk := range checks {
		url := strings.ReplaceAll(chk.URL, "$PORT2", fmt.Sprintf("%d", rec.Port+1))
		url = strings.ReplaceAll(url, "$PORT", fmt.Sprintf("%d", rec.Port))
		// Resolve bodyfile path relative to .ci file directory.
		if chk.BodyFile != "" && !filepath.IsAbs(chk.BodyFile) {
			chk.BodyFile = filepath.Join(ciDir, chk.BodyFile)
		}
		if err := r.executeOneHTTPCheck(ctx, client, chk, url); err != nil {
			return err
		}
	}
	return nil
}

// executeOneHTTPCheck performs a single HTTP request with retry+backoff.
// Retries up to 20 times with 200ms intervals for connection-refused errors
// (server may still be starting). Non-connection errors fail immediately.
func (r *Runner) executeOneHTTPCheck(ctx context.Context, client *http.Client, chk HTTPCheck, url string) error {
	const maxRetries = 20
	const retryInterval = 200 * time.Millisecond

	var lastErr error
	for attempt := range maxRetries {
		if ctx.Err() != nil {
			return fmt.Errorf("http %s %s: context canceled", chk.Method, url)
		}

		req, err := http.NewRequestWithContext(ctx, strings.ToUpper(chk.Method), url, http.NoBody)
		if err != nil {
			return fmt.Errorf("http %s %s: invalid request: %w", chk.Method, url, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// Retry on transient connection errors (server starting up).
			// Covers ECONNREFUSED, ECONNRESET, EOF, and similar.
			if isTransientConnError(err) && attempt < maxRetries-1 {
				select {
				case <-time.After(retryInterval):
					continue
				case <-ctx.Done():
					return fmt.Errorf("http %s %s: %w (after %d retries)", chk.Method, url, lastErr, attempt+1)
				}
			}
			return fmt.Errorf("http %s %s: %w", chk.Method, url, err)
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("http %s %s: reading body: %w", chk.Method, url, readErr)
		}

		// Check status code.
		if resp.StatusCode != chk.Status {
			return fmt.Errorf("http %s %s: expected status %d, got %d (body: %s)",
				chk.Method, url, chk.Status, resp.StatusCode, truncate(string(body), 200))
		}

		// Check body contains (optional).
		if chk.Contains != "" && !strings.Contains(string(body), chk.Contains) {
			return fmt.Errorf("http %s %s: body does not contain %q (body: %s)",
				chk.Method, url, chk.Contains, truncate(string(body), 200))
		}

		// Check body matches file (exact match).
		// BodyFile path already resolved by caller (executeHTTPChecks).
		if chk.BodyFile != "" {
			expected, readFileErr := os.ReadFile(chk.BodyFile) //nolint:gosec // test runner, path from .ci file
			if readFileErr != nil {
				return fmt.Errorf("http %s %s: read bodyfile %q: %w", chk.Method, url, chk.BodyFile, readFileErr)
			}
			if !bytes.Equal(body, expected) {
				return fmt.Errorf("http %s %s: body does not match %s\ngot:\n%s\nexpected:\n%s",
					chk.Method, url, chk.BodyFile, truncate(string(body), 500), truncate(string(expected), 500))
			}
		}

		return nil
	}
	return fmt.Errorf("http %s %s: %w (after %d retries)", chk.Method, url, lastErr, maxRetries)
}

// isTransientConnError checks if an error is a transient connection error
// that should be retried during server startup. Covers ECONNREFUSED (not
// listening yet), ECONNRESET (listener restarting), and EOF (accepted but
// handler not ready). errors.Is unwraps through url.Error/net.OpError chains.
func isTransientConnError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, io.EOF)
}

// executeHTTPWaits polls HTTP endpoints until conditions are met.
// Unlike assertion checks, waits retry on both connection errors AND content
// mismatches, making them suitable for waiting until a server has populated data.
func (r *Runner) executeHTTPWaits(ctx context.Context, rec *Record) error {
	waits := make([]HTTPCheck, len(rec.HTTPWaits))
	copy(waits, rec.HTTPWaits)
	sort.Slice(waits, func(i, j int) bool {
		return waits[i].Seq < waits[j].Seq
	})

	client := &http.Client{Timeout: 5 * time.Second}

	for _, w := range waits {
		url := strings.ReplaceAll(w.URL, "$PORT2", fmt.Sprintf("%d", rec.Port+1))
		url = strings.ReplaceAll(url, "$PORT", fmt.Sprintf("%d", rec.Port))
		if err := r.executeOneHTTPWait(ctx, client, w, url); err != nil {
			return err
		}
	}
	return nil
}

// executeOneHTTPWait polls a single HTTP endpoint until the expected condition
// is met. Retries on connection errors, wrong status codes, and content
// mismatches. Default timeout is 15s, overridden by the check's Timeout field.
func (r *Runner) executeOneHTTPWait(ctx context.Context, client *http.Client, chk HTTPCheck, url string) error {
	const retryInterval = 500 * time.Millisecond

	timeout := 15 * time.Second
	if chk.Timeout != "" {
		if d, err := time.ParseDuration(chk.Timeout); err == nil {
			timeout = d
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	for {
		if waitCtx.Err() != nil {
			if lastErr != nil {
				return fmt.Errorf("http wait %s: %w (timeout %s)", url, lastErr, timeout)
			}
			return fmt.Errorf("http wait %s: timeout %s", url, timeout)
		}

		req, err := http.NewRequestWithContext(waitCtx, strings.ToUpper(chk.Method), url, http.NoBody)
		if err != nil {
			return fmt.Errorf("http wait %s: invalid request: %w", url, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			select {
			case <-time.After(retryInterval):
				continue
			case <-waitCtx.Done():
				return fmt.Errorf("http wait %s: %w (timeout %s)", url, lastErr, timeout)
			}
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("reading body: %w", readErr)
			select {
			case <-time.After(retryInterval):
				continue
			case <-waitCtx.Done():
				return fmt.Errorf("http wait %s: %w (timeout %s)", url, lastErr, timeout)
			}
		}

		// Check status code.
		if resp.StatusCode != chk.Status {
			lastErr = fmt.Errorf("expected status %d, got %d", chk.Status, resp.StatusCode)
			select {
			case <-time.After(retryInterval):
				continue
			case <-waitCtx.Done():
				return fmt.Errorf("http wait %s: %w (timeout %s)", url, lastErr, timeout)
			}
		}

		// Check body contains.
		if chk.Contains != "" && !strings.Contains(string(body), chk.Contains) {
			lastErr = fmt.Errorf("body does not contain %q (body: %s)", chk.Contains, truncate(string(body), 200))
			select {
			case <-time.After(retryInterval):
				continue
			case <-waitCtx.Done():
				return fmt.Errorf("http wait %s: %w (timeout %s)", url, lastErr, timeout)
			}
		}

		// All conditions met.
		return nil
	}
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
