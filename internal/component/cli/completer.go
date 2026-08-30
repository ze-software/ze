// Design: docs/architecture/config/yang-config-design.md — config editor
// Related: completer_command.go — command mode operational completion
// Related: completer_plugin.go — plugin SDK method completion
// Detail: completer_validate.go — YANG value validation

package cli

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/textbuf"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// Completion represents a single completion suggestion.
// Completion is a type alias of contract.Completion.
type Completion = contract.Completion

// The kinds a Completion carries in its Type field. The editor renders each
// kind differently, so a suggestion that names the wrong kind is shown wrong.
const (
	completionCommand = "command"
	completionKeyword = "keyword"
	completionListKey = "list-key"
	completionValue   = "value"
	completionHint    = "hint"
)

// typeHintAny is the hint shown for a leaf whose YANG type names nothing more
// specific. It is a word for the operator to read, not a completion kind.
const typeHintAny = "value"

// leafBackend is the YANG leaf that names the active backend of a component.
const leafBackend = "backend"

// Completer provides YANG-driven completions.
type Completer struct {
	loader   *yang.Loader
	tree     *config.Tree            // Config data for list key completion
	registry *yang.ValidatorRegistry // Validator registry for ze:validate completions
	backends map[string]string       // component root -> active backend name
}

// NewCompleter creates a completer using YANG schema.
func NewCompleter() *Completer {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return &Completer{}
	}
	if err := loader.LoadRegistered(); err != nil {
		return &Completer{}
	}
	if err := loader.Resolve(); err != nil {
		return &Completer{}
	}
	reg := yang.NewValidatorRegistry()
	config.RegisterValidators(reg)
	return &Completer{loader: loader, registry: reg}
}

// SetTree sets the config tree for data-aware completion.
func (c *Completer) SetTree(tree any) {
	t, _ := tree.(*config.Tree)
	c.setTreeInternal(t)
}

func (c *Completer) setTreeInternal(tree *config.Tree) {
	c.tree = tree
	c.backends = c.deriveBackends(tree)
}

// backendLeaves maps component root container names to their backend leaf path.
var backendLeaves = map[string]string{
	"interface":       leafBackend,
	"firewall":        leafBackend,
	"traffic/control": leafBackend,
}

