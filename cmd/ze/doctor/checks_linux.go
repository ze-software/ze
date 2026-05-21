// Design: docs/features/ai-first.md — Linux-specific readiness checks

//go:build linux

package doctor

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

const defaultVPPSocket = "/run/vpp/api.sock"

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

	if tree != nil {
		ifaceBlock := tree.GetContainer("interface")
		if ifaceBlock != nil {
			backend, _ := ifaceBlock.Get("backend")
			if backend == "vpp" {
				required = append(required, "vhost_net")
			}
		}

		ipsecBlock := tree.GetContainer("ipsec")
		if ipsecBlock != nil {
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
	return diags
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
