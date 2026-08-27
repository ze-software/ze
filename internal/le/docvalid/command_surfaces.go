// Overview: drift.go -- registration in the documentation drift gate
//
// command_surfaces.go regenerates every command-facing document from the live
// command catalog, validates each rendered command container structurally, and
// compares sibling publications when those checkouts are present.

package docvalid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func commandSurfaceModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

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

type publishedCommand struct {
	Path          string                     `json:"path"`
	Description   string                     `json:"description,omitempty"`
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
	Syntax        string                     `json:"syntax,omitempty"`
	Subcommands   []string                   `json:"subcommands,omitempty"`
}

const commandCatalogGenerationTimeout = 2 * time.Minute

var commandSurfaceRendererFiles = []string{
	"models.py",
	"page_registry.py",
	"render-cli-catalog.py",
	"render-command-equivalents.py",
	"render-llms-txt.py",
	"sitefacts.py",
	"sitelib.py",
	"sitepaths.py",
	"zebinary.py",
}

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
	websiteCandidate := filepath.Join(filepath.Dir(root), "gh-pages", "data", "cli-commands.json")
	wikiCandidate := filepath.Join(filepath.Dir(root), "wiki", "command-catalog.md")
	if commandCatalogPath != "" {
		websiteCandidate = filepath.Join(root, "website", "data", "cli-commands.json")
		wikiCandidate = filepath.Join(root, "wiki", "command-catalog.md")
	}

	liveRaw, live, err := loadLiveCommandCatalog(root, commandCatalogPath)
	if err != nil {
		return []Issue{{
			File:    "cmd/ze/help_command.go",
			Message: "could not generate or parse the live per-command catalog",
			Detail:  err.Error(),
		}}
	}
	expectedWiki, err := renderExpectedWikiCommandSurface(root, commandCatalogPath, liveRaw)
	if err != nil {
		return []Issue{{
			File:    "scripts/dev/gen_wiki_commands.py",
			Message: "could not generate the expected wiki command catalog",
			Detail:  err.Error(),
		}}
	}
	issues := validateGeneratedWikiCommandSurface(expectedWiki, live)
	wikiPaths, wikiPathErr := existingPaths(wikiCandidate)
	if wikiPathErr != nil {
		issues = append(issues,
			commandSurfaceReadIssue("wiki command catalog", wikiPathErr))
	}
	for _, path := range wikiPaths {
		issues = append(issues, compareWikiCommandCatalog(root, path, expectedWiki)...)
	}

	websitePaths, websitePathErr := existingPaths(websiteCandidate)
	if websitePathErr != nil {
		return append(issues,
			commandSurfaceReadIssue("website command catalog", websitePathErr))
	}
	if len(websitePaths) == 0 {
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
				Detail:  "regenerate the website command surfaces before running ze-doc-verify",
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
			File:    "website/tools",
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
	root, commandCatalogPath, publicWebsiteRoot string,
	liveRaw []byte,
	commandCount int,
) (string, error) {
	moduleRoot, err := commandSurfaceModuleRoot()
	if err != nil {
		return "", fmt.Errorf("locate command renderers: %w", err)
	}
	tmpParent := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		return "", fmt.Errorf("create command render temporary parent %s: %w", tmpParent, err)
	}
	outputRoot, err := os.MkdirTemp(tmpParent, "docvalid-command-surfaces-")
	if err != nil {
		return "", fmt.Errorf("create command render temporary root: %w", err)
	}
	if err := prepareCommandRenderer(
		root, moduleRoot, commandCatalogPath, publicWebsiteRoot, outputRoot, liveRaw, commandCount,
	); err != nil {
		if removeErr := os.RemoveAll(outputRoot); removeErr != nil {
			return "", fmt.Errorf("%w; remove failed renderer output %s: %v",
				err, outputRoot, removeErr)
		}
		return "", err
	}
	for _, name := range []string{
		"render-cli-catalog.py",
		"render-command-equivalents.py",
		"render-llms-txt.py",
	} {
		if err := runCommandSurfaceRenderer(moduleRoot, outputRoot, name); err != nil {
			if removeErr := os.RemoveAll(outputRoot); removeErr != nil {
				return "", fmt.Errorf("%w; remove failed renderer output %s: %v",
					err, outputRoot, removeErr)
			}
			return "", err
		}
	}
	return outputRoot, nil
}

func prepareCommandRenderer(
	root, moduleRoot, commandCatalogPath, publicWebsiteRoot, outputRoot string,
	liveRaw []byte,
	commandCount int,
) error {
	toolsOutput := filepath.Join(outputRoot, "tools")
	if err := os.MkdirAll(toolsOutput, 0o755); err != nil {
		return fmt.Errorf("create temporary renderer tools directory: %w", err)
	}
	for _, name := range commandSurfaceRendererFiles {
		source := filepath.Join(moduleRoot, "website", "tools", name)
		override := filepath.Join(root, "website", "tools", name)
		if commandCatalogPath != "" {
			exists, err := optionalCommandSurfacePath(override)
			if err != nil {
				return fmt.Errorf("inspect command renderer override %s: %w", override, err)
			}
			if exists {
				source = override
			}
		}
		if err := copyCommandSurfaceFile(source, filepath.Join(toolsOutput, name)); err != nil {
			return fmt.Errorf("copy command renderer %s: %w", name, err)
		}
	}

	dataSource := filepath.Join(moduleRoot, "website", "data")
	if publicWebsiteRoot != "" {
		if commandCatalogPath == "" {
			dataSource = filepath.Join(publicWebsiteRoot, "data")
		}
	}
	dataOutput := filepath.Join(outputRoot, "data")
	if err := copyCommandSurfaceTree(dataSource, dataOutput, false); err != nil {
		return fmt.Errorf("copy command renderer data from %s: %w", dataSource, err)
	}
	if commandCatalogPath != "" {
		if err := writeCommandSurfaceFixtureData(dataOutput, commandCount); err != nil {
			return err
		}
	}
	if publicWebsiteRoot == "" {
		if err := writeMissingCommandSurfaceData(dataOutput, commandCount); err != nil {
			return err
		}
	}
	if err := os.WriteFile(
		filepath.Join(dataOutput, "cli-commands.json"), liveRaw, 0o644,
	); err != nil {
		return fmt.Errorf("write live command catalog for renderers: %w", err)
	}

	useCasesSource := filepath.Join(moduleRoot, "website", "use-cases")
	if publicWebsiteRoot != "" {
		publishedUseCases := filepath.Join(publicWebsiteRoot, "use-cases")
		exists, err := optionalCommandSurfacePath(publishedUseCases)
		if err != nil {
			return fmt.Errorf("inspect published use-case sources %s: %w", publishedUseCases, err)
		}
		if exists {
			useCasesSource = publishedUseCases
		}
	}
	if err := copyCommandSurfaceTree(
		useCasesSource, filepath.Join(outputRoot, "use-cases"), true,
	); err != nil {
		return fmt.Errorf("copy command renderer use-case sources: %w", err)
	}
	return nil
}

