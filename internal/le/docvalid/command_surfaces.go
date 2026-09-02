// Overview: drift.go -- registration in the documentation drift gate
//
// command_surfaces.go regenerates every command-facing document from the live
// command catalog, validates each rendered command container structurally, and
// compares sibling publications when those checkouts are present.

package docvalid

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/wikicatalog"
)

// The four availability keys the live command catalog publishes for a pipe
// operator, and the label each rendered surface gives them. The primary CLI
// page and the per-command equivalent page word the same key differently, so
// each surface has its own label.
const (
	availabilityAlways        = "always"
	availabilityWithRows      = "with-rows"
	availabilityWhenStreaming = "when-streaming"
	availabilityLocalOnly     = "local-only"

	alwaysLabel         = "Always"
	withRowsLabel       = "With rows"
	whileStreamingLabel = "While streaming"
	localOnlyLabel      = "Local process only"

	pipesAlwaysLabel         = "Pipes, always"
	pipesOnRowsLabel         = "Pipes, on rows"
	pipesWhileStreamingLabel = "Pipes, while streaming"
	pipesLocalOnlyLabel      = "Pipes, local process only"
)

// The HTML element names the published command surfaces are read for.
const (
	articleElement = "article"
	codeElement    = "code"
	spanElement    = "span"
	strongElement  = "strong"
)

// The named dimensions a per-command contract comparison reports.
const (
	pathField        = "path"
	descriptionField = "description"

	// malformedIdentity stands in for a command identity the surface holds in
	// a shape the reader could not parse, so a report never names an empty
	// command.
	malformedIdentity = "<malformed>"
)

// The published surfaces and sibling producers this gate reads by name.
const (
	llmsSurfaceName             = "llms.txt"
	wikiCatalogProducer         = "internal/le/wikicatalog/catalog.go"
	wikiCatalogRenderer         = "internal/le/wikicatalog/render.go"
	wikiCatalogNormalizeFailure = "could not normalize the shipping wiki command catalog producer"
)

type publishedCommandArg struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Mandatory bool     `json:"mandatory,omitempty"`
}

type publishedCommandPipe struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TakesArg    bool   `json:"takes-arg,omitempty"`
}

type publishedCommandOperator struct {
	Name        string `json:"name"`
	Class       string `json:"class"`
	Available   string `json:"available"`
	LocalOnly   bool   `json:"local-only,omitempty"`
	Description string `json:"description"`
}

type publishedCommandAlias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expansion   string `json:"expansion"`
}

// publishedCommandToken mirrors one grammar token of the live catalog. It is
// spelled here rather than imported so a change to the published shape shows up
// as drift instead of following the producer silently.
type publishedCommandToken struct {
	Text   string   `json:"text"`
	Values []string `json:"values,omitempty"`
	// Group holds the values a modifier group carries, in declaration order.
	// Only a group token has one (internal/component/command/usage.go,
	// UsageToken), and the parser rejects an unknown field, so a catalog
	// carrying a group is unreadable without it.
	Group []publishedCommandToken `json:"group,omitempty"`
	Kind  string                  `json:"kind"`
}

type publishedCommand struct {
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	// LongHelp is the command's own explanation, from ze:help. The catalog
	// carries it beside Description because the two answer different
	// questions, and the parser rejects an unknown field, so a catalog
	// carrying a long help is unreadable without it.
	LongHelp      string                     `json:"long-help,omitempty"`
	Mode          string                     `json:"mode"`
	WireMethod    string                     `json:"wire-method,omitempty"`
	Backend       []string                   `json:"backend,omitempty"`
	TaskSupport   string                     `json:"task-support,omitempty"`
	Args          []publishedCommandArg      `json:"args,omitempty"`
	Pipes         []publishedCommandPipe     `json:"pipes,omitempty"`
	Operators     []publishedCommandOperator `json:"operators,omitempty"`
	AnswerShape   string                     `json:"answer-shape,omitempty"`
	AddressFields []string                   `json:"address-fields,omitempty"`
	Aliases       []publishedCommandAlias    `json:"pipe-aliases,omitempty"`
	Usage         string                     `json:"usage,omitempty"`
	Grammar       []publishedCommandToken    `json:"grammar,omitempty"`
	Syntax        string                     `json:"syntax,omitempty"`
	Subcommands   []string                   `json:"subcommands,omitempty"`
}

const commandCatalogGenerationTimeout = 2 * time.Minute

func (c *checker) checkPublishedCommandSurfaces(commandCatalogPath string) []Issue {
	root := c.root
	if commandCatalogPath == "" {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return []Issue{{
				File:    commandSurfacePath(root, filepath.Join(root, "go.mod")),
				Message: "could not determine whether the root owns command surfaces",
				Detail:  err.Error(),
			}}
		}
	}
	var websiteCandidates, wikiCandidates []string
	if commandCatalogPath != "" {
		websiteCandidates = append(websiteCandidates,
			filepath.Join(root, "website", "data", "cli-commands.json"))
		wikiCandidates = append(wikiCandidates,
			filepath.Join(root, "wiki", "command-catalog.md"))
	} else if checkSiblingPublications {
		websiteCandidates = append(websiteCandidates,
			filepath.Join(filepath.Dir(root), "gh-pages", "data", "cli-commands.json"))
		wikiCandidates = append(wikiCandidates,
			filepath.Join(filepath.Dir(root), "wiki", "command-catalog.md"))
	}

	liveRaw, live, err := loadLiveCommandCatalog(root, commandCatalogPath)
	if err != nil {
		return []Issue{{
			File:    "cmd/ze/help_command.go",
			Message: "could not generate or parse the live per-command catalog",
			Detail:  err.Error(),
		}}
	}
	wikiEntries := c.collectWikiCatalogEntries()
	if producerIssues := compareWikiCatalogProducer(live, wikiEntries); len(producerIssues) != 0 {
		return producerIssues
	}
	expectedWiki, err := wikicatalog.Render(wikiEntries)
	if err != nil {
		return []Issue{{
			File:    "internal/le/wikicatalog/render.go",
			Message: "could not generate the expected wiki command catalog",
			Detail:  err.Error(),
		}}
	}
	issues := validateGeneratedWikiCommandSurface(expectedWiki, live)
	wikiPaths, wikiPathErr := existingPaths(wikiCandidates...)
	if wikiPathErr != nil {
		issues = append(issues,
			commandSurfaceReadIssue("wiki command catalog", wikiPathErr))
	}
	for _, path := range wikiPaths {
		issues = append(issues, compareWikiCommandCatalog(root, path, expectedWiki)...)
	}

	websitePaths, websitePathErr := existingPaths(websiteCandidates...)
	if websitePathErr != nil {
		return append(issues,
			commandSurfaceReadIssue("website command catalog", websitePathErr))
	}
	if len(websitePaths) == 0 && len(websiteCandidates) != 0 {
		websiteCandidate := websiteCandidates[0]
		hasPublishedWebsite, inspectErr := publishedWebsiteRootExists(
			filepath.Dir(filepath.Dir(websiteCandidate)), commandCatalogPath != "",
		)
		if inspectErr != nil {
			return append(issues,
				commandSurfaceReadIssue("website command surfaces", inspectErr))
		}
		if hasPublishedWebsite {
			return append(issues, Issue{
				File:    commandSurfacePath(root, websiteCandidate),
				Message: "the published website command catalog is missing",
				Detail:  "run `./le site build` before the documentation check",
			})
		}
	}

	publicWebsiteRoot := ""
	if len(websitePaths) != 0 {
		publicWebsiteRoot = filepath.Dir(filepath.Dir(websitePaths[0]))
	}
	expectedRoot, err := renderExpectedCommandSurfaces(
		root, commandCatalogPath, publicWebsiteRoot, liveRaw, len(live),
	)
	if err != nil {
		return append(issues, Issue{
			File:    "internal/le/site",
			Message: "could not generate the expected per-command surfaces",
			Detail:  err.Error(),
		})
	}
	issues = append(issues, validateGeneratedCommandSurfaces(root, expectedRoot, live)...)
	if publicWebsiteRoot != "" {
		issues = append(issues,
			compareWebsiteCommandCatalog(root, websitePaths[0], live)...)
		issues = append(issues,
			compareRenderedCommandSurfaces(root, publicWebsiteRoot, expectedRoot, live)...)
	}
	if err := os.RemoveAll(expectedRoot); err != nil {
		issues = append(issues, Issue{
			File:    commandSurfacePath(root, expectedRoot),
			Message: "could not remove the temporary command surfaces",
			Detail:  err.Error(),
		})
	}
	return issues
}

func renderExpectedCommandSurfaces(
	root, _, _ string,
	liveRaw []byte,
	_ int,
) (string, error) {
	commands, err := parseCommandCatalog("live command catalog", liveRaw)
	if err != nil {
		return "", err
	}
	tmpParent := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpParent, 0o750); err != nil {
		return "", fmt.Errorf("create command render temporary parent %s: %w", tmpParent, err)
	}
	outputRoot, err := os.MkdirTemp(tmpParent, "docvalid-command-surfaces-")
	if err != nil {
		return "", fmt.Errorf("create command render temporary root: %w", err)
	}
	if err := renderCommandSurfaces(outputRoot, commands); err != nil {
		os.RemoveAll(outputRoot) //nolint:errcheck // best-effort cleanup after the primary error
		return "", err
	}
	return outputRoot, nil
}

func optionalCommandSurfacePath(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func publishedWebsiteRootExists(path string, fixture bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect published website root %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("published website root %s is not a directory", path)
	}
	if !fixture {
		return true, nil
	}
	for _, relative := range []string{"data", "reference", llmsSurfaceName} {
		candidate := filepath.Join(path, relative)
		exists, err := optionalCommandSurfacePath(candidate)
		if err != nil {
			return false, fmt.Errorf("inspect published website surface %s: %w", candidate, err)
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func existingPaths(paths ...string) ([]string, error) {
	var existing []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return nil, fmt.Errorf("%s is a directory", path)
			}
			existing = append(existing, path)
			continue
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		parent, parentErr := os.Stat(filepath.Dir(path))
		if parentErr == nil {
			if !parent.IsDir() {
				return nil, fmt.Errorf("%s is not a directory", filepath.Dir(path))
			}
			return nil, fmt.Errorf("%s is missing", path)
		}
		if !os.IsNotExist(parentErr) {
			return nil, fmt.Errorf("inspect %s: %w", filepath.Dir(path), parentErr)
		}
	}
	return existing, nil
}

func commandSurfaceReadIssue(surface string, err error) Issue {
	return Issue{
		File:    surface,
		Message: "could not read the published per-command surface",
		Detail:  err.Error(),
	}
}

func loadLiveCommandCatalog(root, commandCatalogPath string) ([]byte, []publishedCommand, error) {
	if commandCatalogPath != "" {
		data, err := os.ReadFile(commandCatalogPath) //nolint:gosec // caller-selected fixture or repository artifact
		if err != nil {
			return nil, nil, fmt.Errorf("read command catalog %s: %w", commandCatalogPath, err)
		}
		commands, err := parseCommandCatalog(commandCatalogPath, data)
		return data, commands, err
	}

	tags, err := shippedCommandCatalogTags(root)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandCatalogGenerationTimeout)
	defer cancel()
	args := []string{"run", "-tags", strings.Join(tags, ","), "./cmd/ze", "help", "command", "--json"}
	// #nosec G204 -- the argv is fixed apart from the build tags, which are read
	// from the checkout's own feature-gates.txt.
	cmd := osexec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("generate `ze help command --json`: %w: %s",
			err, strings.TrimSpace(stderr.String()))
	}
	commands, err := parseCommandCatalog("ze help command --json", data)
	return data, commands, err
}

func shippedCommandCatalogTags(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "feature-gates.txt")) // #nosec G304 -- the feature manifest under the checkout root
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt for command generation: %w", err)
	}
	tags := []string{"ze_core"}
	seen := map[string]bool{"ze_core": true}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(fields[0], "#") {
			continue
		}
		if !seen[fields[0]] {
			seen[fields[0]] = true
			tags = append(tags, fields[0])
		}
	}
	return tags, nil
}

func parseCommandCatalog(source string, data []byte) ([]publishedCommand, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var commands []publishedCommand
	if err := decoder.Decode(&commands); err != nil {
		return nil, fmt.Errorf("parse %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse %s: content follows the command array", source)
		}
		return nil, fmt.Errorf("parse %s after command array: %w", source, err)
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("parse %s: command array is empty", source)
	}
	seen := make(map[string]bool, len(commands))
	for index := range commands {
		entry := &commands[index]
		if err := validatePublishedCommand(source, entry, seen); err != nil {
			return nil, err
		}
		seen[entry.Path] = true
	}
	return commands, nil
}

func validatePublishedCommand(source string, entry *publishedCommand, seen map[string]bool) error {
	if entry.Path == "" {
		return fmt.Errorf("parse %s: command has an empty path", source)
	}
	if seen[entry.Path] {
		return fmt.Errorf("parse %s: command path %q appears twice", source, entry.Path)
	}
	if entry.Mode == "" {
		return fmt.Errorf("parse %s: command %q has no mode", source, entry.Path)
	}
	switch entry.AnswerShape {
	case "", "doc", "map", "tab":
	default:
		return fmt.Errorf("parse %s: command %q has unknown answer shape %q",
			source, entry.Path, entry.AnswerShape)
	}
	seenArgs := make(map[string]bool, len(entry.Args))
	for _, arg := range entry.Args {
		if arg.Name == "" {
			return fmt.Errorf("parse %s: command %q has an argument without a name", source, entry.Path)
		}
		if seenArgs[arg.Name] {
			return duplicatePublishedCommandIdentity(source, entry.Path, "args", arg.Name)
		}
		seenArgs[arg.Name] = true
		if arg.Type == "" {
			return fmt.Errorf("parse %s: command %q argument %q has no kind", source, entry.Path, arg.Name)
		}
	}
	seenPipes := make(map[string]bool, len(entry.Pipes))
	for _, pipe := range entry.Pipes {
		if pipe.Name == "" {
			return fmt.Errorf("parse %s: command %q has a filter without a name", source, entry.Path)
		}
		if seenPipes[pipe.Name] {
			return duplicatePublishedCommandIdentity(source, entry.Path, "pipes", pipe.Name)
		}
		seenPipes[pipe.Name] = true
		if pipe.Description == "" {
			return fmt.Errorf("parse %s: command %q filter %q has no description", source, entry.Path, pipe.Name)
		}
	}
	seenAddressFields := make(map[string]bool, len(entry.AddressFields))
	for _, field := range entry.AddressFields {
		if field == "" {
			return fmt.Errorf("parse %s: command %q has an empty address field", source, entry.Path)
		}
		if seenAddressFields[field] {
			return duplicatePublishedCommandIdentity(
				source, entry.Path, "address-fields", field,
			)
		}
		seenAddressFields[field] = true
	}
	seenOperators := make(map[string]bool, len(entry.Operators))
	for _, op := range entry.Operators {
		if op.Name == "" {
			return fmt.Errorf("parse %s: command %q has an operator without a name", source, entry.Path)
		}
		if seenOperators[op.Name] {
			return duplicatePublishedCommandIdentity(
				source, entry.Path, "operators", op.Name,
			)
		}
		seenOperators[op.Name] = true
		if op.Class == "" {
			return fmt.Errorf("parse %s: command %q operator %q has no class", source, entry.Path, op.Name)
		}
		if op.Description == "" {
			return fmt.Errorf("parse %s: command %q operator %q has no description", source, entry.Path, op.Name)
		}
		switch op.Available {
		case availabilityAlways, availabilityWithRows, availabilityWhenStreaming:
		default:
			return fmt.Errorf("parse %s: command %q operator %q has unknown availability %q",
				source, entry.Path, op.Name, op.Available)
		}
	}
	seenAliases := make(map[string]bool, len(entry.Aliases))
	for _, alias := range entry.Aliases {
		if alias.Name == "" {
			return fmt.Errorf("parse %s: command %q has an alias without a name", source, entry.Path)
		}
		if seenAliases[alias.Name] {
			return duplicatePublishedCommandIdentity(
				source, entry.Path, "pipe-aliases", alias.Name,
			)
		}
		seenAliases[alias.Name] = true
		if alias.Description == "" {
			return fmt.Errorf("parse %s: command %q alias %q has no description", source, entry.Path, alias.Name)
		}
		if alias.Expansion == "" {
			return fmt.Errorf("parse %s: command %q alias %q has no expansion", source, entry.Path, alias.Name)
		}
	}
	seenSubcommands := make(map[string]bool, len(entry.Subcommands))
	for _, subcommand := range entry.Subcommands {
		if subcommand == "" {
			return fmt.Errorf("parse %s: command %q has an empty subcommand", source, entry.Path)
		}
		if seenSubcommands[subcommand] {
			return duplicatePublishedCommandIdentity(
				source, entry.Path, "subcommands", subcommand,
			)
		}
		seenSubcommands[subcommand] = true
	}
	return nil
}

func duplicatePublishedCommandIdentity(source, path, list, identity string) error {
	return fmt.Errorf(
		"parse %s: command %q has duplicate identity %q in its %s list",
		source, path, identity, list,
	)
}

func compareWebsiteCommandCatalog(root, path string, live []publishedCommand) []Issue {
	publishedRaw, err := os.ReadFile(path) //nolint:gosec // generated sibling checkout artifact
	if err != nil {
		return []Issue{commandSurfaceReadIssue(commandSurfacePath(root, path), err)}
	}
	published, err := parseCommandCatalog(commandSurfacePath(root, path), publishedRaw)
	if err != nil {
		return []Issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not parse the published website command catalog",
			Detail:  err.Error(),
		}}
	}
	for i := range published {
		// The website derives display syntax from the canonical description.
		// It is not part of `ze help command --json`.
		published[i].Syntax = ""
	}
	liveJSON, err := json.Marshal(live)
	if err != nil {
		return []Issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not encode the live website command catalog",
			Detail:  err.Error(),
		}}
	}
	publishedJSON, err := json.Marshal(published)
	if err != nil {
		return []Issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not encode the published website command catalog",
			Detail:  err.Error(),
		}}
	}
	if bytes.Equal(liveJSON, publishedJSON) {
		return nil
	}
	return []Issue{{
		File:    commandSurfacePath(root, path),
		Message: "the published website command catalog and the live command catalog disagree",
		Detail: "run `./le site build`; every command's operators, qualifiers, aliases, " +
			"filters, shape, address fields, descriptions, and argument kinds must match",
	}}
}

func (c *checker) collectWikiCatalogEntries() []wikicatalog.Entry {
	if c.wikiCatalogCollect != nil {
		return c.wikiCatalogCollect()
	}
	return wikicatalog.Collect()
}

func compareWikiCatalogProducer(
	live []publishedCommand,
	entries []wikicatalog.Entry,
) []Issue {
	producedRaw, err := json.Marshal(entries)
	if err != nil {
		return []Issue{{
			File:    wikiCatalogProducer,
			Message: wikiCatalogNormalizeFailure,
			Detail:  err.Error(),
		}}
	}
	produced, err := parseCommandCatalog("wikicatalog.Collect", producedRaw)
	if err != nil {
		return []Issue{{
			File:    wikiCatalogProducer,
			Message: wikiCatalogNormalizeFailure,
			Detail:  err.Error(),
		}}
	}
	liveNormalized, err := json.Marshal(live)
	if err != nil {
		return []Issue{{
			File:    "cmd/ze/help_command.go",
			Message: "could not normalize the live per-command catalog",
			Detail:  err.Error(),
		}}
	}
	producedNormalized, err := json.Marshal(produced)
	if err != nil {
		return []Issue{{
			File:    wikiCatalogProducer,
			Message: wikiCatalogNormalizeFailure,
			Detail:  err.Error(),
		}}
	}
	if bytes.Equal(liveNormalized, producedNormalized) {
		return nil
	}
	detail := fmt.Sprintf("live catalog has %d rows and wiki producer has %d", len(live), len(produced))
	for index := 0; index < len(live) && index < len(produced); index++ {
		liveRow, _ := json.Marshal(live[index])
		producedRow, _ := json.Marshal(produced[index])
		if !bytes.Equal(liveRow, producedRow) {
			detail = fmt.Sprintf("first mismatch at row %d\nlive: %s\nwiki: %s", index, liveRow, producedRow)
			break
		}
	}
	return []Issue{{
		File:    wikiCatalogProducer,
		Message: "the shipping wiki catalog producer and the live command catalog disagree",
		Detail:  detail,
	}}
}

func renderExpectedWikiCommandSurface(
	_, _ string,
	liveRaw []byte,
) ([]byte, error) {
	var entries []wikicatalog.Entry
	if err := json.Unmarshal(liveRaw, &entries); err != nil {
		return nil, fmt.Errorf("decode wiki command catalog: %w", err)
	}
	return wikicatalog.Render(entries)
}

func compareWikiCommandCatalog(root, path string, want []byte) []Issue {
	published, err := os.ReadFile(path) //nolint:gosec // generated sibling checkout artifact
	if err != nil {
		return []Issue{commandSurfaceReadIssue(commandSurfacePath(root, path), err)}
	}
	if bytes.Equal(published, want) {
		return nil
	}
	return []Issue{{
		File:    commandSurfacePath(root, path),
		Message: "the published wiki command catalog and the live command catalog disagree",
		Detail:  "run `./le wiki-catalog update file <catalog.md>`; the wiki must preserve every per-command contract field",
	}}
}

