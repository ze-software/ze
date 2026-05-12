// Design: plan/spec-host-3-smart.md — SMART health monitoring via smartctl
// Overview: smart.go — parseSMARTJSON and SmartInfo extraction
// Related: storage_linux.go — DetectStorage calls detectSMART

//go:build linux

package host

import (
	"context"
	"os/exec"
	"time"
)

const smartctlTimeout = 10 * time.Second

// smartctlBinary caches the resolved smartctl path. Checked once per
// process lifetime to avoid repeated LookPath calls on each device.
var smartctlBinary = resolveSmartctl()

func resolveSmartctl() string {
	p, err := exec.LookPath("smartctl")
	if err != nil {
		return ""
	}
	return p
}

// detectSMART runs smartctl for the named device and returns parsed
// SMART data. Returns nil when smartctl is not installed or when
// Root is set (testdata mode where /dev/ paths are not real).
func (d *Detector) detectSMART(deviceName string) *SmartInfo {
	if d.Root != "" {
		return nil
	}
	if smartctlBinary == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), smartctlTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, smartctlBinary, "--json=c", "--all", "/dev/"+deviceName)
	out, err := cmd.Output()
	if err != nil {
		if len(out) > 0 {
			info, parseErr := parseSMARTJSON(out)
			if parseErr == nil {
				return info
			}
		}
		return nil
	}

	info, err := parseSMARTJSON(out)
	if err != nil {
		return nil
	}
	return info
}
