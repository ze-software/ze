// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch
// Related: decode_open.go — OPEN message decoding uses plugin invocation
// Related: decode_mp.go — NLRI parsing uses plugin invocation

package bgp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// PluginMode represents how a plugin should be invoked.
type PluginMode int

const (
	// ModeFork spawns a subprocess via exec.
	ModeFork PluginMode = iota
	// ModeInternal runs the plugin in a goroutine with pipes.
	ModeInternal
	// ModeDirect calls the decode function synchronously.
	ModeDirect
)

// parsePluginName extracts the plugin name, invocation mode, and optional path from input.
//
// Syntax:
//   - "name" → (name, ModeFork, "") - subprocess
//   - "ze.name" → (name, ModeInternal, "") - goroutine + pipe
//   - "ze-name" → (name, ModeDirect, "", nil) - sync in-process
//   - "/path/to/prog" → ("", ModeFork, path, nil) - execute path directly
//   - "/path/to/prog --arg" → ("", ModeFork, path, ["--arg"]) - path with args
func parsePluginName(input string) (name string, mode PluginMode, path string, args []string) {
	if after, ok := strings.CutPrefix(input, "ze."); ok {
		return after, ModeInternal, "", nil
	}
	if after, ok := strings.CutPrefix(input, "ze-"); ok {
		return after, ModeDirect, "", nil
	}
	if strings.Contains(input, "/") {
		// Path mode: split into binary path and arguments.
		parts := strings.Fields(input)
		if len(parts) == 1 {
			return "", ModeFork, input, nil
		}
		return "", ModeFork, parts[0], parts[1:]
	}
	// Plain name: fork subprocess.
	return input, ModeFork, "", nil
}

// invokePluginDecodeRequest spawns a plugin in decode mode and sends a decode request.
// Returns decoded JSON map or nil if decoding failed.
// The request format varies: "decode capability <code> <hex>" or "decode nlri <family> <hex>".
func invokePluginDecodeRequest(pluginName, request string) map[string]any {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil
	}

	// Build plugin command - pluginName comes from fixed maps (pluginCapabilityMap, pluginFamilyMap)
	args := []string{"plugin", pluginName, "--decode"}

	// Create command with timeout context for subprocess decode operation.
	// 5s allows for process startup chain (sh -> wrapper -> ze plugin --decode).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // pluginName from fixed map

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Debug("plugin stdin pipe failed", "plugin", pluginName, "err", err)
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Debug("plugin stdout pipe failed", "plugin", pluginName, "err", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Debug("plugin start failed", "plugin", pluginName, "err", err)
		return nil
	}

	// Send decode request
	if _, err := stdin.Write([]byte(request + "\n")); err != nil {
		slog.Debug("plugin write failed", "plugin", pluginName, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin stdin close failed", "plugin", pluginName, "err", err)
	}

	// Read response
	scanner := bufio.NewScanner(stdout)
	var result map[string]any
	if scanner.Scan() {
		line := scanner.Text()
		// Parse: "decoded json <json>" or "decoded unknown"
		if after, ok := strings.CutPrefix(line, "decoded json "); ok {
			jsonStr := after
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				_ = cmd.Wait()
				return result
			}
		}
	}

	_ = cmd.Wait()
	return nil
}

// invokePluginDecode decodes a capability via plugin.
// Tries subprocess first, falls back to in-process (matching NLRI pattern).
func invokePluginDecode(pluginName string, code uint8, hexData string) map[string]any {
	request := fmt.Sprintf("decode capability %d %s", code, hexData)

	// Try subprocess first.
	result := invokePluginDecodeRequest(pluginName, request)
	if result != nil {
		return result
	}

	// Fallback: in-process decode (for tests or when subprocess unavailable).
	if inResult := invokePluginInProcess(pluginName, request); inResult != nil {
		if mapResult, ok := inResult.(map[string]any); ok {
			return mapResult
		}
	}

	return nil
}

