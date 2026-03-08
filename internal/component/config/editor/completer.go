// Design: docs/architecture/config/yang-config-design.md — config editor
// Related: completer_command.go — command mode operational completion

package editor

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// Completion represents a single completion suggestion.
type Completion struct {
	Text        string // The completion text
	Description string // Help text
	Type        string // "command", "keyword", "value", "list-key"
}

// Completer provides YANG-driven completions.
type Completer struct {
	loader *yang.Loader
	tree   *config.Tree // Config data for list key completion
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
	return &Completer{loader: loader}
}

// SetTree sets the config tree for data-aware completion.
func (c *Completer) SetTree(tree *config.Tree) {
	c.tree = tree
}

// commands returns the available editor commands.
var commands = []Completion{
	{Text: cmdSet, Description: "Set a configuration value", Type: "command"},
	{Text: cmdDelete, Description: "Delete a configuration value", Type: "command"},
	{Text: cmdEdit, Description: "Enter a subsection context", Type: "command"},
	{Text: cmdShow, Description: "Display configuration", Type: "command"},
	{Text: cmdCompare, Description: "Show diff vs original", Type: "command"},
	{Text: cmdCommit, Description: "Save changes with backup", Type: "command"},
	{Text: cmdDiscard, Description: "Revert all changes", Type: "command"},
	{Text: cmdTop, Description: "Return to root context", Type: "command"},
	{Text: cmdUp, Description: "Go up one level", Type: "command"},
	{Text: cmdHistory, Description: "List backup files", Type: "command"},
	{Text: cmdRollback, Description: "Restore from backup", Type: "command"},
	{Text: cmdExit, Description: "Exit editor", Type: "command"},
	{Text: cmdHelp, Description: "Show help", Type: "command"},
	{Text: cmdCommand, Description: "Switch to operational command mode", Type: "command"},
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
		children := c.getChildrenAtPath(contextPath)
		for _, name := range children {
			if strings.HasPrefix(name, cmd) {
				cmdCompletions = append(cmdCompletions, Completion{
					Text:        cmdSet + " " + name,
					Description: "Set " + name,
					Type:        "keyword",
				})
			}
		}
		return cmdCompletions
	}

	// Dispatch based on command
	switch cmd {
	case cmdSet, cmdDelete:
		return c.completeSetPath(tokens[1:], contextPath, endsWithSpace)
	case cmdEdit:
		return c.completeEditPath(tokens[1:], contextPath, endsWithSpace)
	case cmdShow:
		return c.completeShowPath(tokens[1:], contextPath, endsWithSpace)
	default:
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

// completeSetPath completes paths for set/delete commands.
func (c *Completer) completeSetPath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	currentPath := append([]string{}, contextPath...)
	// Track how many path elements came from tokens (vs context)
	tokensAdded := 0

	// Navigate through tokens, handling list keys
	for i, token := range tokens {
		isLast := i == len(tokens)-1

		if isLast && !endsWithSpace {
			// Partial match on this token
			// Check if we're at a list that hasn't been keyed yet — show existing keys
			if tokensAdded > 0 && c.isListNeedingKey(currentPath) {
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
				// Invalid key value — return no completions (user needs to fix the key)
				return nil
			}
		}
		currentPath = append(currentPath, token)
		tokensAdded++
	}

	// If we ended with space, show next level
	// Check if current path ends at a list that still needs a key
	if tokensAdded > 0 && c.isListNeedingKey(currentPath) && len(currentPath) > 0 {
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
	lastName := path[len(path)-1]
	parentEntry := c.getEntry(path[:len(path)-1])
	if parentEntry != nil && parentEntry.Dir != nil {
		if _, isSchemaChild := parentEntry.Dir[lastName]; isSchemaChild {
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

// completeShowPath completes paths for show command.
func (c *Completer) completeShowPath(tokens, contextPath []string, endsWithSpace bool) []Completion {
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		return c.matchEditTargets(contextPath, prefix)
	}

	if len(tokens) == 1 && endsWithSpace {
		listPath := append(append([]string{}, contextPath...), tokens[0])
		if c.isList(listPath) {
			return c.listKeyCompletions(tokens[0], "", contextPath)
		}
	}

	return nil
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
			Type:        "list-key",
		})
	}

	// Multiple entries — show actual keys for named entries, #N for unnamed
	for i, entry := range orderedEntries {
		if isDefaultKey(entry.Key) {
			// Unnamed entry — show as positional ID
			id := fmt.Sprintf("#%d", i+1)
			if prefix == "" || strings.HasPrefix(id, prefix) {
				completions = append(completions, Completion{
					Text:        id,
					Description: listName,
					Type:        "list-key",
				})
			}
		} else if prefix == "" || strings.HasPrefix(entry.Key, prefix) {
			// Named entry — show actual key
			completions = append(completions, Completion{
				Text:        entry.Key,
				Description: listName,
				Type:        "list-key",
			})
		}
	}

	if prefix == "" && entryCount == 0 {
		// No entries and nothing typed — show placeholder hint (display-only, not applicable)
		completions = append(completions, Completion{
			Text:        "<value>",
			Description: "New " + listName + " key",
			Type:        "hint",
		})
	} else if prefix != "" && len(completions) == 0 {
		// User typed a value that doesn't match any existing key —
		// validate against YANG key type before offering as completion.
		listPath := append(append([]string{}, contextPath...), listName)
		keyEntry := c.getListKeyEntry(listPath)
		if validateLeafValue(keyEntry, prefix) {
			completions = append(completions, Completion{
				Text:        prefix,
				Description: "New " + listName + " key",
				Type:        "list-key",
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
			if entry.Dir == nil {
				return nil
			}
			child, ok := entry.Dir[part]
			if !ok {
				return nil
			}
			entry = child
		}

		// If this is a list, next path element is the key value
		if entry.IsList() && i+1 < len(contextPath) {
			nextPart := contextPath[i+1]
			if _, hasChild := entry.Dir[nextPart]; !hasChild {
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
			child := entry.Dir[name]
			completions = append(completions, Completion{
				Text:        name,
				Description: c.entryDescription(child),
				Type:        "keyword",
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
			child := entry.Dir[name]
			completions = append(completions, Completion{
				Text:        name,
				Description: c.entryDescription(child),
				Type:        "keyword",
			})
		}
	}

	return completions
}

// valueCompletions returns completions for a leaf value.
func (c *Completer) valueCompletions(entry *gyang.Entry, prefix string) []Completion {
	if entry.Type == nil {
		return []Completion{{Text: "<value>", Description: "value", Type: "hint"}}
	}

	// Handle enums
	if entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
		var completions []Completion
		for _, name := range entry.Type.Enum.Names() {
			if prefix == "" || strings.HasPrefix(name, prefix) {
				completions = append(completions, Completion{
					Text:        name,
					Description: "enum value",
					Type:        "value",
				})
			}
		}
		return completions
	}

	// Handle booleans
	if entry.Type.Kind == gyang.Ybool {
		return filterCompletions([]Completion{
			{Text: "true", Description: "Enable", Type: "value"},
			{Text: "false", Description: "Disable", Type: "value"},
		}, prefix)
	}

	// Type hint based on YANG type — hint-only, not applicable by Tab
	hint := c.typeHint(entry.Type)
	return []Completion{{Text: "<" + hint + ">", Description: hint + " value", Type: "hint"}}
}

// typeHint returns a hint string for a YANG type.
func (c *Completer) typeHint(t *gyang.YangType) string {
	if t == nil {
		return "value"
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
			return strings.Join(names, " or ")
		}
	}
	// For typedef and other types: use the YANG type name if available
	if t.Name != "" && t.Name != "union" {
		return t.Name
	}
	return "value"
}

// entryDescription returns description for a YANG entry.
func (c *Completer) entryDescription(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	desc := entry.Description
	if entry.Mandatory == gyang.TSTrue && !strings.Contains(desc, "(required)") {
		if desc != "" {
			desc += " (required)"
		} else {
			desc = "required"
		}
	}
	return desc
}

// confModules lists the YANG config modules to search for schema entries.
var confModules = []string{"ze-bgp-conf", "ze-hub-conf", "ze-plugin-conf"}

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
		if entry.Dir == nil {
			return nil
		}
		child, ok := entry.Dir[part]
		if !ok {
			return nil
		}
		entry = child

		// If this is a list and there's a next element that's not in Dir, skip the key
		if entry.IsList() && i+1 < len(path) {
			nextPart := path[i+1]
			if _, hasChild := entry.Dir[nextPart]; !hasChild {
				// Next element is a key value, skip it
				i++
			}
		}
	}

	return entry
}

// mergedRoot returns a virtual root entry with children from all config modules.
func (c *Completer) mergedRoot() *gyang.Entry {
	root := &gyang.Entry{
		Kind: gyang.DirectoryEntry,
		Dir:  make(map[string]*gyang.Entry),
	}
	for _, modName := range confModules {
		modEntry := c.loader.GetEntry(modName)
		if modEntry == nil || modEntry.Dir == nil {
			continue
		}
		maps.Copy(root.Dir, modEntry.Dir)
	}
	if len(root.Dir) == 0 {
		return nil
	}
	return root
}

// findModuleEntry searches all config modules for a top-level child by name,
// returning the child entry (not the module root).
func (c *Completer) findModuleEntry(name string) *gyang.Entry {
	for _, modName := range confModules {
		modEntry := c.loader.GetEntry(modName)
		if modEntry == nil || modEntry.Dir == nil {
			continue
		}
		if child, ok := modEntry.Dir[name]; ok {
			return child
		}
	}
	return nil
}

// getChildrenAtPath returns children names at path.
func (c *Completer) getChildrenAtPath(path []string) []string {
	entry := c.getEntry(path)
	if entry == nil || entry.Dir == nil {
		return nil
	}
	return c.getSortedChildren(entry)
}

// getSortedChildren returns sorted child names.
func (c *Completer) getSortedChildren(entry *gyang.Entry) []string {
	if entry == nil || entry.Dir == nil {
		return nil
	}
	children := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
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
// Path should include the leaf name (e.g., ["bgp", "peer", "1.1.1.1", "hold-time"]).
func (c *Completer) ValidateValueAtPath(path []string, value string) error {
	if c.loader == nil {
		return nil // No schema loaded — cannot validate
	}
	entry := c.getEntry(path)
	if entry == nil {
		return fmt.Errorf("unknown path: %s", strings.Join(path, " "))
	}
	if entry.Kind != gyang.LeafEntry {
		return fmt.Errorf("%s is not a settable leaf", path[len(path)-1])
	}
	// Reject config false paths — read-only state cannot be set.
	// Check the leaf itself and all ancestors (config false is inherited per RFC 7950 §7.21.1).
	if c.isConfigFalse(path) {
		return fmt.Errorf("path %s is read-only (config false)", strings.Join(path, " "))
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
		hint := c.typeHint(entry.Type)
		return fmt.Errorf("invalid value %q for %s (expected %s)", value, path[len(path)-1], hint)
	}
	return nil
}

// isConfigFalse checks if any node in the path has config false set.
// YANG config false is inherited: if a container is config false, all children are too.
func (c *Completer) isConfigFalse(path []string) bool {
	if c.loader == nil {
		return false
	}
	// Check each prefix of the path for config false
	for i := 1; i <= len(path); i++ {
		entry := c.getEntry(path[:i])
		if entry != nil && entry.Config == gyang.TSFalse {
			return true
		}
	}
	return false
}

// validateTokenPath walks the full token path (including list key values) against the schema.
// Unlike getEntry (which skips list keys silently), this enforces that every list has a key value.
// Returns the leaf entry at the end of the path, or an error if the path is invalid.
func (c *Completer) validateTokenPath(tokens []string) (*gyang.Entry, error) {
	if c.loader == nil {
		return nil, fmt.Errorf("no schema loaded")
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	entry := c.findModuleEntry(tokens[0])
	if entry == nil {
		return nil, fmt.Errorf("unknown path: %s", tokens[0])
	}

	for i := 1; i < len(tokens); i++ {
		part := tokens[i]
		if entry.Dir == nil {
			return nil, fmt.Errorf("unknown path: %s", strings.Join(tokens[:i+1], " "))
		}
		child, ok := entry.Dir[part]
		if !ok {
			return nil, fmt.Errorf("unknown path: %s", strings.Join(tokens[:i+1], " "))
		}
		entry = child

		// If this is a list, the next token MUST be a key value (not a schema child name).
		// If the next token is a known child, the user forgot the list key.
		if entry.IsList() {
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("%s is a list — requires a key (e.g., %s <key> ...)", part, part)
			}
			nextToken := tokens[i+1]
			if _, isChild := entry.Dir[nextToken]; isChild {
				// Next token is a schema child, not a key value — key is missing
				return nil, fmt.Errorf("%s is a list — requires a key (e.g., %s <key> %s ...)", part, part, nextToken)
			}
			// Next token is the key value — skip it
			i++
		}
	}

	return entry, nil
}

// getListKeyEntry returns the YANG entry for a list's key leaf.
// For example, peer list with key "address" returns the address leaf entry.
func (c *Completer) getListKeyEntry(listPath []string) *gyang.Entry {
	listEntry := c.getEntry(listPath)
	if listEntry == nil || !listEntry.IsList() || listEntry.Key == "" {
		return nil
	}
	if listEntry.Dir == nil {
		return nil
	}
	keyLeaf, ok := listEntry.Dir[listEntry.Key]
	if !ok {
		return nil
	}
	return keyLeaf
}

// validateLeafValue checks if a value is valid for a given YANG leaf type.
// Returns true if the value passes type validation, false if it's clearly invalid.
// Used to prevent Tab from accepting invalid list keys or set values.
func validateLeafValue(entry *gyang.Entry, value string) bool {
	if entry == nil || entry.Type == nil {
		return true // No type info — accept anything
	}
	return validateYangType(entry.Type, value)
}

// validateYangType checks a value against a resolved YANG type.
// Types not explicitly handled are accepted (completer assists, validator enforces).
func validateYangType(t *gyang.YangType, value string) bool {
	if t == nil {
		return true
	}

	// Recognized types with validation
	switch t.Kind { //nolint:exhaustive // unrecognized types accepted — completer assists, validator enforces
	case gyang.Yunion:
		// Union: valid if any member type accepts it
		for _, member := range t.Type {
			if validateYangType(member, value) {
				return true
			}
		}
		return false

	case gyang.Ystring:
		if len(t.Pattern) > 0 {
			return validateStringPatterns(t, value)
		}

	case gyang.Yuint8:
		return validateUintRange(value, 0, 255)
	case gyang.Yuint16:
		return validateUintRange(value, 0, 65535)
	case gyang.Yuint32:
		return validateUintRange(value, 0, 4294967295)

	case gyang.Ybool:
		return value == "true" || value == "false"

	case gyang.Yenum:
		if t.Enum == nil {
			return true
		}
		return slices.Contains(t.Enum.Names(), value)
	}

	return true // Unrecognized types accepted — completer is best-effort, validator is authoritative
}

// validateStringPatterns checks if a value could match any of the YANG patterns.
// For IP address types specifically, uses net.ParseIP for robust validation.
func validateStringPatterns(t *gyang.YangType, value string) bool {
	// Check if any pattern looks like an IP address pattern.
	// YANG ip-address typedef resolves to union of string-with-pattern types.
	// Rather than implementing full regex (YANG uses XSD patterns, not Go regex),
	// detect IP types by their pattern structure and use net.ParseIP.
	for _, p := range t.Pattern {
		if strings.Contains(p, "25[0-5]") || strings.Contains(p, "[0-9a-fA-F]") {
			// IPv4 or IPv6 pattern — validate with net.ParseIP
			return net.ParseIP(value) != nil
		}
	}
	// Non-IP string patterns: accept (full XSD regex validation is complex)
	return true
}

// validateUintRange validates a string as an unsigned integer within [min, max].
func validateUintRange(value string, minVal, maxVal uint64) bool {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return false
	}
	return n >= minVal && n <= maxVal
}
