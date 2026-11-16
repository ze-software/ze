// Design: docs/architecture/cli/command-completion.md -- one runtime argument query.

package command

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ArgumentQuery asks the daemon as one authenticated caller. Its owner MUST bind
// the caller's context and authorization generation; it is safe to share only
// within that request or session. Errors are distinct from an empty candidate set.
type ArgumentQuery func(string) (string, error)

// ArgumentSuggestions asks the existing completion RPC, preserving completed
// argument boundaries and an empty partial. This is the shared response decoder
// for attached, SSH and browser completion. query MUST be non-nil.
func ArgumentSuggestions(query ArgumentQuery, name string, completed []string, partial string) ([]Suggestion, error) {
	var line textbuf.Buffer
	line.Str("system command complete args ")
	WriteInvocationValue(&line, name)
	for _, word := range completed {
		line.Byte(' ')
		WriteInvocationValue(&line, word)
	}
	line.Byte(' ')
	WriteInvocationValue(&line, partial)
	queryLine := line.String()
	output, err := query(queryLine)
	if err != nil {
		return nil, err
	}
	var data struct {
		Completions *[]struct {
			Value     string `json:"value"`
			ShortHelp string `json:"short-help"`
		} `json:"completions"`
	}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, opererr.Wrap(err, "argument completion could not read the daemon's answer", "runtime values are unavailable", "system command list")
	}
	if data.Completions == nil {
		return nil, opererr.New("argument completion received no candidate collection", "the daemon's answer cannot establish the available values", "system command list")
	}
	suggestions := make([]Suggestion, 0, len(*data.Completions))
	for _, candidate := range *data.Completions {
		suggestions = append(suggestions, Suggestion{Text: candidate.Value, Description: candidate.ShortHelp, Type: SuggestionValue})
	}
	return suggestions, nil
}
