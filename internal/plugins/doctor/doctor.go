// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Overview: register.go — command registration

package doctor

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/host"
	zeradius "codeberg.org/thomas-mangin/ze/internal/component/radius"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/resolve"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const (
	doctorListenerFailEnv = "ze.test.doctor.listener-fail-code"
	doctorPlatformEnv     = "ze.test.doctor.platform"
	doctorServiceUnitEnv  = "ze.test.doctor.service-unit"
	configTrueValue       = "true"
	defaultServiceUnit    = "/etc/systemd/system/ze.service"
	defaultNTPPersistPath = "/perm/ze/timefile"
	gokrazyResolvConfPath = "/tmp/resolv.conf"
	linuxResolvConfPath   = "/etc/resolv.conf"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorListenerFailEnv,
	Type:        "string",
	Description: "Force selected doctor listener codes to fail (test infrastructure)",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorServiceUnitEnv,
	Type:        "string",
	Description: "Override ze.service unit path for doctor functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorPlatformEnv,
	Type:        "string",
	Description: "Override doctor platform detection for functional tests",
	Private:     true,
})

var readServiceUnitFile = os.ReadFile
var statServiceExecutable = os.Stat
var lookupServiceUser = user.Lookup
var lookupServiceGroup = user.LookupGroup

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

	platform, platformDiags := checkPlatform()
	diags = append(diags, platformDiags...)
	diags = append(diags, checkStoreIntegrity()...)
	diags = append(diags, checkSystemdServiceInstall(platform)...)
	diags = append(diags, checkMachineID(platform, store)...)
	diags = append(diags, checkRandomSeed(platform)...)
	baseCtx := doctorCheckContext{Store: store, Platform: platform}
	diags = append(diags, runDoctorChecks(doctorCheckPhasePreConfig, baseCtx)...)

	configData, configName, err := loadConfigData(store, configPath)
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
	diags = append(diags, checkRADIUSServers(tree)...)
	diags = append(diags, checkTelemetryProcfs(tree)...)
	diags = append(diags, checkSysctlProcfs(tree)...)
	diags = append(diags, checkConntrackProcfs(tree)...)
	diags = append(diags, checkPolicyRouteNetlink(tree)...)
	diags = append(diags, checkConfigReferences(tree)...)
	diags = append(diags, checkClockSkew()...)
	diags = append(diags, checkVPPVersion(tree)...)
	diags = append(diags, checkBGPMD5(tree)...)
	diags = append(diags, checkNTPClient(tree, platform)...)
	diags = append(diags, checkNTPClockPrivilege(tree)...)
	diags = append(diags, checkRPKIServers(tree)...)
	diags = append(diags, checkBMPCollectors(tree)...)
	diags = append(diags, checkVPPDPDK(tree)...)
	diags = append(diags, checkUpdateCheckURL(tree, platform)...)
	diags = append(diags, checkUpdateBackendConfig(tree, platform)...)
	diags = append(diags, checkArchiveDestinations(tree)...)
	diags = append(diags, checkWritableDestinations(tree, platform)...)
	diags = append(diags, checkResolvConfPath(tree, platform)...)
	diags = append(diags, checkSmartEnabled(tree)...)

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

func checkPlatform() (*host.PlatformInfo, []diagnostic.Diagnostic) {
	p, err := detectDoctorPlatform()
	if err != nil {
		return nil, []diagnostic.Diagnostic{{
			Code:     "doctor-platform-detect",
			Severity: diagnostic.SeverityWarning,
			Message:  "platform detection failed: " + err.Error(),
		}}
	}
	if p == nil {
		return nil, []diagnostic.Diagnostic{{
			Code:     "doctor-platform-detect",
			Severity: diagnostic.SeverityWarning,
			Message:  "platform detection returned no platform information",
		}}
	}
	var diags []diagnostic.Diagnostic
	if p.Type == host.PlatformUnknown {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-platform-unknown",
			Severity: diagnostic.SeverityWarning,
			Message:  "could not identify runtime platform",
		})
	}
	if p.Type == host.PlatformGokrazy && !p.PersistentStorageWritable {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-platform-perm",
			Severity: diagnostic.SeverityError,
			Message:  "gokrazy /perm partition is not writable; config and state persistence will fail",
		})
	}
	if p.Type == host.PlatformContainer && p.ReadOnlyRoot {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-platform-container-ro",
			Severity: diagnostic.SeverityWarning,
			Message:  "running in container with read-only root filesystem; ensure writable volumes are mounted for config and state",
		})
	}
	return p, diags
}

func detectDoctorPlatform() (*host.PlatformInfo, error) {
	forced := strings.TrimSpace(env.Get(doctorPlatformEnv))
	if forced != "" {
		return forcedPlatformInfo(forced)
	}
	return host.DetectPlatform()
}

