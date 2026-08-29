// Design: website/AI.md -- the published command surfaces come from the live catalog
// Detail: commands.go renders the CLI reference; equivalents.go the vendor map; derived.go llms.txt.
// Related: build.go writes data/cli-commands.json before any producer runs.
package site

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// catalogFile is the published command catalog, relative to the artifact.
//
// refreshCommandSurfaces writes it from the live binary before any producer
// runs, so a producer reads the same catalog the site publishes rather than
// building the daemon a second time.
const catalogFile = "data/cli-commands.json"

// catalogCommand is one command of the live catalog.
//
// The shape is spelled here rather than imported from the producer, so a change
// to the published contract shows up as a compile or a decode error in this
// package instead of following the producer in silence. It is the same reason
// internal/le/docvalid spells it a third time for its drift fixture.
type catalogCommand struct {
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Mode        string `json:"mode"`
	WireMethod  string `json:"wire-method,omitempty"`
	// Backend names the data planes that implement the command. It is empty
	// when the command model restricts it to none, which means every backend
	// implements it (internal/component/command/node.go, Node.Backend).
	Backend []string `json:"backend,omitempty"`
	// TaskSupport says whether the MCP server answers this command with a task
	// handle. It is empty when the command declares no level, which is the
	// `optional` default (ze-extensions.yang, extension task-support).
	TaskSupport string `json:"task-support,omitempty"`
	// Usage is the invocation form an operator types, generated from the
	// command model. It replaces the retired `syntax` field, which a Python
	// helper scraped out of the description prose and truncated at the first
	// ". ", leaving several values cut mid-bracket.
	Usage string `json:"usage,omitempty"`
	// Grammar is that same form as an ordered token list, for a machine reader
	// that must not parse brackets out of a string. No surface renders it: a
	// person reads the line itself, which command.UsageLine writes from these
	// same tokens, so a second rendering would state one fact twice. It is
	// modeled because the model is what says which published fields this
	// package has considered (catalogfields_test.go).
	Grammar       []catalogToken    `json:"grammar,omitempty"`
	Args          []catalogArg      `json:"args,omitempty"`
	Pipes         []catalogPipe     `json:"pipes,omitempty"`
	Operators     []catalogOperator `json:"operators,omitempty"`
	AnswerShape   string            `json:"answer-shape,omitempty"`
	AddressFields []string          `json:"address-fields,omitempty"`
	Aliases       []catalogAlias    `json:"pipe-aliases,omitempty"`
	// Subcommands names the children an operator can type after this command.
	Subcommands []string `json:"subcommands,omitempty"`
}

type catalogArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Values is the closed set the argument's type states, and it is empty
	// when the type states none. It is what stops a reader guessing what an
	// argument of type enum accepts.
	Values    []string `json:"values,omitempty"`
	Mandatory bool     `json:"mandatory,omitempty"`
}

// catalogToken is one element of a command's invocation form
// (internal/component/command/usage.go, UsageToken).
type catalogToken struct {
	Text   string   `json:"text"`
	Values []string `json:"values,omitempty"`
	// Group holds the values a modifier group carries, in declaration order.
	// Only a group token has one.
	Group []catalogToken `json:"group,omitempty"`
	Kind  string         `json:"kind"`
}

type catalogPipe struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TakesArg    bool   `json:"takes-arg,omitempty"`
}

type catalogOperator struct {
	Name        string `json:"name"`
	Class       string `json:"class"`
	Available   string `json:"available"`
	LocalOnly   bool   `json:"local-only,omitempty"`
	Description string `json:"description"`
}

type catalogAlias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expansion   string `json:"expansion"`
}

// The availability values an operator declares, and the order the surfaces
// present them in. An operator carrying anything else is refused by name rather
// than published under a label nobody wrote.
const (
	availabilityAlways        = "always"
	availabilityWithRows      = "with-rows"
	availabilityWhenStreaming = "when-streaming"
	availabilityLocalOnly     = "local-only"
)

// availabilityOrder is the presentation order, widest availability first. It is
// a slice rather than a map because the order IS the published contract and a
// Go map states none.
var availabilityOrder = []string{
	availabilityAlways, availabilityWithRows, availabilityWhenStreaming, availabilityLocalOnly,
}

// availabilityLabels name each availability for a reader.
var availabilityLabels = map[string]string{
	availabilityAlways:        "Always",
	availabilityWithRows:      "With rows",
	availabilityWhenStreaming: "While streaming",
	availabilityLocalOnly:     "Local process only",
}