// deriveBackends reads backend leaf values from the config tree.
func (c *Completer) deriveBackends(tree *config.Tree) map[string]string {
	if tree == nil {
		return nil
	}
	m := make(map[string]string)
	for root, leaf := range backendLeaves {
		container := tree.GetContainerPath(root)
		if container == nil {
			continue
		}
		if v, ok := container.Get(leaf); ok && v != "" {
			m[root] = v
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// commands returns the available editor commands.
var commands = []Completion{
	{Text: cmdSet, Description: "Set a configuration value", Type: completionCommand},
	{Text: cmdDelete, Description: "Delete a configuration value", Type: completionCommand},
	{Text: cmdEdit, Description: "Enter a subsection context", Type: completionCommand},
	{Text: cmdShow, Description: "Display configuration", Type: completionCommand},
	{Text: cmdOption, Description: "Display settings (columns, errors)", Type: completionCommand},
	{Text: cmdCommit, Description: "Apply config (must be valid)", Type: completionCommand},
	{Text: cmdSave, Description: "Snapshot work-in-progress", Type: completionCommand},
	{Text: cmdDiscard, Description: "Revert all changes", Type: completionCommand},
	{Text: cmdTop, Description: "Return to root context", Type: completionCommand},
	{Text: cmdUp, Description: "Go up one level", Type: completionCommand},
	{Text: cmdRollback, Description: "Restore from backup", Type: completionCommand},
	{Text: cmdExit, Description: "Exit current mode", Type: completionCommand},
	{Text: cmdHelp, Description: "Show help", Type: completionCommand},
	{Text: cmdRun, Description: "Run operational command", Type: completionCommand},
	{Text: cmdWho, Description: "List active editing sessions", Type: completionCommand},
	{Text: cmdDisconnect, Description: "Remove another session", Type: completionCommand},
	{Text: cmdDeactivate, Description: "Mark a config block inactive", Type: completionCommand},
	{Text: cmdActivate, Description: "Reactivate an inactive config block", Type: completionCommand},
	{Text: cmdRename, Description: "Rename a list entry", Type: completionCommand},
	{Text: cmdCopy, Description: "Copy a list entry", Type: completionCommand},
	{Text: cmdInsert, Description: "Insert into a leaf-list at position", Type: completionCommand},
}

// Complete returns completions for the given input at cursor position.
// contextPath is the current edit context (e.g., ["bgp", "peer", "192.168.1.1"]).
func (c *Completer) Complete(input string, contextPath []string) []Completion {
	if c.loader == nil {
		return commands
	}

	input = strings.TrimLeft(input, " ")
	tokens := tokenize(input)

	// Empty input or no tokens: show commands
	if len(tokens) == 0 {
		return commands
	}

	cmd := tokens[0]
	endsWithSpace := strings.HasSuffix(input, " ")

	// If we're still typing the first word
	if len(tokens) == 1 && !endsWithSpace {
		cmdCompletions := filterCompletions(commands, cmd)

		// Also suggest keywords with "set" prefix if partial matches a keyword
		entry := c.getEntry(contextPath)
		if entry != nil && entry.Dir != nil {
			for _, name := range c.getSortedChildren(entry) {
				if strings.HasPrefix(name, cmd) {
					child := effectiveChild(entry, name)
					if !c.backendAllowed(child) {
						continue
					}
					var tb textbuf.Buffer
					cmdCompletions = append(cmdCompletions, Completion{
						Text:        tb.Str(cmdSet).Byte(' ').Str(name).String(),
						Description: tb.Reset().Str("Set ").Str(name).String(),
						Type:        completionKeyword,
					})
				}
			}
		}
		return cmdCompletions
	}

	// Dispatch based on command
	switch cmd {
	case cmdSet, cmdDelete, cmdDeactivate, cmdActivate, cmdInsert:
		return c.completeSetPath(tokens[1:], contextPath, endsWithSpace)
	case cmdRename, cmdCopy:
		return c.completeRenamePath(tokens[1:], contextPath, endsWithSpace)
	case cmdEdit:
		return c.completeEditPath(tokens[1:], contextPath, endsWithSpace)
	case cmdShow:
		return c.completeShowPath(tokens[1:], contextPath, endsWithSpace)
	case cmdOption:
		return c.completeOptionPath(tokens[1:], contextPath, endsWithSpace)
	case cmdDiscard:
		return c.completeDiscardPath(tokens[1:], contextPath, endsWithSpace)
	default: // No subcommand completions for other commands
		return nil
	}
}

// GhostText returns the best single completion for inline ghost text.
func (c *Completer) GhostText(input string, contextPath []string) string {
	if input == "" || c.loader == nil {
		return ""
	}

	tokens := tokenize(input)
	if len(tokens) == 0 {
		return ""
	}

	endsWithSpace := strings.HasSuffix(input, " ")
	if endsWithSpace {
		return ""
	}

	lastWord := tokens[len(tokens)-1]
	completions := c.Complete(input, contextPath)

	if len(completions) == 0 {
		return ""
	}

	// Find completions that start with the last word
	var matches []Completion
	for _, comp := range completions {
		if strings.HasPrefix(comp.Text, lastWord) {
			matches = append(matches, comp)
		}
	}

	if len(matches) == 1 {
		return matches[0].Text[len(lastWord):]
	}

	if len(matches) > 1 {
		common := matches[0].Text
		for _, m := range matches[1:] {
			common = commonPrefix(common, m.Text)
		}
		if len(common) > len(lastWord) {
			return common[len(lastWord):]
		}
	}

	return ""
}

// completeRenamePath completes paths for the rename command.
// Before "to": delegates to completeSetPath for list entry navigation.
// After "to": no completions (user types the new name).
func (c *Completer) completeRenamePath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	// Check if "to" is already present
	if slices.Contains(tokens, "to") {
		return nil
	}
	// Before "to", complete the path to the list entry
	return c.completeSetPath(tokens, contextPath, endsWithSpace)
}

// completeSetPath completes paths for set/delete commands.
func (c *Completer) completeSetPath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	currentPath := append([]string{}, contextPath...)
	// Track how many path elements came from tokens (vs context)
	tokensAdded := 0

	// Navigate through tokens, handling list keys
	for i, token := range tokens {
		isLast := i == len(tokens)-1

		if isLast && !endsWithSpace {
			// Partial match on this token.
			// Check if we're at a list that needs a key — show existing keys.
			if c.isListNeedingKey(currentPath) {
				return c.listKeyCompletions(currentPath[len(currentPath)-1], token, currentPath[:len(currentPath)-1])
			}
			return c.matchChildren(currentPath, token)
		}

		// Navigate deeper (token is either a schema child or a list key value).
		// If the current path ends at a list needing a key, validate the token as a key value.
		if tokensAdded > 0 && c.isListNeedingKey(currentPath) {
			listPath := make([]string, len(currentPath))
			copy(listPath, currentPath)
			keyEntry := c.getListKeyEntry(listPath)
			if !validateLeafValue(keyEntry, token) {
				listName := currentPath[len(currentPath)-1]
				hint := c.TypeHint(keyEntry.Type)
				return []Completion{{
					Text: token,
					Description: func() string {
						var tb textbuf.Buffer
						return tb.Str("invalid ").Str(listName).Str(" key (expected ").Str(hint).Byte(')').String()
					}(),
					Type: "error",
				}}
			}
		}
		currentPath = append(currentPath, token)
		tokensAdded++
	}

	// If we ended with space, show next level.
	// Only show list key completions if the user navigated here via tokens.
	// When the context path itself ends at a list (tokensAdded == 0),
	// show schema children instead -- the user is already in that context.
	if c.isListNeedingKey(currentPath) && len(currentPath) > 0 && tokensAdded > 0 {
		listName := currentPath[len(currentPath)-1]
		parentPath := currentPath[:len(currentPath)-1]
		return c.listKeyCompletions(listName, "", parentPath)
	}
	return c.matchChildren(currentPath, "")
}

// isListNeedingKey returns true if the path ends at a list element (not a key value).
// Path ["bgp", "peer"] → true (peer is a list, needs a key).
// Path ["bgp", "peer", "1.1.1.1"] → false (1.1.1.1 is a key, we're inside the entry).
func (c *Completer) isListNeedingKey(path []string) bool {
	if len(path) == 0 {
		return false
	}
	entry := c.getEntry(path)
	if entry == nil || !entry.IsList() {
		return false
	}
	// Check if the last element is a schema child name (list name) or a key value.
	// If the parent's schema has the last element as a child, it's the list name itself.
	// Exception: if the parent is a list and the last element matches its key leaf,
	// the element is a key value (e.g., "peer name" where "name" is peer's key leaf).
	lastName := path[len(path)-1]
	parentEntry := c.getEntry(path[:len(path)-1])
	if parentEntry != nil && parentEntry.Dir != nil {
		if _, isSchemaChild := parentEntry.Dir[lastName]; isSchemaChild {
			// If parent is a list and lastName is its key leaf, it's a key value, not the list name.
			if parentEntry.IsList() && parentEntry.Key == lastName {
				return false
			}
			return true // Last element IS the list name → needs a key
		}
	}
	return false // Last element is a key value → inside the entry
}

// completeEditPath completes paths for edit command.
func (c *Completer) completeEditPath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	// For edit, we're looking for lists and containers
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		return c.matchEditTargets(contextPath, prefix)
	}

	// After first token (list name), show existing keys
	if len(tokens) == 1 && endsWithSpace {
		listPath := append(append([]string{}, contextPath...), tokens[0])
		if c.isList(listPath) {
			return c.listKeyCompletions(tokens[0], "", contextPath)
		}
	}

	// Partial match on list key
	if len(tokens) == 2 && !endsWithSpace {
		listPath := append(append([]string{}, contextPath...), tokens[0])
		if c.isList(listPath) {
			return c.listKeyCompletions(tokens[0], tokens[1], contextPath)
		}
	}

	return nil
}

