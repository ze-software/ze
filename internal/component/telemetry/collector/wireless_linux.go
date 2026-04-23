// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type wirelessCollector struct {
	fs procfs.FS

	signal  metrics.GaugeVec
	quality metrics.GaugeVec
	discard metrics.GaugeVec
	missed  metrics.GaugeVec
}

func newWirelessCollector(fs procfs.FS) *wirelessCollector {
	return &wirelessCollector{fs: fs}
}

func (c *wirelessCollector) Name() string { return "wireless" }

func (c *wirelessCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.signal = reg.GaugeVec(prefix+"_net_wireless_signal_level_dBm_average", "Wireless Signal", labels)
	c.quality = reg.GaugeVec(prefix+"_net_wireless_quality_link_average", "Wireless Quality", labels)
	c.discard = reg.GaugeVec(prefix+"_net_wireless_discarded_packets_packets_persec_average", "Wireless Discarded", labels)
	c.missed = reg.GaugeVec(prefix+"_net_wireless_missed_beacon_beacons_average", "Wireless Missed Beacons", labels)
}

func (c *wirelessCollector) Collect() error {
	stats, err := c.fs.Wireless()
	if err != nil {
		return err
	}

	for _, w := range stats {
		chart := "net_wireless." + w.Name
		family := w.Name

		c.signal.With(chart, "level", family).Set(float64(w.QualityLevel))
		c.quality.With(chart, "link", family).Set(float64(w.QualityLink))

		c.discard.With(chart, "nwid", family).Set(float64(w.DiscardedNwid))
		c.discard.With(chart, "crypt", family).Set(float64(w.DiscardedCrypt))
		c.discard.With(chart, "frag", family).Set(float64(w.DiscardedFrag))
		c.discard.With(chart, "retry", family).Set(float64(w.DiscardedRetry))
		c.discard.With(chart, "misc", family).Set(float64(w.DiscardedMisc))

		c.missed.With(chart, "beacon", family).Set(float64(w.MissedBeacon))
	}

	return nil
}
