// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type netstatCollector struct {
	fs       procfs.FS
	interval time.Duration

	sysIPv4     metrics.GaugeVec
	mcast       metrics.GaugeVec
	mcastPkts   metrics.GaugeVec
	bcast       metrics.GaugeVec
	bcastPkts   metrics.GaugeVec
	ecn         metrics.GaugeVec
	tcpAborts   metrics.GaugeVec
	tcpMemPres  metrics.GaugeVec
	tcpReorders metrics.GaugeVec
	tcpOFO      metrics.GaugeVec

	prev  procfs.ProcNetstat
	first bool
}

func newNetstatCollector(fs procfs.FS, interval time.Duration) *netstatCollector {
	return &netstatCollector{fs: fs, interval: interval, first: true}
}

func (c *netstatCollector) Name() string { return "netstat" }

//nolint:dupl // distinct metric registrations share GaugeVec pattern
func (c *netstatCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.sysIPv4 = reg.GaugeVec(prefix+"_system_ipv4_kilobits_persec_average", "System IPv4 Bandwidth", labels)
	c.mcast = reg.GaugeVec(prefix+"_ipv4_mcast_kilobits_persec_average", "IPv4 Multicast Bandwidth", labels)
	c.mcastPkts = reg.GaugeVec(prefix+"_ipv4_mcastpkts_packets_persec_average", "IPv4 Multicast Packets", labels)
	c.bcast = reg.GaugeVec(prefix+"_ipv4_bcast_kilobits_persec_average", "IPv4 Broadcast Bandwidth", labels)
	c.bcastPkts = reg.GaugeVec(prefix+"_ipv4_bcastpkts_packets_persec_average", "IPv4 Broadcast Packets", labels)
	c.ecn = reg.GaugeVec(prefix+"_ipv4_ecnpkts_packets_persec_average", "IPv4 ECN Packets", labels)
	c.tcpAborts = reg.GaugeVec(prefix+"_ip_tcpconnaborts_connections_persec_average", "TCP Connection Aborts", labels)
	c.tcpMemPres = reg.GaugeVec(prefix+"_ip_tcpmemorypressures_events_persec_average", "TCP Memory Pressures", labels)
	c.tcpReorders = reg.GaugeVec(prefix+"_ip_tcpreorders_packets_persec_average", "TCP Reorders", labels)
	c.tcpOFO = reg.GaugeVec(prefix+"_ip_tcpofo_packets_persec_average", "TCP Out-of-Order", labels)
}

