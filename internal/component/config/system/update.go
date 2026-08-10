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

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
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
		client: &http.Client{
			Timeout: updateFetchTimeout,
		},
		done: make(chan struct{}),
	}
}

// ValidateUpdateCheckURL returns an error if the URL is not acceptable.
// HTTPS is required; HTTP is permitted only for 127.0.0.1 and localhost.
func ValidateUpdateCheckURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("update-check url is empty")
	}
	if strings.HasPrefix(rawURL, "https://") {
		return nil
	}
	if strings.HasPrefix(rawURL, "http://127.0.0.1") ||
		rawURL == "http://localhost" ||
		strings.HasPrefix(rawURL, "http://localhost/") ||
		strings.HasPrefix(rawURL, "http://localhost:") {
		return nil
	}
	return fmt.Errorf("update-check url must use HTTPS: %s", rawURL)
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
		logger.Warn("fetch failed", "url", uc.url, "error", err)
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
			map[string]any{"remote": remoteVer, "running": running})
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
		return "", err
	}
	req.Header.Set("User-Agent", version.HTTPHeader())
	req.Header.Set("X-Ze-Arch", runtime.GOOS+"/"+runtime.GOARCH)

	resp, err := uc.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, uc.url)
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
