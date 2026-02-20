// Design: docs/architecture/cli/plugin-modes.md — plugin CLI framework
//
// Package cli provides the shared plugin CLI framework.
//
// This package is imported by both plugin packages (to build their CLI handlers
// in register.go) and by cmd/ze/bgp/ (for dispatch). It depends only on the
// registry package (a leaf), not on specific plugin implementations, preventing
// import cycles.
package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
)

// BaseConfig creates a PluginConfig pre-filled with common fields from a Registration.
// This eliminates duplication between Registration and PluginConfig in register.go files.
// Plugin-specific handlers (GetYANG, ConfigLogger, RunCLIDecode, etc.) must be set by the caller.
func BaseConfig(reg *registry.Registration) PluginConfig {
	return PluginConfig{
		Name:         reg.Name,
		Features:     reg.Features,
		SupportsNLRI: reg.SupportsNLRI,
		SupportsCapa: reg.SupportsCapa,
		RunEngine:    reg.RunEngine,
	}
}

// PluginConfig defines the capabilities and handlers for a plugin CLI.
type PluginConfig struct {
	Name     string // Plugin name (e.g., "evpn", "hostname").
	Features string // Space-separated features (e.g., "nlri", "capa yang").

	// Feature support.
	SupportsNLRI bool
	SupportsCapa bool

	// Handlers.
	GetYANG       func() string                                                          // Returns YANG schema.
	ConfigLogger  func(level string)                                                     // Configures plugin logger.
	RunCLIDecode  func(hex string, text bool, out, errW io.Writer) int                   // CLI decode handler.
	RunDecode     func(in io.Reader, out io.Writer) int                                  // Engine decode handler (optional).
	RunEngine     func(engineConn, callbackConn net.Conn) int                            // Engine mode handler (RPC).
	ExtraFlags    func(fs *flag.FlagSet)                                                 // Register extra flags (optional).
	RunCLIWithCtx func(hex string, text bool, out, errW io.Writer, fs *flag.FlagSet) int // CLI decode with flag context (optional).
}

// RunPlugin runs a plugin with the standard CLI interface.
// Returns exit code.
func RunPlugin(cfg PluginConfig, args []string) int {
	fs := flag.NewFlagSet("plugin "+cfg.Name, flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")

	var decodeMode *bool
	if cfg.RunDecode != nil {
		decodeMode = fs.Bool("decode", false, "Engine decode protocol mode (reads commands from stdin)")
	}

	var nlriHex, capaHex *string
	if cfg.SupportsNLRI {
		nlriHex = fs.String("nlri", "", "Decode NLRI hex and output JSON (use - for stdin)")
	} else {
		nlriHex = fs.String("nlri", "", "Decode NLRI hex (not supported by this plugin)")
	}
	if cfg.SupportsCapa {
		capaHex = fs.String("capa", "", "Decode capability hex and output JSON (use - for stdin)")
	} else {
		capaHex = fs.String("capa", "", "Decode capability hex (not supported by this plugin)")
	}
	textOutput := fs.Bool("text", false, "Output human-readable text instead of JSON")

	// Register plugin-specific extra flags.
	if cfg.ExtraFlags != nil {
		cfg.ExtraFlags(fs)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested.
	if *showFeatures {
		printFeatures(cfg.Features)
		return 0
	}

	// Output YANG schema if requested.
	if *showYang && cfg.GetYANG != nil {
		printYANG(cfg.GetYANG())
		return 0
	}

	// Configure plugin logger.
	if cfg.ConfigLogger != nil {
		cfg.ConfigLogger(*logLevel)
	}

	// Check for unsupported features.
	if *nlriHex != "" && !cfg.SupportsNLRI {
		available := availableFeatures(cfg)
		unsupportedFeatureError(os.Stderr, cfg.Name, "nlri", available)
		return 1
	}
	if *capaHex != "" && !cfg.SupportsCapa {
		available := availableFeatures(cfg)
		unsupportedFeatureError(os.Stderr, cfg.Name, "capa", available)
		return 1
	}

	// CLI Mode: --nlri or --capa with optional --text.
	hexValue := ""
	if cfg.SupportsNLRI && *nlriHex != "" {
		hexValue = *nlriHex
	} else if cfg.SupportsCapa && *capaHex != "" {
		hexValue = *capaHex
	}

	if hexValue != "" {
		hex, ok := resolveHexInput(hexValue)
		if !ok {
			writeError(os.Stderr, "error: no input on stdin")
			return 1
		}
		if cfg.RunCLIWithCtx != nil {
			return cfg.RunCLIWithCtx(hex, *textOutput, os.Stdout, os.Stderr, fs)
		}
		return cfg.RunCLIDecode(hex, *textOutput, os.Stdout, os.Stderr)
	}

	// Engine Decode Mode.
	if decodeMode != nil && *decodeMode {
		return cfg.RunDecode(os.Stdin, os.Stdout)
	}

	// Engine Mode (RPC over inherited socket pairs).
	engineConn, callbackConn, err := connsFromEnv()
	if err != nil {
		writeError(os.Stderr, "error: %v", err)
		return 1
	}
	return cfg.RunEngine(engineConn, callbackConn)
}

// readHexFromStdin reads a single line of hex from stdin.
func readHexFromStdin() (string, bool) {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), true
	}
	return "", false
}

