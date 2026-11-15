// Design: docs/architecture/core-design.md -- conservative cleanup of ended sessions
package session

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/sessiondir"
	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/procscan"
)

const sessionRootRel = "tmp/session"

// RootDir answers tmp/session under root, the directory Reap and Judge walk.
func RootDir(root string) string {
	return filepath.Join(root, filepath.FromSlash(sessionRootRel))
}

// ReapReport is one conservative session cleanup decision.
type ReapReport struct {
	Dry            bool `json:"dry"`
	RemovedDirs    int  `json:"removed-directories"`
	RemovedMarkers int  `json:"removed-markers"`
	// Kept counts the session directories whose owner a proof of life names.
	Kept    int `json:"kept-running"`
	Unowned int `json:"kept-unowned"`
	// Undecided counts the session directories kept because the judgement
	// proved their owner neither alive nor dead, one entry per cause present,
	// in UndecidedCause order. None of them is known to run.
	Undecided []UndecidedCount `json:"kept-undecided,omitempty"`
	// UndecidedClaims counts the spec claim markers kept because their
	// holder's liveness cannot be told (Liveness, LivenessCannotTell).
	UndecidedClaims int      `json:"kept-claims-undecided"`
	Paths           []string `json:"paths,omitempty"`
	Notice          string   `json:"notice,omitempty"`
	MissingRoot     bool     `json:"missing-root"`
}

// UndecidedCause is why the judgement kept a session it proved neither alive
// nor dead. Every cause keeps the session's directory and markers.
type UndecidedCause uint8

const (
	// UndecidedUnspecified is the zero value and never a cause.
	UndecidedUnspecified UndecidedCause = iota
	// UndecidedRecentTree: something in its directory tree was written within
	// IdleAge.
	UndecidedRecentTree
	// UndecidedUnwalkable: its directory tree cannot be walked, so how long
	// it has been idle is unknown.
	UndecidedUnwalkable
	// UndecidedUnreadableClaim: one of its claim markers cannot be read.
	UndecidedUnreadableClaim
	// UndecidedRecentMarker: one of its flat markers was written within
	// IdleAge.
	UndecidedRecentMarker
	// UndecidedUnreadableMarker: one of its flat markers cannot be stat'ed.
	UndecidedUnreadableMarker
)

// undecidedCauses is every cause, in the order a report lists them.
var undecidedCauses = [...]UndecidedCause{
	UndecidedRecentTree, UndecidedUnwalkable, UndecidedUnreadableClaim,
	UndecidedRecentMarker, UndecidedUnreadableMarker,
}

// String names a cause for a human, as the report's summary line prints it.
// Never compare with it.
func (c UndecidedCause) String() string {
	switch c {
	case UndecidedUnspecified:
		return "unspecified"
	case UndecidedRecentTree:
		return "written within the idle age"
	case UndecidedUnwalkable:
		return "cannot be walked"
	case UndecidedUnreadableClaim:
		return "a claim marker cannot be read"
	case UndecidedRecentMarker:
		return "a flat marker written within the idle age"
	case UndecidedUnreadableMarker:
		return "a flat marker cannot be read"
	}
	return "UndecidedCause(" + strconv.Itoa(int(c)) + ")"
}

// MarshalText writes the cause as String names it, so the JSON report reads
// as its text does.
func (c UndecidedCause) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

// UndecidedCount is how many session directories one cause kept.
type UndecidedCount struct {
	Cause UndecidedCause `json:"cause"`
	Count int            `json:"count"`
}

// UndecidedDir is one session directory the judgement kept because it proved
// its owner neither alive nor dead, with the cause and the evidence in words.
type UndecidedDir struct {
	Path    string
	Session string
	Cause   UndecidedCause
	Reason  string
}

// undecidedCounts answers the count of dirs for each cause present, in
// undecidedCauses order.
func undecidedCounts(dirs []UndecidedDir) []UndecidedCount {
	var counts []UndecidedCount
	for _, cause := range undecidedCauses {
		count := 0
		for _, dir := range dirs {
			if dir.Cause == cause {
				count++
			}
		}
		if count > 0 {
			counts = append(counts, UndecidedCount{Cause: cause, Count: count})
		}
	}
	return counts
}