// invokePluginNLRIDecode invokes a plugin to decode NLRI.
// Routes by mode based on plugin name syntax:
//   - "name" → Fork subprocess (with in-process retry)
//   - "ze.name" → Internal (goroutine + pipe)
//   - "ze-name" → Direct (sync in-process)
//   - "/path" → Fork external binary
func invokePluginNLRIDecode(pluginName, family, hexData string) any {
	request := fmt.Sprintf("decode nlri %s %s", family, hexData)
	name, mode, path, args := parsePluginName(pluginName)

	switch mode {
	case ModeInternal:
		return invokePluginInternal(name, request)
	case ModeDirect:
		return invokePluginInProcess(name, request)
	case ModeFork:
		if path != "" {
			return invokePluginPath(path, args, request)
		}
		return invokePluginNLRIDecodeRequest(name, request)
	}
	return nil
}

// invokePluginNLRIDecodeRequest spawns a built-in plugin subprocess.
// For plain names (ModeFork without path), retries with in-process on failure.
func invokePluginNLRIDecodeRequest(pluginName, request string) any {
	// Try subprocess first.
	result := invokePluginSubprocess(pluginName, request)
	if result != nil {
		return result
	}

	// Retry: in-process decode (for tests where subprocess can't run).
	return invokePluginInProcess(pluginName, request)
}

// invokePluginPath executes an external plugin binary at the given path.
// User-provided args are passed before the mandatory --decode flag.
func invokePluginPath(path string, userArgs []string, request string) any {
	// 30s allows for process startup under the broad parallel race-test gate.
	// Normal decode returns immediately; the larger budget avoids false negatives
	// when the host is saturated by many concurrently starting test binaries.
	// Longer than invokePluginSubprocess (5s) because the external path may involve
	// an extra shell layer and a separately-built binary.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Build args: user args + --decode
	cmdArgs := append(userArgs, "--decode")           //nolint:gocritic // intentional append to new slice
	cmd := exec.CommandContext(ctx, path, cmdArgs...) //nolint:gosec // path from user input

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Debug("plugin path stdin failed", "path", path, "err", err)
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Debug("plugin path stdout failed", "path", path, "err", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Debug("plugin path start failed", "path", path, "err", err)
		return nil
	}

	if _, err := stdin.Write([]byte(request + "\n")); err != nil {
		slog.Debug("plugin path write failed", "path", path, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin path stdin close failed", "path", path, "err", err)
	}

	// Loop stdout lines — skip unexpected output (shell warnings, runtime messages)
	// until we find the "decoded json ..." response or EOF.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if result := parseDecodedJSON(line); result != nil {
			if err := cmd.Wait(); err != nil {
				slog.Debug("plugin path wait failed", "path", path, "err", err)
			}
			return result
		}
	}

	if err := cmd.Wait(); err != nil {
		slog.Debug("plugin path wait failed", "path", path, "err", err)
	}
	slog.Debug("plugin path produced no decoded output", "path", path)
	return nil
}

// parseDecodedJSON extracts JSON from "decoded json <json>" response.
func parseDecodedJSON(line string) any {
	if !strings.HasPrefix(line, "decoded json ") {
		return nil
	}
	jsonStr := strings.TrimPrefix(line, "decoded json ")
	var arrResult []any
	if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
		return arrResult
	}
	var mapResult map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &mapResult); err == nil {
		return mapResult
	}
	return nil
}

// invokePluginSubprocess spawns a plugin subprocess for NLRI decode.
func invokePluginSubprocess(pluginName, request string) any {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil // Fall back to in-process via invokePluginNLRIDecodeRequest
	}

	args := []string{"plugin", pluginName, "--decode"}

	// 5s allows for process startup chain (sh -> wrapper -> ze plugin --decode).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // pluginName from fixed map

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Debug("plugin stdin pipe failed", "plugin", pluginName, "err", err)
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Debug("plugin stdout pipe failed", "plugin", pluginName, "err", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Debug("plugin start failed", "plugin", pluginName, "err", err)
		return nil
	}

	if _, err := stdin.Write([]byte(request + "\n")); err != nil {
		slog.Debug("plugin write failed", "plugin", pluginName, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin stdin close failed", "plugin", pluginName, "err", err)
	}

	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "decoded json "); ok {
			jsonStr := after
			// Try array first (EVPN returns array), then map (FlowSpec returns map)
			var arrResult []any
			if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
				_ = cmd.Wait()
				return arrResult
			}
			var mapResult map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &mapResult); err == nil {
				_ = cmd.Wait()
				return mapResult
			}
			slog.Debug("plugin json parse failed", "plugin", pluginName, "json", jsonStr)
		}
	}

	_ = cmd.Wait()
	return nil
}