func validateGeneratedWikiCommandSurface(
	generated []byte,
	live []publishedCommand,
) []Issue {
	const surface = wikiCatalogRenderer
	content := string(generated)
	var issues []Issue
	var rendered textbuf.Buffer
	expectedPaths := make([]string, 0, len(live))
	for index := range live {
		command := &live[index]
		expectedPaths = append(expectedPaths, command.Path)
	}
	issues = append(issues, validateWikiCatalogStructure(
		surface, content, live,
	)...)
	issues = append(issues, compareCommandNamedGroup(
		surface, "<wiki catalog>", "command", expectedPaths,
		wikiCommandSummaryPaths(content),
	)...)
	issues = append(issues, compareCommandNamedGroup(
		surface, "<wiki catalog>", "command detail", expectedPaths,
		wikiCommandDetailPaths(content),
	)...)
	for index := range live {
		command := &live[index]
		// The summary column takes the declared summary whole, and the detail
		// block takes the declared long form. Neither is a cut of the other:
		// the wiki renderer reads two fields the command model declares
		// separately (internal/le/wikicatalog/render.go, Render).
		summary := wikiTableProse(normalizeWikiDescription(command.Description))
		longHelp := normalizeWikiDescription(command.LongHelp)
		wantRow := rendered.Reset().Str("| ").
			Str(markdownCodeLiteral(commandMarkdownTableValue(command.Path))).Str(" | ").
			Str(command.Mode).Str(" | ").
			Str(markdownLiteralProse(summary)).Str(" |").String()
		row, rowCount, rowMalformed := commandSurfaceMarkdownRow(content, command.Path)
		if rowCount != 1 || rowMalformed || row != wantRow {
			issues = append(issues, generatedCommandContractIssue(
				surface, command.Path, "wiki command summary row",
			))
		}

		detail, detailCount := wikiCommandDetail(content, command.Path)
		if detailCount != 1 {
			issues = append(issues, commandContainerCountIssue(
				surface, command.Path, "wiki command detail section", detailCount,
			))
			continue
		}
		if longHelp != "" {
			lines := strings.Split(longHelp, "\n")
			for index := range lines {
				lines[index] = markdownLiteralProse(lines[index])
			}
			wantLongHelp := rendered.Reset().Byte('\n').Str(strings.Join(lines, "\n")).
				Str("\n\n").String()
			if !strings.HasPrefix(detail, wantLongHelp) {
				issues = append(issues, generatedCommandContractIssue(
					surface, command.Path, "wiki command long help",
				))
			}
		}
		rendered.Reset().Str("Mode: ").Str(command.Mode)
		if command.WireMethod != "" {
			rendered.Str(" | Wire: ").Str(markdownCodeLiteral(command.WireMethod))
		}
		if !wikiLineEquals(detail, rendered.String()) {
			issues = append(issues, generatedCommandContractIssue(
				surface, command.Path, "wiki mode and wire metadata",
			))
		}
		wantShape := ""
		if command.AnswerShape != "" {
			wantShape = rendered.Reset().Str("Answer shape: ").
				Str(markdownCodeLiteral(command.AnswerShape)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			command.Path, detail, "Answer shape: ",
			"answer shape", wantShape,
		)...)
		wantAddressFields := ""
		if len(command.AddressFields) != 0 {
			wantAddressFields = rendered.Reset().Str("Address fields: ").
				Str(wikiCodeList(command.AddressFields)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			command.Path, detail, "Address fields: ",
			"address fields", wantAddressFields,
		)...)
		wantBackend := ""
		if len(command.Backend) != 0 {
			wantBackend = rendered.Reset().Str("**Requires backend:** ").
				Str(wikiCodeList(command.Backend)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			command.Path, detail, "**Requires backend:** ",
			"backend requirements", wantBackend,
		)...)
		wantTaskSupport := ""
		if command.TaskSupport != "" {
			wantTaskSupport = rendered.Reset().Str("**Task support:** ").
				Str(markdownLiteralProse(command.TaskSupport)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			command.Path, detail, "**Task support:** ",
			"task support", wantTaskSupport,
		)...)
		expectedArgs := make([]string, 0, len(command.Args))
		for _, arg := range command.Args {
			required := ""
			if arg.Mandatory {
				required = "yes"
			}
			rendered.Reset().Str("| ").
				Str(markdownCodeLiteral(commandMarkdownTableValue(arg.Name))).Str(" | ").
				Str(markdownCodeLiteral(commandMarkdownTableValue(arg.Type))).
				Str(" | ").Str(required).Str(" | ").
				Str(wikiTableCodeList(arg.Values)).Str(" |")
			expectedArgs = append(expectedArgs, rendered.String())
		}
		issues = append(issues, compareCommandNamedGroup(
			surface, command.Path, "wiki argument", expectedArgs,
			wikiArgumentRows(detail),
		)...)

		pipeScan := scanWikiPipeGroups(detail)
		issues = append(issues, validateWikiPipeSupport(
			surface, command.Path, pipeScan,
			len(command.Operators) != 0 || len(command.Pipes) != 0 ||
				len(command.Aliases) != 0,
		)...)
		for _, availability := range commandOperatorAvailabilities {
			issues = append(issues, compareCommandOperatorGroups(
				surface, command.Path, availability,
				commandOperatorNames(command, availability),
				pipeScan.groups[availability],
			)...)
		}
		expectedAliases := make([]string, 0, len(command.Aliases))
		for _, alias := range command.Aliases {
			rendered.Reset().Str("- ").Str(markdownCodeLiteral(alias.Name)).Str(" -- ").
				Str(markdownLiteralProse(alias.Description)).Str(" (").
				Str(markdownCodeLiteral(alias.Expansion)).Byte(')')
			expectedAliases = append(expectedAliases, rendered.String())
		}
		issues = append(issues, compareCommandNamedGroups(
			surface, command.Path, "wiki pipe alias", expectedAliases,
			wikiBulletGroups(detail, "Named chains:"),
		)...)
		expectedFilters := make([]string, 0, len(command.Pipes))
		for _, filter := range command.Pipes {
			rendered.Reset().Str("- ").Str(markdownCodeLiteral(filter.Name))
			if filter.TakesArg {
				rendered.Byte(' ').Str(markdownCodeLiteral("<value>"))
			}
			rendered.Str(" -- ").Str(markdownLiteralProse(filter.Description))
			expectedFilters = append(expectedFilters, rendered.String())
		}
		issues = append(issues, compareCommandNamedGroups(
			surface, command.Path, "wiki command filter", expectedFilters,
			wikiBulletGroups(detail, "Command-specific:"),
		)...)
		wantSubcommands := ""
		if len(command.Subcommands) != 0 {
			wantSubcommands = rendered.Reset().Str("**Subcommands:** ").
				Str(wikiCodeList(command.Subcommands)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			command.Path, detail, "**Subcommands:** ",
			"subcommands", wantSubcommands,
		)...)
	}
	return issues
}

type wikiVerbGroup struct {
	verb   string
	anchor string
	count  int
}

type wikiContentsEntry struct {
	label  string
	anchor string
	count  int
}

func validateWikiCatalogStructure(
	surface, content string,
	live []publishedCommand,
) []Issue {
	expected := wikiExpectedVerbGroups(live)
	var issues []Issue

	contents, contentsValid := wikiContentsEntries(content)
	if !contentsValid || len(contents) != len(expected) {
		issues = append(issues, generatedCommandContractIssue(
			surface, "<wiki catalog>", "wiki contents",
		))
	} else {
		for index := range expected {
			if contents[index].label != expected[index].verb ||
				contents[index].anchor != expected[index].anchor ||
				contents[index].count != expected[index].count {
				issues = append(issues, generatedCommandContractIssue(
					surface, "<wiki catalog>", "wiki contents",
				))
				break
			}
		}
	}

	headings := wikiVerbHeadings(content)
	if len(headings) != len(expected) {
		issues = append(issues, generatedCommandContractIssue(
			surface, "<wiki catalog>", "wiki verb headings",
		))
	} else {
		for index, group := range expected {
			want := "## " + markdownLiteralProse(group.verb)
			if headings[index] != want {
				issues = append(issues, generatedCommandContractIssue(
					surface, "<wiki catalog>", "wiki verb headings",
				))
				break
			}
		}
	}

	wantTotal := fmt.Sprintf("*%d commands total.*", len(live))
	totalLines, totalValid := wikiTotalLines(content)
	if !totalValid || len(totalLines) != 1 || totalLines[0] != wantTotal ||
		!strings.HasSuffix(content, wantTotal+"\n") {
		issues = append(issues, generatedCommandContractIssue(
			surface, "<wiki catalog>", "wiki command total",
		))
	}
	return issues
}

func wikiExpectedVerbGroups(live []publishedCommand) []wikiVerbGroup {
	commands := make(map[string][]publishedCommand)
	for index := range live {
		command := &live[index]
		words := strings.Fields(command.Path)
		verb := command.Path
		if len(words) != 0 {
			verb = words[0]
		}
		commands[verb] = append(commands[verb], *command)
	}
	verbs := make([]string, 0, len(commands))
	for verb := range commands {
		verbs = append(verbs, verb)
	}
	sort.Strings(verbs)
	for _, verb := range verbs {
		group := commands[verb]
		sort.Slice(group, func(left, right int) bool {
			return group[left].Path < group[right].Path
		})
		commands[verb] = group
	}

	usedAnchors := make(map[string]bool)
	anchorSuffixes := make(map[string]int)
	nextAnchor := func(heading string) string {
		base := wikiHeadingAnchor(heading)
		anchor := base
		for usedAnchors[anchor] {
			anchorSuffixes[base]++
			anchor = base + "-" + strconv.Itoa(anchorSuffixes[base])
		}
		usedAnchors[anchor] = true
		return anchor
	}
	nextAnchor("Command Catalog")
	nextAnchor("Contents")

	groups := make([]wikiVerbGroup, 0, len(verbs))
	for _, verb := range verbs {
		groups = append(groups, wikiVerbGroup{
			verb: verb, anchor: nextAnchor(verb), count: len(commands[verb]),
		})
		for index := range commands[verb] {
			nextAnchor(commands[verb][index].Path)
		}
	}
	return groups
}

func wikiHeadingAnchor(value string) string {
	var anchor textbuf.Buffer
	for _, character := range strings.ToLower(value) {
		switch {
		case character == ' ' || character == '\t':
			anchor.Byte('-')
		case character <= unicode.MaxASCII:
			if character >= 'a' && character <= 'z' ||
				character >= '0' && character <= '9' ||
				character == '-' || character == '_' {
				anchor.WriteRune(character)
			}
		case !unicode.IsPunct(character) && !unicode.IsSpace(character):
			anchor.WriteRune(character)
		}
	}
	if anchor.Len() == 0 {
		return "u--" + hex.EncodeToString([]byte(value))
	}
	return anchor.String()
}

func wikiContentsEntries(content string) ([]wikiContentsEntry, bool) {
	block, count := markdownHeadingContent(content, "## Contents")
	if count != 1 {
		return nil, false
	}
	var entries []wikiContentsEntry
	for _, line := range scanMarkdownLines(block) {
		if !line.active || strings.TrimSpace(line.text) == "" {
			continue
		}
		entry, ok := wikiContentsLine(line.text)
		if !ok {
			return nil, false
		}
		entries = append(entries, entry)
	}
	return entries, true
}

func wikiContentsLine(line string) (wikiContentsEntry, bool) {
	var entry wikiContentsEntry
	if !strings.HasPrefix(line, "- [") {
		return entry, false
	}
	labelEnd := markdownInlineClosingBracket(line, len("- ["))
	if labelEnd == -1 {
		return entry, false
	}
	suffix := line[labelEnd+1:]
	anchorEnd := strings.Index(suffix, ") (")
	if !strings.HasPrefix(suffix, "(#") || anchorEnd == -1 ||
		!strings.HasSuffix(suffix, ")") {
		return entry, false
	}
	count, err := strconv.Atoi(suffix[anchorEnd+3 : len(suffix)-1])
	if err != nil || count < 0 {
		return entry, false
	}
	entry.label = markdownInlineVisibleText(line[len("- ["):labelEnd])
	entry.anchor = suffix[len("(#"):anchorEnd]
	entry.count = count
	return entry, true
}

func wikiVerbHeadings(content string) []string {
	var headings []string
	for _, line := range scanMarkdownLines(content) {
		if !line.active || line.headingLevel != 2 || line.text == "## Contents" {
			continue
		}
		headings = append(headings, line.text)
	}
	return headings
}

func wikiTotalLines(content string) ([]string, bool) {
	var totals []string
	footer := false
	valid := true
	for _, line := range scanMarkdownLines(content) {
		if !line.active || strings.TrimSpace(line.text) == "" {
			continue
		}
		if line.text == "---" {
			if footer {
				valid = false
			}
			footer = true
			continue
		}
		if footer {
			totals = append(totals, line.text)
		}
	}
	return totals, footer && valid
}

// wikiTableProse answers one prose value as the wiki renders it into a Markdown
// table cell, which cannot hold a line break
// (internal/le/wikicatalog/render.go, tableProse).
func wikiTableProse(value string) string {
	if !strings.ContainsRune(value, '\n') {
		return value
	}
	return strings.ReplaceAll(value, "\n", " ")
}

func normalizeWikiDescription(value string) string {
	if !strings.ContainsRune(value, '\r') {
		return value
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

type markdownLine struct {
	text         string
	active       bool
	headingLevel int
	heading      string
}

func scanMarkdownLines(content string) []markdownLine {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	rawLines := strings.Split(content, "\n")
	lines := make([]markdownLine, 0, len(rawLines))
	var fence byte
	fenceWidth := 0
	for _, text := range rawLines {
		marker, width, rest := markdownFence(text)
		if fence != 0 {
			lines = append(lines, markdownLine{text: text})
			if marker == fence && width >= fenceWidth && markdownASCIIBlank(rest) {
				fence = 0
				fenceWidth = 0
			}
			continue
		}
		if marker != 0 && width >= 3 {
			fence = marker
			fenceWidth = width
			lines = append(lines, markdownLine{text: text})
			continue
		}
		lines = append(lines, markdownLine{text: text, active: true})
	}

	lines = joinMarkdownCodeSpanContinuations(lines)
	for index := range lines {
		if !lines[index].active {
			continue
		}
		lines[index].headingLevel, lines[index].heading = markdownATXHeading(lines[index].text)
	}
	return lines
}

func markdownASCIIBlank(value string) bool {
	for index := range len(value) {
		if value[index] != ' ' && value[index] != '\t' {
			return false
		}
	}
	return true
}

func joinMarkdownCodeSpanContinuations(lines []markdownLine) []markdownLine {
	joined := make([]markdownLine, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		codeAt, indent, candidate := markdownContainerCodeOffset(line.text)
		if !line.active || !candidate {
			joined = append(joined, line)
			continue
		}
		_, _, _, closed := markdownCodeSpanPrefix(line.text[codeAt:])
		for !closed && index+1 < len(lines) && lines[index+1].active {
			index++
			continuation := lines[index].text
			for removed := 0; removed < indent && continuation != "" &&
				(continuation[0] == ' ' || continuation[0] == '\t'); removed++ {
				continuation = continuation[1:]
			}
			line.text += "\n" + continuation
			_, _, _, closed = markdownCodeSpanPrefix(line.text[codeAt:])
		}
		joined = append(joined, line)
	}
	return joined
}

func markdownContainerCodeOffset(line string) (int, int, bool) {
	offset := 0
	for offset < len(line) && offset < 4 && line[offset] == ' ' {
		offset++
	}
	if offset == 4 || offset == len(line) {
		return 0, 0, false
	}
	switch line[offset] {
	case '|', '-':
		offset++
	case '#':
		for offset < len(line) && line[offset] == '#' {
			offset++
		}
		if offset == len(line) || (line[offset] != ' ' && line[offset] != '\t') {
			return 0, 0, false
		}
	default:
		return 0, 0, false
	}
	for offset < len(line) && (line[offset] == ' ' || line[offset] == '\t') {
		offset++
	}
	if offset == len(line) || line[offset] != '`' {
		return 0, 0, false
	}
	return offset, offset, true
}

func markdownFence(line string) (byte, int, string) {
	trimmed := line
	spaces := 0
	for spaces < len(trimmed) && spaces < 4 && trimmed[spaces] == ' ' {
		spaces++
	}
	if spaces == 4 || spaces == len(trimmed) {
		return 0, 0, ""
	}
	trimmed = trimmed[spaces:]
	if trimmed[0] != '`' && trimmed[0] != '~' {
		return 0, 0, ""
	}
	marker := trimmed[0]
	width := 0
	for width < len(trimmed) && trimmed[width] == marker {
		width++
	}
	if width < 3 {
		return 0, 0, ""
	}
	rest := trimmed[width:]
	if marker == '`' && strings.ContainsRune(rest, '`') {
		return 0, 0, ""
	}
	return marker, width, rest
}

func markdownATXHeading(line string) (int, string) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" || trimmed[0] != '#' {
		return 0, ""
	}
	level := 0
	for level < len(trimmed) && level < 7 && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(trimmed) ||
		(trimmed[level] != ' ' && trimmed[level] != '\t') {
		return 0, ""
	}
	heading := strings.TrimSpace(trimmed[level:])
	hashAt := len(heading)
	for hashAt > 0 && heading[hashAt-1] == '#' {
		hashAt--
	}
	if hashAt > 0 && hashAt < len(heading) &&
		(heading[hashAt-1] == ' ' || heading[hashAt-1] == '\t') {
		heading = strings.TrimSpace(heading[:hashAt-1])
	}
	return level, heading
}

func markdownHeadingContent(content, canonical string) (string, int) {
	level, heading := markdownATXHeading(canonical)
	lines := scanMarkdownLines(content)
	target := markdownRenderedHeadingIdentity(heading)
	var rendered textbuf.Buffer
	targetPrefix := rendered.Str(target).Byte(' ').String()
	canonicalAt := -1
	count := 0
	malformed := 0
	for index, line := range lines {
		if !line.active || line.headingLevel == 0 {
			continue
		}
		normalized := markdownRenderedHeadingIdentity(line.heading)
		if normalized != target && !strings.HasPrefix(normalized, targetPrefix) {
			continue
		}
		if line.text == canonical {
			canonicalAt = index
			count++
		} else {
			malformed++
		}
	}
	if count == 0 {
		return "", malformed
	}
	if count != 1 || malformed != 0 {
		return "", count + malformed
	}
	end := len(lines)
	for index := canonicalAt + 1; index < len(lines); index++ {
		if lines[index].active && lines[index].headingLevel != 0 &&
			lines[index].headingLevel <= level {
			end = index
			break
		}
	}
	return markdownLineRange(lines, canonicalAt+1, end), count
}

func markdownRenderedHeadingIdentity(heading string) string {
	tokenizer := xhtml.NewTokenizer(strings.NewReader(heading))
	var rendered textbuf.Buffer
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return strings.Join(
				strings.Fields(markdownInlineVisibleText(rendered.String())), " ",
			)
		case xhtml.TextToken:
			rendered.Str(tokenizer.Token().Data)
		case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken,
			xhtml.CommentToken, xhtml.DoctypeToken:
			// Only visible text contributes to the rendered heading identity.
		}
	}
}

func markdownInlineVisibleText(value string) string {
	tokenizer := xhtml.NewTokenizer(strings.NewReader(markdownHTMLSafeInline(value)))
	var text textbuf.Buffer
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return markdownInlineVisibleTextNoHTML(text.String())
		case xhtml.TextToken:
			text.Str(tokenizer.Token().Data)
		case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken,
			xhtml.CommentToken, xhtml.DoctypeToken:
			// Inline markup is not visible text.
		}
	}
}

func markdownHTMLSafeInline(value string) string {
	var safe textbuf.Buffer
	for index := 0; index < len(value); {
		if value[index] == '\\' && index+1 < len(value) &&
			strings.ContainsRune(`<>&`, rune(value[index+1])) {
			safe.Byte('\\')
			safe.Str(html.EscapeString(value[index+1 : index+2]))
			index += 2
			continue
		}
		if value[index] == '`' {
			_, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
			if closed {
				end := len(value) - len(suffix)
				safe.Str(html.EscapeString(value[index:end]))
				index = end
				continue
			}
		}
		safe.Byte(value[index])
		index++
	}
	return safe.String()
}

