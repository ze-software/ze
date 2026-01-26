package editor

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
	hubschema "codeberg.org/thomas-mangin/ze/internal/hub/schema"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/yang"
	gyang "github.com/openconfig/goyang/pkg/yang"
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
	// Load module-specific YANG from their packages
	if err := loader.AddModuleFromText("ze-hub.yang", hubschema.ZeHubYANG); err != nil {
		return &Completer{}
	}
	if err := loader.AddModuleFromText("ze-bgp.yang", bgpschema.ZeBGPYANG); err != nil {
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
	{Text: "set", Description: "Set a configuration value", Type: "command"},
	{Text: "delete", Description: "Delete a configuration value", Type: "command"},
	{Text: "edit", Description: "Enter a subsection context", Type: "command"},
	{Text: "show", Description: "Display configuration", Type: "command"},
	{Text: "compare", Description: "Show diff vs original", Type: "command"},
	{Text: "commit", Description: "Save changes with backup", Type: "command"},
	{Text: "discard", Description: "Revert all changes", Type: "command"},
	{Text: "top", Description: "Return to root context", Type: "command"},
	{Text: "up", Description: "Go up one level", Type: "command"},
	{Text: "history", Description: "List backup files", Type: "command"},
	{Text: "rollback", Description: "Restore from backup", Type: "command"},
	{Text: "exit", Description: "Exit editor", Type: "command"},
	{Text: "help", Description: "Show help", Type: "command"},
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
					Text:        "set " + name,
					Description: "Set " + name,
					Type:        "keyword",
				})
			}
		}
		return cmdCompletions
	}

	// Dispatch based on command
	switch cmd {
	case "set", "delete":
		return c.completeSetPath(tokens[1:], contextPath, endsWithSpace)
	case "edit":
		return c.completeEditPath(tokens[1:], contextPath, endsWithSpace)
	case "show":
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
func (c *Completer) completeSetPath(tokens []string, contextPath []string, endsWithSpace bool) []Completion {
	currentPath := append([]string{}, contextPath...)

	// Navigate through tokens
	for i, token := range tokens {
		isLast := i == len(tokens)-1

		if isLast && !endsWithSpace {
			// Partial match on this token
			return c.matchChildren(currentPath, token)
		}

		// Navigate deeper
		currentPath = append(currentPath, token)
	}

	// If we ended with space, show next level
	return c.matchChildren(currentPath, "")
}

// completeEditPath completes paths for edit command.
func (c *Completer) completeEditPath(tokens []string, contextPath []string, endsWithSpace bool) []Completion {
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
			return c.listKeyCompletions(tokens[0], "")
		}
	}

	// Partial match on list key
	if len(tokens) == 2 && !endsWithSpace {
		listPath := append(append([]string{}, contextPath...), tokens[0])
		if c.isList(listPath) {
			return c.listKeyCompletions(tokens[0], tokens[1])
		}
	}

	return nil
}

// completeShowPath completes paths for show command.
func (c *Completer) completeShowPath(tokens []string, contextPath []string, endsWithSpace bool) []Completion {
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
			return c.listKeyCompletions(tokens[0], "")
		}
	}

	return nil
}

// listKeyCompletions returns completions for list keys.
func (c *Completer) listKeyCompletions(listName, prefix string) []Completion {
	var completions []Completion

	// Add wildcard template option
	if prefix == "" || prefix == "*" {
		completions = append(completions, Completion{
			Text:        "*",
			Description: "Template for all entries",
			Type:        "list-key",
		})
	}

	// Add existing keys from config tree
	if c.tree != nil {
		keys := c.tree.ListKeys(listName)
		for _, key := range keys {
			if prefix == "" || strings.HasPrefix(key, prefix) {
				completions = append(completions, Completion{
					Text:        key,
					Description: "Existing " + listName,
					Type:        "list-key",
				})
			}
		}
	}

	// Add hint for new entry
	if len(completions) <= 1 {
		completions = append(completions, Completion{
			Text:        "<value>",
			Description: "New " + listName + " key",
			Type:        "list-key",
		})
	}

	return completions
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

	// Get children
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

// matchEditTargets returns completions for containers and lists.
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
			if child.Dir != nil { // Container or list
				completions = append(completions, Completion{
					Text:        name,
					Description: c.entryDescription(child),
					Type:        "keyword",
				})
			}
		}
	}

	return completions
}

// valueCompletions returns completions for a leaf value.
func (c *Completer) valueCompletions(entry *gyang.Entry, prefix string) []Completion {
	if entry.Type == nil {
		return []Completion{{Text: "<value>", Description: "value", Type: "value"}}
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

	// Type hint based on YANG type
	hint := c.typeHint(entry.Type)
	return []Completion{{Text: "<" + hint + ">", Description: hint + " value", Type: "value"}}
}

// typeHint returns a hint string for a YANG type.
func (c *Completer) typeHint(t *gyang.YangType) string {
	if t == nil {
		return "value"
	}
	//nolint:exhaustive // default handles all other types
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
	default:
		return "value"
	}
}

// entryDescription returns description for a YANG entry.
func (c *Completer) entryDescription(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	desc := entry.Description
	if entry.Mandatory == gyang.TSTrue {
		if desc != "" {
			desc += " (required)"
		} else {
			desc = "required"
		}
	}
	return desc
}

// getEntry returns the YANG entry at the given path.
func (c *Completer) getEntry(path []string) *gyang.Entry {
	if c.loader == nil {
		return nil
	}

	// Start with bgp module
	entry := c.loader.GetEntry("ze-bgp")
	if entry == nil {
		return nil
	}

	// Navigate through path
	for _, part := range path {
		if entry.Dir == nil {
			return nil
		}
		child, ok := entry.Dir[part]
		if !ok {
			return nil
		}
		entry = child
	}

	return entry
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
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:minLen]
}