func writeCommandSurfaceFixtureData(dataRoot string, commandCount int) error {
	mapping := `{
  "schema-version": 1,
  "summary": "Docvalid renderer fixture.",
  "vendors": {
    "fixture": {
      "label": "Fixture",
      "short-label": "Fixture",
      "rooting-model": "fixture-rooted",
      "documentation": []
    }
  },
  "entries": []
}
`
	if err := os.WriteFile(
		filepath.Join(dataRoot, "command-equivalents.json"), []byte(mapping), 0o644,
	); err != nil {
		return fmt.Errorf("write command-equivalents renderer fixture: %w", err)
	}
	return writeMissingCommandSurfaceData(dataRoot, commandCount)
}

func writeMissingCommandSurfaceData(dataRoot string, commandCount int) error {
	var facts textbuf.Buffer
	facts.Str(`{
  "features": {"core_experimental": 0, "planned": 0},
  "tests": {"unit_display": "0", "fuzz_display": "0", "e2e_display": "0"},
  "interop": {"scenarios": 0, "target_display": "0"},
  "cli_commands": `).Int(int64(commandCount)).Str(`,
  "config_sections": 0,
  "dependencies": 0,
  "changes": 0,
  "blog_articles": 0,
  "generated_at": "docvalid fixture"
}
`)
	for name, content := range map[string]string{
		"plugin-registry.json":  "[]\n",
		"site-facts.json":       facts.String(),
		"yang-config-tree.json": "{}\n",
	} {
		path := filepath.Join(dataRoot, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s renderer fixture: %w", name, err)
		}
	}
	return nil
}

func copyCommandSurfaceTree(source, target string, markdownOnly bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if markdownOnly {
			if filepath.Ext(path) != ".md" {
				return nil
			}
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		return copyCommandSurfaceFile(path, targetPath)
	})
}

func copyCommandSurfaceFile(source, target string) error {
	data, err := os.ReadFile(source) //nolint:gosec // repository renderer or generated public artifact
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644) //nolint:gosec // isolated temporary renderer output
}

func runCommandSurfaceRenderer(moduleRoot, outputRoot, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandCatalogGenerationTimeout)
	defer cancel()
	path := filepath.Join(outputRoot, "tools", name)
	cmd := osexec.CommandContext(ctx, "python3", path)
	cmd.Dir = outputRoot
	var envValue textbuf.Buffer
	mainRepoEnv := envValue.Str("ZE_MAIN_REPO=").Str(moduleRoot).String()
	envValue.Reset()
	repoRootEnv := envValue.Str("ZE_REPO_ROOT=").Str(moduleRoot).String()
	envValue.Reset()
	siteOutputEnv := envValue.Str("ZE_SITE_OUTPUT=").Str(outputRoot).String()
	cmd.Env = append(os.Environ(),
		"PYTHONDONTWRITEBYTECODE=1",
		"ZE_CLI_CATALOG_USE_CACHE=1",
		mainRepoEnv,
		repoRootEnv,
		siteOutputEnv,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run canonical command renderer %s: %w: %s",
			name, err, strings.TrimSpace(string(out)))
	}
	return nil
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
	for _, relative := range []string{"data", "reference", "llms.txt"} {
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
	data, err := os.ReadFile(filepath.Join(root, "feature-gates.txt"))
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
	for _, entry := range commands {
		if err := validatePublishedCommand(source, entry, seen); err != nil {
			return nil, err
		}
		seen[entry.Path] = true
	}
	return commands, nil
}

func validatePublishedCommand(source string, entry publishedCommand, seen map[string]bool) error {
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
	for _, arg := range entry.Args {
		if arg.Name == "" {
			return fmt.Errorf("parse %s: command %q has an argument without a name", source, entry.Path)
		}
		if arg.Type == "" {
			return fmt.Errorf("parse %s: command %q argument %q has no kind", source, entry.Path, arg.Name)
		}
	}
	for _, pipe := range entry.Pipes {
		if pipe.Name == "" {
			return fmt.Errorf("parse %s: command %q has a filter without a name", source, entry.Path)
		}
		if pipe.Description == "" {
			return fmt.Errorf("parse %s: command %q filter %q has no description", source, entry.Path, pipe.Name)
		}
	}
	for _, field := range entry.AddressFields {
		if field == "" {
			return fmt.Errorf("parse %s: command %q has an empty address field", source, entry.Path)
		}
	}
	for _, op := range entry.Operators {
		if op.Name == "" {
			return fmt.Errorf("parse %s: command %q has an operator without a name", source, entry.Path)
		}
		if op.Class == "" {
			return fmt.Errorf("parse %s: command %q operator %q has no class", source, entry.Path, op.Name)
		}
		if op.Description == "" {
			return fmt.Errorf("parse %s: command %q operator %q has no description", source, entry.Path, op.Name)
		}
		switch op.Available {
		case "always", "with-rows", "when-streaming":
		default:
			return fmt.Errorf("parse %s: command %q operator %q has unknown availability %q",
				source, entry.Path, op.Name, op.Available)
		}
	}
	for _, alias := range entry.Aliases {
		if alias.Name == "" {
			return fmt.Errorf("parse %s: command %q has an alias without a name", source, entry.Path)
		}
		if alias.Description == "" {
			return fmt.Errorf("parse %s: command %q alias %q has no description", source, entry.Path, alias.Name)
		}
		if alias.Expansion == "" {
			return fmt.Errorf("parse %s: command %q alias %q has no expansion", source, entry.Path, alias.Name)
		}
	}
	return nil
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
		Detail: "regenerate the website CLI surface; every command's operators, qualifiers, aliases, " +
			"filters, shape, address fields, descriptions, and argument kinds must match",
	}}
}

func renderExpectedWikiCommandSurface(
	root, commandCatalogPath string,
	liveRaw []byte,
) ([]byte, error) {
	moduleRoot, err := commandSurfaceModuleRoot()
	if err != nil {
		return nil, fmt.Errorf("locate wiki command generator: %w", err)
	}
	generator := filepath.Join(moduleRoot, "scripts", "dev", "gen_wiki_commands.py")
	if commandCatalogPath != "" {
		override := filepath.Join(root, "scripts", "dev", "gen_wiki_commands.py")
		exists, inspectErr := optionalCommandSurfacePath(override)
		if inspectErr != nil {
			return nil, fmt.Errorf("inspect wiki command generator override %s: %w",
				override, inspectErr)
		}
		if exists {
			generator = override
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandCatalogGenerationTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "python3", generator)
	cmd.Dir = moduleRoot
	cmd.Stdin = bytes.NewReader(liveRaw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	generated, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run wiki command generator: %w: %s",
			err, strings.TrimSpace(stderr.String()))
	}
	if len(generated) == 0 {
		return nil, fmt.Errorf("run wiki command generator: empty output")
	}
	return generated, nil
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
		Detail:  "run `make ze-wiki-commands-update`; the wiki must preserve every per-command contract field",
	}}
}

