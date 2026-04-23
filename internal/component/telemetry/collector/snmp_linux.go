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

	// TCP current connections (gauge, not rate)
	c.tcpConnections.With("ipv4.tcpsock", "connections", "tcp").Set(ptrVal(cur.Tcp.CurrEstab))

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	// IPv4 packets
	c.ipv4Packets.With("ipv4.packets", "received", "packets").Set(safeDeltaF64(cur.Ip.InReceives, c.prev.Ip.InReceives) / secs)
	c.ipv4Packets.With("ipv4.packets", "sent", "packets").Set(safeDeltaF64(cur.Ip.OutRequests, c.prev.Ip.OutRequests) / secs)
	c.ipv4Packets.With("ipv4.packets", "forwarded", "packets").Set(safeDeltaF64(cur.Ip.ForwDatagrams, c.prev.Ip.ForwDatagrams) / secs)
	c.ipv4Packets.With("ipv4.packets", "delivered", "packets").Set(safeDeltaF64(cur.Ip.InDelivers, c.prev.Ip.InDelivers) / secs)

	// IPv4 errors
	c.ipv4Errors.With("ipv4.errors", "InDiscards", "errors").Set(safeDeltaF64(cur.Ip.InDiscards, c.prev.Ip.InDiscards) / secs)
	c.ipv4Errors.With("ipv4.errors", "OutDiscards", "errors").Set(safeDeltaF64(cur.Ip.OutDiscards, c.prev.Ip.OutDiscards) / secs)
	c.ipv4Errors.With("ipv4.errors", "InHdrErrors", "errors").Set(safeDeltaF64(cur.Ip.InHdrErrors, c.prev.Ip.InHdrErrors) / secs)
	c.ipv4Errors.With("ipv4.errors", "InAddrErrors", "errors").Set(safeDeltaF64(cur.Ip.InAddrErrors, c.prev.Ip.InAddrErrors) / secs)
	c.ipv4Errors.With("ipv4.errors", "InUnknownProtos", "errors").Set(safeDeltaF64(cur.Ip.InUnknownProtos, c.prev.Ip.InUnknownProtos) / secs)
	c.ipv4Errors.With("ipv4.errors", "OutNoRoutes", "errors").Set(safeDeltaF64(cur.Ip.OutNoRoutes, c.prev.Ip.OutNoRoutes) / secs)

	// TCP packets
	c.tcpPackets.With("ipv4.tcppackets", "received", "tcp").Set(safeDeltaF64(cur.Tcp.InSegs, c.prev.Tcp.InSegs) / secs)
	c.tcpPackets.With("ipv4.tcppackets", "sent", "tcp").Set(safeDeltaF64(cur.Tcp.OutSegs, c.prev.Tcp.OutSegs) / secs)

	// TCP errors
	c.tcpErrors.With("ipv4.tcperrors", "InErrs", "tcp").Set(safeDeltaF64(cur.Tcp.InErrs, c.prev.Tcp.InErrs) / secs)
	c.tcpErrors.With("ipv4.tcperrors", "InCsumErrors", "tcp").Set(safeDeltaF64(cur.Tcp.InCsumErrors, c.prev.Tcp.InCsumErrors) / secs)
	c.tcpErrors.With("ipv4.tcperrors", "RetransSegs", "tcp").Set(safeDeltaF64(cur.Tcp.RetransSegs, c.prev.Tcp.RetransSegs) / secs)

	// TCP handshake
	c.tcpHandshake.With("ipv4.tcphandshake", "EstabResets", "tcp").Set(safeDeltaF64(cur.Tcp.EstabResets, c.prev.Tcp.EstabResets) / secs)
	c.tcpHandshake.With("ipv4.tcphandshake", "ActiveOpens", "tcp").Set(safeDeltaF64(cur.Tcp.ActiveOpens, c.prev.Tcp.ActiveOpens) / secs)
	c.tcpHandshake.With("ipv4.tcphandshake", "PassiveOpens", "tcp").Set(safeDeltaF64(cur.Tcp.PassiveOpens, c.prev.Tcp.PassiveOpens) / secs)
	c.tcpHandshake.With("ipv4.tcphandshake", "AttemptFails", "tcp").Set(safeDeltaF64(cur.Tcp.AttemptFails, c.prev.Tcp.AttemptFails) / secs)

	// UDP packets
	c.udpPackets.With("ipv4.udppackets", "received", "udp").Set(safeDeltaF64(cur.Udp.InDatagrams, c.prev.Udp.InDatagrams) / secs)
	c.udpPackets.With("ipv4.udppackets", "sent", "udp").Set(safeDeltaF64(cur.Udp.OutDatagrams, c.prev.Udp.OutDatagrams) / secs)

	// UDP errors
	c.udpErrors.With("ipv4.udperrors", "InErrors", "udp").Set(safeDeltaF64(cur.Udp.InErrors, c.prev.Udp.InErrors) / secs)
	c.udpErrors.With("ipv4.udperrors", "NoPorts", "udp").Set(safeDeltaF64(cur.Udp.NoPorts, c.prev.Udp.NoPorts) / secs)
	c.udpErrors.With("ipv4.udperrors", "RcvbufErrors", "udp").Set(safeDeltaF64(cur.Udp.RcvbufErrors, c.prev.Udp.RcvbufErrors) / secs)
	c.udpErrors.With("ipv4.udperrors", "SndbufErrors", "udp").Set(safeDeltaF64(cur.Udp.SndbufErrors, c.prev.Udp.SndbufErrors) / secs)
	c.udpErrors.With("ipv4.udperrors", "InCsumErrors", "udp").Set(safeDeltaF64(cur.Udp.InCsumErrors, c.prev.Udp.InCsumErrors) / secs)

	c.prev = cur
	return nil
}
