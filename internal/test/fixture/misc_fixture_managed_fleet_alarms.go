// Design: docs/architecture/fleet-alarms.md -- hub-derived fleet alarm lifecycle, reconnect, persistence, and history pagination fixtures
// Related: register_managed_fleet_alarms.go -- registers the two fleet alarm functional test drivers
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRunWithHubConfig provisions the hub and device stores
// Related: misc_fixture_managed_fleet_history.go -- controlled managed client and SSH CLI query pattern
// Related: misc_fixture_managed_fleet_reports.go -- fleetJSONCompactSuffix and report query helpers

package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/managed"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	fleetAlarmPeerName           = "edge-02"
	fleetAlarmCompBGP            = "bgp"
	fleetAlarmCompIface          = "iface"
	fleetAlarmCompOne            = "comp-1"
	fleetAlarmCompTwo            = "comp-2"
	fleetAlarmCompThree          = "comp-3"
	fleetAlarmCompFour           = "comp-4"
	fleetAlarmSubjectCPU         = "cpu"
	fleetAlarmShowPrefix         = "show fleet alarm"
	fleetAlarmHistoryPrefix      = "show fleet alarm history device "
	fleetAlarmOfflineDelaySec    = 5
	fleetAlarmResourceCPUPct     = 80
	fleetAlarmResourceDurSec     = 2
	fleetAlarmHistoryInterval    = 1
	fleetAlarmMaxEpisodesDevice  = 3
	fleetAlarmMaxEpisodesHub     = 64
	fleetAlarmHighCPUFirst       = 95.0
	fleetAlarmLowCPU             = 40.0
	fleetAlarmHighCPUSecond      = 96.0
	fleetAlarmRPCTimeout         = 5 * time.Second
	fleetAlarmWaitTimeout        = 20 * time.Second
	fleetAlarmAuthLineMax        = 512
	fleetAlarmConnectAttempts    = 40
	fleetAlarmConnectDelay       = 100 * time.Millisecond
	fleetAlarmSamplePollDelay    = 350 * time.Millisecond
	fleetAlarmSampleAttempts     = 60
	fleetAlarmPageLimit          = 2
	fleetAlarmMaxPages           = 16
	fleetAlarmDeclaredAtPrefix   = "declared-at="
	fleetAlarmFetchStatusCurrent = "current"
)

// fleetAlarmClient holds one authenticated managed client TLS+MuxConn session
// to the real ze hub daemon for deterministic on-demand health and sample RPCs.
// Callers MUST invoke close before returning.
type fleetAlarmClient struct {
	conn   net.Conn
	mc     *rpc.MuxConn
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (c *fleetAlarmClient) serveHubRequests(ctx context.Context) {
	defer close(c.done)
	for {
		select {
		case req, ok := <-c.mc.Requests():
			if !ok {
				return
			}
			_ = c.mc.SendOK(ctx, req.ID) //nolint:errcheck // best-effort ack of hub push during fixture
		case <-c.mc.Done():
			return
		case <-ctx.Done():
			return
		}
	}
}

// close terminates the managed session and waits for the responder goroutine
// to exit. Idempotent. MUST be called by the owner of every fleetAlarmClient.
func (c *fleetAlarmClient) close() error {
	var closeErr error
	c.once.Do(func() {
		c.cancel()
		mcErr := c.mc.Close()
		connErr := c.conn.Close()
		<-c.done
		if mcErr != nil && !errors.Is(mcErr, net.ErrClosed) {
			closeErr = mcErr
		}
		if closeErr == nil && connErr != nil && !errors.Is(connErr, net.ErrClosed) {
			closeErr = connErr
		}
	})
	return closeErr
}

func (c *fleetAlarmClient) sendHealth(ctx context.Context, rep fleet.HealthReport) error {
	body, err := fleet.EncodeHealthReport(rep)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
	defer cancel()
	_, err = c.mc.CallRPC(callCtx, fleet.VerbHealthReport, json.RawMessage(body))
	if err != nil {
		return fmt.Errorf("send health report RPC: %w", err)
	}
	return nil
}

func (c *fleetAlarmClient) sendSamples(ctx context.Context, readings []fleet.SampleReading) error {
	rep := fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    time.Now().UTC().Format(time.RFC3339Nano),
		Samples:       append([]fleet.SampleReading(nil), readings...),
	}
	body, err := fleet.EncodeSampleReport(rep)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
	defer cancel()
	_, err = c.mc.CallRPC(callCtx, fleet.VerbSampleReport, json.RawMessage(body))
	if err != nil {
		return fmt.Errorf("send sample report RPC: %w", err)
	}
	return nil
}

