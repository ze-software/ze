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

var (
	errEmptyAddressFamily       = errors.New("empty address family")
	errSubprocessSkippedInTest  = errors.New("plugin subprocess skipped in test binary")
	errUnexpectedPluginJSONType = errors.New("expected JSON object for capability decode")
)

const (
	// zeCmdPlugin is the argv verb of the top-level `ze plugin` command, which this
	// file re-executes to decode. It is not the `ze bgp plugin` subcommand.
	zeCmdPlugin = "plugin"

	decodeFlag        = "--decode"
	decodedJSONPrefix = "decoded json "
)

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
// asks the registered decoders. A non-nil err reports a plugin execution or
// protocol failure so the caller surfaces it to the operator.
func (plugins decodePlugins) capability(code uint8, hexData string) (result map[string]any, decoded bool, err error) {
	if len(plugins) == 0 {
		return nil, false, nil
	}
	request := capabilityDecodeRequest(code, hexData)
	for i := range plugins {
		raw, ok, err := plugins[i].decode(request)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}
		if result, isMap := raw.(map[string]any); isMap {
			return result, true, nil
		}
	}
	return nil, false, nil
}

// nlri answers the first named plugin's decoding of NLRI of a family.
// decoded is false when no named plugin decodes it, and the caller then asks
// the registered decoders. A non-nil err reports a plugin execution or
// protocol failure so the caller surfaces it to the operator.
func (plugins decodePlugins) nlri(family, hexData string) (result any, decoded bool, err error) {
	if len(plugins) == 0 {
		return nil, false, nil
	}
	request := nlriDecodeRequest(family, hexData)
	for i := range plugins {
		result, ok, err := plugins[i].decode(request)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return result, true, nil
		}
	}
	return nil, false, nil
}

// capabilityDecodeRequest is the decode request for a capability code.
func capabilityDecodeRequest(code uint8, hexData string) string {
	var tb textbuf.Buffer
	return tb.Str("decode capability ").Uint8(code).Byte(' ').Str(hexData).String()
}

// nlriDecodeRequest is the decode request for NLRI of a family.
func nlriDecodeRequest(family, hexData string) string {
	var tb textbuf.Buffer
	return tb.Str("decode nlri ").Str(family).Byte(' ').Str(hexData).String()
}