// Text renders the established dry-run paths and summary line. The undecided
// directories are counted apart from the running ones, by cause, never listed
// one by one: a checkout can hold thousands.
func (r ReapReport) Text() string {
	if r.Notice != "" {
		var text textbuf.Buffer
		text.Reset()
		text.Str(r.Notice).Byte('\n')
		if r.UndecidedClaims > 0 {
			text.Str("session-reap: kept ").Int(int64(r.UndecidedClaims)).Byte(' ').
				Str(claimMarkerNoun(r.UndecidedClaims)).Str(" whose holder cannot be judged.\n")
		}
		return text.String()
	}
	if r.MissingRoot {
		return "session-reap: no tmp/session, nothing to do\n"
	}
	var text textbuf.Buffer
	text.Reset()
	if r.Dry {
		for _, path := range r.Paths {
			text.Str(path).Byte('\n')
		}
	}
	verb := "Removed"
	if r.Dry {
		verb = "Would remove"
	}
	directory := "directories"
	if r.RemovedDirs == 1 {
		directory = "directory"
	}
	marker := "markers"
	if r.RemovedMarkers == 1 {
		marker = "marker"
	}
	text.Str("session-reap: ").Str(verb).Byte(' ').Int(int64(r.RemovedDirs)).
		Str(" dead session ").Str(directory).Str(" and ").Int(int64(r.RemovedMarkers)).
		Byte(' ').Str(marker).Str("; kept ").Int(int64(r.Kept)).Str(" running")
	if r.Unowned > 0 {
		text.Str(" and ").Int(int64(r.Unowned)).Str(" with no readable ").Str(sessiondir.IDFile).
			Str(", whose owner is unknown")
	}
	if len(r.Undecided) > 0 {
		total := 0
		for _, count := range r.Undecided {
			total += count.Count
		}
		text.Str("; kept ").Int(int64(total)).Str(" whose owner is neither proven running nor dead (")
		for index, count := range r.Undecided {
			if index > 0 {
				text.Str(", ")
			}
			text.Int(int64(count.Count)).Byte(' ').Str(count.Cause.String())
		}
		text.Byte(')')
	}
	if r.UndecidedClaims > 0 {
		text.Str("; kept ").Int(int64(r.UndecidedClaims)).Byte(' ').
			Str(claimMarkerNoun(r.UndecidedClaims)).Str(" whose holder cannot be judged")
	}
	text.Str(".\n")
	return text.String()
}

func claimMarkerNoun(count int) string {
	if count == 1 {
		return "spec claim marker"
	}
	return "spec claim markers"
}

type reapOps struct {
	processes func() ([]procscan.Process, error)
	removeDir func(string) error
	remove    func(string) error
}

// Reap removes only dated session directories whose identity has no live
// process, live pid pin, current-session ownership, or recent transcript, and
// whose tree and own flat markers were written nothing within IdleAge, with
// those markers, and
// the flat markers of a session that owns no directory once its spec claim
// marker's holder is proven dead (claims.go). With no Claude CLI in the
// process table it removes nothing and says why.
func Reap(root, configDir string, dry bool) (ReapReport, error) {
	scope, err := newReapScope(root, configDir)
	if err != nil {
		return ReapReport{}, err
	}
	if scope.missingRoot {
		return ReapReport{Dry: dry, MissingRoot: true}, nil
	}
	return reap(root, scope.configDir, scope.ownID, dry, reapOps{
		processes: procscan.Scan,
		removeDir: os.RemoveAll,
		remove:    os.Remove,
	})
}

// Judgement is Reap's decision over tmp/session without its removals: every
// dated session directory, and the ones Reap would remove. Notice is set, and
// Dead is empty, when Reap would remove nothing because it cannot tell an idle
// session from a dead one.
//
// Live is the sorted ids of the sessions, among the directory owners and the
// spec claim holders, that the judgement proved alive; it is empty under the
// notice, when nothing is judged. Claims is every spec claim marker with the
// verdict on its holder (claims.go): under the notice each answers
// LivenessCannotTell, never live and never dead.
//
// Undecided is the directories among Dirs kept because their owner was
// proven neither alive nor dead, sorted by path, each with its cause. It is
// empty under the notice, when nothing is judged. A directory in Dirs that is
// in neither Dead nor Undecided, and has a readable owner, is proven alive.
type Judgement struct {
	Dirs        []string
	Dead        []string
	Live        []string
	Undecided   []UndecidedDir
	Claims      []Claim
	Notice      string
	MissingRoot bool
}