func markdownInlineVisibleTextNoHTML(value string) string {
	var rendered textbuf.Buffer
	emphasisMarkers := markdownEmphasisMarkers(value)
	for index := 0; index < len(value); {
		switch value[index] {
		case '\\':
			if index+1 < len(value) &&
				isCommonMarkASCIIPunctuation(value[index+1]) {
				rendered.Byte(value[index+1])
				index += 2
				continue
			}
		case '`':
			content, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
			if closed {
				rendered.Str(content)
				index = len(value) - len(suffix)
				continue
			}
		case '[', '!':
			labelAt := index
			if value[index] == '!' {
				if index+1 >= len(value) || value[index+1] != '[' {
					break
				}
				labelAt++
			}
			labelEnd := markdownInlineClosingBracket(value, labelAt+1)
			if labelEnd != -1 {
				suffixEnd := markdownLinkSuffixEnd(value, labelEnd+1)
				if suffixEnd != -1 {
					rendered.Str(markdownInlineVisibleText(
						value[labelAt+1 : labelEnd],
					))
					index = suffixEnd
					continue
				}
			}
		case '*', '_':
			runEnd := index + 1
			for runEnd < len(value) && value[runEnd] == value[index] {
				runEnd++
			}
			for markerAt := index; markerAt < runEnd; markerAt++ {
				if !emphasisMarkers[markerAt] {
					rendered.Byte(value[markerAt])
				}
			}
			index = runEnd
			continue
		}
		rendered.Byte(value[index])
		index++
	}
	return rendered.String()
}

func markdownDelimiterFlanking(
	value string,
	start, end int,
	marker byte,
) (canOpen, canClose bool) {
	beforeSpace, beforePunctuation := true, false
	if start != 0 {
		previous, _ := utf8.DecodeLastRuneInString(value[:start])
		beforeSpace = unicode.IsSpace(previous)
		beforePunctuation = markdownIsPunctuation(previous)
	}
	afterSpace, afterPunctuation := true, false
	if end != len(value) {
		next, _ := utf8.DecodeRuneInString(value[end:])
		afterSpace = unicode.IsSpace(next)
		afterPunctuation = markdownIsPunctuation(next)
	}
	leftFlanking := !afterSpace &&
		(!afterPunctuation || beforeSpace || beforePunctuation)
	rightFlanking := !beforeSpace &&
		(!beforePunctuation || afterSpace || afterPunctuation)
	if marker == '_' {
		return leftFlanking && (!rightFlanking || beforePunctuation),
			rightFlanking && (!leftFlanking || afterPunctuation)
	}
	return leftFlanking, rightFlanking
}

func markdownIsPunctuation(value rune) bool {
	return (value <= unicode.MaxASCII &&
		isCommonMarkASCIIPunctuation(byte(value))) ||
		unicode.IsPunct(value)
}

type markdownEmphasisDelimiter struct {
	marker              byte
	start, end          int
	leftUsed, rightUsed int
	canOpen, canClose   bool
	eligible            bool
}

func markdownEmphasisMarkers(value string) []bool {
	excluded := make([]bool, len(value))
	for index := 0; index < len(value); {
		switch value[index] {
		case '\\':
			if index+1 < len(value) {
				excluded[index+1] = true
			}
			index += min(2, len(value)-index)
			continue
		case '`':
			_, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
			if closed {
				end := len(value) - len(suffix)
				for offset := range end - index {
					excluded[index+offset] = true
				}
				index = end
				continue
			}
		case '[', '!':
			labelAt := index
			if value[index] == '!' {
				if index+1 >= len(value) || value[index+1] != '[' {
					break
				}
				labelAt++
			}
			labelEnd := markdownInlineClosingBracket(value, labelAt+1)
			if labelEnd != -1 {
				suffixEnd := markdownLinkSuffixEnd(value, labelEnd+1)
				if suffixEnd != -1 {
					for offset := range suffixEnd - labelEnd - 1 {
						excluded[labelEnd+1+offset] = true
					}
				}
			}
		}
		index++
	}

	var delimiters []markdownEmphasisDelimiter
	for index := 0; index < len(value); {
		if excluded[index] || value[index] != '*' && value[index] != '_' {
			index++
			continue
		}
		end := index + 1
		for end < len(value) && !excluded[end] && value[end] == value[index] {
			end++
		}
		canOpen, canClose := markdownDelimiterFlanking(
			value, index, end, value[index],
		)
		delimiters = append(delimiters, markdownEmphasisDelimiter{
			marker: value[index], start: index, end: end,
			canOpen: canOpen, canClose: canClose, eligible: true,
		})
		index = end
	}

	matched := make([]bool, len(value))
	for closerAt := range delimiters {
		closer := &delimiters[closerAt]
		for closer.eligible &&
			closer.end-closer.start-closer.leftUsed-closer.rightUsed > 0 &&
			closer.canClose {
			openerAt := -1
			for candidateAt := closerAt - 1; candidateAt >= 0; candidateAt-- {
				opener := &delimiters[candidateAt]
				openWidth := opener.end - opener.start -
					opener.leftUsed - opener.rightUsed
				closeWidth := closer.end - closer.start -
					closer.leftUsed - closer.rightUsed
				if !opener.eligible || opener.marker != closer.marker ||
					!opener.canOpen || openWidth == 0 {
					continue
				}
				ruleOfThreeBlocks := (opener.canClose || closer.canOpen) &&
					(openWidth+closeWidth)%3 == 0 &&
					(openWidth%3 != 0 || closeWidth%3 != 0)
				if !ruleOfThreeBlocks {
					openerAt = candidateAt
					break
				}
			}
			if openerAt == -1 {
				break
			}
			opener := &delimiters[openerAt]
			openWidth := opener.end - opener.start -
				opener.leftUsed - opener.rightUsed
			closeWidth := closer.end - closer.start -
				closer.leftUsed - closer.rightUsed
			used := 1
			if openWidth >= 2 && closeWidth >= 2 {
				used = 2
			}
			openStart := opener.end - opener.rightUsed - used
			closeStart := closer.start + closer.leftUsed
			for offset := range used {
				matched[openStart+offset] = true
				matched[closeStart+offset] = true
			}
			opener.rightUsed += used
			closer.leftUsed += used
			for delimiterAt := openerAt + 1; delimiterAt < closerAt; delimiterAt++ {
				delimiters[delimiterAt].eligible = false
			}
		}
	}
	return matched
}

func markdownInlineClosingBracket(value string, start int) int {
	depth := 0
	for index := start; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		switch value[index] {
		case '[':
			depth++
		case ']':
			if depth == 0 {
				return index
			}
			depth--
		}
	}
	return -1
}

func markdownLinkSuffixEnd(value string, start int) int {
	if start >= len(value) {
		return -1
	}
	var opener, closer byte
	switch value[start] {
	case '(':
		opener, closer = '(', ')'
	case '[':
		opener, closer = '[', ']'
	default:
		return -1
	}
	depth := 0
	for index := start + 1; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		switch value[index] {
		case opener:
			depth++
		case closer:
			if depth == 0 {
				return index + 1
			}
			depth--
		}
	}
	return -1
}

func markdownLineRange(lines []markdownLine, start, end int) string {
	text := make([]string, 0, end-start)
	for _, line := range lines[start:end] {
		text = append(text, line.text)
	}
	return strings.Join(text, "\n")
}

func wikiCommandDetail(content, path string) (string, int) {
	lines := scanMarkdownLines(content)
	canonicalAt := -1
	count := 0
	malformed := 0
	for index, line := range lines {
		if !line.active || line.headingLevel == 0 {
			continue
		}
		candidate, suffix, _, closed := markdownCodeSpanPrefix(line.heading)
		if candidate != path {
			continue
		}
		if line.headingLevel == 3 && closed && suffix == "" {
			canonicalAt = index
			count++
		} else {
			malformed++
		}
	}
	if count == 0 {
		return "", malformed
	}
	if count != 1 || malformed != 0 {
		return "", count + malformed
	}
	end := len(lines)
	for index := canonicalAt + 1; index < len(lines); index++ {
		if lines[index].active && lines[index].headingLevel != 0 &&
			lines[index].headingLevel <= 3 {
			end = index
			break
		}
	}
	return markdownLineRange(lines, canonicalAt+1, end), count
}

func wikiCommandSummaryPaths(content string) []string {
	var paths []string
	inTable := false
	pastSeparator := false
	for _, scanned := range scanMarkdownLines(content) {
		if !scanned.active {
			continue
		}
		line := scanned.text
		switch {
		case line == "| Command | Mode | Description |":
			inTable = true
			pastSeparator = false
		case inTable && !pastSeparator:
			pastSeparator = strings.HasPrefix(line, "|---")
		case inTable && !strings.HasPrefix(line, "| "):
			inTable = false
		case inTable:
			if path, ok := markdownFirstCodeCell(line); ok {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func wikiCommandDetailPaths(content string) []string {
	var paths []string
	for _, line := range scanMarkdownLines(content) {
		if !line.active || line.headingLevel != 3 {
			continue
		}
		if path, ok := markdownCodeSpan(line.heading); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

func markdownCodeSpan(value string) (string, bool) {
	content, suffix, _, closed := markdownCodeSpanPrefix(value)
	return content, closed && suffix == ""
}

// markdownCodeSpanPrefix reads one CommonMark code span. A closing run must
// have exactly the opening run's width; shorter and longer runs are content.
func markdownCodeSpanPrefix(value string) (string, string, int, bool) {
	if value == "" || value[0] != '`' {
		return "", value, 0, false
	}
	width := 0
	for width < len(value) && value[width] == '`' {
		width++
	}
	for index := width; index < len(value); {
		if value[index] != '`' {
			index++
			continue
		}
		end := index
		for end < len(value) && value[end] == '`' {
			end++
		}
		if end-index == width {
			return normalizeMarkdownCodeSpan(value[width:index]), value[end:], width, true
		}
		index = end
	}
	return normalizeMarkdownCodeSpan(value[width:]), "", width, false
}

func normalizeMarkdownCodeSpan(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	if len(value) >= 2 && value[0] == ' ' && value[len(value)-1] == ' ' &&
		strings.Trim(value, " ") != "" {
		return value[1 : len(value)-1]
	}
	return value
}

func markdownFirstCodeCell(line string) (string, bool) {
	path, closed, canonical := markdownTableCodeCell(line)
	return path, closed && canonical
}

func wikiArgumentRows(content string) []string {
	lines := activeMarkdownLines(content)
	for index, line := range lines {
		if line != "| Name | Type | Required | Values |" {
			continue
		}
		var rows []string
		for _, row := range lines[index+2:] {
			if !strings.HasPrefix(row, "| ") {
				break
			}
			rows = append(rows, row)
		}
		return rows
	}
	return nil
}

func wikiBulletGroups(content, heading string) [][]string {
	lines := activeMarkdownLines(content)
	var groups [][]string
	for index, line := range lines {
		if line != heading {
			continue
		}
		var rows []string
		for _, row := range lines[index+1:] {
			if !strings.HasPrefix(row, "- ") {
				break
			}
			rows = append(rows, row)
		}
		groups = append(groups, rows)
	}
	return groups
}

func wikiLineEquals(content, want string) bool {
	for _, line := range scanMarkdownLines(content) {
		if line.active && line.text == want {
			return true
		}
	}
	return false
}

// compareWikiOptionalLine reports the wiki detail lines that start with prefix
// when they disagree with expected. An absent line agrees with an empty
// expectation. The wiki renderer is the only producer of these lines, so the
// issue is always filed against it.
func compareWikiOptionalLine(
	command, content, prefix, field, expected string,
) []Issue {
	var actual []string
	for _, line := range scanMarkdownLines(content) {
		if line.active && strings.HasPrefix(line.text, prefix) {
			actual = append(actual, line.text)
		}
	}
	if len(actual) <= 1 && (len(actual) == 0 && expected == "" ||
		len(actual) == 1 && actual[0] == expected) {
		return nil
	}
	return []Issue{generatedCommandSurfaceValueIssue(
		wikiCatalogRenderer, command, "wiki "+field, expected, strings.Join(actual, " | "),
	)}
}

func wikiCodeList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, markdownCodeLiteral(value))
	}
	return strings.Join(quoted, ", ")
}

func wikiTableCodeList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted,
			markdownCodeLiteral(commandMarkdownTableValue(value)))
	}
	return strings.Join(quoted, ", ")
}

type wikiPipeGroupScan struct {
	groups        map[string][][]string
	headers       int
	noSupport     int
	unknownLabels []string
}

func scanWikiPipeGroups(content string) wikiPipeGroupScan {
	scan := wikiPipeGroupScan{
		groups: make(map[string][][]string, len(commandOperatorAvailabilities)),
	}
	inCommandContract := false
	for _, line := range scanMarkdownLines(content) {
		if !line.active {
			continue
		}
		if strings.HasPrefix(line.text, "Mode: ") {
			inCommandContract = true
			continue
		}
		if !inCommandContract {
			continue
		}
		switch line.text {
		case "**Pipes:**":
			scan.headers++
			continue
		case "**Pipes:** not available":
			scan.noSupport++
			continue
		}
		label, candidate := wikiPipeGroupLabel(line.text)
		if !candidate {
			continue
		}
		availability := wikiOperatorAvailability(label)
		if availability == "" {
			scan.unknownLabels = append(scan.unknownLabels, label)
			continue
		}
		scan.groups[availability] = append(
			scan.groups[availability],
			wikiOperatorGroupNames(line.text, label),
		)
	}
	return scan
}

func wikiPipeGroupLabel(line string) (string, bool) {
	separator, valid := markdownInlineDelimiter(line, ": ")
	var rawLabel string
	switch {
	case valid && separator >= 0:
		rawLabel = line[:separator]
	case strings.HasPrefix(line, "**"):
		end := strings.Index(line[2:], "**")
		if end < 0 {
			return "", false
		}
		rawLabel = line[:2+end+2]
	case !valid:
		separator = strings.Index(line, ": ")
		if separator < 0 {
			return "", false
		}
		rawLabel = line[:separator]
	default:
		return "", false
	}
	label := strings.Join(strings.Fields(strings.TrimSuffix(
		markdownInlineVisibleText(rawLabel), ":",
	)), " ")
	candidate := commandOperatorLabelCandidate(label)
	if !candidate {
		candidate = malformedWikiOperatorLabelCandidate(rawLabel)
	}
	return label, candidate
}

func markdownInlineDelimiter(value, delimiter string) (int, bool) {
	for index := 0; index < len(value); {
		switch value[index] {
		case '\\':
			index += min(2, len(value)-index)
			continue
		case '`':
			_, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
			if !closed {
				return -1, false
			}
			index = len(value) - len(suffix)
			continue
		case '<':
			end, html := markdownInlineHTMLEnd(value, index)
			if html {
				if end < 0 {
					return -1, false
				}
				index = end
				continue
			}
		case ']':
			if index+1 < len(value) &&
				(value[index+1] == '(' || value[index+1] == '[') {
				end := markdownLinkSuffixEnd(value, index+1)
				if end < 0 {
					return -1, false
				}
				index = end
				continue
			}
		}
		if strings.HasPrefix(value[index:], delimiter) {
			return index, true
		}
		index++
	}
	return -1, true
}

func markdownInlineHTMLEnd(value string, start int) (int, bool) {
	switch {
	case strings.HasPrefix(value[start:], "<!--"):
		return markdownInlineHTMLTerminatorEnd(value, start+4, "-->")
	case strings.HasPrefix(value[start:], "<?"):
		return markdownInlineHTMLTerminatorEnd(value, start+2, "?>")
	case strings.HasPrefix(value[start:], "<![CDATA["):
		return markdownInlineHTMLTerminatorEnd(value, start+9, "]]>")
	case start+2 < len(value) && value[start+1] == '!' &&
		markdownASCIIAlpha(value[start+2]):
		return markdownInlineHTMLTerminatorEnd(value, start+3, ">")
	default:
		return markdownInlineHTMLTagEnd(value, start)
	}
}

func markdownInlineHTMLTerminatorEnd(
	value string,
	contentStart int,
	terminator string,
) (int, bool) {
	end := strings.Index(value[contentStart:], terminator)
	if end < 0 {
		return -1, true
	}
	return contentStart + end + len(terminator), true
}

func markdownInlineHTMLTagEnd(value string, start int) (int, bool) {
	index := start + 1
	if index < len(value) && value[index] == '/' {
		index++
	}
	if index >= len(value) || !markdownASCIIAlpha(value[index]) {
		return 0, false
	}
	for index < len(value) &&
		(markdownASCIIAlpha(value[index]) ||
			value[index] >= '0' && value[index] <= '9' ||
			value[index] == '-') {
		index++
	}
	if index < len(value) &&
		!strings.ContainsRune(" \t\r\n/>", rune(value[index])) {
		return 0, false
	}
	var quote byte
	for index < len(value) {
		if quote != 0 {
			if value[index] == quote {
				quote = 0
			}
			index++
			continue
		}
		switch value[index] {
		case '\'', '"':
			quote = value[index]
		case '>':
			return index + 1, true
		}
		index++
	}
	return -1, true
}

func markdownASCIIAlpha(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

// CommonMark renders unmatched openers literally. Ignore only a leading inline
// wrapper opener when deciding whether a malformed label must be reported.
func malformedWikiOperatorLabelCandidate(rawLabel string) bool {
	label := strings.TrimLeft(strings.TrimSpace(rawLabel), "*_`[")
	if commandOperatorLabelCandidate(label) {
		return true
	}
	switch {
	case strings.HasPrefix(label, "<!--"):
		label = label[4:]
	case strings.HasPrefix(label, "<?"):
		label = label[2:]
	case strings.HasPrefix(label, "<![CDATA["):
		label = label[9:]
	case strings.HasPrefix(label, "<!"):
		label = label[2:]
	}
	if commandOperatorLabelCandidate(label) {
		return true
	}
	if strings.HasPrefix(label, "<") {
		if end := strings.IndexByte(label, '>'); end >= 0 {
			return commandOperatorLabelCandidate(label[end+1:])
		}
	}
	return false
}

func wikiOperatorAvailability(label string) string {
	switch label {
	case alwaysLabel:
		return availabilityAlways
	case "When the answer has rows":
		return availabilityWithRows
	case "While the command keeps answering":
		return availabilityWhenStreaming
	case localOnlyLabel:
		return availabilityLocalOnly
	default:
		return ""
	}
}

func wikiOperatorGroupNames(line, label string) []string {
	values := strings.TrimSpace(strings.TrimPrefix(line, label+":"))
	parts, valid := splitMarkdownOutsideCode(values, " -- ")
	if valid && len(parts) <= 2 {
		values = parts[0]
	} else {
		valid = false
	}
	var tokens []string
	if valid && values != "" {
		tokens, valid = splitMarkdownOutsideCode(values, ", ")
	}
	var names []string
	if valid {
		for _, token := range tokens {
			name, suffix, _, closed := markdownCodeSpanPrefix(token)
			if !closed || suffix != "" {
				valid = false
				break
			}
			names = append(names, name)
		}
	}
	if !valid || len(names) == 0 {
		return []string{values}
	}
	return names
}

func validateWikiPipeSupport(
	path, command string,
	scan wikiPipeGroupScan,
	hasSupport bool,
) []Issue {
	var issues []Issue
	for _, label := range scan.unknownLabels {
		issues = append(issues,
			unknownCommandOperatorGroupLabelIssue(path, command, label))
	}
	expectedHeaders, expectedNoSupport := 0, 1
	if hasSupport {
		expectedHeaders, expectedNoSupport = 1, 0
	}
	if scan.headers != expectedHeaders {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command, "wiki pipe support header",
			strconv.Itoa(expectedHeaders), strconv.Itoa(scan.headers),
		))
	}
	if scan.noSupport != expectedNoSupport {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command, "wiki no-support verdict",
			strconv.Itoa(expectedNoSupport), strconv.Itoa(scan.noSupport),
		))
	}
	return issues
}

func activeMarkdownLines(content string) []string {
	var active []string
	for _, line := range scanMarkdownLines(content) {
		if line.active {
			active = append(active, line.text)
		}
	}
	return active
}

var commandSurfaceSlugSeparator = regexp.MustCompile(`[^a-z0-9]+`)