// completeShowPath completes paths for show command (content display).
func (c *Completer) completeShowPath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	// Check for pipe: find the last "|" and complete pipe filters after it.
	pipeIdx := -1
	for i, t := range tokens {
		if t == "|" {
			pipeIdx = i
		}
	}
	if pipeIdx >= 0 {
		return completePipeFilter(showPipeFilters, tokens[pipeIdx+1:], endsWithSpace)
	}

	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		// Offer schema children (peer, rib, etc.) and pipe.
		completions := c.matchEditTargets(contextPath, prefix)
		if prefix == "" || prefix == "|" {
			completions = append(completions, Completion{Text: "|", Description: "Pipe output through filters", Type: completionKeyword})
		}
		return completions
	}

	if len(tokens) == 1 && endsWithSpace {
		listPath := append(append([]string{}, contextPath...), tokens[0])
		if c.isList(listPath) {
			return c.listKeyCompletions(tokens[0], "", contextPath)
		}
	}

	return nil
}

// optionSubcommands are completions offered when typing "option ".
var optionSubcommands = []Completion{
	{Text: colAuthor, Description: "Toggle author column (enable/disable)", Type: completionKeyword},
	{Text: colDate, Description: "Toggle date column (enable/disable)", Type: completionKeyword},
	{Text: colSource, Description: "Toggle source column (enable/disable)", Type: completionKeyword},
	{Text: colChanges, Description: "Toggle changes column (enable/disable)", Type: completionKeyword},
	{Text: cmdErrors, Description: "Error display (hints/hide)", Type: completionKeyword},
	{Text: cmdAll, Description: "Enable all display columns", Type: completionKeyword},
	{Text: cmdNone, Description: "Disable all display columns", Type: completionKeyword},
}