// Judge answers the session directories Reap's rules would remove, and
// removes nothing. It takes the process table from the caller, so a caller
// that also judges other stores by liveness scans the processes once
// (internal/le/scratch, the store trim). The rules are Reap's own: judge is
// the one copy both entries call.
//
// A call costs one listing of tmp/session, one owner file read per session
// directory, one read per claim marker, one stat per transcript under the
// projects directory when a Claude CLI runs, and a walk of the whole tree of
// every session directory no proof of life names, which stops at its first
// entry written within IdleAge. An old dead directory is read in full, so the
// cost grows with the file count of the idle directories, not with the
// session count: a caller under a time budget (the session-start path) MUST
// measure it on a full checkout before calling it inside that budget.
func Judge(root, configDir string, processes []procscan.Process) (Judgement, error) {
	scope, err := newReapScope(root, configDir)
	if err != nil {
		return Judgement{}, err
	}
	if scope.missingRoot {
		return Judgement{MissingRoot: true}, nil
	}
	decision, err := judge(root, scope.configDir, scope.ownID, processes)
	if err != nil {
		return Judgement{}, err
	}
	judgement := Judgement{Notice: decision.notice, MissingRoot: decision.missingRoot}
	for _, path := range decision.candidates {
		judgement.Dirs = append(judgement.Dirs, path)
	}
	// An unowned directory is kept, so the walk over kept directories sees it.
	judgement.Dirs = append(judgement.Dirs, decision.unowned...)
	for _, path := range decision.dead {
		judgement.Dead = append(judgement.Dead, path)
	}
	judgement.Undecided = decision.undecidedDirs()
	judgement.Claims = decision.claims
	if decision.notice == "" {
		judgement.Live = liveSubjects(decision)
	}
	slices.Sort(judgement.Dirs)
	slices.Sort(judgement.Dead)
	return judgement, nil
}

// liveSubjects answers the sorted ids of the directory owners and claim
// holders decision proved alive. The live map holds every transcript's
// session, from every project, so it is narrowed to the sessions this
// checkout records.
func liveSubjects(decision reapDecision) []string {
	subjects := map[string]bool{}
	for sid := range decision.candidates {
		subjects[sid] = true
	}
	for _, claim := range decision.claims {
		subjects[claim.Session] = true
	}
	var live []string
	for sid := range subjects {
		if _, proven := decision.live[sid]; proven {
			live = append(live, sid)
		}
	}
	slices.Sort(live)
	return live
}

// reapScope is what both Reap and Judge resolve before judging: this
// process's own session id, which is always live, and the Claude config
// directory whose transcripts keep an idle session alive.
type reapScope struct {
	ownID       string
	configDir   string
	missingRoot bool
}

func newReapScope(root, configDir string) (reapScope, error) {
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	info, err := os.Lstat(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return reapScope{missingRoot: true}, nil
	}
	if err != nil {
		return reapScope{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return reapScope{}, fmt.Errorf("session-reap: refusing unsafe session root %s", sessionRoot)
	}
	// The id alone: judging liveness must not create a session directory.
	ownID, err := lepath.ResolveSessionID(root)
	if err != nil {
		return reapScope{}, err
	}
	if configDir == "" {
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	if configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return reapScope{}, homeErr
		}
		configDir = filepath.Join(home, ".claude")
	}
	return reapScope{ownID: ownID, configDir: configDir}, nil
}

// reapDecision is judge's answer: every candidate directory and the dead ones,
// keyed by session id, the dated directories whose owner could not be read
// (never removed), the pid pins whose process is gone, and the notice that
// stops every removal. live maps each session proven alive to its proof,
// undecided each session no proof names that is neither live nor dead (it
// wrote within IdleAge, its directory cannot be walked, or one of its markers
// cannot be read) to why, holders is every spec claim marker read, claims the
// verdict on each, and orphans the sessions that own no directory and whose
// claim markers are proven dead.
type reapDecision struct {
	candidates  map[string]string
	unowned     []string
	dead        map[string]string
	stalePins   []string
	notice      string
	missingRoot bool
	live        map[string]string
	undecided   map[string]undecidedSession
	holders     []claimHolder
	claims      []Claim
	orphans     map[string]bool
}

// undecidedSession is why judge kept one session it proved neither alive nor
// dead: the cause a report counts, and the evidence in words.
type undecidedSession struct {
	cause  UndecidedCause
	reason string
}

// undecidedDirs answers the session directories judge kept with neither
// proof, sorted by path. It is empty under the notice, which judges nothing.
func (d reapDecision) undecidedDirs() []UndecidedDir {
	var dirs []UndecidedDir
	for sid, path := range d.candidates {
		why, held := d.undecided[sid]
		if !held {
			continue
		}
		dirs = append(dirs, UndecidedDir{Path: path, Session: sid, Cause: why.cause, Reason: why.reason})
	}
	slices.SortFunc(dirs, func(a, b UndecidedDir) int { return cmp.Compare(a.Path, b.Path) })
	return dirs
}

