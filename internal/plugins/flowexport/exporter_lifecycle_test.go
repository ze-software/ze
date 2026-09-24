package flowexport

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// templateFailureEncoder makes the first template too large for Sender, while
// data would fit. The flow path sends one template before the injected failure
// to cover accounting when only part of the template batch reaches the socket.
type templateFailureEncoder struct {
	fail bool
}

func (e *templateFailureEncoder) EncodeTemplate(sender *Sender) error {
	if e.fail {
		e.fail = false
		return sender.Send(make([]byte, sender.MaxDatagram()+1))
	}
	if err := sender.Send([]byte("template")); err != nil {
		return err
	}
	sender.AdvanceSequence(1)
	return nil
}

func (e *templateFailureEncoder) EncodeFlowTemplate(sender *Sender) error {
	if err := sender.Send([]byte("family")); err != nil {
		return err
	}
	sender.AdvanceSequence(1)
	return e.EncodeTemplate(sender)
}

func (e *templateFailureEncoder) Encode(snap CounterSnapshot, sender *Sender) (int, error) {
	if err := sender.Send([]byte("data")); err != nil {
		return 0, err
	}
	sender.AdvanceSequence(1)
	return len(snap.Interfaces), nil
}

func (e *templateFailureEncoder) EncodeFlows(flows []ConntrackFlow, sender *Sender) (int, error) {
	if err := sender.Send([]byte("data")); err != nil {
		return 0, err
	}
	sender.AdvanceSequence(1)
	return len(flows), nil
}

// TestExporterTemplateFailureRetriesBeforeData observes UDP output and public
// counters across a failed template and an immediate retry. Data must wait for
// its template, and successfully sent templates must appear in metrics/status.
//
// RFC requirement: RFC3954-x-1 negative -- exporter failure handling does not
// send data before its template; the UDP transcript contains templates before
// data after retry (RFC 3954 Section 7 recommends sending templates in advance).
func TestExporterTemplateFailureRetriesBeforeData(t *testing.T) {
	for _, flow := range []bool{false, true} {
		name := "counter"
		if flow {
			name = "flow"
		}
		t.Run(name, func(t *testing.T) {
			var lc net.ListenConfig
			pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = pc.Close() })
			collector, ok := pc.LocalAddr().(*net.UDPAddr)
			if !ok {
				t.Fatalf("collector address is %T, want *net.UDPAddr", pc.LocalAddr())
			}
			exp, err := newExporter(&Config{Collectors: []CollectorConfig{{
				Name: "c1", Address: "127.0.0.1", Port: collector.Port,
				Protocol: "netflow9", PollingInterval: 1, TemplateRefresh: 600,
				MaxDatagramSize: DatagramSizeDefault,
			}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(exp.stop)
			previousMetrics := metricsPtr.Load()
			t.Cleanup(func() { metricsPtr.Store(previousMetrics) })
			reg := metrics.NewPrometheusRegistry()
			BindMetrics(reg)
			enc := &templateFailureEncoder{fail: true}
			exp.setEncoder("c1", enc)
			exp.setFlowRecordEncoder("c1", enc)
			snap := CounterSnapshot{Time: time.Now(), Interfaces: make([]InterfaceCounters, 3)}
			export := func() {
				if flow {
					exp.exportFlows(make([]ConntrackFlow, 3))
					return
				}
				exp.notifySnapshot(snap)
			}
			export()
			firstDatagrams := uint64(0)
			firstBytes := uint64(0)
			if flow {
				firstDatagrams, firstBytes = 1, 6
			}
			first := exp.status()[0]
			if first["datagrams-sent"] != firstDatagrams || first["bytes-sent"] != firstBytes || first["errors"] != uint64(1) {
				t.Fatalf("failed-template status = %v; data must not be sent", first)
			}
			// The same snapshot is eligible: the failure must not postpone the
			// retry by either the polling interval or the refresh interval.
			export()
			want := []string{"template", "data"}
			if flow {
				want = []string{"family", "family", "template", "data"}
			}
			if err := pc.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var totalBytes uint64
			for _, expected := range want {
				var buf [64]byte
				n, _, err := pc.ReadFrom(buf[:])
				if err != nil {
					t.Fatal(err)
				}
				if string(buf[:n]) != expected {
					t.Fatalf("received %q, want %q", buf[:n], expected)
				}
				totalBytes += uint64(n)
			}
			status := exp.status()[0]
			if status["sequence"] != uint32(len(want)) || status["datagrams-sent"] != uint64(len(want)) || status["bytes-sent"] != totalBytes {
				t.Fatalf("status = %v, want %d packets/sequence and %d bytes", status, len(want), totalBytes)
			}
			scrape := httptest.NewRecorder()
			reg.Handler().ServeHTTP(scrape, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody))
			wantDatagrams := `ze_flowexport_datagrams_total{collector="c1",protocol="netflow9"} 2`
			wantBytes := `ze_flowexport_bytes_total{collector="c1",protocol="netflow9"} 12`
			if flow {
				wantDatagrams = `ze_flowexport_datagrams_total{collector="c1",protocol="netflow9"} 4`
				wantBytes = `ze_flowexport_bytes_total{collector="c1",protocol="netflow9"} 24`
			}
			for _, expected := range []string{wantDatagrams, wantBytes} {
				if !strings.Contains(scrape.Body.String(), expected+"\n") {
					t.Errorf("metrics lack %q:\n%s", expected, scrape.Body.String())
				}
			}
		})
	}
}