// modeLabels name each command mode for a reader.
var modeLabels = map[string]string{
	"daemon": "Daemon", "read-only": "Read-only", "offline": "Offline",
}

// taskSupportLabels say what each MCP task-support level means for a reader.
// The wording is the extension's own (ze-extensions.yang, extension
// task-support): a level a reader cannot interpret is a level nobody acts on.
var taskSupportLabels = map[string]string{
	"required":  "required: the MCP server always answers with a task handle",
	"optional":  "optional: the MCP call is synchronous",
	"forbidden": "forbidden: the MCP server never answers with a task handle",
}

// taskSupportDefault is what a command that declares no level takes. The
// default is stated rather than left blank, because "nothing is declared" is
// the answer to a question a reader still has.
const taskSupportDefault = "optional: the MCP call is synchronous, which is the default"

// backendsUnrestricted is what an empty backend list means: the command model
// restricts the command to no backend, so every backend implements it.
const backendsUnrestricted = "any backend"

// subcommandsNone is what an empty subcommand list means. The page and the
// mirror read one constant so the two cannot state the absence differently.
const subcommandsNone = "none: this command takes no subcommand"

// argumentValuesAny is what an argument with no closed set accepts. The absence
// is a fact about the argument's type, so the surfaces state it rather than
// leaving the reader with an empty cell.
const argumentValuesAny = "any value of this type"

// operatorClassLabels name each operator class for a reader.
var operatorClassLabels = map[string]string{
	"global": "Output and control", "stream": "Streaming", "data": "Row data",
}

// loadCommandCatalog reads the catalog this build published.
//
// It fails when the file is absent rather than publishing an empty command
// surface: an empty catalog would leave 398 pages unwritten and every check
// downstream would read the absence as success.
func loadCommandCatalog(output string) ([]catalogCommand, error) {
	path := filepath.Join(output, filepath.FromSlash(catalogFile))
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if err != nil {
		return nil, fmt.Errorf("read the published command catalog %s: %w", path, err)
	}
	var commands []catalogCommand
	if err := json.Unmarshal(content, &commands); err != nil {
		return nil, fmt.Errorf("read the published command catalog %s: %w", path, err)
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("the published command catalog %s names no command", path)
	}
	sort.Slice(commands, func(left, right int) bool { return commands[left].Path < commands[right].Path })
	for index := range commands {
		if err := (&commands[index]).validate(); err != nil {
			return nil, err
		}
	}
	return commands, nil
}

// validate refuses a command the surfaces cannot render honestly.
//
// An operator whose availability nothing names would be published without its
// qualifier, and the qualifier is part of the contract: a reader who cannot see
// that `match` needs rows will pipe into it and get an error.
func (command *catalogCommand) validate() error {
	if command.Path == "" {
		return fmt.Errorf("the published command catalog holds a command with no path")
	}
	if command.Mode == "" {
		return fmt.Errorf("command %q states no mode", command.Path)
	}
	for _, operator := range command.Operators {
		if _, known := availabilityLabels[operator.Available]; !known {
			return fmt.Errorf("command %q: operator %q declares the unknown availability %q",
				command.Path, operator.Name, operator.Available)
		}
	}
	return nil
}

// commandSlugSeparator matches every run this slugifier collapses to one dash.
var commandSlugSeparator = regexp.MustCompile(`[^a-z0-9]+`)

// commandSlug answers the URL segment and the anchor id of one command.
//
// It is derived from the registry PATH and never from the invocation form, so
// the page's own anchor, its detail page's directory and the identity the
// documentation drift check reads are one value rather than three.
func commandSlug(path string) string {
	return strings.Trim(commandSlugSeparator.ReplaceAllString(strings.ToLower(path), "-"), "-")
}

// commandModeLabel answers the reader's word for one mode, or the raw value
// when the catalog states a mode this site has no word for.
func commandModeLabel(mode string) string {
	if label, known := modeLabels[mode]; known {
		return label
	}
	return mode
}

// commandTaskSupportLabel answers what a command's task-support level means for
// a reader: the default when the command declares none, and the raw value when
// the catalog states a level this site has no wording for.
func commandTaskSupportLabel(level string) string {
	if level == "" {
		return taskSupportDefault
	}
	if label, known := taskSupportLabels[level]; known {
		return label
	}
	return level
}

// argumentRequiredLabel answers whether an argument is owed. Both command
// surfaces ask that question with the word "required" and answer it with this
// one, so a reader meets one vocabulary on either page.
func argumentRequiredLabel(argument catalogArg) string {
	if argument.Mandatory {
		return "yes"
	}
	return "no"
}

