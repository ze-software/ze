// ze-subsystem is a forked process that handles a subset of commands.
// It communicates with ze engine via stdin/stdout pipes using the
// same 5-stage protocol as external plugins.
//
// Bidirectional communication:
//   - Engine → Subsystem: #alpha command (alpha serial a-j)
//   - Subsystem → Engine: #N command (numeric serial)
//   - Responses use @serial format
//
// Usage:
//
//	ze-subsystem --mode=cache
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// Protocol stage markers.
const (
	markerConfigDone   = "config done"
	markerRegistryDone = "registry done"
)

// Global state for bidirectional communication.
var (
	nextSerial      atomic.Uint64
	pendingRequests = make(map[string]chan string)
	pendingMu       sync.Mutex
	stdinScanner    *bufio.Scanner
	scannerMu       sync.Mutex
)

func main() {
	mode := flag.String("mode", "", "Subsystem mode: cache|route|session")
	flag.Parse()

	if *mode == "" {
		fmt.Fprintln(os.Stderr, "ze-subsystem: --mode is required")
		os.Exit(1)
	}

	stdinScanner = bufio.NewScanner(os.Stdin)

	var err error
	switch *mode {
	case "cache":
		err = runCacheSubsystem()
	case "route":
		err = runRouteSubsystem()
	case "session":
		err = runSessionSubsystem()
	default:
		fmt.Fprintf(os.Stderr, "ze-subsystem: unknown mode: %s\n", *mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ze-subsystem: %v\n", err)
		os.Exit(1)
	}
}

// callEngine sends a command to the engine and waits for response.
// Uses numeric serial to differentiate from engine-initiated (alpha) requests.
func callEngine(command string) (string, error) {
	serial := nextSerial.Add(1)
	serialStr := fmt.Sprintf("%d", serial)

	// Create response channel
	respCh := make(chan string, 1)

	pendingMu.Lock()
	pendingRequests[serialStr] = respCh
	pendingMu.Unlock()

	defer func() {
		pendingMu.Lock()
		delete(pendingRequests, serialStr)
		pendingMu.Unlock()
	}()

	// Send command to engine (stdout)
	fmt.Printf("#%s %s\n", serialStr, command)

	// Wait for response (will be delivered by readLine loop)
	// The response comes as JSON: {"answer":{"serial":"N","status":"done",...}}
	resp := <-respCh
	return resp, nil
}

// mainLoop reads commands and dispatches to handler.
// Also handles responses to our engine callbacks.
func mainLoop(handler commandHandler) error {
	for readLine() {
		line := currentLine()

		// Check for shutdown
		if strings.Contains(line, `"shutdown"`) || strings.Contains(line, `"answer": "shutdown"`) {
			return nil
		}

		// Check for JSON response to our engine callback
		if strings.HasPrefix(line, "{") {
			handleEngineResponse(line)
			continue
		}

		// Parse #serial command format (engine-initiated request)
		serial, command, args := parseCommand(line)
		if serial == "" {
			continue
		}

		response := handler(serial, command, args)
		fmt.Println(response)
	}

	return scannerError()
}

// handleEngineResponse parses JSON response and delivers to waiting callback.
func handleEngineResponse(line string) {
	// Parse: {"answer":{"serial":"N","status":"done","data":...}}
	var wrapper struct {
		Answer struct {
			Serial string `json:"serial"`
			Status string `json:"status"`
			Data   any    `json:"data"`
		} `json:"answer"`
	}

	if err := json.Unmarshal([]byte(line), &wrapper); err != nil {
		return
	}

	serial := wrapper.Answer.Serial
	if serial == "" {
		return
	}

	pendingMu.Lock()
	ch, found := pendingRequests[serial]
	pendingMu.Unlock()

	if !found {
		return
	}

	// Build response string
	var resp string
	if wrapper.Answer.Data != nil {
		data, _ := json.Marshal(wrapper.Answer.Data)
		resp = string(data)
	}

	select {
	case ch <- resp:
	default:
	}
}

// Scanner helpers for thread-safe reading.
var currentLineValue string

func readLine() bool {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	if stdinScanner.Scan() {
		currentLineValue = stdinScanner.Text()
		return true
	}
	return false
}

func currentLine() string {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	return currentLineValue
}

func scannerError() error {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	return stdinScanner.Err()
}

// runCacheSubsystem implements the cache subsystem protocol.
func runCacheSubsystem() error {
	fmt.Println("declare encoding text")
	fmt.Println("declare cmd bgp cache list")
	fmt.Println("declare cmd bgp cache retain")
	fmt.Println("declare cmd bgp cache release")
	fmt.Println("declare cmd bgp cache expire")
	fmt.Println("declare cmd bgp cache forward")
	fmt.Println("declare done")

	for readLine() {
		if currentLine() == markerConfigDone {
			break
		}
	}
	fmt.Println("capability done")
	for readLine() {
		if currentLine() == markerRegistryDone {
			break
		}
	}
	fmt.Println("ready")

	return mainLoop(handleCacheCommand)
}

// runRouteSubsystem implements the route subsystem protocol.
func runRouteSubsystem() error {
	fmt.Println("declare encoding text")
	fmt.Println("declare cmd bgp route announce")
	fmt.Println("declare cmd bgp route withdraw")
	fmt.Println("declare done")

	for readLine() {
		if currentLine() == markerConfigDone {
			break
		}
	}
	fmt.Println("capability done")
	for readLine() {
		if currentLine() == markerRegistryDone {
			break
		}
	}
	fmt.Println("ready")

	return mainLoop(handleRouteCommand)
}

// runSessionSubsystem implements the session subsystem protocol.
func runSessionSubsystem() error {
	fmt.Println("declare encoding text")
	fmt.Println("declare cmd bgp session ping")
	fmt.Println("declare cmd bgp session bye")
	fmt.Println("declare cmd bgp session ready")
	fmt.Println("declare done")

	for readLine() {
		if currentLine() == markerConfigDone {
			break
		}
	}
	fmt.Println("capability done")
	for readLine() {
		if currentLine() == markerRegistryDone {
			break
		}
	}
	fmt.Println("ready")

	return mainLoop(handleSessionCommand)
}

// commandHandler is a function that handles a command and returns a response.
type commandHandler func(serial, command string, args []string) string

// parseCommand extracts serial, command, and args from "#serial command args...".
func parseCommand(line string) (serial string, command string, args []string) {
	if !strings.HasPrefix(line, "#") {
		return "", "", nil
	}

	idx := strings.Index(line, " ")
	if idx <= 1 {
		return "", "", nil
	}

	serial = line[1:idx]
	rest := strings.TrimSpace(line[idx+1:])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return serial, "", nil
	}

	return serial, rest, parts
}