func validateGeneratedWikiCommandSurface(
	generated []byte,
	live []publishedCommand,
) []Issue {
	const surface = "scripts/dev/gen_wiki_commands.py"
	content := string(generated)
	var issues []Issue
	var rendered textbuf.Buffer
	expectedPaths := make([]string, 0, len(live))
	expectedDetailPaths := make([]string, 0, len(live))
	for _, command := range live {
		expectedPaths = append(expectedPaths, command.Path)
		if wikiCommandNeedsDetail(command) {
			expectedDetailPaths = append(expectedDetailPaths, command.Path)
		}
	}
	issues = append(issues, compareCommandNamedGroup(
		surface, "<wiki catalog>", "command", expectedPaths,
		wikiCommandSummaryPaths(content),
	)...)
	issues = append(issues, compareCommandNamedGroup(
		surface, "<wiki catalog>", "command detail", expectedDetailPaths,
		wikiCommandDetailPaths(content),
	)...)
	for _, command := range live {
		description := command.Description
		if line, _, found := strings.Cut(description, "\n"); found {
			description = line
		}
		description = strings.ReplaceAll(description, "|", `\|`)
		wantRow := rendered.Reset().Str("| `").Str(command.Path).Str("` | ").
			Str(command.Mode).Str(" | ").Str(description).Str(" |").String()
		row, rowCount, rowMalformed := commandSurfaceMarkdownRow(content, command.Path)
		if rowCount != 1 || rowMalformed || row != wantRow {
			issues = append(issues, generatedCommandContractIssue(
				surface, command.Path, "wiki command summary row",
			))
		}

		if !wikiCommandNeedsDetail(command) {
			continue
		}
		detail, detailCount := wikiCommandDetail(content, command.Path)
		if detailCount != 1 {
			issues = append(issues, commandContainerCountIssue(
				surface, command.Path, "wiki command detail section", detailCount,
			))
			continue
		}
		if strings.Contains(command.Description, "\n") {
			wantDescription := rendered.Reset().Byte('\n').Str(command.Description).
				Str("\n\n").String()
			if !strings.HasPrefix(detail, wantDescription) {
				issues = append(issues, generatedCommandContractIssue(
					surface, command.Path, "wiki command description",
				))
			}
		}
		rendered.Reset().Str("Mode: ").Str(command.Mode)
		if command.WireMethod != "" {
			rendered.Str(" | Wire: `").Str(command.WireMethod).Byte('`')
		}
		if !wikiLineEquals(detail, rendered.String()) {
			issues = append(issues, generatedCommandContractIssue(
				surface, command.Path, "wiki mode and wire metadata",
			))
		}
		wantShape := ""
		if command.AnswerShape != "" {
			wantShape = rendered.Reset().Str("Answer shape: `").
				Str(command.AnswerShape).Byte('`').String()
		}
		issues = append(issues, compareWikiOptionalLine(
			surface, command.Path, detail, "Answer shape: ",
			"answer shape", wantShape,
		)...)
		wantAddressFields := ""
		if len(command.AddressFields) != 0 {
			wantAddressFields = rendered.Reset().Str("Address fields: ").
				Str(wikiCodeList(command.AddressFields)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			surface, command.Path, detail, "Address fields: ",
			"address fields", wantAddressFields,
		)...)
		wantBackend := ""
		if len(command.Backend) != 0 {
			wantBackend = rendered.Reset().Str("**Requires backend:** ").
				Str(wikiCodeList(command.Backend)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			surface, command.Path, detail, "**Requires backend:** ",
			"backend requirements", wantBackend,
		)...)
		wantTaskSupport := ""
		if command.TaskSupport != "" {
			wantTaskSupport = rendered.Reset().Str("**Task support:** ").
				Str(command.TaskSupport).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			surface, command.Path, detail, "**Task support:** ",
			"task support", wantTaskSupport,
		)...)
		expectedArgs := make([]string, 0, len(command.Args))
		for _, arg := range command.Args {
			required := ""
			if arg.Mandatory {
				required = "yes"
			}
			rendered.Reset().Str("| `").Str(arg.Name).Str("` | ").Str(arg.Type).
				Str(" | ").Str(required).Str(" | ").
				Str(strings.Join(arg.Values, ", ")).Str(" |")
			expectedArgs = append(expectedArgs, rendered.String())
		}
		issues = append(issues, compareCommandNamedGroup(
			surface, command.Path, "wiki argument", expectedArgs,
			wikiArgumentRows(detail),
		)...)

		for _, group := range []struct {
			availability string
			label        string
		}{
			{availability: "always", label: "Always"},
			{availability: "with-rows", label: "When the answer has rows"},
			{availability: "when-streaming", label: "While the command keeps answering"},
			{availability: "local-only", label: "Local process only"},
		} {
			actual := wikiOperatorGroups(detail, group.label)
			expectedNames := commandOperatorNames(command, group.availability)
			issues = append(issues, compareCommandOperatorGroups(
				surface, command.Path, group.availability, expectedNames, actual,
			)...)
		}
		expectedAliases := make([]string, 0, len(command.Aliases))
		for _, alias := range command.Aliases {
			rendered.Reset().Str("- `").Str(alias.Name).Str("` -- ").
				Str(alias.Description).Str(" (`").Str(alias.Expansion).Str("`)")
			expectedAliases = append(expectedAliases, rendered.String())
		}
		issues = append(issues, compareCommandNamedGroup(
			surface, command.Path, "wiki pipe alias", expectedAliases,
			wikiBulletGroup(detail, "Named chains:"),
		)...)
		expectedFilters := make([]string, 0, len(command.Pipes))
		for _, filter := range command.Pipes {
			rendered.Reset().Str("- `").Str(filter.Name).Byte('`')
			if filter.TakesArg {
				rendered.Str(" `<value>`")
			}
			rendered.Str(" -- ").Str(filter.Description)
			expectedFilters = append(expectedFilters, rendered.String())
		}
		issues = append(issues, compareCommandNamedGroup(
			surface, command.Path, "wiki command filter", expectedFilters,
			wikiBulletGroup(detail, "Command-specific:"),
		)...)
		wantSubcommands := ""
		if len(command.Subcommands) != 0 {
			wantSubcommands = rendered.Reset().Str("**Subcommands:** ").
				Str(wikiCodeList(command.Subcommands)).String()
		}
		issues = append(issues, compareWikiOptionalLine(
			surface, command.Path, detail, "**Subcommands:** ",
			"subcommands", wantSubcommands,
		)...)
	}
	return issues
}

func wikiCommandNeedsDetail(command publishedCommand) bool {
	return len(command.Args) != 0 ||
		len(command.Pipes) != 0 ||
		len(command.Aliases) != 0 ||
		len(command.Subcommands) != 0 ||
		len(command.Backend) != 0 ||
		command.WireMethod != "" ||
		command.TaskSupport != "" ||
		len(command.Operators) != 0 ||
		command.AnswerShape != "" ||
		len(command.AddressFields) != 0 ||
		strings.Contains(command.Description, "\n")
}

type markdownLine struct {
	text         string
	active       bool
	headingLevel int
	heading      string
}