// pipeDisplayName answers how a command pipe is written for a reader: its name,
// and a value placeholder when the pipe takes one.
func pipeDisplayName(pipe catalogPipe) string {
	if pipe.TakesArg {
		return pipe.Name + " <value>"
	}
	return pipe.Name
}

// operatorsByAvailability groups one command's operators, in availabilityOrder.
//
// An operator marked local-only appears twice: once under the availability it
// declares, and once under local-only. Both facts are true of it and the page
// states both, which is what the retired renderer did.
func operatorsByAvailability(command *catalogCommand) map[string][]string {
	grouped := make(map[string][]string, len(availabilityOrder))
	for _, operator := range command.Operators {
		grouped[operator.Available] = append(grouped[operator.Available], operator.Name)
		if operator.LocalOnly {
			grouped[availabilityLocalOnly] = append(grouped[availabilityLocalOnly], operator.Name)
		}
	}
	return grouped
}

// The group sizes that decide how the catalog page splits its tables. A verb
// with more commands than commandGroupMax is split by subject, and a subject
// with fewer than commandSubgroupMin joins the verb's catch-all rather than
// becoming a one-row table of its own.
const (
	commandGroupMax    = 20
	commandSubgroupMin = 4
)

// commandGroup is one labeled table of the catalog page.
//
// The commands are pointers into the slice loadCommandCatalog answered, so
// grouping the 398-entry catalog copies no command.
type commandGroup struct {
	Label    string
	Commands []*catalogCommand
}

// groupCommands splits the catalog into the tables a reader scans.
//
// The rule is the retired renderer's, and it exists because `show` alone holds
// 254 commands across 67 subjects: splitting every subject out gives dozens of
// one-row tables, and splitting none gives one table nobody can scan. So a verb
// under the maximum stays whole, a frequent subject takes its own table, and
// the long tail shares the verb's "(other)" table.
func groupCommands(commands []catalogCommand) []commandGroup {
	byVerb := make(map[string][]*catalogCommand, len(commands))
	verbs := make([]string, 0, len(commands))
	for index := range commands {
		command := &commands[index]
		verb, _, _ := strings.Cut(command.Path, " ")
		if _, seen := byVerb[verb]; !seen {
			verbs = append(verbs, verb)
		}
		byVerb[verb] = append(byVerb[verb], command)
	}
	sort.Strings(verbs)

	groups := make([]commandGroup, 0, len(verbs))
	for _, verb := range verbs {
		entries := byVerb[verb]
		if len(entries) <= commandGroupMax {
			groups = append(groups, commandGroup{Label: verb, Commands: entries})
			continue
		}
		groups = append(groups, splitVerbBySubject(verb, entries)...)
	}
	return groups
}

// splitVerbBySubject breaks one oversized verb into its frequent subjects plus
// one catch-all. The catalog arrives sorted by path, so each bucket stays
// sorted without a second sort.
func splitVerbBySubject(verb string, entries []*catalogCommand) []commandGroup {
	bySubject := make(map[string][]*catalogCommand, len(entries))
	subjects := make([]string, 0, len(entries))
	for _, command := range entries {
		fields := strings.Fields(command.Path)
		subject := ""
		if len(fields) > 1 {
			subject = fields[1]
		}
		if _, seen := bySubject[subject]; !seen {
			subjects = append(subjects, subject)
		}
		bySubject[subject] = append(bySubject[subject], command)
	}
	sort.Strings(subjects)

	groups := make([]commandGroup, 0, len(subjects)+1)
	var other []*catalogCommand
	for _, subject := range subjects {
		bucket := bySubject[subject]
		if len(bucket) < commandSubgroupMin {
			other = append(other, bucket...)
			continue
		}
		label := verb
		if subject != "" {
			label = verb + " " + subject
		}
		groups = append(groups, commandGroup{Label: label, Commands: bucket})
	}
	if len(other) != 0 {
		sort.Slice(other, func(left, right int) bool { return other[left].Path < other[right].Path })
		groups = append(groups, commandGroup{Label: verb + " (other)", Commands: other})
	}
	return groups
}

// writePublishedPage writes one page and its Markdown mirror into the artifact.
//
// The two are written together because the mirror contract is per PAGE: every
// published route carries an index.md beside its index.html, and `./le site
// check` reports a route that carries only one of them.
func writePublishedPage(output, dest, page, mirror string) error {
	path := filepath.Join(output, filepath.FromSlash(dest))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	if err := os.WriteFile(path, []byte(page), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("write %s: %w", path, err)
	}
	return writeMarkdownMirror(path, mirror)
}