// completeOptionPath completes paths for option command (display settings).
func (c *Completer) completeOptionPath(tokens, _ []string, endsWithSpace bool) []Completion {
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		return filterCompletions(optionSubcommands, prefix)
	}

	// "option errors " -> offer hints/hide.
	if len(tokens) == 1 && tokens[0] == cmdErrors && endsWithSpace {
		return []Completion{
			{Text: "hints", Description: "Toggle inline diagnostic hints", Type: completionKeyword},
			{Text: "hide", Description: "Hide error annotations", Type: completionKeyword},
		}
	}

	// "option <column> " -> offer enable/disable.
	if len(tokens) == 1 && endsWithSpace && isOptionColumn(tokens[0]) {
		return []Completion{
			{Text: cmdEnable, Description: "Enable column", Type: completionKeyword},
			{Text: cmdDisable, Description: "Disable column", Type: completionKeyword},
		}
	}

	return nil
}

// textPipeFilters are basic text filters available to any piped command.
var textPipeFilters = []Completion{
	{Text: cmdMatch, Description: "Filter lines matching pattern", Type: completionKeyword},
	{Text: cmdHead, Description: "Show first N lines", Type: completionKeyword},
	{Text: cmdTail, Description: "Show last N lines", Type: completionKeyword},
}

// showPipeFilters extend text filters with show-specific pipes.
var showPipeFilters = append([]Completion{
	{Text: cmdBlame, Description: "Annotate with authorship", Type: completionKeyword},
	{Text: cmdChanges, Description: "Pending changes", Type: completionKeyword},
	{Text: cmdCompare, Description: "Diff against baseline", Type: completionKeyword},
	{Text: cmdErrors, Description: "Validation issues", Type: completionKeyword},
	{Text: cmdFormat, Description: "Output format (tree or config)", Type: completionKeyword},
	{Text: cmdHistory, Description: "Rollback revisions", Type: completionKeyword},
	{Text: cmdActive, Description: "Show only active nodes (hide inactive)", Type: completionKeyword},
	{Text: cmdInactive, Description: "Show only inactive nodes", Type: completionKeyword},
}, textPipeFilters...)

// completePipeFilter completes pipe filter names and their arguments.
// The available filters list is passed in so each command can offer different pipes.
func completePipeFilter(available []Completion, tokens []string, endsWithSpace bool) []Completion {
	// No tokens after pipe or partial filter name: suggest filter names.
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		return filterCompletions(available, prefix)
	}

	// After a filter name, suggest arguments.
	filter := tokens[len(tokens)-1]
	if !endsWithSpace {
		if len(tokens) >= 2 {
			filter = tokens[len(tokens)-2]
		} else {
			return nil
		}
	}

	switch filter {
	case cmdFormat:
		prefix := ""
		if !endsWithSpace && len(tokens) >= 2 {
			prefix = tokens[len(tokens)-1]
		}
		return filterCompletions([]Completion{
			{Text: fmtTree, Description: "Hierarchical tree format", Type: completionKeyword},
			{Text: fmtConfig, Description: "Flat set-command format", Type: completionKeyword},
		}, prefix)
	case cmdCompare:
		prefix := ""
		if !endsWithSpace && len(tokens) >= 2 {
			prefix = tokens[len(tokens)-1]
		}
		return filterCompletions([]Completion{
			{Text: "committed", Description: "Compare with committed config", Type: completionKeyword},
			{Text: "saved", Description: "Compare with saved draft", Type: completionKeyword},
			{Text: "rollback", Description: "Compare with rollback N", Type: completionKeyword},
		}, prefix)
	case cmdChanges:
		prefix := ""
		if !endsWithSpace && len(tokens) >= 2 {
			prefix = tokens[len(tokens)-1]
		}
		return filterCompletions([]Completion{
			{Text: cmdAll, Description: "All sessions' pending changes", Type: completionKeyword},
		}, prefix)
	}

	return nil
}

// completeDiscardPath completes paths for the discard command.
// Offers "all" alongside YANG path completions in session mode.
func (c *Completer) completeDiscardPath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		results := filterCompletions([]Completion{
			{Text: cmdAll, Description: "Discard all pending changes", Type: completionKeyword},
		}, prefix)
		results = append(results, c.completeSetPath(tokens, contextPath, endsWithSpace)...)
		return results
	}
	return c.completeSetPath(tokens, contextPath, endsWithSpace)
}