func scanMarkdownLines(content string) []markdownLine {
	rawLines := strings.Split(content, "\n")
	lines := make([]markdownLine, 0, len(rawLines))
	var fence byte
	fenceWidth := 0
	for _, text := range rawLines {
		marker, width, rest := markdownFence(text)
		if fence != 0 {
			lines = append(lines, markdownLine{text: text})
			if marker == fence && width >= fenceWidth && strings.TrimSpace(rest) == "" {
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
		level, heading := markdownATXHeading(text)
		lines = append(lines, markdownLine{
			text:         text,
			active:       true,
			headingLevel: level,
			heading:      heading,
		})
	}
	return lines
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
	if end := strings.LastIndex(heading, " #"); end != -1 {
		suffix := strings.TrimSpace(heading[end+1:])
		if suffix != "" && strings.Trim(suffix, "#") == "" {
			heading = strings.TrimSpace(heading[:end])
		}
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
	var rendered strings.Builder
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return strings.Join(strings.Fields(rendered.String()), " ")
		case xhtml.TextToken:
			rendered.WriteString(tokenizer.Token().Data)
		}
	}
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
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) >= 2 && value[0] == ' ' && value[len(value)-1] == ' ' &&
		strings.Trim(value, " ") != "" {
		return value[1 : len(value)-1]
	}
	return value
}

func markdownFirstCodeCell(line string) (string, bool) {
	if !strings.HasPrefix(line, "| `") {
		return "", false
	}
	remaining := line[len("| `"):]
	end := strings.Index(remaining, "` |")
	if end == -1 {
		return "", false
	}
	return strings.ReplaceAll(remaining[:end], `\|`, "|"), true
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

func wikiBulletGroup(content, heading string) []string {
	lines := activeMarkdownLines(content)
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
		return rows
	}
	return nil
}

func wikiLineEquals(content, want string) bool {
	for _, line := range scanMarkdownLines(content) {
		if line.active && line.text == want {
			return true
		}
	}
	return false
}

func compareWikiOptionalLine(
	path, command, content, prefix, field, expected string,
) []Issue {
	var rendered textbuf.Buffer
	actual := ""
	for _, line := range scanMarkdownLines(content) {
		if line.active && strings.HasPrefix(line.text, prefix) {
			actual = line.text
			break
		}
	}
	if actual == expected {
		return nil
	}
	fieldName := rendered.Str("wiki ").Str(field).String()
	return []Issue{generatedCommandSurfaceValueIssue(
		path, command, fieldName, expected, actual,
	)}
}

func wikiCodeList(values []string) string {
	quoted := make([]string, 0, len(values))
	var rendered textbuf.Buffer
	for _, value := range values {
		quoted = append(quoted,
			rendered.Reset().Byte('`').Str(value).Byte('`').String())
	}
	return strings.Join(quoted, ", ")
}

func wikiOperatorGroups(content, label string) [][]string {
	var marker textbuf.Buffer
	prefix := marker.Str(label).Str(": ").String()
	var groups [][]string
	for _, line := range scanMarkdownLines(content) {
		if !line.active || !strings.HasPrefix(line.text, prefix) {
			continue
		}
		values := strings.TrimPrefix(line.text, prefix)
		if before, _, found := strings.Cut(values, " -- "); found {
			values = before
		}
		var names []string
		for _, value := range strings.Split(values, ", ") {
			value = strings.Trim(value, "`")
			if value != "" {
				names = append(names, value)
			}
		}
		groups = append(groups, names)
	}
	return groups
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
	llmsPath := filepath.Join(expectedRoot, "llms.txt")
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

	var issues []Issue
	for _, command := range live {
		slug := commandSurfaceSlug(command.Path)
		var equivalentHTMLMarker textbuf.Buffer
		if !strings.Contains(string(equivalentHTML),
			equivalentHTMLMarker.Str(`id="cmd-eq-`).Str(slug).Byte('"').String()) {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, equivalentHTMLPath), command.Path,
				"command-equivalent HTML index row",
			))
		}
		var equivalentMarkdownMarker textbuf.Buffer
		if !strings.Contains(string(equivalentMarkdown),
			equivalentMarkdownMarker.Str("](").Str(slug).Str("/)").String()) {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, equivalentMarkdownPath), command.Path,
				"command-equivalent Markdown index row",
			))
		}
		primaryRow, rowCount, rowClosed := commandSurfaceHTMLRow(
			primaryHTMLDocument, slug,
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
		if markdownRowCount != 1 {
			issues = append(issues, commandContainerCountIssue(
				commandSurfacePath(root, primaryMarkdownPath), command.Path,
				"primary CLI Markdown command row", markdownRowCount,
			))
		} else if markdownRowMalformed {
			issues = append(issues, malformedCommandContainerIssue(
				commandSurfacePath(root, primaryMarkdownPath), command.Path,
				"primary CLI Markdown command row",
			))
		} else {
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

		meta, metadataCount, metadataClosed := llmsCommandMetadata(
			string(llms), command.Path,
		)
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
					commandSurfacePath(root, llmsPath), meta, command,
				)...)
		}
	}
	issues = append(issues, validatePrimaryOperatorCatalog(
		commandSurfacePath(root, primaryHTMLPath), string(primaryHTML), live,
	)...)
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

