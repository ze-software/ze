package bmp

import (
	"encoding/binary"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestBMPCompositeKeyFormat(t *testing.T) {
	tests := []struct {
		name   string
		router string
		peer   PeerHeader
		want   string
	}{
		{
			name:   "ipv4 peer",
			router: "10.0.0.1:12345",
			peer: PeerHeader{
				Address: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 168, 1, 1},
				Flags:   0,
			},
			want: "10.0.0.1:12345:192.168.1.1",
		},
		{
			name:   "ipv6 peer",
			router: "10.0.0.1:12345",
			peer: PeerHeader{
				Address: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				Flags:   PeerFlagV,
			},
			want: "10.0.0.1:12345:2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bmpCompositeKey(tt.router, tt.peer)
			if got != tt.want {
				t.Errorf("bmpCompositeKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBMPCollectorSourceAddress verifies the collector's source-address leaf is
// parsed from the config JSON into collectorConfig, and that a collector without
// it leaves the field empty (OS-selected source).
//
// VALIDATES: AC-3/AC-4 -- source-address flows from YANG-derived JSON to the
// collector config that the sender dialer binds as LocalAddr.
// PREVENTS: the leaf being accepted by YANG but dropped before the dialer.
func TestBMPCollectorSourceAddress(t *testing.T) {
	data := `{"bgp":{"bmp":{"sender":{"collector":{
		"withsrc":{"address":"10.0.0.100","port":"11019","source-address":"192.168.1.1"},
		"nosrc":{"address":"10.0.0.101","port":"11019"}
	}}}}}`

	cfg, err := parseSenderConfig(data)
	if err != nil {
		t.Fatalf("parseSenderConfig: %v", err)
	}

	withSrc, ok := cfg.Collectors["withsrc"]
	if !ok {
		t.Fatal("collector \"withsrc\" missing")
	}
	if withSrc.SourceAddress != "192.168.1.1" {
		t.Errorf("SourceAddress = %q, want %q", withSrc.SourceAddress, "192.168.1.1")
	}

	noSrc, ok := cfg.Collectors["nosrc"]
	if !ok {
		t.Fatal("collector \"nosrc\" missing")
	}
	if noSrc.SourceAddress != "" {
		t.Errorf("SourceAddress = %q, want empty (unconfigured)", noSrc.SourceAddress)
	}
}

func TestPeerAddressString(t *testing.T) {
	tests := []struct {
		name string
		peer PeerHeader
		want string
	}{
		{
			name: "ipv4",
			peer: PeerHeader{
				Address: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 10, 0, 0, 1},
			},
			want: "10.0.0.1",
		},
		{
			name: "ipv6",
			peer: PeerHeader{
				Address: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				Flags:   PeerFlagV,
			},
			want: "2001:db8::1",
		},
		{
			name: "ipv4 zero",
			peer: PeerHeader{
				Address: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
			want: "0.0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peerAddressString(tt.peer)
			if got != tt.want {
				t.Errorf("peerAddressString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes verifies that BMP
// receiver RIB sync errors (route injection, termination/peer-down withdraw)
// and mid-body session read errors log WARN with their registered doctor codes,
// count route inject errors in Prometheus, enqueue issuequeue reports, and stay
// silent once stopCh is closed.
//
// VALIDATES: AC-2, AC-4 (bmp.go handleSession, processTermination, processPeerDown, processRouteMonitoring).
// PREVENTS: BMP receiver RIB sync and session read failures vanishing below WARN.
func TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes(t *testing.T) {
	reg := &bmpTestMetrics{}
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(nil) })

	logs := &bmpSyncWriter{}
	prevLogger := logger()
	setLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(prevLogger) })

	pub := &bmpTestPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	// Closed SDK connection forces InjectWireRoute and DispatchCommandArgs to return an error.
	c1, c2 := net.Pipe()
	_ = c1.Close()
	_ = c2.Close()
	p := sdk.NewWithConn("bgp-bmp", c1)

	bp := &BMPPlugin{
		plugin: p,
		issues: q,
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}

	// 1. Route Monitoring inject failure (twice -> counted twice, warned/raised once).
	peerHdr := bmpEventPeerHeader(42)
	rm := &RouteMonitoring{
		Peer:      peerHdr,
		BGPUpdate: make([]byte, bgpHeaderSize+4),
	}
	bp.processRouteMonitoring("198.51.100.1:11019", rm)
	bp.processRouteMonitoring("198.51.100.1:11019", rm)

	// 2. Peer Down and Termination RIB withdraw failures.
	bp.processPeerDown("198.51.100.1:11019", &PeerDown{Peer: peerHdr, Reason: PeerDownLocalNoNotify})
	bp.processTermination("198.51.100.1:11019", &Termination{})

	// 3. Session body read failure: 6-byte BMP common header declaring 40 bytes, followed by immediate EOF.
	sConn, rConn := net.Pipe()
	var hdr [CommonHeaderSize]byte
	hdr[0] = Version
	binary.BigEndian.PutUint32(hdr[1:5], 40)
	hdr[5] = MsgInitiation
	go func() {
		_, _ = rConn.Write(hdr[:])
		_ = rConn.Close()
	}()
	bp.handleSession(sConn)

	q.Flush()

	peerKey := bmpCompositeKey("198.51.100.1:11019", peerHdr)
	injectVec := reg.vecs[metricBMPRouteInjectErrors]
	require.NotNil(t, injectVec, "ze_bmp_route_inject_errors_total must be registered")
	assert.Equal(t, float64(2), injectVec.value(peerKey), "route inject failures must be counted twice")

	out := logs.String()
	assert.Contains(t, out, "level=WARN")
	assert.Contains(t, out, "code="+codeDoctorBMPRIBSyncFailed)
	assert.Contains(t, out, "code="+codeDoctorBMPReceiverSessionError)
	assert.Contains(t, out, fixShowBMPPeers)
	assert.Contains(t, out, fixShowBMPSessions)
	assert.Contains(t, out, "report=queued")

	pub.mu.Lock()
	raises := append([]sdk.Issue(nil), pub.raises...)
	pub.mu.Unlock()
	var sawRIBSync, sawSessionErr bool
	for _, issue := range raises {
		if issue.Code == codeDoctorBMPRIBSyncFailed {
			sawRIBSync = true
			assert.Equal(t, rpc.ReportError, issue.Severity)
			assert.Equal(t, fixShowBMPPeers, issue.Fix)
		}
		if issue.Code == codeDoctorBMPReceiverSessionError {
			sawSessionErr = true
			assert.Equal(t, rpc.ReportError, issue.Severity)
			assert.Equal(t, fixShowBMPSessions, issue.Fix)
		}
	}
	assert.True(t, sawRIBSync, "expected issuequeue raise for %s", codeDoctorBMPRIBSyncFailed)
	assert.True(t, sawSessionErr, "expected issuequeue raise for %s", codeDoctorBMPReceiverSessionError)

	// Shutdown suppression: once stopCh is closed, sync errors stay silent.
	close(bp.stopCh)
	beforeCount := len(raises)
	bp.processTermination("198.51.100.2:11019", &Termination{})
	q.Flush()
	pub.mu.Lock()
	afterCount := len(pub.raises)
	pub.mu.Unlock()
	assert.Equal(t, beforeCount, afterCount, "receiver sync errors during shutdown must not raise doctor warnings")
	assert.True(t, strings.Contains(out, "report=queued"))
}