func validateGeneratedCommandSurfaces(
	root, expectedRoot string,
	live []publishedCommand,
) []Issue {
	primaryHTMLPath := filepath.Join(expectedRoot, "reference", "cli", "index.html")
	primaryMarkdownPath := filepath.Join(expectedRoot, "reference", "cli", "index.md")
	llmsPath := filepath.Join(expectedRoot, llmsSurfaceName)
	equivalentHTMLPath := filepath.Join(
		expectedRoot, "reference", "command-equivalents", "index.html",
	)
	equivalentMarkdownPath := filepath.Join(
		expectedRoot, "reference", "command-equivalents", "index.md",
	)
	primaryHTML, err := os.ReadFile(primaryHTMLPath) //nolint:gosec // isolated renderer output
	if err != nil {
		return []Issue{generatedCommandSurfaceReadIssue(root, primaryHTMLPath, err)}
	}
	primaryMarkdown, err := os.ReadFile(primaryMarkdownPath) //nolint:gosec // isolated renderer output
	if err != nil {
		return []Issue{generatedCommandSurfaceReadIssue(root, primaryMarkdownPath, err)}
	}
	llms, err := os.ReadFile(llmsPath) //nolint:gosec // isolated renderer output
	if err != nil {
		return []Issue{generatedCommandSurfaceReadIssue(root, llmsPath, err)}
	}
	equivalentHTML, err := os.ReadFile(equivalentHTMLPath) //nolint:gosec // rendered command surface
	if err != nil {
		return []Issue{generatedCommandSurfaceReadIssue(root, equivalentHTMLPath, err)}
	}
	equivalentMarkdown, err := os.ReadFile(equivalentMarkdownPath) //nolint:gosec // rendered command surface
	if err != nil {
		return []Issue{generatedCommandSurfaceReadIssue(root, equivalentMarkdownPath, err)}
	}

	primaryHTMLDocument := parseRenderedHTML(string(primaryHTML))
	llmsCommandSurface := llmsCommandSurfaceContent(string(llms))

	var issues []Issue
	issues = append(issues, validatePrimaryHTMLIdentities(
		commandSurfacePath(root, primaryHTMLPath),
		"primary CLI HTML command row",
		live,
		primaryHTMLCommandIdentities(primaryHTMLDocument),
	)...)
	issues = append(issues, validateAggregateCommandIdentities(
		commandSurfacePath(root, primaryMarkdownPath),
		"primary CLI Markdown command row",
		live,
		primaryMarkdownCommandIdentities(string(primaryMarkdown)),
	)...)
	issues = append(issues, validateAggregateCommandIdentities(
		commandSurfacePath(root, llmsPath),
		"llms.txt command metadata row",
		live,
		llmsCommandIdentities(llmsCommandSurface),
	)...)
	equivalentHTMLDocument := parseRenderedHTML(string(equivalentHTML))
	issues = append(issues, validateEquivalentIndexIdentities(
		commandSurfacePath(root, equivalentHTMLPath),
		"command-equivalent HTML index row",
		live,
		equivalentHTMLCommandIdentities(equivalentHTMLDocument),
	)...)
	issues = append(issues, validateEquivalentIndexIdentities(
		commandSurfacePath(root, equivalentMarkdownPath),
		"command-equivalent Markdown index row",
		live,
		equivalentMarkdownCommandIdentities(string(equivalentMarkdown)),
	)...)
	for index := range live {
		command := &live[index]
		slug := commandSurfaceSlug(command.Path)
		primaryRow, rowCount, rowClosed := commandSurfaceHTMLRow(
			primaryHTMLDocument, command.Path,
		)
		switch {
		case rowCount != 1:
			issues = append(issues, commandContainerCountIssue(
				commandSurfacePath(root, primaryHTMLPath), command.Path,
				"primary CLI HTML command row", rowCount,
			))
		case !rowClosed:
			issues = append(issues, malformedCommandContainerIssue(
				commandSurfacePath(root, primaryHTMLPath), command.Path,
				"primary CLI HTML command row",
			))
		default:
			issues = append(issues, validatePrimaryCommandContract(
				commandSurfacePath(root, primaryHTMLPath),
				primaryRow,
				primaryHTMLDocument,
				command,
			)...)
		}

		primaryMarkdownRow, markdownRowCount, markdownRowMalformed :=
			commandSurfaceMarkdownRow(string(primaryMarkdown), command.Path)
		switch {
		case markdownRowCount != 1:
			issues = append(issues, commandContainerCountIssue(
				commandSurfacePath(root, primaryMarkdownPath), command.Path,
				"primary CLI Markdown command row", markdownRowCount,
			))
		case markdownRowMalformed:
			issues = append(issues, malformedCommandContainerIssue(
				commandSurfacePath(root, primaryMarkdownPath), command.Path,
				"primary CLI Markdown command row",
			))
		default:
			issues = append(issues, validatePrimaryMarkdownContract(
				commandSurfacePath(root, primaryMarkdownPath),
				primaryMarkdownRow,
				command,
			)...)
		}

		detailHTMLPath := filepath.Join(
			expectedRoot, "reference", "command-equivalents", slug, "index.html",
		)
		detailMarkdownPath := filepath.Join(
			expectedRoot, "reference", "command-equivalents", slug, "index.md",
		)
		detailHTML, detailHTMLErr := os.ReadFile(detailHTMLPath) //nolint:gosec // isolated renderer output
		if detailHTMLErr != nil {
			issues = append(issues, generatedCommandSurfaceReadIssue(
				root, detailHTMLPath, detailHTMLErr,
			))
		} else {
			issues = append(issues, validateEquivalentCommandContract(
				commandSurfacePath(root, detailHTMLPath),
				parseRenderedHTML(string(detailHTML)),
				command,
			)...)
		}
		detailMarkdown, detailMarkdownErr := os.ReadFile(detailMarkdownPath) //nolint:gosec // isolated renderer output
		if detailMarkdownErr != nil {
			issues = append(issues, generatedCommandSurfaceReadIssue(
				root, detailMarkdownPath, detailMarkdownErr,
			))
		} else {
			issues = append(issues, validateEquivalentMarkdownContract(
				commandSurfacePath(root, detailMarkdownPath),
				string(detailMarkdown),
				command,
			)...)
		}

		identity, meta, description, metadataCount, metadataClosed :=
			llmsCommandMetadata(llmsCommandSurface, command.Path)
		switch {
		case metadataCount != 1:
			issues = append(issues, commandContainerCountIssue(
				commandSurfacePath(root, llmsPath), command.Path,
				"llms.txt command metadata row", metadataCount,
			))
		case !metadataClosed:
			issues = append(issues, malformedCommandContainerIssue(
				commandSurfacePath(root, llmsPath), command.Path,
				"llms.txt command metadata row",
			))
		default:
			issues = append(issues,
				validateLLMSCommandContract(
					commandSurfacePath(root, llmsPath),
					identity, meta, description, command,
				)...)
		}
	}
	issues = append(issues, validatePrimaryOperatorCatalog(
		commandSurfacePath(root, primaryHTMLPath), string(primaryHTML), live,
	)...)
	return issues
}

type renderedPrimaryHTMLIdentity struct {
	path  string
	valid bool
}

func primaryHTMLCommandIdentities(
	document renderedHTMLDocument,
) []renderedPrimaryHTMLIdentity {
	identities := make([]renderedPrimaryHTMLIdentity, 0, len(document.htmlRows))
	for _, container := range document.htmlRows {
		identity, candidate, identityValid := primaryHTMLRowIdentity(container.root)
		if !candidate {
			continue
		}
		slug := commandSurfaceSlug(identity)
		identities = append(identities, renderedPrimaryHTMLIdentity{
			path: identity,
			valid: identityValid && slug != "" &&
				primaryHTMLCanonicalID(container.root, "cmd-"+slug),
		})
	}
	return identities
}

func primaryHTMLCanonicalID(row *xhtml.Node, expected string) bool {
	count := 0
	matches := false
	for _, attribute := range row.Attr {
		if attribute.Key != "id" {
			continue
		}
		count++
		matches = attribute.Val == expected
	}
	return count == 1 && matches
}

func primaryHTMLRowIdentity(row *xhtml.Node) (string, bool, bool) {
	for parent := row.Parent; parent != nil; parent = parent.Parent {
		if parent.Data == "section" && htmlHasClass(parent, "cli-pipe-guide") {
			return "", false, false
		}
	}
	idCandidate := strings.HasPrefix(htmlAttribute(row, "id"), "cmd-")
	var cells []*xhtml.Node
	for child := row.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && child.Data == "td" {
			cells = append(cells, child)
		}
	}
	if len(cells) == 0 {
		return "", idCandidate, false
	}
	var codes []*xhtml.Node
	htmlWalk(cells[0], func(node *xhtml.Node) {
		if node.Data == codeElement {
			codes = append(codes, node)
		}
	})
	if len(codes) == 0 {
		return "", idCandidate, false
	}
	identity := normalizeRenderedHTMLText(htmlText(codes[0]))
	valid := len(cells) == 4 && len(codes) == 1 && identity != "" &&
		normalizeRenderedHTMLText(htmlText(cells[0])) == identity
	return identity, true, valid
}

func primaryMarkdownCommandIdentities(content string) []string {
	var identities []string
	for _, line := range scanMarkdownLines(content) {
		if !line.active {
			continue
		}
		cells, valid := markdownTableCells(line.text)
		if !valid {
			trimmed := strings.TrimLeft(line.text, " ")
			if len(line.text)-len(trimmed) <= 3 && strings.HasPrefix(trimmed, "|") {
				firstCell := strings.TrimLeft(trimmed[1:], " ")
				if separator := strings.Index(firstCell, "|"); separator != -1 {
					firstCell = firstCell[:separator]
				}
				if strings.Contains(firstCell, "`") {
					identities = append(identities, "")
				}
			}
			continue
		}
		if len(cells) == 0 {
			continue
		}
		path, wrapped, candidate := markdownWrappedCodeSpan(strings.TrimSpace(cells[0]))
		if !candidate {
			continue
		}
		if !wrapped {
			path = ""
		}
		identities = append(identities, path)
	}
	return identities
}

type renderedCommandIndexIdentity struct {
	path  string
	slug  string
	valid bool
}

func equivalentHTMLCommandIdentities(
	document renderedHTMLDocument,
) []renderedCommandIndexIdentity {
	ids := make([]string, 0, len(document.rows))
	for id := range document.rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var identities []renderedCommandIndexIdentity
	for _, id := range ids {
		slug, idValid := strings.CutPrefix(id, "cmd-eq-")
		idValid = idValid && slug != ""
		for _, container := range document.rows[id] {
			var cells []*xhtml.Node
			for child := container.root.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == xhtml.ElementNode && child.Data == "td" {
					cells = append(cells, child)
				}
			}
			var codes []*xhtml.Node
			if len(cells) != 0 {
				htmlWalk(cells[0], func(node *xhtml.Node) {
					if node.Data == codeElement {
						codes = append(codes, node)
					}
				})
			}
			identity := renderedCommandIndexIdentity{slug: slug}
			if len(codes) == 1 {
				identity.path = normalizeRenderedHTMLText(htmlText(codes[0]))
			}
			identity.valid = document.err == nil && idValid && container.closed &&
				len(cells) != 0 && len(codes) == 1 &&
				htmlVisibleSubtreeClosed(document, container.root) &&
				normalizeRenderedHTMLText(htmlText(cells[0])) == identity.path
			identities = append(identities, identity)
		}
	}
	return identities
}

func equivalentMarkdownCommandIdentities(content string) []renderedCommandIndexIdentity {
	var identities []renderedCommandIndexIdentity
	for _, line := range scanMarkdownLines(content) {
		if !line.active {
			continue
		}
		if identity, candidate := equivalentMarkdownIndexIdentity(line.text); candidate {
			identities = append(identities, identity)
		}
	}
	return identities
}

func equivalentMarkdownIndexIdentity(line string) (renderedCommandIndexIdentity, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return renderedCommandIndexIdentity{}, false
	}
	if strings.HasPrefix(trimmed, "- [") {
		labelEnd := markdownInlineClosingBracket(trimmed, 3)
		if labelEnd == -1 {
			return renderedCommandIndexIdentity{}, strings.Contains(trimmed, "`")
		}
		path, pathValid := markdownCodeSpan(trimmed[3:labelEnd])
		destination, destinationEnd, destinationValid :=
			markdownInlineLinkDestination(trimmed, labelEnd+1)
		return renderedCommandIndexIdentity{
			path: path,
			slug: strings.TrimSuffix(destination, "/"),
			valid: pathValid && destinationValid && destinationEnd == len(trimmed) &&
				destination != "" && strings.HasSuffix(destination, "/"),
		}, true
	}
	if !strings.HasPrefix(trimmed, "|") {
		return renderedCommandIndexIdentity{}, false
	}
	cells, cellsValid := markdownTableCells(trimmed)
	if len(cells) == 0 {
		return renderedCommandIndexIdentity{}, false
	}
	path, pathValid, candidate := markdownWrappedCodeSpan(strings.TrimSpace(cells[0]))
	if !candidate {
		return renderedCommandIndexIdentity{}, false
	}
	identity := renderedCommandIndexIdentity{path: path}
	if !cellsValid || len(cells) < 2 {
		return identity, true
	}
	detail := strings.TrimSpace(cells[len(cells)-1])
	labelEnd := markdownInlineClosingBracket(detail, 1)
	if !strings.HasPrefix(detail, "[") || labelEnd == -1 {
		return identity, true
	}
	destination, destinationEnd, destinationValid :=
		markdownInlineLinkDestination(detail, labelEnd+1)
	identity.slug = strings.TrimSuffix(destination, "/")
	identity.valid = pathValid && destinationValid && destinationEnd == len(detail) &&
		destination != "" && strings.HasSuffix(destination, "/")
	return identity, true
}

func markdownWrappedCodeSpan(value string) (string, bool, bool) {
	tokens, closed := markdownDetailTokens(value)
	path := ""
	codeCount := 0
	for _, token := range tokens {
		if token.code {
			path = token.value
			codeCount++
		}
	}
	candidate := codeCount != 0 || strings.Contains(value, "`")
	if !closed || codeCount != 1 {
		return path, false, candidate
	}
	return path, markdownCodeSpanWrapper(value), true
}

func markdownCodeSpanWrapper(value string) bool {
	if _, valid := markdownCodeSpan(value); valid {
		return true
	}
	for _, marker := range []string{"**", "__", "*", "_"} {
		if strings.HasPrefix(value, marker) && strings.HasSuffix(value, marker) &&
			len(value) > 2*len(marker) &&
			markdownCodeSpanWrapper(value[len(marker):len(value)-len(marker)]) {
			return true
		}
	}
	if !strings.HasPrefix(value, "[") {
		return false
	}
	labelEnd := markdownInlineClosingBracket(value, 1)
	if labelEnd == -1 {
		return false
	}
	suffixEnd := markdownLinkSuffixEnd(value, labelEnd+1)
	return suffixEnd == len(value) &&
		markdownCodeSpanWrapper(value[1:labelEnd])
}

func markdownWrappedCodeSpanPrefix(
	value string,
) (path, suffix string, valid, candidate bool) {
	candidate = strings.Contains(value, "`")
	if !candidate {
		return "", value, false, false
	}
	for end := 1; end <= len(value); end++ {
		path, valid, prefixCandidate := markdownWrappedCodeSpan(value[:end])
		if prefixCandidate && valid {
			return path, value[end:], true, true
		}
	}
	return "", value, false, true
}

func markdownInlineLinkDestination(value string, start int) (string, int, bool) {
	if start >= len(value) || value[start] != '(' {
		return "", start, false
	}
	end := markdownLinkSuffixEnd(value, start)
	if end == -1 {
		return "", len(value), false
	}
	destination := strings.TrimSpace(value[start+1 : end-1])
	if strings.HasPrefix(destination, "<") && strings.HasSuffix(destination, ">") {
		destination = destination[1 : len(destination)-1]
	}
	return destination, end, destination != ""
}

func validateEquivalentIndexIdentities(
	path, kind string,
	live []publishedCommand,
	identities []renderedCommandIndexIdentity,
) []Issue {
	expected := make(map[renderedCommandIndexIdentity]bool, len(live))
	for index := range live {
		command := &live[index]
		expected[renderedCommandIndexIdentity{
			path: command.Path,
			slug: commandSurfaceSlug(command.Path),
		}] = true
	}
	observed := make(map[renderedCommandIndexIdentity]int, len(identities))
	var issues []Issue
	for _, identity := range identities {
		key := renderedCommandIndexIdentity{path: identity.path, slug: identity.slug}
		if identity.valid {
			observed[key]++
		}
		if identity.valid && expected[key] {
			continue
		}
		rendered := malformedIdentity
		if identity.path != "" || identity.slug != "" {
			rendered = fmt.Sprintf("%s -> %s/", identity.path, identity.slug)
		}
		issues = append(issues, generatedCommandExtraIssue(
			path, rendered, kind+" identity absent from the live command catalog",
		))
	}
	for index := range live {
		command := &live[index]
		key := renderedCommandIndexIdentity{
			path: command.Path,
			slug: commandSurfaceSlug(command.Path),
		}
		if observed[key] != 1 {
			issues = append(issues, commandContainerCountIssue(
				path, command.Path, kind+" identity", observed[key],
			))
		}
	}
	return issues
}

func llmsCommandSurfaceContent(content string) string {
	section, count := markdownHeadingContent(content, "## CLI command surface")
	if count != 1 {
		return ""
	}
	return section
}

func llmsCommandIdentities(content string) []string {
	var identities []string
	for _, line := range scanMarkdownLines(content) {
		trimmed := strings.TrimLeft(line.text, " ")
		if !line.active || len(line.text)-len(trimmed) > 3 ||
			!strings.HasPrefix(trimmed, "-") {
			continue
		}
		item := strings.TrimLeft(trimmed[1:], " ")
		path, _, valid, candidate := markdownWrappedCodeSpanPrefix(item)
		if !candidate {
			continue
		}
		if !valid {
			path = ""
		}
		identities = append(identities, path)
	}
	return identities
}

func validatePrimaryHTMLIdentities(
	path, kind string,
	live []publishedCommand,
	identities []renderedPrimaryHTMLIdentity,
) []Issue {
	expected := make(map[string]bool, len(live))
	for index := range live {
		command := &live[index]
		expected[command.Path] = true
	}
	observed := make(map[string]int, len(identities))
	var issues []Issue
	for _, identity := range identities {
		if identity.path != "" {
			observed[identity.path]++
		}
		rendered := identity.path
		if rendered == "" {
			rendered = malformedIdentity
		}
		if !identity.valid {
			issues = append(issues, malformedPrimaryHTMLIdentityIssue(
				path, rendered, kind,
			))
		}
		if !expected[identity.path] {
			issues = append(issues, generatedCommandExtraIssue(
				path, rendered, kind+" absent from the live command catalog",
			))
		}
	}
	for index := range live {
		command := &live[index]
		if observed[command.Path] != 1 {
			issues = append(issues, commandContainerCountIssue(
				path, command.Path, kind+" identity", observed[command.Path],
			))
		}
	}
	return issues
}
func malformedPrimaryHTMLIdentityIssue(path, command, kind string) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface has a malformed command container",
		Detail: detail.Str("command ").Quoted(command).Byte(' ').Str(kind).
			Str(" does not have its canonical cmd-* row ID and identity cells").String(),
	}
}

func validateAggregateCommandIdentities(
	path, kind string,
	live []publishedCommand,
	identities []string,
) []Issue {
	expected := make(map[string]bool, len(live))
	for index := range live {
		command := &live[index]
		expected[command.Path] = true
	}
	observed := make(map[string]int, len(identities))
	var issues []Issue
	for _, identity := range identities {
		observed[identity]++
		if expected[identity] {
			continue
		}
		if identity == "" {
			identity = malformedIdentity
		}
		issues = append(issues, generatedCommandExtraIssue(
			path, identity, kind+" absent from the live command catalog",
		))
	}
	for index := range live {
		command := &live[index]
		if observed[command.Path] != 1 {
			issues = append(issues, commandContainerCountIssue(
				path, command.Path, kind+" identity", observed[command.Path],
			))
		}
	}
	return issues
}

func generatedCommandSurfaceReadIssue(root, path string, err error) Issue {
	return Issue{
		File:    commandSurfacePath(root, path),
		Message: "the canonical command renderer did not produce a required surface",
		Detail:  err.Error(),
	}
}

func generatedCommandContractIssue(path, command, dimension string) Issue {
	return Issue{
		File:    path,
		Message: "the generated per-command surface dropped part of the live command contract",
		Detail:  missingCommandDimension(command, dimension),
	}
}

func missingCommandDimension(command, dimension string) string {
	var detail textbuf.Buffer
	return detail.Str("command ").Quoted(command).Str(" is missing ").Str(dimension).String()
}

func commandContainerCountIssue(path, command, kind string, count int) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface does not have exactly one command container",
		Detail: detail.Str("command ").Quoted(command).Str(" has ").
			Int(int64(count)).Byte(' ').Str(kind).Str(" containers; expected exactly one").String(),
	}
}

func malformedCommandContainerIssue(path, command, kind string) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface has a malformed command container",
		Detail: detail.Str("command ").Quoted(command).Byte(' ').Str(kind).
			Str(" is missing its closing delimiter").String(),
	}
}

func availabilityCommandDimension(availability, name string) string {
	var dimension textbuf.Buffer
	return dimension.Str(availability).Str(" availability for operator ").
		Quoted(name).String()
}

func namedCommandDimension(kind, name string) string {
	var dimension textbuf.Buffer
	return dimension.Str(kind).Byte(' ').Quoted(name).String()
}

func commandOperatorNames(command *publishedCommand, availability string) []string {
	names := make([]string, 0, len(command.Operators))
	for _, operator := range command.Operators {
		if availability == availabilityLocalOnly {
			if operator.LocalOnly {
				names = append(names, operator.Name)
			}
			continue
		}
		if operator.Available == availability {
			names = append(names, operator.Name)
		}
	}
	return names
}

func compareCommandOperatorGroups(
	path, command, availability string,
	expected []string, actual [][]string,
) []Issue {
	if len(actual) > 1 {
		return []Issue{duplicateCommandOperatorGroupIssue(
			path, command, availability, len(actual),
		)}
	}
	if len(expected) == 0 && len(actual) != 0 {
		return []Issue{generatedCommandExtraIssue(
			path, command, availability+" operator availability group",
		)}
	}
	if len(actual) == 0 {
		return compareCommandOperatorGroup(
			path, command, availability, expected, nil,
		)
	}
	return compareCommandOperatorGroup(
		path, command, availability, expected, actual[0],
	)
}

