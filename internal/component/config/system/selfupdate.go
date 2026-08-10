//go:build ze_distro

// Design: docs/architecture/appliance/self-update.md -- download, verify, stage, restart logic

package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/identity"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/version"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	selfUpdateSource  = "system"
	selfUpdateCode    = "self-update"
	selfUpdateSubject = "firmware"

	downloadMaxBytes = 500 << 20 // 500 MB hard cap
	historyMaxEvents = 20
	drainDelay       = 5 * time.Second

	statusStaged             = "staged"
	statusComplete           = "complete"
	statusWaitingMaintenance = "waiting for maintenance window"
)

// extendedManifest is the enhanced server manifest (wire format, not config).
type extendedManifest struct {
	Ver            string `json:"version"`
	SHA256         string `json:"sha256,omitempty"`
	Size           int64  `json:"size,omitempty"`
	Paused         bool   `json:"paused,omitempty"`
	MinimumVersion string `json:"minimum-version,omitempty"`
	DownloadURL    string `json:"download-url,omitempty"`
}

// SelfUpdater extends UpdateChecker with download/verify/stage/restart logic.
type SelfUpdater struct {
	url      string
	interval time.Duration
	client   *http.Client
	running  string

	cfg SelfUpdateConfig

	mu               sync.RWMutex
	status           UpdateStatus
	downloadStatus   string
	downloadSHA256   string
	stagedVersion    string
	stagedPath       string
	serverPaused     bool
	heldVersion      string
	verifiedTempPath string

	spreadDelay     map[string]time.Duration
	spreadFirstSeen map[string]time.Time

	history   []UpdateEvent
	historyMu sync.Mutex

	targetPath string
	identity   string

	cancel context.CancelFunc
	done   chan struct{}

	store        identity.Storage
	restartFunc  func(binPath string) error
	nowFunc      func() time.Time
	identityFunc func() string
}

// newSelfUpdater creates a self-updater. Call Start to begin.
// The store parameter is used for persisting machine identity in zefs;
// nil is safe (falls back to filesystem sources).
func newSelfUpdater(url string, intervalSecs uint32, cfg SelfUpdateConfig, store identity.Storage) *SelfUpdater {
	return &SelfUpdater{
		url:      url,
		interval: time.Duration(intervalSecs) * time.Second,
		client: &http.Client{
			Timeout: updateFetchTimeout,
		},
		cfg:             cfg,
		done:            make(chan struct{}),
		spreadDelay:     make(map[string]time.Duration),
		spreadFirstSeen: make(map[string]time.Time),
		restartFunc:     defaultRestart,
		nowFunc:         time.Now,
		store:           store,
	}
}

func defaultRestart(binPath string) error {
	return syscall.Exec(binPath, os.Args, os.Environ()) //nolint:gosec // G204: binary path is from os.Executable, not user input
}

// Start begins the periodic check loop.
func (su *SelfUpdater) Start(ctx context.Context) {
	ctx, su.cancel = context.WithCancel(ctx)
	su.resolveTarget()
	su.resolveIdentity()
	su.cleanStaleTempFiles()
	su.loadHistory()
	go su.run(ctx)
}

// Stop halts the updater and waits for the goroutine to exit.
func (su *SelfUpdater) Stop() {
	if su.cancel != nil {
		su.cancel()
	}
	<-su.done
}

// Status returns the current update status (compatible with UpdateChecker).
func (su *SelfUpdater) Status() UpdateStatus {
	su.mu.RLock()
	defer su.mu.RUnlock()
	return su.status
}

// extendedStatus returns the full self-update status.
func (su *SelfUpdater) extendedStatus() ExtendedUpdateStatus {
	su.mu.RLock()
	defer su.mu.RUnlock()
	return ExtendedUpdateStatus{
		UpdateStatus:   su.status,
		DownloadStatus: su.downloadStatus,
		DownloadSHA256: su.downloadSHA256,
		StagedVersion:  su.stagedVersion,
		StagedPath:     su.stagedPath,
		RestartPolicy:  su.restartPolicy(),
		ServerPaused:   su.serverPaused,
		HeldVersion:    su.heldVersion,
	}
}

