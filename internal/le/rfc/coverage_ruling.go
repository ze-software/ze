// Design: docs/contributing/rfc-conformance-gates.md -- the ratchets, the owner-ruled tag move
// Related: check_ratchets.go -- checkCoverageRatchet, the one caller
// Related: approve.go -- the approval rows this reads
//
// coverage_ruling.go decides whether a polarity lost against HEAD is a tag
// move the owner ruled.
package rfc

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// ownerRulingPattern finds the ruling a reason cites: `owner ruling` and the
// ruling's identity, a date or a number (`owner ruling 2026-09-30`,
// `OWNER RULING 6`), as plan/handover rulings are named.
var ownerRulingPattern = regexp.MustCompile(`(?i)\bowner ruling \(?([0-9A-Za-z][0-9A-Za-z-]*)`)

// ownerRuling answers the identity of the owner ruling text cites, and "" when
// it cites none.
func ownerRuling(text string) string {
	match := ownerRulingPattern.FindStringSubmatch(text)
	if match == nil {
		return ""
	}
	return strings.ToLower(match[1])
}

// ownerRuledMove reports whether req losing polarity is a loss the owner
// ruled. Both halves must cite ONE ruling: the row now carries an annotation
// stating what it keeps (ruledAnnotation), and every unit that held the lost
// polarity at HEAD carries an approval in this commit session. Either half
// alone is something an author can write unaided: the annotation is a summary
// edit, and an approval with no annotation leaves the row claiming a proof
// that is gone.
func ownerRuledMove(req Requirement, polarity string, held map[Cover][]Tag, approvals map[string]string) bool {
	ruling := ruledAnnotation(req.Annotation, polarity)
	if ruling == "" {
		return false
	}
	units := 0
	for cover := range held {
		if cover.RID != req.RID || cover.Polarity != polarity {
			continue
		}
		units++
		if ownerRuling(approvals[approvalUnitOf(cover.Unit)]) != ruling {
			return false
		}
	}
	return units > 0
}

// ruledAnnotation answers the owner ruling annotation cites when it states
// what the row keeps after losing polarity, and "" otherwise. Two kinds state
// it: {single-polarity} keeping the OTHER polarity, a tag move (OWNER RULING
// 6(b)), and {gap}, an absent feature whose row keeps no tag at all, so it may
// lose both polarities (OWNER RULING 8(g)). Every other kind excuses a row
// without saying the proof it held was ruled away, so it answers "".
func ruledAnnotation(annotation *Annotation, polarity string) string {
	if annotation == nil {
		return ""
	}
	if annotation.Kind == AnnotationGap {
		return ownerRuling(annotation.Reason)
	}
	if annotation.Kind != AnnotationSinglePolarity {
		return ""
	}
	if annotation.Polarity == polarity {
		return ""
	}
	return ownerRuling(annotation.Reason)
}

// approvalUnitOf turns a cover's unit key (`<path>::<Function>`, or the bare
// path for a file-scoped tag) into the `<package>.<TestName>` an approval row
// names: the directory's base name, then the function or the file stem. It is
// the form internal/le/commit/rfcchange.go changedRFCUnits gives a changed
// unit, so the row the commit gate wants is the row the ratchet reads.
func approvalUnitOf(unit string) string {
	file, name, _ := strings.Cut(unit, "::")
	if name == "" {
		base := path.Base(file)
		name = strings.TrimSuffix(base, path.Ext(base))
	}
	directory := path.Dir(file)
	if directory == "." {
		return name
	}
	return path.Base(directory) + "." + name
}

// heldPolarities answers the polarities each requirement held, from a cover
// set.
func heldPolarities(held map[Cover][]Tag) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for cover := range held {
		if out[cover.RID] == nil {
			out[cover.RID] = map[string]bool{}
		}
		out[cover.RID][cover.Polarity] = true
	}
	return out
}

// approvalRows reads one commit session's approval file, the one `./le rfc
// approve` writes, as unit to reason. An absent file approves nothing, which is
// the answer for a session that recorded no ruling.
func approvalRows(root, session string) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ApprovalPath(session)))) //nolint:gosec // the path is this session's own tmp/ file under the checkout root
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows := map[string]string{}
	for line := range strings.SplitSeq(string(content), "\n") {
		cells := strings.Split(strings.TrimSpace(line), "|")
		if len(cells) != 4 {
			continue
		}
		unit := strings.TrimSpace(cells[1])
		if !approvalUnitPattern.MatchString(unit) {
			continue
		}
		rows[unit] = strings.TrimSpace(cells[2])
	}
	return rows, nil
}

// sessionApprovals answers the approval rows of the commit session this
// checkout's harness session maps to, for `./le rfc check`. The session
// resolver mints an identity where the harness supplies none, so an error here
// is a checkout that cannot be read, and the check reports it rather than
// judging without the rows.
func sessionApprovals(tree string) (map[string]string, error) {
	session, err := lepath.CommitSession(tree, "")
	if err != nil {
		return nil, err
	}
	return approvalRows(tree, session)
}
