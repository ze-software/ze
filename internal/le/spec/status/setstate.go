// Design: docs/contributing/spec-workflow.md -- spec sets
// Related: specset.go -- GroupSets, the sets whose state is derived here
// Related: closed.go -- the record of closed specs each set's closed children come from
// Related: ../../session/claims.go -- ListClaims, every claim and the reaper's verdict on its holder

package specstatus

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lesession "github.com/ze-software/ze/internal/le/session"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// SetState is the derived state of one spec set. It is computed from the
// set's files, the record of closed specs and the claims held on its members,
// and never declared: an umbrella's own Status does not decide it.
type SetState uint8

const (
	// SetStateUnspecified is the zero value and never a verdict.
	SetStateUnspecified SetState = iota
	// SetIdea: no child closed, no member is claimed, and no open child is
	// past ready.
	SetIdea
	// SetLive: the set was started, a child is still open, and a live session
	// holds a claim on one of its members.
	SetLive
	// SetStalled: the set was started, a child is still open, and no live
	// session or running agent holds a claim on any member.
	SetStalled
	// SetChildrenClosed: every child closed and only the umbrella is left,
	// which owes its own criteria and its closure.
	SetChildrenClosed
	// SetCannotTell: an input the state needs could not be judged, and
	// SetView.Why says which. A reader MUST NOT read it as live, stalled or
	// idea.
	SetCannotTell
)

// String names a state for a human. Never compare with it.
func (state SetState) String() string {
	switch state {
	case SetStateUnspecified:
		return "unspecified"
	case SetIdea:
		return "idea"
	case SetLive:
		return "live"
	case SetStalled:
		return "stalled"
	case SetChildrenClosed:
		return "children closed"
	case SetCannotTell:
		return "cannot tell"
	}
	return "SetState(" + strconv.Itoa(int(state)) + ")"
}

// MarshalText publishes the state as a kebab-case word, so `| json` carries
// "stalled" and never a number.
func (state SetState) MarshalText() ([]byte, error) {
	return []byte(strings.ReplaceAll(state.String(), " ", "-")), nil
}

// SetClaims is the claims a set state is derived from: every claim marker
// with the reaper's verdict on its holder, or why none could be read. The
// zero value is the claims not read, so a caller that never read them gets
// cannot tell for a started set, never stalled.
type SetClaims struct {
	claims []lesession.Claim
	read   bool
	reason string
	fix    string
}

// ClaimsRead answers the claims ListClaims read.
func ClaimsRead(claims []lesession.Claim) SetClaims {
	return SetClaims{claims: slices.Clone(claims), read: true}
}

// ClaimsUnread answers claims that were not read: why, and what reads them,
// which only the caller knows (a failed read is repaired, a read skipped for
// a time budget is made by another command).
func ClaimsUnread(reason, fix string) SetClaims { return SetClaims{reason: reason, fix: fix} }

// unread answers why the claims were not read and what reads them, or two
// empty strings when they were.
func (claims SetClaims) unread() (reason, fix string) {
	if claims.read {
		return "", ""
	}
	reason, fix = claims.reason, claims.fix
	if reason == "" {
		reason = "the claims on spec files were not read"
	}
	if fix == "" {
		fix = "run `" + setListCommand + "`, which reads every claim and judges its holder"
	}
	return reason, fix
}

// setDoubt is which input a cannot tell verdict could not judge, so a reader
// prints the fix for that input.
type setDoubt uint8

const (
	// The zero value: the state was judged. It is never named, since a judged
	// state carries no doubt to branch on, and the blank keeps iota at zero.
	_ setDoubt = iota
	// doubtRecord: the record of closed specs is not computed.
	doubtRecord
	// doubtClaims: the claims were not read.
	doubtClaims
	// doubtHolder: a holder's liveness cannot be told.
	doubtHolder
	// doubtMarker: a claim marker could not be read.
	doubtMarker
)

// setVerdict is a set's derived state and, for cannot tell, why, which input
// was in doubt and the command that settles it.
type setVerdict struct {
	state SetState
	why   string
	fix   string
	doubt setDoubt
}

// SetReport is the set view: every set of the population with its derived
// state, and the record of closed specs the closed children came from.
type SetReport struct {
	// ClosedCommit is the commit the record of closed specs was computed at,
	// or "" when the record is not known; ClosedUnknown then says why.
	ClosedCommit  string    `json:"closed-commit"`
	ClosedUnknown string    `json:"closed-unknown"`
	Sets          []SetView `json:"sets"`
}

