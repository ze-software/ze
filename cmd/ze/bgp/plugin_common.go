package bgp

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// PluginConfig defines the capabilities and handlers for a plugin.
type PluginConfig struct {
	Name     string // Plugin name (e.g., "evpn", "hostname")
	Features string // Space-separated features (e.g., "nlri", "capa yang")

	// Feature support
	SupportsNLRI bool
	SupportsCapa bool

	// Handlers
	GetYANG       func() string                                                         // Returns YANG schema
	ConfigLogger  func(level string)                                                    // Configures plugin logger
	RunCLIDecode  func(hex string, text bool, out, err io.Writer) int                   // CLI decode handler
	RunDecode     func(in io.Reader, out io.Writer) int                                 // Engine decode handler (optional)
	RunEngine     func(in io.Reader, out io.Writer) int                                 // Engine mode handler
	ExtraFlags    func(fs *flag.FlagSet)                                                // Register extra flags (optional)
	RunCLIWithCtx func(hex string, text bool, out, err io.Writer, fs *flag.FlagSet) int // CLI decode with flag context (optional)
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

	// Register plugin-specific extra flags
	if cfg.ExtraFlags != nil {
		cfg.ExtraFlags(fs)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested
	if *showFeatures {
		printFeatures(cfg.Features)
		return 0
	}

	// Output YANG schema if requested
	if *showYang && cfg.GetYANG != nil {
		printYANG(cfg.GetYANG())
		return 0
	}

	// Configure plugin logger
	if cfg.ConfigLogger != nil {
		cfg.ConfigLogger(*logLevel)
	}

	// Check for unsupported features
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

	// CLI Mode: --nlri or --capa with optional --text
	hexValue := ""
	if cfg.SupportsNLRI && *nlriHex != "" {
		hexValue = *nlriHex
	} else if cfg.SupportsCapa && *capaHex != "" {
		hexValue = *capaHex
	}

	if hexValue != "" {
		hex, ok := resolveHexInput(hexValue)
		if !ok {
			cliError(os.Stderr, "error: no input on stdin")
			return 1
		}
		if cfg.RunCLIWithCtx != nil {
			return cfg.RunCLIWithCtx(hex, *textOutput, os.Stdout, os.Stderr, fs)
		}
		return cfg.RunCLIDecode(hex, *textOutput, os.Stdout, os.Stderr)
	}

	// Engine Decode Mode
	if decodeMode != nil && *decodeMode {
		return cfg.RunDecode(os.Stdin, os.Stdout)
	}

	// Engine Mode
	return cfg.RunEngine(os.Stdin, os.Stdout)
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

// cliError writes an error message to the given writer.
func cliError(w io.Writer, format string, args ...any) {
	msg := fmt.Sprintf(format+"\n", args...)
	_, err := io.WriteString(w, msg)
	_ = err // CLI output - pipe failure is unrecoverable
}

// unsupportedFeatureError prints a standard error for unsupported decode features.
func unsupportedFeatureError(w io.Writer, plugin, feature, available string) {
	cliError(w, "error: plugin '%s' does not support --%s (available: %s)",
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