func forcedPlatformInfo(name string) (*host.PlatformInfo, error) {
	switch strings.ToLower(name) {
	case "unknown":
		return &host.PlatformInfo{Type: host.PlatformUnknown}, nil
	case "gokrazy":
		return &host.PlatformInfo{Type: host.PlatformGokrazy, ReadOnlyRoot: true, PermAvailable: true, PersistentStorageWritable: true}, nil
	case "systemd":
		return &host.PlatformInfo{Type: host.PlatformSystemd, SystemdAvailable: true}, nil
	case "container":
		return &host.PlatformInfo{Type: host.PlatformContainer}, nil
	case "plain", "plain-linux":
		return &host.PlatformInfo{Type: host.PlatformPlainLinux}, nil
	case "darwin":
		return &host.PlatformInfo{Type: host.PlatformDarwin}, nil
	default:
		return nil, errors.New("unknown forced platform: " + name)
	}
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

type serviceUnitInfo struct {
	execStart string
	user      string
	group     string
}

func checkSystemdServiceInstall(platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform != nil && (platform.Type == host.PlatformGokrazy || platform.Type == host.PlatformContainer) {
		return nil
	}

	unitPath := env.Get(doctorServiceUnitEnv)
	if unitPath == "" {
		unitPath = defaultServiceUnit
	}

	data, err := readServiceUnitFile(unitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-unit",
			Severity: diagnostic.SeverityWarning,
			Message:  "systemd service unit cannot be read: " + unitPath + ": " + err.Error(),
			Path:     unitPath,
		}}
	}

	unit := parseServiceUnit(data)
	var diags []diagnostic.Diagnostic
	diags = append(diags, checkServiceExecutable(unitPath, unit.execStart)...)
	if unit.user != "" {
		diags = append(diags, checkServiceUser(unitPath, unit.user)...)
	}
	if unit.group != "" {
		diags = append(diags, checkServiceGroup(unitPath, unit.group)...)
	}
	return diags
}

func parseServiceUnit(data []byte) serviceUnitInfo {
	var unit serviceUnitInfo
	inService := false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inService = line == "[Service]"
			continue
		}
		if !inService {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ExecStart":
			unit.execStart = firstSystemdCommand(value)
		case "User":
			unit.user = strings.TrimSpace(value)
		case "Group":
			unit.group = strings.TrimSpace(value)
		}
	}
	return unit
}

func firstSystemdCommand(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	cmd := strings.Trim(fields[0], `"'`)
	for cmd != "" {
		switch cmd[0] {
		case '-', '+', '!', '@', ':':
			cmd = cmd[1:]
		default:
			return cmd
		}
	}
	return cmd
}

func checkServiceExecutable(unitPath, executable string) []diagnostic.Diagnostic {
	if executable == "" {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-executable",
			Severity: diagnostic.SeverityError,
			Message:  "systemd service unit has no ExecStart command: " + unitPath,
			Path:     unitPath,
		}}
	}
	if !filepath.IsAbs(executable) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-executable",
			Severity: diagnostic.SeverityError,
			Message:  "systemd service ExecStart is not an absolute path: " + executable,
			Path:     unitPath,
			Expected: "absolute executable path",
			Actual:   executable,
		}}
	}
	info, err := statServiceExecutable(executable)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-executable",
			Severity: diagnostic.SeverityError,
			Message:  "systemd service executable not found: " + executable + ": " + err.Error(),
			Path:     executable,
		}}
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-executable",
			Severity: diagnostic.SeverityError,
			Message:  "systemd service executable is not executable: " + executable,
			Path:     executable,
			Expected: "executable file",
			Actual:   info.Mode().String(),
		}}
	}
	return nil
}

func checkServiceUser(unitPath, name string) []diagnostic.Diagnostic {
	if _, err := lookupServiceUser(name); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-user",
			Severity: diagnostic.SeverityError,
			Message:  "systemd service user not found: " + name,
			Path:     unitPath,
			Expected: "existing user",
			Actual:   name,
		}}
	}
	return nil
}

func checkServiceGroup(unitPath, name string) []diagnostic.Diagnostic {
	if _, err := lookupServiceGroup(name); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-group",
			Severity: diagnostic.SeverityError,
			Message:  "systemd service group not found: " + name,
			Path:     unitPath,
			Expected: "existing group",
			Actual:   name,
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
		sockPath := ""
		if vpp := tree.GetContainer("vpp"); vpp != nil {
			sockPath, _ = vpp.Get("api-socket")
		}
		return checkVPPSocket(sockPath)
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

func checkPKICerts(tree *config.Tree) []diagnostic.Diagnostic {
	pki := tree.GetContainer("pki")
	if pki == nil {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, ca := range pki.GetListOrdered("ca") {
		path := "pki/ca/" + ca.Key + "/certificate"
		certData, ok := ca.Value.Get("certificate")
		if !ok || certData == "" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-pki-cert",
				Severity: diagnostic.SeverityError,
				Message:  "PKI CA " + ca.Key + ": certificate missing",
				Path:     path,
			})
			continue
		}
		diags = append(diags, checkBase64DERCert("PKI CA "+ca.Key, path, certData)...)
	}

	for _, cert := range pki.GetListOrdered("certificate") {
		path := "pki/certificate/" + cert.Key + "/certificate"
		certData, ok := cert.Value.Get("certificate")
		if !ok || certData == "" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-pki-cert",
				Severity: diagnostic.SeverityError,
				Message:  "PKI certificate " + cert.Key + ": certificate missing",
				Path:     path,
			})
			continue
		}
		diags = append(diags, checkBase64DERCert("PKI certificate "+cert.Key, path, certData)...)
	}

	return diags
}

func checkBase64DERCert(service, path, value string) []diagnostic.Diagnostic {
	der, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  service + ": certificate is not base64 DER: " + err.Error(),
			Path:     path,
		}}
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  service + ": cannot parse certificate: " + err.Error(),
			Path:     path,
		}}
	}

	now := time.Now()
	notAfter := cert.NotAfter.Format(time.RFC3339)
	if now.After(cert.NotAfter) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  service + ": certificate expired on " + notAfter,
			Path:     path,
			Expected: "not-after > now",
			Actual:   notAfter,
		}}
	}
	if now.Before(cert.NotBefore) {
		notBefore := cert.NotBefore.Format(time.RFC3339)
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
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
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityWarning,
			Message:  service + ": certificate expires in " + strconv.Itoa(daysLeft) + " days (" + notAfter + ")",
			Path:     path,
		}}
	}
	return nil
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
	if enabled != configTrueValue {
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
	service  string
	network  string
	host     string
	port     string
	code     string
	severity diagnostic.Severity
}

