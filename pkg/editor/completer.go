package editor

import (
	"strings"

	"github.com/exa-networks/zebgp/pkg/config"
)

// Completion represents a single completion suggestion.
type Completion struct {
	Text        string // The completion text
	Description string // Help text
	Type        string // "command", "keyword", "value", "list-key"
}

// Completer provides schema-driven completions.
type Completer struct {
	schema *config.Schema
	tree   *config.Tree // Config data for list key completion
}

// NewCompleter creates a new completer with the given schema.
func NewCompleter(schema *config.Schema) *Completer {
	return &Completer{schema: schema}
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
// contextPath is the current edit context (e.g., ["neighbor", "192.168.1.1"]).
func (c *Completer) Complete(input string, contextPath []string) []Completion {
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
		node := c.navigateToContext(contextPath)
		if node != nil {
			children := c.getChildren(node)
			for _, name := range children {
				if strings.HasPrefix(name, cmd) {
					cmdCompletions = append(cmdCompletions, Completion{
						Text:        "set " + name,
						Description: "Set " + name,
						Type:        "keyword",
					})
				}
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
// Returns empty string if no clear single match or multiple matches.
func (c *Completer) GhostText(input string, contextPath []string) string {
	if input == "" {
		return ""
	}

	tokens := tokenize(input)
	if len(tokens) == 0 {
		return ""
	}

	// Get the last partial word
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
		// Single match: return the remainder
		return matches[0].Text[len(lastWord):]
	}

	if len(matches) > 1 {
		// Multiple matches: find common prefix beyond what's typed
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
	// Start from context path
	node := c.navigateToContext(contextPath)
	if node == nil {
		return nil
	}

	// Navigate through tokens
	for i, token := range tokens {
		isLast := i == len(tokens)-1

		if isLast && !endsWithSpace {
			// Partial match on this token
			return c.matchNode(node, token)
		}

		// Navigate deeper
		node = c.navigateNode(node, token)
		if node == nil {
			return nil
		}
	}

	// If we ended with space, show next level
	return c.matchNode(node, "")
}

// completeEditPath completes paths for edit command.
func (c *Completer) completeEditPath(tokens []string, contextPath []string, endsWithSpace bool) []Completion {
	// Start from context path
	node := c.navigateToContext(contextPath)
	if node == nil {
		return nil
	}

	// For edit, we're looking for lists and containers
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		return c.matchEditTargets(node, prefix)
	}

	// After first token (list name), show "*" for template and existing keys
	if len(tokens) == 1 && endsWithSpace {
		listName := tokens[0]
		child := c.getChild(node, listName)
		if child != nil && isListNode(child) {
			return c.listKeyCompletions(listName, "")
		}
	}

	// Partial match on list key
	if len(tokens) == 2 && !endsWithSpace {
		listName := tokens[0]
		child := c.getChild(node, listName)
		if child != nil && isListNode(child) {
			return c.listKeyCompletions(listName, tokens[1])
		}
	}

	return nil
}

// listKeyCompletions returns completions for list keys including existing entries.
func (c *Completer) listKeyCompletions(listName, prefix string) []Completion {
	var completions []Completion

	// Add wildcard template option
	if prefix == "" || strings.HasPrefix("*", prefix) {
		completions = append(completions, Completion{
			Text:        "*",
			Description: "Template for all entries",
			Type:        "list-key",
		})
	}

	// Add existing keys from config
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

	// Add hint for new entry if no matches
	if len(completions) == 0 || (prefix == "" && len(completions) == 1) {
		completions = append(completions, Completion{
			Text:        "<address>",
			Description: "New " + listName + " address",
			Type:        "list-key",
		})
	}

	return completions
}

// completeShowPath completes paths for show command.
func (c *Completer) completeShowPath(tokens []string, contextPath []string, endsWithSpace bool) []Completion {
	// Show can display sections - similar to edit targets
	node := c.navigateToContext(contextPath)
	if node == nil {
		return nil
	}

	// No tokens or partial first token: show sections
	if len(tokens) == 0 || (len(tokens) == 1 && !endsWithSpace) {
		prefix := ""
		if len(tokens) == 1 {
			prefix = tokens[0]
		}
		return c.matchEditTargets(node, prefix)
	}

	// After section name, show existing entries
	if len(tokens) == 1 && endsWithSpace {
		listName := tokens[0]
		child := c.getChild(node, listName)
		if child != nil && isListNode(child) {
			return c.listKeyCompletions(listName, "")
		}
	}

	return nil
}

// navigateToContext returns the schema node at the context path.
func (c *Completer) navigateToContext(contextPath []string) config.Node {
	if len(contextPath) == 0 {
		return c.schemaRoot()
	}

	node := c.schemaRoot()
	for i := 0; i < len(contextPath); i++ {
		part := contextPath[i]
		node = c.navigateNode(node, part)
		if node == nil {
			return nil
		}
	}
	return node
}

// schemaRoot returns the root of the schema as a Node.
func (c *Completer) schemaRoot() config.Node {
	// Schema root is accessed via Get/Has methods
	// We need a container-like wrapper
	return &schemaRootWrapper{schema: c.schema}
}

// schemaRootWrapper wraps Schema to implement container-like behavior.
type schemaRootWrapper struct {
	schema *config.Schema
}

func (w *schemaRootWrapper) Kind() config.NodeKind {
	return config.NodeContainer
}

// navigateNode navigates from a node using a token.
func (c *Completer) navigateNode(node config.Node, token string) config.Node {
	switch n := node.(type) {
	case *schemaRootWrapper:
		return n.schema.Get(token)
	case *config.ContainerNode:
		return n.Get(token)
	case *config.ListNode:
		// For lists, token is the key - return the list itself for children
		return n
	case *config.FlexNode:
		return n.Get(token)
	default:
		return nil
	}
}

// getChild returns a child node by name.
func (c *Completer) getChild(node config.Node, name string) config.Node {
	return c.navigateNode(node, name)
}

// matchNode returns completions matching a prefix from a node's children.
func (c *Completer) matchNode(node config.Node, prefix string) []Completion {
	children := c.getChildren(node)
	var completions []Completion

	for _, name := range children {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			child := c.getChild(node, name)
			completions = append(completions, Completion{
				Text:        name,
				Description: c.nodeDescription(child),
				Type:        c.nodeType(child),
			})
		}
	}

	// If node is a leaf, show value hints
	if leaf := asLeaf(node); leaf != nil {
		return c.valueCompletions(leaf.Type, prefix)
	}

	return completions
}

// matchEditTargets returns completions for edit command (lists and containers).
func (c *Completer) matchEditTargets(node config.Node, prefix string) []Completion {
	children := c.getChildren(node)
	var completions []Completion

	for _, name := range children {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			child := c.getChild(node, name)
			if isListNode(child) || isContainerNode(child) {
				completions = append(completions, Completion{
					Text:        name,
					Description: c.nodeDescription(child),
					Type:        c.nodeType(child),
				})
			}
		}
	}

	return completions
}

// getChildren returns the child names of a node.
func (c *Completer) getChildren(node config.Node) []string {
	switch n := node.(type) {
	case *schemaRootWrapper:
		// Get top-level schema children
		var children []string
		for _, name := range []string{"router-id", "local-as", "neighbor", "process"} {
			if n.schema.Has(name) {
				children = append(children, name)
			}
		}
		return children
	case *config.ContainerNode:
		return n.Children()
	case *config.ListNode:
		return n.Children()
	case *config.FlexNode:
		return n.Children()
	default:
		return nil
	}
}

// nodeDescription returns a description for a node.
func (c *Completer) nodeDescription(node config.Node) string {
	switch n := node.(type) {
	case *config.LeafNode:
		return n.Type.String() + " value"
	case *config.ListNode:
		return "list of " + n.KeyType.String()
	case *config.ContainerNode:
		return "configuration section"
	case *config.FlexNode:
		return "optional section"
	default:
		return ""
	}
}

const completionTypeKeyword = "keyword"

// nodeType returns the completion type for a node.
func (c *Completer) nodeType(_ config.Node) string {
	return completionTypeKeyword
}

// valueCompletions returns completions for a value type.
func (c *Completer) valueCompletions(typ config.ValueType, prefix string) []Completion {
	switch typ { //nolint:exhaustive // TypeString handled by default
	case config.TypeBool:
		return filterCompletions([]Completion{
			{Text: "true", Description: "Enable", Type: "value"},
			{Text: "false", Description: "Disable", Type: "value"},
		}, prefix)
	case config.TypeIPv4:
		return []Completion{{Text: "<ipv4>", Description: "IPv4 address (e.g., 1.2.3.4)", Type: "value"}}
	case config.TypeIPv6:
		return []Completion{{Text: "<ipv6>", Description: "IPv6 address", Type: "value"}}
	case config.TypeIP:
		return []Completion{{Text: "<ip>", Description: "IP address", Type: "value"}}
	case config.TypeUint16:
		return []Completion{{Text: "<0-65535>", Description: "16-bit number", Type: "value"}}
	case config.TypeUint32:
		return []Completion{{Text: "<0-4294967295>", Description: "32-bit number (e.g., ASN)", Type: "value"}}
	case config.TypePrefix:
		return []Completion{{Text: "<prefix>", Description: "CIDR prefix (e.g., 10.0.0.0/8)", Type: "value"}}
	default:
		return []Completion{{Text: "<value>", Description: "String value", Type: "value"}}
	}
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

func isListNode(node config.Node) bool {
	_, ok := node.(*config.ListNode)
	if ok {
		return true
	}
	_, ok = node.(*config.InlineListNode)
	return ok
}

func isContainerNode(node config.Node) bool {
	_, ok := node.(*config.ContainerNode)
	return ok
}

func asLeaf(node config.Node) *config.LeafNode {
	leaf, ok := node.(*config.LeafNode)
	if ok {
		return leaf
	}
	return nil
}
