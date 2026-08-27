// Design: plan/spec-le-is-a-ze-binary.md -- native scratch-link gates
// Detail: move.go -- cross-device moves that preserve filesystem metadata
//
// Package scratch keeps tmp and cache outside a checkout without overwriting
// paths that already hold user work.
package scratch

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	area          = "scratch"
	tmpName       = "tmp"
	cacheName     = "cache"
	targetDirMode = 0o750
	sentinelMode  = 0o600
)

// Sentinel is the nested module written only while tmp is a real directory.
const Sentinel = `// Sentinel module: marks a REAL tmp/ as a nested module so ` + "`go list ./...`" + ` and
// ` + "`go test ./...`" + ` skip the Go/QEMU caches under it (they hold foreign go.mod files
// that would otherwise fail with "directory ... outside main module").
//
// Committed so it is present on a fresh checkout. scripts/dev/ensure-links.py recreates
// it whenever tmp/ is a real directory; after the opt-in ` + "`make ze-scratch-migrate`" + `, tmp/
// is a symlink that ` + "`go list`" + ` skips without any sentinel, so this file is not needed there.
// Keep this content in sync with SENTINEL in scripts/dev/ensure-links.py.
module ze-tmp-scratch

go 1.25
`

// migratableScratch is the closed allowlist of build-artifact directories that
// can leave a real tmp directory. Unclassified names stay beside session work.
var migratableScratch = [...]string{
	"qemu",
	"kernel",
	"gokrazy",
	"golangci-lint-cache",
	"terminal-demos",
}

// Result is one status line and the stream on which the Python producer wrote it.
type Result struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Line   string `json:"line"`
	Stderr bool   `json:"stderr"`
}

// Report is the structured answer from one ensure or migration run.
type Report struct {
	Results []Result `json:"results"`
	Quiet   bool     `json:"quiet,omitempty"`
}

// Text renders only stdout. Answer writes stderr rows separately so pipe output
// stays data while refusals remain visible to a person.
func (r Report) Text() string {
	if r.Quiet {
		flagged := false
		for _, result := range r.Results {
			if result.Stderr {
				flagged = true
				break
			}
		}
		if !flagged {
			return ""
		}
	}
	var text strings.Builder
	for _, result := range r.Results {
		if result.Stderr {
			continue
		}
		text.WriteString(result.Line)
		text.WriteByte('\n')
	}
	return text.String()
}

type filesystem struct {
	access func(string, uint32) error
	device func(string) (uint64, error)
	rename func(string, string) error
}

// Manager applies one action to one explicit checkout and environment. It is
// not safe for concurrent use. Separate managers can operate concurrently.
type Manager struct {
	Root    string
	Environ []string
	fs      filesystem
}

// New returns a scratch manager for root. Environ uses os.Environ syntax and
// last-value-wins lookup, matching an executed process environment.
func New(root string, environ []string) *Manager {
	return &Manager{
		Root:    root,
		Environ: append([]string(nil), environ...),
		fs: filesystem{
			access: unix.Access,
			device: deviceOf,
			rename: os.Rename,
		},
	}
}

// CheckoutID returns the stable human-readable key for one absolute checkout.
func CheckoutID(root string) string {
	digest := sha256.Sum256([]byte(root))
	var text textbuf.Buffer
	return text.Str(filepath.Base(root)).Byte('-').Hex(digest[:8]).String()
}

// ScratchTarget derives the per-checkout disposable target.
func (m *Manager) ScratchTarget() string {
	base := m.environment("TMPDIR")
	if base == "" {
		base = "/tmp"
	}
	return filepath.Join(base, "ze", CheckoutID(m.Root))
}

// CacheTarget derives the durable per-user target.
func (m *Manager) CacheTarget() (string, error) {
	if xdg := m.environment("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "ze"), nil
	}
	home := m.environment("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
	}
	return filepath.Join(home, ".cache", "ze"), nil
}

// Ensure creates or repairs the links without converting real paths.
func (m *Manager) Ensure(repointCache bool) (Report, int) {
	cacheTarget, err := m.CacheTarget()
	if err != nil {
		return failureReport(cacheName, err), 1
	}
	results := []Result{
		m.ensureSymlink(filepath.Join(m.Root, tmpName), m.ScratchTarget(), true),
		m.ensureSymlink(filepath.Join(m.Root, cacheName), cacheTarget, repointCache),
	}
	if err := m.ensureSentinel(); err != nil {
		results = append(results, errorResult(tmpName, err))
	}
	return Report{Results: results, Quiet: true}, verdict(results)
}