func dialFleetAlarmClientOnce(ctx context.Context, addr, secret string) (*fleetAlarmClient, error) {
	callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
	defer cancel()

	dialer := &net.Dialer{Timeout: fleetAlarmRPCTimeout}
	rawConn, err := dialer.DialContext(callCtx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial hub %s: %w", addr, err)
	}

	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName:         addrLoopback,
		InsecureSkipVerify: true, //nolint:gosec // test fixture dials local self-signed hub over loopback
		MinVersion:         tls.VersionTLS13,
	})
	if err := tlsConn.HandshakeContext(callCtx); err != nil {
		_ = rawConn.Close() //nolint:errcheck // cleanup on handshake failure
		return nil, fmt.Errorf("tls handshake with hub %s: %w", addr, err)
	}

	if err := pluginipc.SendAuth(callCtx, tlsConn, secret, caTrustClientName); err != nil {
		_ = tlsConn.Close() //nolint:errcheck // cleanup on auth failure
		return nil, fmt.Errorf("send auth for %s: %w", caTrustClientName, err)
	}
	if err := tlsConn.SetReadDeadline(time.Now().Add(fleetAlarmRPCTimeout)); err != nil {
		_ = tlsConn.Close() //nolint:errcheck // cleanup on deadline error
		return nil, fmt.Errorf("set auth read deadline: %w", err)
	}
	authLine, err := readAlarmAuthLine(tlsConn)
	if err != nil {
		_ = tlsConn.Close() //nolint:errcheck // cleanup on auth read failure
		return nil, fmt.Errorf("read auth response for %s: %w", caTrustClientName, err)
	}
	if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
		_ = tlsConn.Close() //nolint:errcheck // cleanup on deadline reset error
		return nil, fmt.Errorf("clear auth read deadline: %w", err)
	}
	_, verb, _, parseErr := rpc.ParseLine(authLine)
	if parseErr != nil {
		_ = tlsConn.Close() //nolint:errcheck // cleanup on invalid auth line
		return nil, fmt.Errorf("parse auth response for %s: %w", caTrustClientName, parseErr)
	}
	if verb != "ok" {
		_ = tlsConn.Close() //nolint:errcheck // cleanup on rejected auth
		return nil, fmt.Errorf("hub rejected auth for %s: %s", caTrustClientName, string(authLine))
	}

	rc := rpc.NewConn(tlsConn, tlsConn)
	mc := rpc.NewMuxConn(rc)

	resp, err := managed.FetchConfig(callCtx, mc, "")
	if err != nil {
		_ = mc.Close()      //nolint:errcheck // cleanup on fetch failure
		_ = tlsConn.Close() //nolint:errcheck // cleanup on fetch failure
		return nil, fmt.Errorf("initial config-fetch for %s: %w", caTrustClientName, err)
	}
	if resp.Version != "" && resp.Status != fleetAlarmFetchStatusCurrent {
		if ackErr := managed.SendConfigAck(callCtx, mc, fleet.ConfigAck{Version: resp.Version, OK: true}); ackErr != nil {
			_ = mc.Close()      //nolint:errcheck // cleanup on ack failure
			_ = tlsConn.Close() //nolint:errcheck // cleanup on ack failure
			return nil, fmt.Errorf("initial config-ack for %s: %w", caTrustClientName, ackErr)
		}
	}

	sessCtx, sessCancel := context.WithCancel(ctx)
	client := &fleetAlarmClient{
		conn:   tlsConn,
		mc:     mc,
		cancel: sessCancel,
		done:   make(chan struct{}),
	}
	go client.serveHubRequests(sessCtx)
	return client, nil
}

func dialFleetAlarmClient(ctx context.Context, addr, secret string) (*fleetAlarmClient, error) {
	var client *fleetAlarmClient
	var lastErr error
	connected := Poll(ctx, fleetAlarmConnectAttempts, fleetAlarmConnectDelay, func() bool {
		c, err := dialFleetAlarmClientOnce(ctx, addr, secret)
		if err != nil {
			lastErr = err
			return false
		}
		client = c
		return true
	})
	if !connected {
		return nil, fmt.Errorf("managed alarm client %s failed to connect to %s: %w", caTrustClientName, addr, lastErr)
	}
	return client, nil
}

func readAlarmAuthLine(conn net.Conn) ([]byte, error) {
	buf := make([]byte, 0, 128)
	b := make([]byte, 1)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if b[0] == '\n' {
			if len(buf) > 0 && buf[len(buf)-1] == '\r' {
				buf = buf[:len(buf)-1]
			}
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) >= fleetAlarmAuthLineMax {
			return nil, fmt.Errorf("auth response exceeds %d bytes", fleetAlarmAuthLineMax)
		}
	}
}