func commandOperatorNames(command publishedCommand, availability string) []string {
	names := make([]string, 0, len(command.Operators))
	for _, operator := range command.Operators {
		if availability == "local-only" {
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
		if availability == "local-only" {
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

func compareCommandNamedGroup(
	path, command, kind string,
	expected, actual []string,
) []Issue {
	var issues []Issue
	var rendered textbuf.Buffer
	missing, extra := commandNameDifferences(expected, actual)
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

var primaryOperatorRowPattern = regexp.MustCompile(
	`<tr><td><code>([^<]*)</code></td><td>([^<]*)</td><td>([^<]*)</td><td>([^<]*)</td></tr>`,
)

func validatePrimaryOperatorCatalog(
	path, content string,
	live []publishedCommand,
) []Issue {
	expected := make(map[string]renderedOperatorMetadata)
	for _, command := range live {
		for _, operator := range command.Operators {
			meta, exists := expected[operator.Name]
			if !exists {
				meta.class = primaryOperatorClassLabel(operator.Class)
				meta.description = operator.Description
			}
			availability := commandAvailabilityLabel(operator.Available)
			if !containsString(meta.available, availability) {
				meta.available = append(meta.available, availability)
			}
			if operator.LocalOnly && !containsString(meta.available, "Local process only") {
				meta.available = append(meta.available, "Local process only")
			}
			expected[operator.Name] = meta
		}
	}

	guide := ""
	if start := strings.Index(content, `<section class="cli-pipe-guide"`); start != -1 {
		remaining := content[start:]
		if end := strings.Index(remaining, "</section>"); end != -1 {
			guide = remaining[:end+len("</section>")]
		}
	}
	actual := make(map[string]renderedOperatorMetadata)
	var duplicate []string
	for _, match := range primaryOperatorRowPattern.FindAllStringSubmatch(guide, -1) {
		name := html.UnescapeString(match[1])
		if _, exists := actual[name]; exists {
			duplicate = append(duplicate, name)
			continue
		}
		actual[name] = renderedOperatorMetadata{
			class:       html.UnescapeString(match[2]),
			available:   splitNonEmpty(html.UnescapeString(match[3]), ", "),
			description: html.UnescapeString(match[4]),
		}
	}

	expectedNames := make([]string, 0, len(expected))
	actualNames := make([]string, 0, len(actual)+len(duplicate))
	for name := range expected {
		expectedNames = append(expectedNames, name)
	}
	for name := range actual {
		actualNames = append(actualNames, name)
	}
	actualNames = append(actualNames, duplicate...)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func splitNonEmpty(value, separator string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, separator)
}

func commandSurfaceSlug(path string) string {
	slug := commandSurfaceSlugSeparator.ReplaceAllString(strings.ToLower(path), "-")
	return strings.Trim(slug, "-")
}

type renderedHTMLDocument struct {
	rows       map[string][]renderedHTMLContainer
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
	root       *xhtml.Node
}

type renderedHTMLOpenElement struct {
	name string
	node *xhtml.Node
}

func parseRenderedHTML(content string) renderedHTMLDocument {
	document := renderedHTMLDocument{
		rows:       make(map[string][]renderedHTMLContainer),
		nodeClosed: make(map[*xhtml.Node]bool),
	}
	tokenizer := xhtml.NewTokenizer(strings.NewReader(content))
	var capture *renderedHTMLCapture
	var openElements []renderedHTMLOpenElement
	finish := func(closed bool) {
		if capture == nil {
			return
		}
		container := renderedHTMLContainer{
			root:       capture.root,
			closed:     closed,
			classValid: capture.classValid,
		}
		switch capture.kind {
		case "tr":
			document.rows[htmlAttribute(capture.root, "id")] = append(
				document.rows[htmlAttribute(capture.root, "id")], container,
			)
		case "article":
			document.zeArticles = append(document.zeArticles, container)
		}
		capture = nil
	}
	for {
		tokenType := tokenizer.Next()
		if tokenType == xhtml.ErrorToken {
			finish(false)
			if err := tokenizer.Err(); err != io.EOF {
				document.err = err
			}
			return document
		}
		raw := tokenizer.Raw()
		token := tokenizer.Token()
		switch tokenType {
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			if htmlStartTokenMalformed(raw) {
				finish(false)
				document.err = fmt.Errorf("malformed HTML start tag %q", raw)
				return document
			}
			var node *xhtml.Node
			if capture == nil {
				switch token.Data {
				case "tr":
					if id := tokenAttribute(token, "id"); strings.HasPrefix(id, "cmd-") {
						node = htmlTokenNode(token)
						capture = &renderedHTMLCapture{kind: "tr", root: node}
					}
				case "article":
					classValid, candidate := zeArticleClass(tokenAttribute(token, "class"))
					if candidate {
						node = htmlTokenNode(token)
						capture = &renderedHTMLCapture{
							kind:       "article",
							classValid: classValid,
							root:       node,
						}
					}
				}
			}
			if capture != nil {
				if node == nil {
					node = htmlTokenNode(token)
					htmlAppendChild(openElements[len(openElements)-1].node, node)
				}
				closed := tokenType == xhtml.SelfClosingTagToken || htmlVoidElement(token.Data)
				document.nodeClosed[node] = closed
				if closed {
					if node == capture.root {
						finish(false)
					}
					continue
				}
			}
			if tokenType != xhtml.SelfClosingTagToken && !htmlVoidElement(token.Data) {
				openElements = append(openElements, renderedHTMLOpenElement{
					name: token.Data,
					node: node,
				})
			}
		case xhtml.TextToken:
			if capture != nil {
				htmlAppendChild(openElements[len(openElements)-1].node, &xhtml.Node{
					Type: xhtml.TextNode,
					Data: token.Data,
				})
			}
		case xhtml.CommentToken:
			if capture != nil {
				htmlAppendChild(openElements[len(openElements)-1].node, &xhtml.Node{
					Type: xhtml.CommentNode,
					Data: token.Data,
				})
			}
		case xhtml.EndTagToken:
			match := -1
			for index := len(openElements) - 1; index >= 0; index-- {
				if openElements[index].name == token.Data {
					match = index
					break
				}
			}
			if match == -1 {
				continue
			}
			rootPopped := false
			rootMatched := false
			for index := len(openElements) - 1; index >= match; index-- {
				node := openElements[index].node
				if node == nil {
					continue
				}
				document.nodeClosed[node] = index == match
				if capture != nil && node == capture.root {
					rootPopped = true
					rootMatched = index == match
				}
			}
			openElements = openElements[:match]
			if rootPopped {
				finish(rootMatched)
			}
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
	hasCard := false
	hasZe := false
	partialZe := false
	for _, name := range strings.Fields(class) {
		hasCard = hasCard || name == "cmd-detail-card"
		hasZe = hasZe || name == "cmd-detail-ze"
		partialZe = partialZe ||
			(strings.HasPrefix(name, "cmd-detail-ze") && name != "cmd-detail-ze")
	}
	return hasCard && hasZe, hasCard && (hasZe || partialZe)
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
	slug string,
) (*xhtml.Node, int, bool) {
	if document.err != nil {
		return nil, 1, false
	}
	var rendered textbuf.Buffer
	rows := document.rows[rendered.Str("cmd-").Str(slug).String()]
	if len(rows) != 1 {
		return nil, len(rows), false
	}
	return rows[0].root, len(rows), rows[0].closed
}

func validatePrimaryCommandContract(
	path string,
	row *xhtml.Node,
	document renderedHTMLDocument,
	command publishedCommand,
) []Issue {
	rowHTML := htmlNodeString(row)
	var issues []Issue
	var marker textbuf.Buffer
	if command.AnswerShape != "" {
		want := marker.Str("<span>Answer shape</span><code>").
			Str(html.EscapeString(command.AnswerShape)).Str("</code>").String()
		if !strings.Contains(rowHTML, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	if len(command.AddressFields) != 0 {
		marker.Reset()
		want := marker.Str("<span>Address fields</span><code>").
			Str(html.EscapeString(strings.Join(command.AddressFields, " · "))).
			Str("</code>").String()
		if !strings.Contains(rowHTML, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "address fields",
			))
		}
	}
	for _, group := range []struct {
		availability string
		label        string
	}{
		{availability: "always", label: "Always"},
		{availability: "with-rows", label: "With rows"},
		{availability: "when-streaming", label: "While streaming"},
		{availability: "local-only", label: "Local process only"},
	} {
		scan := commandHTMLGroups(document, row, group.label)
		issues = append(issues, compareScannedCommandOperatorGroups(
			path, command.Path, group.availability,
			commandOperatorNames(command, group.availability), scan,
		)...)
	}

	filterNames := htmlCodeNames(row, "Command pipes", "cli-pipe-chips")
	expectedFilters := make([]string, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Str(filter.Name)
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		name := marker.String()
		expectedFilters = append(expectedFilters, name)
		marker.Reset().Str("<dt><code>").Str(html.EscapeString(name)).
			Str("</code></dt><dd>").Str(html.EscapeString(filter.Description)).
			Str("</dd>")
		if !strings.Contains(rowHTML, marker.String()) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("command filter", filter.Name),
			))
		}
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "command filter", expectedFilters, filterNames,
	)...)

	aliasNames := htmlDefinitionNames(row, "Aliases")
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases, alias.Name)
		marker.Reset().Str("<dt><code>").Str(html.EscapeString(alias.Name)).
			Str("</code></dt><dd>").Str(html.EscapeString(alias.Description)).
			Byte(' ').Str("<code>").Str(html.EscapeString(alias.Expansion)).
			Str("</code></dd>")
		if !strings.Contains(rowHTML, marker.String()) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("pipe alias", alias.Name),
			))
		}
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "pipe alias", expectedAliases, aliasNames,
	)...)
	return issues
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
	path, suffix, width, closed := markdownCodeSpanPrefix(cell)
	path = strings.ReplaceAll(path, `\|`, "|")
	if !closed {
		return path, false, false
	}
	canonical := width == 1 && strings.HasPrefix(line, "| `") &&
		strings.HasPrefix(cell, "`"+strings.ReplaceAll(path, "|", `\|`)+"`") &&
		strings.HasPrefix(suffix, " |")
	return path, true, canonical
}

