// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type snmp6Collector struct {
	fs       procfs.FS
	interval time.Duration

	sysIPv6    metrics.GaugeVec
	packets    metrics.GaugeVec
	errors     metrics.GaugeVec
	udpPackets metrics.GaugeVec
	udpErrors  metrics.GaugeVec
	mcast      metrics.GaugeVec
	fragsOut   metrics.GaugeVec

	prev  procfs.ProcSnmp6
	first bool
}

func newSNMP6Collector(fs procfs.FS, interval time.Duration) *snmp6Collector {
	return &snmp6Collector{fs: fs, interval: interval, first: true}
}

func (c *snmp6Collector) Name() string { return "snmp6" }

func (c *snmp6Collector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.sysIPv6 = reg.GaugeVec(prefix+"_system_ipv6_kilobits_persec_average", "System IPv6 Bandwidth", labels)
	c.packets = reg.GaugeVec(prefix+"_ipv6_packets_packets_persec_average", "IPv6 Packets", labels)
	c.errors = reg.GaugeVec(prefix+"_ipv6_errors_packets_persec_average", "IPv6 Errors", labels)
	c.udpPackets = reg.GaugeVec(prefix+"_ipv6_udppackets_packets_persec_average", "IPv6 UDP Packets", labels)
	c.udpErrors = reg.GaugeVec(prefix+"_ipv6_udperrors_events_persec_average", "IPv6 UDP Errors", labels)
	c.mcast = reg.GaugeVec(prefix+"_ipv6_mcast_kilobits_persec_average", "IPv6 Multicast Bandwidth", labels)
	c.fragsOut = reg.GaugeVec(prefix+"_ipv6_fragsout_packets_persec_average", "IPv6 Fragments Out", labels)
}

func (c *snmp6Collector) Collect() error {
	self, err := c.fs.Self()
	if err != nil {
		return err
	}
	cur, err := self.Snmp6()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	c.sysIPv6.With("system.ipv6", "received", "network").Set(safeDeltaF64(cur.Ip6.InOctets, c.prev.Ip6.InOctets) * 8 / 1000 / secs)
	c.sysIPv6.With("system.ipv6", "sent", "network").Set(safeDeltaF64(cur.Ip6.OutOctets, c.prev.Ip6.OutOctets) * 8 / 1000 / secs)

	c.packets.With("ipv6.packets", "received", "packets").Set(safeDeltaF64(cur.Ip6.InReceives, c.prev.Ip6.InReceives) / secs)
	c.packets.With("ipv6.packets", "sent", "packets").Set(safeDeltaF64(cur.Ip6.OutRequests, c.prev.Ip6.OutRequests) / secs)
	c.packets.With("ipv6.packets", "forwarded", "packets").Set(safeDeltaF64(cur.Ip6.OutForwDatagrams, c.prev.Ip6.OutForwDatagrams) / secs)
	c.packets.With("ipv6.packets", "delivers", "packets").Set(safeDeltaF64(cur.Ip6.InDelivers, c.prev.Ip6.InDelivers) / secs)

	c.errors.With("ipv6.errors", "InDiscards", "errors").Set(safeDeltaF64(cur.Ip6.InDiscards, c.prev.Ip6.InDiscards) / secs)
	c.errors.With("ipv6.errors", "OutDiscards", "errors").Set(safeDeltaF64(cur.Ip6.OutDiscards, c.prev.Ip6.OutDiscards) / secs)
	c.errors.With("ipv6.errors", "InHdrErrors", "errors").Set(safeDeltaF64(cur.Ip6.InHdrErrors, c.prev.Ip6.InHdrErrors) / secs)
	c.errors.With("ipv6.errors", "InAddrErrors", "errors").Set(safeDeltaF64(cur.Ip6.InAddrErrors, c.prev.Ip6.InAddrErrors) / secs)
	c.errors.With("ipv6.errors", "InUnknownProtos", "errors").Set(safeDeltaF64(cur.Ip6.InUnknownProtos, c.prev.Ip6.InUnknownProtos) / secs)
	c.errors.With("ipv6.errors", "InTooBigErrors", "errors").Set(safeDeltaF64(cur.Ip6.InTooBigErrors, c.prev.Ip6.InTooBigErrors) / secs)
	c.errors.With("ipv6.errors", "InTruncatedPkts", "errors").Set(safeDeltaF64(cur.Ip6.InTruncatedPkts, c.prev.Ip6.InTruncatedPkts) / secs)
	c.errors.With("ipv6.errors", "InNoRoutes", "errors").Set(safeDeltaF64(cur.Ip6.InNoRoutes, c.prev.Ip6.InNoRoutes) / secs)
	c.errors.With("ipv6.errors", "OutNoRoutes", "errors").Set(safeDeltaF64(cur.Ip6.OutNoRoutes, c.prev.Ip6.OutNoRoutes) / secs)

	c.udpPackets.With("ipv6.udppackets", "received", "udp6").Set(safeDeltaF64(cur.Udp6.InDatagrams, c.prev.Udp6.InDatagrams) / secs)
	c.udpPackets.With("ipv6.udppackets", "sent", "udp6").Set(safeDeltaF64(cur.Udp6.OutDatagrams, c.prev.Udp6.OutDatagrams) / secs)

	c.udpErrors.With("ipv6.udperrors", "RcvbufErrors", "udp6").Set(safeDeltaF64(cur.Udp6.RcvbufErrors, c.prev.Udp6.RcvbufErrors) / secs)
	c.udpErrors.With("ipv6.udperrors", "SndbufErrors", "udp6").Set(safeDeltaF64(cur.Udp6.SndbufErrors, c.prev.Udp6.SndbufErrors) / secs)
	c.udpErrors.With("ipv6.udperrors", "InErrors", "udp6").Set(safeDeltaF64(cur.Udp6.InErrors, c.prev.Udp6.InErrors) / secs)
	c.udpErrors.With("ipv6.udperrors", "NoPorts", "udp6").Set(safeDeltaF64(cur.Udp6.NoPorts, c.prev.Udp6.NoPorts) / secs)
	c.udpErrors.With("ipv6.udperrors", "InCsumErrors", "udp6").Set(safeDeltaF64(cur.Udp6.InCsumErrors, c.prev.Udp6.InCsumErrors) / secs)

	c.mcast.With("ipv6.mcast", "received", "multicast6").Set(safeDeltaF64(cur.Ip6.InMcastOctets, c.prev.Ip6.InMcastOctets) * 8 / 1000 / secs)
	c.mcast.With("ipv6.mcast", "sent", "multicast6").Set(safeDeltaF64(cur.Ip6.OutMcastOctets, c.prev.Ip6.OutMcastOctets) * 8 / 1000 / secs)

	c.fragsOut.With("ipv6.fragsout", "ok", "fragments6").Set(safeDeltaF64(cur.Ip6.FragOKs, c.prev.Ip6.FragOKs) / secs)
	c.fragsOut.With("ipv6.fragsout", "failed", "fragments6").Set(safeDeltaF64(cur.Ip6.FragFails, c.prev.Ip6.FragFails) / secs)
	c.fragsOut.With("ipv6.fragsout", "all", "fragments6").Set(safeDeltaF64(cur.Ip6.FragCreates, c.prev.Ip6.FragCreates) / secs)

	c.prev = cur
	return nil
}