// inProcessDecoders maps plugin names to their in-process decode functions.
// Populated from plugin registry at init time.
var inProcessDecoders = registry.InProcessDecoders()

// invokePluginInProcess runs plugin decode in-process (Direct mode: ze-name).
// Synchronous, blocking - fastest invocation for CLI decode.
func invokePluginInProcess(pluginName, request string) any {
	decoder, ok := inProcessDecoders[pluginName]
	if !ok {
		return nil
	}

	input := bytes.NewBufferString(request + "\n")
	output := &bytes.Buffer{}

	decoder(input, output)

	return parsePluginResponse(output.String())
}

// invokePluginInternal runs decode via goroutine + pipes (Internal mode: ze.name).
// Uses decode-only runners (same as Direct) but with async pipe I/O.
func invokePluginInternal(pluginName, request string) any {
	decoder, ok := inProcessDecoders[pluginName]
	if !ok {
		slog.Debug("internal plugin decoder not available", "plugin", pluginName)
		return nil
	}

	// Create pipes for communication.
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	// Run decoder in goroutine.
	done := make(chan int, 1)
	go func() {
		// Wrap pipes in buffers for decoder interface.
		inBuf := &bytes.Buffer{}
		if _, err := io.Copy(inBuf, inR); err != nil {
			slog.Debug("internal plugin read input failed", "plugin", pluginName, "err", err)
		}
		outBuf := &bytes.Buffer{}
		exitCode := decoder(inBuf, outBuf)
		if _, err := outW.Write(outBuf.Bytes()); err != nil {
			slog.Debug("internal plugin write output failed", "plugin", pluginName, "err", err)
		}
		if err := outW.Close(); err != nil {
			slog.Debug("internal plugin outW close failed", "plugin", pluginName, "err", err)
		}
		done <- exitCode
	}()

	// Send request and close input.
	if _, err := inW.Write([]byte(request + "\n")); err != nil {
		slog.Debug("internal plugin write failed", "plugin", pluginName, "err", err)
		if err := inW.Close(); err != nil {
			slog.Debug("internal plugin inW close failed", "plugin", pluginName, "err", err)
		}
		return nil
	}
	if err := inW.Close(); err != nil {
		slog.Debug("internal plugin inW close failed", "plugin", pluginName, "err", err)
	}

	// Read response.
	var output bytes.Buffer
	if _, err := io.Copy(&output, outR); err != nil {
		slog.Debug("internal plugin read failed", "plugin", pluginName, "err", err)
	}

	<-done // Wait for decoder to finish.

	return parsePluginResponse(output.String())
}

// parsePluginResponse parses the "decoded json ..." response from a plugin.
func parsePluginResponse(output string) any {
	line := strings.TrimSpace(output)
	if after, ok := strings.CutPrefix(line, "decoded json "); ok {
		jsonStr := after
		var arrResult []any
		if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
			return arrResult
		}
		var mapResult map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &mapResult); err == nil {
			return mapResult
		}
	}
	return nil
}

// validateDecodeFamily validates a family string has valid AFI/SAFI format.
// Families are registered dynamically by plugins, so this validates format only:
// must be non-empty and contain "afi/safi" structure.
func validateDecodeFamily(family string) error {
	if family == "" {
		return fmt.Errorf("empty address family")
	}
	parts := strings.SplitN(family, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid address family format: %s (expected afi/safi)", family)
	}
	return nil
}

// lookupFamilyPlugin returns the plugin name for a family.
// For families in pluginFamilyMap, the plugin is auto-invoked without requiring --plugin flag.
// Family string is normalized to lowercase for lookup.
func lookupFamilyPlugin(family string) string {
	if pluginName, ok := pluginFamilyMap[strings.ToLower(family)]; ok {
		return pluginName
	}
	return ""
}
