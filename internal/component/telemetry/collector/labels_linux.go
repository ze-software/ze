// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection
// Related: collector.go — the Collector interface whose Init registers these labels

//go:build linux

package collector

// chartLabels are the three Prometheus labels every Netdata-compatible sample
// carries: the chart the sample belongs to, the dimension inside that chart,
// and the family the chart is grouped under. Every collector registers the same
// three, in this order, so the label set is written once here.
//
// A new slice is returned on each call. A registry keeps the slice it is given,
// so one shared slice would let two collectors alias the same backing array.
func chartLabels() []string {
	return []string{"chart", "dimension", "family"}
}