// History returns the update event history.
func (su *SelfUpdater) History() []UpdateEvent {
	su.historyMu.Lock()
	defer su.historyMu.Unlock()
	out := make([]UpdateEvent, len(su.history))
	copy(out, su.history)
	return out
}

func (su *SelfUpdater) restartPolicy() string {
	if su.cfg.RestartImmediate {
		return "immediate"
	}
	if su.cfg.RestartTime != "" {
		var tb textbuf.Buffer
		return tb.Str("time ").Str(su.cfg.RestartTime).String()
	}
	return "manual"
}

func (su *SelfUpdater) run(ctx context.Context) {
	defer close(su.done)
	su.check(ctx)

	ticker := time.NewTicker(su.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			report.ClearWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject)
			return
		case <-ticker.C:
			su.check(ctx)
		}
	}
}

func (su *SelfUpdater) runningVersion() string {
	if su.running != "" {
		return su.running
	}
	return version.Release()
}

func (su *SelfUpdater) check(ctx context.Context) {
	logger := slogutil.Logger("self-update")
	running := su.runningVersion()

	manifest, err := su.fetchManifest(ctx)
	if err != nil {
		logger.Warn("fetch failed", "url", su.url, "error", err)
		su.mu.Lock()
		su.status = UpdateStatus{
			LastCheck:      su.nowFunc(),
			RunningVersion: running,
			LastError:      err.Error(),
		}
		su.downloadStatus = ""
		su.mu.Unlock()
		return
	}

	available := manifest.Ver != "" && manifest.Ver != running && isNewer(manifest.Ver, running)

	su.mu.Lock()
	su.status = UpdateStatus{
		LastCheck:       su.nowFunc(),
		RunningVersion:  running,
		RemoteVersion:   manifest.Ver,
		UpdateAvailable: available,
	}
	su.serverPaused = manifest.Paused
	su.mu.Unlock()

	if !available {
		report.ClearWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject)
		su.mu.Lock()
		su.downloadStatus = ""
		su.mu.Unlock()
		return
	}

	if manifest.Paused {
		su.mu.Lock()
		su.downloadStatus = "paused by server"
		su.mu.Unlock()
		report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
			"update available ("+manifest.Ver+"), paused by server",
			map[string]any{"remote": manifest.Ver, "running": running, "paused": true})
		return
	}

	if !su.cfg.AutoApply {
		report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
			"newer firmware available: "+manifest.Ver+" (running "+running+")",
			map[string]any{"remote": manifest.Ver, "running": running})
		return
	}

	if manifest.SHA256 == "" {
		su.mu.Lock()
		su.downloadStatus = "error: auto-apply requires server to provide sha256"
		su.mu.Unlock()
		report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
			"auto-apply requires server to provide sha256",
			map[string]any{"remote": manifest.Ver, "running": running})
		return
	}

	if manifest.MinimumVersion != "" && isNewer(manifest.MinimumVersion, running) {
		su.mu.Lock()
		var tb textbuf.Buffer
		su.downloadStatus = tb.Str("error: upgrade requires intermediate version ").Str(manifest.MinimumVersion).String()
		su.mu.Unlock()
		su.recordEvent(running, manifest.Ver, "blocked-minimum-version")
		report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
			"upgrade to "+manifest.Ver+" requires intermediate version "+manifest.MinimumVersion+" first",
			map[string]any{"remote": manifest.Ver, "running": running, "minimum": manifest.MinimumVersion})
		return
	}

	su.mu.RLock()
	heldVer := su.heldVersion
	heldTemp := su.verifiedTempPath
	su.mu.RUnlock()

	if heldVer == manifest.Ver && heldTemp != "" {
		if fileExists(heldTemp) {
			su.attemptStage(ctx, manifest, running)
			return
		}
		su.mu.Lock()
		su.heldVersion = ""
		su.verifiedTempPath = ""
		su.mu.Unlock()
	}

	if heldVer != "" && heldVer != manifest.Ver && heldTemp != "" {
		os.Remove(heldTemp) //nolint:errcheck // best-effort cleanup of superseded temp
		su.mu.Lock()
		su.heldVersion = ""
		su.verifiedTempPath = ""
		su.downloadStatus = ""
		su.mu.Unlock()
	}

	if su.cfg.Spread > 0 {
		delay := su.computeSpreadDelay(manifest.Ver)
		firstSeen := su.getFirstSeen(manifest.Ver)
		elapsed := su.nowFunc().Sub(firstSeen)
		if elapsed < delay {
			su.mu.Lock()
			su.downloadStatus = "waiting for spread"
			su.mu.Unlock()
			report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
				"update available ("+manifest.Ver+"), waiting for spread delay",
				map[string]any{"remote": manifest.Ver, "running": running})
			return
		}
	}

	su.download(ctx, manifest, running)
}

