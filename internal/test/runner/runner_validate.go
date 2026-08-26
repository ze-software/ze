// Design: docs/architecture/testing/ci-format.md — result validation (JSON, logging, HTTP)
// Overview: runner.go — Runner struct and lifecycle
// Related: runner_exec.go — test execution that calls validation

package runner

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/syslog"
)

// hasJSONExpectations reports whether the record declares any expect=json line.
// The caller uses it to decide whether a "json-match" step belongs in the trace:
// validateJSON returns nil for a record with no JSON expectation, and recording
// a passing assertion for an assertion nobody wrote is the kind of false signal
// this suite exists to remove.
func hasJSONExpectations(rec *Record) bool {
	for _, msg := range rec.Messages {
		if msg.JSON != "" {
			return true
		}
	}
	return false
}

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
			// No NLRI could be extracted to match this expectation by content.
			// That is correct for genuinely content-free messages (EOR, keepalive)
			// and harmless when a wire-level expect=bgp (msg.RawHex) backs the same
			// message: the peer's exact byte comparison is authoritative and the
			// JSON check is supplementary. But when JSON is the ONLY assertion for
			// this message (no RawHex) and it still carries route entries the
			// matcher cannot extract (extractNLRIs only understands unicast/flow
			// families), the comparison would be skipped and the message validated
			// by nothing. Fail loudly so a json-only expectation in an unsupported
			// family cannot pass vacuously.
			if msg.RawHex == "" && jsonHasRouteContent(expectedMap) {
				return fmt.Errorf("message %d: json-only expectation carries NLRI content the matcher cannot extract (unsupported family?); add a wire-level expect=bgp:hex= so the message is actually validated", msg.Index)
			}
			continue // genuinely no NLRI (EOR/keepalive), or wire-hex-backed
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