// listKeyCompletions returns completions for list keys.
// contextPath is used to navigate to the correct container in the config tree.
// Single-entry lists return nil (auto-selected, no key needed).
// Multi-entry lists show positional IDs (#1, #2, ...) instead of raw keys.
func (c *Completer) listKeyCompletions(listName, prefix string, contextPath []string) []Completion {
	// Count existing entries to decide behavior
	var entryCount int
	var orderedEntries []struct {
		Key   string
		Value *config.Tree
	}
	if c.tree != nil {
		tree := c.navigateTreeToPath(contextPath)
		if tree != nil {
			orderedEntries = tree.GetListOrdered(listName)
			entryCount = len(orderedEntries)
		}
	}

	// Single entry with no prefix — auto-select, no key needed.
	// When user has typed a prefix, still offer it as a completion so Tab can accept it.
	if entryCount == 1 && prefix == "" {
		return nil
	}

	var completions []Completion

	// Add wildcard template option
	if prefix == "" || prefix == "*" {
		completions = append(completions, Completion{
			Text:        "*",
			Description: "Template for all entries",
			Type:        completionListKey,
		})
	}

	// Multiple entries — show actual keys for named entries, #N for unnamed
	for i, entry := range orderedEntries {
		if isDefaultKey(entry.Key) {
			// Unnamed entry — show as positional ID
			id := textbuf.StrInt("#", int64(i+1))
			if prefix == "" || strings.HasPrefix(id, prefix) {
				completions = append(completions, Completion{
					Text:        id,
					Description: listName,
					Type:        completionListKey,
				})
			}
		} else if prefix == "" || strings.HasPrefix(entry.Key, prefix) {
			// Named entry — show actual key
			completions = append(completions, Completion{
				Text:        entry.Key,
				Description: listName,
				Type:        completionListKey,
			})
		}
	}

	if prefix == "" && entryCount == 0 {
		// No entries and nothing typed — show placeholder hint (display-only, not applicable)
		var tb textbuf.Buffer
		completions = append(completions, Completion{
			Text:        "<value>",
			Description: tb.Str("New ").Str(listName).Str(" key").String(),
			Type:        completionHint,
		})
	} else if prefix != "" && len(completions) == 0 {
		// User typed a value that doesn't match any existing key —
		// validate against YANG key type before offering as completion.
		listPath := append(append([]string{}, contextPath...), listName)
		keyEntry := c.getListKeyEntry(listPath)
		var tb textbuf.Buffer
		if validateLeafValue(keyEntry, prefix) {
			completions = append(completions, Completion{
				Text:        prefix,
				Description: tb.Str("New ").Str(listName).Str(" key").String(),
				Type:        completionListKey,
			})
		} else {
			hint := c.TypeHint(keyEntry.Type)
			completions = append(completions, Completion{
				Text:        prefix,
				Description: tb.Reset().Str("invalid ").Str(listName).Str(" key (expected ").Str(hint).Byte(')').String(),
				Type:        "warning",
			})
		}
	}

	return completions
}

// isDefaultKey returns true if the key is auto-generated (KeyDefault or KeyDefault#N).
func isDefaultKey(key string) bool {
	return key == config.KeyDefault || strings.HasPrefix(key, config.KeyDefault+"#")
}

// navigateTreeToPath walks the config tree along a context path,
// handling both containers and list entries using the YANG schema
// to distinguish list keys from child names.
func (c *Completer) navigateTreeToPath(contextPath []string) *config.Tree {
	tree := c.tree
	if tree == nil {
		return nil
	}

	if len(contextPath) == 0 {
		return tree
	}

	// Find the module that owns the first path element
	entry := c.findModuleEntry(contextPath[0])
	if entry == nil {
		return nil
	}

	for i := 0; i < len(contextPath); i++ {
		var part string
		if i == 0 {
			// First element already resolved to entry
			part = contextPath[0]
		} else {
			part = contextPath[i]
			children := effectiveChildren(entry)
			child, ok := children[part]
			if !ok {
				return nil
			}
			entry = child
		}

		// If this is a list, next path element is the key value
		if entry.IsList() && i+1 < len(contextPath) {
			nextPart := contextPath[i+1]
			children := effectiveChildren(entry)
			if _, hasChild := children[nextPart]; !hasChild {
				// Next element is a list key — navigate into the list entry
				entries := tree.GetList(part)
				if entries == nil {
					return nil
				}
				entryTree, ok := entries[nextPart]
				if !ok {
					return nil
				}
				tree = entryTree
				i++ // skip the key element
				continue
			}
		}

		// Container navigation
		container := tree.GetContainer(part)
		if container != nil {
			tree = container
		} else {
			return nil
		}
	}

	return tree
}

// matchChildren returns completions for children at path matching prefix.
func (c *Completer) matchChildren(path []string, prefix string) []Completion {
	entry := c.getEntry(path)
	if entry == nil {
		return nil
	}

	// If this is a leaf, show value hints
	if entry.Kind == gyang.LeafEntry {
		return c.valueCompletions(entry, prefix)
	}

	// Get children, excluding list key when inside an entry (key already set as identifier).
	// When AT the list level (no key provided yet), show the key so users can set it.
	var completions []Completion
	children := c.getSortedChildren(entry)
	skipKey := entry.IsList() && !c.isListNeedingKey(path)

	for _, name := range children {
		if skipKey && entry.Key == name {
			continue // Skip list key — already set as the identifier
		}
		if prefix == "" || strings.HasPrefix(name, prefix) {
			child := effectiveChild(entry, name)
			if !c.backendAllowed(child) {
				continue
			}
			completions = append(completions, Completion{
				Text:        name,
				Description: c.entryDescription(child),
				Type:        completionKeyword,
			})
		}
	}

	return completions
}