// liveDirs counts the session directories whose owner judge proved alive.
func (d reapDecision) liveDirs() int {
	count := 0
	for sid := range d.candidates {
		if _, proven := d.live[sid]; proven {
			count++
		}
	}
	return count
}

// noCLINotice is the refusal with no Claude CLI in the process table: no
// transcript is read then, so the absence of a proof of life proves nothing.
const noCLINotice = "session-reap: no Claude CLI is in the process table, so no session can be proven dead from here (a QEMU guest, a container, or a host where no session runs). Removed nothing. Run `./le session clean` on the host while a Claude session runs there."

func reap(root, configDir, ownID string, dry bool, ops reapOps) (ReapReport, error) {
	report := ReapReport{Dry: dry}
	processes, err := ops.processes()
	if err != nil {
		return report, err
	}
	decision, err := judge(root, configDir, ownID, processes)
	if err != nil {
		return report, err
	}
	if decision.missingRoot {
		report.MissingRoot = true
		return report, nil
	}
	if decision.notice != "" {
		report.Notice = decision.notice
		report.UndecidedClaims = undecidedClaims(decision.claims)
		return report, nil
	}
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	dead := decision.dead
	// A dead session's markers go whether or not it still owns a directory:
	// a walk over directories alone never sees one that owns none, so its
	// markers leaked.
	owners := make(map[string]bool, len(dead)+len(decision.orphans))
	for sid := range dead {
		owners[sid] = true
	}
	for sid := range decision.orphans {
		owners[sid] = true
	}
	markers := append(flatMarkers(sessionRoot, owners, notProvenDead(decision, owners)), decision.stalePins...)
	report.UndecidedClaims = undecidedClaims(decision.claims)
	paths := make([]string, 0, len(dead)+len(markers))
	for _, path := range dead {
		paths = append(paths, path)
	}
	paths = append(paths, markers...)
	slices.Sort(paths)
	report.Paths = paths
	report.RemovedDirs = len(dead)
	// Only a proven owner counts as running: a directory kept for want of a
	// verdict is counted apart, by cause, because nothing shows it runs.
	report.Kept = decision.liveDirs()
	report.Undecided = undecidedCounts(decision.undecidedDirs())
	report.Unowned = len(decision.unowned)
	if !dry {
		for _, path := range dead {
			_ = ops.removeDir(path)
		}
	}
	for _, marker := range markers {
		if dry {
			report.RemovedMarkers++
			continue
		}
		if err := ops.remove(marker); err == nil {
			report.RemovedMarkers++
		}
	}
	return report, nil
}

