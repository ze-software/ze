// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Overview: register.go — command registration
// Related: registry.go — doctor check registry bridge run from runChecks
// Related: checks_platform.go, checks_storage.go, checks_tls.go — check implementations
// Related: checks_listener.go, checks_reach.go, checks_config.go — check implementations
// Related: checks_helpers.go — shared config-tree navigation helpers

// Runner and output contract: argument parsing, config loading, the ordered
// runChecks sequence, and the text/JSON output formats. Check implementations
// live in the checks_*.go siblings, grouped by concern; owner-registered
// checks arrive through registry.go.

package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// Run executes the doctor command.
func Run(args []string) int {
	jsonOutput := false
	var configPath string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "help", "-h", "--help":
			usage()
			return 0
		default:
			if configPath != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument: %s\n", arg)
				usage()
				return 1
			}
			configPath = arg
		}
	}

	diags := runChecks(configPath)

	ready := true
	for i := range diags {
		if diags[i].Severity == diagnostic.SeverityError {
			ready = false
			break
		}
	}

	if jsonOutput {
		return outputJSON(ready, diags)
	}
	return outputText(ready, diags)
}

func runChecks(configPath string) (diags []diagnostic.Diagnostic) {
	store, storeDiags := resolveStorageWithDiag()
	diags = append(diags, storeDiags...)
	defer func() {
		if err := store.Close(); err != nil {
			var tb textbuf.Buffer
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-storage-unavailable",
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Str("close storage: ").Err(err).String(),
			})
		}
	}()

	platform, platformDiags := checkPlatform()
	diags = append(diags, platformDiags...)
	diags = append(diags, checkStoreIntegrity()...)
	diags = append(diags, checkSystemdServiceInstall(platform)...)
	diags = append(diags, checkMachineID(platform, store)...)
	diags = append(diags, checkRandomSeed(platform)...)
	baseCtx := doctorCheckContext{Store: store, Platform: platform}
	diags = append(diags, runDoctorChecks(doctorCheckPhasePreConfig, baseCtx)...)

	configData, configName, err := loadDoctorConfig(store, configPath)
	if err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-config-missing",
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		})
		diags = append(diags, runDoctorChecks(doctorCheckPhaseMissingConfig, baseCtx)...)
		diags = append(diags, checkKernelModules(nil)...)
		return diags
	}

	result, parseErr := config.LoadConfig(string(configData), configName, nil)
	if parseErr != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-config-parse",
			Severity: diagnostic.SeverityError,
			Message:  parseErr.Error(),
		})
		return diags
	}

	tree := result.Tree
	checkCtx := doctorCheckContext{
		Tree:      tree,
		ConfigDir: result.ConfigDir,
		Plugins:   result.Plugins,
		Store:     store,
		Platform:  platform,
	}

	diags = append(diags, checkSemanticValidation(tree)...)
	diags = append(diags, checkBGPPeerConfig(tree)...)
	diags = append(diags, checkIfaceBackend(tree)...)
	diags = append(diags, checkInterfaces(tree)...)
	diags = append(diags, checkDHCPInterfaces(tree)...)
	diags = append(diags, checkKernelModules(tree)...)
	diags = append(diags, checkFirewallBackend(tree)...)
	diags = append(diags, checkKernelNexthop()...)
	diags = append(diags, checkMPLSSupport(tree)...)
	diags = append(diags, checkTLS(tree, result.ConfigDir)...)
	diags = append(diags, checkWebTLS(tree, store)...)
	diags = append(diags, checkPKICerts(tree)...)
	diags = append(diags, runDoctorChecks(doctorCheckPhasePostConfig, checkCtx)...)
	diags = append(diags, checkSSHHostKey(tree, result.ConfigDir)...)
	diags = append(diags, checkListeners(tree)...)
	diags = append(diags, checkDiskSpace()...)
	diags = append(diags, checkDNSResolvers(tree)...)
	diags = append(diags, checkTACACSServers(tree)...)
	diags = append(diags, checkTelemetryProcfs(tree)...)
	diags = append(diags, checkSysctlProcfs(tree)...)
	diags = append(diags, checkConntrackProcfs(tree)...)
	diags = append(diags, checkPolicyRouteNetlink(tree)...)
	diags = append(diags, checkConfigReferences(tree)...)
	diags = append(diags, checkClockSkew()...)
	diags = append(diags, checkVPPVersion(tree)...)
	diags = append(diags, checkBGPMD5(tree)...)
	diags = append(diags, checkAS112WatchdogWithdraw(tree)...)
	diags = append(diags, checkAS112GlobalOriginCoordination(tree)...)
	diags = append(diags, checkAS112RedistributeOriginCoordination(tree)...)
	diags = append(diags, checkAS112RedistributeNotImported(tree)...)
	diags = append(diags, checkNTPClient(tree, platform)...)
	diags = append(diags, checkNTPClockPrivilege(tree)...)
	diags = append(diags, checkRPKIServers(tree)...)
	diags = append(diags, checkBMPCollectors(tree)...)
	diags = append(diags, checkVPPDPDK(tree)...)
	diags = append(diags, checkUpdateCheckURL(tree, platform)...)
	diags = append(diags, checkUpdateBackendConfig(tree, platform)...)
	diags = append(diags, checkArchiveDestinations(tree)...)
	diags = append(diags, checkWritableDestinations(tree, platform)...)
	diags = append(diags, checkBGPCaptureDirectory(tree)...)
	diags = append(diags, checkResolvConfPath(tree, platform)...)
	diags = append(diags, checkSmartEnabled(tree)...)
	diags = append(diags, checkConfigClaims(tree)...)

	return diags
}

