// Design: docs/architecture/core-design.md — interactive CLI
// Related: ../run/main.go — run convenience command (uses BuildCommandTree)
// Related: ../show/main.go — show convenience command (uses BuildCommandTree)
// Related: ../internal/cmdutil/cmdutil.go — shared command utilities (uses BuildCommandTree)
//
// Package cli provides the ze cli subcommand.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/cache"             // init() registers cache command RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/commit"            // init() registers commit command RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/meta"              // init() registers help/discovery RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"              // init() registers peer management RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"               // init() registers raw message RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/subscribe"         // init() registers subscribe/unsubscribe RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"            // init() registers update parsing RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler" // init() registers route-refresh command RPCs
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Run executes the cli subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	// Check for help first
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return 0
	}

	// Check for subsystem prefix (e.g., "ze cli bgp ...")
	if len(args) > 0 && args[0] == "bgp" {
		return runBGP(args[1:])
	}

	// Default: BGP subsystem (only one for now)
	return runBGP(args)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze cli [subsystem] [options]

Interactive CLI for Ze daemons.

Subsystems:
  bgp    BGP daemon (default)

Options:
  --socket <path>    Path to API socket (default: auto-detected)
  --run <command>    Execute single command and exit

Examples:
  ze cli                           Interactive BGP CLI
  ze cli bgp                       Interactive BGP CLI (explicit)
  ze cli --run "peer list"         Execute command
  ze cli bgp --run "daemon status" Execute command (explicit)