func (su *SelfUpdater) download(ctx context.Context, manifest extendedManifest, running string) {
	logger := slogutil.Logger("self-update")

	su.mu.Lock()
	su.downloadStatus = "downloading"
	su.mu.Unlock()

	downloadURL, err := su.resolveDownloadURL(manifest)
	if err != nil {
		logger.Warn("download URL error", "error", err)
		su.mu.Lock()
		var tb textbuf.Buffer
		su.downloadStatus = tb.Str("error: ").Err(err).String()
		su.mu.Unlock()
		su.recordEvent(running, manifest.Ver, "failed-download")
		return
	}

	if manifest.Size > 0 {
		if err := su.checkDiskSpace(manifest.Size); err != nil {
			logger.Warn("disk space check failed", "error", err)
			su.mu.Lock()
			var tb textbuf.Buffer
			su.downloadStatus = tb.Str("error: ").Err(err).String()
			su.mu.Unlock()
			su.recordEvent(running, manifest.Ver, "failed-download")
			return
		}
	}

	tempPath, err := su.downloadBinary(ctx, downloadURL)
	if err != nil {
		logger.Warn("download failed", "url", downloadURL, "error", err)
		su.mu.Lock()
		su.downloadStatus = "error: download failed"
		su.mu.Unlock()
		su.recordEvent(running, manifest.Ver, "failed-download")
		return
	}

	su.mu.Lock()
	su.downloadStatus = "verifying"
	su.mu.Unlock()

	actualHash, err := hashFile(tempPath)
	if err != nil {
		os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on hash failure
		logger.Warn("hash computation failed", "error", err)
		su.mu.Lock()
		su.downloadStatus = "error: hash computation failed"
		su.mu.Unlock()
		su.recordEvent(running, manifest.Ver, "failed-checksum")
		return
	}

	if !strings.EqualFold(actualHash, manifest.SHA256) {
		os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on mismatch
		logger.Warn("checksum mismatch", "expected", manifest.SHA256, "actual", actualHash)
		su.mu.Lock()
		su.downloadStatus = "error: checksum mismatch"
		su.mu.Unlock()
		su.recordEvent(running, manifest.Ver, "failed-checksum")
		return
	}

	su.mu.Lock()
	su.heldVersion = manifest.Ver
	su.verifiedTempPath = tempPath
	su.downloadStatus = statusComplete
	su.downloadSHA256 = actualHash
	su.mu.Unlock()

	su.attemptStage(ctx, manifest, running)
}

func (su *SelfUpdater) attemptStage(ctx context.Context, manifest extendedManifest, running string) {
	if su.cfg.MaintenanceStart != "" && su.cfg.MaintenanceEnd != "" {
		if !su.inMaintenanceWindow() {
			su.mu.Lock()
			su.downloadStatus = statusWaitingMaintenance
			su.mu.Unlock()
			report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
				"update downloaded, waiting for maintenance window",
				map[string]any{"remote": manifest.Ver, "running": running})
			return
		}
	}

	su.mu.RLock()
	tempPath := su.verifiedTempPath
	su.mu.RUnlock()

	if tempPath == "" {
		return
	}

	if err := su.stageBinary(tempPath); err != nil {
		slogutil.Logger("self-update").Warn("stage failed", "error", err)
		su.mu.Lock()
		su.downloadStatus = "error: stage failed"
		su.mu.Unlock()
		su.recordEvent(running, manifest.Ver, "failed-stage")
		return
	}

	su.mu.Lock()
	su.stagedVersion = manifest.Ver
	su.stagedPath = su.targetPath
	su.downloadStatus = statusStaged
	su.heldVersion = ""
	su.verifiedTempPath = ""
	su.mu.Unlock()

	su.recordEvent(running, manifest.Ver, "success")

	report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
		"update staged: "+manifest.Ver+" (restart to activate)",
		map[string]any{"remote": manifest.Ver, "running": running, "staged": true})

	su.handleRestart(ctx, manifest.Ver)
}