// jsonHasRouteContent reports whether a plugin-format JSON map carries routing
// NLRI entries: an array whose entries have an "nlri" or "action" field. It is
// family-agnostic (does not depend on the key name), so it returns true even for
// families extractNLRIs does not understand (vpn, evpn, bgp-ls, mup, ...). Used
// to tell a genuinely content-free message (EOR, keepalive, attribute-only) apart
// from one whose family is simply not handled by the NLRI content matcher.
func jsonHasRouteContent(m map[string]any) bool {
	for _, v := range m {
		entries, ok := v.([]any)
		if !ok {
			continue
		}
		for _, e := range entries {
			em, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if _, hasNLRI := em["nlri"]; hasNLRI {
				return true
			}
			if _, hasAction := em["action"]; hasAction {
				return true
			}
		}
	}
	return false
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

// observerSentinelInSyslog scans the captured syslog messages for the
// ZE-OBSERVER-FAIL sentinel and returns a descriptive error if found. This is
// the syslog counterpart to checkObserverSentinel: the runner starts ze with
// ze.log.backend=syslog whenever a syslog server is active (runner_exec.go), so
// the relayed sentinel lands in syslog rather than the client's stderr. Without
// this, an observer's runtime_fail would be invisible to the stderr-only check.
func observerSentinelInSyslog(syslogSrv *syslog.Server) error {
	if syslogSrv == nil {
		return nil
	}
	for _, m := range syslogSrv.Messages() {
		if idx := strings.Index(m, observerFailSentinel); idx >= 0 {
			return fmt.Errorf("observer reported runtime failure (syslog): %s", extractObserverFailLine(m, idx))
		}
	}
	return nil
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
//
// The sentinel exists because an observer's own exit code never reaches the
// runner: the observer ends its failure branch with `daemon shutdown`, ze exits
// 0, and the runner used to report PASS on a test whose observer had detected a
// real failure. See plan/known-failures/README.md for background.
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
	if serr := observerSentinelInSyslog(syslogSrv); serr != nil {
		return serr
	}

	// Check expected stderr patterns
	for _, pattern := range rec.ExpectStderr {
		if pattern == "" {
			return errors.New("expect=stderr pattern is empty (an empty regex matches everything); use a specific pattern")
		}
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
		if pattern == "" {
			return errors.New("reject=stderr pattern is empty (an empty regex matches everything); use a specific pattern")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid reject=stderr pattern %q: %w", pattern, err)
		}
		if re.MatchString(stderr) {
			return fmt.Errorf("reject=stderr pattern found: %s", pattern)
		}
	}

	// Syslog assertions require the capture server. runner_exec starts it
	// whenever ExpectSyslog OR RejectSyslog is present; a nil server here with
	// syslog assertions declared means the harness failed to wire it up, so fail
	// loudly rather than passing the assertion vacuously.
	if len(rec.ExpectSyslog) > 0 || len(rec.RejectSyslog) > 0 {
		if syslogSrv == nil {
			return errors.New("syslog assertions declared but no syslog server was started")
		}
		// Expected syslog patterns must appear.
		for _, pattern := range rec.ExpectSyslog {
			if pattern == "" {
				return errors.New("expect=syslog pattern is empty (an empty regex matches everything); use a specific pattern")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid expect=syslog pattern %q: %w", pattern, err)
			}
			if !syslogSrv.Match(pattern) {
				return fmt.Errorf("expect=syslog pattern not found: %s", pattern)
			}
		}
		// Rejected syslog patterns must NOT appear. Compile explicitly first: a
		// bad regex must error rather than fail open. Server.Match returns false
		// on an uncompilable pattern, which would otherwise silently satisfy the
		// reject and let a forbidden log line through unnoticed.
		for _, pattern := range rec.RejectSyslog {
			if pattern == "" {
				return errors.New("reject=syslog pattern is empty (an empty regex matches everything); use a specific pattern")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid reject=syslog pattern %q: %w", pattern, err)
			}
			if syslogSrv.Match(pattern) {
				return fmt.Errorf("reject=syslog pattern found: %s", pattern)
			}
		}
	}

	return nil
}

func (r *Runner) validateFileChecks(rec *Record) error {
	if len(rec.FileChecks) == 0 {
		return nil
	}
	baseDir := filepath.Dir(rec.CIFile)
	if rec.TmpfsTempDir != "" {
		baseDir = rec.TmpfsTempDir
	}
	for _, check := range rec.FileChecks {
		if err := validateOneFileCheck(baseDir, check); err != nil {
			return err
		}
	}
	return nil
}

func validateOneFileCheck(baseDir string, check fileCheck) error {
	if check.Path != "" {
		return validateOnePathCheck(baseDir, check)
	}
	return validateOneGlobCheck(baseDir, check)
}

func validateOnePathCheck(baseDir string, check fileCheck) error {
	path, err := resolveCheckPath(baseDir, check.Path)
	if err != nil {
		return err
	}
	data, readErr := os.ReadFile(path) //nolint:gosec // path is constrained relative to test temp dir
	if check.Absent {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("expect=file:path=%s: stat failed: %w", check.Path, readErr)
		}
		return fmt.Errorf("expect=file:path=%s: expected absent", check.Path)
	}
	if readErr != nil {
		return fmt.Errorf("expect=file:path=%s: read failed: %w", check.Path, readErr)
	}
	return validateFileContent(check.Path, string(data), check)
}

func validateOneGlobCheck(baseDir string, check fileCheck) error {
	pattern, err := resolveCheckPath(baseDir, check.Glob)
	if err != nil {
		return err
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("expect=file:glob=%s: invalid glob: %w", check.Glob, err)
	}
	sort.Strings(matches)
	if check.Count != nil && len(matches) != *check.Count {
		return fmt.Errorf("expect=file:glob=%s: count=%d, want %d", check.Glob, len(matches), *check.Count)
	}
	if check.Absent && len(matches) != 0 {
		return fmt.Errorf("expect=file:glob=%s: expected absent, matched %d", check.Glob, len(matches))
	}
	if check.Exists && len(matches) == 0 {
		return fmt.Errorf("expect=file:glob=%s: expected at least one match", check.Glob)
	}
	if check.Contains != "" {
		for _, match := range matches {
			data, readErr := os.ReadFile(match) //nolint:gosec // path comes from constrained glob under test temp dir
			if readErr != nil {
				return fmt.Errorf("expect=file:glob=%s: read %s: %w", check.Glob, match, readErr)
			}
			if strings.Contains(string(data), check.Contains) {
				return nil
			}
		}
		return fmt.Errorf("expect=file:glob=%s: no matched file contains %q", check.Glob, check.Contains)
	}
	if check.NotContains != "" {
		for _, match := range matches {
			data, readErr := os.ReadFile(match) //nolint:gosec // path comes from constrained glob under test temp dir
			if readErr != nil {
				return fmt.Errorf("expect=file:glob=%s: read %s: %w", check.Glob, match, readErr)
			}
			if strings.Contains(string(data), check.NotContains) {
				return fmt.Errorf("expect=file:glob=%s: %s contains %q", check.Glob, filepath.Base(match), check.NotContains)
			}
		}
	}
	return nil
}