// Migrate selectively moves tmp artifacts and moves the whole cache directory.
func (m *Manager) Migrate(repointCache bool) (Report, int) {
	cacheTarget, err := m.CacheTarget()
	if err != nil {
		return failureReport(cacheName, err), 1
	}
	results := []Result{
		m.migrateScratchDirs(filepath.Join(m.Root, tmpName), m.ScratchTarget()),
		m.migrate(filepath.Join(m.Root, cacheName), cacheTarget, repointCache),
	}
	if err := m.ensureSentinel(); err != nil {
		results = append(results, errorResult(tmpName, err))
	}
	return Report{Results: results}, verdict(results)
}

func (m *Manager) environment(name string) string {
	value := ""
	var text textbuf.Buffer
	prefix := text.Str(name).Byte('=').String()
	for _, entry := range m.Environ {
		if environment, ok := strings.CutPrefix(entry, prefix); ok {
			value = environment
		}
	}
	return value
}

func (m *Manager) ensureSymlink(link, target string, autoRepoint bool) Result {
	info, err := os.Lstat(link)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return m.ensureExistingSymlink(link, target, autoRepoint)
		}
	}
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errorResult(filepath.Base(link), fmt.Errorf("inspect link: %w", err))
		}
	}
	if mkdirErr := os.MkdirAll(target, targetDirMode); mkdirErr != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("create target: %w", mkdirErr))
	}
	var text textbuf.Buffer
	if err == nil {
		line := text.Str("SKIP     ").Str(filepath.Base(link)).
			Str(": a real path exists here; run `make ze-scratch-migrate` to convert it to a symlink").String()
		return statusResult("SKIP", line)
	}
	if symlinkErr := os.Symlink(target, link); symlinkErr != nil {
		if errors.Is(symlinkErr, os.ErrExist) {
			line := text.Str("ok       ").Str(filepath.Base(link)).Str(" -> ").Str(target).
				Str(" (created concurrently)").String()
			return statusResult("ok", line)
		}
		return errorResult(filepath.Base(link), fmt.Errorf("create symlink: %w", symlinkErr))
	}
	line := text.Str("created  ").Str(filepath.Base(link)).Str(" -> ").Str(target).String()
	return statusResult("created", line)
}

func (m *Manager) ensureExistingSymlink(link, target string, autoRepoint bool) Result {
	current, err := os.Readlink(link)
	if err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("read symlink: %w", err))
	}
	var text textbuf.Buffer
	if current == target {
		if err := os.MkdirAll(target, targetDirMode); err != nil {
			return errorResult(filepath.Base(link), fmt.Errorf("create target: %w", err))
		}
		line := text.Str("ok       ").Str(filepath.Base(link)).Str(" -> ").Str(target).String()
		return statusResult("ok", line)
	}
	if !autoRepoint {
		line := text.Str("MISMATCH ").Str(filepath.Base(link)).Str(" -> ").Str(current).
			Str(" (expected ").Str(target).
			Str("); left as is. If this checkout is yours, run: python3 scripts/dev/ensure-links.py --repoint-cache").
			String()
		return statusResult("MISMATCH", line)
	}
	if err := os.MkdirAll(target, targetDirMode); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("create target: %w", err))
	}
	if err := os.Remove(link); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("remove old symlink: %w", err))
	}
	if err := os.Symlink(target, link); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("create symlink: %w", err))
	}
	return statusResult("repointed",
		text.Str("repointed ").Str(filepath.Base(link)).Str(" -> ").Str(target).String())
}

func (m *Manager) migrate(link, target string, autoRepoint bool) Result {
	info, err := os.Lstat(link)
	var text textbuf.Buffer
	if errors.Is(err, os.ErrNotExist) {
		return m.ensureSymlink(link, target, autoRepoint)
	}
	if err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("inspect path: %w", err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return m.ensureSymlink(link, target, autoRepoint)
	}
	if !info.IsDir() {
		return statusResult("REFUSE", text.Str("REFUSE   ").Str(filepath.Base(link)).
			Str(": exists and is not a directory; resolve manually").String())
	}
	undeletable, checkErr := m.firstUndeletableDir(link)
	if checkErr != nil {
		return errorResult(filepath.Base(link), checkErr)
	}
	if undeletable != "" {
		return undeletableResult(filepath.Base(link), undeletable)
	}
	if err := os.MkdirAll(target, targetDirMode); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("create target: %w", err))
	}
	entries, err := os.ReadDir(link)
	if err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("read directory: %w", err))
	}
	moved := 0
	for _, entry := range entries {
		destination := filepath.Join(target, entry.Name())
		if pathExists(destination) {
			line := text.Reset().Str("REFUSE   ").Str(filepath.Base(link)).Str(": ").
				Str(entry.Name()).Str(" already exists in the target; resolve manually (moved ").
				Int(int64(moved)).Str(" so far)").String()
			return statusResult("REFUSE", line)
		}
		if err := m.moveEntry(filepath.Join(link, entry.Name()), destination); err != nil {
			return errorResult(filepath.Base(link), fmt.Errorf("move %s: %w", entry.Name(), err))
		}
		moved++
	}
	if err := os.Remove(link); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("remove emptied directory: %w", err))
	}
	if err := os.Symlink(target, link); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("create symlink: %w", err))
	}
	return statusResult("migrated", text.Reset().Str("migrated ").Str(filepath.Base(link)).
		Str(": moved ").Int(int64(moved)).Str(" entries -> ").Str(target).
		Str("; now a symlink").String())
}