func (su *SelfUpdater) handleRestart(ctx context.Context, newVer string) {
	logger := slogutil.Logger("self-update")

	if su.cfg.RestartImmediate {
		logger.Info("restarting in 5 seconds", "new-ver", newVer)
		report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
			"restarting into "+newVer+" in 5 seconds",
			map[string]any{"new-ver": newVer, "action": "restart-immediate"})

		select {
		case <-time.After(drainDelay):
		case <-ctx.Done():
			return
		}

		if err := su.restartFunc(su.targetPath); err != nil {
			logger.Warn("exec failed", "error", err)
		}
		return
	}

	if su.cfg.RestartTime != "" {
		go su.waitForRestartTime(ctx, newVer)
		return
	}

	report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
		"update staged, restart to activate ("+newVer+")",
		map[string]any{"new-ver": newVer, "action": "manual-restart-required"})
}

func (su *SelfUpdater) waitForRestartTime(ctx context.Context, newVer string) {
	logger := slogutil.Logger("self-update")
	target, err := parseHHMM(su.cfg.RestartTime)
	if err != nil {
		logger.Warn("invalid restart time", "time", su.cfg.RestartTime, "error", err)
		return
	}

	now := su.nowFunc()
	next := time.Date(now.Year(), now.Month(), now.Day(), target.hour, target.minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	delay := next.Sub(now)
	logger.Info("scheduled restart", "at", next.Format("15:04"), "in", delay.Truncate(time.Second))

	select {
	case <-time.After(delay):
		logger.Info("restarting (scheduled)", "new-ver", newVer)
		report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
			"restarting into "+newVer+" (scheduled)",
			map[string]any{"new-ver": newVer, "action": "restart-scheduled"})
		if err := su.restartFunc(su.targetPath); err != nil {
			logger.Warn("exec failed", "error", err)
		}
	case <-ctx.Done():
	}
}

// manualCheck triggers an immediate check.
func (su *SelfUpdater) manualCheck(ctx context.Context) {
	su.check(ctx)
}

// manualDownload triggers an immediate download bypassing spread and maintenance window.
func (su *SelfUpdater) manualDownload(ctx context.Context) (string, error) {
	logger := slogutil.Logger("self-update")
	running := su.runningVersion()
	manifest, err := su.fetchManifest(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch manifest: %w", err)
	}

	if !isNewer(manifest.Ver, running) {
		return "", errors.New("no newer version available")
	}

	if manifest.MinimumVersion != "" && isNewer(manifest.MinimumVersion, running) {
		return "", fmt.Errorf("upgrade to %s requires intermediate version %s first", manifest.Ver, manifest.MinimumVersion)
	}

	downloadURL, err := su.resolveDownloadURL(manifest)
	if err != nil {
		return "", err
	}

	tempPath, err := su.downloadBinary(ctx, downloadURL)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	if manifest.SHA256 != "" {
		actualHash, hashErr := hashFile(tempPath)
		if hashErr != nil {
			os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on hash failure
			return "", fmt.Errorf("hash: %w", hashErr)
		}
		if !strings.EqualFold(actualHash, manifest.SHA256) {
			os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on mismatch
			return "", errors.New("checksum mismatch")
		}
		su.mu.Lock()
		su.downloadSHA256 = actualHash
		su.mu.Unlock()
	} else {
		logger.Warn("no checksum available, binary will not be verified")
	}

	su.mu.Lock()
	su.heldVersion = manifest.Ver
	su.verifiedTempPath = tempPath
	su.downloadStatus = statusComplete
	su.mu.Unlock()

	return manifest.Ver, nil
}

