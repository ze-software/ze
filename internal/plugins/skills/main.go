// Design: docs/features/ai-first.md — skills command
// Overview: register.go — command registration
//
// `show skills` and `show skill` are local-data commands: each answers a value
// and writes nothing, so every route that calls it decides where the answer
// and the refusal go (docs/architecture/api/commands.md, "Local-data
// commands"). The text an operator reads is the drawing registered for each
// answer's envelope key, identical to what the commands printed before.

package skills

import (
	"bytes"
	"embed"
	"encoding/json"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// flagJSON is the CLI flag that selects structured output on the local route.
const flagJSON = "--json"

// The envelope keys of the two answers, each with a registered drawing.
const (
	keySkills = "skills"
	keySkill  = "skill"
)

//go:embed data/*.md
var skillFS embed.FS

type skill struct {
	Name        string
	Description string
	File        string
	FullFile    string
}

var inventory = []skill{
	{Name: "ze", Description: "Ze network OS overview and agent entry points", File: "data/ze.md", FullFile: "data/ze-full.md"},
	{Name: "ze-diagnostics", Description: "Read Ze diagnostics, explanations, and fix plans", File: "data/ze-diagnostics.md"},
	{Name: "ze-config", Description: "Ze config syntax, validation, and common errors", File: "data/ze-config.md"},
	{Name: "ze-commands", Description: "CLI command reference, dispatch keys, and MCP tools", File: "data/ze-commands.md"},
	{Name: "ze-agent", Description: "Agent workflow for config validation and repair", File: "data/ze-agent.md"},
}

// skillDocument is one bundled skill with its content.
type skillDocument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// listAnswer is the answer of `show skills`. asJSON records the local `--json`
// rendering flag; it is not data, so JSON never carries it.
type listAnswer struct {
	Skills []diagnostic.SkillEntry `json:"skills"`
	asJSON bool
}

// skillAnswer is the answer of `show skill <name>`. asJSON as in listAnswer.
type skillAnswer struct {
	Skill  skillDocument `json:"skill"`
	asJSON bool
}

// showSkills answers `show skills`: every bundled skill.
func showSkills(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if wantsHelp(args) {
		return command.LocalRefusal(0, usage())
	}
	entries := make([]diagnostic.SkillEntry, len(inventory))
	for i, s := range inventory {
		entries[i] = diagnostic.SkillEntry{Name: s.Name, Description: s.Description}
	}
	return command.LocalAnswer{Payload: listAnswer{Skills: entries, asJSON: hasFlag(args, flagJSON)}}
}

// showSkill answers `show skill <name> [--full]`: one bundled skill.
func showSkill(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if wantsHelp(args) {
		return command.LocalRefusal(0, usage())
	}
	full := false
	var name string
	for _, a := range args {
		switch a {
		case flagJSON:
			// A rendering flag, read by hasFlag below: never a skill name.
		case "--full":
			full = true
		default:
			name = a
		}
	}

	if name == "" {
		return command.LocalRefusal(1, "error: missing skill name\navailable: "+skillNames()+"\n")
	}

	var found *skill
	for i := range inventory {
		if inventory[i].Name == name {
			found = &inventory[i]
			break
		}
	}
	if found == nil {
		var tb textbuf.Buffer
		tb.Str("error: unknown skill: ").Str(name).Str("\navailable: ").Str(skillNames()).Byte('\n')
		return command.LocalRefusal(1, tb.String())
	}

	file := found.File
	if full && found.FullFile != "" {
		file = found.FullFile
	}

	content, err := skillFS.ReadFile(file)
	if err != nil {
		var tb textbuf.Buffer
		return command.LocalRefusal(1, tb.Str("error: reading skill: ").Err(err).Byte('\n').String())
	}
	return command.LocalAnswer{Payload: skillAnswer{
		Skill:  skillDocument{Name: found.Name, Description: found.Description, Content: string(content)},
		asJSON: hasFlag(args, flagJSON),
	}}
}

// render prints an answer on the local route. `--json` prints the indented
// JSON the commands always printed; every other answer goes through the
// configured default format, whose text form is the registered drawing.
func render(path string, payload any) int {
	switch answer := payload.(type) {
	case listAnswer:
		if answer.asJSON {
			return printIndented(answer.Skills)
		}
	case skillAnswer:
		if answer.asJSON {
			return printIndented(answer.Skill)
		}
	}
	return command.RenderLocalAnswer(path, payload)
}

// printIndented prints v as two-space indented JSON on stdout.
func printIndented(v any) int {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("error: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI diagnostic
		return 1
	}
	os.Stdout.Write(out.Bytes()) //nolint:errcheck // CLI output
	return 0
}

// drawSkills draws the skill list one `name — description` line per skill.
func drawSkills(raw json.RawMessage) (string, error) {
	var entries []diagnostic.SkillEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	for _, s := range entries {
		b.Str(s.Name).Str(" — ").Str(s.Description).Byte('\n')
	}
	return b.String(), nil
}

// drawSkill draws one skill as its Markdown content, unchanged.
func drawSkill(raw json.RawMessage) (string, error) {
	var document skillDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return "", err
	}
	return document.Content, nil
}

// wantsHelp reports whether the operator asked for the usage page.
func wantsHelp(args []string) bool {
	return hasFlag(args, "-h") || hasFlag(args, "--help")
}

// hasFlag reports whether flag is one of args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func skillNames() string {
	names := make([]string, len(inventory))
	for i, s := range inventory {
		names[i] = s.Name
	}
	return textbuf.Join(names, ", ")
}

// usage answers the help page, colored when stderr, where the local route
// prints it, is a terminal.
func usage() string {
	p := helpfmt.Page{
		Command:   "ze show skills",
		ShortHelp: "List and retrieve version-matched Ze skills for agents",
		Usage:     []string{"ze show skills [--json]", "ze show skill <name> [--full] [--json]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "show skills", Desc: "List all bundled skills"},
				{Name: "show skill <name>", Desc: "Retrieve skill content by name"},
			}},
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: flagJSON, Desc: "Output as structured JSON"},
				{Name: "--full", Desc: "Return expanded content (with examples and patterns)"},
			}},
		},
		Examples: []string{
			"ze show skills",
			"ze show skill ze",
			"ze show skill ze-diagnostics --full",
			"ze show skills --json",
		},
	}
	var help bytes.Buffer
	p.WriteTo(&help, slogutil.UseColor(os.Stderr))
	return help.String()
}
