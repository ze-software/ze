// Design: docs/architecture/iface/offload.md -- no-op stub for non-Linux

//go:build !linux

package iface

func applyOffloads(ifaceName string, cfg *offloadConfig) {}
func applyRFSGlobal(cfg *ifaceConfig)                    {}