func markdownUnclosedPath(candidate, path string) bool {
	var rendered textbuf.Buffer
	return candidate == path ||
		strings.HasPrefix(candidate, rendered.Str(path).Str(" |").String())
}

func commandMarkdownValue(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func validatePrimaryMarkdownContract(
	path, row string,
	command publishedCommand,
) []Issue {
	var issues []Issue
	var marker textbuf.Buffer
	if command.AnswerShape != "" {
		want := marker.Str("Answer shape: `").
			Str(commandMarkdownValue(command.AnswerShape)).Byte('`').String()
		if !strings.Contains(row, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "address field", command.AddressFields,
		commandMarkdownGroupValues(row, "Address fields"),
	)...)
	for _, group := range []struct {
		availability string
		label        string
	}{
		{availability: "always", label: "Always"},
		{availability: "with-rows", label: "With rows"},
		{availability: "when-streaming", label: "While streaming"},
		{availability: "local-only", label: "Local process only"},
	} {
		issues = append(issues, compareCommandOperatorGroups(
			path, command.Path, group.availability,
			commandOperatorNames(command, group.availability),
			commandMarkdownGroups(row, group.label),
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
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "command filter", expectedFilters,
		commandMarkdownGroupValues(row, "Command"),
	)...)
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases,
			marker.Reset().Str(alias.Name).Str(" -> ").Str(alias.Expansion).String())
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "pipe alias", expectedAliases,
		commandMarkdownGroupValues(row, "Aliases"),
	)...)
	return issues
}

func commandMarkdownGroupValues(content, label string) []string {
	groups := commandMarkdownGroups(content, label)
	if len(groups) == 0 {
		return nil
	}
	return groups[0]
}

func commandMarkdownGroups(content, label string) [][]string {
	var marker textbuf.Buffer
	startMarker := marker.Str(label).Str(": ").String()
	var groups [][]string
	remaining := content
	for {
		start := strings.Index(remaining, startMarker)
		if start == -1 {
			return groups
		}
		values := remaining[start+len(startMarker):]
		end := len(values)
		if lineBreak := strings.Index(values, "<br>"); lineBreak != -1 {
			end = lineBreak
		}
		if cellEnd := strings.Index(values, " |"); cellEnd != -1 {
			end = min(end, cellEnd)
		}
		values = values[:end]
		parsed := make([]string, 0, strings.Count(values, ", ")+1)
		for _, value := range strings.Split(values, ", ") {
			value = strings.Trim(value, "`")
			if value != "" {
				parsed = append(parsed, strings.ReplaceAll(value, `\|`, "|"))
			}
		}
		groups = append(groups, parsed)
		remaining = remaining[start+len(startMarker):]
	}
}

func commandAvailabilityLabel(availability string) string {
	switch availability {
	case "always":
		return "Always"
	case "with-rows":
		return "With rows"
	case "when-streaming":
		return "While streaming"
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
		if node.Data != "span" || htmlText(node) != label {
			return
		}
		code := htmlNextElement(node)
		if !document.nodeClosed[node] || code == nil || code.Data != "code" ||
			!document.nodeClosed[code] {
			scan.malformed = true
			return
		}
		values := htmlText(code)
		var parsed []string
		if values != "" {
			parsed = strings.Split(values, " · ")
		}
		scan.groups = append(scan.groups, parsed)
	})
	return scan
}

func htmlCodeNames(root *xhtml.Node, label, class string) []string {
	var names []string
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "strong" || htmlText(node) != label {
			return
		}
		group := htmlNextElement(node)
		if group == nil || !htmlHasClass(group, class) {
			return
		}
		htmlWalk(group, func(child *xhtml.Node) {
			if child.Data == "code" {
				names = append(names, htmlText(child))
			}
		})
	})
	return names
}

func htmlDefinitionNames(root *xhtml.Node, label string) []string {
	var names []string
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "strong" || htmlText(node) != label {
			return
		}
		list := htmlNextElement(node)
		if list == nil || list.Data != "dl" {
			return
		}
		htmlWalk(list, func(child *xhtml.Node) {
			if child.Data != "dt" {
				return
			}
			code := htmlFirstElement(child)
			if code != nil && code.Data == "code" {
				names = append(names, htmlText(code))
			}
		})
	})
	return names
}

func htmlDefinitionValue(root *xhtml.Node, label string) string {
	value := ""
	htmlWalk(root, func(node *xhtml.Node) {
		if value != "" || node.Data != "dt" || htmlText(node) != label {
			return
		}
		definition := htmlNextElement(node)
		if definition != nil && definition.Data == "dd" {
			value = htmlInnerString(definition)
		}
	})
	return value
}

func htmlDefinitionCodeIdentity(
	root *xhtml.Node,
	label, expected string,
) (bool, bool, bool) {
	matches := false
	count := 0
	valid := true
	htmlWalk(root, func(node *xhtml.Node) {
		if node.Data != "dt" || htmlText(node) != label {
			return
		}
		count++
		definition := htmlFollowingDefinition(node)
		code := htmlFirstDescendant(definition, "code")
		if code != nil {
			matches = matches || normalizeRenderedHTMLText(htmlText(code)) == expected
		}
		directDefinition := htmlNextElement(node)
		var directCode *xhtml.Node
		if definition != nil {
			directCode = htmlFirstElement(definition)
		}
		if definition == nil || definition != directDefinition ||
			directCode == nil || directCode != code ||
			htmlNextElement(code) != nil ||
			strings.TrimSpace(htmlText(definition)) != htmlText(code) {
			valid = false
		}
	})
	return matches, count != 0, count == 1 && valid
}

