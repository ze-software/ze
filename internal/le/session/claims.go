// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: reap.go -- the liveness judgement every verdict here reads
//
// The claim enumerator: every spec claim marker under tmp/session, the
// session and agent that hold it, and whether that holder still runs, as the
// reaper's judgement decides it. It lives beside the reaper because the
// verdict is the reaper's, and in this package because neither
// internal/le/spec nor the session-start hook is imported by it, so both can
// import it without a cycle.
package session

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/procscan"
)

// Liveness is the reaper's answer about the holder of one spec claim.
type Liveness uint8

const (
	// LivenessUnspecified is the zero value and never a verdict: a Claim built
	// without one reads as unjudged, never as live or dead.
	LivenessUnspecified Liveness = iota
	// LivenessLive: the judgement proved the holder's session runs.
	LivenessLive
	// LivenessDead: no proof of life names the holder's session and it wrote
	// nothing for IdleAge. holderLiveness names the false-dead classes left.
	LivenessDead
	// LivenessCannotTell: the judgement could not decide, and Claim.Reason
	// says why. A caller MUST NOT read it as live or as dead.
	LivenessCannotTell
)

// String names a verdict for a human. Never compare with it.
func (l Liveness) String() string {
	switch l {
	case LivenessUnspecified:
		return "unspecified"
	case LivenessLive:
		return "live"
	case LivenessDead:
		return "dead"
	case LivenessCannotTell:
		return "cannot tell"
	}
	return "Liveness(" + strconv.Itoa(int(l)) + ")"
}

// Claim is one spec claim marker and the reaper's verdict on its holder.
type Claim struct {
	// Marker is the marker file's path.
	Marker string
	// Spec is the spec file name the marker holds, as written. It is "" only
	// when the marker could not be read, and Unread then says why.
	Spec   string
	Unread string
	// Session is the harness session the claim was made under. Agent is the
	// agent that made it, or "" for the session's main thread.
	Session string
	Agent   string
	// Liveness is the verdict on the holder, and Reason the evidence behind
	// it in words for a report.
	Liveness Liveness
	Reason   string
}

// ListClaims answers every spec claim under root's tmp/session, sorted by
// marker path, with the reaper's verdict on each holder. It costs one process
// table scan (procscan.Scan: a /proc walk on Linux, one ps fork elsewhere),
// one listing of tmp/session, one owner file read per session directory, one
// read per claim marker, when a Claude CLI runs one stat per transcript under
// the projects directory, and a walk of the whole tree of every session
// directory no proof of life names, which stops at its first entry written
// within IdleAge. An old dead directory is read in full, so the cost grows
// with the file count of the idle directories, not with the claim count: a
// caller under a time budget (the session-start path) MUST measure it on a
// full checkout before calling it inside that budget. A caller that already
// holds the process table calls Judge and reads Judgement.Claims, which costs
// the same without the scan. configDir "" is the Claude config directory Reap
// reads.
func ListClaims(root, configDir string) ([]Claim, error) {
	processes, err := procscan.Scan()
	if err != nil {
		return nil, err
	}
	judgement, err := Judge(root, configDir, processes)
	if err != nil {
		return nil, err
	}
	return judgement.Claims, nil
}

// claimHolder is one claim marker as its name and first line read: the
// session and agent SpecClaimMarker built the name from, and the spec it
// holds ("" for a marker that claims nothing, or one that could not be read,
// when unread says why).
type claimHolder struct {
	marker  string
	session string
	agent   string
	spec    string
	unread  string
}

