// Design: docs/contributing/spec-workflow.md -- what Thomas's approval of a design binds
// Related: judge.go -- the judge record, bound to this digest
//
// Related: approve.go -- the approval record the lookup reads first
// Related: carry.go -- the carried record the lookup reads next, and what lets an approval carry over
//
// approval.go computes the design digest, what the judge judged and what
// Thomas approves, and holds the one lookup that answers whether a spec's
// design is approved: the local record whose digest matches, his own or the
// carried one, else a
// Design-approved: line reachable from HEAD with the stem and the digest
// (spec-design-needs-owner-approval, "The records",
// C-8). Claim, the ready hook, the closure checks and the decision-row check
// all ask it, so no reader judges an approval its own way.
package specdesign

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
	"github.com/ze-software/ze/internal/le/gitenv"
)

// digestSections are the four sections the design digest covers, in order.
// Status, Updated, Phase, the checklists and implementation bookkeeping lie
// outside them, so moving a spec through its lifecycle never stales an
// approval; a change of scope, criteria, verdicts or presentation does.
var digestSections = []string{"Task", "Acceptance Criteria", documentsHeading, presentationHeading}

// choicesHeading is the subsection of Design presentation the digest leaves
// out: recording Thomas's answers there must not send the spec back to him.
const choicesHeading = "Choices for Thomas"