// judge applies Reap's liveness rules to tmp/session under root and removes
// nothing.
func judge(root, configDir, ownID string, processes []procscan.Process) (reapDecision, error) {
	var decision reapDecision
	sessionRoot := filepath.Join(root, filepath.FromSlash(sessionRootRel))
	rootInfo, err := os.Lstat(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		decision.missingRoot = true
		return decision, nil
	}
	if err != nil {
		return decision, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return decision, fmt.Errorf("session-reap: refusing unsafe session root %s", sessionRoot)
	}
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		return decision, err
	}
	candidates := make(map[string]string)
	for _, entry := range entries {
		if !sessiondir.Dated(entry.Name()) || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		path := filepath.Join(sessionRoot, entry.Name())
		sid, ok := candidateSID(path)
		if !ok {
			decision.unowned = append(decision.unowned, path)
			continue
		}
		candidates[sid] = path
	}
	decision.candidates = candidates
	holders := claimHolders(sessionRoot, entries)
	// live keeps the first proof found for each session, for the report.
	live := map[string]string{}
	prove := func(sid, proof string) {
		if _, proven := live[sid]; !proven {
			live[sid] = proof
		}
	}
	if ownID != "" {
		prove(ownID, "it is this process's own session")
	}
	pins, stalePins := pinnedSessions(sessionRoot, processes)
	decision.stalePins = stalePins
	for sid := range pins {
		prove(sid, "a pid pin names it and its process runs")
	}
	var argv textbuf.Buffer
	argv.Reset()
	var oldest time.Time
	cliRunning := false
	for _, process := range processes {
		for _, argument := range process.Argv {
			argv.Str(argument).Byte(0)
		}
		argv.Byte('\n')
		if process.CLI {
			cliRunning = true
			if oldest.IsZero() || process.StartedAt.Before(oldest) {
				oldest = process.StartedAt
			}
		}
	}
	argvText := argv.String()
	const argvProof = "a running process's arguments name it"
	for sid := range candidates {
		if strings.Contains(argvText, sid) {
			prove(sid, argvProof)
		}
	}
	// A claim marker's session is judged like a directory's, whether or not it
	// owns one, so its id is looked for in the arguments too.
	for _, holder := range holders {
		if strings.Contains(argvText, holder.session) {
			prove(holder.session, argvProof)
		}
	}
	decision.live = live
	decision.holders = holders
	// Fail closed: with no Claude CLI visible no transcript is read, so the
	// judgement refuses rather than call every unproven session dead.
	if !cliRunning {
		decision.notice = noCLINotice
		decision.claims, _ = judgeClaims(holders, decision)
		return decision, nil
	}
	// The transcripts, in a scope of their own so their names stay local.
	{
		projects := filepath.Join(configDir, "projects")
		info, statErr := os.Stat(projects)
		projectsUsable := statErr == nil && info.IsDir()
		if !projectsUsable {
			decision.notice = fmt.Sprintf("session-reap: a Claude CLI is running but %s does not exist, so an idle session cannot be told from a dead one. Removed nothing.", projects)
			// Every claim answers cannot tell under the notice.
			decision.claims, _ = judgeClaims(holders, decision)
			return decision, nil
		}
		transcripts, globErr := filepath.Glob(filepath.Join(projects, "*", "*.jsonl"))
		if globErr != nil {
			return decision, globErr
		}
		for _, transcript := range transcripts {
			info, statErr := os.Stat(transcript)
			if statErr == nil && !info.ModTime().Before(oldest) {
				prove(strings.TrimSuffix(filepath.Base(transcript), ".jsonl"),
					"its transcript was written since the oldest Claude CLI started")
			}
		}
	}
	now := time.Now()
	decision.undecided = make(map[string]undecidedSession)
	// Fail closed: a claim marker this process cannot read was most likely
	// written by another account on the host, whose transcripts sit under
	// that account's config directory and were not read here, so its whole
	// session stays undecided, directory and markers alike.
	for _, holder := range holders {
		if _, proven := live[holder.session]; proven {
			continue
		}
		if holder.unread != "" {
			decision.undecided[holder.session] = undecidedSession{
				cause: UndecidedUnreadableClaim,
				reason: "its claim marker cannot be read (" + holder.unread +
					"), so it may belong to another account whose transcripts were not read",
			}
		}
	}
	decision.dead = make(map[string]string)
	for sid, path := range candidates {
		if _, proven := live[sid]; proven {
			continue
		}
		if _, held := decision.undecided[sid]; held {
			continue
		}
		// Fail closed: a directory written within IdleAge, or one whose tree
		// cannot be walked, may belong to a session the proof cannot see. The
		// tree is read first, so a session with both a recent tree and a
		// recent marker is reported by its tree.
		idle, idleErr := TreeIdle(path, now, IdleAge)
		if idleErr != nil {
			decision.undecided[sid] = undecidedSession{
				cause:  UndecidedUnwalkable,
				reason: "its directory cannot be walked (" + idleErr.Error() + ")",
			}
			continue
		}
		if !idle {
			decision.undecided[sid] = undecidedSession{
				cause:  UndecidedRecentTree,
				reason: "no proof of life names it, but its directory was written within " + IdleAge.String(),
			}
			continue
		}
		// Fail closed: a session whose own flat marker was written within
		// IdleAge, or cannot be stat'ed, may still run, whether or not it owns
		// a directory, so both rules read the same evidence.
		if why, recent := markersRecent(entries, sid, now); recent {
			decision.undecided[sid] = why
			continue
		}
		decision.dead[sid] = path
	}
	for _, holder := range holders {
		if _, owned := candidates[holder.session]; owned {
			continue
		}
		if _, proven := live[holder.session]; proven {
			continue
		}
		if _, held := decision.undecided[holder.session]; held {
			continue
		}
		if why, recent := markersRecent(entries, holder.session, now); recent {
			decision.undecided[holder.session] = why
		}
	}
	decision.claims, decision.orphans = judgeClaims(holders, decision)
	return decision, nil
}

