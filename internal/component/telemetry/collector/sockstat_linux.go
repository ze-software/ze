// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type sockStatCollector struct {
	fs procfs.FS

	sockets    metrics.GaugeVec
	tcpSockets metrics.GaugeVec
	udpSockets metrics.GaugeVec
	tcpMem     metrics.GaugeVec
	udpMem     metrics.GaugeVec
}

func newSockStatCollector(fs procfs.FS) *sockStatCollector {
	return &sockStatCollector{fs: fs}
}

func (c *sockStatCollector) Name() string { return "sockstat" }

func (c *sockStatCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.sockets = reg.GaugeVec(prefix+"_ip_sockstat_sockets_sockets_average", "IP Sockets", labels)
	c.tcpSockets = reg.GaugeVec(prefix+"_ipv4_sockstat_tcp_sockets_sockets_average", "TCP Sockets", labels)
	c.udpSockets = reg.GaugeVec(prefix+"_ipv4_sockstat_udp_sockets_sockets_average", "UDP Sockets", labels)
	c.tcpMem = reg.GaugeVec(prefix+"_ipv4_sockstat_tcp_mem_KiB_average", "TCP Memory", labels)
	c.udpMem = reg.GaugeVec(prefix+"_ipv4_sockstat_udp_mem_KiB_average", "UDP Memory", labels)
}

func (c *sockStatCollector) Collect() error {
	ss, err := c.fs.NetSockstat()
	if err != nil {
		return err
	}

	if ss.Used != nil {
		c.sockets.With("ip.sockstat_sockets", "used", "sockets").Set(float64(*ss.Used))
	}

	for _, p := range ss.Protocols {
		switch p.Protocol {
		case "TCP":
			c.tcpSockets.With("ipv4.sockstat_tcp_sockets", "inuse", "tcp").Set(float64(p.InUse))
			if p.Orphan != nil {
				c.tcpSockets.With("ipv4.sockstat_tcp_sockets", "orphan", "tcp").Set(float64(*p.Orphan))
			}
			if p.TW != nil {
				c.tcpSockets.With("ipv4.sockstat_tcp_sockets", "timewait", "tcp").Set(float64(*p.TW))
			}
			if p.Alloc != nil {
				c.tcpSockets.With("ipv4.sockstat_tcp_sockets", "alloc", "tcp").Set(float64(*p.Alloc))
			}
			if p.Mem != nil {
				// Mem is in pages, convert to KiB (page = 4096 bytes typically)
				c.tcpMem.With("ipv4.sockstat_tcp_mem", "mem", "tcp").Set(float64(*p.Mem) * 4)
			}
		case "UDP":
			c.udpSockets.With("ipv4.sockstat_udp_sockets", "inuse", "udp").Set(float64(p.InUse))
			if p.Mem != nil {
				c.udpMem.With("ipv4.sockstat_udp_mem", "mem", "udp").Set(float64(*p.Mem) * 4)
			}
		}
	}

	return nil
}