// handleCacheCommand handles cache subsystem commands.
func handleCacheCommand(serial, command string, _ []string) string {
	switch {
	case strings.HasPrefix(command, "bgp cache list"):
		// Call back to engine for actual cache list
		resp, err := callEngine("bgp cache list")
		if err != nil {
			return fmt.Sprintf("@%s error %v", serial, err)
		}
		return fmt.Sprintf("@%s ok %s", serial, resp)

	case strings.HasPrefix(command, "bgp cache retain"):
		return fmt.Sprintf("@%s ok", serial)

	case strings.HasPrefix(command, "bgp cache release"):
		return fmt.Sprintf("@%s ok", serial)

	case strings.HasPrefix(command, "bgp cache expire"):
		return fmt.Sprintf("@%s ok", serial)

	case strings.HasPrefix(command, "bgp cache forward"):
		return fmt.Sprintf("@%s ok", serial)

	default:
		return fmt.Sprintf("@%s error unknown command: %s", serial, command)
	}
}

// handleRouteCommand handles route subsystem commands.
func handleRouteCommand(serial, command string, _ []string) string {
	switch {
	case strings.HasPrefix(command, "bgp route announce"):
		return fmt.Sprintf("@%s ok", serial)

	case strings.HasPrefix(command, "bgp route withdraw"):
		return fmt.Sprintf("@%s ok", serial)

	default:
		return fmt.Sprintf("@%s error unknown command: %s", serial, command)
	}
}

// handleSessionCommand handles session subsystem commands.
func handleSessionCommand(serial, command string, _ []string) string {
	switch {
	case strings.HasPrefix(command, "bgp session ping"):
		data, _ := json.Marshal(map[string]any{"pong": os.Getpid()})
		return fmt.Sprintf("@%s ok %s", serial, data)

	case strings.HasPrefix(command, "bgp session bye"):
		data, _ := json.Marshal(map[string]any{"status": "goodbye"})
		return fmt.Sprintf("@%s ok %s", serial, data)

	case strings.HasPrefix(command, "bgp session ready"):
		data, _ := json.Marshal(map[string]any{"api": "ready acknowledged"})
		return fmt.Sprintf("@%s ok %s", serial, data)

	default:
		return fmt.Sprintf("@%s error unknown command: %s", serial, command)
	}
}