func compareScannedCommandOperatorGroups(
	path, command, availability string,
	expected []string,
	scan commandHTMLGroupScan,
) []Issue {
	issues := compareCommandOperatorGroups(
		path, command, availability, expected, scan.groups,
	)
	if scan.malformed {
		issues = append(issues, malformedCommandOperatorGroupIssue(
			path, command, availability,
		))
	}
	return issues
}

func compareCommandOperatorGroup(
	path, command, availability string,
	expected, actual []string,
) []Issue {
	var issues []Issue
	var rendered textbuf.Buffer
	missing, extra := commandNameDifferences(expected, actual)
	for _, name := range missing {
		dimension := availabilityCommandDimension(availability, name)
		if availability == availabilityLocalOnly {
			dimension = namedCommandDimension(
				"local-only surface qualifier for operator", name,
			)
		}
		issues = append(issues, generatedCommandContractIssue(path, command, dimension))
	}
	for _, name := range extra {
		dimension := rendered.Reset().Str("catalog-absent operator in ").
			Str(availability).Str(" group").String()
		issues = append(issues, generatedCommandExtraIssue(
			path, command, namedCommandDimension(dimension, name),
		))
	}
	if len(issues) == 0 && !sameStrings(expected, actual) {
		dimension := rendered.Reset().Str(availability).Str(" operator order").String()
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command, dimension,
			strings.Join(expected, ", "), strings.Join(actual, ", "),
		))
	}
	return issues
}

func duplicateCommandOperatorGroupIssue(
	path, command, availability string,
	count int,
) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface has a duplicate operator availability group",
		Detail: detail.Str("command ").Quoted(command).Str(" has ").
			Int(int64(count)).Byte(' ').Str(availability).
			Str(" operator groups; expected at most one").String(),
	}
}

func malformedCommandOperatorGroupIssue(path, command, availability string) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface has a malformed operator availability group",
		Detail: detail.Str("command ").Quoted(command).Byte(' ').Str(availability).
			Str(" operator group is missing its closing delimiter").String(),
	}
}

func compareCommandNamedGroups(
	path, command, kind string,
	expected []string, actual [][]string,
) []Issue {
	if len(actual) > 1 {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, kind+" cardinality", "at most one",
			fmt.Sprintf("%d labeled values", len(actual)),
		)}
	}
	if len(actual) == 0 {
		return compareCommandNamedGroup(path, command, kind, expected, nil)
	}
	if len(expected) == 0 {
		return []Issue{generatedCommandExtraIssue(path, command, kind)}
	}
	return compareCommandNamedGroup(path, command, kind, expected, actual[0])
}

func compareCommandNamedGroup(
	path, command, kind string,
	expected, actual []string,
) []Issue {
	missing, extra := commandNameDifferences(expected, actual)
	issues := make([]Issue, 0, len(missing)+len(extra))
	var rendered textbuf.Buffer
	for _, name := range missing {
		issues = append(issues, generatedCommandContractIssue(
			path, command, namedCommandDimension(kind, name),
		))
	}
	for _, name := range extra {
		dimension := rendered.Reset().Str("catalog-absent ").Str(kind).String()
		issues = append(issues, generatedCommandExtraIssue(
			path, command, namedCommandDimension(dimension, name),
		))
	}
	return issues
}

func commandNameDifferences(expected, actual []string) (missing, extra []string) {
	expectedCounts := make(map[string]int, len(expected))
	actualCounts := make(map[string]int, len(actual))
	for _, name := range expected {
		expectedCounts[name]++
	}
	for _, name := range actual {
		actualCounts[name]++
	}
	for name, count := range expectedCounts {
		difference := count - actualCounts[name]
		if difference <= 0 {
			continue
		}
		for range difference {
			missing = append(missing, name)
		}
	}
	for name, count := range actualCounts {
		difference := count - expectedCounts[name]
		if difference <= 0 {
			continue
		}
		for range difference {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

func generatedCommandExtraIssue(path, command, dimension string) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface publishes data absent from the live command contract",
		Detail: detail.Str("command ").Quoted(command).Str(" has extra ").
			Str(dimension).String(),
	}
}

type renderedOperatorMetadata struct {
	class       string
	description string
	available   []string
}

type renderedOperatorRow struct {
	values [4]string
	valid  bool
}

func walkOperatorHTML(root *xhtml.Node, visit func(*xhtml.Node)) {
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			visit(node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
}

func operatorHTMLSubtreeClosed(
	document renderedHTMLDocument,
	root *xhtml.Node,
) bool {
	closed := true
	walkOperatorHTML(root, func(node *xhtml.Node) {
		closed = closed && document.nodeClosed[node]
	})
	return closed
}

func primaryOperatorRows(document renderedHTMLDocument) ([]renderedOperatorRow, bool) {
	var guides []*xhtml.Node
	walkOperatorHTML(document.root, func(node *xhtml.Node) {
		if node.Data == "section" && htmlHasClass(node, "cli-pipe-guide") {
			guides = append(guides, node)
		}
	})
	if document.err != nil || len(guides) != 1 ||
		!operatorHTMLSubtreeClosed(document, guides[0]) {
		return nil, false
	}
	var rows []renderedOperatorRow
	walkOperatorHTML(guides[0], func(node *xhtml.Node) {
		if node == guides[0] || node.Data != "tr" {
			return
		}
		var cells []*xhtml.Node
		valid := operatorHTMLSubtreeClosed(document, node)
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			switch {
			case child.Type == xhtml.ElementNode && child.Data == "td":
				cells = append(cells, child)
			case child.Type == xhtml.TextNode && strings.TrimSpace(child.Data) == "":
			default:
				valid = false
			}
		}
		row := renderedOperatorRow{valid: valid && len(cells) == 4}
		for index := range min(len(cells), len(row.values)) {
			row.values[index] = normalizeRenderedHTMLText(htmlText(cells[index]))
		}
		rows = append(rows, row)
	})
	return rows, true
}

func validatePrimaryOperatorCatalog(
	path, content string,
	live []publishedCommand,
) []Issue {
	expected := make(map[string]renderedOperatorMetadata)
	for index := range live {
		command := &live[index]
		for _, operator := range command.Operators {
			meta, exists := expected[operator.Name]
			if !exists {
				meta.class = primaryOperatorClassLabel(operator.Class)
				meta.description = operator.Description
			}
			availability := commandAvailabilityLabel(operator.Available)
			if !slices.Contains(meta.available, availability) {
				meta.available = append(meta.available, availability)
			}
			if operator.LocalOnly && !slices.Contains(meta.available, localOnlyLabel) {
				meta.available = append(meta.available, localOnlyLabel)
			}
			expected[operator.Name] = meta
		}
	}

	rows, guideValid := primaryOperatorRows(parseRenderedHTML(content))
	actual := make(map[string]renderedOperatorMetadata)
	var duplicate []string
	var malformed int
	for _, row := range rows {
		if !row.valid || row.values[0] == "" {
			malformed++
			continue
		}
		name := row.values[0]
		if _, exists := actual[name]; exists {
			duplicate = append(duplicate, name)
			continue
		}
		actual[name] = renderedOperatorMetadata{
			class:       row.values[1],
			available:   splitNonEmpty(row.values[2], ", "),
			description: row.values[3],
		}
	}

	expectedNames := make([]string, 0, len(expected))
	actualNames := make([]string, 0, len(actual)+len(duplicate)+malformed)
	for name := range expected {
		expectedNames = append(expectedNames, name)
	}
	for name := range actual {
		actualNames = append(actualNames, name)
	}
	actualNames = append(actualNames, duplicate...)
	for range malformed {
		actualNames = append(actualNames, malformedIdentity)
	}
	if !guideValid {
		actualNames = append(actualNames, "<malformed operator guide>")
	}
	var issues []Issue
	missing, extra := commandNameDifferences(expectedNames, actualNames)
	for _, name := range missing {
		issues = append(issues, generatedCommandContractIssue(
			path, "<operator catalog>", namedCommandDimension("operator metadata", name),
		))
	}
	for _, name := range extra {
		issues = append(issues, generatedCommandExtraIssue(
			path, "<operator catalog>", namedCommandDimension("catalog-absent operator", name),
		))
	}
	for name, want := range expected {
		got, exists := actual[name]
		if !exists {
			continue
		}
		if got.class != want.class {
			issues = append(issues, generatedCommandValueIssue(
				path, name, "class", want.class, got.class,
			))
		}
		if got.description != want.description {
			issues = append(issues, generatedCommandValueIssue(
				path, name, "description", want.description, got.description,
			))
		}
		missingAvailability, extraAvailability := commandNameDifferences(
			want.available, got.available,
		)
		if len(missingAvailability) != 0 || len(extraAvailability) != 0 {
			issues = append(issues, generatedCommandValueIssue(
				path, name, "availability",
				strings.Join(want.available, ", "), strings.Join(got.available, ", "),
			))
		}
	}
	return issues
}

func primaryOperatorClassLabel(class string) string {
	switch class {
	case "global":
		return "Output and control"
	case "stream":
		return "Streaming"
	case "data":
		return "Row data"
	default:
		return class
	}
}

func generatedCommandValueIssue(path, operator, field, expected, actual string) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated operator metadata disagrees with the live command catalog",
		Detail: detail.Str("operator ").Quoted(operator).Byte(' ').Str(field).
			Str(" is ").Quoted(actual).Str("; expected ").Quoted(expected).String(),
	}
}

func generatedCommandSurfaceValueIssue(
	path, command, field, expected, actual string,
) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface disagrees with the live command contract",
		Detail: detail.Str("command ").Quoted(command).Byte(' ').Str(field).
			Str(" is ").Quoted(actual).Str("; expected ").Quoted(expected).String(),
	}
}

func splitNonEmpty(value, separator string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, separator)
}

func commandSurfaceSlug(path string) string {
	if !utf8.ValidString(path) {
		return ""
	}
	for _, character := range path {
		if character > unicode.MaxASCII {
			return "u--" + hex.EncodeToString([]byte(path))
		}
	}
	slug := commandSurfaceSlugSeparator.ReplaceAllString(strings.ToLower(path), "-")
	return strings.Trim(slug, "-")
}

type renderedHTMLDocument struct {
	root       *xhtml.Node
	rows       map[string][]renderedHTMLContainer
	htmlRows   []renderedHTMLContainer
	zeArticles []renderedHTMLContainer
	nodeClosed map[*xhtml.Node]bool
	err        error
}

type renderedHTMLContainer struct {
	root       *xhtml.Node
	closed     bool
	classValid bool
}

type renderedHTMLCapture struct {
	kind       string
	classValid bool
	closable   bool
	root       *xhtml.Node
}

type renderedHTMLOpenElement struct {
	name string
	node *xhtml.Node
}

func parseRenderedHTML(content string) renderedHTMLDocument {
	document := renderedHTMLDocument{
		root:       &xhtml.Node{Type: xhtml.DocumentNode},
		rows:       make(map[string][]renderedHTMLContainer),
		nodeClosed: make(map[*xhtml.Node]bool),
	}
	tokenizer := xhtml.NewTokenizer(strings.NewReader(content))
	var captures []renderedHTMLCapture
	var openElements []renderedHTMLOpenElement
	finish := func() renderedHTMLDocument {
		for _, capture := range captures {
			container := renderedHTMLContainer{
				root:       capture.root,
				closed:     capture.closable && document.nodeClosed[capture.root],
				classValid: capture.classValid,
			}
			switch capture.kind {
			case "tr":
				document.htmlRows = append(document.htmlRows, container)
				id := htmlAttribute(capture.root, "id")
				if strings.HasPrefix(id, "cmd-") {
					document.rows[id] = append(document.rows[id], container)
				}
			case articleElement:
				document.zeArticles = append(document.zeArticles, container)
			}
		}
		return document
	}
	appendNode := func(node *xhtml.Node) {
		parent := document.root
		if len(openElements) != 0 {
			parent = openElements[len(openElements)-1].node
		}
		htmlAppendChild(parent, node)
	}
	for {
		tokenType := tokenizer.Next()
		if tokenType == xhtml.ErrorToken {
			if err := tokenizer.Err(); !errors.Is(err, io.EOF) {
				document.err = err
			}
		}
		raw := tokenizer.Raw()
		token := tokenizer.Token()
		switch tokenType {
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			if htmlStartTokenMalformed(raw) {
				document.err = fmt.Errorf("malformed HTML start tag %q", raw)
				return finish()
			}
			node := htmlTokenNode(token)
			appendNode(node)
			closable := tokenType != xhtml.SelfClosingTagToken
			switch token.Data {
			case "tr":
				captures = append(captures, renderedHTMLCapture{
					kind: "tr", closable: closable, root: node,
				})
			case articleElement:
				classValid, candidate := zeArticleClass(tokenAttribute(token, "class"))
				if candidate {
					captures = append(captures, renderedHTMLCapture{
						kind:       articleElement,
						classValid: classValid,
						closable:   closable,
						root:       node,
					})
				}
			}
			closed := tokenType == xhtml.SelfClosingTagToken || htmlVoidElement(token.Data)
			document.nodeClosed[node] = closed
			if closed {
				continue
			}
			openElements = append(openElements, renderedHTMLOpenElement{
				name: token.Data,
				node: node,
			})
		case xhtml.TextToken:
			appendNode(&xhtml.Node{Type: xhtml.TextNode, Data: token.Data})
		case xhtml.CommentToken:
			appendNode(&xhtml.Node{Type: xhtml.CommentNode, Data: token.Data})
		case xhtml.EndTagToken:
			match := -1
			for index, open := range slices.Backward(openElements) {
				if open.name == token.Data {
					match = index
					break
				}
			}
			if match == -1 {
				continue
			}
			for index := len(openElements) - 1; index >= match; index-- {
				document.nodeClosed[openElements[index].node] = index == match
			}
			openElements = openElements[:match]
		case xhtml.ErrorToken:
			return finish()
		case xhtml.DoctypeToken:
			// A document type does not contribute command content.
		}
	}
}

func htmlStartTokenMalformed(raw []byte) bool {
	if len(raw) < 3 || raw[0] != '<' || raw[len(raw)-1] != '>' {
		return true
	}
	var quote byte
	for _, current := range raw[1 : len(raw)-1] {
		switch {
		case quote != 0 && current == quote:
			quote = 0
		case quote == 0 && (current == '\'' || current == '"'):
			quote = current
		case quote == 0 && current == '<':
			return true
		}
	}
	return quote != 0
}

func htmlTokenNode(token xhtml.Token) *xhtml.Node {
	return &xhtml.Node{
		Type:     xhtml.ElementNode,
		DataAtom: token.DataAtom,
		Data:     token.Data,
		Attr:     token.Attr,
	}
}

func htmlAppendChild(parent, child *xhtml.Node) {
	child.Parent = parent
	if parent.LastChild == nil {
		parent.FirstChild = child
		parent.LastChild = child
		return
	}
	child.PrevSibling = parent.LastChild
	parent.LastChild.NextSibling = child
	parent.LastChild = child
}

func tokenAttribute(token xhtml.Token, name string) string {
	for _, attribute := range token.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func htmlAttribute(node *xhtml.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func zeArticleClass(class string) (bool, bool) {
	const zeClass = "cmd-detail-ze"
	hasCard := false
	hasZe := false
	malformedZe := false
	for name := range strings.FieldsSeq(class) {
		hasCard = hasCard || name == "cmd-detail-card"
		hasZe = hasZe || name == zeClass
		malformedZe = malformedZe || name != "cmd-detail-card" && name != zeClass &&
			(strings.HasPrefix(zeClass, name) ||
				strings.HasSuffix(zeClass, name) ||
				strings.HasPrefix(name, zeClass) ||
				strings.HasSuffix(name, zeClass))
	}
	return hasCard && hasZe, hasZe || malformedZe
}

func htmlVoidElement(name string) bool {
	switch name {
	case "area", "base", "br", "col", "embed", "hr", "img", "input",
		"link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}

func commandSurfaceHTMLRow(
	document renderedHTMLDocument,
	path string,
) (*xhtml.Node, int, bool) {
	if document.err != nil {
		return nil, 1, false
	}
	var matches []renderedHTMLContainer
	for _, row := range document.htmlRows {
		identity, candidate, _ := primaryHTMLRowIdentity(row.root)
		if candidate && identity == path {
			matches = append(matches, row)
		}
	}
	if len(matches) != 1 {
		return nil, len(matches), false
	}
	return matches[0].root, len(matches), matches[0].closed
}
func validatePrimaryCommandContract(
	path string,
	row *xhtml.Node,
	document renderedHTMLDocument,
	command *publishedCommand,
) []Issue {
	var issues []Issue
	visible, visibleValid := primaryHTMLCommandValues(document, row)
	if !visibleValid {
		issues = append(issues, malformedCommandContainerIssue(
			path, command.Path, "primary CLI HTML visible command fields",
		))
	} else {
		for _, field := range []struct {
			name     string
			expected string
			actual   string
		}{
			{name: pathField, expected: command.Path, actual: visible[0]},
			{name: "mode", expected: normalizedCommandMode(command.Mode), actual: normalizedCommandMode(visible[1])},
			{name: descriptionField, expected: command.Description, actual: visible[2]},
		} {
			expected := normalizeRenderedHTMLText(field.expected)
			if field.actual != expected {
				issues = append(issues, generatedCommandSurfaceValueIssue(
					path, command.Path, field.name, expected, field.actual,
				))
			}
		}
	}

	var marker textbuf.Buffer
	issues = append(issues, compareHTMLLabeledValue(
		path,
		command.Path,
		"answer shape",
		normalizeRenderedHTMLText(command.AnswerShape),
		htmlLabeledValues(document, row, spanElement, "Answer shape", codeElement),
	)...)
	issues = append(issues, compareHTMLLabeledValue(
		path,
		command.Path,
		"address fields",
		normalizeRenderedHTMLText(strings.Join(command.AddressFields, " · ")),
		htmlLabeledValues(document, row, spanElement, "Address fields", codeElement),
	)...)
	issues = append(issues, validatePrimaryHTMLOperatorGroupLabels(
		path, command.Path, row,
	)...)
	for _, availability := range commandOperatorAvailabilities {
		scan := commandHTMLGroups(
			document,
			row,
			commandOperatorGroupLabel(primaryOperatorGroupSurface, availability, false),
		)
		issues = append(issues, compareScannedCommandOperatorGroups(
			path, command.Path, availability,
			commandOperatorNames(command, availability), scan,
		)...)
	}

	expectedFilters := make([]renderedCommandFilterDetail, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Str(filter.Name)
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		expectedFilters = append(expectedFilters, renderedCommandFilterDetail{
			identity:    normalizeRenderedHTMLText(marker.String()),
			description: normalizeRenderedHTMLText(filter.Description),
		})
	}
	actualFilters, filterGroups, filtersValid := htmlPrimaryFilterDetails(
		document, row, "Command pipes", "cli-pipe-chips",
	)
	expectedFilterGroups := 0
	if len(expectedFilters) != 0 {
		expectedFilterGroups = 1
	}
	issues = append(issues, compareEquivalentDetailValues(
		path, command.Path, "command filters",
		renderedFilterDetailValues(expectedFilters),
		renderedFilterDetailValues(actualFilters),
		expectedFilterGroups, filterGroups, filtersValid,
	)...)

	expectedAliases := make([]renderedCommandAliasDetail, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases, renderedCommandAliasDetail{
			identity:    normalizeRenderedHTMLText(alias.Name),
			description: normalizeRenderedHTMLText(alias.Description),
			expansion:   normalizeRenderedHTMLText(alias.Expansion),
		})
	}
	actualAliases, aliasGroups, aliasesValid := htmlPrimaryAliasDetails(
		document, row, "Aliases",
	)
	expectedAliasGroups := 0
	if len(expectedAliases) != 0 {
		expectedAliasGroups = 1
	}
	issues = append(issues, compareEquivalentDetailValues(
		path, command.Path, "pipe aliases",
		renderedAliasDetailValues(expectedAliases),
		renderedAliasDetailValues(actualAliases),
		expectedAliasGroups, aliasGroups, aliasesValid,
	)...)
	return issues
}

func primaryHTMLCommandValues(
	document renderedHTMLDocument,
	row *xhtml.Node,
) ([3]string, bool) {
	var values [3]string
	var cells []*xhtml.Node
	for child := row.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && child.Data == "td" {
			cells = append(cells, child)
		}
	}
	if len(cells) != 4 {
		return values, false
	}
	for _, cell := range cells {
		if !htmlVisibleSubtreeClosed(document, cell) {
			return values, false
		}
	}
	var codes []*xhtml.Node
	htmlWalk(cells[0], func(node *xhtml.Node) {
		if node.Data == codeElement {
			codes = append(codes, node)
		}
	})
	if len(codes) != 1 {
		return values, false
	}
	values[0] = normalizeRenderedHTMLText(htmlText(codes[0]))
	if normalizeRenderedHTMLText(htmlText(cells[0])) != values[0] {
		return values, false
	}
	values[1] = normalizeRenderedHTMLText(htmlText(cells[1]))
	values[2] = normalizeRenderedHTMLText(htmlText(cells[2]))
	return values, true
}

func commandSurfaceMarkdownRow(content, path string) (string, int, bool) {
	var row string
	count := 0
	malformed := false
	for _, scanned := range scanMarkdownLines(content) {
		if !scanned.active {
			continue
		}
		candidate, closed, canonical := markdownTableCodeCell(scanned.text)
		switch {
		case closed && candidate == path && canonical:
			row = scanned.text
			count++
		case closed && candidate == path:
			malformed = true
		case !closed && markdownUnclosedPath(candidate, path):
			malformed = true
		}
	}
	return row, count, malformed
}

func markdownTableCodeCell(line string) (string, bool, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || !strings.HasPrefix(trimmed, "|") {
		return "", true, false
	}
	cell := strings.TrimLeft(trimmed[1:], " ")
	path, suffix, _, closed := markdownCodeSpanPrefix(cell)
	path = decodeCommandMarkdownTableValue(path)
	if !closed {
		return path, false, false
	}
	canonical := strings.HasPrefix(trimmed, "| ") && strings.HasPrefix(suffix, " |")
	return path, true, canonical
}
func markdownTableCells(line string) ([]string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" || trimmed[0] != '|' {
		return nil, false
	}
	var cells []string
	cellAt := 1
	codeWidth := 0
	for index := 1; index < len(trimmed); {
		if trimmed[index] == '\\' {
			index += min(2, len(trimmed)-index)
			continue
		}
		if trimmed[index] == '`' {
			end := index
			for end < len(trimmed) && trimmed[end] == '`' {
				end++
			}
			width := end - index
			switch codeWidth {
			case 0:
				codeWidth = width
			case width:
				codeWidth = 0
			}
			index = end
			continue
		}
		if trimmed[index] == '|' {
			cells = append(cells, strings.TrimSpace(trimmed[cellAt:index]))
			cellAt = index + 1
		}
		index++
	}
	if codeWidth != 0 || cellAt != len(trimmed) {
		return nil, false
	}
	return cells, true
}

func splitMarkdownOutsideCode(value, delimiter string) ([]string, bool) {
	parts := make([]string, 0, strings.Count(value, delimiter)+1)
	partAt := 0
	for index := 0; index < len(value); {
		switch {
		case value[index] == '\\':
			index += min(2, len(value)-index)
		case value[index] == '`':
			_, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
			if !closed {
				return nil, false
			}
			index = len(value) - len(suffix)
		case strings.HasPrefix(value[index:], delimiter):
			parts = append(parts, value[partAt:index])
			index += len(delimiter)
			partAt = index
		default:
			index++
		}
	}
	parts = append(parts, value[partAt:])
	return parts, true
}

func markdownInlineValue(value string) string {
	value = strings.TrimSpace(value)
	if code, ok := markdownCodeSpan(value); ok {
		return decodeCommandMarkdownTableValue(code)
	}
	return markdownInlineVisibleText(value)
}

func primaryMarkdownCommandValues(row string) ([3]string, bool) {
	var values [3]string
	cells, valid := markdownTableCells(row)
	if !valid || len(cells) != 4 {
		return values, false
	}
	path, ok := markdownCodeSpan(cells[0])
	if !ok {
		return values, false
	}
	values[0] = strings.Join(strings.Fields(
		decodeCommandMarkdownTableValue(path),
	), " ")
	values[1] = markdownInlineVisibleText(cells[1])
	values[2] = markdownInlineVisibleText(cells[2])
	return values, true
}

func markdownUnclosedPath(candidate, path string) bool {
	var rendered textbuf.Buffer
	return candidate == path ||
		strings.HasPrefix(candidate, rendered.Str(path).Str(" |").String())
}
func commandMarkdownTableValue(value string) string {
	var encoded textbuf.Buffer
	encoded.Grow(len(value))
	for index := range len(value) {
		switch value[index] {
		case '\\':
			encoded.Str(`\\`)
		case '|':
			encoded.Str(`\|`)
		default:
			encoded.Byte(value[index])
		}
	}
	return encoded.String()
}

func decodeCommandMarkdownTableValue(value string) string {
	var decoded textbuf.Buffer
	decoded.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) &&
			(value[index+1] == '\\' || value[index+1] == '|') {
			index++
		}
		decoded.Byte(value[index])
	}
	return decoded.String()
}