var listenerProbe = probeListener

var registerListenerDefaultsOnce sync.Once

func collectSchemaListeners(tree *config.Tree) []serviceListener {
	schema, err := config.YANGSchema()
	if err != nil {
		return collectHardcodedListeners(tree)
	}
	services := config.DiscoverListenerServices(schema)
	if len(services) == 0 {
		return collectHardcodedListeners(tree)
	}
	registerListenerDefaultsOnce.Do(config.RegisterBuiltinListenerDefaults)
	endpoints := config.CollectListenersWithDefaults(tree, schema)

	listeners := make([]serviceListener, 0, len(endpoints))
	for _, ep := range endpoints {
		l := serviceListener{
			service:  ep.Service,
			network:  ep.Protocol,
			host:     ep.IP.String(),
			port:     textbuf.Uint16(ep.Port),
			code:     "doctor-listen-unavailable",
			severity: diagnostic.SeverityWarning,
		}
		listeners = append(listeners, l)
	}
	return listeners
}

func collectHardcodedListeners(tree *config.Tree) []serviceListener {
	var listeners []serviceListener

	if webCfg, ok := config.ExtractWebConfig(tree); ok {
		for _, s := range webCfg.Servers {
			listeners = append(listeners, tcpListener("web", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		for _, s := range mcpCfg.Servers {
			listeners = append(listeners, tcpListener("mcp", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if lgCfg, ok := config.ExtractLGConfig(tree); ok {
		for _, s := range lgCfg.Servers {
			listeners = append(listeners, tcpListener("looking-glass", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if apiCfg, ok := config.ExtractAPIConfig(tree); ok {
		if apiCfg.RESTOn {
			for _, s := range apiCfg.REST {
				listeners = append(listeners, tcpListener("api-server-rest", s.Host, s.Port, "doctor-listen-unavailable"))
			}
		}
		if apiCfg.GRPCOn {
			for _, s := range apiCfg.GRPC {
				listeners = append(listeners, tcpListener("api-server-grpc", s.Host, s.Port, "doctor-listen-unavailable"))
			}
		}
	}
	listeners = append(listeners, extractSSHListeners(tree)...)
	listeners = append(listeners, extractTelemetryListeners(tree)...)
	return listeners
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
	if enabled != configTrueValue {
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
			listeners = append(listeners, tcpListener("ssh", host, port, "doctor-listen-unavailable"))
		}
	}

	if len(listeners) == 0 {
		listeners = append(listeners, tcpListener("ssh", "127.0.0.1", "2222", "doctor-listen-unavailable"))
	}

	return listeners
}

func extractTelemetryListeners(tree *config.Tree) []serviceListener {
	prom := getContainerPath(tree, "telemetry", "prometheus")
	if !configEnabled(prom, false) {
		return nil
	}

	servers := prom.GetListOrdered("server")
	if len(servers) == 0 {
		return []serviceListener{tcpListener("telemetry", "127.0.0.1", "9273", "doctor-listen-unavailable")}
	}

	listeners := make([]serviceListener, 0, len(servers))
	for _, s := range servers {
		host := valueOrDefault(s.Value, "ip", "127.0.0.1")
		port := valueOrDefault(s.Value, "port", "9273")
		listeners = append(listeners, tcpListener("telemetry", host, port, "doctor-listen-unavailable"))
	}
	return listeners
}

func checkListeners(tree *config.Tree) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	listeners := collectSchemaListeners(tree)
	listeners = append(listeners, extractBGPListeners(tree)...)
	listeners = append(listeners, extractBFDListeners(tree)...)
	listeners = append(listeners, extractIPsecListeners(tree)...)
	listeners = append(listeners, extractTFTPListeners(tree)...)
	listeners = append(listeners, extractImageListeners(tree)...)
	listeners = append(listeners, extractNTPListeners(tree)...)

	for _, l := range dedupeListeners(listeners) {
		if err := listenerProbe(l); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     l.code,
				Severity: l.severity,
				Message:  l.service + ": cannot bind " + l.network + " " + listenerAddress(l) + ": " + err.Error(),
			})
		}
	}

	return diags
}

func tcpListener(service, host, port, code string) serviceListener {
	return serviceListener{service: service, network: "tcp", host: host, port: port, code: code, severity: diagnostic.SeverityWarning}
}

func udpListener(service, port, code string) serviceListener {
	return serviceListener{service: service, network: "udp", host: "0.0.0.0", port: port, code: code, severity: diagnostic.SeverityWarning}
}

func probeListener(l serviceListener) error {
	if forcedDoctorCode(l.code, env.Get(doctorListenerFailEnv)) {
		return errors.New("forced listener failure")
	}

	addr := listenerAddress(l)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lc net.ListenConfig
	if l.network == "udp" {
		pc, err := lc.ListenPacket(ctx, l.network, addr)
		if err != nil {
			return err
		}
		return pc.Close()
	}

	ln, err := lc.Listen(ctx, l.network, addr)
	if err != nil {
		return err
	}
	return ln.Close()
}

func listenerAddress(l serviceListener) string {
	return net.JoinHostPort(l.host, l.port)
}

func dedupeListeners(listeners []serviceListener) []serviceListener {
	seen := make(map[string]bool, len(listeners))
	result := make([]serviceListener, 0, len(listeners))
	for _, l := range listeners {
		if l.code == "" {
			l.code = "doctor-listen-unavailable"
		}
		if l.severity == "" {
			l.severity = diagnostic.SeverityWarning
		}
		key := l.service + "\x00" + l.network + "\x00" + l.host + "\x00" + l.port + "\x00" + l.code
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, l)
	}
	return result
}

func extractBGPListeners(tree *config.Tree) []serviceListener {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	var listeners []serviceListener
	for _, p := range bgp.GetListOrdered("peer") {
		listeners = appendBGPListener(listeners, nil, p.Value)
	}
	for _, g := range bgp.GetListOrdered("group") {
		if remoteIP, _ := nestedValue(g.Value, "connection", "remote", "ip"); remoteIP == "dynamic" {
			listeners = appendBGPListener(listeners, nil, g.Value)
		}
		for _, p := range g.Value.GetListOrdered("peer") {
			listeners = appendBGPListener(listeners, g.Value, p.Value)
		}
	}
	return listeners
}

func appendBGPListener(listeners []serviceListener, parent, node *config.Tree) []serviceListener {
	if accept, ok := inheritedValue(parent, node, "connection", "local", "accept"); ok && accept == "false" {
		return listeners
	}

	host, ok := inheritedValue(parent, node, "connection", "local", "ip")
	if !ok || host == "" || host == "auto" {
		return listeners
	}

	port, _ := inheritedValue(parent, node, "connection", "local", "port")
	if port == "" {
		port, _ = inheritedValue(parent, node, "connection", "remote", "port")
	}
	if port == "" {
		port = "179"
	}

	return append(listeners, tcpListener("bgp", host, port, "doctor-bgp-listen"))
}

func extractBFDListeners(tree *config.Tree) []serviceListener {
	bfd := tree.GetContainer("bfd")
	if !configEnabled(bfd, true) {
		return nil
	}
	return []serviceListener{udpListener("bfd", "3784", "doctor-bfd-port")}
}

func extractIPsecListeners(tree *config.Tree) []serviceListener {
	if getContainerPath(tree, "vpn", "ipsec") == nil {
		return nil
	}
	return []serviceListener{
		udpListener("ipsec", "500", "doctor-ipsec-listen"),
		udpListener("ipsec", "4500", "doctor-ipsec-listen"),
	}
}

func extractTFTPListeners(tree *config.Tree) []serviceListener {
	tftp := getContainerPath(tree, "service", "tftp-server")
	if !configEnabled(tftp, false) {
		return nil
	}
	return []serviceListener{udpListener("tftp", "69", "doctor-tftp-listen")}
}

func extractImageListeners(tree *config.Tree) []serviceListener {
	image := getContainerPath(tree, "service", "image-server")
	if !configEnabled(image, false) {
		return nil
	}
	return []serviceListener{tcpListener("image-server", "0.0.0.0", valueOrDefault(image, "listen-port", "80"), "doctor-image-listen")}
}

func extractNTPListeners(tree *config.Tree) []serviceListener {
	ntp := getContainerPath(tree, "environment", "ntp")
	if !configEnabled(ntp, false) {
		return nil
	}
	return []serviceListener{udpListener("ntp", "123", "doctor-ntp-listen")}
}

var interfaceByName = net.InterfaceByName

func checkDHCPInterfaces(tree *config.Tree) []diagnostic.Diagnostic {
	dhcp := getContainerPath(tree, "service", "dhcp-server")
	if !configEnabled(dhcp, false) {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, name := range dhcp.GetSlice("listen-interface") {
		if strings.ContainsAny(name, "/\x00") || strings.Contains(name, "..") {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-dhcp-iface",
				Severity: diagnostic.SeverityError,
				Message:  "DHCP server listen interface has invalid name: " + name,
				Path:     "service/dhcp-server/listen-interface",
			})
			continue
		}
		if _, err := interfaceByName(name); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-dhcp-iface",
				Severity: diagnostic.SeverityError,
				Message:  "DHCP server listen interface not found: " + name,
				Path:     "service/dhcp-server/listen-interface",
			})
		}
	}
	return diags
}

var tcpReachable = tcpServerReachable
var udpReachable = udpServerReachable

func checkTACACSServers(tree *config.Tree) []diagnostic.Diagnostic {
	tacacs := getContainerPath(tree, "system", "authentication", "tacacs")
	if tacacs == nil {
		return nil
	}

	timeout := configTimeout(tacacs, "timeout", 5)
	checked := false
	for _, s := range tacacs.GetListOrdered("server") {
		address := valueOrDefault(s.Value, "address", s.Key)
		if address == "" {
			continue
		}
		checked = true
		if tcpReachable(net.JoinHostPort(address, valueOrDefault(s.Value, "port", "49")), timeout) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-tacacs-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured TACACS+ servers are reachable",
	}}
}