func (c *netstatCollector) Collect() error {
	self, err := c.fs.Self()
	if err != nil {
		return err
	}
	cur, err := self.Netstat()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	p := c.prev

	c.sysIPv4.With("system.ipv4", "received", "network").Set(safeDeltaF64(cur.IpExt.InOctets, p.IpExt.InOctets) * 8 / 1000 / secs)
	c.sysIPv4.With("system.ipv4", "sent", "network").Set(safeDeltaF64(cur.IpExt.OutOctets, p.IpExt.OutOctets) * 8 / 1000 / secs)

	c.mcast.With("ipv4.mcast", "received", "multicast").Set(safeDeltaF64(cur.IpExt.InMcastOctets, p.IpExt.InMcastOctets) * 8 / 1000 / secs)
	c.mcast.With("ipv4.mcast", "sent", "multicast").Set(safeDeltaF64(cur.IpExt.OutMcastOctets, p.IpExt.OutMcastOctets) * 8 / 1000 / secs)

	c.mcastPkts.With("ipv4.mcastpkts", "received", "multicast").Set(safeDeltaF64(cur.IpExt.InMcastPkts, p.IpExt.InMcastPkts) / secs)
	c.mcastPkts.With("ipv4.mcastpkts", "sent", "multicast").Set(safeDeltaF64(cur.IpExt.OutMcastPkts, p.IpExt.OutMcastPkts) / secs)

	c.bcast.With("ipv4.bcast", "received", "broadcast").Set(safeDeltaF64(cur.IpExt.InBcastOctets, p.IpExt.InBcastOctets) * 8 / 1000 / secs)
	c.bcast.With("ipv4.bcast", "sent", "broadcast").Set(safeDeltaF64(cur.IpExt.OutBcastOctets, p.IpExt.OutBcastOctets) * 8 / 1000 / secs)

	c.bcastPkts.With("ipv4.bcastpkts", "received", "broadcast").Set(safeDeltaF64(cur.IpExt.InBcastPkts, p.IpExt.InBcastPkts) / secs)
	c.bcastPkts.With("ipv4.bcastpkts", "sent", "broadcast").Set(safeDeltaF64(cur.IpExt.OutBcastPkts, p.IpExt.OutBcastPkts) / secs)

	c.ecn.With("ipv4.ecnpkts", "InCEPkts", "ecn").Set(safeDeltaF64(cur.IpExt.InCEPkts, p.IpExt.InCEPkts) / secs)
	c.ecn.With("ipv4.ecnpkts", "InNoECTPkts", "ecn").Set(safeDeltaF64(cur.IpExt.InNoECTPkts, p.IpExt.InNoECTPkts) / secs)
	c.ecn.With("ipv4.ecnpkts", "InECT0Pkts", "ecn").Set(safeDeltaF64(cur.IpExt.InECT0Pkts, p.IpExt.InECT0Pkts) / secs)
	c.ecn.With("ipv4.ecnpkts", "InECT1Pkts", "ecn").Set(safeDeltaF64(cur.IpExt.InECT1Pkts, p.IpExt.InECT1Pkts) / secs)

	c.tcpAborts.With("ip.tcpconnaborts", "TCPAbortOnData", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPAbortOnData, p.TcpExt.TCPAbortOnData) / secs)
	c.tcpAborts.With("ip.tcpconnaborts", "TCPAbortOnClose", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPAbortOnClose, p.TcpExt.TCPAbortOnClose) / secs)
	c.tcpAborts.With("ip.tcpconnaborts", "TCPAbortOnMemory", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPAbortOnMemory, p.TcpExt.TCPAbortOnMemory) / secs)
	c.tcpAborts.With("ip.tcpconnaborts", "TCPAbortOnTimeout", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPAbortOnTimeout, p.TcpExt.TCPAbortOnTimeout) / secs)
	c.tcpAborts.With("ip.tcpconnaborts", "TCPAbortOnLinger", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPAbortOnLinger, p.TcpExt.TCPAbortOnLinger) / secs)
	c.tcpAborts.With("ip.tcpconnaborts", "TCPAbortFailed", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPAbortFailed, p.TcpExt.TCPAbortFailed) / secs)

	c.tcpMemPres.With("ip.tcpmemorypressures", "TCPMemoryPressures", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPMemoryPressures, p.TcpExt.TCPMemoryPressures) / secs)

	c.tcpReorders.With("ip.tcpreorders", "TCPTSReorder", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPTSReorder, p.TcpExt.TCPTSReorder) / secs)
	c.tcpReorders.With("ip.tcpreorders", "TCPSACKReorder", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPSACKReorder, p.TcpExt.TCPSACKReorder) / secs)
	c.tcpReorders.With("ip.tcpreorders", "TCPRenoReorder", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPRenoReorder, p.TcpExt.TCPRenoReorder) / secs)

	c.tcpOFO.With("ip.tcpofo", "TCPOFOQueue", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPOFOQueue, p.TcpExt.TCPOFOQueue) / secs)
	c.tcpOFO.With("ip.tcpofo", "TCPOFODrop", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPOFODrop, p.TcpExt.TCPOFODrop) / secs)
	c.tcpOFO.With("ip.tcpofo", "TCPOFOMerge", "tcp").Set(safeDeltaF64(cur.TcpExt.TCPOFOMerge, p.TcpExt.TCPOFOMerge) / secs)
	c.tcpOFO.With("ip.tcpofo", "OfoPruned", "tcp").Set(safeDeltaF64(cur.TcpExt.OfoPruned, p.TcpExt.OfoPruned) / secs)

	c.prev = cur
	return nil
}