func normalizeRenderedHTMLText(value string) string {
	fields := strings.FieldsFunc(value, func(current rune) bool {
		switch current {
		case ' ', '\t', '\n', '\r', '\f':
			return true
		default:
			return false
		}
	})
	return strings.Join(fields, " ")
}

func htmlFollowingDefinition(term *xhtml.Node) *xhtml.Node {
	for sibling := term.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type != xhtml.ElementNode {
			continue
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

func htmlFirstDescendant(root *xhtml.Node, element string) *xhtml.Node {
	var found *xhtml.Node
	if root != nil {
		htmlWalk(root, func(node *xhtml.Node) {
			if found == nil && node.Data == element {
				found = node
			}
		})
	}
	return found
}

func equivalentHTMLCommandContent(
	document renderedHTMLDocument,
	path string,
) (*xhtml.Node, int, bool) {
	if document.err != nil {
		return nil, 1, false
	}
	var matches []renderedHTMLContainer
	for _, article := range document.zeArticles {
		matchesExpected, present, valid := htmlDefinitionCodeIdentity(
			article.root, "Registry path", path,
		)
		if present && matchesExpected {
			article.classValid = article.classValid && valid
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
	for node := root; node != nil; {
		if node.Type == xhtml.ElementNode {
			visit(node)
		}
		if node.FirstChild != nil {
			node = node.FirstChild
			continue
		}
		for node != nil && node != root && node.NextSibling == nil {
			node = node.Parent
		}
		if node == nil || node == root {
			return
		}
		node = node.NextSibling
	}
}

func htmlText(root *xhtml.Node) string {
	var text strings.Builder
	var appendText func(*xhtml.Node)
	appendText = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
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
	for _, name := range strings.Fields(htmlAttribute(node, "class")) {
		if name == class {
			return true
		}
	}
	return false
}

func htmlNodeString(node *xhtml.Node) string {
	var rendered strings.Builder
	if err := xhtml.Render(&rendered, node); err != nil {
		return ""
	}
	return rendered.String()
}

func htmlInnerString(node *xhtml.Node) string {
	var rendered strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := xhtml.Render(&rendered, child); err != nil {
			return ""
		}
	}
	return strings.ReplaceAll(rendered.String(), "<br/>", "<br>")
}

func validateEquivalentCommandContract(
	path string,
	document renderedHTMLDocument,
	command publishedCommand,
) []Issue {
	commandNode, containerCount, containerClosed := equivalentHTMLCommandContent(
		document, command.Path,
	)
	switch {
	case containerCount != 1:
		return []Issue{commandContainerCountIssue(
			path, command.Path, "command-equivalent Ze HTML article", containerCount,
		)}
	case !containerClosed:
		return []Issue{malformedCommandContainerIssue(
			path, command.Path, "command-equivalent Ze HTML article",
		)}
	}
	content := htmlNodeString(commandNode)
	var issues []Issue
	var marker textbuf.Buffer
	if command.AnswerShape != "" {
		want := marker.Str("<dt>Answer shape</dt><dd>").
			Str(html.EscapeString(command.AnswerShape)).Str("</dd>").String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	if len(command.AddressFields) != 0 {
		marker.Reset()
		want := marker.Str("<dt>Address fields</dt><dd>").
			Str(html.EscapeString(strings.Join(command.AddressFields, ", "))).
			Str("</dd>").String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "address fields",
			))
		}
	}
	for _, group := range []struct {
		availability string
		label        string
	}{
		{availability: "always", label: "Pipes, always"},
		{
			availability: "with-rows",
			label:        equivalentAvailabilityLabel("with-rows", command.AnswerShape != ""),
		},
		{availability: "when-streaming", label: "Pipes, while streaming"},
		{availability: "local-only", label: "Pipes, local process only"},
	} {
		scan := equivalentHTMLGroups(document, commandNode, group.label)
		issues = append(issues, compareScannedCommandOperatorGroups(
			path, command.Path, group.availability,
			commandOperatorNames(command, group.availability), scan,
		)...)
	}
	expectedFilters := make([]string, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Str(filter.Name)
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		name := marker.String()
		marker.Reset().Str("<code>").Str(html.EscapeString(name)).
			Str("</code>: ").Str(html.EscapeString(filter.Description))
		expectedFilters = append(expectedFilters, marker.String())
	}
	wantFilters := strings.Join(expectedFilters, "<br>")
	if got := htmlDefinitionValue(commandNode, "Command pipes"); got != wantFilters {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "command filters", wantFilters, got,
		))
	}
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		marker.Reset().Str("<code>").Str(html.EscapeString(alias.Name)).
			Str("</code>: ").Str(html.EscapeString(alias.Description)).
			Str(" (<code>").Str(html.EscapeString(alias.Expansion)).
			Str("</code>)")
		expectedAliases = append(expectedAliases, marker.String())
	}
	wantAliases := strings.Join(expectedAliases, "<br>")
	if got := htmlDefinitionValue(commandNode, "Pipe aliases"); got != wantAliases {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "pipe aliases", wantAliases, got,
		))
	}
	return issues
}

func equivalentMarkdownCommandContent(content string) (string, int) {
	return markdownHeadingContent(content, "## Ze command")
}

func validateEquivalentMarkdownContract(
	path, content string,
	command publishedCommand,
) []Issue {
	commandContent, containerCount := equivalentMarkdownCommandContent(content)
	if containerCount != 1 {
		return []Issue{commandContainerCountIssue(
			path, command.Path, "command-equivalent Markdown Ze command section",
			containerCount,
		)}
	}
	content = commandContent
	var issues []Issue
	var marker textbuf.Buffer
	registryPath := marker.Str("- Registry path: `").
		Str(commandMarkdownValue(command.Path)).Byte('`').String()
	if markdownLineCount(content, registryPath) != 1 {
		issues = append(issues, generatedCommandContractIssue(
			path, command.Path, "registry path",
		))
	}
	if command.AnswerShape != "" {
		marker.Reset()
		want := marker.Str("- Answer shape: ").
			Str(commandMarkdownValue(command.AnswerShape)).String()
		if markdownLineCount(content, want) != 1 {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "address field", command.AddressFields,
		equivalentMarkdownGroupValues(content, "Address fields"),
	)...)
	for _, group := range []struct {
		availability string
		label        string
	}{
		{availability: "always", label: "Pipes, always"},
		{availability: "with-rows", label: "Pipes, on rows"},
		{availability: "when-streaming", label: "Pipes, while streaming"},
		{availability: "local-only", label: "Pipes, local process only"},
	} {
		issues = append(issues, compareCommandOperatorGroups(
			path, command.Path, group.availability,
			commandOperatorNames(command, group.availability),
			equivalentMarkdownGroups(content, group.label),
		)...)
	}
	expectedFilters := make([]string, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		marker.Reset().Byte('`').Str(commandMarkdownValue(filter.Name))
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		marker.Byte('`')
		if filter.Description != "" {
			marker.Str(": ").Str(commandMarkdownValue(filter.Description))
		}
		expectedFilters = append(expectedFilters, marker.String())
	}
	wantFilterLine := "- Command pipes: none"
	if len(expectedFilters) != 0 {
		wantFilterLine = marker.Reset().Str("- Command pipes: ").
			Str(strings.Join(expectedFilters, "; ")).String()
	}
	if got := commandMarkdownLine(content, "Command pipes"); got != wantFilterLine {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "command filters", wantFilterLine, got,
		))
	}
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		marker.Reset().Byte('`').Str(commandMarkdownValue(alias.Name)).Byte('`')
		if alias.Description != "" {
			marker.Str(": ").Str(commandMarkdownValue(alias.Description))
		}
		if alias.Expansion != "" {
			marker.Str(" (`").Str(commandMarkdownValue(alias.Expansion)).Str("`)")
		}
		expectedAliases = append(expectedAliases, marker.String())
	}
	wantAliasLine := "- Pipe aliases: none"
	if len(expectedAliases) != 0 {
		wantAliasLine = marker.Reset().Str("- Pipe aliases: ").
			Str(strings.Join(expectedAliases, "; ")).String()
	}
	if got := commandMarkdownLine(content, "Pipe aliases"); got != wantAliasLine {
		issues = append(issues, generatedCommandSurfaceValueIssue(
			path, command.Path, "pipe aliases", wantAliasLine, got,
		))
	}
	return issues
}