// DesignDigest answers the SHA-256, in hex, of the four sections of a spec's
// text: ## Task, ## Acceptance Criteria, ## Design documents, and ## Design
// presentation without its ### Choices for Thomas subsection, each with its
// trailing blanks trimmed. A section the text lacks contributes its name
// alone, so adding it changes the digest.
func DesignDigest(text string) string {
	spec := specFile{text: text, sections: readrecord.Sections(text)}
	hash := sha256.New()
	for _, heading := range digestSections {
		hash.Write([]byte(heading)) //nolint:errcheck // a hash never fails a write
		hash.Write([]byte{0})       //nolint:errcheck // a hash never fails a write
		body, found := spec.digestBody(heading)
		if !found {
			continue
		}
		hash.Write([]byte(body)) //nolint:errcheck // a hash never fails a write
		hash.Write([]byte{0})    //nolint:errcheck // a hash never fails a write
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// digestBody answers the text of the section headed heading as the design
// digest covers it: its trailing blanks trimmed, and ## Design presentation
// without its Choices for Thomas subsection. found false means the spec has
// no such section. The digest and the carry-over comparison (carry.go) both
// read a section through it, so they cannot disagree on what changed.
func (s specFile) digestBody(heading string) (body string, found bool) {
	section, found := s.section(heading)
	if !found {
		return "", false
	}
	body = section.Text
	if heading == presentationHeading {
		body = s.withoutChoices(section)
	}
	return strings.TrimRight(body, " \t\r\n"), true
}

// withoutChoices answers presentation's text with the lines of its Choices
// for Thomas subsection removed.
func (s specFile) withoutChoices(presentation readrecord.Section) string {
	lines := strings.Split(presentation.Text, "\n")
	for _, sub := range s.subsections(presentation) {
		if sub.Heading != choicesHeading {
			continue
		}
		first := sub.Start - presentation.Start
		last := sub.End - presentation.Start
		kept := append([]string{}, lines[:first]...)
		kept = append(kept, lines[last+1:]...)
		return strings.Join(kept, "\n")
	}
	return presentation.Text
}

// shortDigest is the digest as a person reads it: its first twelve digits.
func shortDigest(digest string) string {
	if len(digest) <= 12 {
		return digest
	}
	return digest[:12]
}

// DesignApprovedTrailer is the key of the commit trailer that carries an
// approval to every checkout git reaches. Its line is
// `Design-approved: <stem> <digest>: <date> <owner words>` for Thomas's
// approval, and `Design-approved: <stem> <digest>: away <from>..<until>` for
// an admission while he is away, and `Design-approved: <stem> <digest>:
// carried <approved digest> <date> judge "<sentence>" approved <date> <owner
// words>` for his approval of an earlier text carried to this one (carry.go),
// each with the whole 64-digit digest (FormatApprovalTrailer).
const DesignApprovedTrailer = "Design-approved: "

// awayWord opens the value of an away admission's trailer.
const awayWord = "away "

// carriedWord opens the value of a carried approval's trailer. Thomas's own
// approval opens with its date, so neither word can open his words.
const carriedWord = "carried "

// The words between the cells of a carried approval's trailer value.
const (
	carriedJudgeWord    = "judge "
	carriedApprovedWord = " approved "
)

// Approval is a design approval the lookup found: Thomas's, from his record or
// a trailer, his approval of an earlier text carried to this one
// (CarriedFrom), or an admission while he was away (Away, From, Until).
type Approval struct {
	Stem   string `json:"stem"`
	Digest string `json:"digest"`
	// Date is the day Thomas approved; for an away admission it is From.
	Date  string `json:"date"`
	Words string `json:"owner-words"`
	Away  bool   `json:"away"`
	From  string `json:"from"`
	Until string `json:"until"`
	// Commit is the commit whose trailer carried the approval, or "" for the
	// local record.
	Commit     string      `json:"commit"`
	Departures []Departure `json:"departures"`
	// CarriedFrom is the digest of the text Thomas approved, when his approval
	// was carried to Digest because the later text only writes in decisions he
	// made (carry.go), and "" for an approval of Digest itself. Date and Words
	// stay those of his approval; CarriedOn is the day it was carried, and
	// JudgeSentence what the independent judge said of the later text.
	CarriedFrom   string `json:"carried-from,omitempty"`
	CarriedOn     string `json:"carried-on,omitempty"`
	JudgeSentence string `json:"judge-sentence,omitempty"`
}

// FormatApprovalTrailer answers the trailer line that carries approval, the
// one spelling the lookup reads back.
func FormatApprovalTrailer(approval Approval) string {
	head := DesignApprovedTrailer + approval.Stem + " " + approval.Digest + ": "
	if approval.Away {
		return head + awayWord + approval.From + ".." + approval.Until
	}
	if approval.CarriedFrom != "" {
		return head + carriedWord + approval.CarriedFrom + " " + approval.CarriedOn + " " + carriedJudgeWord +
			strconv.Quote(approval.JudgeSentence) + carriedApprovedWord + approval.Date + " " + approval.Words
	}
	return head + approval.Date + " " + approval.Words
}

// parseCarried reads the value of a carried approval's trailer after its
// opening word, as FormatApprovalTrailer spells it, into approval. An error
// names the cell the value does not spell.
func parseCarried(value string, approval *Approval) error {
	from, rest, found := strings.Cut(value, " ")
	if !found {
		return errors.New("it names no digest it is carried from")
	}
	on, rest, found := strings.Cut(rest, " ")
	if !found {
		return errors.New("it names no day it was carried on")
	}
	rest, found = strings.CutPrefix(rest, carriedJudgeWord)
	if !found {
		return errors.New("it carries no judge sentence")
	}
	quotedSentence, err := strconv.QuotedPrefix(rest)
	if err != nil {
		return fmt.Errorf("its judge sentence is not a quoted string: %w", err)
	}
	sentence, err := strconv.Unquote(quotedSentence)
	if err != nil {
		return fmt.Errorf("its judge sentence is not a quoted string: %w", err)
	}
	rest, found = strings.CutPrefix(rest[len(quotedSentence):], carriedApprovedWord)
	if !found {
		return errors.New("it carries no approval date and owner words")
	}
	date, words, _ := strings.Cut(rest, " ")
	approval.CarriedFrom, approval.CarriedOn, approval.JudgeSentence = from, on, sentence
	approval.Date, approval.Words = date, words
	return nil
}

// Approved is the one lookup: it answers the approval of the design text of
// the spec stem holds, from the local record whose digest matches, else from
// a Design-approved: line reachable from HEAD carrying the stem and the
// digest. found false means no approval of this text exists. An error is a
// record or a history that could not be read (AC-26), never "not approved".
func Approved(root, stem, text string) (approval Approval, found bool, err error) {
	lookup := approvalLookup{root: root}
	return lookup.approved(stem, text)
}

// ReadApproval answers the local approval record of stem whatever text it
// was approved for, so a reader can say the design changed after the date
// Thomas approved it (AC-24). found false means no record exists.
func ReadApproval(root, stem string) (approval Approval, found bool, err error) {
	if !specStem.MatchString(stem) {
		return Approval{}, false, errors.New(stem + errNotASpec)
	}
	record, found, err := readApprovalFile(approvedPath(root, stem))
	if err != nil || !found {
		return Approval{}, found, err
	}
	return record.approval(stem), true, nil
}

// approval converts the local record to the lookup's answer.
func (a approvalRecord) approval(stem string) Approval {
	approval := Approval{Stem: stem, Digest: a.digest, Date: a.date, Words: a.words, Departures: a.departures}
	if a.carriedFrom != "" {
		approval.CarriedFrom, approval.CarriedOn, approval.JudgeSentence = a.carriedFrom, a.carriedOn, a.judgeSentence()
	}
	return approval
}

// judgeSentence answers the sentence of the record's Judge line, which is the
// judge record's digest, date and sentence.
func (a approvalRecord) judgeSentence() string {
	cells := strings.SplitN(a.judge, " ", 3)
	if len(cells) != 3 {
		return a.judge
	}
	return cells[2]
}

// approvalLookup answers approvals for one checkout, reading the history at
// most once however many specs it is asked about. Not safe for concurrent use.
type approvalLookup struct {
	root     string
	trailers []approvalTrailer
	loaded   bool
}

// approvalTrailer is one Design-approved: line of a commit reachable from HEAD.
type approvalTrailer struct {
	commit string
	stem   string
	digest string
	// value is what follows `<stem> <digest>: `.
	value string
}

// approved answers the approval of text for stem (Approved).
func (l *approvalLookup) approved(stem, text string) (Approval, bool, error) {
	if !specStem.MatchString(stem) {
		return Approval{}, false, errors.New(stem + errNotASpec)
	}
	digest := DesignDigest(text)
	record, found, err := localApproval(l.root, stem, digest)
	if err != nil {
		return Approval{}, false, err
	}
	if found {
		return record.approval(stem), true, nil
	}
	trailers, err := l.load()
	if err != nil {
		return Approval{}, false, err
	}
	for _, trailer := range trailers {
		if trailer.stem != stem || trailer.digest != digest {
			continue
		}
		return l.trailerApproval(trailer)
	}
	return Approval{}, false, nil
}

// trailerApproval converts a trailer to the lookup's answer, with the
// departures of the spec text the trailer's commit carries.
func (l *approvalLookup) trailerApproval(trailer approvalTrailer) (Approval, bool, error) {
	approval := Approval{Stem: trailer.stem, Digest: trailer.digest, Commit: trailer.commit}
	if rest, away := strings.CutPrefix(trailer.value, awayWord); away {
		from, until, ok := strings.Cut(rest, "..")
		if !ok {
			return Approval{}, false, errors.New("commit " + trailer.commit + " carries a Design-approved: away line with no from..until; " +
				"fix: have Thomas approve " + trailer.stem + ".md (./le spec design approve)")
		}
		approval.Away, approval.From, approval.Until, approval.Date = true, from, until, from
		return approval, true, nil
	}
	if rest, carried := strings.CutPrefix(trailer.value, carriedWord); carried {
		if err := parseCarried(rest, &approval); err != nil {
			return Approval{}, false, fmt.Errorf("commit %s carries a Design-approved: carried line that cannot be read: %w; "+
				"fix: have Thomas approve %s.md (./le spec design approve)", trailer.commit, err, trailer.stem)
		}
	} else {
		date, words, _ := strings.Cut(trailer.value, " ")
		approval.Date, approval.Words = date, words
	}
	departures, err := l.departuresAt(trailer.commit, trailer.stem)
	if err != nil {
		return Approval{}, false, err
	}
	approval.Departures = departures
	return approval, true, nil
}

// load reads every Design-approved: line of the commits reachable from HEAD,
// newest first. A repository with no commit yet holds none.
func (l *approvalLookup) load() ([]approvalTrailer, error) {
	if l.loaded {
		return l.trailers, nil
	}
	hasHead, err := l.hasHead()
	if err != nil {
		return nil, err
	}
	if hasHead {
		out, err := l.git("log", "-E", "--grep=^"+DesignApprovedTrailer, "--format=%H%x1f%B%x1e", "HEAD")
		if err != nil {
			return nil, err
		}
		l.trailers = parseApprovalTrailers(out)
	}
	l.loaded = true
	return l.trailers, nil
}

// hasHead answers whether the checkout's HEAD names a commit. `git rev-parse
// --verify --quiet` exits 1 and prints nothing for a branch with no commit
// yet; any other failure is the error.
func (l *approvalLookup) hasHead() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitQueryTimeout)
	defer cancel()
	command := l.command(ctx, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 && stderr.Len() == 0 {
		return false, nil
	}
	if err != nil {
		return false, gitError([]string{"rev-parse", "HEAD"}, err, stderr.String())
	}
	return true, nil
}

// parseApprovalTrailers reads the trailer lines of git log's records: each
// record is `<sha>\x1f<body>\x1e`.
func parseApprovalTrailers(out string) []approvalTrailer {
	var trailers []approvalTrailer
	for record := range strings.SplitSeq(out, "\x1e") {
		commit, body, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x1f")
		if !ok {
			continue
		}
		for line := range strings.SplitSeq(body, "\n") {
			rest, isTrailer := strings.CutPrefix(line, DesignApprovedTrailer)
			if !isTrailer {
				continue
			}
			head, value, ok := strings.Cut(rest, ": ")
			if !ok {
				continue
			}
			stem, digest, ok := strings.Cut(head, " ")
			if !ok {
				continue
			}
			trailers = append(trailers, approvalTrailer{commit: commit, stem: stem, digest: digest, value: value})
		}
	}
	return trailers
}

// departuresAt answers the departs rows of the spec stem names, as commit
// carries it: the departures of a trailer-only approval ("The records").
func (l *approvalLookup) departuresAt(commit, stem string) ([]Departure, error) {
	text, err := l.specTextAt(commit, stem)
	if err != nil {
		return nil, err
	}
	return departsRows(specFile{text: text, sections: readrecord.Sections(text)}), nil
}

// specTextAt answers the text of the spec stem names as commit carries it:
// the text a Design-approved: line of that commit approves.
func (l *approvalLookup) specTextAt(commit, stem string) (string, error) {
	names, err := l.git("show", "--name-only", "--format=", commit)
	if err != nil {
		return "", err
	}
	for name := range strings.SplitSeq(names, "\n") {
		if path.Base(name) != stem+".md" {
			continue
		}
		return l.git("show", commit+":"+name)
	}
	return "", errors.New("commit " + commit + " carries a Design-approved: line for " + stem + ".md but not the spec; " +
		"fix: have Thomas approve the spec again (./le spec design approve)")
}

// departsRows answers the departs rows of spec's Design documents table.
func departsRows(spec specFile) []Departure {
	rows, _, _ := spec.documentsTable()
	var departures []Departure
	for _, row := range rows {
		if row.verdict == verdictDeparts {
			departures = append(departures, Departure{Page: row.document, Section: row.section, Statement: row.statement})
		}
	}
	return departures
}

// gitQueryTimeout bounds one git query of the lookup.
const gitQueryTimeout = 30 * time.Second

// command is one read-only git query in the checkout, killed when ctx ends.
func (l *approvalLookup) command(ctx context.Context, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed binary, arguments this file builds
	command.WaitDelay = time.Second
	command.Dir = l.root
	command.Env = gitenv.ReadOnlyQuery(os.Environ())
	return command
}

// git answers the output of one read-only git query, or the error naming it.
func (l *approvalLookup) git(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitQueryTimeout)
	defer cancel()
	command := l.command(ctx, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return "", gitError(args, err, stderr.String())
	}
	return string(out), nil
}