func fleetAlarmLifecycleHubConfig(sshPort, pluginPort, managedPort string) string {
	var b textbuf.Buffer
	b.Str(restoreClientSSHListener(sshPort))
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(managedPort).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	b.Str("            client ").Str(caTrustClientName).Str(" { secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("        }\n")
	b.Str("        alarms {\n")
	b.Str("            offline-delay ").Int(fleetAlarmOfflineDelaySec).Str(";\n")
	b.Str("            resource-cpu-percent ").Int(fleetAlarmResourceCPUPct).Str(";\n")
	b.Str("            resource-duration ").Int(fleetAlarmResourceDurSec).Str(";\n")
	b.Str("        }\n")
	b.Str("        history {\n")
	b.Str("            interval ").Int(fleetAlarmHistoryInterval).Str(";\n")
	b.Str("        }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    session { asn { local 65000; } }\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

func fleetAlarmHistoryHubConfig(sshPort, pluginPort, managedPort string) string {
	var b textbuf.Buffer
	b.Str(restoreClientSSHListener(sshPort))
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(managedPort).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	b.Str("            client ").Str(caTrustClientName).Str(" { secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("            client ").Str(fleetAlarmPeerName).Str(" { secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("        }\n")
	b.Str("        alarms {\n")
	b.Str("            offline-delay ").Int(fleetAlarmOfflineDelaySec).Str(";\n")
	b.Str("            max-episodes-per-device ").Int(fleetAlarmMaxEpisodesDevice).Str(";\n")
	b.Str("            max-episodes-hub ").Int(fleetAlarmMaxEpisodesHub).Str(";\n")
	b.Str("        }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    session { asn { local 65000; } }\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

// decodeAlarmJSON decodes a CLI JSON answer into dst, unwrapping a
// {"status":"done","data":...} envelope if present.
func decodeAlarmJSON(raw string, dst any) error {
	start := strings.IndexAny(raw, "[{")
	if start < 0 {
		return errors.New("alarm CLI answer carries no JSON document")
	}
	trimmed := bytes.TrimSpace([]byte(raw[start:]))
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var env struct {
			Status string          `json:"status"`
			Data   json.RawMessage `json:"data"`
		}
		if json.Unmarshal(trimmed, &env) == nil && env.Status != "" && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, dst)
		}
	}
	return json.NewDecoder(bytes.NewReader(trimmed)).Decode(dst)
}

func (s *fleetHoldScenario) queryAlarmList(ctx context.Context, pipes string) (pluginserver.AlarmListPage, string, error) {
	cmd := fleetAlarmShowPrefix + pipes + fleetJSONCompactSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return pluginserver.AlarmListPage{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var page pluginserver.AlarmListPage
	if err := decodeAlarmJSON(raw, &page); err != nil {
		return pluginserver.AlarmListPage{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return page, raw, nil
}

func (s *fleetHoldScenario) queryAlarmHistory(ctx context.Context, device, pipes string) (pluginserver.AlarmHistoryPage, string, error) {
	cmd := fleetAlarmHistoryPrefix + device + pipes + fleetJSONCompactSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return pluginserver.AlarmHistoryPage{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var page pluginserver.AlarmHistoryPage
	if err := decodeAlarmJSON(raw, &page); err != nil {
		return pluginserver.AlarmHistoryPage{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return page, raw, nil
}

func (s *fleetHoldScenario) waitAlarmList(
	ctx context.Context,
	pipes string,
	predicate func(*pluginserver.AlarmListPage) bool,
	desc string,
) (pluginserver.AlarmListPage, string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, fleetAlarmWaitTimeout)
	defer cancel()

	var matched pluginserver.AlarmListPage
	var lastRaw string
	var lastErr error
	ok := Poll(waitCtx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		page, raw, err := s.queryAlarmList(waitCtx, pipes)
		if raw != "" {
			lastRaw = raw
		}
		lastErr = err
		if err != nil {
			return false
		}
		if !predicate(&page) {
			return false
		}
		matched = page
		return true
	})
	if !ok {
		return pluginserver.AlarmListPage{}, lastRaw, fmt.Errorf("%s did not reach expected alarm state (err=%w): %s", desc, lastErr, lastRaw)
	}
	return matched, lastRaw, nil
}

func (s *fleetHoldScenario) queryAlarmCount(ctx context.Context, pipes string) (int, string, error) {
	cmd := fleetAlarmShowPrefix + pipes + " | count"
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return 0, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	trimmed := strings.TrimSpace(raw)
	if n, convErr := strconv.Atoi(trimmed); convErr == nil {
		return n, raw, nil
	}
	var obj struct {
		Count *int `json:"count"`
	}
	if err := decodeAlarmJSON(raw, &obj); err != nil {
		return 0, raw, fmt.Errorf("%s: decode count: %w: %s", cmd, err, raw)
	}
	if obj.Count == nil {
		return 0, raw, fmt.Errorf("%s: missing count field in answer: %s", cmd, raw)
	}
	return *obj.Count, raw, nil
}

func makeAlarmHealthReport(components []fleet.HealthComponent) fleet.HealthReport {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	overall := fleet.HealthStatusHealthy
	out := make([]fleet.HealthComponent, len(components))
	for i, c := range components {
		c.CheckedAt = now
		out[i] = c
		if c.Status == fleet.HealthStatusDown {
			overall = fleet.HealthStatusDown
		} else if c.Status == fleet.HealthStatusDegraded && overall != fleet.HealthStatusDown {
			overall = fleet.HealthStatusDegraded
		}
	}
	return fleet.HealthReport{
		Status:     overall,
		Components: out,
		CheckedAt:  now,
	}
}

func findOpenAlarm(alarms []pluginserver.AlarmOccurrence, kind, device, subject string) (pluginserver.AlarmOccurrence, bool) {
	for i := range alarms {
		occ := alarms[i]
		if occ.Kind != kind {
			continue
		}
		if occ.Device != device {
			continue
		}
		if occ.Subject != subject {
			continue
		}
		return occ, true
	}
	return pluginserver.AlarmOccurrence{}, false
}

func countHistoryByID(events []pluginserver.AlarmHistoryEvent, id string) (int, pluginserver.AlarmHistoryEvent) {
	count := 0
	var matched pluginserver.AlarmHistoryEvent
	for i := range events {
		if events[i].ID == id {
			count++
			matched = events[i]
		}
	}
	return count, matched
}

func isSampleRateLimitedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "rate limited")
}

// sendCPUSampleAccepted pushes one CPU sample, retrying across UTC second
// boundaries when the hub's 1-second sample interval rate-limits an earlier
// attempt in the same second.
func sendCPUSampleAccepted(ctx context.Context, client *fleetAlarmClient, cpuPct float64) error {
	readings := []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: cpuPct}}
	var lastErr error
	accepted := Poll(ctx, fleetAlarmSampleAttempts, fleetAlarmSamplePollDelay, func() bool {
		err := client.sendSamples(ctx, readings)
		if err == nil {
			return true
		}
		lastErr = err
		return false
	})
	if !accepted {
		return fmt.Errorf("CPU sample %.1f was not accepted: %w", cpuPct, lastErr)
	}
	return nil
}

// driveCPUAlarmOpen pushes high CPU samples across successive UTC seconds until
// the hub's resource-high (subject=cpu) alarm opens for device.
func (s *fleetHoldScenario) driveCPUAlarmOpen(
	ctx context.Context,
	client *fleetAlarmClient,
	device string,
	cpuPct float64,
	desc string,
) (pluginserver.AlarmOccurrence, error) {
	readings := []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: cpuPct}}
	var opened pluginserver.AlarmOccurrence
	var lastRaw string
	var lastErr error
	ok := Poll(ctx, fleetAlarmSampleAttempts, fleetAlarmSamplePollDelay, func() bool {
		if sendErr := client.sendSamples(ctx, readings); sendErr != nil && !isSampleRateLimitedError(sendErr) {
			lastErr = sendErr
			return false
		}
		page, raw, qErr := s.queryAlarmList(ctx, " | kind "+pluginserver.AlarmKindResourceHigh)
		lastRaw = raw
		if qErr != nil {
			lastErr = qErr
			return false
		}
		occ, found := findOpenAlarm(page.Alarms, pluginserver.AlarmKindResourceHigh, device, fleetAlarmSubjectCPU)
		if !found {
			return false
		}
		opened = occ
		return true
	})
	if !ok {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("%s: resource-high did not open (err=%w): %s", desc, lastErr, lastRaw)
	}
	return opened, nil
}

func reconnectAndSendHealth(
	ctx context.Context,
	addr string,
	cur *fleetAlarmClient,
	rep fleet.HealthReport,
) (*fleetAlarmClient, error) {
	if cur != nil {
		if err := cur.close(); err != nil {
			return nil, fmt.Errorf("close prior managed alarm session: %w", err)
		}
	}
	next, err := dialFleetAlarmClient(ctx, addr, caTrustClientSecret)
	if err != nil {
		return nil, err
	}
	if err := next.sendHealth(ctx, rep); err != nil {
		_ = next.close() //nolint:errcheck // cleanup on failed health RPC
		return nil, err
	}
	return next, nil
}

func parseDeclaredAtDetail(detail string) (time.Time, error) {
	for token := range strings.FieldsSeq(detail) {
		if rawTS, ok := strings.CutPrefix(token, fleetAlarmDeclaredAtPrefix); ok {
			ts, err := time.Parse(time.RFC3339, rawTS)
			if err != nil {
				return time.Time{}, fmt.Errorf("parse %q in detail %q: %w", rawTS, detail, err)
			}
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("detail %q does not contain %s", detail, fleetAlarmDeclaredAtPrefix)
}

func requireAlarmIDAndSince(label string, occ *pluginserver.AlarmOccurrence) error {
	if occ.ID == "" {
		return fmt.Errorf("%s missing occurrence id: %+v", label, *occ)
	}
	if occ.Since.IsZero() {
		return fmt.Errorf("%s missing occurrence since: %+v", label, *occ)
	}
	return nil
}

func requireSameAlarmIdentity(label string, want, got *pluginserver.AlarmOccurrence) error {
	if got.ID != want.ID {
		return fmt.Errorf("%s id changed from %s to %s", label, want.ID, got.ID)
	}
	if !got.Since.Equal(want.Since) {
		return fmt.Errorf("%s since changed from %s to %s", label, want.Since, got.Since)
	}
	return nil
}

func verifyLifecyclePipes(ctx context.Context, s *fleetHoldScenario, wantCount int, bgpID string) error {
	gotCount, rawCount, err := s.queryAlarmCount(ctx, "")
	if err != nil {
		return err
	}
	if gotCount != wantCount {
		return fmt.Errorf("show fleet alarm | count = %d, want %d: %s", gotCount, wantCount, rawCount)
	}

	matchedPage, rawMatch, err := s.queryAlarmList(ctx, " | match "+fleetAlarmCompBGP)
	if err != nil {
		return err
	}
	if len(matchedPage.Alarms) != 1 {
		return fmt.Errorf("show fleet alarm | match %s returned %d alarms, want 1: %s", fleetAlarmCompBGP, len(matchedPage.Alarms), rawMatch)
	}
	if matchedPage.Alarms[0].ID != bgpID {
		return fmt.Errorf("show fleet alarm | match %s returned id=%q, want %q: %s", fleetAlarmCompBGP, matchedPage.Alarms[0].ID, bgpID, rawMatch)
	}

	tableOut, err := storageLiveRequest(ctx, s.hubOperator, fleetAlarmShowPrefix+" | table")
	if err != nil {
		return fmt.Errorf("show fleet alarm | table: %w: %s", err, tableOut)
	}
	if !strings.Contains(tableOut, caTrustClientName) {
		return fmt.Errorf("show fleet alarm | table missing device %s:\n%s", caTrustClientName, tableOut)
	}
	if !strings.Contains(tableOut, fleetAlarmCompBGP) {
		return fmt.Errorf("show fleet alarm | table missing subject %s:\n%s", fleetAlarmCompBGP, tableOut)
	}
	return nil
}

// managedFleetAlarmLifecycleDriver validates fleet-ui-5 AC-4, AC-8, AC-9, and
// AC-10 against a real ze hub daemon: two down components open distinct
// occurrence IDs that stay stable across repeat reports and reads without
// duplicate history rows, sustained high CPU opens resource-high, disconnect
// opens device-offline after offline-delay while preserving open occurrences,
// reconnect clears device-offline and immediately closes resource-high while
// preserving still-down bgp and recording recovered iface once, and fresh
// qualifying high CPU samples open a new resource-high ID with a later since.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetAlarmLifecycleDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-alarm-lifecycle", args, false, fleetAlarmLifecycleHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			// Step 1: Connect declared device edge-01 and confirm zero open alarms.
			client, err := dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return err
			}
			defer func() {
				if client != nil {
					_ = client.close() //nolint:errcheck // best-effort cleanup on return
				}
			}()

			initialPage, rawInitial, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				return p.Total == 0 && len(p.Alarms) == 0
			}, "initial zero alarms after edge-01 connect")
			if err != nil {
				return err
			}
			if initialPage.Counts.Total != 0 {
				return fmt.Errorf("initial alarm counts.total=%d, want 0: %s", initialPage.Counts.Total, rawInitial)
			}

			// Step 2: Emit health report with two down components (bgp, iface).
			bothDown := makeAlarmHealthReport([]fleet.HealthComponent{
				{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusDown, Reason: "bgp engine down"},
				{Name: fleetAlarmCompIface, Status: fleet.HealthStatusDown, Reason: "uplink down"},
			})
			if err := client.sendHealth(ctx, bothDown); err != nil {
				return err
			}
			twoDownPage, rawTwoDown, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				return p.Total == 2 && len(p.Alarms) == 2
			}, "two component-down alarms opened")
			if err != nil {
				return err
			}
			if twoDownPage.Counts.Critical != 2 {
				return fmt.Errorf("counts.critical=%d, want 2: %s", twoDownPage.Counts.Critical, rawTwoDown)
			}
			bgpAlarm, okBGP := findOpenAlarm(twoDownPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
			if !okBGP {
				return fmt.Errorf("component-down alarm for %s missing: %s", fleetAlarmCompBGP, rawTwoDown)
			}
			ifaceAlarm, okIface := findOpenAlarm(twoDownPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompIface)
			if !okIface {
				return fmt.Errorf("component-down alarm for %s missing: %s", fleetAlarmCompIface, rawTwoDown)
			}
			if err := requireAlarmIDAndSince("bgp component-down", &bgpAlarm); err != nil {
				return err
			}
			if err := requireAlarmIDAndSince("iface component-down", &ifaceAlarm); err != nil {
				return err
			}
			if bgpAlarm.ID == ifaceAlarm.ID {
				return fmt.Errorf("two down components shared one occurrence id %q: %s", bgpAlarm.ID, rawTwoDown)
			}

			// Confirm severity and device filters on show fleet alarm.
			critPage, rawCrit, err := s.queryAlarmList(ctx, " | severity critical")
			if err != nil {
				return err
			}
			if critPage.Total != 2 {
				return fmt.Errorf("severity critical filter returned total=%d, want 2: %s", critPage.Total, rawCrit)
			}
			if critPage.Counts.Critical != 2 {
				return fmt.Errorf("severity critical filter returned critical=%d, want 2: %s", critPage.Counts.Critical, rawCrit)
			}
			devPage, rawDev, err := s.queryAlarmList(ctx, " | device "+caTrustClientName)
			if err != nil {
				return err
			}
			if devPage.Total != 2 {
				return fmt.Errorf("device %s filter returned total=%d, want 2: %s", caTrustClientName, devPage.Total, rawDev)
			}

			// Step 3 (AC-9): Reconnect quickly (< offline-delay) so the hub's
			// per-connection 1-minute health-report rate limit accepts a repeated
			// identical health report, then read show fleet alarm twice.
			client, err = reconnectAndSendHealth(ctx, s.hubAddress, client, bothDown)
			if err != nil {
				return fmt.Errorf("send repeated identical health report: %w", err)
			}
			for readIdx := range 2 {
				repeatPage, rawRepeat, qErr := s.queryAlarmList(ctx, "")
				if qErr != nil {
					return qErr
				}
				if repeatPage.Total != 2 {
					return fmt.Errorf("repeat read %d: total=%d, want 2: %s", readIdx+1, repeatPage.Total, rawRepeat)
				}
				repBGP, foundBGP := findOpenAlarm(repeatPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
				if !foundBGP {
					return fmt.Errorf("repeat read %d: bgp alarm missing: %s", readIdx+1, rawRepeat)
				}
				if err := requireSameAlarmIdentity("repeat read bgp", &bgpAlarm, &repBGP); err != nil {
					return err
				}
				repIface, foundIface := findOpenAlarm(repeatPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompIface)
				if !foundIface {
					return fmt.Errorf("repeat read %d: iface alarm missing: %s", readIdx+1, rawRepeat)
				}
				if err := requireSameAlarmIdentity("repeat read iface", &ifaceAlarm, &repIface); err != nil {
					return err
				}
			}
			histAfterRepeat, rawHistRepeat, err := s.queryAlarmHistory(ctx, caTrustClientName, "")
			if err != nil {
				return err
			}
			if histAfterRepeat.Total != 2 {
				return fmt.Errorf("history after repeat report has total=%d, want 2 (no duplicates): %s", histAfterRepeat.Total, rawHistRepeat)
			}

			// Step 4: Send high CPU samples (cpu-used=95) spanning >= 2s to open resource-high.
			firstCPUAlarm, err := s.driveCPUAlarmOpen(ctx, client, caTrustClientName, fleetAlarmHighCPUFirst, "initial high CPU qualification")
			if err != nil {
				return err
			}
			if err := requireAlarmIDAndSince("resource-high", &firstCPUAlarm); err != nil {
				return err
			}
			if firstCPUAlarm.Severity != pluginserver.AlarmSeverityWarning {
				return fmt.Errorf("resource-high severity=%q, want warning", firstCPUAlarm.Severity)
			}

			// Step 5: Disconnect edge-01 and verify device-offline opens after
			// offline-delay while already-open component-down and resource-high
			// retain their original IDs and opening times.
			if err := client.close(); err != nil {
				return fmt.Errorf("disconnect edge-01: %w", err)
			}
			client = nil

			offlinePage, rawOffline, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				_, found := findOpenAlarm(p.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
				return found
			}, "device-offline after disconnect")
			if err != nil {
				return err
			}
			offlineAlarm, _ := findOpenAlarm(offlinePage.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
			if offlinePage.Total != 4 {
				return fmt.Errorf("while offline total=%d, want 4: %s", offlinePage.Total, rawOffline)
			}
			if offlinePage.Counts.Critical != 3 {
				return fmt.Errorf("while offline counts.critical=%d, want 3: %s", offlinePage.Counts.Critical, rawOffline)
			}
			if offlinePage.Counts.Warning != 1 {
				return fmt.Errorf("while offline counts.warning=%d, want 1: %s", offlinePage.Counts.Warning, rawOffline)
			}
			offlineChecks := []struct {
				label string
				want  pluginserver.AlarmOccurrence
			}{
				{fleetAlarmCompBGP, bgpAlarm},
				{fleetAlarmCompIface, ifaceAlarm},
				{fleetAlarmSubjectCPU, firstCPUAlarm},
			}
			for i := range offlineChecks {
				check := &offlineChecks[i]
				got, found := findOpenAlarm(offlinePage.Alarms, check.want.Kind, check.want.Device, check.want.Subject)
				if !found {
					return fmt.Errorf("while offline %s alarm disappeared: %s", check.label, rawOffline)
				}
				if err := requireSameAlarmIdentity("while offline "+check.label, &check.want, &got); err != nil {
					return err
				}
			}

			// Step 6 (AC-10): Reconnect edge-01, keep bgp down, recover iface, and
			// send a below-threshold CPU sample.
			client, err = dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return fmt.Errorf("reconnect edge-01: %w", err)
			}
			bgpOnlyDown := makeAlarmHealthReport([]fleet.HealthComponent{
				{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusDown, Reason: "bgp engine still down"},
				{Name: fleetAlarmCompIface, Status: fleet.HealthStatusHealthy, Reason: "uplink recovered"},
			})
			if err := client.sendHealth(ctx, bgpOnlyDown); err != nil {
				return fmt.Errorf("send post-reconnect health report: %w", err)
			}
			if err := sendCPUSampleAccepted(ctx, client, fleetAlarmLowCPU); err != nil {
				return fmt.Errorf("send post-reconnect below-threshold CPU sample: %w", err)
			}

			reconnPage, rawReconn, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				return p.Total == 1 && len(p.Alarms) == 1
			}, "post-reconnect alarm reconciliation")
			if err != nil {
				return err
			}
			if reconnPage.Counts.Critical != 1 {
				return fmt.Errorf("post-reconnect counts.critical=%d, want 1: %s", reconnPage.Counts.Critical, rawReconn)
			}
			if reconnPage.Counts.Warning != 0 {
				return fmt.Errorf("post-reconnect counts.warning=%d, want 0: %s", reconnPage.Counts.Warning, rawReconn)
			}
			reconnBGP, foundBGP := findOpenAlarm(reconnPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
			if !foundBGP {
				return fmt.Errorf("post-reconnect bgp component-down missing: %s", rawReconn)
			}
			if err := requireSameAlarmIdentity("post-reconnect bgp", &bgpAlarm, &reconnBGP); err != nil {
				return err
			}

			// Confirm prior resource-high, iface component-down, and device-offline
			// are recorded once each as cleared in history with their original IDs.
			histAfterReconn, rawHistReconn, err := s.queryAlarmHistory(ctx, caTrustClientName, "")
			if err != nil {
				return err
			}
			for _, closedCheck := range []struct {
				label string
				id    string
			}{
				{"prior resource-high", firstCPUAlarm.ID},
				{"recovered iface component-down", ifaceAlarm.ID},
				{"cleared device-offline", offlineAlarm.ID},
			} {
				n, ev := countHistoryByID(histAfterReconn.Events, closedCheck.id)
				if n != 1 {
					return fmt.Errorf("history after reconnect has %d rows for %s (%s), want 1: %s",
						n, closedCheck.label, closedCheck.id, rawHistReconn)
				}
				if !ev.Cleared {
					return fmt.Errorf("history row for %s (%s) has cleared=false, want true: %s",
						closedCheck.label, closedCheck.id, rawHistReconn)
				}
				if ev.ClearedAt.IsZero() {
					return fmt.Errorf("history row for %s (%s) missing cleared-at: %s",
						closedCheck.label, closedCheck.id, rawHistReconn)
				}
			}

			// Step 7: Send fresh qualifying high CPU samples (cpu-used=96) after
			// reconnect; confirm a single sample does not immediately open
			// resource-high, then confirm fresh sustained qualification opens a NEW
			// ID with a strictly later since.
			if err := sendCPUSampleAccepted(ctx, client, fleetAlarmHighCPUSecond); err != nil {
				return fmt.Errorf("send first post-reconnect high CPU sample: %w", err)
			}
			singleSamplePage, rawSingleSample, err := s.queryAlarmList(ctx, " | kind "+pluginserver.AlarmKindResourceHigh)
			if err != nil {
				return err
			}
			if singleSamplePage.Total != 0 {
				return fmt.Errorf("resource-high opened after a single post-reconnect sample before resource-duration elapsed: %s", rawSingleSample)
			}

			secondCPUAlarm, err := s.driveCPUAlarmOpen(ctx, client, caTrustClientName, fleetAlarmHighCPUSecond, "post-reconnect high CPU qualification")
			if err != nil {
				return err
			}
			if secondCPUAlarm.ID == firstCPUAlarm.ID {
				return fmt.Errorf("post-reconnect resource-high reused prior id %q", firstCPUAlarm.ID)
			}
			if !secondCPUAlarm.Since.After(firstCPUAlarm.Since) {
				return fmt.Errorf("post-reconnect resource-high since %s is not after prior since %s",
					secondCPUAlarm.Since, firstCPUAlarm.Since)
			}

			// Step 8: Verify generic pipes (| count, | match bgp, | table) on show fleet alarm.
			if err := verifyLifecyclePipes(ctx, s, 2, bgpAlarm.ID); err != nil {
				return err
			}

			return storageLiveOK("fleet alarm lifecycle preserved IDs across repeat and disconnect, cleared resource-high on reconnect, and required fresh qualification")
		})
}

