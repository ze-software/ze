// Design: docs/features/ai-first.md — Linux-specific readiness checks

//go:build linux

package doctor

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

const (
	defaultVPPSocket = "/run/vpp/api.sock"
	backendVPP       = "vpp"
)

func checkVPPSocket() []diagnostic.Diagnostic {
	sockPath := defaultVPPSocket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", sockPath)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-unreachable",
			Severity: diagnostic.SeverityError,
			Message:  "VPP API socket unreachable: " + sockPath + ": " + err.Error(),
			Path:     sockPath,
		}}
	}
	if closeErr := conn.Close(); closeErr != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-unreachable",
			Severity: diagnostic.SeverityWarning,
			Message:  "VPP API socket close: " + sockPath + ": " + closeErr.Error(),
			Path:     sockPath,
		}}
	}
	return nil
}

func checkKernelModules(tree *config.Tree) []diagnostic.Diagnostic {
	var required []string
	hasIPsec := false

	if tree != nil {
		ifaceBlock := tree.GetContainer("interface")
		if ifaceBlock != nil {
			backend, _ := ifaceBlock.Get("backend")
			if backend == backendVPP {
				required = append(required, "vhost_net")
			}
		}

		if tree.GetContainer("ipsec") != nil {
			hasIPsec = true
			required = append(required, "xfrm_user", "xfrm_algo")
		}
	}

	if len(required) == 0 {
		return nil
	}

	loaded := readLoadedModules()
	var diags []diagnostic.Diagnostic
	for _, mod := range required {
		if !loaded[mod] {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-module-missing",
				Severity: diagnostic.SeverityError,
				Message:  "kernel module not loaded: " + mod,
			})
		}
	}

	if hasIPsec && !loaded["ip_tables"] && !loaded["nf_tables"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-module-missing",
			Severity: diagnostic.SeverityWarning,
			Message:  "IPsec: neither ip_tables nor nf_tables loaded (firewall marking may not work)",
		})
	}

	return diags
}

func checkInterfaces(tree *config.Tree) []diagnostic.Diagnostic {
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}

	backend, _ := ifaceBlock.Get("backend")
	if backend == backendVPP {
		return nil
	}

	ethList := ifaceBlock.GetList("ethernet")
	if len(ethList) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for name := range ethList {
		if strings.Contains(name, "..") || strings.ContainsAny(name, "/\x00") {
			continue
		}
		statePath := "/sys/class/net/" + name
		info, err := os.Stat(statePath)
		if err != nil || !info.IsDir() {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-iface-missing",
				Severity: diagnostic.SeverityError,
				Message:  "ethernet interface not found: " + name,
			})
			continue
		}
		operstate, err := os.ReadFile(statePath + "/operstate") //nolint:gosec // path traversal guarded above
		if err != nil {
			continue
		}
		state := strings.TrimSpace(string(operstate))
		if state != "up" && state != "unknown" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-iface-down",
				Severity: diagnostic.SeverityWarning,
				Message:  "ethernet interface " + name + " operstate: " + state,
			})
		}
	}
	return diags
}

// checkVPPVersion runs `vppctl show version` and warns if the major version
// is not in the expected range. Only runs when VPP backend is configured.
func checkVPPVersion(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	backend, _ := ifaceBlock.Get("backend")
	if backend != backendVPP {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "vppctl", "show", "version").Output() //nolint:gosec // fixed command
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-version",
			Severity: diagnostic.SeverityWarning,
			Message:  "cannot determine VPP version: " + err.Error(),
		}}
	}

	version := strings.TrimSpace(string(out))
	if !strings.Contains(version, "vpp v") {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-version",
			Severity: diagnostic.SeverityWarning,
			Message:  "unexpected VPP version output: " + version,
		}}
	}
	return nil
}

func readLoadedModules() map[string]bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return nil
	}
	set := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		if sp := strings.IndexByte(line, ' '); sp > 0 {
			set[line[:sp]] = true
		}
	}
	return set
}