// manualApply triggers the full update cycle bypassing all scheduling.
func (su *SelfUpdater) manualApply(ctx context.Context) (string, error) {
	logger := slogutil.Logger("self-update")
	running := su.runningVersion()
	manifest, err := su.fetchManifest(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch manifest: %w", err)
	}

	if !isNewer(manifest.Ver, running) {
		return "", errors.New("no newer version available")
	}

	if manifest.MinimumVersion != "" && isNewer(manifest.MinimumVersion, running) {
		return "", fmt.Errorf("upgrade to %s requires intermediate version %s first", manifest.Ver, manifest.MinimumVersion)
	}

	downloadURL, err := su.resolveDownloadURL(manifest)
	if err != nil {
		return "", err
	}

	tempPath, err := su.downloadBinary(ctx, downloadURL)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	if manifest.SHA256 != "" {
		actualHash, hashErr := hashFile(tempPath)
		if hashErr != nil {
			os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on hash failure
			return "", fmt.Errorf("hash: %w", hashErr)
		}
		if !strings.EqualFold(actualHash, manifest.SHA256) {
			os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on mismatch
			su.recordEvent(running, manifest.Ver, "failed-checksum")
			return "", errors.New("checksum mismatch")
		}
		su.mu.Lock()
		su.downloadSHA256 = actualHash
		su.mu.Unlock()
	} else {
		logger.Warn("no checksum available, binary will not be verified")
	}

	if err := su.stageBinary(tempPath); err != nil {
		os.Remove(tempPath) //nolint:errcheck // best-effort cleanup on stage failure
		su.recordEvent(running, manifest.Ver, "failed-stage")
		return "", fmt.Errorf("stage: %w", err)
	}

	su.mu.Lock()
	su.stagedVersion = manifest.Ver
	su.stagedPath = su.targetPath
	su.downloadStatus = statusStaged
	su.mu.Unlock()

	su.recordEvent(running, manifest.Ver, "success")

	logger.Info("manual apply: restarting", "new-ver", manifest.Ver)
	report.RaiseWarning(selfUpdateSource, selfUpdateCode, selfUpdateSubject,
		"restarting into "+manifest.Ver,
		map[string]any{"new-ver": manifest.Ver, "action": "manual-apply"})

	select {
	case <-time.After(drainDelay):
	case <-ctx.Done():
		return manifest.Ver, ctx.Err()
	}

	if err := su.restartFunc(su.targetPath); err != nil {
		return manifest.Ver, fmt.Errorf("exec: %w", err)
	}
	return manifest.Ver, nil
}

// manualRestart restarts into a staged version.
func (su *SelfUpdater) manualRestart() error {
	su.mu.RLock()
	staged := su.stagedVersion
	su.mu.RUnlock()

	if staged == "" {
		return errors.New("no staged version")
	}

	slogutil.Logger("self-update").Info("manual restart", "staged-ver", staged)
	return su.restartFunc(su.targetPath)
}

// Rollback restores the .prev binary and restarts.
func (su *SelfUpdater) Rollback() error {
	var tb textbuf.Buffer
	prevPath := tb.Str(su.targetPath).Str(".prev").String()
	if !fileExists(prevPath) {
		return errors.New("no previous version available")
	}

	if err := os.Rename(prevPath, su.targetPath); err != nil {
		return fmt.Errorf("rollback rename: %w", err)
	}

	slogutil.Logger("self-update").Info("rolled back, restarting")
	return su.restartFunc(su.targetPath)
}

// --- manifest fetch ---

func (su *SelfUpdater) fetchManifest(ctx context.Context) (extendedManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, su.url, http.NoBody)
	if err != nil {
		return extendedManifest{}, err
	}
	req.Header.Set("User-Agent", version.HTTPHeader())
	req.Header.Set("X-Ze-Arch", runtime.GOOS+"/"+runtime.GOARCH)

	resp, err := su.client.Do(req)
	if err != nil {
		return extendedManifest{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	if resp.StatusCode != http.StatusOK {
		return extendedManifest{}, fmt.Errorf("HTTP %d from %s", resp.StatusCode, su.url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, updateMaxBody))
	if err != nil {
		return extendedManifest{}, err
	}

	var m extendedManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return extendedManifest{}, err
	}
	m.Ver = strings.TrimSpace(m.Ver)
	return m, nil
}

