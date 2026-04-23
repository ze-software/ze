// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type snmpCollector struct {
	fs       procfs.FS
	interval time.Duration

	ipv4Packets    metrics.GaugeVec
	ipv4Errors     metrics.GaugeVec
	tcpPackets     metrics.GaugeVec
	tcpErrors      metrics.GaugeVec
	tcpHandshake   metrics.GaugeVec
	tcpConnections metrics.GaugeVec
	udpPackets     metrics.GaugeVec
	udpErrors      metrics.GaugeVec
	icmp           metrics.GaugeVec
	icmpMsg        metrics.GaugeVec
	ipFragsOut     metrics.GaugeVec
	ipFragsIn      metrics.GaugeVec

	prev  procfs.ProcSnmp
	first bool
}

func newSNMPCollector(fs procfs.FS, interval time.Duration) *snmpCollector {
	return &snmpCollector{fs: fs, interval: interval, first: true}
}

func (c *snmpCollector) Name() string { return "snmp" }

func (c *snmpCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.ipv4Packets = reg.GaugeVec(prefix+"_ipv4_packets_packets_persec_average", "IPv4 Packets", labels)
	c.ipv4Errors = reg.GaugeVec(prefix+"_ipv4_errors_packets_persec_average", "IPv4 Errors", labels)
	c.tcpPackets = reg.GaugeVec(prefix+"_ipv4_tcppackets_packets_persec_average", "TCP Packets", labels)
	c.tcpErrors = reg.GaugeVec(prefix+"_ipv4_tcperrors_events_persec_average", "TCP Errors", labels)
	c.tcpHandshake = reg.GaugeVec(prefix+"_ipv4_tcphandshake_events_persec_average", "TCP Handshake", labels)
	c.tcpConnections = reg.GaugeVec(prefix+"_ipv4_tcpsock_active_connections_average", "TCP Active Connections", labels)
	c.udpPackets = reg.GaugeVec(prefix+"_ipv4_udppackets_packets_persec_average", "UDP Packets", labels)
	c.udpErrors = reg.GaugeVec(prefix+"_ipv4_udperrors_events_persec_average", "UDP Errors", labels)
	c.icmp = reg.GaugeVec(prefix+"_ipv4_icmp_packets_persec_average", "ICMP Packets", labels)
	c.icmpMsg = reg.GaugeVec(prefix+"_ipv4_icmpmsg_packets_persec_average", "ICMP Messages", labels)
	c.ipFragsOut = reg.GaugeVec(prefix+"_ipv4_fragsout_packets_persec_average", "IPv4 Fragments Sent", labels)
	c.ipFragsIn = reg.GaugeVec(prefix+"_ipv4_fragsin_packets_persec_average", "IPv4 Fragments Reassembly", labels)
}

