// Design: docs/contributing/running-commands.md -- exact shell-safe command replay.
// Related: internal/le/le/root/prompt.go -- the invocation echo that shares this renderer.
package human

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// AppendArguments appends shell-safe words to text, each after one space. It is
// the invocation echo's quoting rule, shared with authorization retry commands.
func AppendArguments(text *textbuf.Buffer, arguments []string) {
	for _, word := range arguments {
		text.Byte(' ')
		if word != "" {
			if !strings.ContainsAny(word, " \t\n'\"\\$`|&;<>()*?[]{}!#~") {
				text.Str(word)
				continue
			}
		}
		text.Byte('\'').Str(strings.ReplaceAll(word, "'", `'\''`)).Byte('\'')
	}
}

// authorizationCommand preserves existing origin context without resolving any
// session directory or minting an identity. An explicit review session is the
// raw harness identity; a commit namespace remains the existing session operand.
func authorizationCommand(root, session, action string, arguments []string) (string, error) {
	if session == "" {
		known, err := lepath.KnownSessionID(root)
		if err != nil {
			return "", err
		}
		session = known
	}
	agent, err := lepath.Agent()
	if err != nil {
		return "", err
	}
	var text textbuf.Buffer
	text.Str("env")
	AppendArguments(&text, []string{"ZE_REPO_ROOT=" + root, "CLAUDE_CODE_SESSION_ID=" + session, "ZE_AGENT_ID=" + agent})
	text.Str(" ./le ").Str(action)
	AppendArguments(&text, arguments)
	return text.String(), nil
}
