// Design: docs/architecture/core-design.md -- hook policy executes in the native le process
// Detail: bash.go -- command guards
// Detail: writeedit.go -- edit guards
// Detail: agent.go -- delegation guards
// Detail: postwrite.go -- post-edit guards
//
// Package hookruntime implements the Claude hook JSON protocol without an
// interpreter or a shell wrapper.
package hookruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// toolWrite is the Claude tool that replaces a whole file. Several checks judge
// it differently from an Edit, which changes one region.
const toolWrite = "Write"

// specUnassigned is what a session marker holds when the session claims no spec.
const specUnassigned = "unassigned"

const (
	red    = "\033[31m"
	yellow = "\033[33m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	reset  = "\033[0m"
)

// Payload is the stable subset of the Claude hook envelope used by repository
// hooks. ToolInput deliberately remains a map because each tool owns its schema.
type Payload struct {
	SessionID      any            `json:"session_id"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
	TranscriptPath string         `json:"transcript_path"`
	AgentID        string         `json:"agent_id"`
	StopHookActive bool           `json:"stop_hook_active"`
	Prompt         string         `json:"prompt"`
	LastMessage    string         `json:"last_assistant_message"`
	Reason         string         `json:"reason"`
}

type verdict struct {
	code    int
	message string
}

type context struct {
	root       string
	tool       string
	input      map[string]any
	path       string
	content    string
	transcript string
	payload    Payload
}

type hookCheck func(context) *verdict

type hookAction struct {
	tools  []string
	checks []hookCheck
}

// nativeHookActions is the runtime authority for native checks. The rule
// coverage gate reads this Go declaration, then requires every named function
// to carry a `// ze point:` binding and refuses bindings on functions absent
// from this registry.
var nativeHookActions = map[string]hookAction{
	"pretool-bash": {
		tools: []string{"Bash"},
		checks: []hookCheck{
			bashWorktreeCopy, bashDestructiveGit, bashBranchMove, bashRootBuild, bashLossyPipe,
			bashRawHeavy, bashPollLoop, bashSystemTmp, bashScratch,
			bashTestDeletion, bashGovernedWrite,
		},
	},
	"pretool-writeedit": {
		tools: []string{toolWrite, "Edit", "MultiEdit", "NotebookEdit"},
		checks: []hookCheck{
			writeLineCitation, writeGenerated, writeRenderedRule, writePointOverwrite,
			writePointLanguage, writeDesignEvidence, writeSpecStatus, writeGoPatterns,
			writeFilePatterns, writeWeakening, writeCISleep, writeYangDescription,
		},
	},
	"posttool-writeedit": {
		tools: []string{toolWrite, "Edit"},
		checks: []hookCheck{
			postFormatGo, postFileSize, postDeferral, postJournal, postRFCHeader,
			postTestDocs, postFuzz, postVague, postBoundary,
		},
	},
	"pretool-agent-skill": {
		tools:  []string{"Agent", "Task"},
		checks: []hookCheck{agentReviewModel, agentSkill, agentStyleGuide},
	},
}

// Run reads one hook payload and writes the hook protocol response. Hooks fail
// open on malformed input and unexpected internal errors, preserving the hook
// protocol contract.
func Run(kind string, in io.Reader, out, errOut io.Writer) (code int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(errOut, "[%s] internal error (failing open): %v\n", kind, recovered) //nolint:errcheck // hook protocol
			code = 0
		}
	}()

	var payload Payload
	decoder := json.NewDecoder(in)
	if err := decoder.Decode(&payload); err != nil {
		if kind == "session-id" {
			return 2
		}
		return 0
	}
	if payload.ToolInput == nil {
		payload.ToolInput = map[string]any{}
	}
	root := os.Getenv("CLAUDE_PROJECT_DIR")
	if root == "" {
		resolved, err := lepath.Root()
		if err != nil {
			return 0
		}
		root = resolved
	}
	ctx := context{
		root:       root,
		tool:       payload.ToolName,
		input:      payload.ToolInput,
		path:       stringInput(payload.ToolInput, "file_path"),
		content:    standardContent(payload.ToolInput),
		transcript: payload.TranscriptPath,
		payload:    payload,
	}

	var results []verdict
	if action, ok := nativeHookActions[kind]; ok {
		if !oneOf(ctx.tool, action.tools...) {
			return 0
		}
		results = make([]verdict, 0, len(action.checks))
		for _, check := range action.checks {
			if result := check(ctx); result != nil {
				results = append(results, *result)
			}
		}
	} else {
		switch kind {
		case "session-id":
			return runSessionID(ctx, out, errOut)
		default:
			return runLifecycleHook(kind, ctx, out, errOut)
		}
	}

	worst := 0
	messages := make([]string, 0, len(results))
	for _, result := range results {
		if result.message != "" {
			messages = append(messages, result.message)
		}
		if result.code > worst {
			worst = result.code
		}
	}
	if len(messages) != 0 {
		fmt.Fprintln(errOut, strings.Join(messages, "\n")) //nolint:errcheck // hook protocol
	}
	if kind == "pretool-bash" && worst < 2 {
		writeParentSessionPrefix(ctx, out)
	}
	return worst
}

func stringInput(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return value
}

func standardContent(input map[string]any) string {
	if value, ok := input["content"].(string); ok {
		return value
	}
	if value, ok := input["new_string"].(string); ok {
		return value
	}
	edits, _ := input["edits"].([]any)
	var joined textbuf.Buffer
	joined.Reset()
	for _, raw := range edits {
		edit, _ := raw.(map[string]any)
		value, _ := edit["new_string"].(string)
		if joined.Len() != 0 {
			joined.Byte('\n')
		}
		joined.Str(value)
	}
	return joined.String()
}

func oneOf(value string, choices ...string) bool {
	return slices.Contains(choices, value)
}

// anyPrefix reports whether value starts with one of the prefixes. It names
// the membership test so a caller states the invariant positively instead of
// negating a chain of HasPrefix calls.
func anyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

// anyContains reports whether value holds one of the parts. It names the
// membership test for the same reason anyPrefix does.
func anyContains(value string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}

// isAdHocScratch reports whether path is an unowned file directly under the
// checkout tmp directory. Both Bash and edit hooks call this one definition.
func isAdHocScratch(path, root string) bool {
	given := path
	if !filepath.IsAbs(given) {
		given = filepath.Join(root, given)
	}
	tmpRoot, err := filepath.EvalSymlinks(filepath.Join(root, "tmp"))
	if err != nil {
		tmpRoot = filepath.Clean(filepath.Join(root, "tmp"))
	}
	resolved, err := filepath.EvalSymlinks(given)
	if err != nil {
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(given))
		if parentErr == nil {
			resolved = filepath.Join(parent, filepath.Base(given))
		} else {
			resolved = filepath.Clean(given)
		}
	}
	if filepath.Dir(resolved) != tmpRoot {
		return false
	}
	name := strings.TrimPrefix(filepath.Base(given), ".")
	for _, prefix := range []string{"ze-verify", "commit-", "delete-", "mutation", "test-timings"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}