// --- download URL ---

func (su *SelfUpdater) resolveDownloadURL(manifest extendedManifest) (string, error) {
	if manifest.DownloadURL != "" {
		if err := ValidateUpdateCheckURL(manifest.DownloadURL); err != nil {
			return "", fmt.Errorf("download-url validation: %w", err)
		}
		return manifest.DownloadURL, nil
	}

	idx := strings.LastIndex(su.url, "/")
	if idx < 0 {
		return "", errors.New("cannot derive download URL from config URL")
	}
	var tb textbuf.Buffer
	return tb.Str(su.url[:idx+1]).Str(runtime.GOOS).Byte('/').Str(runtime.GOARCH).String(), nil
}

// --- binary download ---

func (su *SelfUpdater) downloadBinary(ctx context.Context, downloadURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", version.HTTPHeader())

	resp, err := su.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	dir := filepath.Dir(su.targetPath)
	base := filepath.Base(su.targetPath)
	tmpFile, err := os.CreateTemp(dir, base+".update.")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	tempPath := tmpFile.Name()

	cleanup := func() {
		tmpFile.Close()     //nolint:errcheck // best-effort close in cleanup
		os.Remove(tempPath) //nolint:errcheck // best-effort cleanup of temp file
	}

	limited := io.LimitReader(resp.Body, downloadMaxBytes+1)
	n, err := io.Copy(tmpFile, limited)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("download write: %w", err)
	}
	if n > downloadMaxBytes {
		cleanup()
		return "", errors.New("download exceeds 500 MB limit")
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tempPath) //nolint:errcheck // best-effort cleanup of temp file
		return "", fmt.Errorf("close temp: %w", err)
	}

	return tempPath, nil
}

// --- binary staging ---

func (su *SelfUpdater) stageBinary(tempPath string) error {
	target := su.targetPath

	// Pre-flight: verify writable filesystem
	var tb textbuf.Buffer
	testPath := tb.Str(target).Str(".writetest").String()
	f, err := os.Create(testPath) //nolint:gosec // G304: path derived from os.Executable, not user input
	if err != nil {
		return fmt.Errorf("self-update not supported on read-only filesystem: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(testPath) //nolint:errcheck // best-effort cleanup of writetest on close failure
		return fmt.Errorf("close writetest: %w", err)
	}
	os.Remove(testPath) //nolint:errcheck // best-effort cleanup of writetest

	// Copy permissions from current binary
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}
	if err := os.Chmod(tempPath, info.Mode()); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}

	// Hard-link current binary to .prev for rollback
	prevPath := tb.Reset().Str(target).Str(".prev").String()
	os.Remove(prevPath) //nolint:errcheck // old .prev may not exist
	if err := os.Link(target, prevPath); err != nil {
		return fmt.Errorf("backup link: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, target); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			return errors.New("binary and temp directory must be on the same filesystem")
		}
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

// --- disk space ---

func (su *SelfUpdater) checkDiskSpace(binarySize int64) error {
	dir := filepath.Dir(su.targetPath)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return nil //nolint:nilerr // skip check if statfs fails (unsupported FS)
	}
	available := int64(stat.Bavail) * int64(stat.Bsize) //nolint:gosec // product bounded by filesystem capacity
	needed := binarySize * 2
	if available < needed {
		return fmt.Errorf("insufficient disk space: need %d bytes, have %d", needed, available)
	}
	return nil
}

// --- spread ---

func (su *SelfUpdater) computeSpreadDelay(ver string) time.Duration {
	if d, ok := su.spreadDelay[ver]; ok {
		return d
	}
	identity := su.resolvedIdentity()
	h := fnv.New64a()
	h.Write([]byte(identity))
	h.Write([]byte(ver))
	seed := h.Sum64()
	delaySecs := seed % uint64(su.cfg.Spread)
	d := time.Duration(delaySecs) * time.Second
	su.spreadDelay[ver] = d
	return d
}

