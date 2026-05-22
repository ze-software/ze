// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Overview: register.go — command registration

package doctor

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/resolve"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	zeplugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
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
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-storage-unavailable",
				Severity: diagnostic.SeverityWarning,
				Message:  "close storage: " + err.Error(),
			})
		}
	}()

	diags = append(diags, checkStoreIntegrity()...)

	configData, configName, err := loadConfigData(store, configPath)
	if err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-config-missing",
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		})
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

	diags = append(diags, checkIfaceBackend(tree)...)
	diags = append(diags, checkInterfaces(tree)...)
	diags = append(diags, checkKernelModules(tree)...)
	diags = append(diags, checkTLS(tree, result.ConfigDir)...)
	diags = append(diags, checkWebTLS(tree, store)...)
	diags = append(diags, checkPlugins(result.Plugins)...)
	diags = append(diags, checkSSHHostKey(tree, result.ConfigDir)...)
	diags = append(diags, checkListeners(tree)...)

	return diags
}

func resolveStorageWithDiag() (storage.Storage, []diagnostic.Diagnostic) {
	s, err := resolve.Storage()
	if err != nil {
		return s, []diagnostic.Diagnostic{{
			Code:     "doctor-storage-unavailable",
			Severity: diagnostic.SeverityWarning,
			Message:  "blob storage: " + err.Error(),
		}}
	}
	return s, nil
}

func loadConfigData(store storage.Storage, configPath string) ([]byte, string, error) {
	if configPath != "" {
		data, err := os.ReadFile(configPath) //nolint:gosec // user-supplied config path
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

func checkStoreIntegrity() []diagnostic.Diagnostic {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return nil
	}
	storePath := filepath.Join(configDir, "database.zefs")
	if _, err := os.Stat(storePath); err != nil {
		return nil
	}

	report, err := zefs.Check(storePath)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-store-integrity",
			Severity: diagnostic.SeverityError,
			Message:  "store integrity check failed: " + err.Error(),
		}}
	}

	if report.ContainerError != "" {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-store-integrity",
			Severity: diagnostic.SeverityError,
			Message:  "store corrupt: " + report.ContainerError,
		}}
	}

	if report.CorruptEntries > 0 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-store-integrity",
			Severity: diagnostic.SeverityError,
			Message:  "store has " + strconv.Itoa(report.CorruptEntries) + " corrupt entries",
		}}
	}

	return nil
}

func checkIfaceBackend(tree *config.Tree) []diagnostic.Diagnostic {
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	backend, ok := ifaceBlock.Get("backend")
	if !ok || backend == "" {
		return nil
	}

	if backend == "vpp" {
		return checkVPPSocket()
	}

	return nil
}

func checkTLS(tree *config.Tree, configDir string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		diags = append(diags, checkCertPair("mcp", mcpCfg.TLS.Cert, mcpCfg.TLS.Key, configDir)...)
	}

	if apiCfg, ok := config.ExtractAPIConfig(tree); ok {
		diags = append(diags, checkCertPair("api-grpc", apiCfg.GRPCTLSCert, apiCfg.GRPCTLSKey, configDir)...)
	}

	return diags
}

func checkWebTLS(tree *config.Tree, store storage.Storage) []diagnostic.Diagnostic {
	if _, ok := config.ExtractWebConfig(tree); !ok {
		return nil
	}

	certData, certErr := store.ReadFile(zefs.KeyWebCert.Pattern)
	keyExists := store.Exists(zefs.KeyWebKey.Pattern)

	if certErr != nil && !keyExists {
		return nil
	}

	var diags []diagnostic.Diagnostic

	if certErr == nil && len(certData) > 0 {
		diags = append(diags, checkCertExpiry("web", zefs.KeyWebCert.Pattern, certData)...)
	}

	if certErr == nil && !keyExists {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-tls-missing",
			Severity: diagnostic.SeverityError,
			Message:  "web: certificate present in storage but key missing",
		})
	}

	if certErr != nil && keyExists {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-tls-missing",
			Severity: diagnostic.SeverityError,
			Message:  "web: key present in storage but certificate missing",
		})
	}

	return diags
}

