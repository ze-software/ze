// Design: docs/architecture/config/system-update.md -- periodic version check against remote manifest

package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

const (
	reportFieldRemoteVersion  = "remote"
	reportFieldRunningVersion = "running"
)

const (
	updateSource       = "system"
	updateCode         = "update-available"
	updateSubject      = "firmware"
	updateFetchTimeout = 30 * time.Second
	updateMaxBody      = 64 << 10 // 64 KiB
)

// UpdateStatus holds the last update check result.
type UpdateStatus struct {
	LastCheck       time.Time
	RunningVersion  string
	RemoteVersion   string
	UpdateAvailable bool
	LastError       string
}

// UpdateChecker periodically fetches a remote version manifest and
// reports when a newer version is available via the report bus.
type UpdateChecker struct {
	url      string
	interval time.Duration
	client   *http.Client
	running  string // override for testing; empty means use version.Release()

	mu     sync.RWMutex
	status UpdateStatus

	cancel context.CancelFunc
	done   chan struct{}
}

// newUpdateChecker creates a checker. Call Start to begin periodic checks.
// URL must use HTTPS; HTTP is accepted only for localhost (testing).
func newUpdateChecker(url string, intervalSecs uint32) *UpdateChecker {
	return &UpdateChecker{
		url:      url,
		interval: time.Duration(intervalSecs) * time.Second,
		client:   newUpdateClient(),
		done:     make(chan struct{}),
	}
}

// newUpdateClient builds the HTTP client every update request goes through:
// the version check's manifest, and the self-updater's manifest and binary
// (selfupdate.go). It follows a redirect only within the scheme and host of
// the first request (fetch.SameOrigin). The default client follows one to any
// host and from https to http, so a server, or anyone able to answer for it,
// could send the request with the userinfo of the configured url to a host
// nobody configured, or take the TLS away from the manifest and the binary.
func newUpdateClient() *http.Client {
	return &http.Client{Timeout: updateFetchTimeout, CheckRedirect: fetch.SameOrigin}
}

// manifestFetchError is the error a failed manifest request answers. A
// redirect fetch.SameOrigin refused becomes an operator error naming the
// redirect target and the fix; any other error keeps its text, with the URL
// net/http attaches to it redacted.
func manifestFetchError(err error, manifestURL string) error {
	var refused *fetch.RedirectRefused
	if !errors.As(err, &refused) {
		return redact.URLError(err)
	}
	return opererr.New(
		"the update manifest at "+redact.URL(manifestURL)+" redirects to "+refused.Target()+
			", and Ze did not follow it",
		updateRedirectWhy(refused),
		manifestRedirectFix(refused, manifestURL))
}

// manifestRedirectFix names the url that reaches the manifest with no
// redirect. A target over http is no url to configure: Ze refuses a plain
// http update url to any host but this one (ValidateUpdateCheckURL), so the
// server has to answer over https where it is.
func manifestRedirectFix(refused *fetch.RedirectRefused, manifestURL string) string {
	if refused.TargetScheme() == "https" {
		return "set system update-check url " + refused.Target()
	}
	return "serve the manifest at " + redact.URL(manifestURL) + " over https, with no redirect"
}

// updateRedirectWhy says why an update request refused a redirect, for the
// manifest and the binary alike.
func updateRedirectWhy(refused *fetch.RedirectRefused) string {
	return "an update request follows a redirect only within the scheme and host it started at, " +
		"and this one leaves " + refused.Left() + ": another host is a server nobody configured, " +
		"and http drops the TLS that authenticates the server"
}

// ValidateUpdateCheckURL returns an error if the URL is not acceptable.
// HTTPS is required; HTTP is permitted only from the host itself.
//
// The rule is config.ValidateFetchURL, which every URL Ze reads a file from
// answers to. This function held its own copy until 2026-09-03, and that copy
// compared the host by PREFIX: "http://127.0.0.1.example.com/" passed it, and
// reached this function from a self-update manifest's download URL
// (selfupdate.go) as well as from the operator's own leaf.
func ValidateUpdateCheckURL(rawURL string) error {
	if err := config.ValidateFetchURL(rawURL); err != nil {
		return fmt.Errorf("update-check url: %w", err)
	}
	return nil
}

// Start begins the periodic check loop. Safe to call once.
func (uc *UpdateChecker) Start(ctx context.Context) {
	ctx, uc.cancel = context.WithCancel(ctx)
	go uc.run(ctx)
}

// Stop halts the checker and waits for the goroutine to exit.
func (uc *UpdateChecker) Stop() {
	if uc.cancel != nil {
		uc.cancel()
	}
	<-uc.done
}

// Status returns the most recent check result.
func (uc *UpdateChecker) Status() UpdateStatus {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.status
}

func (uc *UpdateChecker) run(ctx context.Context) {
	defer close(uc.done)

	uc.check(ctx)

	ticker := time.NewTicker(uc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			report.ClearWarning(updateSource, updateCode, updateSubject)
			return
		case <-ticker.C:
			uc.check(ctx)
		}
	}
}

