package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

const defaultSocketPath = "/var/run/zebgp.sock"

// cliResponse represents an API response.
type cliResponse struct {
	Status string         `json:"status"`
	Error  string         `json:"error,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	socketPath := fs.String("socket", defaultSocketPath, "Path to API socket")
	interactive := fs.Bool("i", false, "Interactive mode")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	client, err := newCLIClient(*socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot connect to %s: %v\n", *socketPath, err)
		return 1
	}
	defer func() { _ = client.Close() }()

	// If command provided on command line, execute it
	if fs.NArg() > 0 {
		command := strings.Join(fs.Args(), " ")
		return client.Execute(command)
	}

	// Interactive mode
	if *interactive || isTerminal() {
		return client.Interactive()
	}

	// Read commands from stdin
	return client.ReadStdin()
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
		default:
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

// Interactive runs an interactive command loop.
func (c *cliClient) Interactive() int {
	fmt.Println("ZeBGP CLI - type 'system help' for commands, 'quit' to exit")

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			fmt.Print("> ")
			continue
		}

		if line == "quit" || line == "exit" {
			break
		}

		resp, err := c.SendCommand(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			c.PrintResponse(resp)
		}

		fmt.Print("> ")
	}

	fmt.Println()
	return 0
}

// ReadStdin reads commands from stdin (non-interactive).
func (c *cliClient) ReadStdin() int {
	scanner := bufio.NewScanner(os.Stdin)
	exitCode := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if code := c.Execute(line); code != 0 {
			exitCode = code
		}
	}

	return exitCode
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
