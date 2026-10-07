// Design: docs/architecture/config/yang-config-design.md — config archive

package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
)

var errEmptyArchiveLocation = errors.New("empty archive location")

// Archive URL schemes.
const (
	schemeFile  = "file"
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// Trigger keywords for archive blocks.
// enumeration: gated by TestTriggerVocabularyMatchesModel
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
// to configured locations. Returns a slice of errors: one per location whose
// write failed, and one per location written whose commit-revisions pruning
// failed. Returns nil if all locations succeed or no locations are configured.
type Notifier func(content []byte) []error

// NewNotifier creates a Notifier for the given named archive configs.
// Uses fan-out: all configs are attempted regardless of individual failures.
// eventFn is called after each successful archive (may be nil).
func NewNotifier(configFile string, configs []ArchiveConfig, sys *system.SystemConfig, eventFn EventEmitter) Notifier {
	return func(content []byte) []error {
		var errs []error
		ts := time.Now()

		for _, ac := range configs {
			filename := FormatFilename(ac.Filename, configFile, sys, ac.Name, ts)
			if err := archiveToLocation(content, ac.Location, filename, ac.Timeout); err != nil {
				errs = append(errs, fmt.Errorf("archive %s: %w", ac.Name, err))
				continue
			}
			if eventFn != nil {
				eventFn(ac.Name, filename, content)
			}
			if err := pruneAfterWrite(ac, configFile, sys); err != nil {
				errs = append(errs, fmt.Errorf("archive %s: %w", ac.Name, err))
			}
		}

		return errs
	}
}

// pruneAfterWrite applies commit-revisions to the location one archive block
// has just written to. A cap of 0 keeps every file. Every archive write path
// calls it: the notifier, the boot archive and each daily or hourly archive.
func pruneAfterWrite(ac ArchiveConfig, configFile string, sys *system.SystemConfig) error {
	if sys.CommitRevisions == 0 {
		return nil
	}
	matcher, err := ArchiveMatcher(ac.Filename, configFile, sys, ac.Name)
	if err != nil {
		return err
	}
	return PruneFileArchives(ac.Location, sys.CommitRevisions, matcher)
}

// FormatFilename generates a filename by substituting tokens in the format string.
// Tokens: {name} = config basename, {host} = system host, {domain} = system domain,
// {date} = YYYYMMDD, {time} = HHMMSS, {archive} = archive block name.
// Always appends .conf extension.
func FormatFilename(format, configFile string, sys *system.SystemConfig, archiveName string, ts time.Time) string {
	if format == "" {
		format = DefaultFilenameFormat
	}

	r := strings.NewReplacer(
		"{name}", configBaseName(configFile),
		"{host}", sys.Host,
		"{domain}", sys.Domain,
		"{date}", ts.Format(dateLayout),
		"{time}", ts.Format(timeLayout),
		"{archive}", archiveName,
	)

	return r.Replace(format) + ".conf"
}

// Layouts of the {date} and {time} filename tokens. ArchiveMatcher derives
// the digit count it accepts from their length.
const (
	dateLayout = "20060102"
	timeLayout = "150405"
)

// configBaseName is the {name} token: the config file name without directory
// or extension.
func configBaseName(configFile string) string {
	base := filepath.Base(configFile)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// RedactURL sanitizes a URL string by replacing any embedded password with "xxxxx".
func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	return parsed.Redacted()
}

// ValidateLocation checks that a URL has a supported scheme.
// Supported schemes: file, http, https.
func ValidateLocation(rawURL string) error {
	if rawURL == "" {
		return errEmptyArchiveLocation
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	switch parsed.Scheme {
	case schemeFile, schemeHTTP, schemeHTTPS:
		return nil
	case "":
		return fmt.Errorf("missing URL scheme in %q (use file://, http://, or https://)", parsed.Redacted())
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

	// Path traversal check: reject filenames that escape the destination directory.
	// Config tokens like {host} could contain path separators (e.g., "../../etc/cron.d/evil").
	cleanDir := filepath.Clean(destDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(destPath)+string(filepath.Separator), cleanDir) {
		return fmt.Errorf("archive filename %q escapes destination directory %s", filename, destDir)
	}

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
		return fmt.Errorf("archive HTTP upload to %s: %w", RedactURL(baseURL), err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck // drain for connection reuse
		resp.Body.Close()              //nolint:errcheck // close error non-fatal
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("archive HTTP upload to %s: status %d", RedactURL(baseURL), resp.StatusCode)
	}

	return nil
}

// archiveToLocation dispatches to the appropriate uploader based on URL scheme.
func archiveToLocation(content []byte, location, filename string, timeout time.Duration) error {
	parsed, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("invalid archive location %q: %w", RedactURL(location), err)
	}

	switch parsed.Scheme {
	case schemeFile:
		return ToFile(content, parsed.Path, filename)
	case schemeHTTP, schemeHTTPS:
		return ToHTTP(content, location, filename, timeout)
	}

	return fmt.Errorf("unsupported archive scheme %q in location %q", parsed.Scheme, parsed.Redacted())
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

// ArchiveMatcher returns the pattern that a file name matches when this
// archive block could have written it: the whole format, with {date} as 8
// digits, {time} as 6 digits, every other token as its literal value, and the
// .conf extension. Pruning counts and removes only matching files, so a file
// that differs from the format anywhere but in those digits is never touched,
// even when the format starts with {date} or {time}. A file someone else gave
// a name of the same shape is counted as one of this block's copies.
//
// The literal parts are quoted first and the tokens replaced in the quoted
// text: QuoteMeta escapes each brace, so a token reads `\{name\}` there.
//
// The error is an operating error, not a Ze defect: Go's regexp refuses a
// pattern that is not valid UTF-8, and the config file name, the host, the
// domain, the block name and the format all come from outside Ze.
func ArchiveMatcher(format, configFile string, sys *system.SystemConfig, archiveName string) (*regexp.Regexp, error) {
	if format == "" {
		format = DefaultFilenameFormat
	}

	r := strings.NewReplacer(
		regexp.QuoteMeta("{name}"), regexp.QuoteMeta(configBaseName(configFile)),
		regexp.QuoteMeta("{host}"), regexp.QuoteMeta(sys.Host),
		regexp.QuoteMeta("{domain}"), regexp.QuoteMeta(sys.Domain),
		regexp.QuoteMeta("{date}"), "[0-9]{"+strconv.Itoa(len(dateLayout))+"}",
		regexp.QuoteMeta("{time}"), "[0-9]{"+strconv.Itoa(len(timeLayout))+"}",
		regexp.QuoteMeta("{archive}"), regexp.QuoteMeta(archiveName),
	)

	matcher, err := regexp.Compile("^" + r.Replace(regexp.QuoteMeta(format)) + `\.conf$`)
	if err != nil {
		return nil, fmt.Errorf("archive filename matcher: %w", err)
	}
	return matcher, nil
}

// PruneFileArchives removes the oldest files that matcher accepts from a
// file:// archive location, keeping at most maxKeep of them; build matcher
// with ArchiveMatcher. Files are ordered by modification time, and files with
// equal times by name, so the choice is the same on every run. A file that
// is gone by the time it is examined is skipped. Non-file schemes are
// ignored: the receiving server decides what to keep. The error reports a
// directory that cannot be read and every file that could not be removed.
func PruneFileArchives(location string, maxKeep uint16, matcher *regexp.Regexp) error {
	parsed, err := url.Parse(location)
	if err != nil {
		return errors.New("prune: archive location does not parse")
	}
	if parsed.Scheme != schemeFile {
		return nil
	}

	dir := parsed.Path
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("prune %s: %w", dir, err)
	}

	type fileWithTime struct {
		name    string
		modTime time.Time
	}
	var files []fileWithTime
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !matcher.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the listing: nothing left to prune
		}
		if err != nil {
			return fmt.Errorf("prune: %w", err)
		}
		files = append(files, fileWithTime{name: e.Name(), modTime: info.ModTime()})
	}

	if len(files) <= int(maxKeep) {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].name < files[j].name
		}
		return files[i].modTime.Before(files[j].modTime)
	})

	var errs []error
	for _, f := range files[:len(files)-int(maxKeep)] {
		if err := os.Remove(filepath.Join(dir, f.name)); err != nil {
			errs = append(errs, fmt.Errorf("prune: %w", err))
		}
	}
	return errors.Join(errs...)
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
