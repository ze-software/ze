// Package cli provides the ze cli subcommand.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultSocketPath = "/var/run/ze-bgp.sock"
const statusError = "error"

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
  --socket <path>    Path to API socket (default: /var/run/ze-bgp.sock)
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
	socketPath := fs.String("socket", defaultSocketPath, "Path to API socket")
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
		textInput: ti,
		client:    client,
	}

	// Run the bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

// cliResponse represents an API response.
type cliResponse struct {
	Status string         `json:"status"`
	Error  string         `json:"error,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

// cliClient handles communication with the API server.
type cliClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

func newCLIClient(socketPath string) (*cliClient, error) {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &cliClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

func (c *cliClient) Close() error {
	return c.conn.Close()
}

// Execute sends a command and prints the response.
func (c *cliClient) Execute(command string) int {
	resp, err := c.SendCommand(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	c.PrintResponse(resp)

	if resp.Status == "error" {
		return 1
	}
	return 0
}

// SendCommand sends a command and returns the response.
func (c *cliClient) SendCommand(command string) (*cliResponse, error) {
	// Send command
	_, err := fmt.Fprintf(c.conn, "%s\n", command)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Read response
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}

	var resp cliResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return &resp, nil
}

// PrintResponse formats and prints a response.
func (c *cliClient) PrintResponse(resp *cliResponse) {
	if resp.Status == "error" {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		return
	}

	if resp.Data == nil {
		fmt.Println("OK")
		return
	}

	// Pretty print data
	c.printData(resp.Data, "")
}

func (c *cliClient) printData(data map[string]any, indent string) {
	for key, value := range data {
		switch v := value.(type) {
		case []any:
			if len(v) == 0 {
				fmt.Printf("%s%s: (none)\n", indent, key)
			} else {
				fmt.Printf("%s%s:\n", indent, key)
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						c.printItem(m, indent+"  ")
					} else if s, ok := item.(string); ok {
						fmt.Printf("%s  - %s\n", indent, s)
					} else {
						fmt.Printf("%s  - %v\n", indent, item)
					}
				}
			}
		case map[string]any:
			fmt.Printf("%s%s:\n", indent, key)
			c.printData(v, indent+"  ")
		default: // print unknown types with generic format
			fmt.Printf("%s%s: %v\n", indent, key, value)
		}
	}
}

func (c *cliClient) printItem(m map[string]any, indent string) {
	// For peer info, format nicely
	if addr, ok := m["Address"]; ok {
		state := m["State"]
		fmt.Printf("%s%v [%v]\n", indent, addr, state)
		return
	}

	// Generic map
	for k, v := range m {
		fmt.Printf("%s%s: %v\n", indent, k, v)
	}
}

// Command represents a CLI command with metadata.
type Command struct {
	Name        string
	Description string
	Children    map[string]*Command
}

// commandTree holds all available commands for completion.
var commandTree = buildCommandTree()

func buildCommandTree() *Command {
	return &Command{
		Name: "",
		Children: map[string]*Command{
			"daemon": {
				Name:        "daemon",
				Description: "Daemon control commands",
				Children: map[string]*Command{
					"shutdown": {Name: "shutdown", Description: "Gracefully shutdown the daemon"},
					"status":   {Name: "status", Description: "Show daemon status"},
				},
			},
			"peer": {
				Name:        "peer",
				Description: "Peer operations",
				Children: map[string]*Command{
					"list": {Name: "list", Description: "List all peers (brief)"},
					"show": {Name: "show", Description: "Show peer details [<ip>]"},
				},
			},
			"rib": {
				Name:        "rib",
				Description: "RIB operations",
				Children: map[string]*Command{
					"show": {
						Name:        "show",
						Description: "Show RIB contents",
						Children: map[string]*Command{
							"in":  {Name: "in", Description: "Show Adj-RIB-In"},
							"out": {Name: "out", Description: "Show Adj-RIB-Out"},
						},
					},
				},
			},
			"system": {
				Name:        "system",
				Description: "System commands",
				Children: map[string]*Command{
					"help":    {Name: "help", Description: "Show available commands"},
					"version": {Name: "version", Description: "Show version"},
				},
			},
		},
	}
}

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

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			if input == "quit" || input == "exit" {
				m.quitting = true
				return m, tea.Quit
			}

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
		if resp.Status == statusError {
			output.WriteString(errorStyle.Render("Error: " + resp.Error))
		} else {
			if resp.Data != nil {
				formatted, _ := json.MarshalIndent(resp.Data, "", "  ")
				output.WriteString(string(formatted))
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