func checkRADIUSServers(tree *config.Tree) []diagnostic.Diagnostic {
	radiusCfg := getContainerPath(tree, "l2tp", "auth", "radius")
	if radiusCfg == nil {
		return nil
	}

	timeout := configTimeout(radiusCfg, "timeout", 3)
	nasID := valueOrDefault(radiusCfg, "nas-identifier", "ze-doctor")
	var sourceIP net.IP
	if source, ok := radiusCfg.Get("source-address"); ok && source != "" {
		sourceIP = net.ParseIP(source)
	}
	checked := false
	for _, s := range radiusCfg.GetListOrdered("server") {
		address, ok := s.Value.Get("address")
		if !ok || address == "" {
			continue
		}
		checked = true
		secret := []byte(valueOrDefault(s.Value, "shared-key", ""))
		if udpReachable(net.JoinHostPort(address, valueOrDefault(s.Value, "port", "1812")), secret, sourceIP, nasID, timeout) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-radius-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured RADIUS servers are reachable",
	}}
}

func tcpServerReachable(addr string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func udpServerReachable(addr string, secret []byte, sourceIP net.IP, nasID string, timeout time.Duration) bool {
	if len(secret) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := zeradius.NewClient(zeradius.ClientConfig{Timeout: timeout, Retries: 1, SourceAddress: sourceIP})
	if err != nil {
		return false
	}
	defer func() { _ = client.Close() }()

	auth, err := zeradius.RandomAuthenticator()
	if err != nil {
		return false
	}
	attrs := []zeradius.Attr{
		{Type: zeradius.AttrUserName, Value: zeradius.AttrString("ze-doctor")},
		{Type: zeradius.AttrUserPassword, Value: zeradius.AttrString("ze-doctor")},
	}
	if nasID != "" {
		attrs = append(attrs, zeradius.Attr{Type: zeradius.AttrNASIdentifier, Value: zeradius.AttrString(nasID)})
	}
	pkt := &zeradius.Packet{
		Code:          zeradius.CodeAccessRequest,
		Identifier:    byte(time.Now().UnixNano()),
		Authenticator: auth,
		Attrs:         attrs,
	}
	_, err = client.Exchange(ctx, pkt, secret, addr)
	return err == nil
}

func forcedDoctorCode(code, configured string) bool {
	if configured == "" {
		return false
	}
	for item := range strings.SplitSeq(configured, ",") {
		if strings.TrimSpace(item) == code {
			return true
		}
	}
	return false
}

func configTimeout(tree *config.Tree, leaf string, def int) time.Duration {
	if v, ok := tree.Get(leaf); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(def) * time.Second
}

func configEnabled(tree *config.Tree, defaultValue bool) bool {
	if tree == nil {
		return false
	}
	if v, ok := tree.Get("enabled"); ok {
		return v == configTrueValue
	}
	return defaultValue
}

func getContainerPath(tree *config.Tree, names ...string) *config.Tree {
	cur := tree
	for _, name := range names {
		if cur == nil {
			return nil
		}
		cur = cur.GetContainer(name)
	}
	return cur
}

func nestedValue(tree *config.Tree, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	containerPath := path[:len(path)-1]
	leaf := path[len(path)-1]
	container := getContainerPath(tree, containerPath...)
	if container == nil {
		return "", false
	}
	return container.Get(leaf)
}

func inheritedValue(parent, node *config.Tree, path ...string) (string, bool) {
	if v, ok := nestedValue(node, path...); ok {
		return v, true
	}
	return nestedValue(parent, path...)
}

func valueOrDefault(tree *config.Tree, name, def string) string {
	if tree == nil {
		return def
	}
	if v, ok := tree.Get(name); ok && v != "" {
		return v
	}
	return def
}

func checkConfigReferences(tree *config.Tree) []diagnostic.Diagnostic {
	bgpBlock := tree.GetContainer("bgp")
	if bgpBlock == nil {
		return nil
	}

	// Collect defined filter instance names from bgp/policy.
	// Policy lists (prefix-list, as-path, etc.) are added by plugins via YANG
	// augment. Each list's keys are filter instance names.
	defined := make(map[string]bool)
	if policy := bgpBlock.GetContainer("policy"); policy != nil {
		policyMap := policy.ToMap()
		collectFilterNamesFromMap(policyMap, defined)
	}

	var diags []diagnostic.Diagnostic

	// Collect all filter references from global, group, and peer levels.
	if filter := bgpBlock.GetContainer("filter"); filter != nil {
		diags = append(diags, checkFilterRefs(filter, defined, "bgp/filter")...)
	}

	groups := bgpBlock.GetListOrdered("group")
	for _, g := range groups {
		groupPath := "bgp/group/" + g.Key + "/filter"
		if filter := g.Value.GetContainer("filter"); filter != nil {
			diags = append(diags, checkFilterRefs(filter, defined, groupPath)...)
		}
		peers := g.Value.GetListOrdered("peer")
		for _, p := range peers {
			peerPath := "bgp/group/" + g.Key + "/peer/" + p.Key + "/filter"
			if filter := p.Value.GetContainer("filter"); filter != nil {
				diags = append(diags, checkFilterRefs(filter, defined, peerPath)...)
			}
		}
	}

	peers := bgpBlock.GetListOrdered("peer")
	for _, p := range peers {
		peerPath := "bgp/peer/" + p.Key + "/filter"
		if filter := p.Value.GetContainer("filter"); filter != nil {
			diags = append(diags, checkFilterRefs(filter, defined, peerPath)...)
		}
	}

	return diags
}

// collectFilterNamesFromMap walks the policy map (from ToMap()) and collects
// all second-level keys as filter instance names. The map structure is:
//
//	{"prefix-list": {"customers": {...}}, "as-path": {"as1234": {...}}}
func collectFilterNamesFromMap(m map[string]any, defined map[string]bool) {
	for _, v := range m {
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for name := range sub {
			defined[name] = true
		}
	}
}

// filterInstanceName extracts the filter instance name from a reference.
// Filter references can use three forms:
//   - "bgp-filter-prefix:customers"  (plugin-process:name)
//   - "prefix-list:customers"        (filter-type:name)
//   - "customers"                    (plain name)
//
// All resolve to the same instance name after stripping the prefix.
func filterInstanceName(ref string) string {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func checkFilterRefs(filter *config.Tree, defined map[string]bool, path string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	for _, dir := range []string{"import", "export"} {
		refs := filter.GetSlice(dir)
		for _, ref := range refs {
			name := filterInstanceName(ref)
			if len(defined) == 0 || !defined[name] {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-config-reference",
					Severity: diagnostic.SeverityError,
					Message:  path + "/" + dir + ": references undefined filter '" + ref + "'",
				})
			}
		}
	}
	return diags
}