func driveUnreadHistoryTransitions(ctx context.Context, hubAddr string) (*fleetAlarmClient, error) {
	// Zero calls to show fleet alarm or show fleet alarm history occur anywhere
	// in this sequence, proving transitions are recorded at the producer boundary.
	healthy := makeAlarmHealthReport([]fleet.HealthComponent{
		{Name: fleetAlarmCompOne, Status: fleet.HealthStatusHealthy},
		{Name: fleetAlarmCompTwo, Status: fleet.HealthStatusHealthy},
		{Name: fleetAlarmCompThree, Status: fleet.HealthStatusHealthy},
		{Name: fleetAlarmCompFour, Status: fleet.HealthStatusHealthy},
		{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusHealthy},
	})
	comp1Down := makeAlarmHealthReport([]fleet.HealthComponent{
		{Name: fleetAlarmCompOne, Status: fleet.HealthStatusDown, Reason: "episode 1 down"},
	})
	comp2Down := makeAlarmHealthReport([]fleet.HealthComponent{
		{Name: fleetAlarmCompTwo, Status: fleet.HealthStatusDown, Reason: "episode 2 down"},
	})
	comp3Down := makeAlarmHealthReport([]fleet.HealthComponent{
		{Name: fleetAlarmCompThree, Status: fleet.HealthStatusDown, Reason: "episode 3 down"},
	})
	comp4AndBGPDown := makeAlarmHealthReport([]fleet.HealthComponent{
		{Name: fleetAlarmCompFour, Status: fleet.HealthStatusDown, Reason: "episode 4 down"},
		{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusDown, Reason: "bgp stays open across restart"},
	})
	bgpOnlyDown := makeAlarmHealthReport([]fleet.HealthComponent{
		{Name: fleetAlarmCompFour, Status: fleet.HealthStatusHealthy, Reason: "episode 4 cleared"},
		{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusDown, Reason: "bgp stays open across restart"},
	})

	steps := []fleet.HealthReport{
		comp1Down,       // open comp-1
		comp1Down,       // repeat comp-1 (idempotent)
		healthy,         // clear comp-1 (closed #1)
		comp2Down,       // open comp-2
		healthy,         // clear comp-2 (closed #2)
		comp3Down,       // open comp-3
		healthy,         // clear comp-3 (closed #3)
		comp4AndBGPDown, // open comp-4 and bgp
		bgpOnlyDown,     // clear comp-4 (closed #4 -> evicts comp-1), leave bgp open
	}

	var client *fleetAlarmClient
	for i, rep := range steps {
		next, err := reconnectAndSendHealth(ctx, hubAddr, client, rep)
		if err != nil {
			return nil, fmt.Errorf("unread transition step %d: %w", i+1, err)
		}
		client = next
	}
	return client, nil
}