func (uc *UpdateChecker) runningVersion() string {
	if uc.running != "" {
		return uc.running
	}
	return version.Release()
}

func (uc *UpdateChecker) check(ctx context.Context) {
	logger := slogutil.Logger("update-check")
	running := uc.runningVersion()

	remoteVer, err := uc.fetchVersion(ctx)
	if err != nil {
		logger.Warn("fetch failed", "url", redact.URL(uc.url), "error", err)
		uc.mu.Lock()
		uc.status = UpdateStatus{
			LastCheck:      time.Now(),
			RunningVersion: running,
			LastError:      err.Error(),
		}
		uc.mu.Unlock()
		return
	}

	available := remoteVer != "" && remoteVer != running && isNewer(remoteVer, running)

	uc.mu.Lock()
	uc.status = UpdateStatus{
		LastCheck:       time.Now(),
		RunningVersion:  running,
		RemoteVersion:   remoteVer,
		UpdateAvailable: available,
	}
	uc.mu.Unlock()

	if available {
		report.RaiseWarning(updateSource, updateCode, updateSubject,
			"newer firmware available: "+remoteVer+" (running "+running+")",
			map[string]any{reportFieldRemoteVersion: remoteVer, reportFieldRunningVersion: running})
	} else {
		report.ClearWarning(updateSource, updateCode, updateSubject)
	}
}

// UpdateCheckConfig holds the extracted update-check config from a map tree.
type UpdateCheckConfig struct {
	URL        string
	Interval   uint32
	SelfUpdate SelfUpdateConfig
}

// ExtractUpdateCheckFromMap extracts update-check config from a map tree
// (used by the reload path which has no *config.Tree).
func ExtractUpdateCheckFromMap(tree map[string]any) UpdateCheckConfig {
	sys, _ := tree["system"].(map[string]any)
	if sys == nil {
		return UpdateCheckConfig{}
	}
	uc, _ := sys["update-check"].(map[string]any)
	if uc == nil {
		return UpdateCheckConfig{}
	}
	cfg := UpdateCheckConfig{Interval: 86400}
	if url, _ := uc["url"].(string); url != "" {
		cfg.URL = url
	}
	if v, _ := uc["interval"].(string); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 60 && n <= 604800 {
			cfg.Interval = uint32(n) //nolint:gosec // Bounded by range check above
		}
	}
	cfg.SelfUpdate = extractSelfUpdateFromMap(uc)
	return cfg
}

func extractSelfUpdateFromMap(uc map[string]any) SelfUpdateConfig {
	var cfg SelfUpdateConfig

	if v, _ := uc["auto-apply"].(string); v == "true" {
		cfg.AutoApply = true
	}

	cfg.Spread = 3600
	if v, _ := uc["spread"].(string); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 && n <= 86400 {
			cfg.Spread = uint32(n) //nolint:gosec // Bounded by range check above
		}
	}

	if mw, _ := uc["maintenance-window"].(map[string]any); mw != nil {
		if v, _ := mw["start"].(string); v != "" {
			cfg.MaintenanceStart = v
		}
		if v, _ := mw["end"].(string); v != "" {
			cfg.MaintenanceEnd = v
		}
	}

	if restart, _ := uc["restart"].(map[string]any); restart != nil {
		if _, ok := restart["immediate"]; ok {
			cfg.RestartImmediate = true
		}
		if v, _ := restart["time"].(string); v != "" {
			cfg.RestartTime = v
		}
	}

	return cfg
}

// versionManifest is the expected JSON structure at the version URL.
type versionManifest struct {
	Version string `json:"version"`
}

func (uc *UpdateChecker) fetchVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uc.url, http.NoBody)
	if err != nil {
		return "", redact.URLError(err)
	}
	req.Header.Set("User-Agent", version.HTTPHeader())
	req.Header.Set("X-Ze-Arch", runtime.GOOS+"/"+runtime.GOARCH)

	// The URL is operator-supplied and may carry userinfo, and every error on
	// this path reaches the operator's terminal through UpdateStatus.LastError
	// as well as the log, so each one names the URL through redact.
	resp, err := uc.client.Do(req)
	if err != nil {
		return "", manifestFetchError(err, uc.url)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, redact.URL(uc.url))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, updateMaxBody))
	if err != nil {
		return "", err
	}

	var manifest versionManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return "", err
	}

	return strings.TrimSpace(manifest.Version), nil
}

// isNewer returns true if remote is lexicographically greater than running.
// Ze uses date-based versioning (e.g. "26.05.17") where lexicographic
// comparison is correct. Non-numeric prefixes ("dev", "unknown") on either
// side mean the comparison is not meaningful, so returns false.
func isNewer(remote, running string) bool {
	if running == "" || running[0] < '0' || running[0] > '9' {
		return false
	}
	if remote == "" || remote[0] < '0' || remote[0] > '9' {
		return false
	}
	return remote > running
}