// SetView is one set and its derived state.
type SetView struct {
	Prefix string `json:"prefix"`
	// Umbrella is the umbrella's path, or "" when the set has none.
	Umbrella string   `json:"umbrella"`
	State    SetState `json:"state"`
	// Why says what could not be judged when State is SetCannotTell, and Fix
	// what settles it.
	Why   string `json:"why"`
	Fix   string `json:"fix"`
	doubt setDoubt
	// Started is whether a child closed, a member is claimed, or an open child
	// is past ready.
	Started bool           `json:"started"`
	Open    []SetOpenChild `json:"open"`
	// Closed is the stems of the closed children; ClosedRead says whether the
	// record was known, because an empty list is an answer only then.
	Closed     []string    `json:"closed"`
	ClosedRead bool        `json:"closed-read"`
	Holders    []SetHolder `json:"holders"`
	// Exactly one of Next, WaitsOn and NoNext is set. Next is the file name of
	// the lowest-numbered open child no live claim runs whose dependencies are
	// met. WaitsOn is the path of the open spec outside the set that the open
	// children wait on when none can run. NoNext says why no child can run
	// when neither applies.
	Next    string `json:"next"`
	WaitsOn string `json:"waits-on"`
	NoNext  string `json:"no-next"`
	// Handovers is every tracked file under plan/handover/ whose name carries
	// the set's prefix.
	Handovers []string `json:"handovers"`
}

