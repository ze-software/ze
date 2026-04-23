// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build !linux

package collector

func registerPlatformCollectors(_ *Manager) {}
