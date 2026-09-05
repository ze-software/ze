// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: session_report.go -- structured ownership and state-path answers

package specsession

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

const (
	defaultWIPCap = 12
	wipCapKey     = "ze.spec.wip.cap"
)

var wipCapEntry = env.MustRegister(env.EnvEntry{
	Key:         wipCapKey,
	Type:        "int",
	Default:     strconv.Itoa(defaultWIPCap),
	Description: "the maximum number of ready specs that can transition to in-progress",
	Private:     true,
})

// specOwner owns this session's marker and its per-spec state path. It is safe
// for concurrent use after construction; callers MUST NOT mutate its fields.
type specOwner struct {
	Root      string
	SessionID string
	Now       func() time.Time
	WIPCap    int
}

// currentSpec returns this session's claimed spec. A missing marker and the
// historical unassigned marker both return an empty claim.
func (o specOwner) currentSpec() (string, error) {
	if err := o.validate(); err != nil {
		return "", err
	}
	body, err := os.ReadFile(o.markerPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	line := string(body)
	if newline := strings.IndexByte(line, '\n'); newline >= 0 {
		line = line[:newline]
	}
	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		return "", nil
	}
	if line == "unassigned" {
		return "", nil
	}
	if !validSpecMarker(line) {
		return "", fmt.Errorf("malformed spec claim in %s: %q", filepath.ToSlash(o.markerPath()), line)
	}
	return line, nil
}

// Claim atomically publishes this session's marker. A ready spec transitions to
// in-progress under the repository claim lock, after the WIP cap is checked.
func (o specOwner) Claim(spec string) (ClaimReport, error) {
	if err := o.validate(); err != nil {
		return ClaimReport{}, err
	}
	spec = filepath.Base(spec)
	if !validSpecMarker(spec) {
		return ClaimReport{}, fmt.Errorf("invalid spec name %q", spec)
	}
	// The marker carries the file NAME, so the bucket is what specpath
	// resolves. Find refuses an absent name and a name two buckets hold, which
	// is what keeps a claim from naming a spec nobody can open.
	relative, err := specpath.Find(o.Root, spec)
	if err != nil {
		return ClaimReport{}, fmt.Errorf("claim %s: %w", spec, err)
	}
	path := filepath.Join(o.Root, filepath.FromSlash(relative))

	var report ClaimReport
	err = o.withLock(func() error {
		body, err := os.ReadFile(path) //nolint:gosec // the path is a spec or session artifact under the checkout root
		if err != nil {
			return err
		}
		rows := specMetadataRows(string(body))
		status := strings.ToLower(specMetadataField(rows, "Status"))
		report = ClaimReport{Spec: spec, Status: status, Cap: o.cap()}
		if status == statusReady {
			wip, err := wip(o.Root, o.cap())
			if err != nil {
				return err
			}
			report.InProgress = len(wip.Specs)
			report.Stalest = append(report.Stalest, wip.Specs[:min(5, len(wip.Specs))]...)
			if report.InProgress >= report.Cap {
				report.Refused = true
				return nil
			}
		}
		if err := writeAtomic(o.markerPath(), []byte(spec+"\n"), 0o600); err != nil {
			return err
		}
		if status != statusReady {
			return nil
		}
		updated, err := transitionReadySpec(string(body), o.now().Format(time.DateOnly))
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := writeAtomic(path, []byte(updated), info.Mode().Perm()); err != nil {
			return err
		}
		report.Transitioned = true
		report.Status = "in-progress"
		report.InProgress++
		return nil
	})
	return report, err
}

// Release removes this session's claim. A missing claim is already released.
func (o specOwner) Release() error {
	if err := o.validate(); err != nil {
		return err
	}
	return o.withLock(func() error {
		err := os.Remove(o.markerPath())
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	})
}

// StateFile resolves and creates this session's state directory. Its filename
// carries the claimed spec stem when this session has one.
func (o specOwner) StateFile() (string, error) {
	if err := o.validate(); err != nil {
		return "", err
	}
	paths, err := lepath.ResolveSession(o.Root, false)
	if err != nil {
		return "", err
	}
	if paths.ID != o.SessionID {
		return "", fmt.Errorf("resolved session %q does not match owner %q", paths.ID, o.SessionID)
	}
	stateDir := filepath.Join(o.Root, filepath.FromSlash(paths.Dir), "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return "", err
	}
	spec, err := o.currentSpec()
	if err != nil {
		return "", err
	}
	name := "session-state-" + o.SessionID + ".md"
	if spec != "" {
		name = "session-state-" + specStemName(spec) + "-" + o.SessionID + ".md"
	}
	return filepath.ToSlash(filepath.Join(paths.Dir, "state", name)), nil
}

// LatestStateForSpec returns the newest compatible state file for stem. It
// reads the current dated and flat locations before the two legacy locations.
func LatestStateForSpec(root, stem string) (string, error) {
	if safeSessionID(stem) == "" {
		return "", fmt.Errorf("invalid spec stem %q", stem)
	}
	patterns := []string{
		filepath.Join(root, "tmp", "session", "????-??-??-*", "state", "session-state-"+stem+"-*.md"),
		filepath.Join(root, "tmp", "session", "session-state-"+stem+"-*.md"),
	}
	latest, err := newestMatch(root, patterns)
	if err != nil {
		return "", err
	}
	if latest != "" {
		return latest, nil
	}
	latest, err = newestMatch(root, []string{filepath.Join(root, ".claude", "session-state-"+stem+"-*.md")})
	if err != nil {
		return "", err
	}
	if latest != "" {
		return latest, nil
	}
	legacy := filepath.Join(root, ".claude", "session-state-"+stem+".md")
	info, err := os.Stat(legacy)
	if err == nil {
		if info.Mode().IsRegular() {
			relative, relErr := filepath.Rel(root, legacy)
			return filepath.ToSlash(relative), relErr
		}
	}
	return "", nil
}