func validateFileContent(label, content string, check fileCheck) error {
	if check.Contains != "" && !strings.Contains(content, check.Contains) {
		return fmt.Errorf("expect=file:path=%s: missing %q", label, check.Contains)
	}
	if check.NotContains != "" && strings.Contains(content, check.NotContains) {
		return fmt.Errorf("expect=file:path=%s: found forbidden %q", label, check.NotContains)
	}
	return nil
}

func resolveCheckPath(baseDir, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("expect=file path must be relative: %s", rel)
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("expect=file path escapes test directory: %s", rel)
	}
	return filepath.Join(baseDir, clean), nil
}

// decodeToEnvelope decodes a hex message using ze bgp decode and returns the envelope.
func (r *Runner) decodeToEnvelope(hexMsg string) (map[string]any, error) {
	// Scale the per-decode fork budget by the parallel headroom (identity for
	// serial runs): under oversubscription a slow `ze bgp decode` fork must not
	// time out and drop a received message, which would surface as a spurious
	// JSON expectation mismatch rather than the real (absent) defect.
	ctx, cancel := context.WithTimeout(context.Background(), r.withParallelHeadroom(5*time.Second))
	defer cancel()

	cmd := exec.CommandContext(ctx, r.zePath, "bgp", "decode", "--json", "--update", hexMsg) //nolint:gosec // test runner
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("ze bgp decode: %w: %s", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("ze bgp decode: %w", err)
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
	checks := make([]httpCheck, len(rec.HTTPChecks))
	copy(checks, rec.HTTPChecks)
	sort.Slice(checks, func(i, j int) bool {
		return checks[i].Seq < checks[j].Seq
	})

	ciDir := filepath.Dir(rec.CIFile)
	// Index rather than range-copy: httpCheck is large enough that a per-iteration
	// copy trips gocritic's rangeValCopy. The path rewrites below therefore land in
	// `checks`, which is already the defensive copy made above, so rec.HTTPChecks
	// still keeps the authored relative paths.
	for i := range checks {
		chk := &checks[i]
		client := r.httpClientForCheck(chk)
		url := strings.ReplaceAll(chk.URL, "$PORT2", strconv.Itoa(rec.Port+1))
		url = strings.ReplaceAll(url, "$PORT", strconv.Itoa(rec.Port))
		// Resolve bodyfile and sendfile paths relative to .ci file directory.
		if chk.BodyFile != "" && !filepath.IsAbs(chk.BodyFile) {
			chk.BodyFile = filepath.Join(ciDir, chk.BodyFile)
		}
		if chk.SendFile != "" && !filepath.IsAbs(chk.SendFile) {
			// Resolve against tmpfs temp dir first (tmpfs= files land there),
			// then fall back to the .ci file directory.
			if rec.TmpfsTempDir != "" {
				candidate := filepath.Join(rec.TmpfsTempDir, chk.SendFile)
				if _, statErr := os.Stat(candidate); statErr == nil {
					chk.SendFile = candidate
				} else {
					chk.SendFile = filepath.Join(ciDir, chk.SendFile)
				}
			} else {
				chk.SendFile = filepath.Join(ciDir, chk.SendFile)
			}
		}
		if err := r.executeOneHTTPCheck(ctx, client, chk, url); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) httpClientForCheck(chk *httpCheck) *http.Client {
	// Scale the per-request budget by the parallel headroom (identity for serial
	// runs): a server that is listening but slow to respond under oversubscription
	// must not trip a net timeout, which isTransientConnError treats as
	// non-retryable so the check would fail immediately.
	client := &http.Client{Timeout: r.withParallelHeadroom(5 * time.Second)}
	if !chk.InsecureTLS {
		return client
	}
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test runner for self-signed local web servers
	}
	return client
}

// applyCheckHeaders puts the check's header= keys on the request. It runs AFTER
// any default Content-Type, so an explicit header=Content-Type: ... wins. The
// first occurrence of a field name replaces whatever is already set. Any repeat
// of that same name is appended, which matches HTTP's multi-value field
// semantics.
func applyCheckHeaders(req *http.Request, headers []httpHeader) {
	if len(headers) == 0 {
		return
	}
	seen := make(map[string]bool, len(headers))
	for _, h := range headers {
		canonical := http.CanonicalHeaderKey(h.Name)
		// net/http writes req.Host on the wire and IGNORES a "Host" entry in
		// req.Header, so routing it through Set would silently do nothing.
		if canonical == "Host" {
			req.Host = h.Value
			continue
		}
		if seen[canonical] {
			req.Header.Add(h.Name, h.Value)
			continue
		}
		seen[canonical] = true
		req.Header.Set(h.Name, h.Value)
	}
}

// executeOneHTTPCheck performs a single HTTP request with retry+backoff.
// Retries up to 20 times with 200ms intervals for connection-refused errors
// (the server can still be starting). Non-connection errors fail immediately.
func (r *Runner) executeOneHTTPCheck(ctx context.Context, client *http.Client, chk *httpCheck, url string) error {
	// The retry budget is a server-startup readiness window (connection-refused ->
	// wait -> retry). 20 x 200ms = 4s is fine unloaded, but a parallel run
	// oversubscribes every core and a web/LG server can take longer than 4s to
	// bind. Scale the attempt count by the same parallel headroom the outer test
	// budget gets; identity for serial runs so a genuinely dead server still fails fast.
	maxRetries := 20 * r.parallelFactor()
	const retryInterval = 200 * time.Millisecond

	var lastErr error
	for attempt := range maxRetries {
		if ctx.Err() != nil {
			return fmt.Errorf("http %s %s: context canceled", chk.Method, url)
		}

		var reqBody io.Reader = http.NoBody
		if chk.SendFile != "" {
			sendData, readErr := os.ReadFile(chk.SendFile) //nolint:gosec // test runner, path from .ci file
			if readErr != nil {
				return fmt.Errorf("http %s %s: read sendfile %q: %w", chk.Method, url, chk.SendFile, readErr)
			}
			reqBody = bytes.NewReader(sendData)
		}
		req, err := http.NewRequestWithContext(ctx, strings.ToUpper(chk.Method), url, reqBody)
		if err != nil {
			return fmt.Errorf("http %s %s: invalid request: %w", chk.Method, url, err)
		}
		if chk.SendFile != "" {
			contentType := chk.ContentType
			if contentType == "" {
				contentType = "application/json"
			}
			req.Header.Set("Content-Type", contentType)
		}
		applyCheckHeaders(req, chk.Headers)

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
	waits := make([]httpCheck, len(rec.HTTPWaits))
	copy(waits, rec.HTTPWaits)
	sort.Slice(waits, func(i, j int) bool {
		return waits[i].Seq < waits[j].Seq
	})

	// Indexed for the same reason as executeHTTPChecks: httpCheck is too large to
	// range-copy per iteration.
	for i := range waits {
		w := &waits[i]
		client := r.httpClientForCheck(w)
		url := strings.ReplaceAll(w.URL, "$PORT2", strconv.Itoa(rec.Port+1))
		url = strings.ReplaceAll(url, "$PORT", strconv.Itoa(rec.Port))
		if err := r.executeOneHTTPWait(ctx, client, w, url); err != nil {
			return err
		}
	}
	return nil
}

// executeOneHTTPWait polls a single HTTP endpoint until the expected condition
// is met. Retries on connection errors, wrong status codes, and content
// mismatches. Default timeout is 15s, overridden by the check's Timeout field.
func (r *Runner) executeOneHTTPWait(ctx context.Context, client *http.Client, chk *httpCheck, url string) error {
	const retryInterval = 500 * time.Millisecond

	timeout := 15 * time.Second
	if chk.Timeout != "" {
		if d, err := time.ParseDuration(chk.Timeout); err == nil {
			timeout = d
		}
	}

	// The authored wait (default 15s, or the check's Timeout=) is measured against
	// an unloaded server startup. Under a parallel run the server shares every core
	// and can take multiples of that to bind, while the outer test budget -- already
	// widened by the same headroom -- still has room. Scale the wait to match so the
	// inner readiness gate stops expiring first (identity for serial runs).
	waitCtx, cancel := context.WithTimeout(ctx, r.withParallelHeadroom(timeout))
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
		applyCheckHeaders(req, chk.Headers)

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
	var tb textbuf.Buffer
	return tb.Str(s[:maxLen]).Str("...").String()
}