func (m *Manager) migrateScratchDirs(link, target string) Result {
	var text textbuf.Buffer
	info, err := os.Lstat(link)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return statusResult("REFUSE", text.Str("REFUSE   ").Str(filepath.Base(link)).
				Str(": exists and is not a directory; resolve manually").String())
		}
		return errorResult(filepath.Base(link), fmt.Errorf("inspect path: %w", err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		line := text.Reset().Str("SKIP     ").Str(filepath.Base(link)).
			Str(": already a symlink; selective migration needs a real directory").String()
		return statusResult("SKIP", line)
	}
	if !info.IsDir() {
		return statusResult("REFUSE", text.Reset().Str("REFUSE   ").Str(filepath.Base(link)).
			Str(": exists and is not a directory; resolve manually").String())
	}
	linkDevice, err := m.fs.device(link)
	if err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("resolve source device: %w", err))
	}
	targetDevice, err := m.fs.device(target)
	if err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("resolve target device: %w", err))
	}
	if linkDevice == targetDevice {
		line := text.Reset().Str("REFUSE   ").Str(filepath.Base(link)).Str(": ").Str(target).
			Str(" is on the same device, so moving there frees nothing. Point TMPDIR at a directory on another drive and retry").
			String()
		return statusResult("REFUSE", line)
	}
	if err := os.MkdirAll(target, targetDirMode); err != nil {
		return errorResult(filepath.Base(link), fmt.Errorf("create target: %w", err))
	}

	moved := make([]string, 0, len(migratableScratch))
	repointed := make([]string, 0, len(migratableScratch))
	skipped := make([]string, 0, len(migratableScratch))
	refused := make([]string, 0, len(migratableScratch))
	for _, name := range migratableScratch {
		entry := filepath.Join(link, name)
		entryInfo, inspectErr := os.Lstat(entry)
		if errors.Is(inspectErr, os.ErrNotExist) {
			continue
		}
		if inspectErr != nil {
			refused = append(refused,
				text.Reset().Str(name).Str(" (inspect failed: ").Err(inspectErr).Byte(')').String())
			continue
		}
		destination := filepath.Join(target, name)
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			m.repointScratchDir(entry, destination, name, &repointed, &skipped, &refused)
			continue
		}
		if !entryInfo.IsDir() {
			skipped = append(skipped, text.Reset().Str(name).Str(" (not a directory)").String())
			continue
		}
		if pathExists(destination) {
			skipped = append(skipped, text.Reset().Str(name).Str(" (already in the target)").String())
			continue
		}
		undeletable, checkErr := m.firstUndeletableDir(entry)
		if checkErr != nil {
			refused = append(refused,
				text.Reset().Str(name).Str(" (scan failed: ").Err(checkErr).Byte(')').String())
			continue
		}
		if undeletable != "" {
			refused = append(refused, filepath.Join(name, undeletable))
			continue
		}
		if err := m.moveEntry(entry, destination); err != nil {
			refused = append(refused,
				text.Reset().Str(name).Str(" (move failed: ").Err(err).Byte(')').String())
			continue
		}
		if err := os.Symlink(destination, entry); err != nil {
			refused = append(refused, text.Reset().Str(name).
				Str(" (moved but symlink failed: ").Err(err).Byte(')').String())
			continue
		}
		moved = append(moved, name)
	}
	return scratchMigrationResult(filepath.Base(link), target, moved, repointed, skipped, refused)
}