func (su *SelfUpdater) getFirstSeen(ver string) time.Time {
	if t, ok := su.spreadFirstSeen[ver]; ok {
		return t
	}
	t := su.nowFunc()
	su.spreadFirstSeen[ver] = t
	return t
}

// --- identity ---

func (su *SelfUpdater) resolvedIdentity() string {
	if su.identity != "" {
		return su.identity
	}
	if su.identityFunc != nil {
		su.identity = su.identityFunc()
		return su.identity
	}
	su.identity = identity.Resolve(su.store)
	return su.identity
}

// --- target resolution ---

func (su *SelfUpdater) resolveTarget() {
	exe, err := os.Executable()
	if err != nil {
		su.targetPath = os.Args[0]
		return
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		su.targetPath = exe
		return
	}
	su.targetPath = resolved
}

func (su *SelfUpdater) resolveIdentity() {
	su.identity = su.resolvedIdentity()
}

// --- stale temp cleanup ---

func (su *SelfUpdater) cleanStaleTempFiles() {
	dir := filepath.Dir(su.targetPath)
	base := filepath.Base(su.targetPath)
	var tb textbuf.Buffer
	pattern := tb.Str(base).Str(".update.").String()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), pattern) {
			os.Remove(filepath.Join(dir, e.Name())) //nolint:errcheck // best-effort stale temp cleanup
		}
	}
}

// --- maintenance window ---

func (su *SelfUpdater) inMaintenanceWindow() bool {
	start, err := parseHHMM(su.cfg.MaintenanceStart)
	if err != nil {
		return true
	}
	end, err := parseHHMM(su.cfg.MaintenanceEnd)
	if err != nil {
		return true
	}

	now := su.nowFunc()
	nowMinutes := now.Hour()*60 + now.Minute()
	startMinutes := start.hour*60 + start.minute
	endMinutes := end.hour*60 + end.minute

	if startMinutes <= endMinutes {
		return nowMinutes >= startMinutes && nowMinutes < endMinutes
	}
	return nowMinutes >= startMinutes || nowMinutes < endMinutes
}

// --- history ---

func (su *SelfUpdater) recordEvent(from, to, result string) {
	event := UpdateEvent{
		Timestamp:   su.nowFunc(),
		FromVersion: from,
		ToVersion:   to,
		Result:      result,
	}

	su.historyMu.Lock()
	su.history = append(su.history, event)
	if len(su.history) > historyMaxEvents {
		su.history = su.history[len(su.history)-historyMaxEvents:]
	}
	su.historyMu.Unlock()

	su.saveHistory()
}

// loadHistory restores the event history from the shared zefs store
// (<config-dir>/database.zefs) under the update-history key. Best-effort: a
// no-op when the store or key is absent, or the blob is malformed.
func (su *SelfUpdater) loadHistory() {
	data, ok := statestore.Get(zefs.KeyConfigUpdateHistory.Pattern)
	if !ok {
		return
	}
	var events []UpdateEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return
	}
	if len(events) > historyMaxEvents {
		events = events[len(events)-historyMaxEvents:]
	}
	su.historyMu.Lock()
	su.history = events
	su.historyMu.Unlock()
}

// saveHistory persists the event history into the shared zefs store under the
// update-history key. Best-effort: a no-op when no store exists (statestore
// never creates it); the next save retries.
func (su *SelfUpdater) saveHistory() {
	su.historyMu.Lock()
	events := make([]UpdateEvent, len(su.history))
	copy(events, su.history)
	su.historyMu.Unlock()

	data, err := json.Marshal(events)
	if err != nil {
		return
	}

	if _, perr := statestore.Put(zefs.KeyConfigUpdateHistory.Pattern, data); perr != nil {
		slogutil.Logger("self-update").Debug("history persist failed", "error", perr)
	}
}

// --- helpers ---

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path is from our temp dir, not user input
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only file handle

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
