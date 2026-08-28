// Design: plan/spec-le-is-a-ze-binary.md -- native repository refactors after the Go tooling cutover
package yangmigration

import (
	"fmt"
	"strings"
)

// Workflow identifies one of the three legacy YANG refactors.
type Workflow string

const (
	WorkflowCommandsToPlugins Workflow = "commands-to-plugins"
	WorkflowPathRefactor      Workflow = "path-refactor"
	WorkflowSchemaToYang      Workflow = "schema-to-yang"
)

// Move is one exact filesystem relocation. Identical is true when the
// destination already holds the source bytes and apply only removes the source.
type Move struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Identical   bool   `json:"identical,omitempty"`
}

// Edit is one byte-for-byte file rewrite. Before and After make preview a
// structured patch rather than prose that a caller must parse.
type Edit struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// ManualEdit is a syntax location the path refactor deliberately does not
// rewrite automatically.
type ManualEdit struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// Refusal names an unsafe or malformed input. Planning finds every refusal
// before apply performs its first write.
type Refusal struct {
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason"`
}

// Report is the common structured result of all three workflows.
type Report struct {
	Workflow Workflow     `json:"workflow"`
	Apply    bool         `json:"apply"`
	Scanned  int          `json:"scanned,omitempty"`
	Moves    []Move       `json:"moves,omitempty"`
	Edits    []Edit       `json:"edits,omitempty"`
	Removals []string     `json:"removals,omitempty"`
	Manual   []ManualEdit `json:"manual,omitempty"`
	Skipped  []string     `json:"skipped,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
	Refusals []Refusal    `json:"refusals,omitempty"`
}

// Changed reports whether apply would change at least one byte or path.
func (r Report) Changed() bool {
	return len(r.Moves) != 0 || len(r.Edits) != 0 || len(r.Removals) != 0
}

// Refused reports whether preflight rejected the whole operation.
func (r Report) Refused() bool { return len(r.Refusals) != 0 }

// Text is a compact human rendering. JSON and YAML renderers retain the exact
// before/after bytes from the structured report.
func (r Report) Text() string {
	mode := "preview"
	if r.Apply {
		mode = "apply"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%s %s: %d move(s), %d edit(s), %d removal(s), %d manual edit(s)\n", r.Workflow, mode, len(r.Moves), len(r.Edits), len(r.Removals), len(r.Manual))
	for _, refusal := range r.Refusals {
		if refusal.Path == "" {
			fmt.Fprintf(&out, "refused: %s\n", refusal.Reason)
			continue
		}
		fmt.Fprintf(&out, "refused: %s: %s\n", refusal.Path, refusal.Reason)
	}
	return out.String()
}