func commandMarkdownValue(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func validatePrimaryMarkdownContract(
	path, row string,
	command *publishedCommand,
) []Issue {
	var issues []Issue
	visible, visibleValid := primaryMarkdownCommandValues(row)
	if !visibleValid {
		issues = append(issues, malformedCommandContainerIssue(
			path, command.Path, "primary CLI Markdown visible command fields",
		))
	} else {
		for _, field := range []struct {
			name     string
			expected string
			actual   string
		}{
			{name: pathField, expected: strings.Join(strings.Fields(command.Path), " "), actual: visible[0]},
			{name: "mode", expected: normalizedCommandMode(command.Mode), actual: normalizedCommandMode(visible[1])},
			{name: descriptionField, expected: markdownInlineVisibleText(markdownLiteralProse(command.Description)), actual: visible[2]},
		} {
			if field.actual != field.expected {
				issues = append(issues, generatedCommandSurfaceValueIssue(
					path, command.Path, field.name, field.expected, field.actual,
				))
			}
		}
	}

	var marker textbuf.Buffer
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "answer shape", splitNonEmpty(command.AnswerShape, "\x00"),
		commandMarkdownGroups(row, "Answer shape"),
	)...)
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "address field", command.AddressFields,
		commandMarkdownGroups(row, "Address fields"),
	)...)
	issues = append(issues, validatePrimaryMarkdownOperatorGroupLabels(
		path, command.Path, row,
	)...)
	for _, availability := range commandOperatorAvailabilities {
		issues = append(issues, compareCommandOperatorGroups(
			path, command.Path, availability,
			commandOperatorNames(command, availability),
			commandMarkdownGroups(
				row,
				commandOperatorGroupLabel(
					primaryOperatorGroupSurface, availability, false,
				),
			),
		)...)
	}
	expectedFilters := make([]string, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Str(filter.Name)
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		expectedFilters = append(expectedFilters, marker.String())
	}
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "command filter", expectedFilters,
		commandMarkdownGroups(row, "Command"),
	)...)
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases,
			marker.Reset().Str(alias.Name).Str(" -> ").Str(alias.Expansion).String())
	}
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "pipe alias", expectedAliases,
		commandMarkdownGroups(row, "Aliases"),
	)...)
	return issues
}
func commandMarkdownGroups(row, label string) [][]string {
	cells, valid := markdownTableCells(row)
	if !valid || len(cells) != 4 {
		return nil
	}
	segments, valid := splitMarkdownOutsideCode(cells[3], "<br>")
	if !valid {
		return nil
	}
	prefix := label + ": "
	var groups [][]string
	for _, segment := range segments {
		if !strings.HasPrefix(segment, prefix) {
			continue
		}
		values, valuesValid := splitMarkdownOutsideCode(
			strings.TrimPrefix(segment, prefix), ", ",
		)
		if !valuesValid {
			return nil
		}
		parsed := make([]string, 0, len(values))
		for _, value := range values {
			value = markdownInlineValue(value)
			if value != "" {
				parsed = append(parsed, value)
			}
		}
		groups = append(groups, parsed)
	}
	return groups
}

type commandOperatorGroupSurface uint8

const (
	primaryOperatorGroupSurface commandOperatorGroupSurface = iota
	equivalentHTMLOperatorGroupSurface
	equivalentMarkdownOperatorGroupSurface
)

var commandOperatorAvailabilities = [...]string{
	availabilityAlways,
	availabilityWithRows,
	availabilityWhenStreaming,
	availabilityLocalOnly,
}

var commandOperatorLabelFamilyTokens = [...]string{
	alwaysLabel,
	withRowsLabel,
	"When the answer has rows",
	whileStreamingLabel,
	"Streaming",
	"While the command keeps answering",
	localOnlyLabel,
	"Local only",
	"Pipes",
}

func commandOperatorGroupLabel(
	surface commandOperatorGroupSurface,
	availability string,
	declaredShape bool,
) string {
	switch surface {
	case primaryOperatorGroupSurface:
		return commandAvailabilityLabel(availability)
	case equivalentHTMLOperatorGroupSurface:
		if availability == availabilityWithRows {
			return equivalentAvailabilityLabel(availability, declaredShape)
		}
	case equivalentMarkdownOperatorGroupSurface:
		if availability == availabilityWithRows {
			return pipesOnRowsLabel
		}
	}
	switch availability {
	case availabilityAlways:
		return pipesAlwaysLabel
	case availabilityWhenStreaming:
		return pipesWhileStreamingLabel
	case availabilityLocalOnly:
		return pipesLocalOnlyLabel
	default:
		return availability
	}
}

func classifyCommandOperatorGroupLabel(
	label string,
	surface commandOperatorGroupSurface,
	declaredShape bool,
) (string, bool) {
	label = strings.Join(strings.Fields(label), " ")
	for _, availability := range commandOperatorAvailabilities {
		if label == commandOperatorGroupLabel(surface, availability, declaredShape) {
			return availability, true
		}
	}
	switch surface {
	case primaryOperatorGroupSurface,
		equivalentHTMLOperatorGroupSurface,
		equivalentMarkdownOperatorGroupSurface:
		return "", commandOperatorLabelCandidate(label)
	default:
		return "", false
	}
}

func commandOperatorLabelCandidate(label string) bool {
	for _, token := range commandOperatorLabelFamilyTokens {
		if label == token {
			return true
		}
		if !strings.HasPrefix(label, token) || len(label) == len(token) {
			continue
		}
		next, _ := utf8.DecodeRuneInString(label[len(token):])
		if unicode.IsPunct(next) || unicode.IsSpace(next) {
			return true
		}
	}
	return false
}

func unknownCommandOperatorGroupLabelIssue(path, command, label string) Issue {
	var detail textbuf.Buffer
	return Issue{
		File:    path,
		Message: "the generated per-command surface has a malformed operator availability group",
		Detail: detail.Str("command ").Quoted(command).
			Str(" has unknown operator group label ").Quoted(label).String(),
	}
}

func validatePrimaryHTMLOperatorGroupLabels(
	path, command string,
	root *xhtml.Node,
) []Issue {
	contractCell := primaryHTMLContractCell(root)
	if contractCell == nil {
		return nil
	}
	var issues []Issue
	htmlWalk(contractCell, func(node *xhtml.Node) {
		if node.Data != spanElement {
			return
		}
		label := normalizeRenderedHTMLText(htmlText(node))
		availability, candidate := classifyCommandOperatorGroupLabel(
			label, primaryOperatorGroupSurface, false,
		)
		if candidate && availability == "" {
			issues = append(issues,
				unknownCommandOperatorGroupLabelIssue(path, command, label))
		}
	})
	return issues
}

func primaryHTMLContractCell(row *xhtml.Node) *xhtml.Node {
	var cells []*xhtml.Node
	for child := row.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && child.Data == "td" {
			cells = append(cells, child)
		}
	}
	if len(cells) != 4 {
		return nil
	}
	return cells[3]
}

func validatePrimaryMarkdownOperatorGroupLabels(
	path, command, row string,
) []Issue {
	cells, valid := markdownTableCells(row)
	if !valid || len(cells) != 4 {
		return nil
	}
	segments, valid := splitMarkdownOutsideCode(cells[3], "<br>")
	if !valid {
		return nil
	}
	var issues []Issue
	for _, segment := range segments {
		label := markdownLabeledSegmentLabel(segment)
		availability, candidate := classifyCommandOperatorGroupLabel(
			label, primaryOperatorGroupSurface, false,
		)
		if candidate && availability == "" {
			issues = append(issues,
				unknownCommandOperatorGroupLabelIssue(path, command, label))
		}
	}
	return issues
}

// markdownLabeledSegmentLabel answers the visible label of a `<label>: <value>`
// segment. A segment that carries no separator is its own label, so the reader
// still has a name to report.
func markdownLabeledSegmentLabel(segment string) string {
	parts, valid := splitMarkdownOutsideCode(segment, ": ")
	if !valid || len(parts) < 2 {
		return strings.Join(strings.Fields(markdownInlineVisibleText(segment)), " ")
	}
	return strings.Join(strings.Fields(markdownInlineVisibleText(parts[0])), " ")
}

func commandAvailabilityLabel(availability string) string {
	switch availability {
	case availabilityAlways:
		return alwaysLabel
	case availabilityWithRows:
		return withRowsLabel
	case availabilityWhenStreaming:
		return whileStreamingLabel
	case availabilityLocalOnly:
		return localOnlyLabel
	default:
		return availability
	}
}

type commandHTMLGroupScan struct {
	groups    [][]string
	malformed bool
}

func commandHTMLGroups(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label string,
) commandHTMLGroupScan {
	var scan commandHTMLGroupScan
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != spanElement ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		var parsed []string
		codeCount := 0
	segment:
		for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
			switch sibling.Type {
			case xhtml.CommentNode:
				continue
			case xhtml.TextNode:
				if strings.TrimSpace(sibling.Data) != "" {
					scan.malformed = true
				}
				continue
			case xhtml.ElementNode:
				if sibling.Data == spanElement || sibling.Data == strongElement ||
					htmlCommandContainerRoot(sibling) {
					break segment
				}
				if sibling.Data != codeElement || codeCount != 0 {
					scan.malformed = true
				}
				if !htmlVisibleSubtreeClosed(document, sibling) {
					scan.malformed = true
				}
				if sibling.Data == codeElement {
					codeCount++
					values := normalizeRenderedHTMLText(htmlText(sibling))
					if values != "" {
						parsed = append(parsed, strings.Split(values, " · ")...)
					}
				}
			case xhtml.ErrorNode, xhtml.DocumentNode, xhtml.DoctypeNode, xhtml.RawNode:
				scan.malformed = true
			}
		}
		if !htmlVisibleSubtreeClosed(document, node) || codeCount == 0 {
			scan.malformed = true
		}
		scan.groups = append(scan.groups, parsed)
	})
	return scan
}

func htmlPrimaryFilterDetails(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label, chipClass string,
) ([]renderedCommandFilterDetail, int, bool) {
	var details []renderedCommandFilterDetail
	groups := 0
	valid := true
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != strongElement ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		groups++
		chips := htmlNextElement(node)
		var descriptionRoot *xhtml.Node
		if chips != nil {
			descriptionRoot = htmlNextElement(chips)
		}
		var list *xhtml.Node
		if descriptionRoot != nil && descriptionRoot.Data == "dl" {
			list = descriptionRoot
		} else if descriptionRoot != nil &&
			!htmlCommandContainerRoot(descriptionRoot) {
			htmlWalk(descriptionRoot, func(descendant *xhtml.Node) {
				if descendant.Data != "dl" {
					return
				}
				if list != nil {
					valid = false
				}
				list = descendant
			})
		}
		if !htmlVisibleSubtreeClosed(document, node) || chips == nil ||
			!htmlHasClass(chips, chipClass) ||
			!htmlVisibleSubtreeClosed(document, chips) ||
			descriptionRoot == nil ||
			!htmlVisibleSubtreeClosed(document, descriptionRoot) ||
			list == nil || !htmlVisibleSubtreeClosed(document, list) {
			valid = false
			return
		}
		var chipNames []string
		htmlWalk(chips, func(descendant *xhtml.Node) {
			if descendant.Data == codeElement {
				chipNames = append(chipNames,
					normalizeRenderedHTMLText(htmlText(descendant)))
			}
		})
		var entries []*xhtml.Node
		for child := list.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == xhtml.ElementNode {
				entries = append(entries, child)
			}
		}
		if len(entries)%2 != 0 {
			valid = false
			return
		}
		var detailNames []string
		for index := 0; index < len(entries); index += 2 {
			term, definition := entries[index], entries[index+1]
			code := htmlFirstElement(term)
			if term.Data != "dt" || definition.Data != "dd" ||
				code == nil || code.Parent != term || code.Data != codeElement ||
				htmlNextElement(code) != nil ||
				!htmlVisibleSubtreeClosed(document, term) ||
				!htmlVisibleSubtreeClosed(document, definition) ||
				normalizeRenderedHTMLText(htmlText(term)) !=
					normalizeRenderedHTMLText(htmlText(code)) {
				valid = false
				continue
			}
			inert := false
			htmlWalk(definition, func(descendant *xhtml.Node) {
				inert = inert || descendant.Data == "template" ||
					descendant.Data == codeElement
			})
			if inert {
				valid = false
				continue
			}
			identity := normalizeRenderedHTMLText(htmlText(code))
			detailNames = append(detailNames, identity)
			details = append(details, renderedCommandFilterDetail{
				identity:    identity,
				description: normalizeRenderedHTMLText(htmlText(definition)),
			})
		}
		if !sameStrings(chipNames, detailNames) {
			valid = false
		}
	})
	return details, groups, valid
}

func htmlPrimaryAliasDetails(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label string,
) ([]renderedCommandAliasDetail, int, bool) {
	var details []renderedCommandAliasDetail
	groups := 0
	valid := true
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != strongElement ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		groups++
		list := htmlNextElement(node)
		if !htmlVisibleSubtreeClosed(document, node) || list == nil ||
			list.Data != "dl" || !htmlVisibleSubtreeClosed(document, list) {
			valid = false
			return
		}
		var entries []*xhtml.Node
		for child := list.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == xhtml.ElementNode {
				entries = append(entries, child)
			}
		}
		if len(entries)%2 != 0 {
			valid = false
			return
		}
		for index := 0; index < len(entries); index += 2 {
			term, definition := entries[index], entries[index+1]
			code := htmlFirstElement(term)
			if term.Data != "dt" || definition.Data != "dd" ||
				code == nil || code.Parent != term || code.Data != codeElement ||
				htmlNextElement(code) != nil ||
				!htmlVisibleSubtreeClosed(document, term) ||
				!htmlVisibleSubtreeClosed(document, definition) ||
				normalizeRenderedHTMLText(htmlText(term)) !=
					normalizeRenderedHTMLText(htmlText(code)) {
				valid = false
				continue
			}
			inert := false
			htmlWalk(definition, func(descendant *xhtml.Node) {
				inert = inert || descendant.Data == "template"
			})
			tokenGroups, tokensValid := htmlEquivalentDetailTokens(document, definition)
			if inert || !tokensValid || len(tokenGroups) != 1 {
				valid = false
				continue
			}
			tokens := tokenGroups[0]
			codeAt := -1
			for tokenIndex, token := range tokens {
				if !token.code {
					continue
				}
				if codeAt != -1 {
					valid = false
				}
				codeAt = tokenIndex
			}
			if codeAt == -1 || normalizedHTMLDetailText(tokens[codeAt+1:]) != "" {
				valid = false
				continue
			}
			details = append(details, renderedCommandAliasDetail{
				identity:    normalizeRenderedHTMLText(htmlText(code)),
				description: normalizedHTMLDetailText(tokens[:codeAt]),
				expansion:   tokens[codeAt].value,
			})
		}
	})
	return details, groups, valid
}

type renderedHTMLValueScan struct {
	values    []string
	malformed bool
}

func htmlLabeledValues(
	document renderedHTMLDocument,
	root *xhtml.Node,
	labelElement, label, valueElement string,
) renderedHTMLValueScan {
	var scan renderedHTMLValueScan
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != labelElement ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		value := htmlNextElement(node)
		if !htmlVisibleSubtreeClosed(document, node) || value == nil ||
			value.Data != valueElement ||
			!htmlVisibleSubtreeClosed(document, value) {
			scan.malformed = true
			scan.values = append(scan.values, "")
			return
		}
		scan.values = append(
			scan.values, normalizeRenderedHTMLText(htmlText(value)),
		)
	})
	return scan
}

func compareHTMLLabeledValue(
	path, command, dimension, expected string,
	actual renderedHTMLValueScan,
) []Issue {
	if actual.malformed {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, dimension, expected, malformedIdentity,
		)}
	}
	if expected == "" {
		if len(actual.values) == 0 {
			return nil
		}
		return []Issue{generatedCommandExtraIssue(path, command, dimension)}
	}
	if len(actual.values) == 0 {
		return []Issue{generatedCommandContractIssue(path, command, dimension)}
	}
	if len(actual.values) != 1 {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, dimension, expected, strings.Join(actual.values, " | "),
		)}
	}
	if actual.values[0] != expected {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, dimension, expected, actual.values[0],
		)}
	}
	return nil
}

func htmlDefinitionCodeValue(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label string,
) (string, bool, bool) {
	value := ""
	count := 0
	valid := true
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "dt" ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		count++
		definition := htmlFollowingDefinition(node)
		var codes []*xhtml.Node
		if definition != nil {
			htmlWalk(definition, func(descendant *xhtml.Node) {
				if descendant.Data == codeElement {
					codes = append(codes, descendant)
				}
			})
		}
		directDefinition := htmlNextElement(node)
		var directCode *xhtml.Node
		if definition != nil {
			directCode = htmlFirstElement(definition)
		}
		nodesClosed := htmlVisibleSubtreeClosed(document, node) &&
			definition != nil &&
			htmlVisibleSubtreeClosed(document, definition)
		if len(codes) == 1 {
			value = normalizeRenderedHTMLText(htmlText(codes[0]))
		}
		if definition == nil || definition != directDefinition ||
			len(codes) != 1 || directCode != codes[0] ||
			htmlNextElement(codes[0]) != nil || !nodesClosed ||
			normalizeRenderedHTMLText(htmlText(definition)) != value {
			valid = false
		}
	})
	return value, count != 0, count == 1 && valid
}