// markersRecent answers why a session may still run on the evidence of its
// own flat markers, whether or not it owns a directory: one of them was
// written within IdleAge, or one could not be stat'ed. It answers false when
// every marker it owns is older.
func markersRecent(entries []os.DirEntry, sid string, now time.Time) (undecidedSession, bool) {
	for _, entry := range entries {
		if !lepath.MarkerOwnedBy(entry.Name(), sid) {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return undecidedSession{
				cause:  UndecidedUnreadableMarker,
				reason: "its marker " + entry.Name() + " cannot be read (" + err.Error() + ")",
			}, true
		}
		if !idleSince(info.ModTime(), now, IdleAge) {
			return undecidedSession{
				cause: UndecidedRecentMarker,
				reason: "no proof of life names it, but its marker " + entry.Name() +
					" was written within " + IdleAge.String(),
			}, true
		}
	}
	return undecidedSession{}, false
}

// candidateSID reads the owner of a dated session directory from its
// .session-id. The name carries no id, so a directory whose file is missing,
// unreadable or unsafe has no known owner and answers false: liveness cannot
// be judged for it, so it is never reaped.
func candidateSID(dir string) (string, bool) {
	sid, err := sessiondir.Owner(dir)
	if err != nil {
		return "", false
	}
	if !safeReapSID(sid) {
		return "", false
	}
	return sid, true
}

func safeReapSID(sid string) bool {
	if sid == "" || sid == "." || sid == ".." {
		return false
	}
	for _, character := range sid {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func pinnedSessions(root string, processes []procscan.Process) (map[string]bool, []string) {
	starts := make(map[int]string, len(processes))
	for _, process := range processes {
		starts[process.PID] = process.Start
	}
	entries, _ := os.ReadDir(root)
	live := map[string]bool{}
	var stale []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".sid-by-pid-") || strings.HasSuffix(name, ".tmp") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		remainder := strings.TrimPrefix(name, ".sid-by-pid-")
		pidText, start, found := strings.Cut(remainder, "-")
		pid, parseErr := strconv.Atoi(pidText)
		path := filepath.Join(root, name)
		if !found || parseErr != nil || start == "" || starts[pid] != start {
			stale = append(stale, path)
			continue
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // a session state file this walk found under the checkout
		if readErr != nil {
			continue
		}
		sid := strings.TrimSpace(strings.SplitN(string(content), "\n", 2)[0])
		if safeReapSID(sid) {
			live[sid] = true
		}
	}
	slices.Sort(stale)
	return live, stale
}

// notProvenDead answers every session the judgement knows of and did not
// prove dead: the live ones, and the directory owners and claim holders
// outside dead (recent, unreadable, an agent's under a live parent).
func notProvenDead(decision reapDecision, dead map[string]bool) map[string]bool {
	kept := make(map[string]bool, len(decision.live)+len(decision.candidates)+len(decision.holders))
	for sid := range decision.live {
		kept[sid] = true
	}
	for sid := range decision.candidates {
		if !dead[sid] {
			kept[sid] = true
		}
	}
	for _, holder := range decision.holders {
		if !dead[holder.session] {
			kept[holder.session] = true
		}
	}
	return kept
}

// flatMarkers answers the flat markers under root that a dead session owns.
// A marker a session not proven dead also owns is kept: MarkerOwnedBy matches
// a suffix, so a dead id that is the tail of another ("bar" and "foo-bar")
// would otherwise take the other session's markers with it. Fail closed: of
// two owners, the one not proven dead wins.
func flatMarkers(root string, dead, kept map[string]bool) []string {
	entries, _ := os.ReadDir(root)
	var markers []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".sid-by-pid-") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if ownedByAny(entry.Name(), dead) && !ownedByAny(entry.Name(), kept) {
			markers = append(markers, filepath.Join(root, entry.Name()))
		}
	}
	slices.Sort(markers)
	return markers
}

func ownedByAny(name string, sessions map[string]bool) bool {
	for sid := range sessions {
		if lepath.MarkerOwnedBy(name, sid) {
			return true
		}
	}
	return false
}

// undecidedClaims counts the claims whose holder could not be judged, which is
// the count of claim markers the report kept for want of a verdict. Every
// such marker is kept: under the notice reap removes nothing, and otherwise
// flatMarkers keeps every marker a session not proven dead owns, which a
// cannot-tell holder always is (a live parent, or an entry in undecided).
func undecidedClaims(claims []Claim) int {
	count := 0
	for _, claim := range claims {
		if claim.Liveness != LivenessCannotTell {
			continue
		}
		count++
	}
	return count
}

// snapshotKind is the summary block kind the reaper keeps.
const snapshotKind = "snapshot"