// SetOpenChild is one open child of a set, as the set view prints it.
type SetOpenChild struct {
	Number  int    `json:"number"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Depends string `json:"depends"`
}

// SetHolder is one claim on a member of a set and the reaper's verdict on its
// holder.
type SetHolder struct {
	Spec     string `json:"spec"`
	Session  string `json:"session"`
	Agent    string `json:"agent"`
	Liveness string `json:"liveness"`
	Reason   string `json:"reason"`
	liveness lesession.Liveness
}

// startedStatuses are the Status values only work on a spec produces: a claim
// moves a ready spec to in-progress, and the statuses after it follow from
// that work.
var startedStatuses = []string{statusInProgress, statusVerification, statusDone, statusBlocked}

// handoverDir is where a committed handover lives (ai/META-DECISIONS.md, "A
// handover lives in the repository"), and handoverReadme the file that keeps
// the directory in the tree, which is no handover.
const (
	handoverDir    = "plan/handover/"
	handoverReadme = handoverDir + "README.md"
)

// recordFix is the fix of a cannot tell whose record is not computed.
const recordFix = "run `" + setListCommand + "`, which computes the record of closed specs"

// setListCommand is the command that computes the record of closed specs and
// prints the whole set view.
const setListCommand = "./le spec status set list"

// DeriveSets answers the state of every set of a population. The sets come
// from GroupSets, so membership is the files' and the record only adds closed
// children. claims are every claim marker with its holder's verdict, and
// handovers the tracked files under plan/handover/.
func DeriveSets(specs []Spec, closed ClosedSpecs, claims SetClaims, handovers []string) SetReport {
	population := make(map[string]string, len(specs))
	for index := range specs {
		population[specs[index].Name] = specs[index].Path
	}
	sets := GroupSets(specs, closed)
	report := SetReport{ClosedCommit: closed.Commit(), ClosedUnknown: closed.Unknown(), Sets: make([]SetView, 0, len(sets))}
	for _, set := range sets {
		report.Sets = append(report.Sets, deriveSet(set, closed, population, claims, handovers))
	}
	return report
}

// deriveSet answers one set's view.
func deriveSet(set SpecSet, closed ClosedSpecs, population map[string]string, claims SetClaims, handovers []string) SetView {
	view := SetView{Prefix: set.Prefix, Umbrella: set.Umbrella, Handovers: setHandovers(set.Prefix, handovers)}
	for _, child := range set.Open {
		view.Open = append(view.Open, SetOpenChild(child))
	}
	closedChildren, read := set.ClosedChildren()
	view.ClosedRead = read
	for _, child := range closedChildren {
		view.Closed = append(view.Closed, child.Stem)
	}
	var unreadMarker string
	view.Holders, unreadMarker = setHolders(set, claims)
	view.Started = setStarted(set, view)
	verdict := setState(view, closed, claims, unreadMarker)
	view.State, view.Why, view.Fix, view.doubt = verdict.state, verdict.why, verdict.fix, verdict.doubt
	view.Next, view.WaitsOn, view.NoNext = setNext(set, population, view.Holders)
	return view
}

// setHolders answers the claims on the set's umbrella and open children, and
// the reason of the first claim marker that could not be read: such a marker
// may hold this set, so it keeps a started set from reading as stalled.
func setHolders(set SpecSet, claims SetClaims) ([]SetHolder, string) {
	members := map[string]bool{}
	if set.HasUmbrella() {
		members[path.Base(set.Umbrella)] = true
	}
	for _, child := range set.Open {
		members[path.Base(child.Path)] = true
	}
	var holders []SetHolder
	unreadMarker := ""
	for _, claim := range claims.claims {
		if claim.Spec == "" {
			if unreadMarker == "" {
				unreadMarker = "the claim marker " + claim.Marker + " could not be read, so it may hold this set: " + claim.Unread
			}
			continue
		}
		name := path.Base(claim.Spec)
		if !members[name] {
			continue
		}
		holders = append(holders, SetHolder{
			Spec: name, Session: claim.Session, Agent: claim.Agent,
			Liveness: claim.Liveness.String(), Reason: claim.Reason, liveness: claim.Liveness,
		})
	}
	return holders, unreadMarker
}

// setStarted reports whether anything shows the set was worked on: a closed
// child, a claim on a member, or an open child whose Status only work on it
// produces.
func setStarted(set SpecSet, view SetView) bool {
	if len(view.Closed) != 0 {
		return true
	}
	if len(view.Holders) != 0 {
		return true
	}
	for _, child := range set.Open {
		if slices.Contains(startedStatuses, child.Status) {
			return true
		}
	}
	return false
}

// setState answers the set's state and, for cannot tell, why. Every doubt
// answers cannot tell: a stalled verdict offers the set to another session,
// and an idea verdict hides a set somebody started.
func setState(view SetView, closed ClosedSpecs, claims SetClaims, unreadMarker string) setVerdict {
	if len(view.Open) == 0 {
		if !view.ClosedRead {
			return setVerdict{SetCannotTell, "whether its children closed cannot be told: the record of closed specs is " + closed.Unknown(), recordFix, doubtRecord}
		}
		if len(view.Closed) == 0 {
			return setVerdict{state: SetIdea}
		}
		return setVerdict{state: SetChildrenClosed}
	}
	// With the claims not read, a set started only by a claim reads as not
	// started here, so a set whose open children are all ready or earlier is
	// an idea. That holds only while a claim always moves its spec to
	// in-progress (startedStatuses): a claim that left a spec ready would make
	// such a set an idea on every reader that did not read the claims.
	if !view.Started {
		if !view.ClosedRead {
			return setVerdict{SetCannotTell, "whether a child closed cannot be told: the record of closed specs is " + closed.Unknown(), recordFix, doubtRecord}
		}
		return setVerdict{state: SetIdea}
	}
	if reason, fix := claims.unread(); reason != "" {
		return setVerdict{SetCannotTell, "whether anyone holds it cannot be told: " + reason, fix, doubtClaims}
	}
	undecided := ""
	for _, holder := range view.Holders {
		switch holder.liveness {
		case lesession.LivenessLive:
			return setVerdict{state: SetLive}
		case lesession.LivenessDead:
		case lesession.LivenessCannotTell, lesession.LivenessUnspecified:
			if undecided == "" {
				undecided = "whether the claim on " + holder.Spec + " is live cannot be told: " + holder.Reason
			}
		default:
			panic("BUG: a claim carries an invalid liveness verdict")
		}
	}
	if undecided != "" {
		return setVerdict{SetCannotTell, undecided,
			"wait for the holder to end or to write again, which the reaper reads as live or dead; `" + setListCommand + "` names the holder and its evidence", doubtHolder}
	}
	if unreadMarker != "" {
		return setVerdict{SetCannotTell, unreadMarker,
			"read the marker; once its session is gone, `./le session clean` removes it", doubtMarker}
	}
	return setVerdict{state: SetStalled}
}

// setNext answers the child to run next, or the spec outside the set the open
// children wait on, or why no child can run. A child a live claim runs is not
// next: it is already being run.
func setNext(set SpecSet, population map[string]string, holders []SetHolder) (next, waitsOn, noNext string) {
	if len(set.Open) == 0 {
		return "", "", "no child of the set is open"
	}
	running := map[string]bool{}
	for _, holder := range holders {
		if holder.liveness == lesession.LivenessLive {
			running[holder.Spec] = true
		}
	}
	members := map[string]bool{}
	for _, child := range set.Open {
		members[specpath.Stem(child.Path)] = true
	}
	umbrella := ""
	if set.HasUmbrella() {
		umbrella = specpath.Stem(set.Umbrella)
	}
	candidates := 0
	for _, child := range set.Open {
		if running[path.Base(child.Path)] {
			continue
		}
		candidates++
		unmet := unmetDepends(child.Depends, specpath.Stem(child.Path), umbrella, population)
		if len(unmet) == 0 {
			return path.Base(child.Path), "", ""
		}
		if waitsOn != "" {
			continue
		}
		for _, stem := range unmet {
			if !members[stem] {
				waitsOn = population[stem]
				break
			}
		}
	}
	if waitsOn != "" {
		return "", waitsOn, ""
	}
	if candidates == 0 {
		return "", "", "every open child is held by a live claim"
	}
	return "", "", "every open child no live claim runs waits on another open child of the set"
}

// dependsName matches a spec named in a Depends cell, with or without its
// directory and its .md suffix.
var dependsName = regexp.MustCompile(`spec-[a-zA-Z0-9][a-zA-Z0-9._-]*`)

// unmetDepends answers the stems of the open specs a Depends cell names. A
// spec named there that the population no longer holds closed, so it is met;
// so is the child itself, and the set's own umbrella, which waits on its
// children and never the reverse. A name glued to a longer word
// (`myspec-x`) is not a reference.
func unmetDepends(depends, self, umbrella string, population map[string]string) []string {
	var unmet []string
	for _, match := range dependsName.FindAllStringIndex(depends, -1) {
		if match[0] > 0 {
			if dependsWordByte(depends[match[0]-1]) {
				continue
			}
		}
		stem := specpath.Stem(strings.TrimRight(depends[match[0]:match[1]], "."))
		if stem == self {
			continue
		}
		if stem == umbrella {
			continue
		}
		if _, open := population[stem]; !open {
			continue
		}
		if !slices.Contains(unmet, stem) {
			unmet = append(unmet, stem)
		}
	}
	return unmet
}

// dependsWordByte reports whether value continues a word, so a spec name
// right after it is part of that word.
func dependsWordByte(value byte) bool {
	for _, span := range [][2]byte{{'a', 'z'}, {'A', 'Z'}, {'0', '9'}} {
		if value < span[0] {
			continue
		}
		if value <= span[1] {
			return true
		}
	}
	switch value {
	case '-', '_':
		return true
	}
	return false
}

// setHandovers answers the handovers whose file name holds the set's prefix
// as whole tokens: `2026-11-14-agent-economy.md` for set `agent-economy`. A
// one-token prefix ties too: a set prefix names a family of numbered specs,
// so `42-container-support.md` belongs to set `container`.
func setHandovers(prefix string, handovers []string) []string {
	var found []string
	for _, handover := range handovers {
		if handoverCarries(handover, prefix) {
			found = append(found, handover)
		}
	}
	return found
}

// handoverCarries reports whether the handover's file name holds name's
// hyphen tokens as one contiguous run, so `2026-11-14-agent-economy.md`
// carries `agent-economy` and does not carry `agent-econ`.
func handoverCarries(handover, name string) bool {
	return contiguousTokens(strings.Split(name, "-"), strings.Split(strings.TrimSuffix(path.Base(handover), ".md"), "-"))
}

// HandoverNamesSpec reports whether a handover is tied to the spec whose stem
// is stem, outside any set: the handover's file name holds the whole stem as
// contiguous tokens, and the stem has two tokens or more. A one-token stem
// (`vrf`, `cgnat`) names a topic rather than one piece of work, so
// `2026-11-20-vrf-leak.md` is no handover of spec `vrf`.
func HandoverNamesSpec(handover, stem string) bool {
	if !strings.Contains(stem, "-") {
		return false
	}
	return handoverCarries(handover, stem)
}

// IsHandover reports whether a root-relative path names a handover: a
// markdown file directly under plan/handover/, other than its README.
func IsHandover(file string) bool {
	if file == handoverReadme {
		return false
	}
	if path.Dir(file)+"/" != handoverDir {
		return false
	}
	return strings.HasSuffix(file, ".md")
}

// trackedHandovers answers the tracked files under plan/handover/, sorted,
// from the git index: a file read, never a git process.
func trackedHandovers(root string) ([]string, error) {
	tracked, err := trackedPaths(root)
	if err != nil {
		return nil, fmt.Errorf("read the tracked handovers: %w", err)
	}
	var handovers []string
	for file := range tracked {
		if IsHandover(file) {
			handovers = append(handovers, file)
		}
	}
	slices.Sort(handovers)
	return handovers, nil
}

// readPopulation reads every spec's metadata with no git: the set state needs
// Status and Depends, never the dates Collect forks git for.
func readPopulation(root string, warn func(string)) ([]Spec, error) {
	paths, err := specpath.All(root)
	if err != nil {
		return nil, err
	}
	specs := make([]Spec, 0, len(paths))
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a spec path specpath matched under the repository root
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		spec, err := Parse(data, rel, warn)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", rel, err)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// setList is the set view's body: it extends the record of closed specs to
// HEAD, reads the population, the tracked handovers and the claims, and
// derives every set's state. A record that cannot be extended is said in
// ClosedUnknown, never read as no closed child; claims that cannot be read
// make every started set cannot tell.
func setList(ctx context.Context, root string, listClaims func(string) ([]lesession.Claim, error), warn func(string)) (SetReport, error) {
	closed, err := LoadClosedSpecs(ctx, root)
	if err != nil {
		closed = ClosedSpecsNotComputed(err.Error())
	}
	specs, err := readPopulation(root, warn)
	if err != nil {
		return SetReport{}, err
	}
	handovers, err := trackedHandovers(root)
	if err != nil {
		return SetReport{}, err
	}
	var claims SetClaims
	listed, err := listClaims(root)
	if err != nil {
		claims = ClaimsUnread("the claims could not be read: "+err.Error(),
			"repair what the error names, then run `"+setListCommand+"` again")
	} else {
		claims = ClaimsRead(listed)
	}
	return DeriveSets(specs, closed, claims, handovers), nil
}

// SessionStartSets answers the session-start summary's lines about spec sets:
// each stalled set with its next child and the command that claims it, each
// set whose children all closed, and each started set whose state cannot be
// told, with what settles it. It reads the shared record of closed specs as it
// stands and runs no git process: the record is extended by `./le spec status
// set list`, which the line names when the record is not computed. claims are
// the caller's, so the caller decides whether ListClaims fits its time budget;
// claims it did not read name every started set with an open child on one
// line beside the command that reads them.
func SessionStartSets(root string, claims SetClaims) ([]string, error) {
	specs, err := readPopulation(root, func(string) {})
	if err != nil {
		return nil, err
	}
	handovers, err := trackedHandovers(root)
	if err != nil {
		return nil, err
	}
	return DeriveSets(specs, ReadClosedSpecs(root), claims, handovers).sessionStartLines(), nil
}

// sessionStartLines renders the report for the session-start summary. A live
// set and an idea print nothing there: `./le spec status set list` prints
// them.
func (report SetReport) sessionStartLines() []string {
	var lines []string
	var unjudged []string
	unjudgedWhy, unjudgedFix := "", ""
	for index := range report.Sets {
		view := &report.Sets[index]
		switch view.State {
		case SetStalled:
			lines = append(lines, stalledLine(view))
		case SetChildrenClosed:
			lines = append(lines, "  Spec set "+view.Prefix+": every child closed, so its umbrella "+view.Umbrella+
				" owes its own acceptance criteria and its closure")
		case SetCannotTell:
			if !view.Started {
				continue
			}
			if view.doubt == doubtClaims {
				// One cause and one fix for every such set: one line.
				unjudged = append(unjudged, view.Prefix)
				unjudgedWhy, unjudgedFix = view.Why, view.Fix
				continue
			}
			lines = append(lines, "  Spec set "+view.Prefix+" was started, and its state cannot be told: "+view.Why+"; "+view.Fix)
		case SetIdea, SetLive:
		case SetStateUnspecified:
			panic("BUG: a derived spec set carries no state")
		default:
			panic("BUG: a derived spec set carries an invalid state")
		}
	}
	if len(unjudged) != 0 {
		lines = append(lines, "  Spec sets started with an open child, not judged live or stalled here: "+strings.Join(unjudged, ", ")+
			"; "+unjudgedWhy+"; "+unjudgedFix)
	}
	var header []string
	if report.ClosedUnknown != "" {
		header = append(header, "Spec sets: the record of closed specs is "+report.ClosedUnknown+
			"; run `"+setListCommand+"` to compute it")
	}
	if len(lines) == 0 {
		return header
	}
	if report.ClosedUnknown == "" {
		header = append(header, "Spec sets, with closed children as recorded at commit "+shortCommit(report.ClosedCommit)+":")
	}
	return append(header, lines...)
}

// stalledLine is the session-start line of one stalled set.
func stalledLine(view *SetView) string {
	var tb textbuf.Buffer
	tb.Str("  Stalled spec set ").Str(view.Prefix).Str(": no live session or running agent holds it. ")
	switch {
	case view.Next != "":
		tb.Str("Next: ").Str(view.Next).Str(", claim it with `./le spec claim ").Str(view.Next).Str("`")
	case view.WaitsOn != "":
		tb.Str("Every open child waits on ").Str(view.WaitsOn).Str(", outside the set")
	default:
		tb.Str("No child can run: ").Str(view.NoNext)
	}
	if len(view.Handovers) != 0 {
		tb.Str("; handover ").Str(strings.Join(view.Handovers, ", "))
	}
	return tb.String()
}

// shortCommit answers the first twelve characters of a commit id.
func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// Text renders the set view `./le spec status set list` prints.
func (report SetReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("Spec sets: ").Int(int64(len(report.Sets))).Byte('\n')
	if report.ClosedUnknown == "" {
		tb.Str("Closed children: as recorded at commit ").Str(report.ClosedCommit).Byte('\n')
	} else {
		tb.Str("Closed children: the record of closed specs is ").Str(report.ClosedUnknown).Byte('\n')
	}
	for index := range report.Sets {
		view := &report.Sets[index]
		tb.Byte('\n').Str(view.Prefix).Str(": ").Str(view.State.String())
		if view.Why != "" {
			tb.Str(" (").Str(view.Why).Str(")")
		}
		tb.Byte('\n')
		if view.Fix != "" {
			tb.Str("  to tell it: ").Str(view.Fix).Byte('\n')
		}
		writeSetBody(&tb, view)
	}
	return tb.String()
}

// writeSetBody writes the lines under one set's heading.
func writeSetBody(tb *textbuf.Buffer, view *SetView) {
	umbrella := view.Umbrella
	if umbrella == "" {
		umbrella = "none"
	}
	tb.Str("  umbrella: ").Str(umbrella).Byte('\n')
	tb.Str("  closed: ")
	switch {
	case !view.ClosedRead:
		tb.Str("not read")
	case len(view.Closed) == 0:
		tb.Str("none")
	default:
		tb.Str(strings.Join(view.Closed, ", "))
	}
	tb.Byte('\n')
	for _, child := range view.Open {
		tb.Str("  open: ").Int(int64(child.Number)).Byte(' ').Str(child.Path).Str(" (").Str(child.Status).Str(")\n")
	}
	for _, holder := range view.Holders {
		tb.Str("  held: ").Str(holder.Spec).Str(" by session ").Str(holder.Session)
		if holder.Agent != "" {
			tb.Str(" agent ").Str(holder.Agent)
		}
		tb.Str(", ").Str(holder.Liveness).Str(": ").Str(holder.Reason).Byte('\n')
	}
	switch {
	case view.Next != "":
		tb.Str("  next: ").Str(view.Next).Str(", claim it with `./le spec claim ").Str(view.Next).Str("`\n")
	case view.WaitsOn != "":
		tb.Str("  waits on: ").Str(view.WaitsOn).Str(", outside the set\n")
	default:
		tb.Str("  next: none, ").Str(view.NoNext).Byte('\n')
	}
	for _, handover := range view.Handovers {
		tb.Str("  handover: ").Str(handover).Byte('\n')
	}
}