func normalizeRenderedHTMLText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func htmlFollowingDefinition(term *xhtml.Node) *xhtml.Node {
	for sibling := term.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type != xhtml.ElementNode {
			continue
		}
		if htmlCommandContainerRoot(sibling) {
			return nil
		}
		switch sibling.Data {
		case "dt":
			return nil
		case "dd":
			return sibling
		}
	}
	return nil
}

func equivalentHTMLCommandContent(
	document renderedHTMLDocument,
) (*xhtml.Node, int, bool) {
	if document.err != nil {
		return nil, 1, false
	}
	var matches []renderedHTMLContainer
	for _, article := range document.zeArticles {
		if !htmlNestedCommandContainer(article.root) {
			matches = append(matches, article)
		}
	}
	if len(matches) != 1 {
		return nil, len(matches), false
	}
	return matches[0].root, len(matches),
		matches[0].closed && matches[0].classValid
}

func htmlWalk(root *xhtml.Node, visit func(*xhtml.Node)) {
	if root == nil {
		return
	}
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			visit(node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child != root && htmlCommandContainerRoot(child) {
				continue
			}
			walk(child)
		}
	}
	walk(root)
}

func htmlCommandContainerRoot(node *xhtml.Node) bool {
	if node.Type != xhtml.ElementNode {
		return false
	}
	if node.Data == "tr" && strings.HasPrefix(htmlAttribute(node, "id"), "cmd-") {
		return true
	}
	return node.Data == articleElement && htmlHasClass(node, "cmd-detail-card")
}

func htmlNestedCommandContainer(node *xhtml.Node) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if htmlCommandContainerRoot(parent) {
			return true
		}
	}
	return false
}
func htmlVisibleSubtreeClosed(
	document renderedHTMLDocument,
	root *xhtml.Node,
) bool {
	if root == nil {
		return false
	}
	closed := true
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if !closed {
			return
		}
		if node.Type == xhtml.ElementNode && !document.nodeClosed[node] {
			closed = false
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child != root && htmlCommandContainerRoot(child) {
				continue
			}
			walk(child)
		}
	}
	walk(root)
	return closed
}

func htmlText(root *xhtml.Node) string {
	var text textbuf.Buffer
	var appendText func(*xhtml.Node)
	appendText = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			text.Str(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child != root && htmlCommandContainerRoot(child) {
				continue
			}
			appendText(child)
		}
	}
	appendText(root)
	return text.String()
}

func htmlNextElement(node *xhtml.Node) *xhtml.Node {
	for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == xhtml.ElementNode {
			return sibling
		}
		if sibling.Type == xhtml.TextNode && strings.TrimSpace(sibling.Data) != "" {
			return nil
		}
	}
	return nil
}

func htmlFirstElement(node *xhtml.Node) *xhtml.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode {
			return child
		}
		if child.Type == xhtml.TextNode && strings.TrimSpace(child.Data) != "" {
			return nil
		}
	}
	return nil
}

func htmlHasClass(node *xhtml.Node, class string) bool {
	return slices.Contains(strings.Fields(htmlAttribute(node, "class")), class)
}

type renderedCommandFilterDetail struct {
	identity    string
	description string
}

type renderedCommandAliasDetail struct {
	identity    string
	description string
	expansion   string
}

type renderedHTMLInlineToken struct {
	code      bool
	breakLine bool
	value     string
}

func htmlEquivalentDetailTokens(
	document renderedHTMLDocument,
	root *xhtml.Node,
) ([][]renderedHTMLInlineToken, bool) {
	if !htmlVisibleSubtreeClosed(document, root) {
		return nil, false
	}
	groups := [][]renderedHTMLInlineToken{{}}
	valid := true
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			switch {
			case htmlCommandContainerRoot(child):
				valid = false
			case child.Type == xhtml.TextNode:
				groups[len(groups)-1] = append(groups[len(groups)-1],
					renderedHTMLInlineToken{value: child.Data})
			case child.Type != xhtml.ElementNode:
			case !document.nodeClosed[child]:
				valid = false
			case child.Data == "br":
				groups[len(groups)-1] = append(groups[len(groups)-1],
					renderedHTMLInlineToken{breakLine: true})
				groups = append(groups, nil)
			case child.Data == codeElement:
				groups[len(groups)-1] = append(groups[len(groups)-1],
					renderedHTMLInlineToken{
						code:  true,
						value: normalizeRenderedHTMLText(htmlText(child)),
					})
			default:
				walk(child)
			}
		}
	}
	walk(root)
	return groups, valid
}

func normalizedHTMLDetailText(tokens []renderedHTMLInlineToken) string {
	var value textbuf.Buffer
	for _, token := range tokens {
		if !token.code && !token.breakLine {
			value.Str(token.value)
		}
	}
	return normalizeRenderedHTMLText(value.String())
}

func parseHTMLFilterDetail(
	tokens []renderedHTMLInlineToken,
) (renderedCommandFilterDetail, bool) {
	var detail renderedCommandFilterDetail
	codeAt := -1
	for index, token := range tokens {
		if !token.code {
			continue
		}
		if codeAt != -1 {
			return detail, false
		}
		codeAt = index
		detail.identity = token.value
	}
	if codeAt == -1 || normalizedHTMLDetailText(tokens[:codeAt]) != "" {
		return detail, false
	}
	suffix := normalizedHTMLDetailText(tokens[codeAt+1:])
	if !strings.HasPrefix(suffix, ":") {
		return detail, false
	}
	detail.description = normalizeRenderedHTMLText(strings.TrimPrefix(suffix, ":"))
	return detail, true
}

func parseHTMLAliasDetail(
	tokens []renderedHTMLInlineToken,
) (renderedCommandAliasDetail, bool) {
	var detail renderedCommandAliasDetail
	var codes []int
	for index, token := range tokens {
		if token.code {
			codes = append(codes, index)
		}
	}
	if len(codes) != 2 || normalizedHTMLDetailText(tokens[:codes[0]]) != "" {
		return detail, false
	}
	detail.identity = tokens[codes[0]].value
	detail.expansion = tokens[codes[1]].value
	middle := normalizedHTMLDetailText(tokens[codes[0]+1 : codes[1]])
	suffix := normalizedHTMLDetailText(tokens[codes[1]+1:])
	if !strings.HasPrefix(middle, ":") || !strings.HasSuffix(middle, "(") ||
		suffix != ")" {
		return detail, false
	}
	detail.description = normalizeRenderedHTMLText(
		strings.TrimSuffix(strings.TrimPrefix(middle, ":"), "("),
	)
	return detail, true
}

func equivalentHTMLDefinitionList(
	document renderedHTMLDocument,
	article *xhtml.Node,
) bool {
	var lists []*xhtml.Node
	valid := true
	htmlWalk(article, func(node *xhtml.Node) {
		if node.Data == "dl" {
			lists = append(lists, node)
			return
		}
		if node.Data != "dt" && node.Data != "dd" {
			return
		}
		owned := false
		for parent := node.Parent; parent != nil && parent != article; parent = parent.Parent {
			if parent.Data == "dl" {
				owned = true
				break
			}
		}
		valid = valid && owned
	})
	return valid && len(lists) == 1 && lists[0].Parent == article &&
		htmlVisibleSubtreeClosed(document, lists[0])
}

func equivalentHTMLDirectDefinitionTerms(article *xhtml.Node) []string {
	var lists []*xhtml.Node
	for child := article.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && child.Data == "dl" {
			lists = append(lists, child)
		}
	}
	var terms []string
	for _, list := range lists {
		htmlWalk(list, func(node *xhtml.Node) {
			if node.Data != "dt" {
				return
			}
			for parent := node.Parent; parent != nil && parent != article; parent = parent.Parent {
				if parent.Data != "dl" {
					continue
				}
				if parent == list {
					terms = append(terms, normalizeRenderedHTMLText(htmlText(node)))
				}
				return
			}
		})
	}
	return terms
}

func validateEquivalentHTMLPipeTerms(
	path string,
	article *xhtml.Node,
	command *publishedCommand,
) []Issue {
	var issues []Issue
	for _, term := range equivalentHTMLDirectDefinitionTerms(article) {
		availability, candidate := classifyCommandOperatorGroupLabel(
			term,
			equivalentHTMLOperatorGroupSurface,
			command.AnswerShape != "",
		)
		if !candidate || availability != "" {
			continue
		}
		issues = append(issues,
			unknownCommandOperatorGroupLabelIssue(path, command.Path, term))
	}
	return issues
}

func htmlEquivalentFilterDetails(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label string,
) ([]renderedCommandFilterDetail, int, bool) {
	var details []renderedCommandFilterDetail
	groups := 0
	valid := true
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "dt" ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		groups++
		definition := htmlNextElement(node)
		if !htmlVisibleSubtreeClosed(document, node) || definition == nil ||
			definition.Data != "dd" {
			valid = false
			return
		}
		entries, entriesValid := htmlEquivalentDetailTokens(document, definition)
		valid = valid && entriesValid
		for _, entry := range entries {
			detail, entryValid := parseHTMLFilterDetail(entry)
			valid = valid && entryValid
			if entryValid {
				details = append(details, detail)
			}
		}
	})
	return details, groups, valid
}

func htmlEquivalentAliasDetails(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label string,
) ([]renderedCommandAliasDetail, int, bool) {
	var details []renderedCommandAliasDetail
	groups := 0
	valid := true
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "dt" ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		groups++
		definition := htmlNextElement(node)
		if !htmlVisibleSubtreeClosed(document, node) || definition == nil ||
			definition.Data != "dd" {
			valid = false
			return
		}
		entries, entriesValid := htmlEquivalentDetailTokens(document, definition)
		valid = valid && entriesValid
		for _, entry := range entries {
			detail, entryValid := parseHTMLAliasDetail(entry)
			valid = valid && entryValid
			if entryValid {
				details = append(details, detail)
			}
		}
	})
	return details, groups, valid
}

func renderedFilterDetailValues(details []renderedCommandFilterDetail) []string {
	values := make([]string, 0, len(details))
	for _, detail := range details {
		values = append(values, detail.identity+": "+detail.description)
	}
	return values
}

func renderedAliasDetailValues(details []renderedCommandAliasDetail) []string {
	values := make([]string, 0, len(details))
	for _, detail := range details {
		values = append(values,
			detail.identity+": "+detail.description+" ("+detail.expansion+")")
	}
	return values
}

func compareEquivalentDetailValues(
	path, command, dimension string,
	expected, actual []string,
	expectedGroups, groups int,
	valid bool,
) []Issue {
	if groups != expectedGroups {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, dimension+" cardinality",
			fmt.Sprintf("%d labeled groups", expectedGroups),
			fmt.Sprintf("%d labeled groups", groups),
		)}
	}
	if !valid {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, dimension+" structure", "well-formed details", "malformed details",
		)}
	}
	if len(expected) != len(actual) {
		return []Issue{generatedCommandSurfaceValueIssue(
			path, command, dimension,
			strings.Join(expected, " | "), strings.Join(actual, " | "),
		)}
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return []Issue{generatedCommandSurfaceValueIssue(
				path, command, dimension,
				strings.Join(expected, " | "), strings.Join(actual, " | "),
			)}
		}
	}
	return nil
}

func validateEquivalentCommandContract(
	path string,
	document renderedHTMLDocument,
	command *publishedCommand,
) []Issue {
	var issues []Issue
	for _, article := range document.zeArticles {
		if !htmlNestedCommandContainer(article.root) {
			continue
		}
		identity, present, valid := htmlDefinitionCodeValue(
			document, article.root, "Registry path",
		)
		if !present || !valid {
			issues = append(issues, generatedCommandExtraIssue(
				path, malformedIdentity,
				"command-equivalent Ze HTML article identity absent from the live command catalog",
			))
			continue
		}
		if identity != command.Path {
			issues = append(issues, generatedCommandExtraIssue(
				path, identity,
				"command-equivalent Ze HTML article identity absent from the live command catalog",
			))
			continue
		}
		issues = append(issues, generatedCommandExtraIssue(
			path, identity, "nested command-equivalent Ze HTML article",
		))
	}
	commandNode, containerCount, containerClosed := equivalentHTMLCommandContent(
		document,
	)
	switch {
	case containerCount != 1:
		issues = append(issues, commandContainerCountIssue(
			path, command.Path, "command-equivalent Ze HTML article", containerCount,
		))
		return issues
	case !containerClosed:
		issues = append(issues, malformedCommandContainerIssue(
			path, command.Path, "command-equivalent Ze HTML article",
		))
		return issues
	}
	registryPath, registryPresent, registryValid := htmlDefinitionCodeValue(
		document, commandNode, "Registry path",
	)
	switch {
	case !registryPresent:
		issues = append(issues, generatedCommandContractIssue(
			path, command.Path, "registry path",
		))
	case !registryValid:
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "registry path structure",
			"one direct code value", "malformed Registry path definition",
		))
	case registryPath != command.Path:
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "registry path", command.Path, registryPath,
		))
	}
	if !equivalentHTMLDefinitionList(document, commandNode) {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "definition list structure",
			"one command-owned dl", "malformed term ownership",
		))
	}
	issues = append(issues,
		validateEquivalentHTMLPipeTerms(path, commandNode, command)...)
	var marker textbuf.Buffer
	issues = append(issues, compareHTMLLabeledValue(
		path,
		command.Path,
		"answer shape",
		normalizeRenderedHTMLText(command.AnswerShape),
		htmlLabeledValues(document, commandNode, "dt", "Answer shape", "dd"),
	)...)
	issues = append(issues, compareHTMLLabeledValue(
		path,
		command.Path,
		"address fields",
		normalizeRenderedHTMLText(strings.Join(command.AddressFields, ", ")),
		htmlLabeledValues(document, commandNode, "dt", "Address fields", "dd"),
	)...)
	for _, availability := range commandOperatorAvailabilities {
		scan := equivalentHTMLGroups(
			document,
			commandNode,
			commandOperatorGroupLabel(
				equivalentHTMLOperatorGroupSurface,
				availability,
				command.AnswerShape != "",
			),
		)
		issues = append(issues, compareScannedCommandOperatorGroups(
			path, command.Path, availability,
			commandOperatorNames(command, availability), scan,
		)...)
	}
	expectedFilters := make([]renderedCommandFilterDetail, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Str(filter.Name)
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		expectedFilters = append(expectedFilters, renderedCommandFilterDetail{
			identity:    normalizeRenderedHTMLText(marker.String()),
			description: normalizeRenderedHTMLText(filter.Description),
		})
	}
	actualFilters, filterGroups, filtersValid := htmlEquivalentFilterDetails(
		document, commandNode, "Command pipes",
	)
	expectedFilterGroups := 0
	if len(expectedFilters) != 0 {
		expectedFilterGroups = 1
	}
	issues = append(issues, compareEquivalentDetailValues(
		path, command.Path, "command filters",
		renderedFilterDetailValues(expectedFilters),
		renderedFilterDetailValues(actualFilters),
		expectedFilterGroups, filterGroups, filtersValid,
	)...)
	expectedAliases := make([]renderedCommandAliasDetail, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases, renderedCommandAliasDetail{
			identity:    normalizeRenderedHTMLText(alias.Name),
			description: normalizeRenderedHTMLText(alias.Description),
			expansion:   normalizeRenderedHTMLText(alias.Expansion),
		})
	}
	actualAliases, aliasGroups, aliasesValid := htmlEquivalentAliasDetails(
		document, commandNode, "Pipe aliases",
	)
	expectedAliasGroups := 0
	if len(expectedAliases) != 0 {
		expectedAliasGroups = 1
	}
	issues = append(issues, compareEquivalentDetailValues(
		path, command.Path, "pipe aliases",
		renderedAliasDetailValues(expectedAliases),
		renderedAliasDetailValues(actualAliases),
		expectedAliasGroups, aliasGroups, aliasesValid,
	)...)
	return issues
}

func equivalentMarkdownCommandContent(content string) (string, int) {
	return markdownHeadingContent(content, "## Ze command")
}
func equivalentMarkdownTitleIdentity(content string) (string, int, bool) {
	identity := ""
	count := 0
	valid := true
	for _, line := range scanMarkdownLines(content) {
		if !line.active || line.headingLevel != 1 {
			continue
		}
		count++
		path, wrapped, candidate := markdownWrappedCodeSpan(
			strings.TrimSpace(line.heading),
		)
		identity = path
		if !candidate || !wrapped {
			valid = false
		}
	}
	return identity, count, count == 1 && valid
}

type renderedMarkdownInlineToken struct {
	code  bool
	value string
}

func markdownDetailTokens(value string) ([]renderedMarkdownInlineToken, bool) {
	var tokens []renderedMarkdownInlineToken
	textAt := 0
	for index := 0; index < len(value); {
		if value[index] != '`' || index > 0 && value[index-1] == '\\' {
			index++
			continue
		}
		code, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
		if !closed {
			return tokens, false
		}
		if textAt != index {
			tokens = append(tokens, renderedMarkdownInlineToken{
				value: value[textAt:index],
			})
		}
		tokens = append(tokens, renderedMarkdownInlineToken{code: true, value: code})
		index = len(value) - len(suffix)

		textAt = index
	}
	if textAt != len(value) {
		tokens = append(tokens, renderedMarkdownInlineToken{value: value[textAt:]})
	}
	return tokens, true
}

func normalizedCommandMarkdownDetailValue(value string) string {
	return markdownInlineVisibleText(markdownLiteralProse(value))
}

func splitMarkdownDetailEntries(value string) ([]string, bool) {
	var entries []string
	entryAt := 0
	for index := 0; index < len(value); {
		switch value[index] {
		case '\\':
			index += min(2, len(value)-index)
		case '`':
			_, suffix, _, closed := markdownCodeSpanPrefix(value[index:])
			if !closed {
				return nil, false
			}
			index = len(value) - len(suffix)
		case ';':
			entries = append(entries, strings.TrimSpace(value[entryAt:index]))
			index++
			entryAt = index
		default:
			index++
		}
	}
	entries = append(entries, strings.TrimSpace(value[entryAt:]))
	return entries, true
}

func unescapeMarkdownDetailValue(value string) string {
	return markdownInlineVisibleText(value)
}

func parseMarkdownFilterDetail(
	value string,
) (renderedCommandFilterDetail, bool) {
	var detail renderedCommandFilterDetail
	identity, suffix, valid, candidate := markdownWrappedCodeSpanPrefix(
		strings.TrimSpace(value),
	)
	if !candidate || !valid || strings.Contains(suffix, "`") {
		return detail, false
	}
	detail.identity = identity
	description, valid := markdownDetailDescription(suffix)
	if !valid {
		return renderedCommandFilterDetail{}, false
	}
	detail.description = description
	return detail, true
}

func markdownDetailDescription(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	if !strings.HasPrefix(value, ":") {
		return "", false
	}
	description := strings.TrimPrefix(value, ":")
	description = strings.TrimPrefix(description, " ")
	return unescapeMarkdownDetailValue(description), true
}

func parseMarkdownAliasDetail(
	value string,
) (renderedCommandAliasDetail, bool) {
	var detail renderedCommandAliasDetail
	identity, suffix, valid, candidate := markdownWrappedCodeSpanPrefix(
		strings.TrimSpace(value),
	)
	if !candidate || !valid {
		return detail, false
	}
	detail.identity = identity
	suffix = strings.TrimSpace(suffix)
	if strings.HasSuffix(suffix, ")") {
		for open := len(suffix) - 2; open >= 0; open-- {
			if suffix[open] != '(' || open > 0 && suffix[open-1] == '\\' {
				continue
			}
			expansion, wrapped, expansionCandidate := markdownWrappedCodeSpan(
				strings.TrimSpace(suffix[open+1 : len(suffix)-1]),
			)
			if !expansionCandidate || !wrapped {
				continue
			}
			description, descriptionValid := markdownDetailDescription(
				strings.TrimSuffix(suffix[:open], " "),
			)
			if !descriptionValid {
				return renderedCommandAliasDetail{}, false
			}
			detail.description = description
			detail.expansion = expansion
			return detail, true
		}
	}
	if strings.Contains(suffix, "`") {
		return renderedCommandAliasDetail{}, false
	}
	description, valid := markdownDetailDescription(suffix)
	if !valid {
		return renderedCommandAliasDetail{}, false
	}
	detail.description = description
	return detail, true
}
func markdownDetailIsNone(value string) (bool, bool) {
	if normalizedCommandMode(markdownInlineVisibleText(value)) != "none" {
		return false, true
	}
	tokens, valid := markdownDetailTokens(value)
	for _, token := range tokens {
		if token.code {
			valid = false
		}
	}
	return true, valid
}