// matchEditTargets returns completions for show/edit targets.
// Includes containers, lists, and leaves — anything that can be shown or navigated.
func (c *Completer) matchEditTargets(path []string, prefix string) []Completion {
	entry := c.getEntry(path)
	if entry == nil {
		return nil
	}

	var completions []Completion
	children := c.getSortedChildren(entry)

	for _, name := range children {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			child := effectiveChild(entry, name)
			if !c.backendAllowed(child) {
				continue
			}
			completions = append(completions, Completion{
				Text:        name,
				Description: c.entryDescription(child),
				Type:        completionKeyword,
			})
		}
	}

	return completions
}

// Backends returns the current per-component backend map.
func (c *Completer) Backends() map[string]string {
	return c.backends
}

// backendAllowed returns true if the YANG entry should be shown given active backends.
// No ze:backend annotation means unrestricted (always shown).
func (c *Completer) backendAllowed(entry *gyang.Entry) bool {
	be := yang.GetBackendExtension(entry)
	if be == nil || c.backends == nil {
		return true
	}
	for _, allowed := range be {
		for _, active := range c.backends {
			if allowed == active {
				return true
			}
		}
	}
	return false
}

// valueCompletions returns completions for a leaf value.
func (c *Completer) valueCompletions(entry *gyang.Entry, prefix string) []Completion {
	if entry.Type == nil {
		return []Completion{{Text: "<" + typeHintAny + ">", Description: typeHintAny, Type: completionHint}}
	}

	// Check ze:validate extension for CompleteFn-based completions.
	// CompleteFn takes priority over enum because it provides runtime-determined
	// values (e.g., registered address families, event types). If a developer
	// sets ze:validate on an enum leaf, they want dynamic completion.
	if completions := c.validateCompletions(entry, prefix); len(completions) > 0 {
		return completions
	}

	// Handle enums (static YANG values, used when no ze:validate is present)
	if entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
		var completions []Completion
		for _, name := range entry.Type.Enum.Names() {
			if prefix == "" || strings.HasPrefix(name, prefix) {
				completions = append(completions, Completion{
					Text:        name,
					Description: "enum value",
					Type:        completionValue,
				})
			}
		}
		return completions
	}

	// Handle booleans
	if entry.Type.Kind == gyang.Ybool {
		return filterCompletions([]Completion{
			{Text: boolTrue, Description: "Enable", Type: completionValue},
			{Text: boolFalse, Description: "Disable", Type: completionValue},
		}, prefix)
	}

	// Handle unions: collect enum values from member types, add type hint for non-enum members.
	if entry.Type.Kind == gyang.Yunion {
		var completions []Completion
		for _, t := range entry.Type.Type {
			if t.Kind == gyang.Yenum && t.Enum != nil {
				for _, name := range t.Enum.Names() {
					if prefix == "" || strings.HasPrefix(name, prefix) {
						completions = append(completions, Completion{
							Text:        name,
							Description: "enum value",
							Type:        completionValue,
						})
					}
				}
			}
		}
		if len(completions) > 0 {
			return completions
		}
	}

	// Type hint based on YANG type — hint-only, not applicable by Tab
	hint := c.TypeHint(entry.Type)
	var tb textbuf.Buffer
	return []Completion{{Text: tb.Byte('<').Str(hint).Byte('>').String(), Description: tb.Reset().Str(hint).Str(" value").String(), Type: completionHint}}
}