// decode sends one decode request to the plugin by the route its spelling
// chose, and answers its decoding, whether it decoded, and any execution or
// protocol error:
//   - "name": fork `ze plugin <name> --decode`, in-process when that cannot run
//   - "ze.name": goroutine and pipes
//   - "ze-name": synchronous, in-process
//   - "/path": fork the program at the path
func (plugin *decodePlugin) decode(request string) (any, bool, error) {
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

// formatPluginStepErr formats a plugin I/O or protocol error naming the plugin,
// the failed step, why it matters, and what to check.
func formatPluginStepErr(plugin, step string, err error) error {
	return fmt.Errorf("decode plugin %s %s failed (%w): plugin communication broke before a complete decode; verify the plugin speaks the --decode line protocol", plugin, step, err)
}

// formatPluginExitErr formats a non-zero exit status from an in-process or
// internal decoder without fmt.Sprintf.
func formatPluginExitErr(plugin string, exitCode int) error {
	var tb textbuf.Buffer
	tb.Str("decode plugin ").Str(plugin).Str(" exited with status ").Int(int64(exitCode)).
		Str(": plugin decode failed; verify the input and plugin decoder")
	return errors.New(tb.String())
}

// formatPluginMissingDecoderErr reports a missing internal decoder by name.
func formatPluginMissingDecoderErr(plugin string) error {
	var tb textbuf.Buffer
	tb.Str("decode plugin ").Str(plugin).
		Str(" has no in-process decoder; name a registered plugin or a program path")
	return errors.New(tb.String())
}

// runPluginSubprocess executes binary with args under timeout, sends one
// decode request on stdin, drains stdout for the first "decoded json ..." line,
// and waits for the process to exit. Every pipe, write, close, scan, JSON, and
// exit error is returned so callers never mistake a broken plugin for an
// unknown capability or NLRI.
func runPluginSubprocess(label, binary string, args []string, timeout time.Duration, request string) (any, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // binary is os.Args[0] or operator-named --plugin path

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, false, formatPluginStepErr(label, "open stdin pipe", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, false, formatPluginStepErr(label, "open stdout pipe", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("decode plugin %s failed to start (%w): cannot decode with this plugin; verify the plugin path exists and is executable", label, err)
	}

	var tb textbuf.Buffer
	_, writeErr := stdin.Write(tb.Str(request).Byte('\n').Bytes())
	closeErr := stdin.Close()

	// Loop stdout lines until EOF so the child never blocks on a full stdout
	// pipe before cmd.Wait; skip unexpected output (shell warnings, runtime
	// messages) and keep the first "decoded json ..." response.
	scanner := bufio.NewScanner(stdout)
	var (
		result   any
		decoded  bool
		parseErr error
	)
	for scanner.Scan() {
		if decoded {
			continue
		}
		val, ok, err := parseDecodedJSON(scanner.Text())
		if err != nil {
			if parseErr == nil {
				parseErr = err
			}
			continue
		}
		if ok {
			result = val
			decoded = true
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()

	if waitErr != nil {
		return nil, false, fmt.Errorf("decode plugin %s failed (%w): plugin output cannot be trusted; run the plugin directly with --decode to inspect its error", label, waitErr)
	}
	if writeErr != nil {
		return nil, false, formatPluginStepErr(label, "write request", writeErr)
	}
	if closeErr != nil {
		return nil, false, formatPluginStepErr(label, "close stdin", closeErr)
	}
	if scanErr != nil {
		return nil, false, formatPluginStepErr(label, "read output", scanErr)
	}
	if parseErr != nil {
		return nil, false, formatPluginStepErr(label, "parse JSON response", parseErr)
	}
	if decoded {
		return result, true, nil
	}
	return nil, false, nil
}

// invokePluginDecodeRequest spawns a built-in plugin in decode mode and sends a
// capability decode request. Returns (map, true, nil) on decode, (nil, false, nil)
// on "decoded unknown", or an error on execution/protocol failure.
func invokePluginDecodeRequest(pluginName, request string) (map[string]any, bool, error) {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil, false, errSubprocessSkippedInTest
	}

	// Build plugin command - pluginName comes from the plugin registry (capabilityMap, lookupFamilyPlugin).
	// 5s allows for process startup chain (sh -> wrapper -> ze plugin --decode).
	args := []string{zeCmdPlugin, pluginName, decodeFlag}
	result, decoded, err := runPluginSubprocess(pluginName, os.Args[0], args, 5*time.Second, request)
	if err != nil {
		return nil, false, err
	}
	if !decoded {
		return nil, false, nil
	}
	mapResult, ok := result.(map[string]any)
	if !ok {
		return nil, false, formatPluginStepErr(pluginName, "decode capability response", errUnexpectedPluginJSONType)
	}
	return mapResult, true, nil
}

// invokePluginDecode decodes a capability via plugin.
// Tries subprocess first, falling back to in-process in test binaries.
func invokePluginDecode(pluginName string, code uint8, hexData string) (map[string]any, bool, error) {
	request := capabilityDecodeRequest(code, hexData)

	// Try subprocess first.
	result, decoded, err := invokePluginDecodeRequest(pluginName, request)
	if err == nil {
		return result, decoded, nil
	}
	if !errors.Is(err, errSubprocessSkippedInTest) {
		return nil, false, err
	}

	// Fallback: in-process decode when running under a Go test binary.
	inResult, inDecoded, inErr := invokePluginInProcess(pluginName, request)
	if inErr != nil {
		return nil, false, inErr
	}
	if !inDecoded {
		return nil, false, nil
	}
	mapResult, ok := inResult.(map[string]any)
	if !ok {
		return nil, false, formatPluginStepErr(pluginName, "decode capability response", errUnexpectedPluginJSONType)
	}
	return mapResult, true, nil
}

// invokePluginNLRIDecode invokes a plugin to decode NLRI, returning the decoded
// value or nil when undecoded or failed. Callers on the CLI decode path use
// invokePluginNLRIDecodeResult so plugin errors reach the operator.
func invokePluginNLRIDecode(pluginName, family, hexData string) any {
	result, decoded, err := invokePluginNLRIDecodeResult(pluginName, family, hexData)
	// refuse-by-default: guards invokePluginNLRIDecodeResult -- legacy signature kept for RFC tests; CLI decode callers use invokePluginNLRIDecodeResult
	if err != nil {
		return nil
	}
	if decoded {
		return result
	}
	return nil
}

// invokePluginNLRIDecodeResult invokes a plugin to decode NLRI, by the route its
// spelling chooses (decodePlugin.decode lists them). The registry's names
// carry no prefix, so a plugin the registry names for a family forks.
func invokePluginNLRIDecodeResult(pluginName, family, hexData string) (any, bool, error) {
	name, mode, path, args := parsePluginName(pluginName)
	plugin := decodePlugin{spelling: pluginName, name: name, mode: mode, path: path, args: args}
	return plugin.decode(nlriDecodeRequest(family, hexData))
}

// invokePluginForkRequest spawns a built-in plugin subprocess.
// For plain names (ModeFork without path), falls back to in-process under test binaries.
func invokePluginForkRequest(pluginName, request string) (any, bool, error) {
	result, decoded, err := invokePluginSubprocess(pluginName, request)
	if err == nil {
		return result, decoded, nil
	}
	if !errors.Is(err, errSubprocessSkippedInTest) {
		return nil, false, err
	}
	return invokePluginInProcess(pluginName, request)
}

// invokePluginPath executes an external plugin binary at the given path.
// User-provided args are passed before the mandatory --decode flag.
func invokePluginPath(path string, userArgs []string, request string) (any, bool, error) {
	// 30s allows for process startup under the broad parallel race-test gate.
	// Normal decode returns immediately; the larger budget avoids false negatives
	// when the host is saturated by many concurrently starting test binaries.
	// Longer than invokePluginSubprocess (5s) because the external path may involve
	// an extra shell layer and a separately-built binary.
	cmdArgs := append(slices.Clone(userArgs), decodeFlag)
	return runPluginSubprocess(path, path, cmdArgs, 30*time.Second, request)
}

// parseDecodedJSON extracts JSON from a "decoded json <json>" response line.
// Non-matching lines (such as "decoded unknown" or shell warnings) return
// (nil, false, nil); malformed JSON after the prefix returns an error.
func parseDecodedJSON(line string) (any, bool, error) {
	jsonStr, ok := strings.CutPrefix(line, decodedJSONPrefix)
	if !ok {
		return nil, false, nil
	}
	var arrResult []any
	if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
		return arrResult, true, nil
	}
	var mapResult map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &mapResult); err != nil {
		return nil, false, err
	}
	return mapResult, true, nil
}

// invokePluginSubprocess spawns a plugin subprocess and sends it one decode
// request, a capability or an NLRI.
func invokePluginSubprocess(pluginName, request string) (any, bool, error) {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil, false, errSubprocessSkippedInTest
	}

	// 5s allows for process startup chain (sh -> wrapper -> ze plugin --decode).
	args := []string{zeCmdPlugin, pluginName, decodeFlag}
	return runPluginSubprocess(pluginName, os.Args[0], args, 5*time.Second, request)
}

