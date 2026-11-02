// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch
// Related: decode_open.go — OPEN message decoding uses plugin invocation
// Related: decode_mp.go — NLRI parsing uses plugin invocation

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errEmptyAddressFamily = errors.New("empty address family")

// zeCmdPlugin is the argv verb of the top-level `ze plugin` command, which this
// file re-executes to decode. It is not the `ze bgp plugin` subcommand.
const zeCmdPlugin = "plugin"

// pluginMode represents how a plugin should be invoked.
type pluginMode int

const (
	// ModeFork spawns a subprocess via exec.
	ModeFork pluginMode = iota
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
func parsePluginName(input string) (name string, mode pluginMode, path string, args []string) {
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

// decodePlugin is one plugin the operator named with `--plugin`, or with the
// plugin keyword, to decode capabilities and NLRI before the registered
// decoders do. parseDecodePlugin builds it once the name was found in the
// registry, or once the route was found to run a program path.
type decodePlugin struct {
	// spelling is the word as the operator typed it, which the answer names.
	spelling string
	name     string
	mode     pluginMode
	path     string
	args     []string
}

// decodePlugins is the plugins the operator named, in the order given: the
// first one that decodes a capability or an NLRI answers for it, and the
// registered decoders answer what none of them decodes. Empty when none was
// named.
type decodePlugins []decodePlugin

// parseDecodePlugins reads every plugin the operator named. A name no
// registered plugin decodes under is refused, never ignored, and a program
// path is refused when runsPrograms is false: such a path names a program on
// the host the decode runs on, which only the operator's own process may run.
func parseDecodePlugins(spellings []string, runsPrograms bool) (decodePlugins, error) {
	plugins := make(decodePlugins, 0, len(spellings))
	for _, spelling := range spellings {
		plugin, err := parseDecodePlugin(spelling, runsPrograms)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// parseDecodePlugin reads one plugin the operator named; parseDecodePlugins
// states the refusals.
func parseDecodePlugin(spelling string, runsPrograms bool) (decodePlugin, error) {
	name, mode, path, args := parsePluginName(spelling)
	plugin := decodePlugin{spelling: spelling, name: name, mode: mode, path: path, args: args}
	if path != "" {
		if !runsPrograms {
			return decodePlugin{}, fmt.Errorf("plugin %s names a program, which only the ze process "+
				"the operator runs may start; run 'ze bgp decode --plugin %s ...' on the host, "+
				"or name a registered plugin", spelling, spelling)
		}
		return plugin, nil
	}
	if _, ok := decoders()[name]; ok {
		return plugin, nil
	}
	names := slices.Sorted(maps.Keys(decoders()))
	var tb textbuf.Buffer
	tb.Str("plugin ").Str(spelling).Str(": no plugin named ").Quoted(name).
		Str(" decodes; name one of ").Str(textbuf.Join(names, ", ")).
		Str(", or the path of a program that answers 'decode' requests")
	return decodePlugin{}, errors.New(tb.String())
}

// spellings answers the plugins as the operator typed them, in order.
func (plugins decodePlugins) spellings() []string {
	if len(plugins) == 0 {
		return nil
	}
	words := make([]string, len(plugins))
	for i := range plugins {
		words[i] = plugins[i].spelling
	}
	return words
}

// capability answers the first named plugin's decoding of a capability.
// decoded is false when no named plugin decodes the code, and the caller then
// asks the registered decoders.
func (plugins decodePlugins) capability(code uint8, hexData string) (result map[string]any, decoded bool) {
	if len(plugins) == 0 {
		return nil, false
	}
	var tb textbuf.Buffer
	request := tb.Str("decode capability ").Uint8(code).Byte(' ').Str(hexData).String()
	for i := range plugins {
		if result, ok := plugins[i].decode(request).(map[string]any); ok {
			return result, true
		}
	}
	return nil, false
}

// nlri answers the first named plugin's decoding of NLRI of a family.
// decoded is false when no named plugin decodes it, and the caller then asks
// the registered decoders.
func (plugins decodePlugins) nlri(family, hexData string) (result any, decoded bool) {
	if len(plugins) == 0 {
		return nil, false
	}
	request := nlriDecodeRequest(family, hexData)
	for i := range plugins {
		if result := plugins[i].decode(request); result != nil {
			return result, true
		}
	}
	return nil, false
}

// nlriDecodeRequest is the decode request for NLRI of a family.
func nlriDecodeRequest(family, hexData string) string {
	var tb textbuf.Buffer
	return tb.Str("decode nlri ").Str(family).Byte(' ').Str(hexData).String()
}

// decode sends one decode request to the plugin by the route its spelling
// chose, and answers its decoding, or nil when it decoded nothing:
//   - "name": fork `ze plugin <name> --decode`, in-process when that cannot run
//   - "ze.name": goroutine and pipes
//   - "ze-name": synchronous, in-process
//   - "/path": fork the program at the path
func (plugin *decodePlugin) decode(request string) any {
	switch plugin.mode {
	case ModeInternal:
		return invokePluginInternal(plugin.name, request)
	case ModeDirect:
		return invokePluginInProcess(plugin.name, request)
	case ModeFork:
		if plugin.path != "" {
			return invokePluginPath(plugin.path, plugin.args, request)
		}
		return invokePluginForkRequest(plugin.name, request)
	}
	panic("BUG: invalid plugin invocation mode")
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

	// Build plugin command - pluginName comes from the plugin registry (capabilityMap, lookupFamilyPlugin)
	args := []string{zeCmdPlugin, pluginName, "--decode"}

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
	var tb textbuf.Buffer
	if _, err := stdin.Write(tb.Str(request).Byte('\n').Bytes()); err != nil {
		slog.Debug("plugin write failed", "plugin", pluginName, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin stdin close failed", "plugin", pluginName, "err", err)
	}

	// Read response.
	//
	// A scan that stops early leaves result nil, and every caller of these
	// three decode helpers renders nil as {"parsed": false, "raw": hex} or as
	// name "unknown" (decode_mp.go, decode_open.go). A truncated plugin
	// response is never presented as a decode, so there is no wrong answer to
	// report. A truncated JSON body fails json.Unmarshal for the same reason.
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

// invokePluginNLRIDecode invokes a plugin to decode NLRI, by the route its
// spelling chooses (decodePlugin.decode lists them). The registry's names
// carry no prefix, so a plugin the registry names for a family forks.
func invokePluginNLRIDecode(pluginName, family, hexData string) any {
	name, mode, path, args := parsePluginName(pluginName)
	plugin := decodePlugin{spelling: pluginName, name: name, mode: mode, path: path, args: args}
	return plugin.decode(nlriDecodeRequest(family, hexData))
}

// invokePluginForkRequest spawns a built-in plugin subprocess.
// For plain names (ModeFork without path), retries with in-process on failure.
func invokePluginForkRequest(pluginName, request string) any {
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

	var tb2 textbuf.Buffer
	if _, err := stdin.Write(tb2.Str(request).Byte('\n').Bytes()); err != nil {
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

// invokePluginSubprocess spawns a plugin subprocess and sends it one decode
// request, a capability or an NLRI.
func invokePluginSubprocess(pluginName, request string) any {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil // Fall back to in-process via invokePluginForkRequest
	}

	args := []string{zeCmdPlugin, pluginName, "--decode"}

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

	var tb3 textbuf.Buffer
	if _, err := stdin.Write(tb3.Str(request).Byte('\n').Bytes()); err != nil {
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

// decoders returns plugin name to in-process decode function mapping.
// Lazy: all plugin init() functions must complete before first decode call.
var decoders = sync.OnceValue(registry.InProcessDecoders)

// invokePluginInProcess runs plugin decode in-process (Direct mode: ze-name).
// Synchronous, blocking - fastest invocation for CLI decode.
func invokePluginInProcess(pluginName, request string) any {
	decoder, ok := decoders()[pluginName]
	if !ok {
		return nil
	}

	var tb4 textbuf.Buffer
	input := bytes.NewBufferString(tb4.Str(request).Byte('\n').String())
	output := &bytes.Buffer{}

	decoder(input, output)

	return parsePluginResponse(output.String())
}

// invokePluginInternal runs decode via goroutine + pipes (Internal mode: ze.name).
// Uses decode-only runners (same as Direct) but with async pipe I/O.
func invokePluginInternal(pluginName, request string) any {
	decoder, ok := decoders()[pluginName]
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
	var tb5 textbuf.Buffer
	if _, err := inW.Write(tb5.Str(request).Byte('\n').Bytes()); err != nil {
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
		return errEmptyAddressFamily
	}
	parts := strings.SplitN(family, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid address family format: %s (expected afi/safi)", family)
	}
	return nil
}

// lookupFamilyPlugin returns the plugin name for a family, or "" when no
// plugin claims it. A claimed family's plugin is auto-invoked without requiring
// the --plugin flag. Family string is normalized to lowercase for lookup.
//
// It asks the registry on every call rather than caching a copy: a cache taken
// on first use holds whatever the registry held at that moment, so a test that
// registers a plugin and restores the registry afterwards left its claim in
// the copy for every later lookup in the process.
func lookupFamilyPlugin(family string) string {
	return registry.PluginForFamily(strings.ToLower(family))
}