func checkCertPair(service, certPath, keyPath, configDir string) []diagnostic.Diagnostic {
	if certPath == "" && keyPath == "" {
		return nil
	}

	var diags []diagnostic.Diagnostic

	if certPath != "" {
		resolved := resolvePath(certPath, configDir)
		data, err := os.ReadFile(resolved) //nolint:gosec // cert path from parsed config
		if err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-tls-missing",
				Severity: diagnostic.SeverityError,
				Message:  service + ": certificate not found: " + resolved,
				Path:     resolved,
			})
		} else {
			diags = append(diags, checkCertExpiry(service, resolved, data)...)
		}
	}

	if keyPath != "" {
		resolved := resolvePath(keyPath, configDir)
		if _, err := os.Stat(resolved); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-tls-missing",
				Severity: diagnostic.SeverityError,
				Message:  service + ": key not found: " + resolved,
				Path:     resolved,
			})
		}
	}

	return diags
}

func checkCertExpiry(service, path string, pemData []byte) []diagnostic.Diagnostic {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-invalid",
			Severity: diagnostic.SeverityWarning,
			Message:  service + ": " + path + ": not valid PEM",
			Path:     path,
		}}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-invalid",
			Severity: diagnostic.SeverityWarning,
			Message:  service + ": " + path + ": cannot parse certificate: " + err.Error(),
			Path:     path,
		}}
	}
	now := time.Now()
	ts := cert.NotAfter.Format(time.RFC3339)
	if now.After(cert.NotAfter) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-expired",
			Severity: diagnostic.SeverityError,
			Message:  service + ": certificate expired on " + ts,
			Path:     path,
			Expected: "not-after > now",
			Actual:   ts,
		}}
	}
	if now.Before(cert.NotBefore) {
		notBefore := cert.NotBefore.Format(time.RFC3339)
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-expired",
			Severity: diagnostic.SeverityError,
			Message:  service + ": certificate not yet valid (starts " + notBefore + ")",
			Path:     path,
			Expected: "not-before < now",
			Actual:   notBefore,
		}}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < 30 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-expired",
			Severity: diagnostic.SeverityWarning,
			Message:  service + ": certificate expires in " + strconv.Itoa(daysLeft) + " days (" + ts + ")",
			Path:     path,
		}}
	}
	return nil
}

func checkPlugins(plugins []zeplugin.PluginConfig) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	for _, p := range plugins {
		if p.Internal || p.Run == "" {
			continue
		}
		parts := strings.Fields(p.Run)
		if len(parts) == 0 {
			continue
		}
		binary := parts[0]
		if filepath.IsAbs(binary) || strings.HasPrefix(binary, "./") || strings.HasPrefix(binary, "../") {
			if _, err := os.Stat(binary); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-plugin-missing",
					Severity: diagnostic.SeverityError,
					Message:  "plugin " + p.Name + ": binary not found: " + binary,
					Path:     binary,
				})
			}
		} else {
			if _, err := exec.LookPath(binary); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-plugin-missing",
					Severity: diagnostic.SeverityError,
					Message:  "plugin " + p.Name + ": binary not on PATH: " + binary,
				})
			}
		}
	}
	return diags
}

func checkSSHHostKey(tree *config.Tree, configDir string) []diagnostic.Diagnostic {
	if configDir == "" {
		return nil
	}

	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil
	}
	sshBlock := envBlock.GetContainer("ssh")
	if sshBlock == nil {
		return nil
	}
	enabled, _ := sshBlock.Get("enabled")
	if enabled != "true" {
		return nil
	}

	var diags []diagnostic.Diagnostic

	keyPath := ""
	if v, ok := sshBlock.Get("host-key"); ok && v != "" {
		keyPath = resolvePath(v, configDir)
	}
	if keyPath == "" {
		keyPath = filepath.Join(configDir, "ssh_host_ed25519_key")
	}
	if _, err := os.Stat(keyPath); err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-ssh-hostkey-missing",
			Severity: diagnostic.SeverityWarning,
			Message:  "SSH host key not found: " + keyPath + " (will be auto-generated on first start)",
			Path:     keyPath,
		})
	}

	if certPath, ok := sshBlock.Get("host-certificate"); ok && certPath != "" {
		resolved := resolvePath(certPath, configDir)
		if _, err := os.Stat(resolved); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-ssh-hostkey-missing",
				Severity: diagnostic.SeverityError,
				Message:  "SSH host certificate not found: " + resolved,
				Path:     resolved,
			})
		}
	}

	return diags
}