func checkDiskSpace() []diagnostic.Diagnostic {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return nil
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(configDir, &stat); err != nil {
		return nil
	}
	if stat.Blocks == 0 {
		return nil
	}
	pctFree := (stat.Bavail * 100) / stat.Blocks
	if pctFree < 5 {
		pctStr := textbuf.UintStr(pctFree, "%")
		return []diagnostic.Diagnostic{{
			Code:     "doctor-disk-space",
			Severity: diagnostic.SeverityWarning,
			Message:  "config partition has " + pctStr + " free space",
			Path:     configDir,
			Expected: ">= 5%",
			Actual:   pctStr,
		}}
	}
	return nil
}

func checkDNSResolvers(tree *config.Tree) []diagnostic.Diagnostic {
	sysBlock := tree.GetContainer("system")
	if sysBlock == nil {
		return nil
	}
	servers := sysBlock.GetSlice("name-server")
	if len(servers) == 0 {
		return nil
	}

	if slices.ContainsFunc(servers, dnsServerResponds) {
		return nil
	}

	return []diagnostic.Diagnostic{{
		Code:     "doctor-dns-resolver",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured name servers responded",
	}}
}

// dnsServerResponds probes a DNS server with a query. Returns true if the
// server responds at all (including NXDOMAIN or SERVFAIL), false only if
// the server is unreachable or times out.
func dnsServerResponds(addr string) bool {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", net.JoinHostPort(addr, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := resolver.LookupHost(ctx, "_dns-probe.invalid.")
	if err == nil {
		return true
	}
	// A DNS error (NXDOMAIN, SERVFAIL) means the server responded.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.IsTimeout && !dnsErr.IsTemporary {
		return true
	}
	return false
}