func equivalentMarkdownGroupValues(content, label string) []string {
	groups := equivalentMarkdownGroups(content, label)
	if len(groups) == 0 {
		return nil
	}
	return groups[0]
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
		parsed := strings.Split(values, ", ")
		for i := range parsed {
			parsed[i] = strings.ReplaceAll(parsed[i], `\|`, "|")
		}
		groups = append(groups, parsed)
	}
	return groups
}

func commandMarkdownLine(content, label string) string {
	var marker textbuf.Buffer
	prefix := marker.Str("- ").Str(label).Str(": ").String()
	for _, line := range scanMarkdownLines(content) {
		if line.active && strings.HasPrefix(line.text, prefix) {
			return line.text
		}
	}
	return ""
}

func markdownLineCount(content, want string) int {
	count := 0
	for _, line := range scanMarkdownLines(content) {
		if line.active && line.text == want {
			count++
		}
	}
	return count
}

func equivalentAvailabilityLabel(availability string, declaredShape bool) string {
	switch availability {
	case "always":
		return "Pipes, always"
	case "with-rows":
		if declaredShape {
			return "Pipes, on its rows"
		}
		return "Pipes, when the answer has rows"
	case "when-streaming":
		return "Pipes, while streaming"
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
		if node.Data != "dt" || htmlText(node) != label {
			return
		}
		definition := htmlNextElement(node)
		if !document.nodeClosed[node] || definition == nil ||
			definition.Data != "dd" || !document.nodeClosed[definition] {
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

func llmsCommandMetadata(content, path string) (string, int, bool) {
	var meta string
	count := 0
	valid := true
	for _, scanned := range scanMarkdownLines(content) {
		if !scanned.active {
			continue
		}
		candidate, remaining, closed, canonical := markdownListCodeSpan(scanned.text)
		switch {
		case closed && candidate == path && canonical:
			count++
			end := strings.Index(remaining, "): ")
			if end == -1 {
				valid = false
				continue
			}
			meta = remaining[2:end]
		case closed && candidate == path:
			valid = false
		case !closed && markdownUnclosedListPath(candidate, path):
			valid = false
		}
	}
	return meta, count, valid
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
	canonical := closed && width == 1 && strings.HasPrefix(line, "- `") &&
		strings.HasPrefix(item, "`"+path+"`") && strings.HasPrefix(suffix, " (")
	return path, suffix, closed, canonical
}

func validateLLMSCommandContract(
	path, meta string,
	command publishedCommand,
) []Issue {
	var issues []Issue
	var rendered textbuf.Buffer
	mode, _, _ := strings.Cut(meta, "; ")
	if mode != command.Mode {
		issues = append(issues, generatedCommandValueIssue(
			path, command.Path, "mode", command.Mode, mode,
		))
	}
	if wire := commandMetaValue(meta, "wire"); wire != command.WireMethod {
		issues = append(issues, generatedCommandValueIssue(
			path, command.Path, "wire method", command.WireMethod, wire,
		))
	}
	for _, availability := range []string{
		"always", "with-rows", "when-streaming", "local-only",
	} {
		issues = append(issues, compareCommandOperatorGroups(
			path, command.Path, availability,
			commandOperatorNames(command, availability),
			commandMetaPipeGroups(meta, availability),
		)...)
	}
	if shape := commandMetaValue(meta, "shape"); shape != command.AnswerShape {
		issues = append(issues, generatedCommandValueIssue(
			path, command.Path, "answer shape", command.AnswerShape, shape,
		))
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "address field", command.AddressFields,
		strings.Fields(commandMetaValue(meta, "address-fields")),
	)...)
	expectedFilters := make([]string, 0, len(command.Pipes))
	for _, filter := range command.Pipes {
		expectedFilters = append(expectedFilters, filter.Name)
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "command filter", expectedFilters,
		strings.Fields(commandMetaValue(meta, "filters")),
	)...)
	expectedAliases := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		expectedAliases = append(expectedAliases,
			rendered.Reset().Str(alias.Name).Byte('=').Str(alias.Expansion).String())
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "pipe alias", expectedAliases,
		splitNonEmpty(commandMetaValue(meta, "aliases"), ", "),
	)...)
	expectedArgs := make([]string, 0, len(command.Args))
	for _, arg := range command.Args {
		expectedArgs = append(expectedArgs,
			rendered.Reset().Str(arg.Name).Byte(':').Str(arg.Type).String())
	}
	issues = append(issues, compareCommandNamedGroup(
		path, command.Path, "argument", expectedArgs,
		splitNonEmpty(commandMetaValue(meta, "args"), ", "),
	)...)
	return issues
}

func commandMetaPipeGroups(meta, availability string) [][]string {
	var groups [][]string
	for segment := range strings.SplitSeq(meta, "; ") {
		pipes, ok := strings.CutPrefix(segment, "pipes ")
		if !ok {
			continue
		}
		for group := range strings.SplitSeq(pipes, ", ") {
			label, values, ok := strings.Cut(group, ": ")
			if ok && label == availability {
				groups = append(groups, strings.Fields(values))
			}
		}
	}
	return groups
}

func commandMetaValue(meta, label string) string {
	var marker textbuf.Buffer
	prefix := marker.Str(label).Byte(' ').String()
	for segment := range strings.SplitSeq(meta, "; ") {
		if strings.HasPrefix(segment, prefix) {
			return strings.TrimPrefix(segment, prefix)
		}
	}
	return ""
}

func compareRenderedCommandSurfaces(
	root, publicRoot, expectedRoot string,
	live []publishedCommand,
) []Issue {
	expected := map[string]bool{
		"llms.txt":                 true,
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
