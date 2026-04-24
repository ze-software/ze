// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type netIfaceCollector struct {
	speed   metrics.GaugeVec
	duplex  metrics.GaugeVec
	operst  metrics.GaugeVec
	carrier metrics.GaugeVec
	mtu     metrics.GaugeVec
}

func newNetIfaceCollector() *netIfaceCollector {
	return &netIfaceCollector{}
}

func (c *netIfaceCollector) Name() string { return "netiface" }

func (c *netIfaceCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.speed = reg.GaugeVec(prefix+"_net_speed_kilobits_persec_average", "Interface Speed", labels)
	c.duplex = reg.GaugeVec(prefix+"_net_duplex_state_average", "Interface Duplex", labels)
	c.operst = reg.GaugeVec(prefix+"_net_operstate_state_average", "Interface Oper State", labels)
	c.carrier = reg.GaugeVec(prefix+"_net_carrier_state_average", "Interface Carrier", labels)
	c.mtu = reg.GaugeVec(prefix+"_net_mtu_octets_average", "Interface MTU", labels)
}

func (c *netIfaceCollector) Collect() error {
	ifaces, err := listNetIfaces()
	if err != nil {
		return err
	}

	for _, iface := range ifaces {
		base := filepath.Join("/sys/class/net", iface) //nolint:gocritic // sysfs path construction
		chart := "net." + iface
		family := iface

		if v := readSysInt(filepath.Join(base, "speed")); v > 0 {
			c.speed.With(chart, "speed", family).Set(float64(v) * 1000)
		} else {
			c.speed.With(chart, "speed", family).Set(0)
		}

		c.mtu.With(chart, "mtu", family).Set(float64(readSysInt(filepath.Join(base, "mtu"))))
		c.carrier.With(chart, "carrier", family).Set(float64(readSysInt(filepath.Join(base, "carrier"))))

		duplex := readSysStr(filepath.Join(base, "duplex"))
		switch duplex {
		case "full":
			c.duplex.With(chart, "full", family).Set(1)
			c.duplex.With(chart, "half", family).Set(0)
			c.duplex.With(chart, "unknown", family).Set(0)
		case "half":
			c.duplex.With(chart, "full", family).Set(0)
			c.duplex.With(chart, "half", family).Set(1)
			c.duplex.With(chart, "unknown", family).Set(0)
		default:
			c.duplex.With(chart, "full", family).Set(0)
			c.duplex.With(chart, "half", family).Set(0)
			c.duplex.With(chart, "unknown", family).Set(1)
		}

		operstate := readSysStr(filepath.Join(base, "operstate"))
		c.operst.With(chart, "up", family).Set(boolF(operstate == "up"))
		c.operst.With(chart, "down", family).Set(boolF(operstate == "down"))
	}

	return nil
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func listNetIfaces() ([]string, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, err
	}
	var ifaces []string
	for _, e := range entries {
		name := e.Name()
		if name == "lo" {
			continue
		}
		ifaces = append(ifaces, name)
	}
	return ifaces, nil
}
