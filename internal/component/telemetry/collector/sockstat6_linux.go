// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type sockStat6Collector struct {
	fs procfs.FS

	tcpSockets  metrics.GaugeVec
	udpSockets  metrics.GaugeVec
	rawSockets  metrics.GaugeVec
	fragSockets metrics.GaugeVec
}

func newSockStat6Collector(fs procfs.FS) *sockStat6Collector {
	return &sockStat6Collector{fs: fs}
}

func (c *sockStat6Collector) Name() string { return "sockstat6" }

func (c *sockStat6Collector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.tcpSockets = reg.GaugeVec(prefix+"_ipv6_sockstat6_tcp_sockets_sockets_average", "IPv6 TCP Sockets", labels)
	c.udpSockets = reg.GaugeVec(prefix+"_ipv6_sockstat6_udp_sockets_sockets_average", "IPv6 UDP Sockets", labels)
	c.rawSockets = reg.GaugeVec(prefix+"_ipv6_sockstat6_raw_sockets_sockets_average", "IPv6 RAW Sockets", labels)
	c.fragSockets = reg.GaugeVec(prefix+"_ipv6_sockstat6_frag_sockets_fragments_average", "IPv6 Frag Sockets", labels)
}

func (c *sockStat6Collector) Collect() error {
	ss, err := c.fs.NetSockstat6()
	if err != nil {
		return err
	}

	for _, p := range ss.Protocols {
		switch p.Protocol {
		case "TCP6":
			c.tcpSockets.With("ipv6.sockstat6_tcp_sockets", "inuse", "tcp6").Set(float64(p.InUse))
		case "UDP6":
			c.udpSockets.With("ipv6.sockstat6_udp_sockets", "inuse", "udp6").Set(float64(p.InUse))
		case "RAW6":
			c.rawSockets.With("ipv6.sockstat6_raw_sockets", "inuse", "raw6").Set(float64(p.InUse))
		case "FRAG6":
			c.fragSockets.With("ipv6.sockstat6_frag_sockets", "inuse", "frag6").Set(float64(p.InUse))
		}
	}

	return nil
}