func ptrVal(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func (c *snmpCollector) Collect() error {
	self, err := c.fs.Self()
	if err != nil {
		return err
	}
	cur, err := self.Snmp()
	if err != nil {
		return err
	}

	c.tcpConnections.With("ipv4.tcpsock", "connections", "tcp").Set(ptrVal(cur.Tcp.CurrEstab))

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	p := c.prev

	c.ipv4Packets.With("ipv4.packets", "received", "packets").Set(safeDeltaF64(cur.Ip.InReceives, p.Ip.InReceives) / secs)
	c.ipv4Packets.With("ipv4.packets", "sent", "packets").Set(safeDeltaF64(cur.Ip.OutRequests, p.Ip.OutRequests) / secs)
	c.ipv4Packets.With("ipv4.packets", "forwarded", "packets").Set(safeDeltaF64(cur.Ip.ForwDatagrams, p.Ip.ForwDatagrams) / secs)
	c.ipv4Packets.With("ipv4.packets", "delivered", "packets").Set(safeDeltaF64(cur.Ip.InDelivers, p.Ip.InDelivers) / secs)

	c.ipv4Errors.With("ipv4.errors", "InDiscards", "errors").Set(safeDeltaF64(cur.Ip.InDiscards, p.Ip.InDiscards) / secs)
	c.ipv4Errors.With("ipv4.errors", "OutDiscards", "errors").Set(safeDeltaF64(cur.Ip.OutDiscards, p.Ip.OutDiscards) / secs)
	c.ipv4Errors.With("ipv4.errors", "InHdrErrors", "errors").Set(safeDeltaF64(cur.Ip.InHdrErrors, p.Ip.InHdrErrors) / secs)
	c.ipv4Errors.With("ipv4.errors", "InAddrErrors", "errors").Set(safeDeltaF64(cur.Ip.InAddrErrors, p.Ip.InAddrErrors) / secs)
	c.ipv4Errors.With("ipv4.errors", "InUnknownProtos", "errors").Set(safeDeltaF64(cur.Ip.InUnknownProtos, p.Ip.InUnknownProtos) / secs)
	c.ipv4Errors.With("ipv4.errors", "OutNoRoutes", "errors").Set(safeDeltaF64(cur.Ip.OutNoRoutes, p.Ip.OutNoRoutes) / secs)

	c.tcpPackets.With("ipv4.tcppackets", "received", "tcp").Set(safeDeltaF64(cur.Tcp.InSegs, p.Tcp.InSegs) / secs)
	c.tcpPackets.With("ipv4.tcppackets", "sent", "tcp").Set(safeDeltaF64(cur.Tcp.OutSegs, p.Tcp.OutSegs) / secs)

	c.tcpErrors.With("ipv4.tcperrors", "InErrs", "tcp").Set(safeDeltaF64(cur.Tcp.InErrs, p.Tcp.InErrs) / secs)
	c.tcpErrors.With("ipv4.tcperrors", "InCsumErrors", "tcp").Set(safeDeltaF64(cur.Tcp.InCsumErrors, p.Tcp.InCsumErrors) / secs)
	c.tcpErrors.With("ipv4.tcperrors", "RetransSegs", "tcp").Set(safeDeltaF64(cur.Tcp.RetransSegs, p.Tcp.RetransSegs) / secs)

	c.tcpHandshake.With("ipv4.tcphandshake", "EstabResets", "tcp").Set(safeDeltaF64(cur.Tcp.EstabResets, p.Tcp.EstabResets) / secs)
	c.tcpHandshake.With("ipv4.tcphandshake", "ActiveOpens", "tcp").Set(safeDeltaF64(cur.Tcp.ActiveOpens, p.Tcp.ActiveOpens) / secs)
	c.tcpHandshake.With("ipv4.tcphandshake", "PassiveOpens", "tcp").Set(safeDeltaF64(cur.Tcp.PassiveOpens, p.Tcp.PassiveOpens) / secs)
	c.tcpHandshake.With("ipv4.tcphandshake", "AttemptFails", "tcp").Set(safeDeltaF64(cur.Tcp.AttemptFails, p.Tcp.AttemptFails) / secs)

	c.udpPackets.With("ipv4.udppackets", "received", "udp").Set(safeDeltaF64(cur.Udp.InDatagrams, p.Udp.InDatagrams) / secs)
	c.udpPackets.With("ipv4.udppackets", "sent", "udp").Set(safeDeltaF64(cur.Udp.OutDatagrams, p.Udp.OutDatagrams) / secs)

	c.udpErrors.With("ipv4.udperrors", "InErrors", "udp").Set(safeDeltaF64(cur.Udp.InErrors, p.Udp.InErrors) / secs)
	c.udpErrors.With("ipv4.udperrors", "NoPorts", "udp").Set(safeDeltaF64(cur.Udp.NoPorts, p.Udp.NoPorts) / secs)
	c.udpErrors.With("ipv4.udperrors", "RcvbufErrors", "udp").Set(safeDeltaF64(cur.Udp.RcvbufErrors, p.Udp.RcvbufErrors) / secs)
	c.udpErrors.With("ipv4.udperrors", "SndbufErrors", "udp").Set(safeDeltaF64(cur.Udp.SndbufErrors, p.Udp.SndbufErrors) / secs)
	c.udpErrors.With("ipv4.udperrors", "InCsumErrors", "udp").Set(safeDeltaF64(cur.Udp.InCsumErrors, p.Udp.InCsumErrors) / secs)

	c.icmp.With("ipv4.icmp", "InMsgs", "icmp").Set(safeDeltaF64(cur.Icmp.InMsgs, p.Icmp.InMsgs) / secs)
	c.icmp.With("ipv4.icmp", "OutMsgs", "icmp").Set(safeDeltaF64(cur.Icmp.OutMsgs, p.Icmp.OutMsgs) / secs)
	c.icmp.With("ipv4.icmp", "InErrors", "icmp").Set(safeDeltaF64(cur.Icmp.InErrors, p.Icmp.InErrors) / secs)
	c.icmp.With("ipv4.icmp", "OutErrors", "icmp").Set(safeDeltaF64(cur.Icmp.OutErrors, p.Icmp.OutErrors) / secs)

	c.icmpMsg.With("ipv4.icmpmsg", "InType3", "icmp").Set(safeDeltaF64(cur.IcmpMsg.InType3, p.IcmpMsg.InType3) / secs)
	c.icmpMsg.With("ipv4.icmpmsg", "OutType3", "icmp").Set(safeDeltaF64(cur.IcmpMsg.OutType3, p.IcmpMsg.OutType3) / secs)

	c.ipFragsOut.With("ipv4.fragsout", "ok", "fragments").Set(safeDeltaF64(cur.Ip.FragOKs, p.Ip.FragOKs) / secs)
	c.ipFragsOut.With("ipv4.fragsout", "failed", "fragments").Set(safeDeltaF64(cur.Ip.FragFails, p.Ip.FragFails) / secs)
	c.ipFragsOut.With("ipv4.fragsout", "all", "fragments").Set(safeDeltaF64(cur.Ip.FragCreates, p.Ip.FragCreates) / secs)

	c.ipFragsIn.With("ipv4.fragsin", "ok", "fragments").Set(safeDeltaF64(cur.Ip.ReasmOKs, p.Ip.ReasmOKs) / secs)
	c.ipFragsIn.With("ipv4.fragsin", "failed", "fragments").Set(safeDeltaF64(cur.Ip.ReasmFails, p.Ip.ReasmFails) / secs)
	c.ipFragsIn.With("ipv4.fragsin", "all", "fragments").Set(safeDeltaF64(cur.Ip.ReasmReqds, p.Ip.ReasmReqds) / secs)

	c.prev = cur
	return nil
}
