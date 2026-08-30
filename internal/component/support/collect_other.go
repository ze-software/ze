// Design: docs/architecture/core-design.md — non-Linux support collector stubs

//go:build !linux

package support

import "time"

func unavailable() (any, error) {
	return map[string]any{keyAvailable: false, keyReason: "requires Linux"}, nil
}

func collectDmesgInfo(_ time.Time) (any, error) { return unavailable() }
func collectSocketsInfo() (any, error)          { return unavailable() }
func collectKernelModulesInfo() (any, error)    { return unavailable() }
func collectConntrackInfo() (any, error)        { return unavailable() }
func collectFDsInfo() (any, error)              { return unavailable() }
func collectDNSInfo() (any, error)              { return unavailable() }
func collectFirewallInfo() (any, error)         { return unavailable() }