// claimHolders reads every spec claim marker among the entries of
// sessionRoot. A name SpecClaimMarker never produces is not a claim and is
// left to the other walks. A marker removed between the listing and the read
// was released, and is skipped.
func claimHolders(sessionRoot string, entries []os.DirEntry) []claimHolder {
	var holders []claimHolder
	for _, entry := range entries {
		session, agent, ok := lepath.ParseSpecClaimMarker(entry.Name())
		if !ok {
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(sessionRoot, entry.Name())
		holder := claimHolder{marker: path, session: session, agent: agent}
		spec, err := readClaimMarker(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			holder.unread = err.Error()
		} else {
			holder.spec = spec
		}
		holders = append(holders, holder)
	}
	return holders
}

// readClaimMarker answers the spec a claim marker holds: its first line, and
// "" for an empty marker or the historical "unassigned" one, which claim
// nothing.
func readClaimMarker(path string) (string, error) {
	content, err := os.ReadFile(path) //nolint:gosec // a claim marker under the checkout's tmp/session
	if err != nil {
		return "", err
	}
	spec, _, _ := strings.Cut(string(content), "\n")
	if spec == "unassigned" {
		return "", nil
	}
	return spec, nil
}

// judgeClaims gives each holder the verdict decision proves, and answers the
// claims (every holder whose marker holds a spec or could not be read) and
// the orphans: the sessions that own no dated directory and whose holders
// are proven dead, whose markers the reaper removes.
func judgeClaims(holders []claimHolder, decision reapDecision) ([]Claim, map[string]bool) {
	orphans := map[string]bool{}
	var claims []Claim
	for _, holder := range holders {
		liveness, reason := decision.holderLiveness(holder)
		if liveness == LivenessDead {
			if _, owned := decision.candidates[holder.session]; !owned {
				orphans[holder.session] = true
			}
		}
		if holder.spec == "" && holder.unread == "" {
			continue
		}
		claims = append(claims, Claim{
			Marker:   holder.marker,
			Spec:     holder.spec,
			Unread:   holder.unread,
			Session:  holder.session,
			Agent:    holder.agent,
			Liveness: liveness,
			Reason:   reason,
		})
	}
	slices.SortFunc(claims, func(a, b Claim) int { return cmp.Compare(a.Marker, b.Marker) })
	return claims, orphans
}

// holderLiveness is the verdict on one holder. Every doubt answers cannot
// tell, which keeps the marker: the reaper removes a marker only on a dead
// verdict, and a wrong one destroys another session's claim.
//
// The proof of death is the one judge applies to a session directory: a
// process table that shows a Claude CLI, so the transcripts were read; no
// live proof (this process's own session, a live pid pin, the id in a running
// process's arguments, a transcript written since the oldest Claude CLI
// started); and nothing written for IdleAge, neither in the session's
// directory tree nor in any of its own flat markers. A session with no
// directory gets the same proof without the tree.
//
// Dead is still not certain. Known false-dead classes: a session the proof
// cannot see (an identity taken from the environment, another
// CLAUDE_CONFIG_DIR, another machine sharing the checkout, another harness)
// that has written nothing here for IdleAge while it still runs; and a
// session whose id is the tail of another's is protected from the other's
// removal only when the other is known (flatMarkers).
func (d reapDecision) holderLiveness(holder claimHolder) (Liveness, string) {
	// Fail closed: the judgement refused (no Claude CLI visible, or the
	// projects directory is missing), so no session can be proven idle or
	// dead. Every claim answers cannot tell, never live.
	if d.notice != "" {
		return LivenessCannotTell, d.notice
	}
	proof, live := d.live[holder.session]
	if live {
		if holder.agent == "" {
			return LivenessLive, "session " + holder.session + " is live: " + proof
		}
		// Fail closed: an agent's marker carries no signal of whether the agent
		// still runs, so under a live parent it is neither live nor dead.
		return LivenessCannotTell, "the parent session " + holder.session + " is live (" + proof +
			"), and an agent's claim marker carries no signal of whether agent " + holder.agent + " still runs"
	}
	// Fail closed: the proof cannot see a session that took its identity from
	// the environment, one under another CLAUDE_CONFIG_DIR, or one on another
	// machine sharing the checkout, so a recent write keeps it undecided; so
	// does an unreadable claim marker, which another account most likely
	// wrote (judge fills undecided).
	if why, undecided := d.undecided[holder.session]; undecided {
		return LivenessCannotTell, "session " + holder.session + ": " + why.reason
	}
	if holder.agent == "" {
		return LivenessDead, "session " + holder.session + " is dead: " + deathProof
	}
	return LivenessDead, "the parent session " + holder.session + " of agent " + holder.agent + " is dead: " + deathProof
}

// deathProof states what the judgement found missing for a dead session.
const deathProof = "no pid pin, running process or transcript written since the oldest Claude CLI started names it, and nothing in its directory tree, when it owns one, or in any of its own flat markers was written within the idle age"