func verifyHistoryPagination(
	ctx context.Context,
	s *fleetHoldScenario,
	device string,
	full *pluginserver.AlarmHistoryPage,
) error {
	firstPage, rawFirst, err := s.queryAlarmHistory(ctx, device, " | limit "+strconv.Itoa(fleetAlarmPageLimit))
	if err != nil {
		return err
	}
	if len(firstPage.Events) != fleetAlarmPageLimit {
		return fmt.Errorf("history limit %d returned %d events, want %d: %s",
			fleetAlarmPageLimit, len(firstPage.Events), fleetAlarmPageLimit, rawFirst)
	}
	if !firstPage.HasMore {
		return fmt.Errorf("history limit %d returned has-more=false for total=%d: %s",
			fleetAlarmPageLimit, firstPage.Total, rawFirst)
	}
	if firstPage.Cursor == "" {
		return fmt.Errorf("history limit %d returned empty cursor with has-more=true: %s",
			fleetAlarmPageLimit, rawFirst)
	}
	if firstPage.Total != full.Total {
		return fmt.Errorf("history limit %d total=%d, want %d: %s",
			fleetAlarmPageLimit, firstPage.Total, full.Total, rawFirst)
	}

	collected := append([]pluginserver.AlarmHistoryEvent(nil), firstPage.Events...)
	cursor := firstPage.Cursor
	hasMore := firstPage.HasMore
	for pageNum := 2; hasMore && pageNum <= fleetAlarmMaxPages; pageNum++ {
		var pipes textbuf.Buffer
		pipes.Str(" | limit ").Int(fleetAlarmPageLimit).Str(" | cursor ").Str(cursor)
		nextPage, rawNext, qErr := s.queryAlarmHistory(ctx, device, pipes.String())
		if qErr != nil {
			return fmt.Errorf("history cursor page %d: %w", pageNum, qErr)
		}
		if nextPage.Total != full.Total {
			return fmt.Errorf("history cursor page %d total=%d, want %d: %s", pageNum, nextPage.Total, full.Total, rawNext)
		}
		collected = append(collected, nextPage.Events...)
		cursor = nextPage.Cursor
		hasMore = nextPage.HasMore
	}
	if hasMore {
		return fmt.Errorf("history pagination did not terminate within %d pages", fleetAlarmMaxPages)
	}
	if len(collected) != len(full.Events) {
		return fmt.Errorf("paginated history collected %d events, want %d", len(collected), len(full.Events))
	}
	for i := range full.Events {
		if collected[i].ID != full.Events[i].ID {
			return fmt.Errorf("paginated event[%d] id=%q, want %q", i, collected[i].ID, full.Events[i].ID)
		}
	}
	return nil
}

