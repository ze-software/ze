// Design: docs/contributing/running-commands.md -- the ./le launcher and its words
// Related: manifest.go -- the root page that renders the launcher block
// Related: helpai.go -- the `launcher` section of `le help ai`
// Related: complete.go -- completion offers the words at the first place
//
// The `./le` script in the checkout reads some words before le starts, so no
// table of le's own registry can hold them. They are declared here, once, and
// the root page, `le help ai launcher` and completion read this table
// (D-P4-launcher of spec-le-cli-usability). A word the script consumes that is
// also an le area, `rollback`, is declared by that area and not here.
// launcher_test.go reads the words from the script itself and holds the two
// sets equal.

package leroot

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
	"github.com/ze-software/ze/internal/core/helpfmt"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// LauncherWord is one word the ./le script consumes before the binary runs.
type LauncherWord struct {
	// Word is what the reader types first: `--name`, or `--name=` glued to its
	// value.
	Word string
	// Value is the placeholder of the value the word takes.
	Value string
	// Values are the literal values the script itself reads, each with its
	// meaning: `session` is a name the script resolves on its own.
	Values []leaction.Candidate
	// Meaning is one line saying what the word does.
	Meaning string
}

// sessionValue is the value the launcher resolves itself rather than using as
// a build name.
var sessionValue = leaction.Candidate{
	Value:   "session",
	Summary: "name the build after this session, from CLAUDE_CODE_SESSION_ID",
}

// launcherWords is the declaration, in the order the root page lists it.
var launcherWords = []LauncherWord{
	{
		Word: "--name", Value: "<name>", Values: []leaction.Candidate{sessionValue},
		Meaning: "build the working tree as bin/le-<name>/le and run it; session names this session's build",
	},
	{
		Word: "--name=", Value: "<name>", Values: []leaction.Candidate{sessionValue},
		Meaning: "the same as --name <name>, written as one word",
	},
}

// LauncherWords answers a copy of the declaration.
func LauncherWords() []LauncherWord {
	return slices.Clone(launcherWords)
}

// spelling answers how a reader writes the word with its value.
func (w LauncherWord) spelling() string {
	if strings.HasSuffix(w.Word, "=") {
		return w.Word + w.Value
	}
	return w.Word + " " + w.Value
}

// launcherSection is the root page's launcher block.
func launcherSection() helpfmt.HelpSection {
	section := helpfmt.HelpSection{Title: "Launcher", Summary: "words ./le reads before le starts; they come first"}
	for _, word := range launcherWords {
		section.Entries = append(section.Entries, helpfmt.HelpEntry{Name: word.spelling(), Desc: word.Meaning})
	}
	return section
}

// fillLauncher copies the declaration into `le help ai launcher`, each value
// the script reads beside its word.
func fillLauncher(ref *reference.Reference, _ string) {
	for _, word := range launcherWords {
		ref.Launcher = append(ref.Launcher, reference.Entry{Name: word.spelling(), Meaning: word.Meaning})
		for _, value := range word.Values {
			spelled := word.Word + " " + value.Value
			if strings.HasSuffix(word.Word, "=") {
				spelled = word.Word + value.Value
			}
			ref.Launcher = append(ref.Launcher, reference.Entry{Name: spelled, Meaning: value.Summary})
		}
	}
}

// launcherCandidates answers the words completion offers at the first place.
func launcherCandidates() []leaction.Candidate {
	candidates := make([]leaction.Candidate, 0, len(launcherWords))
	for _, word := range launcherWords {
		candidates = append(candidates, leaction.Candidate{Value: word.Word, Summary: word.Meaning})
	}
	return candidates
}

// afterLauncher answers what completion offers once words opens with a
// launcher word: the word's values while its value is being typed, else the
// le line that follows it, completed as if it stood alone.
func afterLauncher(words []string) ([]leaction.Candidate, bool, []string) {
	for _, word := range launcherWords {
		if strings.HasSuffix(word.Word, "=") {
			if !strings.HasPrefix(words[0], word.Word) {
				continue
			}
			if len(words) > 1 {
				return nil, false, words[1:]
			}
			glued := make([]leaction.Candidate, 0, len(word.Values))
			for _, value := range word.Values {
				glued = append(glued, leaction.Candidate{Value: word.Word + value.Value, Summary: value.Summary})
			}
			return glued, true, nil
		}
		// Not this launcher word: try the next one.
		if words[0] != word.Word {
			continue
		}
		// The launcher word alone, nothing typed after it yet: the first-word
		// completion already offered the word itself, so nothing is left to add.
		if len(words) < 2 {
			continue
		}
		if len(words) == 2 {
			return word.Values, true, nil
		}
		return nil, false, words[2:]
	}
	return nil, false, words
}