const clockSkewThreshold = 5 * time.Minute

// checkClockSkew queries a public NTP pool and warns if the system clock
// is off by more than 5 minutes. Uses a lightweight SNTP request (mode 3)
// rather than a full NTP client.
func checkClockSkew() []diagnostic.Diagnostic {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "udp", "pool.ntp.org:123")
	if err != nil {
		return nil // network unavailable, skip silently
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	// SNTP request: version 3, mode 3 (client), 48 bytes.
	req := make([]byte, 48)
	req[0] = 0x1B // LI=0, VN=3, Mode=3
	if _, err := conn.Write(req); err != nil {
		return nil
	}

	resp := make([]byte, 48)
	if _, err := conn.Read(resp); err != nil {
		return nil
	}

	// Transmit timestamp starts at byte 40 (seconds since 1900-01-01).
	const ntpEpochOffset = 2208988800 // seconds between 1900 and 1970
	secs := uint64(resp[40])<<24 | uint64(resp[41])<<16 | uint64(resp[42])<<8 | uint64(resp[43])
	if secs < ntpEpochOffset {
		return nil // invalid response
	}
	ntpTime := time.Unix(int64(secs-ntpEpochOffset), 0)
	skew := time.Since(ntpTime)
	if skew < 0 {
		skew = -skew
	}

	if skew > clockSkewThreshold {
		var b textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-clock-skew",
			Severity: diagnostic.SeverityWarning,
			Message:  b.Reset().Str("system clock skewed by ").Int(int64(skew / time.Second)).Str("s (threshold ").Int(int64(clockSkewThreshold / time.Second)).Str("s)").String(),
		}}
	}
	return nil
}

func checkSemanticValidation(tree *config.Tree) []diagnostic.Diagnostic {
	return config.ValidateSemantics(tree)
}