`)
}

// runBGP runs the BGP CLI.
func runBGP(args []string) int {
	fs := flag.NewFlagSet("cli", flag.ExitOnError)
	socketPath := fs.String("socket", config.DefaultSocketPath(), "Path to API socket")
	runCmd := fs.String("run", "", "Execute single command and exit")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Try to connect to daemon
	client, err := newCLIClient(*socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot connect to %s: %v\n", *socketPath, err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}
	defer func() { _ = client.Close() }()

	// If --run specified, execute single command and exit
	if *runCmd != "" {
		return client.Execute(*runCmd)
	}

	// Create initial model
	ti := textinput.New()
	ti.Placeholder = "type command or press Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	m := model{
		textInput:  ti,
		client:     client,
		historyIdx: -1,
	}

	// Run the bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

// rpcResponse covers both success and error JSON RPC wire formats.
type rpcResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// cliClient handles communication with the API server using NUL-framed JSON RPC.
type cliClient struct {
	conn    net.Conn
	reader  *rpc.FrameReader
	writer  *rpc.FrameWriter
	cmdMap  map[string]string // lowercase CLI command → wire method
	cmdKeys []string          // sorted by length descending for longest-match
}

func newCLIClient(socketPath string) (*cliClient, error) {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, err
	}

	// Build command map from all registered RPCs (builtins + BGP handlers)
	cmdMap := make(map[string]string)
	for _, reg := range AllCLIRPCs() {
		cmdMap[strings.ToLower(reg.CLICommand)] = reg.WireMethod
	}

	// Sort keys by length descending for longest-match
	keys := make([]string, 0, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	return &cliClient{
		conn:    conn,
		reader:  rpc.NewFrameReader(conn),
		writer:  rpc.NewFrameWriter(conn),
		cmdMap:  cmdMap,
		cmdKeys: keys,
	}, nil
}

func (c *cliClient) Close() error {
	return c.conn.Close()
}

// resolveCommand finds the wire method for a text command.
// Tries "bgp <input>" first (default subsystem), then raw input.
// Returns wire method and any remaining args after the matched prefix.
func (c *cliClient) resolveCommand(input string) (method string, args []string) {
	// Try with "bgp " prefix first (default subsystem)
	if m, a := c.matchCommand("bgp " + input); m != "" {
		return m, a
	}
	// Try raw input for system/rib/plugin commands
	return c.matchCommand(input)
}

// matchCommand does longest-prefix matching against registered CLI commands.
func (c *cliClient) matchCommand(input string) (method string, args []string) {
	lower := strings.ToLower(strings.TrimSpace(input))
	for _, key := range c.cmdKeys {
		if strings.HasPrefix(lower, key) {
			// Check word boundary
			if len(lower) == len(key) || lower[len(key)] == ' ' {
				remaining := strings.TrimSpace(input[len(key):])
				if remaining != "" {
					args = strings.Fields(remaining)
				}
				return c.cmdMap[key], args
			}
		}
	}
	return "", nil
}

// Execute sends a command and prints the response.
func (c *cliClient) Execute(command string) int {
	resp, err := c.SendCommand(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	c.PrintResponse(resp)

	if resp.Error != "" {
		return 1
	}
	return 0
}

// SendCommand sends a command as JSON RPC and returns the response.
func (c *cliClient) SendCommand(command string) (*rpcResponse, error) {
	method, args := c.resolveCommand(command)
	if method == "" {
		return &rpcResponse{Error: "unknown command: " + command}, nil
	}

	// Build JSON RPC request
	req := rpc.Request{Method: method}
	if len(args) > 0 {
		params := struct {
			Args []string `json:"args,omitempty"`
		}{Args: args}
		paramBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = paramBytes
	}

	// Send NUL-framed JSON
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := c.writer.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Read NUL-framed response
	respBytes, err := c.reader.Read()
	if err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}

	var resp rpcResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &resp, nil
}

// PrintResponse formats and prints a response.
func (c *cliClient) PrintResponse(resp *rpcResponse) {
	if resp.Error != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		return
	}

	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		fmt.Println("OK")
		return
	}

	// Pretty print result
	var data any
	if err := json.Unmarshal(resp.Result, &data); err != nil {
		fmt.Println(string(resp.Result))
		return
	}

	printValue(data, "")
}

// printValue recursively prints a JSON value with indentation.
func printValue(v any, indent string) {
	switch val := v.(type) {
	case map[string]any:
		for key, value := range val {
			switch child := value.(type) {
			case map[string]any:
				fmt.Printf("%s%s:\n", indent, key)
				printValue(child, indent+"  ")
			case []any:
				if len(child) == 0 {
					fmt.Printf("%s%s: (none)\n", indent, key)
				} else {
					fmt.Printf("%s%s:\n", indent, key)
					printValue(child, indent+"  ")
				}
			default:
				fmt.Printf("%s%s: %v\n", indent, key, formatNumber(value))
			}
		}
	case []any:
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				// For peer info, format nicely
				if addr, ok := m["Address"]; ok {
					state := m["State"]
					fmt.Printf("%s%v [%v]\n", indent, addr, state)
					continue
				}
			}
			fmt.Printf("%s- %v\n", indent, item)
		}
	default:
		fmt.Printf("%s%v\n", indent, v)
	}
}

// formatNumber displays integers without decimal points.
func formatNumber(v any) any {
	if n, ok := v.(float64); ok {
		if n == float64(int64(n)) {
			return int64(n)
		}
	}
	return v
}

// AllCLIRPCs returns all RPCs needed for CLI command mapping.
// All RPCs self-register via init() + pluginserver.RegisterRPCs().
// Exported so other CLI commands (e.g., ze show) can build from the same source.
func AllCLIRPCs() []pluginserver.RPCRegistration {
	return pluginserver.AllBuiltinRPCs()
}

// BuildCommandTree builds the command tree from registered RPCs.
// Strips the "bgp " prefix for BGP commands to create the user-facing tree.
// If readOnly is true, only includes RPCs marked ReadOnly (for "ze show").
func BuildCommandTree(readOnly bool) *Command {
	root := &Command{Children: make(map[string]*Command)}

	for _, reg := range AllCLIRPCs() {
		if readOnly && !reg.ReadOnly {
			continue
		}

		cmd := reg.CLICommand

		// Strip "bgp " prefix for BGP commands (user types "peer list", not "bgp peer list")
		cmd = strings.TrimPrefix(cmd, "bgp ")

		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}

		current := root
		for _, part := range parts {
			if current.Children == nil {
				current.Children = make(map[string]*Command)
			}
			child, ok := current.Children[part]
			if !ok {
				child = &Command{Name: part}
				current.Children[part] = child
			}
			current = child
		}
		current.Description = reg.Help
	}

	return root
}

// Command represents a CLI command with metadata.
type Command struct {
	Name        string
	Description string
	Children    map[string]*Command
}

// commandTree holds all available commands for completion.
var commandTree = BuildCommandTree(false)

// Styles.
var (
	promptStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	suggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// model is the bubbletea model for the CLI.
type model struct {
	textInput   textinput.Model
	suggestions []suggestion
	selected    int
	output      string
	err         error
	client      *cliClient
	quitting    bool
	width       int
	height      int
	history     []string // Previous commands (oldest first)
	historyIdx  int      // Current position in history (-1 = not browsing)
	historyTmp  string   // Saved current input when browsing history
}

type suggestion struct {
	text        string
	description string
}

// Messages.
type executeResultMsg struct {
	output string
	err    error
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // default handles all other keys
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyTab:
			// Cycle through suggestions
			if len(m.suggestions) > 0 {
				m.selected = (m.selected + 1) % len(m.suggestions)
				// Apply the selected suggestion
				m.applySelectedSuggestion()
			}
			return m, nil

		case tea.KeyShiftTab:
			// Cycle backwards through suggestions
			if len(m.suggestions) > 0 {
				m.selected--
				if m.selected < 0 {
					m.selected = len(m.suggestions) - 1
				}
				m.applySelectedSuggestion()
			}
			return m, nil

		case tea.KeyUp:
			m = m.handleHistoryUp()
			return m, nil

		case tea.KeyDown:
			m = m.handleHistoryDown()
			return m, nil

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			if input == "quit" || input == "exit" {
				m.quitting = true
				return m, tea.Quit
			}

			// Save to history (deduplicate consecutive duplicates)
			if len(m.history) == 0 || m.history[len(m.history)-1] != input {
				m.history = append(m.history, input)
			}
			m.historyIdx = -1
			m.historyTmp = ""

			// Execute command
			m.textInput.SetValue("")
			m.suggestions = nil
			m.selected = 0
			return m, m.executeCommand(input)

		default: // handle all other keys via textinput
			m.textInput, cmd = m.textInput.Update(msg)
			m.updateSuggestions()
			m.selected = 0
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = min(msg.Width-4, 80)
		return m, nil

	case executeResultMsg:
		m.output = msg.output
		m.err = msg.err
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleHistoryUp recalls the previous command from history.
func (m model) handleHistoryUp() model {
	if len(m.history) == 0 {
		return m
	}

	if m.historyIdx == -1 {
		// Start browsing: save current input, go to most recent
		m.historyTmp = m.textInput.Value()
		m.historyIdx = len(m.history) - 1
	} else if m.historyIdx > 0 {
		m.historyIdx--
	}

	m.textInput.SetValue(m.history[m.historyIdx])
	m.textInput.CursorEnd()
	m.updateSuggestions()
	return m
}

// handleHistoryDown recalls the next command from history, or restores the original input.
func (m model) handleHistoryDown() model {
	if m.historyIdx == -1 {
		return m
	}

	if m.historyIdx < len(m.history)-1 {
		m.historyIdx++
		m.textInput.SetValue(m.history[m.historyIdx])
	} else {
		// Back to current input
		m.historyIdx = -1
		m.textInput.SetValue(m.historyTmp)
		m.historyTmp = ""
	}

	m.textInput.CursorEnd()
	m.updateSuggestions()
	return m
}

func (m *model) applySelectedSuggestion() {
	if m.selected >= 0 && m.selected < len(m.suggestions) {
		// Get current input and find the last word
		input := m.textInput.Value()
		words := strings.Fields(input)

		// Replace the last partial word with the suggestion
		if len(words) > 0 {
			// Check if input ends with space (completing new word)
			if strings.HasSuffix(input, " ") || input == "" {
				m.textInput.SetValue(input + m.suggestions[m.selected].text + " ")
			} else {
				// Replace last word
				words[len(words)-1] = m.suggestions[m.selected].text
				m.textInput.SetValue(strings.Join(words, " ") + " ")
			}
		} else {
			m.textInput.SetValue(m.suggestions[m.selected].text + " ")
		}
		m.textInput.CursorEnd()
	}
}

func (m *model) updateSuggestions() {
	input := m.textInput.Value()
	words := strings.Fields(input)

	// Navigate command tree
	current := commandTree
	var partial string

	for i, word := range words {
		if current.Children == nil {
			m.suggestions = nil
			return
		}

		// Check if this is the last word (partial match)
		if i == len(words)-1 && !strings.HasSuffix(input, " ") {
			partial = word
			break
		}

		// Find exact match to navigate deeper
		if child, ok := current.Children[word]; ok {
			current = child
		} else {
			// No match, no suggestions
			m.suggestions = nil
			return
		}
	}

	// If input ends with space, show all children
	if strings.HasSuffix(input, " ") || input == "" {
		partial = ""
	}

	// Generate suggestions from current level
	m.suggestions = nil
	if current.Children != nil {
		// Collect and sort keys
		keys := make([]string, 0, len(current.Children))
		for k := range current.Children {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			child := current.Children[name]
			if partial == "" || strings.HasPrefix(name, partial) {
				m.suggestions = append(m.suggestions, suggestion{
					text:        name,
					description: child.Description,
				})
			}
		}
	}
}

func (m model) executeCommand(command string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.SendCommand(command)
		if err != nil {
			return executeResultMsg{err: err}
		}

		// Format response
		var output strings.Builder
		if resp.Error != "" {
			output.WriteString(errorStyle.Render("Error: " + resp.Error))
		} else {
			if len(resp.Result) > 0 && string(resp.Result) != "null" {
				var data any
				if err := json.Unmarshal(resp.Result, &data); err == nil {
					formatted, _ := json.MarshalIndent(data, "", "  ")
					output.WriteString(string(formatted))
				} else {
					output.WriteString(string(resp.Result))
				}
			} else {
				output.WriteString(successStyle.Render("OK"))
			}
		}

		return executeResultMsg{output: output.String()}
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(dimStyle.Render("Ze CLI") + " " + dimStyle.Render("(Tab: complete, Enter: execute, Ctrl+C: quit)"))
	b.WriteString("\n\n")

	// Prompt and input
	b.WriteString(promptStyle.Render("❯ "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	// Suggestions
	if len(m.suggestions) > 0 {
		b.WriteString("\n")
		for i, s := range m.suggestions {
			if i == m.selected {
				b.WriteString(selectedStyle.Render("▸ "+s.text) + " ")
				b.WriteString(dimStyle.Render(s.description))
			} else {
				b.WriteString(suggestionStyle.Render("  "+s.text) + " ")
				b.WriteString(dimStyle.Render(s.description))
			}
			b.WriteString("\n")
		}
	}

	// Output
	if m.output != "" {
		b.WriteString("\n")
		b.WriteString(m.output)
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n")
	}

	return b.String()
}