// gitError names a git query that failed, what git said, and the repair.
func gitError(args []string, err error, stderr string) error {
	return fmt.Errorf("git %s failed: %w: %s; fix: repair the checkout's git history (git status, git log) and run the command again",
		strings.Join(args, " "), err, strings.TrimSpace(stderr))
}

// DepartureApproved answers whether Thomas approved a departs row from
// decision row row of page: a local approval record, or a Design-approved:
// approval reachable from HEAD, lists a departure on that page, in its Design
// decisions section, whose statement carries the row's Decision as it stood
// at the parent. It is the lookup doc consistency's decision-row check asks
// (docconsistency.RegisterDepartureLookup). An error is a record or a history
// that could not be read.
func DepartureApproved(root, page string, row docconsistency.DecisionRow) (bool, error) {
	// A carried record lists the departures of the approval it carries, which a
	// carry never changes, so it names a departure as his own record does.
	for _, records := range []struct{ dir, repair string }{{approvedDir, approveRepair}, {carriedDir, carryRepair}} {
		named, err := departureRecorded(root, records.dir, records.repair, page, row)
		if err != nil {
			return false, err
		}
		if named {
			return true, nil
		}
	}
	lookup := approvalLookup{root: root}
	trailers, err := lookup.load()
	if err != nil {
		return false, err
	}
	for _, trailer := range trailers {
		if strings.HasPrefix(trailer.value, awayWord) {
			// An away admission approves no departure: only a spec with no
			// departs row starts while Thomas is away.
			continue
		}
		departures, err := lookup.departuresAt(trailer.commit, trailer.stem)
		if err != nil {
			return false, err
		}
		if departureNamed(departures, page, row) {
			return true, nil
		}
	}
	return false, nil
}

// departureRecorded reports whether a record of dir, the approval records or
// the carried ones, lists a departure from row of page. A directory that does
// not exist holds no record; one that cannot be read is an error.
func departureRecorded(root, dir, repair, page string, row docconsistency.DecisionRow) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", dir, err)
	}
	for _, entry := range entries {
		stem, isRecord := strings.CutSuffix(entry.Name(), ".md")
		if !isRecord {
			continue
		}
		if !specStem.MatchString(stem) {
			continue
		}
		record, found, err := readApprovalRecord(filepath.Join(root, filepath.FromSlash(dir), entry.Name()), repair)
		if err != nil {
			return false, err
		}
		if !found {
			continue
		}
		if departureNamed(record.departures, page, row) {
			return true, nil
		}
	}
	return false, nil
}

// departureNamed reports whether any of departures names row of page.
func departureNamed(departures []Departure, page string, row docconsistency.DecisionRow) bool {
	for _, departure := range departures {
		if departure.names(page, row.Decision) {
			return true
		}
	}
	return false
}
