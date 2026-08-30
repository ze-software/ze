// Design: docs/architecture/core-design.md — sysctl stub for non-Linux

//go:build !linux

package support

func collectSysctlInfo() (any, error) {
	return map[string]any{keyAvailable: false, keyReason: "sysctl collection requires Linux"}, nil
}
