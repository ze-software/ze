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
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"               // init() registers RIB proxy RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/subscribe"         // init() registers subscribe/unsubscribe RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"            // init() registers update parsing RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler" // init() registers route-refresh command RPCs
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

Pipe operators (interactive mode only, Tab completes after |):
  <command> | match <pattern>    Filter lines matching pattern
  <command> | count              Count output lines
  <command> | table              Render as nushell-style table
  <command> | json               Pretty-print JSON (default)
  <command> | json compact       Single-line JSON
  <command> | no-more            Disable paging

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
	format := fs.String("format", "yaml", "Output format: yaml, json, table")

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
		return client.Execute(*runCmd, *format)
	}

	// Create initial model
	ti := textinput.New()
	ti.Placeholder = "type command or press Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	vp := viewport.New(80, 20)

	m := model{
		textInput:  ti,
		viewport:   vp,
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

// Execute sends a command and prints the response in the given format.
// Valid formats: "yaml" (default), "json", "table".
func (c *cliClient) Execute(command, format string) int {
	resp, err := c.SendCommand(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	c.printFormatted(resp, format)

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

	// Build JSON RPC request.
	// Extract peer selector from args: if the first arg looks like an IP/glob,
	// it's a peer selector that the server expects in params.selector.
	req := rpc.Request{Method: method}
	var selector string
	if len(args) > 0 && looksLikePeerSelector(args[0]) {
		selector = args[0]
		args = args[1:]
	}
	if len(args) > 0 || selector != "" {
		params := struct {
			Selector string   `json:"selector,omitempty"`
			Args     []string `json:"args,omitempty"`
		}{Selector: selector, Args: args}
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

// printFormatted formats and prints a response in the given format.
func (c *cliClient) printFormatted(resp *rpcResponse, format string) {
	if resp.Error != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		return
	}

	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		fmt.Println("OK")
		return
	}

	switch format {
	case "json":
		fmt.Println(command.ApplyJSON(string(resp.Result), "pretty"))
	case "table":
		fmt.Print(command.ApplyTable(string(resp.Result)))
	default: // yaml
		var data any
		if err := json.Unmarshal(resp.Result, &data); err != nil {
			fmt.Println(string(resp.Result))
			return
		}
		fmt.Print(command.RenderYAML(data))
	}
}

// looksLikePeerSelector returns true if the string looks like an IP address or glob.
func looksLikePeerSelector(s string) bool {
	if s == "*" {
		return true
	}
	return strings.ContainsAny(s, ".:")
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
	rpcs := AllCLIRPCs()
	infos := make([]command.RPCInfo, len(rpcs))
	for i, reg := range rpcs {
		infos[i] = command.RPCInfo{
			CLICommand: reg.CLICommand,
			Help:       reg.Help,
			ReadOnly:   reg.ReadOnly,
		}
	}
	return command.BuildTree(infos, readOnly)
}

// Command is an alias for command.Node. Use command.Node directly in new code.
type Command = command.Node

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
	viewport    viewport.Model
	suggestions []suggestion
	selected    int
	client      *cliClient
	quitting    bool
	width       int
	height      int
	history     []string // Previous commands (oldest first)
	historyIdx  int      // Current position in history (-1 = not browsing)
	historyTmp  string   // Saved current input when browsing history
	outputBuf   string   // Accumulated command output history
	tabBase     string   // Input before Tab completion cycle started
	tabActive   bool     // Whether we're cycling through completions
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

		case tea.KeyShiftUp:
			m.viewport.ScrollUp(1)
			return m, nil

		case tea.KeyShiftDown:
			m.viewport.ScrollDown(1)
			return m, nil

		case tea.KeyPgUp:
			m.viewport.PageUp()
			return m, nil

		case tea.KeyPgDown:
			m.viewport.PageDown()
			return m, nil

		case tea.KeyTab:
			// Cycle through suggestions
			if len(m.suggestions) > 0 {
				if !m.tabActive {
					m.tabBase = m.textInput.Value()
					m.tabActive = true
					m.selected = 0
				} else {
					// Single suggestion: already applied, nothing to cycle
					if len(m.suggestions) == 1 {
						return m, nil
					}
					m.selected = (m.selected + 1) % len(m.suggestions)
				}
				m.applySelectedSuggestion()
			}
			return m, nil

		case tea.KeyShiftTab:
			// Cycle backwards through suggestions
			if len(m.suggestions) > 0 {
				if !m.tabActive {
					m.tabBase = m.textInput.Value()
					m.tabActive = true
					m.selected = len(m.suggestions) - 1
				} else {
					if len(m.suggestions) == 1 {
						return m, nil
					}
					m.selected--
					if m.selected < 0 {
						m.selected = len(m.suggestions) - 1
					}
				}
				m.applySelectedSuggestion()
			}
			return m, nil

		case tea.KeyUp:
			m.tabActive = false
			m = m.handleHistoryUp()
			return m, nil

		case tea.KeyDown:
			m.tabActive = false
			m = m.handleHistoryDown()
			return m, nil

		case tea.KeyEnter:
			m.tabActive = false
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

			// Echo command to output history
			m.outputBuf += dimStyle.Render("❯") + " " + input + "\n"
			m.viewport.SetContent(m.outputBuf)
			m.viewport.GotoBottom()

			// Execute command
			m.textInput.SetValue("")
			m.suggestions = nil
			m.selected = 0
			m.resizeViewport()
			return m, m.executeCommand(input)

		default: // handle all other keys via textinput
			m.tabActive = false
			m.textInput, cmd = m.textInput.Update(msg)
			m.updateSuggestions()
			m.selected = 0
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = min(msg.Width-4, 80)
		m.resizeViewport()
		return m, nil

	case executeResultMsg:
		if msg.err != nil {
			m.outputBuf += errorStyle.Render("Error: "+msg.err.Error()) + "\n\n"
		} else if msg.output != "" {
			m.outputBuf += msg.output + "\n"
		}
		m.viewport.SetContent(m.outputBuf)
		m.viewport.GotoBottom()
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
	if m.selected < 0 || m.selected >= len(m.suggestions) {
		return
	}

	// When cycling (Tab pressed multiple times), use the original input
	// before Tab was first pressed — prevents appending suggestions repeatedly.
	input := m.tabBase
	if !m.tabActive {
		input = m.textInput.Value()
	}
	suggested := m.suggestions[m.selected].text

	// After a pipe: replace partial pipe operator name.
	if pipeIdx := strings.LastIndex(input, "|"); pipeIdx >= 0 {
		prefix := input[:pipeIdx+1] + " "
		m.textInput.SetValue(prefix + suggested + " ")
		m.textInput.CursorEnd()
		return
	}

	// Command completion: replace last partial word.
	words := strings.Fields(input)
	if len(words) > 0 {
		if strings.HasSuffix(input, " ") || input == "" {
			m.textInput.SetValue(input + suggested + " ")
		} else {
			words[len(words)-1] = suggested
			m.textInput.SetValue(strings.Join(words, " ") + " ")
		}
	} else {
		m.textInput.SetValue(suggested + " ")
	}
	m.textInput.CursorEnd()
}

// ribRoutesPipeSuggestions lists server-side pipeline keywords for rib routes/best commands.
var ribRoutesPipeSuggestions = []suggestion{
	{text: "path", description: "Filter by AS path (contiguous subsequence, ^ = anchor)"},
	{text: "cidr", description: "Filter by prefix match"},
	{text: "community", description: "Filter by community value"},
	{text: "family", description: "Filter by address family (e.g. ipv4/unicast)"},
	{text: "match", description: "Filter by field value substring"},
	{text: "count", description: "Count matching routes (metadata only)"},
	{text: "json", description: "Output as JSON"},
	{text: "yaml", description: "YAML output"},
	{text: "no-more", description: "Disable paging"},
	{text: "table", description: "Render as table"},
}

func (m *model) updateSuggestions() {
	defer m.resizeViewport()

	input := m.textInput.Value()

	// After a pipe character, suggest pipe operators instead of commands.
	// For rib routes/best commands, suggest server-side pipeline keywords.
	if pipeIdx := strings.LastIndex(input, "|"); pipeIdx >= 0 {
		cmdPart := strings.TrimSpace(input[:pipeIdx])
		after := strings.TrimSpace(input[pipeIdx+1:])
		m.suggestions = nil
		lower := strings.ToLower(cmdPart)
		if strings.HasPrefix(lower, "rib routes") || strings.HasPrefix(lower, "rib best") {
			for _, s := range ribRoutesPipeSuggestions {
				if after == "" || strings.HasPrefix(s.text, after) {
					m.suggestions = append(m.suggestions, s)
				}
			}
		} else {
			// Use shared pipe completion (handles sub-args like json compact/pretty).
			for _, s := range command.CompletePipe(input[pipeIdx+1:]) {
				m.suggestions = append(m.suggestions, suggestion{text: s.Text, description: s.Description})
			}
		}
		return
	}

	words := strings.Fields(input)

	// Navigate command tree.
	current := commandTree
	var partial string

	for i, word := range words {
		if current.Children == nil {
			m.suggestions = nil
			return
		}

		// Check if this is the last word (partial match).
		if i == len(words)-1 && !strings.HasSuffix(input, " ") {
			partial = word
			break
		}

		// Find exact match to navigate deeper.
		if child, ok := current.Children[word]; ok {
			current = child
		} else {
			// No match, no suggestions.
			m.suggestions = nil
			return
		}
	}

	// If input ends with space, show all children.
	if strings.HasSuffix(input, " ") || input == "" {
		partial = ""
	}

	// Generate suggestions from current level.
	m.suggestions = nil
	if current.Children != nil {
		// Collect and sort keys.
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

func (m model) executeCommand(input string) tea.Cmd {
	return func() tea.Msg {
		// Split pipe operators and fold server-side keywords.
		// Default to table format when no explicit format pipe is specified.
		cmd, format := command.ProcessPipesDefaultTable(input)

		resp, err := m.client.SendCommand(cmd)
		if err != nil {
			return executeResultMsg{err: err}
		}

		if resp.Error != "" {
			return executeResultMsg{output: errorStyle.Render("Error: " + resp.Error)}
		}

		if len(resp.Result) == 0 || string(resp.Result) == "null" {
			return executeResultMsg{output: successStyle.Render("OK")}
		}

		return executeResultMsg{output: format(string(resp.Result))}
	}
}

// resizeViewport adjusts viewport height based on terminal size and suggestion count.
func (m *model) resizeViewport() {
	if m.height == 0 {
		return
	}
	// Layout: 1 header + viewport + suggestions + 1 input = total height
	vpHeight := max(m.height-len(m.suggestions)-2, 3)
	m.viewport.Height = vpHeight
	m.viewport.Width = m.width
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(dimStyle.Render("Ze CLI") + " " + dimStyle.Render("(Shift+↑↓: scroll, Tab: complete, | pipe, Ctrl+C: quit)"))
	b.WriteString("\n")

	// Viewport (scrollable output history)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Suggestions (tabulated with aligned columns)
	if len(m.suggestions) > 0 {
		maxWidth := 0
		for _, s := range m.suggestions {
			if l := len(s.text); l > maxWidth {
				maxWidth = l
			}
		}
		for i, s := range m.suggestions {
			padded := fmt.Sprintf("%-*s", maxWidth, s.text)
			if i == m.selected {
				b.WriteString(selectedStyle.Render("▸ " + padded))
			} else {
				b.WriteString(suggestionStyle.Render("  " + padded))
			}
			b.WriteString("  ")
			b.WriteString(dimStyle.Render(s.description))
			b.WriteString("\n")
		}
	}

	// Input prompt at bottom
	b.WriteString(promptStyle.Render("❯ "))
	b.WriteString(m.textInput.View())

	return b.String()
}