// managedFleetAlarmHistoryDriver validates fleet-ui-5 AC-12, AC-13, and AC-14
// against a real ze hub daemon with disk ZeFS and reduced retention budget
// (offline-delay=5, max-episodes-per-device=3, max-episodes-hub=64): an
// unconnected declared device (edge-02) opens device-offline using its original
// pre-restart declaration timestamp across a hub restart; unread open/repeat/
// clear/reopen transitions on edge-01 are recorded and evicted oldest-first
// under the retention cap; open alarm identity and bounded truncated history
// survive hub restart and paginate via cursor; clearing the open alarm after
// restart records its observed clear in history.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetAlarmHistoryDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-alarm-history", args, false, fleetAlarmHistoryHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			// Step 1 (AC-14): edge-02 never connects. Restart hub quickly and verify
			// edge-02 opens device-offline preserving its pre-restart declared-at.
			boot1Observed := time.Now().UTC()
			if err := s.restartHub(ctx); err != nil {
				return fmt.Errorf("quick hub restart for AC-14 grace persistence: %w", err)
			}
			edge02Page, rawEdge02, err := s.waitAlarmList(ctx,
				" | device "+fleetAlarmPeerName+" | kind "+pluginserver.AlarmKindDeviceOffline,
				func(p *pluginserver.AlarmListPage) bool {
					_, found := findOpenAlarm(p.Alarms, pluginserver.AlarmKindDeviceOffline, fleetAlarmPeerName, "")
					return found
				},
				"edge-02 device-offline after grace expiry across restart",
			)
			if err != nil {
				return err
			}
			edge02Offline, _ := findOpenAlarm(edge02Page.Alarms, pluginserver.AlarmKindDeviceOffline, fleetAlarmPeerName, "")
			if edge02Offline.Severity != pluginserver.AlarmSeverityCritical {
				return fmt.Errorf("edge-02 device-offline severity=%q, want critical: %s", edge02Offline.Severity, rawEdge02)
			}
			declaredAt, err := parseDeclaredAtDetail(edge02Offline.Detail)
			if err != nil {
				return fmt.Errorf("edge-02 device-offline detail: %w: %s", err, rawEdge02)
			}
			if declaredAt.After(boot1Observed.Truncate(time.Second)) {
				return fmt.Errorf("edge-02 declared-at %s is after pre-restart observation %s (restart reset grace timer): %s",
					declaredAt, boot1Observed.Truncate(time.Second), rawEdge02)
			}

			// Step 2: Drive opening, repeat, clear, and reopen (4 closed episodes + 1
			// open bgp) on edge-01 WITHOUT calling show fleet alarm or show fleet
			// alarm history in between.
			client, err := driveUnreadHistoryTransitions(ctx, s.hubAddress)
			if err != nil {
				return err
			}
			defer func() {
				if client != nil {
					_ = client.close() //nolint:errcheck // best-effort cleanup on return
				}
			}()

			// Capture open bgp identity once after unread transitions finish so we
			// can prove exact ID and since preservation across hub restart (AC-12).
			beforePage, rawBefore, err := s.waitAlarmList(ctx,
				" | device "+caTrustClientName+" | kind "+pluginserver.AlarmKindComponentDown,
				func(p *pluginserver.AlarmListPage) bool {
					_, found := findOpenAlarm(p.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
					return found
				},
				"edge-01 open bgp alarm before hub restart",
			)
			if err != nil {
				return err
			}
			openBefore, _ := findOpenAlarm(beforePage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
			if err := requireAlarmIDAndSince("open bgp alarm before restart", &openBefore); err != nil {
				return fmt.Errorf("%w: %s", err, rawBefore)
			}

			// Step 3: Disconnect edge-01 cleanly and restart the real ze hub daemon.
			if err := client.close(); err != nil {
				return fmt.Errorf("disconnect edge-01 before hub restart: %w", err)
			}
			client = nil
			if err := s.restartHub(ctx); err != nil {
				return fmt.Errorf("restart hub to verify alarm persistence: %w", err)
			}

			// Step 4 (AC-12, AC-13): Verify open alarm survived restart with exact ID
			// and since, closed episodes survived bounded to <= 3 with oldest evicted,
			// truncated=true, retained-since populated, and cursor pagination works.
			afterOpenPage, rawAfterOpen, err := s.queryAlarmList(ctx, "")
			if err != nil {
				return err
			}
			openAfter, foundAfter := findOpenAlarm(afterOpenPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
			if !foundAfter {
				return fmt.Errorf("open bgp alarm did not survive hub restart: %s", rawAfterOpen)
			}
			if err := requireSameAlarmIdentity("open bgp alarm across restart", &openBefore, &openAfter); err != nil {
				return fmt.Errorf("%w: %s", err, rawAfterOpen)
			}

			histAfter, rawHistAfter, err := s.queryAlarmHistory(ctx, caTrustClientName, "")
			if err != nil {
				return err
			}
			if !histAfter.Truncated {
				return fmt.Errorf("alarm history after restart has truncated=false, want true: %s", rawHistAfter)
			}
			if histAfter.RetainedSince.IsZero() {
				return fmt.Errorf("alarm history after restart missing retained-since: %s", rawHistAfter)
			}

			closedSubjects := make(map[string]bool)
			closedCount := 0
			sawOpenBGP := false
			for i := range histAfter.Events {
				ev := histAfter.Events[i]
				if ev.Cleared {
					closedCount++
					if ev.Kind == pluginserver.AlarmKindComponentDown {
						closedSubjects[ev.Subject] = true
					}
					continue
				}
				if ev.ID == openBefore.ID && ev.Subject == fleetAlarmCompBGP && ev.Opened.Equal(openBefore.Since) {
					sawOpenBGP = true
				}
			}
			if !sawOpenBGP {
				return fmt.Errorf("alarm history after restart missing open bgp occurrence (%s,%s): %s",
					openBefore.ID, openBefore.Since, rawHistAfter)
			}
			if closedCount > fleetAlarmMaxEpisodesDevice {
				return fmt.Errorf("closed episodes after restart=%d, want <= %d: %s",
					closedCount, fleetAlarmMaxEpisodesDevice, rawHistAfter)
			}
			if closedCount != fleetAlarmMaxEpisodesDevice {
				return fmt.Errorf("closed episodes after restart=%d, want %d: %s",
					closedCount, fleetAlarmMaxEpisodesDevice, rawHistAfter)
			}
			if closedSubjects[fleetAlarmCompOne] {
				return fmt.Errorf("oldest closed episode %s was not evicted under cap %d: %s",
					fleetAlarmCompOne, fleetAlarmMaxEpisodesDevice, rawHistAfter)
			}
			for _, wantSubject := range []string{fleetAlarmCompTwo, fleetAlarmCompThree, fleetAlarmCompFour} {
				if !closedSubjects[wantSubject] {
					return fmt.Errorf("expected retained closed episode %s missing after restart: %s", wantSubject, rawHistAfter)
				}
			}

			if err := verifyHistoryPagination(ctx, s, caTrustClientName, &histAfter); err != nil {
				return err
			}

			// Step 5: Reconnect edge-01, send healthy report to clear bgp, and verify
			// its observed clear is recorded in history with the preserved ID.
			client, err = dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return fmt.Errorf("reconnect edge-01 after restart: %w", err)
			}
			allHealthy := makeAlarmHealthReport([]fleet.HealthComponent{
				{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusHealthy, Reason: "bgp recovered after restart"},
			})
			if err := client.sendHealth(ctx, allHealthy); err != nil {
				return fmt.Errorf("send post-restart healthy report: %w", err)
			}
			if _, _, err := s.waitAlarmList(ctx,
				" | device "+caTrustClientName+" | kind "+pluginserver.AlarmKindComponentDown,
				func(p *pluginserver.AlarmListPage) bool {
					return p.Total == 0 && len(p.Alarms) == 0
				},
				"bgp component-down cleared after restart",
			); err != nil {
				return err
			}

			finalHist, rawFinalHist, err := s.queryAlarmHistory(ctx, caTrustClientName, "")
			if err != nil {
				return err
			}
			n, bgpClosed := countHistoryByID(finalHist.Events, openBefore.ID)
			if n != 1 {
				return fmt.Errorf("final history has %d rows for cleared bgp (%s), want 1: %s", n, openBefore.ID, rawFinalHist)
			}
			if !bgpClosed.Cleared {
				return fmt.Errorf("final history row for bgp (%s) has cleared=false, want true: %s", openBefore.ID, rawFinalHist)
			}
			if bgpClosed.ClearedAt.IsZero() {
				return fmt.Errorf("final history row for bgp (%s) missing cleared-at: %s", openBefore.ID, rawFinalHist)
			}
			if bgpClosed.ClearedAt.Before(openBefore.Since) {
				return fmt.Errorf("final history row for bgp (%s) cleared-at %s precedes opened %s: %s",
					openBefore.ID, bgpClosed.ClearedAt, openBefore.Since, rawFinalHist)
			}

			return storageLiveOK("fleet alarm history recorded unread transitions, survived hub restart with open IDs, enforced truncation cap, and paginated via cursor")
		})
}
