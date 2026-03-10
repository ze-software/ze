// Design: docs/architecture/config/yang-config-design.md — config archive

package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
)

// Archive URL schemes.
const (
	schemeFile  = "file"
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// Trigger keywords for archive blocks.
const (
	TriggerCommit = "commit"
	TriggerManual = "manual"
	TriggerDaily  = "daily"
	TriggerHourly = "hourly"
)

// DefaultFilenameFormat is the default archive filename format when none is configured.
const DefaultFilenameFormat = "{name}-{host}-{date}-{time}"

// ArchiveConfig is the runtime representation of one named archive block.
type ArchiveConfig struct {
	Name     string
	Location string
	Filename string // Format string with tokens
	Timeout  time.Duration
	Trigger  string
	OnChange bool
}

// Notifier is called after a successful save to archive the config
// to configured locations. Returns a slice of errors (one per failed location).
// Returns nil if all locations succeed or no locations are configured.
type Notifier func(content []byte) []error

// NewNotifier creates a Notifier for the given named archive configs.
// Uses fan-out: all configs are attempted regardless of individual failures.
func NewNotifier(configFile string, configs []ArchiveConfig, sys system.SystemConfig) Notifier {
	return func(content []byte) []error {
		var errs []error
		ts := time.Now()

		for _, ac := range configs {
			filename := FormatFilename(ac.Filename, configFile, sys, ac.Name, ts)
			if err := archiveToLocation(content, ac.Location, filename, ac.Timeout); err != nil {
				errs = append(errs, fmt.Errorf("archive %s: %w", ac.Name, err))
			}
		}

		return errs
	}
}

// FormatFilename generates a filename by substituting tokens in the format string.
// Tokens: {name} = config basename, {host} = system host, {domain} = system domain,
// {date} = YYYYMMDD, {time} = HHMMSS, {archive} = archive block name.
// Always appends .conf extension.
func FormatFilename(format, configFile string, sys system.SystemConfig, archiveName string, ts time.Time) string {
	if format == "" {
		format = DefaultFilenameFormat
	}

	base := filepath.Base(configFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	r := strings.NewReplacer(
		"{name}", name,
		"{host}", sys.Host,
		"{domain}", sys.Domain,
		"{date}", ts.Format("20060102"),
		"{time}", ts.Format("150405"),
		"{archive}", archiveName,
	)

	return r.Replace(format) + ".conf"
}

// ValidateTrigger checks that a trigger keyword is valid.
func ValidateTrigger(trigger string) error {
	switch trigger {
	case TriggerCommit, TriggerManual, TriggerDaily, TriggerHourly:
		return nil
	case "":
		return fmt.Errorf("empty trigger value")
	}

	return fmt.Errorf("invalid trigger %q (valid: commit, manual, daily, hourly)", trigger)
}

// ValidateLocation checks that a URL has a supported scheme.
// Supported schemes: file, http, https.
func ValidateLocation(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("empty archive location")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	switch parsed.Scheme {
	case schemeFile, schemeHTTP, schemeHTTPS:
		return nil
	case "":
		return fmt.Errorf("missing URL scheme in %q (use file://, http://, or https://)", rawURL)
	}

	return fmt.Errorf("unsupported archive scheme %q (supported: file, http, https)", parsed.Scheme)
}

// ToFile writes content to a file in the target directory.
// Creates the destination directory if needed.
func ToFile(content []byte, destDir, filename string) error {
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("archive create directory %s: %w", destDir, err)
	}

	destPath := filepath.Join(destDir, filename)

	if err := os.WriteFile(destPath, content, 0o600); err != nil {
		return fmt.Errorf("archive to file %s: %w", destPath, err)
	}

	return nil
}

// ToHTTP POSTs config content to an HTTP(S) endpoint.
// The filename is passed in the X-Archive-Filename header.
func ToHTTP(content []byte, baseURL, filename string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("archive HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Archive-Filename", filename)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("archive HTTP upload to %s: %w", baseURL, err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck // drain for connection reuse
		resp.Body.Close()              //nolint:errcheck // close error non-fatal
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("archive HTTP upload to %s: status %d", baseURL, resp.StatusCode)
	}

	return nil
}

// archiveToLocation dispatches to the appropriate uploader based on URL scheme.
func archiveToLocation(content []byte, location, filename string, timeout time.Duration) error {
	parsed, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("invalid archive location %q: %w", location, err)
	}

	switch parsed.Scheme {
	case schemeFile:
		return ToFile(content, parsed.Path, filename)
	case schemeHTTP, schemeHTTPS:
		return ToHTTP(content, location, filename, timeout)
	}

	return fmt.Errorf("unsupported archive scheme %q in location %q", parsed.Scheme, location)
}

// FilterByTrigger returns only the configs with the given trigger type.
func FilterByTrigger(configs []ArchiveConfig, trigger string) []ArchiveConfig {
	var result []ArchiveConfig

	for _, ac := range configs {
		if ac.Trigger == trigger {
			result = append(result, ac)
		}
	}

	return result
}

// ExtractConfigs extracts named archive blocks from a parsed config tree.
// Reads from system.archive list entries.
// Returns nil if no system block or no archive entries exist.
func ExtractConfigs(tree *config.Tree) []ArchiveConfig {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}

	entries := sys.GetListOrdered("archive")
	if len(entries) == 0 {
		return nil
	}

	configs := make([]ArchiveConfig, 0, len(entries))

	for _, entry := range entries {
		ac := ArchiveConfig{
			Name:     entry.Key,
			Filename: DefaultFilenameFormat,
			Timeout:  30 * time.Second,
			Trigger:  TriggerManual,
		}

		if loc, ok := entry.Value.Get("location"); ok {
			ac.Location = loc
		}

		if fn, ok := entry.Value.Get("filename"); ok && fn != "" {
			ac.Filename = fn
		}

		if to, ok := entry.Value.Get("timeout"); ok {
			if d, err := time.ParseDuration(to); err == nil && d > 0 {
				ac.Timeout = d
			}
		}

		if tr, ok := entry.Value.Get("trigger"); ok && tr != "" {
			ac.Trigger = tr
		}

		if oc, ok := entry.Value.Get("on-change"); ok {
			ac.OnChange = oc == "true"
		}

		configs = append(configs, ac)
	}

	return configs
}

// ChangeTracker tracks config content changes per archive name using SHA-256 hashes.
// Thread-safe. Resets on daemon restart (in-memory only).
type ChangeTracker struct {
	mu     sync.Mutex
	hashes map[string][32]byte
}

// NewChangeTracker creates a new empty change tracker.
func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{
		hashes: make(map[string][32]byte),
	}
}

// HasChanged returns true if the content has changed since the last check for this name.
// First call for a name always returns true (boot behavior — no baseline yet).
func (ct *ChangeTracker) HasChanged(name string, content []byte) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	newHash := sha256.Sum256(content)

	oldHash, exists := ct.hashes[name]
	ct.hashes[name] = newHash

	if !exists {
		return true
	}

	return oldHash != newHash
}