// decoders returns plugin name to in-process decode function mapping.
// Lazy: all plugin init() functions must complete before first decode call.
var decoders = sync.OnceValue(registry.InProcessDecoders)

// invokePluginInProcess runs plugin decode in-process (Direct mode: ze-name).
// Synchronous, blocking - fastest invocation for CLI decode.
func invokePluginInProcess(pluginName, request string) (any, bool, error) {
	decoder, ok := decoders()[pluginName]
	if !ok {
		return nil, false, nil
	}

	var tb4 textbuf.Buffer
	input := bytes.NewBufferString(tb4.Str(request).Byte('\n').String())
	output := &bytes.Buffer{}

	if exitCode := decoder(input, output); exitCode != 0 {
		return nil, false, formatPluginExitErr(pluginName, exitCode)
	}

	result, decoded, err := parsePluginResponse(output.String())
	if err != nil {
		return nil, false, formatPluginStepErr(pluginName, "parse JSON response", err)
	}
	return result, decoded, nil
}

// invokePluginInternal runs decode via goroutine + pipes (Internal mode: ze.name).
// Uses decode-only runners (same as Direct) but with async pipe I/O.
func invokePluginInternal(pluginName, request string) (any, bool, error) {
	decoder, ok := decoders()[pluginName]
	if !ok {
		return nil, false, formatPluginMissingDecoderErr(pluginName)
	}
	return runPluginInternal(pluginName, decoder, request)
}