func markdownEquivalentFilterDetails(
	content, label string,
) ([]renderedCommandFilterDetail, int, bool) {
	var marker textbuf.Buffer
	prefix := marker.Str("- ").Str(label).Str(": ").String()
	var details []renderedCommandFilterDetail
	groups := 0
	valid := true
	for _, line := range scanMarkdownLines(content) {
		if !line.active || !strings.HasPrefix(line.text, prefix) {
			continue
		}
		groups++
		value := strings.TrimPrefix(line.text, prefix)
		if none, noneValid := markdownDetailIsNone(value); none {
			valid = valid && noneValid
			continue
		}
		entries, entriesValid := splitMarkdownDetailEntries(value)
		valid = valid && entriesValid
		for _, entry := range entries {
			detail, entryValid := parseMarkdownFilterDetail(entry)
			valid = valid && entryValid
			if entryValid {
				details = append(details, detail)
			}
		}
	}
	return details, groups, valid
}

func markdownEquivalentAliasDetails(
	content, label string,
) ([]renderedCommandAliasDetail, int, bool) {
	var marker textbuf.Buffer
	prefix := marker.Str("- ").Str(label).Str(": ").String()
	var details []renderedCommandAliasDetail
	groups := 0
	valid := true
	for _, line := range scanMarkdownLines(content) {
		if !line.active || !strings.HasPrefix(line.text, prefix) {
			continue
		}
		groups++
		value := strings.TrimPrefix(line.text, prefix)
		if none, noneValid := markdownDetailIsNone(value); none {
			valid = valid && noneValid
			continue
		}
		entries, entriesValid := splitMarkdownDetailEntries(value)
		valid = valid && entriesValid
		for _, entry := range entries {
			detail, entryValid := parseMarkdownAliasDetail(entry)
			valid = valid && entryValid
			if entryValid {
				details = append(details, detail)
			}
		}
	}
	return details, groups, valid
}

func validateEquivalentMarkdownContract(
	path, content string,
	command *publishedCommand,
) []Issue {
	title, titleCount, titleValid := equivalentMarkdownTitleIdentity(content)
	var issues []Issue
	switch {
	case titleCount != 1:
		issues = append(issues, commandContainerCountIssue(
			path, command.Path, "command-equivalent Markdown top-level heading",
			titleCount,
		))
	case !titleValid:
		issues = append(issues, malformedCommandContainerIssue(
			path, command.Path, "command-equivalent Markdown top-level heading",
		))
	case title != command.Path:
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "top-level heading path", command.Path, title,
		))
	}
	commandContent, containerCount := equivalentMarkdownCommandContent(content)
	if containerCount != 1 {
		issues = append(issues, commandContainerCountIssue(
			path, command.Path, "command-equivalent Markdown Ze command section",
			containerCount,
		))
		return issues
	}
	content = commandContent
	var marker textbuf.Buffer
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "registry path", []string{command.Path},
		equivalentMarkdownVisibleGroups(content, "Registry path"),
	)...)
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "answer shape", splitNonEmpty(command.AnswerShape, "\x00"),
		equivalentMarkdownVisibleGroups(content, "Answer shape"),
	)...)
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "address field", command.AddressFields,
		equivalentMarkdownGroups(content, "Address fields"),
	)...)
	labelIssues := validateEquivalentMarkdownOperatorGroupLabels(
		path, content, command,
	)
	issues = append(issues, labelIssues...)
	if len(labelIssues) != 0 {
		return issues
	}
	for _, availability := range commandOperatorAvailabilities {
		issues = append(issues, compareCommandOperatorGroups(
			path, command.Path, availability,
			commandOperatorNames(command, availability),
			equivalentMarkdownGroups(
				content,
				commandOperatorGroupLabel(
					equivalentMarkdownOperatorGroupSurface,
					availability,
					command.AnswerShape != "",
				),
			),
		)...)
	}
	expectedFilters := make([]renderedCommandFilterDetail, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Str(filter.Name)
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		expectedFilters = append(expectedFilters, renderedCommandFilterDetail{
			identity:    normalizeMarkdownCodeSpan(marker.String()),
			description: normalizedCommandMarkdownDetailValue(filter.Description),
		})
	}
	actualFilters, filterGroups, filtersValid := markdownEquivalentFilterDetails(
		content, "Command pipes",
	)
	issues = append(issues, compareEquivalentDetailValues(
		path, command.Path, "command filters",
		renderedFilterDetailValues(expectedFilters),
		renderedFilterDetailValues(actualFilters),
		1, filterGroups, filtersValid,
	)...)
	expectedAliases := make([]renderedCommandAliasDetail, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases, renderedCommandAliasDetail{
			identity:    normalizeMarkdownCodeSpan(alias.Name),
			description: normalizedCommandMarkdownDetailValue(alias.Description),
			expansion:   normalizeMarkdownCodeSpan(alias.Expansion),
		})
	}
	actualAliases, aliasGroups, aliasesValid := markdownEquivalentAliasDetails(
		content, "Pipe aliases",
	)
	issues = append(issues, compareEquivalentDetailValues(
		path, command.Path, "pipe aliases",
		renderedAliasDetailValues(expectedAliases),
		renderedAliasDetailValues(actualAliases),
		1, aliasGroups, aliasesValid,
	)...)
	return issues
}

func validateEquivalentMarkdownOperatorGroupLabels(
	path, content string,
	command *publishedCommand,
) []Issue {
	counts := make(map[string]int, len(commandOperatorAvailabilities))
	var issues []Issue
	for _, line := range scanMarkdownLines(content) {
		if !line.active {
			continue
		}
		label, candidate := markdownListEntryLabel(line.text)
		if !candidate {
			continue
		}
		availability, operatorLabel := classifyCommandOperatorGroupLabel(
			label, equivalentMarkdownOperatorGroupSurface,
			command.AnswerShape != "",
		)
		if !operatorLabel {
			continue
		}
		if availability == "" {
			issues = append(issues,
				unknownCommandOperatorGroupLabelIssue(path, command.Path, label))
			continue
		}
		counts[availability]++
	}
	for _, availability := range commandOperatorAvailabilities {
		if counts[availability] > 1 {
			issues = append(issues, duplicateCommandOperatorGroupIssue(
				path, command.Path, availability, counts[availability],
			))
		}
	}
	return issues
}

func markdownListEntryLabel(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 2 {
		return "", false
	}
	markerEnd := 0
	if strings.ContainsRune("-*+", rune(trimmed[0])) {
		markerEnd = 1
	} else {
		for markerEnd < len(trimmed) && markerEnd < 9 &&
			trimmed[markerEnd] >= '0' && trimmed[markerEnd] <= '9' {
			markerEnd++
		}
		if markerEnd == 0 || markerEnd >= len(trimmed) ||
			(trimmed[markerEnd] != '.' && trimmed[markerEnd] != ')') {
			return "", false
		}
		markerEnd++
	}
	if markerEnd >= len(trimmed) ||
		(trimmed[markerEnd] != ' ' && trimmed[markerEnd] != '\t') {
		return "", false
	}
	item := strings.TrimLeft(trimmed[markerEnd:], " \t")
	label := markdownLabeledSegmentLabel(item)
	return label, label != ""
}

func equivalentMarkdownVisibleGroups(content, label string) [][]string {
	var marker textbuf.Buffer
	prefix := marker.Str("- ").Str(label).Str(": ").String()
	var groups [][]string
	for _, line := range scanMarkdownLines(content) {
		if !line.active || !strings.HasPrefix(line.text, prefix) {
			continue
		}
		value := strings.TrimPrefix(line.text, prefix)
		groups = append(groups, []string{
			strings.Join(strings.Fields(markdownInlineVisibleText(value)), " "),
		})
	}
	return groups
}

func equivalentMarkdownGroups(content, label string) [][]string {
	var marker textbuf.Buffer
	prefix := marker.Str("- ").Str(label).Str(": ").String()
	var groups [][]string
	for _, line := range scanMarkdownLines(content) {
		if !line.active || !strings.HasPrefix(line.text, prefix) {
			continue
		}
		values := strings.TrimPrefix(line.text, prefix)
		if values == "none" {
			groups = append(groups, nil)
			continue
		}
		parsed, valid := splitMarkdownOutsideCode(values, ", ")
		if !valid {
			groups = append(groups, []string{""})
			continue
		}
		for index := range parsed {
			parsed[index] = strings.Join(strings.Fields(markdownInlineValue(parsed[index])), " ")
		}
		groups = append(groups, parsed)
	}
	return groups
}

func equivalentAvailabilityLabel(availability string, declaredShape bool) string {
	switch availability {
	case availabilityAlways:
		return pipesAlwaysLabel
	case availabilityWithRows:
		if declaredShape {
			return "Pipes, on its rows"
		}
		return "Pipes, when the answer has rows"
	case availabilityWhenStreaming:
		return pipesWhileStreamingLabel
	default:
		return availability
	}
}

func equivalentHTMLGroups(
	document renderedHTMLDocument,
	root *xhtml.Node,
	label string,
) commandHTMLGroupScan {
	var scan commandHTMLGroupScan
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "dt" ||
			normalizeRenderedHTMLText(htmlText(node)) != label {
			return
		}
		definition := htmlNextElement(node)
		if !htmlVisibleSubtreeClosed(document, node) || definition == nil ||
			definition.Data != "dd" ||
			!htmlVisibleSubtreeClosed(document, definition) {
			scan.malformed = true
			return
		}
		values := htmlText(definition)
		var parsed []string
		if values != "" {
			parsed = strings.Split(values, ", ")
		}
		scan.groups = append(scan.groups, parsed)
	})
	return scan
}

func llmsCommandMetadata(
	content, path string,
) (identity, meta, description string, count int, valid bool) {
	valid = true
	for _, scanned := range scanMarkdownLines(content) {
		if !scanned.active {
			continue
		}
		candidate, remaining, closed, canonical := markdownListCodeSpan(scanned.text)
		switch {
		case closed && candidate == path && canonical:
			count++
			parts, partsValid := splitMarkdownOutsideCode(remaining, "): ")
			if !partsValid || len(parts) != 2 || !strings.HasPrefix(parts[0], " (") {
				valid = false
				continue
			}
			identity = candidate
			meta = strings.TrimPrefix(parts[0], " (")
			description = parts[1]
		case closed && candidate == path:
			valid = false
		case !closed && markdownUnclosedListPath(candidate, path):
			valid = false
		}
	}
	return identity, meta, description, count, valid
}

func markdownUnclosedListPath(candidate, path string) bool {
	var rendered textbuf.Buffer
	return candidate == path ||
		strings.HasPrefix(candidate, rendered.Str(path).Str(" (").String())
}

func markdownListCodeSpan(line string) (string, string, bool, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || !strings.HasPrefix(trimmed, "-") {
		return "", "", true, false
	}
	item := strings.TrimLeft(trimmed[1:], " ")
	path, suffix, width, closed := markdownCodeSpanPrefix(item)
	if width == 0 {
		return "", "", true, false
	}
	canonical := closed && strings.HasPrefix(trimmed, "- ") &&
		strings.HasPrefix(suffix, " (")
	return path, suffix, closed, canonical
}

func parseLLMSCodeValues(value string) []string {
	tokens, valid := markdownDetailTokens(value)
	if !valid {
		return []string{""}
	}
	var values []string
	for _, token := range tokens {
		if token.code {
			values = append(values, token.value)
			continue
		}
		values = append(values, strings.Fields(token.value)...)
	}
	return values
}

func parseLLMSAliases(value string) []string {
	entries, valid := splitMarkdownOutsideCode(value, ", ")
	if !valid {
		return []string{""}
	}
	aliases := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		var identity, expansion string
		if code, suffix, _, closed := markdownCodeSpanPrefix(entry); closed {
			identity = code
			if !strings.HasPrefix(suffix, "=") {
				aliases = append(aliases, "")
				continue
			}
			expansion = strings.TrimPrefix(suffix, "=")
		} else {
			var found bool
			identity, expansion, found = strings.Cut(entry, "=")
			if !found {
				aliases = append(aliases, "")
				continue
			}
		}
		expansion = strings.TrimSpace(expansion)
		if code, ok := markdownCodeSpan(expansion); ok {
			expansion = code
		} else {
			expansion = markdownInlineVisibleText(expansion)
		}
		aliases = append(aliases, identity+"="+expansion)
	}
	return aliases
}

func parseLLMSCommaValues(value string) []string {
	values, valid := splitMarkdownOutsideCode(value, ", ")
	if !valid {
		return []string{""}
	}
	for index := range values {
		values[index] = markdownInlineValue(values[index])
	}
	return values
}

func validateLLMSCommandContract(
	path, identity, meta, description string,
	command *publishedCommand,
) []Issue {
	var issues []Issue
	var rendered textbuf.Buffer
	for _, field := range []struct {
		name     string
		expected string
		actual   string
	}{
		{
			name:     pathField,
			expected: strings.Join(strings.Fields(command.Path), " "),
			actual:   strings.Join(strings.Fields(identity), " "),
		},
		{
			name:     descriptionField,
			expected: markdownInlineVisibleText(markdownLiteralProse(command.Description)),
			actual:   markdownInlineVisibleText(description),
		},
	} {
		if field.actual != field.expected {
			issues = append(issues, generatedCommandValueIssue(
				path, command.Path, field.name, field.expected, field.actual,
			))
		}
	}
	segments, segmentsValid := splitMarkdownOutsideCode(meta, "; ")
	if !segmentsValid {
		issues = append(issues, malformedCommandContainerIssue(
			path, command.Path, "llms.txt command metadata",
		))
		return issues
	}
	mode := ""
	if len(segments) != 0 {
		mode = normalizedCommandMode(markdownInlineVisibleText(segments[0]))
	}
	expectedMode := normalizedCommandMode(command.Mode)
	if mode != expectedMode {
		issues = append(issues, generatedCommandValueIssue(
			path, command.Path, "mode", expectedMode, mode,
		))
	}
	issues = append(issues, validateCommandMetaSegments(path, command, segments)...)
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "wire method", splitNonEmpty(command.WireMethod, "\x00"),
		commandMetaGroups(meta, "wire", func(value string) []string {
			return []string{value}
		}),
	)...)
	for _, availability := range []string{
		availabilityAlways, availabilityWithRows, availabilityWhenStreaming, availabilityLocalOnly,
	} {
		issues = append(issues, compareCommandOperatorGroups(
			path, command.Path, availability,
			commandOperatorNames(command, availability),
			commandMetaPipeGroups(meta, availability),
		)...)
	}
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "answer shape", splitNonEmpty(command.AnswerShape, "\x00"),
		commandMetaGroups(meta, "shape", func(value string) []string {
			return []string{value}
		}),
	)...)
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "address field", command.AddressFields,
		commandMetaGroups(meta, "address-fields", strings.Fields),
	)...)
	expectedFilters := make([]string, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		expectedFilters = append(expectedFilters, filter.Name)
	}
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "command filter", expectedFilters,
		commandMetaGroups(meta, "filters", parseLLMSCodeValues),
	)...)
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases,
			rendered.Reset().Str(alias.Name).Byte('=').Str(alias.Expansion).String())
	}
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "pipe alias", expectedAliases,
		commandMetaGroups(meta, "aliases", parseLLMSAliases),
	)...)
	expectedArgs := make([]string, 0, len(command.Args))
	for _, arg := range command.Args {
		expectedArgs = append(expectedArgs,
			rendered.Reset().Str(arg.Name).Byte(':').Str(arg.Type).String())
	}
	issues = append(issues, compareCommandNamedGroups(
		path, command.Path, "argument", expectedArgs,
		commandMetaGroups(meta, "args", parseLLMSCommaValues),
	)...)
	return issues
}

func validateCommandMetaSegments(
	path string,
	command *publishedCommand,
	segments []string,
) []Issue {
	known := []string{
		"wire ", "pipes ", "shape ", "address-fields ", "filters ", "aliases ", "args ",
	}
	counts := make(map[string]int, len(known))
	var issues []Issue
	for _, segment := range segments[1:] {
		matched := ""
		for _, prefix := range known {
			if strings.HasPrefix(segment, prefix) {
				matched = prefix
				break
			}
		}
		if matched == "" {
			issues = append(issues, generatedCommandExtraIssue(
				path, command.Path, namedCommandDimension("metadata segment", segment),
			))
			continue
		}
		counts[matched]++
		if matched != "pipes " {
			continue
		}
		pipes := strings.TrimPrefix(segment, matched)
		pipeGroups, pipeGroupsValid := splitMarkdownOutsideCode(pipes, ", ")
		if !pipeGroupsValid {
			issues = append(issues, malformedCommandContainerIssue(
				path, command.Path, "llms.txt pipe metadata",
			))
			continue
		}
		for _, group := range pipeGroups {
			label, _, ok := strings.Cut(group, ": ")
			if !ok || !slices.Contains(commandOperatorAvailabilities[:], label) {
				issues = append(issues, generatedCommandExtraIssue(
					path, command.Path,
					namedCommandDimension("malformed pipe metadata group", group),
				))
			}
		}
	}
	for prefix, count := range counts {
		if count > 1 {
			issues = append(issues, generatedCommandSurfaceValueIssue(
				path, command.Path, strings.TrimSpace(prefix)+" metadata cardinality",
				"at most one", fmt.Sprintf("%d segments", count),
			))
		}
	}
	hasPipes := false
	for _, availability := range []string{
		availabilityAlways, availabilityWithRows, availabilityWhenStreaming, availabilityLocalOnly,
	} {
		hasPipes = hasPipes || len(commandOperatorNames(command, availability)) != 0
	}
	if !hasPipes && counts["pipes "] != 0 {
		issues = append(issues, generatedCommandExtraIssue(
			path, command.Path, "pipes metadata segment",
		))
	}
	return issues
}

func commandMetaPipeGroups(meta, availability string) [][]string {
	segments, valid := splitMarkdownOutsideCode(meta, "; ")
	if !valid {
		return nil
	}
	var groups [][]string
	for _, segment := range segments {
		pipes, ok := strings.CutPrefix(segment, "pipes ")
		if !ok {
			continue
		}
		pipeGroups, pipeGroupsValid := splitMarkdownOutsideCode(pipes, ", ")
		if !pipeGroupsValid {
			return nil
		}
		for _, group := range pipeGroups {
			label, values, ok := strings.Cut(group, ": ")
			if ok && label == availability {
				groups = append(groups, parseLLMSCodeValues(values))
			}
		}
	}
	return groups
}

func commandMetaGroups(
	meta, label string,
	parse func(string) []string,
) [][]string {
	segments, valid := splitMarkdownOutsideCode(meta, "; ")
	if !valid {
		return nil
	}
	var marker textbuf.Buffer
	prefix := marker.Str(label).Byte(' ').String()
	var groups [][]string
	for _, segment := range segments {
		if values, found := strings.CutPrefix(segment, prefix); found {
			groups = append(groups, parse(values))
		}
	}
	return groups
}

func normalizedCommandMode(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func compareRenderedCommandSurfaces(
	root, publicRoot, expectedRoot string,
	live []publishedCommand,
) []Issue {
	expected := map[string]bool{
		llmsSurfaceName:            true,
		"reference/cli/index.html": true,
		"reference/cli/index.md":   true,
	}
	equivalentsRoot := filepath.Join(expectedRoot, "reference", "command-equivalents")
	walkErr := filepath.WalkDir(equivalentsRoot,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			extension := filepath.Ext(path)
			if extension != ".html" {
				if extension != ".md" {
					return nil
				}
			}
			relative, err := filepath.Rel(expectedRoot, path)
			if err != nil {
				return err
			}
			expected[filepath.ToSlash(relative)] = true
			return nil
		})
	if walkErr != nil {
		return []Issue{{
			File:    commandSurfacePath(root, equivalentsRoot),
			Message: "could not enumerate generated command-equivalent surfaces",
			Detail:  walkErr.Error(),
		}}
	}

	issues := validateGeneratedCommandSurfaces(root, publicRoot, live)
	paths := make([]string, 0, len(expected))
	for path := range expected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		publishedPath := filepath.Join(publicRoot, filepath.FromSlash(relative))
		if _, err := os.ReadFile(publishedPath); err != nil { //nolint:gosec // generated sibling artifact
			issues = append(issues, Issue{
				File:    commandSurfacePath(root, publishedPath),
				Message: "the published per-command surface is missing or unreadable",
				Detail:  err.Error(),
			})
		}
	}

	publicEquivalentsRoot := filepath.Join(publicRoot, "reference", "command-equivalents")
	if _, err := os.Stat(publicEquivalentsRoot); err != nil {
		return issues
	}
	walkErr = filepath.WalkDir(publicEquivalentsRoot,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			extension := filepath.Ext(path)
			if extension != ".html" {
				if extension != ".md" {
					return nil
				}
			}
			relative, err := filepath.Rel(publicRoot, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if expected[relative] {
				return nil
			}
			issues = append(issues, Issue{
				File:    commandSurfacePath(root, path),
				Message: "the published command-equivalent surface is stale",
				Detail:  "the live command catalog no longer generates this file",
			})
			return nil
		})
	if walkErr != nil {
		issues = append(issues, Issue{
			File:    commandSurfacePath(root, publicEquivalentsRoot),
			Message: "could not enumerate published command-equivalent surfaces",
			Detail:  walkErr.Error(),
		})
	}
	return issues
}

func commandSurfacePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