func (m *Manager) repointScratchDir(
	entry, destination, name string,
	repointed, skipped, refused *[]string,
) {
	var text textbuf.Buffer
	current, err := filepath.EvalSymlinks(entry)
	if err != nil {
		*skipped = append(*skipped, text.Str(name).Str(" (dangling symlink)").String())
		return
	}
	current, err = filepath.Abs(current)
	if err != nil {
		*refused = append(*refused,
			text.Reset().Str(name).Str(" (resolve failed: ").Err(err).Byte(')').String())
		return
	}
	if filepath.Clean(current) == filepath.Clean(destination) {
		return
	}
	if pathExists(destination) {
		*refused = append(*refused,
			text.Reset().Str(name).Str(" (partial copy at the target; resolve manually)").String())
		return
	}
	undeletable, err := m.firstUndeletableDir(current)
	if err != nil {
		*refused = append(*refused,
			text.Reset().Str(name).Str(" (scan failed: ").Err(err).Byte(')').String())
		return
	}
	if undeletable != "" {
		*refused = append(*refused, filepath.Join(name, undeletable))
		return
	}
	if err := m.moveEntry(current, destination); err != nil {
		*refused = append(*refused,
			text.Reset().Str(name).Str(" (move failed: ").Err(err).Byte(')').String())
		return
	}
	if err := os.Remove(entry); err != nil {
		*refused = append(*refused, text.Reset().Str(name).
			Str(" (remove old symlink: ").Err(err).Byte(')').String())
		return
	}
	if err := os.Symlink(destination, entry); err != nil {
		*refused = append(*refused, text.Reset().Str(name).
			Str(" (moved but symlink failed: ").Err(err).Byte(')').String())
		return
	}
	*repointed = append(*repointed, name)
}

func scratchMigrationResult(path, target string, moved, repointed, skipped, refused []string) Result {
	var text textbuf.Buffer
	parts := []string{text.Str("moved ").Int(int64(len(moved))).String()}
	if len(moved) > 0 {
		parts = append(parts, strings.Join(moved, " "))
	}
	if len(repointed) > 0 {
		parts = append(parts, text.Reset().Str("repointed: ").Join(repointed, " ").String())
	}
	if len(skipped) > 0 {
		parts = append(parts, text.Reset().Str("skipped: ").Join(skipped, ", ").String())
	}
	if len(refused) > 0 {
		parts = append(parts, text.Reset().
			Str("REFUSED (not writable and searchable, so their contents cannot be unlinked; a Go module cache is the usual cause, `go` writes gomodcache directories mode 0555 on purpose): ").
			Join(refused, ", ").String())
	}
	head := "migrated"
	status := "migrated"
	if len(refused) > 0 {
		head = "REFUSE  "
		status = "REFUSE"
	}
	return statusResult(status, text.Reset().Str(head).Byte(' ').Str(path).Str(": ").
		Join(parts, "; ").Str(" -> ").Str(target).String())
}

func (m *Manager) firstUndeletableDir(root string) (string, error) {
	var first string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if err := m.fs.access(path, unix.W_OK|unix.X_OK); err != nil {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			first = relative
			return filepath.SkipAll
		}
		return nil
	})
	return first, err
}

func (m *Manager) ensureSentinel() error {
	tmp := filepath.Join(m.Root, tmpName)
	info, err := os.Lstat(tmp)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect tmp for sentinel: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	gomod := filepath.Join(tmp, "go.mod")
	if pathExists(gomod) {
		return nil
	}
	if err := os.WriteFile(gomod, []byte(Sentinel), sentinelMode); err != nil {
		return fmt.Errorf("write tmp/go.mod sentinel: %w", err)
	}
	return nil
}

func deviceOf(path string) (uint64, error) {
	probe := path
	for {
		info, err := os.Stat(probe)
		if err == nil {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return 0, fmt.Errorf("stat %s has no device id", probe)
			}
			return stat.Dev, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return 0, err
		}
		probe = parent
	}
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	if err == nil {
		return true
	}
	return !errors.Is(err, os.ErrNotExist)
}

func statusResult(status, line string) Result {
	return Result{
		Path: strings.Fields(line)[1], Status: status, Line: line,
		Stderr: diagnosticLine(line),
	}
}

func diagnosticLine(line string) bool {
	if strings.HasPrefix(line, "SKIP") {
		return true
	}
	if strings.HasPrefix(line, "REFUSE") {
		return true
	}
	return strings.HasPrefix(line, "MISMATCH")
}

func errorResult(path string, err error) Result {
	var text textbuf.Buffer
	return Result{Path: path, Status: "REFUSE",
		Line: text.Str("REFUSE   ").Str(path).Str(": ").Err(err).String(), Stderr: true}
}

func failureReport(path string, err error) Report {
	return Report{Results: []Result{errorResult(path, err)}}
}

func verdict(results []Result) int {
	for _, result := range results {
		if result.Status == "REFUSE" {
			return 1
		}
	}
	return 0
}

func undeletableResult(path, undeletable string) Result {
	var text textbuf.Buffer
	line := text.Str("REFUSE   ").Str(path).Str(": ").Str(undeletable).
		Str(" is not writable and searchable, so its contents cannot be unlinked; nothing was moved. A Go module cache is the usual cause: `go` writes gomodcache directories mode 0555 on purpose. Run the download that made it with GOFLAGS=-modcacherw, or remove the cache, then retry").
		String()
	return statusResult("REFUSE", line)
}