// validateCompletions returns completions from ze:validate CompleteFn if available.
// Handles pipe-separated validators by unioning their CompleteFn results.
// Returns nil if no validator has a CompleteFn.
func (c *Completer) validateCompletions(entry *gyang.Entry, prefix string) []Completion {
	if c.registry == nil {
		return nil
	}

	arg := yang.GetValidateExtension(entry)
	if arg == "" {
		return nil
	}

	var values []string
	seen := make(map[string]bool)

	for _, name := range yang.SplitValidatorNames(arg) {
		cv := c.registry.Get(name)
		if cv == nil || cv.CompleteFn == nil {
			continue
		}
		for _, v := range cv.CompleteFn() {
			if !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
	}

	if len(values) == 0 {
		return nil
	}

	sort.Strings(values)

	var completions []Completion
	for _, v := range values {
		if prefix == "" || strings.HasPrefix(v, prefix) {
			completions = append(completions, Completion{
				Text:        v,
				Description: "valid value",
				Type:        completionValue,
			})
		}
	}
	return completions
}

// TypeHint returns a hint string for a YANG type.
func (c *Completer) TypeHint(t *gyang.YangType) string {
	if t == nil {
		return typeHintAny
	}
	//nolint:exhaustive // fallthrough uses type name for unlisted YANG kinds
	switch t.Kind {
	case gyang.Ystring:
		return "string"
	case gyang.Yuint8:
		return "0-255"
	case gyang.Yuint16:
		return "0-65535"
	case gyang.Yuint32:
		return "0-4294967295"
	case gyang.Ybool:
		return "boolean"
	case gyang.Yenum:
		return "enum"
	}
	// For union types: list member type names for clarity
	if t.Kind == gyang.Yunion && len(t.Type) > 0 {
		names := make([]string, 0, len(t.Type))
		for _, member := range t.Type {
			if member.Name != "" {
				names = append(names, member.Name)
			}
		}
		if len(names) > 0 {
			return textbuf.Join(names, " or ")
		}
	}
	// For typedef and other types: use the YANG type name if available
	if t.Name != "" && t.Name != "union" {
		return t.Name
	}
	return typeHintAny
}

// entryDescription returns description for a YANG entry.
func (c *Completer) entryDescription(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	// YANG descriptions can be multi-line; collapse to single line for dropdown display.
	desc := textbuf.Join(strings.Fields(entry.Description), " ")
	if entry.Mandatory == gyang.TSTrue && !strings.Contains(desc, "(required)") {
		if desc != "" {
			var tb textbuf.Buffer
			desc = tb.Str(desc).Str(" (required)").String()
		} else {
			desc = "required"
		}
	}
	return desc
}

// confModuleNames returns all loaded YANG config module names (ending in "-conf").
func (c *Completer) confModuleNames() []string {
	if c.loader == nil {
		return nil
	}
	return c.loader.ConfModuleNames()
}

// getEntry returns the YANG entry at the given path.
// Handles list keys by skipping key values (e.g., "peer", "1.1.1.1" → navigate to peer list children).
// Searches all config modules to find the path root.
func (c *Completer) getEntry(path []string) *gyang.Entry {
	if c.loader == nil {
		return nil
	}

	// Empty path: return a virtual root with children from all modules
	if len(path) == 0 {
		return c.mergedRoot()
	}

	// Find which module owns the first path element
	entry := c.findModuleEntry(path[0])
	if entry == nil {
		return nil
	}

	// Navigate through remaining path, handling list keys
	for i := 1; i < len(path); i++ {
		part := path[i]
		children := effectiveChildren(entry)
		child, ok := children[part]
		if !ok {
			return nil
		}
		entry = child

		// If this is a list and there's a next element, check if it's a key value to skip.
		// A key value is either: (a) not a schema child, or (b) matches the list's key leaf name.
		// Case (b) handles YANG lists where the key leaf (e.g., "name") is both a schema child
		// and used as a key value in config paths (e.g., "peer name" where "name" is the key).
		if entry.IsList() && i+1 < len(path) {
			nextPart := path[i+1]
			children := effectiveChildren(entry)
			_, hasChild := children[nextPart]
			isKeyLeaf := entry.Key == nextPart
			if !hasChild || isKeyLeaf {
				// Next element is a key value, skip it
				i++
			}
		}
	}

	return entry
}

// mergedRoot returns a virtual root entry with children from all config modules.
// When multiple modules declare a top-level container with the same name (the
// augment pattern, e.g. "environment"), their Dir maps are merged recursively
// so the caller sees the union of all augmentations.
func (c *Completer) mergedRoot() *gyang.Entry {
	groups := make(map[string][]*gyang.Entry)
	for _, modName := range c.confModuleNames() {
		modEntry := c.loader.GetEntry(modName)
		if modEntry == nil || modEntry.Dir == nil {
			continue
		}
		for name, child := range modEntry.Dir {
			groups[name] = append(groups[name], child)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	root := &gyang.Entry{
		Kind: gyang.DirectoryEntry,
		Dir:  make(map[string]*gyang.Entry, len(groups)),
	}
	for name, children := range groups {
		root.Dir[name] = mergeAugmentedEntries(children)
	}
	return root
}

// findModuleEntry searches all config modules for a top-level child by name.
// When multiple modules declare the same top-level container (augment), their
// entries are merged recursively so the caller sees the union of all fields.
func (c *Completer) findModuleEntry(name string) *gyang.Entry {
	var matches []*gyang.Entry
	for _, modName := range c.confModuleNames() {
		modEntry := c.loader.GetEntry(modName)
		if modEntry == nil || modEntry.Dir == nil {
			continue
		}
		if child, ok := modEntry.Dir[name]; ok {
			matches = append(matches, child)
		}
	}
	return mergeAugmentedEntries(matches)
}

// mergeAugmentedEntries returns a virtual entry whose Dir is the recursive
// union of the inputs' Dir maps. When the same child name appears in more
// than one input, those children are merged in turn. Non-Dir fields are
// taken from the first entry (YANG augment semantics keep the original
// container's Kind/Node/etc.). Returns nil for an empty list; returns the
// single input unchanged when len==1 to avoid wrapping real entries.
func mergeAugmentedEntries(entries []*gyang.Entry) *gyang.Entry {
	if len(entries) == 0 {
		return nil
	}
	if len(entries) == 1 {
		return entries[0]
	}
	groups := make(map[string][]*gyang.Entry)
	for _, e := range entries {
		for name, child := range e.Dir {
			groups[name] = append(groups[name], child)
		}
	}
	if len(groups) == 0 {
		return entries[0]
	}
	merged := *entries[0]
	merged.Dir = make(map[string]*gyang.Entry, len(groups))
	for name, children := range groups {
		merged.Dir[name] = mergeAugmentedEntries(children)
	}
	return &merged
}

func effectiveChildren(entry *gyang.Entry) map[string]*gyang.Entry {
	if entry == nil || entry.Dir == nil {
		return nil
	}
	out := make(map[string]*gyang.Entry, len(entry.Dir))
	var walk func(node *gyang.Entry)
	walk = func(node *gyang.Entry) {
		if node == nil || node.Dir == nil {
			return
		}
		for name, child := range node.Dir {
			if child == nil {
				continue
			}
			if child.IsChoice() || child.IsCase() {
				walk(child)
				continue
			}
			if existing, ok := out[name]; ok {
				out[name] = mergeAugmentedEntries([]*gyang.Entry{existing, child})
				continue
			}
			out[name] = child
		}
	}
	walk(entry)
	if len(out) == 0 {
		return nil
	}
	return out
}

func effectiveChild(entry *gyang.Entry, name string) *gyang.Entry {
	if entry == nil || entry.Dir == nil {
		return nil
	}
	var find func(node *gyang.Entry) *gyang.Entry
	find = func(node *gyang.Entry) *gyang.Entry {
		if node == nil || node.Dir == nil {
			return nil
		}
		for k, child := range node.Dir {
			if child == nil {
				continue
			}
			if child.IsChoice() || child.IsCase() {
				if found := find(child); found != nil {
					return found
				}
				continue
			}
			if k == name {
				return child
			}
		}
		return nil
	}
	return find(entry)
}

// getSortedChildren returns sorted child names.
func (c *Completer) getSortedChildren(entry *gyang.Entry) []string {
	childrenMap := effectiveChildren(entry)
	if len(childrenMap) == 0 {
		return nil
	}
	children := make([]string, 0, len(childrenMap))
	for name := range childrenMap {
		children = append(children, name)
	}
	sort.Strings(children)
	return children
}

// isList returns true if the path points to a list.
func (c *Completer) isList(path []string) bool {
	entry := c.getEntry(path)
	return entry != nil && entry.IsList()
}

// Helper functions

func tokenize(input string) []string {
	return strings.Fields(input)
}

func filterCompletions(completions []Completion, prefix string) []Completion {
	if prefix == "" {
		return completions
	}
	var filtered []Completion
	for _, c := range completions {
		if strings.HasPrefix(c.Text, prefix) {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func commonPrefix(a, b string) string {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:minLen]
}

// ValidateValueAtPath validates a value against the YANG type at the given path.
// Returns nil if valid, an error describing why the value is invalid.
// Path should include the leaf name (e.g., ["bgp", "peer", "1.1.1.1", "receive-hold-time"]).
func (c *Completer) ValidateValueAtPath(path []string, value string) error {
	if c.loader == nil {
		return nil // No schema loaded — cannot validate
	}
	entry := c.getEntry(path)
	if entry == nil {
		return fmt.Errorf("unknown path: %s", textbuf.Join(path, " "))
	}
	if entry.Kind != gyang.LeafEntry {
		return fmt.Errorf("%s is not a leaf -- did you forget a value?", path[len(path)-1])
	}
	// Reject config false paths — read-only state cannot be set.
	// Check the leaf itself and all ancestors (config false is inherited per RFC 7950 §7.21.1).
	if c.isConfigFalse(path) {
		return fmt.Errorf("path %s is read-only (config false)", textbuf.Join(path, " "))
	}
	// Reject setting a list key leaf — the key is already the list identifier
	// (e.g., "address" in "peer 1.1.1.1" is the key, not a settable field)
	if len(path) >= 2 {
		parentEntry := c.getEntry(path[:len(path)-1])
		if parentEntry != nil && parentEntry.IsList() && parentEntry.Key == path[len(path)-1] {
			return fmt.Errorf("%s is the list key (already set as the identifier)", path[len(path)-1])
		}
	}
	if !validateLeafValue(entry, value) {
		hint := c.TypeHint(entry.Type)
		return fmt.Errorf("invalid value %q for %s (expected %s)", value, path[len(path)-1], hint)
	}
	return nil
}
