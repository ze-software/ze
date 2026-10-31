// Design: docs/contributing/committing.md -- the gates ./le commit create runs
// Related: prepare.go -- Create, which calls this gate after the verification gates
// Related: review.go -- workingTreeSides and the rename rule this gate shares
// Related: ../spec/mechanisms/specmechanisms.go -- Problem, the judgement this gate applies

package commit

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/hunks"
	specmechanisms "github.com/ze-software/ze/internal/le/spec/mechanisms"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// specMechanismsRefusal refuses a commit that edits a spec claiming a design
// with no Existing Mechanisms table row (specmechanisms.Problem). Only the
// specs in paths are judged, read as the commit carries them, so a spec the
// commit does not carry never refuses it. A carried spec whose text is not an
// edit is not judged either: the text HEAD holds at the same path, or the text
// of a spec the commit removes with its stem replaced (a rename or a move
// between buckets, as renamedSpecs reads one). A spec that predates the rule
// therefore owes the section only in the commit that next changes its text. A
// removed spec carries no design and is judged by nobody.
func specMechanismsRefusal(root string, paths, removed []string, selection hunks.Selection) error {
	sides := workingTreeSides(root, selection)
	var problems []string
	for _, path := range paths {
		if !specpath.IsSpec(path) {
			continue
		}
		text, present, problem := sides.after(path)
		if problem != "" {
			return couldNotRun(errors.New("judge the Existing Mechanisms section of " + path + ": " + problem))
		}
		if !present {
			continue
		}
		unedited, problem := specTextUnedited(path, text, removed, sides)
		if problem != "" {
			return couldNotRun(errors.New("judge the Existing Mechanisms section of " + path + ": " + problem))
		}
		if unedited {
			continue
		}
		if refusal := specmechanisms.Problem(path, text); refusal != "" {
			problems = append(problems, refusal)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "\n\n"))
}

// specTextUnedited reports whether text is a spec's committed text carried
// unchanged: HEAD's text at path, or a removed spec's HEAD text with its stem
// replaced by path's stem.
func specTextUnedited(path, text string, removed []string, sides specSides) (bool, string) {
	before, present, problem := sides.before(path)
	if problem != "" {
		return false, problem
	}
	if present && before == text {
		return true, ""
	}
	for _, source := range removed {
		stem := specpath.Stem(source)
		if stem == "" {
			continue
		}
		old, present, problem := sides.before(source)
		if problem != "" {
			return false, problem
		}
		if !present {
			continue
		}
		if strings.ReplaceAll(old, stem, specpath.Stem(path)) == text {
			return true, ""
		}
	}
	return false, ""
}