// runPluginInternal drives decoder over an io.Pipe pair and propagates any
// pipe I/O, response-parse, or non-zero exit error to the caller.
func runPluginInternal(pluginName string, decoder func(input, output *bytes.Buffer) int, request string) (any, bool, error) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	type internalResult struct {
		exitCode int
		err      error
	}
	done := make(chan internalResult, 1)
	go func() {
		inBuf := &bytes.Buffer{}
		if _, err := io.Copy(inBuf, inR); err != nil {
			_ = outW.CloseWithError(err)
			done <- internalResult{err: formatPluginStepErr(pluginName, "read input", err)}
			return
		}
		outBuf := &bytes.Buffer{}
		exitCode := decoder(inBuf, outBuf)
		if _, err := outW.Write(outBuf.Bytes()); err != nil {
			_ = outW.CloseWithError(err)
			done <- internalResult{exitCode: exitCode, err: formatPluginStepErr(pluginName, "write output", err)}
			return
		}
		if err := outW.Close(); err != nil {
			done <- internalResult{exitCode: exitCode, err: formatPluginStepErr(pluginName, "close output", err)}
			return
		}
		done <- internalResult{exitCode: exitCode}
	}()

	// Send request and close input.
	var tb5 textbuf.Buffer
	if _, err := inW.Write(tb5.Str(request).Byte('\n').Bytes()); err != nil {
		_ = inW.CloseWithError(err)
		_ = outR.Close()
		<-done
		return nil, false, formatPluginStepErr(pluginName, "write request", err)
	}
	if err := inW.Close(); err != nil {
		_ = outR.Close()
		<-done
		return nil, false, formatPluginStepErr(pluginName, "close input", err)
	}

	// Read response.
	var output bytes.Buffer
	_, readErr := io.Copy(&output, outR)
	if readErr != nil {
		_ = outR.Close()
	}
	res := <-done // Wait for decoder to finish.

	if readErr != nil {
		return nil, false, formatPluginStepErr(pluginName, "read output", readErr)
	}
	if res.err != nil {
		return nil, false, res.err
	}
	if res.exitCode != 0 {
		return nil, false, formatPluginExitErr(pluginName, res.exitCode)
	}

	result, decoded, err := parsePluginResponse(output.String())
	if err != nil {
		return nil, false, formatPluginStepErr(pluginName, "parse JSON response", err)
	}
	return result, decoded, nil
}

// parsePluginResponse parses the "decoded json ..." response from a plugin.
func parsePluginResponse(output string) (any, bool, error) {
	return parseDecodedJSON(strings.TrimSpace(output))
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
