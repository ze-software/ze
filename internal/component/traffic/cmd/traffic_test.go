package cmd

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/traffic"
)

const (
	testShowIfaceEth0 = "eth0"
	testShowIfaceLo   = "lo"
)

type fakeTrafficReader struct {
	byName map[string]traffic.InterfaceQoS
	all    []traffic.InterfaceQoS
}

func (f *fakeTrafficReader) Apply(context.Context, map[string]traffic.InterfaceQoS) error { return nil }
func (f *fakeTrafficReader) Close() error                                                 { return nil }
func (f *fakeTrafficReader) ListQdiscs(name string) (traffic.InterfaceQoS, error) {
	return f.byName[name], nil
}
func (f *fakeTrafficReader) ListAllQdiscs() ([]traffic.InterfaceQoS, error) { return f.all, nil }

func mustType[T any](t *testing.T, v any) T {
	t.Helper()
	got, ok := v.(T)
	if !ok {
		var zero T
		t.Fatalf("unexpected type %T, want %T (value=%#v)", v, zero, v)
	}
	return got
}

// TestShowTrafficControlWiredWithStats verifies handleShowTraffic across:
// active backend with HTB classes and live stats, unconfigured Linux read-only
// reader fallback, VPP nil-stats omission, and missing-backend error response.
//
// VALIDATES: AC-3 (unconfigured read-only readback, summary and detail stats,
// classes, handle, real qdisc kind, and non-Linux error contract) and AC-4
// (VPP omits stats rather than fabricating zeros).
// PREVENTS: missing live stats/classes in show traffic control or synthetic
// zeros on VPP.
func TestShowTrafficControlWiredWithStats(t *testing.T) {
	t.Run("no backend or reader returns platform error", func(t *testing.T) {
		t.Cleanup(traffic.ResetBackendsForTest())

		resp, err := handleShowTraffic(nil, commandtest.Args())
		if err != nil {
			t.Fatalf("handleShowTraffic error: %v", err)
		}
		if resp.Status != plugin.StatusError {
			t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusError)
		}
		if resp.Error != traffic.ErrNoBackendAvailable.Error() {
			t.Fatalf("error = %q, want %q", resp.Error, traffic.ErrNoBackendAvailable.Error())
		}
	})

	t.Run("active backend summary and detail with stats and classes", func(t *testing.T) {
		t.Cleanup(traffic.ResetBackendsForTest())

		eth0 := traffic.InterfaceQoS{
			Interface: testShowIfaceEth0,
			Qdisc: traffic.Qdisc{
				Type:         traffic.QdiscHTB,
				Kind:         "htb",
				Handle:       "1:0",
				DefaultClass: "default",
				Stats: &traffic.QdiscStats{
					Bytes:      10_000,
					Packets:    100,
					Qlen:       3,
					Backlog:    1500,
					Drops:      2,
					Overlimits: 5,
					Requeues:   1,
				},
				Classes: []traffic.TrafficClass{
					{
						Name:     "control",
						Handle:   "1:1",
						Rate:     10_000_000,
						Ceil:     100_000_000,
						Priority: 0,
						Stats: &traffic.QdiscStats{
							Bytes:      2000,
							Packets:    20,
							Qlen:       1,
							Backlog:    500,
							Drops:      0,
							Overlimits: 1,
							Requeues:   0,
						},
					},
					{
						Name:     "default",
						Handle:   "1:2",
						Rate:     90_000_000,
						Ceil:     100_000_000,
						Priority: 1,
						Stats: &traffic.QdiscStats{
							Bytes:      8000,
							Packets:    80,
							Qlen:       2,
							Backlog:    1000,
							Drops:      2,
							Overlimits: 4,
							Requeues:   1,
						},
					},
				},
			},
		}
		lo := traffic.InterfaceQoS{
			Interface: testShowIfaceLo,
			Qdisc: traffic.Qdisc{
				Kind:   "noqueue",
				Handle: "0:0",
				Stats:  &traffic.QdiscStats{Bytes: 512, Packets: 4},
			},
		}

		fb := &fakeTrafficReader{
			byName: map[string]traffic.InterfaceQoS{testShowIfaceEth0: eth0, testShowIfaceLo: lo},
			all:    []traffic.InterfaceQoS{eth0, lo},
		}
		if err := traffic.RegisterBackend("kernel-test", func() (traffic.Backend, error) { return fb, nil }); err != nil {
			t.Fatalf("RegisterBackend: %v", err)
		}
		if err := traffic.LoadBackend("kernel-test"); err != nil {
			t.Fatalf("LoadBackend: %v", err)
		}

		// Summary.
		sumResp, err := handleShowTraffic(nil, commandtest.Args())
		if err != nil {
			t.Fatalf("summary err: %v", err)
		}
		if sumResp.Status != plugin.StatusDone {
			t.Fatalf("summary status = %q, error = %q", sumResp.Status, sumResp.Error)
		}
		sumData := mustType[plugin.Map](t, sumResp.Data)
		if sumData["count"] != 2 {
			t.Fatalf("summary count = %v, want 2", sumData["count"])
		}
		rows := mustType[[]map[string]any](t, sumData["interfaces"])
		if rows[0][keyQdisc] != "htb" || rows[0][keyHandle] != "1:0" || rows[0]["bytes"] != uint64(10_000) || rows[0]["drops"] != uint32(2) {
			t.Fatalf("unexpected eth0 summary row: %#v", rows[0])
		}
		if rows[1][keyQdisc] != "noqueue" || rows[1]["packets"] != uint64(4) {
			t.Fatalf("unexpected lo summary row: %#v", rows[1])
		}

		// Detail.
		detResp, err := handleShowTraffic(nil, commandtest.Args(testShowIfaceEth0))
		if err != nil {
			t.Fatalf("detail err: %v", err)
		}
		if detResp.Status != plugin.StatusDone {
			t.Fatalf("detail status = %q, error = %q", detResp.Status, detResp.Error)
		}
		det := mustType[plugin.Map](t, detResp.Data)
		if det[keyQdisc] != "htb" || det[keyHandle] != "1:0" || det[keyDefaultClass] != "default" || det[keyClassCount] != 2 {
			t.Fatalf("unexpected detail header: %#v", det)
		}
		statsMap := mustType[map[string]any](t, det[keyStats])
		if statsMap["bytes"] != uint64(10_000) || statsMap["qlen"] != uint32(3) || statsMap["overlimits"] != uint32(5) {
			t.Fatalf("unexpected detail stats: %#v", statsMap)
		}
		classes := mustType[[]map[string]any](t, det["classes"])
		if len(classes) != 2 {
			t.Fatalf("detail classes len = %d, want 2", len(classes))
		}
		if classes[0]["name"] != "control" || classes[0][keyHandle] != "1:1" || classes[0][keyRateBps] != uint64(10_000_000) {
			t.Fatalf("unexpected class[0]: %#v", classes[0])
		}
		clsStats := mustType[map[string]any](t, classes[0][keyStats])
		if clsStats["bytes"] != uint64(2000) || clsStats["packets"] != uint64(20) {
			t.Fatalf("unexpected class[0] stats: %#v", clsStats)
		}
	})

	t.Run("unconfigured read-only reader fallback", func(t *testing.T) {
		defName := traffic.DefaultBackendName()
		if defName == "" {
			t.Skip("platform has no default traffic backend")
		}
		t.Cleanup(traffic.ResetBackendsForTest())

		unshaped := traffic.InterfaceQoS{
			Interface: testShowIfaceEth0,
			Qdisc: traffic.Qdisc{
				Kind:   "fq_codel",
				Handle: "0:0",
				Stats:  &traffic.QdiscStats{Bytes: 4200, Packets: 42, Drops: 1},
			},
		}
		fr := &fakeTrafficReader{
			byName: map[string]traffic.InterfaceQoS{testShowIfaceEth0: unshaped},
			all:    []traffic.InterfaceQoS{unshaped},
		}
		if err := traffic.RegisterReader(defName, fr); err != nil {
			t.Fatalf("RegisterReader: %v", err)
		}

		resp, err := handleShowTraffic(nil, commandtest.Args())
		if err != nil {
			t.Fatalf("fallback summary err: %v", err)
		}
		if resp.Status != plugin.StatusDone {
			t.Fatalf("fallback summary status = %q, error = %q", resp.Status, resp.Error)
		}
		fallbackData := mustType[plugin.Map](t, resp.Data)
		rows := mustType[[]map[string]any](t, fallbackData["interfaces"])
		if len(rows) != 1 || rows[0][keyQdisc] != "fq_codel" || rows[0]["bytes"] != uint64(4200) {
			t.Fatalf("unexpected fallback summary rows: %#v", rows)
		}
	})

	t.Run("VPP nil stats omitted from summary and detail", func(t *testing.T) {
		t.Cleanup(traffic.ResetBackendsForTest())

		vppQoS := traffic.InterfaceQoS{
			Interface: testShowIfaceEth0,
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "fast", Rate: 10_000_000, Ceil: 20_000_000},
				},
			},
		}
		fb := &fakeTrafficReader{
			byName: map[string]traffic.InterfaceQoS{testShowIfaceEth0: vppQoS},
			all:    []traffic.InterfaceQoS{vppQoS},
		}
		if err := traffic.RegisterBackend("vpp-test", func() (traffic.Backend, error) { return fb, nil }); err != nil {
			t.Fatalf("RegisterBackend: %v", err)
		}
		if err := traffic.LoadBackend("vpp-test"); err != nil {
			t.Fatalf("LoadBackend: %v", err)
		}

		sumResp, err := handleShowTraffic(nil, commandtest.Args())
		if err != nil {
			t.Fatalf("vpp summary err: %v", err)
		}
		vppSumData := mustType[plugin.Map](t, sumResp.Data)
		row := mustType[[]map[string]any](t, vppSumData["interfaces"])[0]
		for _, statKey := range []string{"bytes", "packets", "qlen", "backlog", "drops", "overlimits", "requeues"} {
			if _, present := row[statKey]; present {
				t.Fatalf("VPP summary row unexpectedly contains %q: %#v", statKey, row)
			}
		}

		detResp, err := handleShowTraffic(nil, commandtest.Args(testShowIfaceEth0))
		if err != nil {
			t.Fatalf("vpp detail err: %v", err)
		}
		det := mustType[plugin.Map](t, detResp.Data)
		if _, present := det[keyStats]; present {
			t.Fatalf("VPP detail unexpectedly contains stats: %#v", det)
		}
		cls := mustType[[]map[string]any](t, det["classes"])[0]
		if _, present := cls[keyStats]; present {
			t.Fatalf("VPP class unexpectedly contains stats: %#v", cls)
		}
	})
}