// resolveHexInput returns the hex string, reading from stdin if hexValue is "-".
func resolveHexInput(hexValue string) (string, bool) {
	if hexValue == "-" {
		return readHexFromStdin()
	}
	return hexValue, true
}

// writeError writes an error message to the given writer.
func writeError(w io.Writer, format string, args ...any) {
	msg := fmt.Sprintf(format+"\n", args...)
	_, err := io.WriteString(w, msg)
	_ = err // CLI output - pipe failure is unrecoverable.
}

// unsupportedFeatureError prints a standard error for unsupported decode features.
func unsupportedFeatureError(w io.Writer, plugin, feature, available string) {
	writeError(w, "error: plugin '%s' does not support --%s (available: %s)",
		plugin, feature, available)
}

// printFeatures prints the plugin's supported features.
func printFeatures(features string) {
	if features != "" {
		fmt.Print(features + "\n")
	}
}

// printYANG prints the YANG schema.
func printYANG(yang string) {
	fmt.Print(yang)
}

// availableFeatures returns the available features string for error messages.
func availableFeatures(cfg PluginConfig) string {
	var parts []string
	if cfg.SupportsNLRI {
		parts = append(parts, "--nlri")
	}
	if cfg.SupportsCapa {
		parts = append(parts, "--capa")
	}
	if len(parts) == 0 {
		if cfg.GetYANG != nil {
			return "--yang"
		}
		return "none"
	}
	return strings.Join(parts, ", ")
}

// connsFromEnv reads ZE_ENGINE_FD and ZE_CALLBACK_FD environment variables
// and returns net.Conn connections for the RPC protocol.
func connsFromEnv() (net.Conn, net.Conn, error) {
	engineFD, err := envFDInt("ZE_ENGINE_FD")
	if err != nil {
		return nil, nil, err
	}
	callbackFD, err := envFDInt("ZE_CALLBACK_FD")
	if err != nil {
		return nil, nil, err
	}

	engineConn, err := fdToConn(engineFD, "ze-engine")
	if err != nil {
		return nil, nil, fmt.Errorf("engine conn: %w", err)
	}

	callbackConn, err := fdToConn(callbackFD, "ze-callback")
	if err != nil {
		engineConn.Close() //nolint:errcheck,gosec // cleanup on error
		return nil, nil, fmt.Errorf("callback conn: %w", err)
	}

	return engineConn, callbackConn, nil
}

// envFDInt reads an environment variable as a file descriptor number.
func envFDInt(name string) (int, error) {
	s := os.Getenv(name)
	if s == "" {
		return 0, fmt.Errorf("%s not set", name)
	}
	fd, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return fd, nil
}

// fdToConn wraps a file descriptor as a net.Conn.
func fdToConn(fd int, name string) (net.Conn, error) {
	f := os.NewFile(uintptr(fd), name)
	if f == nil {
		return nil, fmt.Errorf("invalid fd %d", fd)
	}
	conn, err := net.FileConn(f)
	f.Close() //nolint:errcheck,gosec // FD ownership transferred.
	if err != nil {
		return nil, err
	}
	return conn, nil
}