type serviceListener struct {
	service string
	host    string
	port    string
}

func checkListeners(tree *config.Tree) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	var listeners []serviceListener

	if webCfg, ok := config.ExtractWebConfig(tree); ok {
		for _, s := range webCfg.Servers {
			listeners = append(listeners, serviceListener{"web", s.Host, s.Port})
		}
	}
	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		for _, s := range mcpCfg.Servers {
			listeners = append(listeners, serviceListener{"mcp", s.Host, s.Port})
		}
	}
	if lgCfg, ok := config.ExtractLGConfig(tree); ok {
		for _, s := range lgCfg.Servers {
			listeners = append(listeners, serviceListener{"looking-glass", s.Host, s.Port})
		}
	}

	if apiCfg, ok := config.ExtractAPIConfig(tree); ok {
		if apiCfg.RESTOn {
			for _, s := range apiCfg.REST {
				listeners = append(listeners, serviceListener{"api-rest", s.Host, s.Port})
			}
		}
		if apiCfg.GRPCOn {
			for _, s := range apiCfg.GRPC {
				listeners = append(listeners, serviceListener{"api-grpc", s.Host, s.Port})
			}
		}
	}

	listeners = append(listeners, extractSSHListeners(tree)...)

	for _, l := range listeners {
		addr := net.JoinHostPort(l.host, l.port)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var lc net.ListenConfig
		ln, err := lc.Listen(ctx, "tcp", addr)
		cancel()
		if err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-listen-unavailable",
				Severity: diagnostic.SeverityWarning,
				Message:  l.service + ": cannot bind " + addr + ": " + err.Error(),
			})
		} else if closeErr := ln.Close(); closeErr != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-listen-unavailable",
				Severity: diagnostic.SeverityWarning,
				Message:  l.service + ": close listener " + addr + ": " + closeErr.Error(),
			})
		}
	}

	return diags
}

func extractSSHListeners(tree *config.Tree) []serviceListener {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil
	}
	sshBlock := envBlock.GetContainer("ssh")
	if sshBlock == nil {
		return nil
	}
	enabled, _ := sshBlock.Get("enabled")
	if enabled != "true" {
		return nil
	}

	var listeners []serviceListener
	if servers := sshBlock.GetListOrdered("server"); len(servers) > 0 {
		for _, s := range servers {
			host := "0.0.0.0"
			port := "2222"
			if v, ok := s.Value.Get("ip"); ok && v != "" {
				host = v
			}
			if v, ok := s.Value.Get("port"); ok && v != "" {
				port = v
			}
			listeners = append(listeners, serviceListener{"ssh", host, port})
		}
	} else if addrs := sshBlock.GetSlice("listen"); len(addrs) > 0 {
		for _, addr := range addrs {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				host = "127.0.0.1"
				port = "2222"
			}
			listeners = append(listeners, serviceListener{"ssh", host, port})
		}
	}

	if len(listeners) == 0 {
		listeners = append(listeners, serviceListener{"ssh", "127.0.0.1", "2222"})
	}

	return listeners
}

func resolvePath(p, configDir string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if configDir != "" {
		return filepath.Join(configDir, p)
	}
	return p
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
		if _, err := os.Stdout.WriteString(b.String()); err != nil {
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

	if _, err := os.Stdout.WriteString(b.String()); err != nil {
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
	p.Write()
}