func checkBGPMD5(tree *config.Tree) []diagnostic.Diagnostic {
	if network.TCPMD5Supported() {
		return nil
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	hasMD5 := func(parent, node *config.Tree) bool {
		if pw, ok := inheritedValue(parent, node, "connection", "md5", "password"); ok && pw != "" {
			return true
		}
		return false
	}

	for _, p := range bgp.GetListOrdered("peer") {
		if hasMD5(nil, p.Value) {
			return []diagnostic.Diagnostic{{
				Code:     "doctor-bgp-md5",
				Severity: diagnostic.SeverityWarning,
				Message:  "BGP peer " + p.Key + " requires TCP MD5 but platform does not support it",
			}}
		}
	}
	for _, g := range bgp.GetListOrdered("group") {
		if hasMD5(nil, g.Value) {
			return []diagnostic.Diagnostic{{
				Code:     "doctor-bgp-md5",
				Severity: diagnostic.SeverityWarning,
				Message:  "BGP group " + g.Key + " requires TCP MD5 but platform does not support it",
			}}
		}
		for _, p := range g.Value.GetListOrdered("peer") {
			if hasMD5(g.Value, p.Value) {
				return []diagnostic.Diagnostic{{
					Code:     "doctor-bgp-md5",
					Severity: diagnostic.SeverityWarning,
					Message:  "BGP peer " + g.Key + "/" + p.Key + " requires TCP MD5 but platform does not support it",
				}}
			}
		}
	}
	return nil
}

func checkNTPClient(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	ntp := getContainerPath(tree, "environment", "ntp")
	if !configEnabled(ntp, false) {
		if severity, ok := clockNoSyncSeverity(platform); ok {
			return []diagnostic.Diagnostic{{
				Code:     "doctor-clock-no-sync",
				Severity: severity,
				Message:  clockNoSyncMessage(platform),
				Path:     "environment/ntp/enabled",
				Expected: "enabled Ze NTP or verified external clock synchronization",
				Actual:   "Ze NTP disabled",
			}}
		}
		return nil
	}

	var diags []diagnostic.Diagnostic

	servers := ntp.GetListOrdered("server")
	reachable := false
	checked := false
	for _, s := range servers {
		addr, ok := s.Value.Get("address")
		if !ok || addr == "" {
			continue
		}
		checked = true
		if ntpServerReachable(net.JoinHostPort(addr, "123"), 3*time.Second) {
			reachable = true
			break
		}
	}
	if checked && !reachable {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-ntp-server-unreachable",
			Severity: diagnostic.SeverityWarning,
			Message:  "none of the configured NTP servers are reachable",
		})
	}

	return diags
}

func clockNoSyncSeverity(platform *host.PlatformInfo) (diagnostic.Severity, bool) {
	if platform == nil {
		return "", false
	}
	switch platform.Type {
	case host.PlatformGokrazy:
		return diagnostic.SeverityError, true
	case host.PlatformSystemd, host.PlatformContainer, host.PlatformPlainLinux:
		return diagnostic.SeverityWarning, true
	default:
		return "", false
	}
}

func clockNoSyncMessage(platform *host.PlatformInfo) string {
	if platform != nil && platform.Type == host.PlatformGokrazy {
		return "gokrazy platform has no configured clock synchronization; enable environment/ntp because Ze owns appliance services"
	}
	if platform != nil {
		return "Ze NTP is disabled on " + platform.Type.String() + "; verify external clock synchronization or enable environment/ntp"
	}
	return "Ze NTP is disabled; verify external clock synchronization or enable environment/ntp"
}

func checkRPKIServers(tree *config.Tree) []diagnostic.Diagnostic {
	rpki := getContainerPath(tree, "bgp", "rpki")
	if rpki == nil {
		return nil
	}
	cacheServers := rpki.GetListOrdered("cache-server")
	if len(cacheServers) == 0 {
		return nil
	}

	checked := false
	for _, s := range cacheServers {
		port := valueOrDefault(s.Value, "port", "323")
		addr := s.Key
		if addr == "" {
			continue
		}
		checked = true
		if tcpReachable(net.JoinHostPort(addr, port), 3*time.Second) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-rpki-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured RPKI cache servers are reachable",
	}}
}

func checkBMPCollectors(tree *config.Tree) []diagnostic.Diagnostic {
	bmp := getContainerPath(tree, "bgp", "bmp", "sender")
	if bmp == nil {
		return nil
	}
	collectors := bmp.GetListOrdered("collector")
	if len(collectors) == 0 {
		return nil
	}

	checked := false
	for _, c := range collectors {
		addr, ok := c.Value.Get("address")
		if !ok || addr == "" {
			continue
		}
		checked = true
		port := valueOrDefault(c.Value, "port", "11019")
		if tcpReachable(net.JoinHostPort(addr, port), 3*time.Second) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-bmp-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured BMP collectors are reachable",
	}}
}

var ntpServerReachable = probeNTPServer

func probeNTPServer(addr string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()
	if deadlineErr := conn.SetDeadline(time.Now().Add(timeout)); deadlineErr != nil {
		return false
	}
	req := make([]byte, 48)
	req[0] = 0x1B // SNTP: LI=0, VN=3, Mode=3 (client)
	if _, writeErr := conn.Write(req); writeErr != nil {
		return false
	}
	resp := make([]byte, 48)
	_, readErr := conn.Read(resp)
	return readErr == nil
}

var probeWritable = probeWritableDir

func probeWritableDir(dir string) error {
	if dir == "" {
		return errors.New("empty path")
	}
	f, err := os.CreateTemp(dir, ".ze-doctor-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

var httpHead = defaultHTTPHead

func defaultHTTPHead(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func checkUpdateCheckURL(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	uc := getContainerPath(tree, "system", "update-check")
	if uc == nil {
		return nil
	}
	if platform != nil && platform.Type == host.PlatformGokrazy {
		return nil
	}
	url, ok := uc.Get("url")
	if !ok || url == "" {
		return nil
	}

	if err := httpHead(url, 5*time.Second); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-update-check-unreachable",
			Severity: diagnostic.SeverityWarning,
			Message:  "update-check URL unreachable: " + err.Error(),
			Path:     url,
		}}
	}
	return nil
}

func checkUpdateBackendConfig(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil || platform.Type != host.PlatformGokrazy {
		return nil
	}
	uc := getContainerPath(tree, "system", "update-check")
	if uc == nil {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-config-platform-mismatch",
		Severity: diagnostic.SeverityWarning,
		Message:  "system update-check config is ignored on gokrazy; image updates are managed by gokrazy",
		Path:     "system/update-check",
	}}
}

