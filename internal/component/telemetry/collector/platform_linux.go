// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import "github.com/prometheus/procfs"

func registerPlatformCollectors(m *Manager) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		m.logger.Warn("procfs unavailable, OS collectors disabled", "error", err)
		return
	}

	m.Register(newCPUCollector(fs, m.interval))
	m.Register(newMemoryCollector(fs))
	m.Register(newLoadAvgCollector(fs))
	m.Register(newNetDevCollector(fs, m.interval))
	m.Register(newDiskStatsCollector(m.interval))
	m.Register(newStatCollector(fs, m.interval))
	m.Register(newUptimeCollector())
	m.Register(newEntropyCollector(fs))
	m.Register(newFileNRCollector())
	m.Register(newPressureCollector(fs))
	m.Register(newSockStatCollector(fs))
	m.Register(newSoftNetCollector(fs, m.interval))
	m.Register(newSNMPCollector(fs, m.interval))
	m.Register(newConntrackCollector(fs, m.interval))
	m.Register(newVMStatCollector(m.interval))
	m.Register(newSNMP6Collector(fs, m.interval))
	m.Register(newDiskSpaceCollector())
	m.Register(newSoftIRQsCollector(fs, m.interval))
	m.Register(newSockStat6Collector(fs))
	m.Register(newCPUFreqCollector(m.interval))
	m.Register(newNetstatCollector(fs, m.interval))
	m.Register(newSoftNetPerCPUCollector(fs, m.interval))
	m.Register(newNetIfaceCollector())
	m.Register(newCPUIdleCollector(m.interval))
	m.Register(newConntrackExpectCollector(m.interval))
	m.Register(newSCTPCollector(m.interval))
	m.Register(newIPVSCollector(fs, m.interval))
	m.Register(newWirelessCollector(fs))
	m.Register(newMDStatCollector(fs))
	m.Register(newZFSCollector(m.interval))
	m.Register(newBtrfsCollector())
}