// wip returns every in-progress spec sorted by Updated date and path. The
// population is every release bucket, so the cap counts the work in flight
// rather than the work in flight that happens to sit in plan/.
func wip(root string, cap int) (wipReport, error) {
	paths, err := specpath.All(root)
	if err != nil {
		return wipReport{}, err
	}
	report := wipReport{Cap: cap}
	for _, relative := range paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative))) //nolint:gosec // a spec path specpath matched under the checkout root
		if err != nil {
			return wipReport{}, err
		}
		rows := specMetadataRows(string(body))
		if strings.ToLower(specMetadataField(rows, "Status")) != "in-progress" {
			continue
		}
		report.Specs = append(report.Specs, WIPSpec{Spec: relative, Updated: specMetadataField(rows, "Updated")})
	}
	sort.Slice(report.Specs, func(left, right int) bool {
		if report.Specs[left].Updated != report.Specs[right].Updated {
			return report.Specs[left].Updated < report.Specs[right].Updated
		}
		return report.Specs[left].Spec < report.Specs[right].Spec
	})
	return report, nil
}

// newSpecOwner resolves the canonical session that owns claims under root.
func newSpecOwner(root string) (specOwner, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return specOwner{}, fmt.Errorf("resolve spec owner root %s: %w", root, err)
	}
	paths, err := lepath.ResolveSession(absolute, false)
	if err != nil {
		return specOwner{}, err
	}
	return specOwner{Root: absolute, SessionID: paths.ID}, nil
}

func transitionReadySpec(body, today string) (string, error) {
	lines := strings.Split(body, "\n")
	statusChanged := false
	updatedChanged := false
	inMetadata := false
	for index, line := range lines {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if !inMetadata {
			inMetadata = specMetadataHeaderPattern.MatchString(line)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		field := strings.TrimSpace(cells[1])
		switch field {
		case "Status":
			if strings.TrimSpace(cells[2]) == statusReady {
				lines[index] = "| Status | in-progress |"
				statusChanged = true
			}
		case "Updated":
			lines[index] = "| Updated | " + today + " |"
			updatedChanged = true
		}
	}
	if !statusChanged {
		return "", errors.New("ready spec has no ready Status row in its metadata table")
	}
	if !updatedChanged {
		return "", errors.New("ready spec has no Updated row in its metadata table")
	}
	return strings.Join(lines, "\n"), nil
}

func (o specOwner) markerPath() string {
	return filepath.Join(o.Root, "tmp", "session", ".session-"+o.SessionID)
}

func (o specOwner) cap() int {
	if o.WIPCap > 0 {
		return o.WIPCap
	}
	return configuredWIPCap()
}

func configuredWIPCap() int {
	return env.GetInt(wipCapEntry.Key, defaultWIPCap)
}

func (o specOwner) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o specOwner) validate() error {
	if o.Root == "" {
		return errors.New("spec owner has no checkout root")
	}
	if safeSessionID(o.SessionID) == "" {
		return fmt.Errorf("spec owner has invalid session ID %q", o.SessionID)
	}
	return nil
}

func (o specOwner) withLock(work func() error) error {
	if err := os.MkdirAll(filepath.Join(o.Root, "tmp", "session"), 0o750); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(o.Root, "tmp", "session", ".spec-session.lock"), os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // fixed lock under root
	if err != nil {
		return err
	}
	defer lock.Close() //nolint:errcheck // process exit releases the lock
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck // closing also releases it
	return work()
}

func newestMatch(root string, patterns []string) (string, error) {
	latest := ""
	var latestTime time.Time
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			if latest == "" {
				latest = match
				latestTime = info.ModTime()
				continue
			}
			if info.ModTime().After(latestTime) {
				latest = match
				latestTime = info.ModTime()
			}
		}
	}
	if latest == "" {
		return "", nil
	}
	relative, err := filepath.Rel(root, latest)
	return filepath.ToSlash(relative), err
}

func validSpecMarker(spec string) bool {
	if filepath.Base(spec) != spec {
		return false
	}
	if !strings.HasPrefix(spec, "spec-") {
		return false
	}
	if !strings.HasSuffix(spec, ".md") {
		return false
	}
	return safeSessionID(specStemName(spec)) != ""
}

func specStemName(spec string) string {
	return strings.TrimSuffix(strings.TrimPrefix(spec, "spec-"), ".md")
}

var specMetadataHeaderPattern = regexp.MustCompile(`^\|\s*Field\s*\|\s*Value\s*\|`)

func specMetadataRows(content string) []string {
	var rows []string
	found := false
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if !found {
			found = specMetadataHeaderPattern.MatchString(line)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		rows = append(rows, line)
	}
	return rows
}

func specMetadataField(rows []string, field string) string {
	for _, line := range rows {
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		if strings.TrimSpace(cells[1]) != field {
			continue
		}
		return strings.TrimSpace(cells[2])
	}
	return ""
}
