// Design: docs/architecture/cli/error-surface.md -- a printed command uses the config token codec.

package cli

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
)

// CommandWords is an immutable sequence of decoded words and bare pipe positions.
// Its zero value is an empty sequence. Tokens returns a copy, and transformations
// retain private ownership of the sequence and its ordered, in-bounds positions.
// Callers use values; a nil *CommandWords is not valid.
type CommandWords struct {
	tokens []string
	pipes  []int
}

// Tokens returns decoded words owned by the caller.
func (words CommandWords) Tokens() []string {
	return slices.Clone(words.tokens)
}

// FirstPipe answers the first structural pipe, or -1 when there is none.
func (words CommandWords) FirstPipe() int {
	if len(words.pipes) == 0 {
		return -1
	}
	return words.pipes[0]
}

// Tail removes the first word and retains operator positions in the rest.
func (words CommandWords) Tail() CommandWords {
	if len(words.tokens) == 0 {
		return words
	}
	return words.after(1)
}

// SplitPipe removes the first bare pipe and returns the words on either side.
// Without a pipe, the left side is unchanged and the right side is empty.
func (words CommandWords) SplitPipe() (CommandWords, CommandWords) {
	index := words.FirstPipe()
	if index < 0 {
		return words, CommandWords{}
	}
	return CommandWords{tokens: words.tokens[:index]}, words.after(index + 1)
}

// after is used only with a prefix bounded by Tail or a stored pipe position.
func (words CommandWords) after(count int) CommandWords {
	rest := CommandWords{tokens: words.tokens[count:]}
	for _, index := range words.pipes {
		if index >= count {
			rest.pipes = append(rest.pipes, index-count)
		}
	}
	return rest
}

func (words CommandWords) isPipe(index int) bool {
	return slices.Contains(words.pipes, index)
}

// ParseCommandWords reads complete command input with the same quote and escape
// codec used to print its fix. Incomplete input remains the completer's concern.
func ParseCommandWords(input string) (CommandWords, error) {
	words, quoted, err := config.ParseEditWords(input)
	if err != nil {
		return CommandWords{}, opererr.New("the command has an unfinished quoted value",
			"Each quoted value needs a closing quote; double-quoted values use the configuration file's escapes.",
			"help")
	}
	parsed := CommandWords{tokens: words}
	for i, word := range words {
		barePipe := word == configtext.Pipe && !quoted[i]
		if barePipe {
			parsed.pipes = append(parsed.pipes, i)
		}
	}
	return parsed, nil
}