func checkArchiveDestinations(tree *config.Tree) []diagnostic.Diagnostic {
	system := tree.GetContainer("system")
	if system == nil {
		return nil
	}
	archives := system.GetListOrdered("archive")
	if len(archives) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, a := range archives {
		loc, ok := a.Value.Get("location")
		if !ok || loc == "" {
			continue
		}
		if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") {
			continue
		}
		if err := httpHead(loc, 5*time.Second); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-archive-unreachable",
				Severity: diagnostic.SeverityWarning,
				Message:  "archive " + a.Key + ": location unreachable: " + err.Error(),
				Path:     loc,
			})
		}
	}
	return diags
}

func checkWritableDestinations(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	if ntp := getContainerPath(tree, "environment", "ntp"); configEnabled(ntp, false) {
		persistPath := valueOrDefault(ntp, "persist-path", defaultNTPPersistPath)
		if persistPath != "" {
			diags = append(diags, checkNTPPersistPath(persistPath, platform)...)
			dir := filepath.Dir(persistPath)
			if err := probeWritable(dir); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-write-destination",
					Severity: diagnostic.SeverityWarning,
					Message:  "NTP persist-path parent not writable: " + dir,
					Path:     persistPath,
				})
			}
		}
	}

	if bfd := tree.GetContainer("bfd"); bfd != nil {
		if persistDir, ok := bfd.Get("persist-dir"); ok && persistDir != "" {
			if err := probeWritable(persistDir); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-write-destination",
					Severity: diagnostic.SeverityWarning,
					Message:  "BFD persist-dir not writable: " + persistDir,
					Path:     persistDir,
				})
			}
		}
	}

	if dns := getContainerPath(tree, "system", "dns"); dns != nil {
		if rcPath, ok := dns.Get("resolv-conf-path"); ok && rcPath != "" {
			dir := filepath.Dir(rcPath)
			if err := probeWritable(dir); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-write-destination",
					Severity: diagnostic.SeverityWarning,
					Message:  "DNS resolv-conf-path parent not writable: " + dir,
					Path:     rcPath,
				})
			}
		}
	}

	if system := tree.GetContainer("system"); system != nil {
		for _, a := range system.GetListOrdered("archive") {
			loc, ok := a.Value.Get("location")
			if !ok || loc == "" {
				continue
			}
			if !strings.HasPrefix(loc, "file://") {
				continue
			}
			path := strings.TrimPrefix(loc, "file://")
			if path == "" {
				continue
			}
			if err := probeWritable(path); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-write-destination",
					Severity: diagnostic.SeverityWarning,
					Message:  "archive " + a.Key + ": file location not writable: " + path,
					Path:     path,
				})
			}
		}
	}

	if uc := getContainerPath(tree, "system", "update-check"); uc != nil {
		autoApply, _ := uc.Get("auto-apply")
		if autoApply == configTrueValue {
			if platform != nil && platform.Type == host.PlatformGokrazy {
				return diags
			}
			if exe, err := os.Executable(); err == nil {
				dir := filepath.Dir(exe)
				if writeErr := probeWritable(dir); writeErr != nil {
					diags = append(diags, diagnostic.Diagnostic{
						Code:     "doctor-write-destination",
						Severity: diagnostic.SeverityWarning,
						Message:  "self-update auto-apply: binary parent not writable: " + dir,
						Path:     exe,
					})
				}
			}
		}
	}

	return diags
}

func checkNTPPersistPath(persistPath string, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil || persistPath == "" || !strings.HasPrefix(persistPath, "/perm/") {
		return nil
	}
	if platform.Type == host.PlatformGokrazy || platform.Type == host.PlatformUnknown || platform.Type == host.PlatformDarwin {
		return nil
	}
	return []diagnostic.Diagnostic{platformMismatch(
		"NTP persist-path uses gokrazy /perm storage on "+platform.Type.String(),
		"environment/ntp/persist-path",
		"non-/perm writable path for "+platform.Type.String(),
		persistPath,
	)}
}

func checkResolvConfPath(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil {
		return nil
	}
	path := effectiveResolvConfPath(tree)
	if path == "" {
		return nil
	}
	switch platform.Type {
	case host.PlatformGokrazy:
		if strings.HasPrefix(path, "/etc/") {
			return []diagnostic.Diagnostic{platformMismatch(
				"DNS resolv-conf-path points at read-only gokrazy root filesystem",
				"system/dns/resolv-conf-path",
				gokrazyResolvConfPath,
				path,
			)}
		}
	case host.PlatformSystemd, host.PlatformPlainLinux:
		if path == gokrazyResolvConfPath {
			return []diagnostic.Diagnostic{platformMismatch(
				"DNS resolv-conf-path uses gokrazy default on "+platform.Type.String(),
				"system/dns/resolv-conf-path",
				linuxResolvConfPath,
				path,
			)}
		}
	default:
		return nil
	}
	return nil
}

func effectiveResolvConfPath(tree *config.Tree) string {
	path := gokrazyResolvConfPath
	if dns := getContainerPath(tree, "system", "dns"); dns != nil {
		if value, ok := dns.Get("resolv-conf-path"); ok {
			path = value
		}
	}
	return path
}

func platformMismatch(message, path, expected, actual string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     "doctor-config-platform-mismatch",
		Severity: diagnostic.SeverityWarning,
		Message:  message,
		Path:     path,
		Expected: expected,
		Actual:   actual,
	}
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