func resolveStorageWithDiag() (storage.Storage, []diagnostic.Diagnostic) {
	s, err := resolve.Storage()
	if err != nil {
		var tb textbuf.Buffer
		return s, []diagnostic.Diagnostic{{
			Code:     "doctor-storage-unavailable",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("blob storage: ").Err(err).String(),
		}}
	}
	return s, nil
}

// loadDoctorConfig returns the config bytes for the doctor check: from configPath
// (or stdin when "-") if given, otherwise from the store's active config. Renamed
// from loadConfigData to end the name collision with the (now-removed) helper in
// internal/component/config/cli (AC-18).
func loadDoctorConfig(store storage.Storage, configPath string) ([]byte, string, error) {
	if configPath != "" {
		data, err := cliio.ReadFile(configPath) // "-" reads stdin
		if err != nil {
			return nil, "", fmt.Errorf("config file: %w", err)
		}
		return data, configPath, nil
	}

	configName := resolve.DefaultConfig(store)
	activeKey := zefs.KeyFileActive.Key(configName)
	data, err := store.ReadFile(activeKey)
	if err != nil {
		data, err = store.ReadFile(configName)
	}
	if err != nil {
		return nil, "", fmt.Errorf("no config found (tried %s): %w", configName, err)
	}
	return data, configName, nil
}

func outputJSON(ready bool, diags []diagnostic.Diagnostic) int {
	result := diagnostic.NewDoctorResult(ready, diags)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if !ready {
		return 1
	}
	return 0
}

func outputText(ready bool, diags []diagnostic.Diagnostic) int {
	b := textbuf.Get()
	defer b.Release()

	if len(diags) == 0 {
		b.Str("all checks passed\n")
		if err := b.StdOut(); err != nil {
			return 1
		}
		return 0
	}

	errCount := 0
	warnCount := 0
	for i := range diags {
		switch diags[i].Severity {
		case diagnostic.SeverityError:
			errCount++
			b.Str("ERROR ")
		case diagnostic.SeverityWarning:
			warnCount++
			b.Str("WARN  ")
		default:
			b.Str("INFO  ")
		}
		b.Str("[").Str(diags[i].Code).Str("] ").Str(diags[i].Message).Byte('\n')
	}

	b.Byte('\n')
	if ready {
		b.Str("ready")
	} else {
		b.Str("not ready")
	}
	b.Str(" (").Str(strconv.Itoa(errCount)).Str(" errors, ").Str(strconv.Itoa(warnCount)).Str(" warnings)\n")

	if err := b.StdOut(); err != nil {
		return 1
	}
	if !ready {
		return 1
	}
	return 0
}

func usage() {
	p := helpfmt.Page{
		Command: "ze doctor",
		Summary: "Check system readiness for running Ze",
		Usage:   []string{"ze doctor [--json] [<config-file>]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--json", Desc: "Output structured JSON diagnostics"},
			}},
		},
		Examples: []string{
			"ze doctor",
			"ze doctor --json",
			"ze doctor --json /etc/ze/ze.conf",
		},
	}
	p.WriteErr()
}
