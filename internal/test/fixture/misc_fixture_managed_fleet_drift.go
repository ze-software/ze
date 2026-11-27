// Design: docs/architecture/fleet-config.md -- live SHA-1 hash chain drift, diff delivery, full-replay fallback, and config-resync-failed alarm fixtures
// Related: register_managed_fleet_drift.go -- registers managed/fleet-drift-hash-chain and managed/fleet-resync-failed-alarm
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRunWithHubConfig provisions the real ze hub and managed device stores
// Related: misc_fixture_managed_fleet_alarms.go -- waitAlarmList, queryAlarmList, findOpenAlarm, and alarm JSON helpers
// Related: misc_fixture_managed_fleet_alarm_causes.go -- fleetAlarmCausesHubConfig, alarm cause history decoding, and hidden pipe constants

package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/managed"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	fleetDriftHashChainFixtureName   = "managed/fleet-drift-hash-chain"
	fleetResyncFailedAlarmFixture    = "managed/fleet-resync-failed-alarm"
	fleetDriftRouterIDV1             = "2.2.2.2"
	fleetDriftRouterIDV2             = "3.3.3.3"
	fleetDriftRouterIDV3             = "4.4.4.4"
	fleetDriftStaleChain             = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fleetDriftMissingBaseVersion     = "0000000000000000"
	fleetDriftCommitFailureReason    = "confirmed commit window is open"
	fleetDriftDiffFailureReason      = "diff verification or apply failed on client"
	fleetDriftDoctorCode             = pluginserver.CodeDoctorManagedChainDrift
	fleetDriftDoctorFix              = "ze show doctor"
	fleetDriftPipeKindResyncFailed   = " | kind " + managed.AlarmKindConfigResyncFailed
	fleetDriftStoreChainPollAttempts = 80
)

// fleetDriftStatusAnswer decodes `show fleet status | json compact` including
// the SHA-1 hash chain fields added by spec-fleet-10.
type fleetDriftStatusAnswer struct {
	Managed        bool   `json:"managed"`
	ActiveHash     string `json:"active-hash"`
	BaselineHash   string `json:"baseline-hash"`
	Diverged       bool   `json:"diverged"`
	DaemonAnswered bool   `json:"daemon-answered"`
	Client         string `json:"client"`
	Connection     string `json:"connection"`
	Holding        string `json:"holding"`
	ChainHash      string `json:"chain-hash"`
	ChainState     string `json:"chain-state"`
	LastResync     string `json:"last-resync"`
}

// fleetDriftSession holds one controlled TLS+MuxConn managed client session to
// the real ze hub daemon without performing automatic initial fetch/ack so
// fixtures drive exact config-fetch, config-ack, and ping exchanges.
// Callers MUST invoke close before returning.
type fleetDriftSession struct {
	conn   net.Conn
	mc     *rpc.MuxConn
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (c *fleetDriftSession) serveHubRequests(ctx context.Context) {
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
// to exit. Idempotent. MUST be called by the owner of every fleetDriftSession.
func (c *fleetDriftSession) close() error {
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

func (c *fleetDriftSession) fetch(ctx context.Context, req fleet.ConfigFetchRequest) (fleet.ConfigFetchResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
	defer cancel()
	raw, err := c.mc.CallRPC(callCtx, fleet.VerbConfigFetch, req)
	if err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("config-fetch RPC: %w", err)
	}
	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("decode config-fetch reply: %w", err)
	}
	return resp, nil
}

func (c *fleetDriftSession) ack(ctx context.Context, ack fleet.ConfigAck) (fleet.ManagedReply, error) {
	callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
	defer cancel()
	raw, err := c.mc.CallRPC(callCtx, fleet.VerbConfigAck, ack)
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("config-ack RPC: %w", err)
	}
	reply, err := fleet.DecodeManagedReply(raw)
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("decode config-ack reply: %w", err)
	}
	return reply, nil
}

func (c *fleetDriftSession) ping(ctx context.Context, chainHash string) (fleet.ManagedReply, error) {
	callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
	defer cancel()
	raw, err := c.mc.CallRPC(callCtx, fleet.VerbPing, fleet.PingRequest{ChainHash: chainHash})
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("ping RPC: %w", err)
	}
	reply, err := fleet.DecodeManagedReply(raw)
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("decode ping reply: %w", err)
	}
	return reply, nil
}

func dialFleetDriftSessionOnce(ctx context.Context, addr, secret string) (*fleetDriftSession, error) {
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
	sessCtx, sessCancel := context.WithCancel(ctx)
	sess := &fleetDriftSession{
		conn:   tlsConn,
		mc:     mc,
		cancel: sessCancel,
		done:   make(chan struct{}),
	}
	go sess.serveHubRequests(sessCtx)
	return sess, nil
}

func dialFleetDriftSession(ctx context.Context, addr, secret string) (*fleetDriftSession, error) {
	var sess *fleetDriftSession
	var lastErr error
	connected := Poll(ctx, fleetAlarmConnectAttempts, fleetAlarmConnectDelay, func() bool {
		c, err := dialFleetDriftSessionOnce(ctx, addr, secret)
		if err != nil {
			lastErr = err
			return false
		}
		sess = c
		return true
	})
	if !connected {
		return nil, fmt.Errorf("managed drift session %s failed to connect to %s: %w", caTrustClientName, addr, lastErr)
	}
	return sess, nil
}

// fleetDriftConfig builds a valid managed client configuration padded with
// comment lines so single-line router-id updates produce a compact diff smaller
// than the full config (ComputeDiff worthSending == true).
func fleetDriftConfig(s *fleetHoldScenario, routerID string) string {
	var b textbuf.Buffer
	b.Str(s.deviceListener)
	for i := range 14 {
		b.Str("# fleet-drift-hash-chain padding line ").Int(int64(i)).Str(" keeps line diffs compact\n")
	}
	b.Str(restoreClientConfig(s.managedPort, routerID))
	return b.String()
}

// fleetDriftFullOnlyConfig builds a valid managed client configuration using
// tab indentation instead of spaces and no shared padding lines so ComputeDiff
// from fleetDriftConfig produces a JSON delta larger than the target bytes
// (worthSending == false), exercising the hub's direct Mode=="full" fallback.
func fleetDriftFullOnlyConfig(s *fleetHoldScenario, routerID string) string {
	var b textbuf.Buffer
	b.Str(strings.ReplaceAll(s.deviceListener, "    ", "\t"))
	b.Str("bus {\n\thub {\n")
	b.Str("\t\tclient ").Str(caTrustClientName).Str(" { host 127.0.0.1; port ").Str(s.managedPort).
		Str("; secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("\t}\n}\n")
	b.Str("bgp {\n\trouter-id ").Str(routerID).Str("\n\tsession { asn { local 65000; } }\n}\n")
	return b.String()
}

func askDriftFleetStatus(ctx context.Context, env []string) (fleetDriftStatusAnswer, error) {
	code, stdout, stderr, err := runCaptured(ctx, env, "", "ze", "cli", "-c", fleetStatusCommand)
	if err != nil {
		return fleetDriftStatusAnswer{}, fmt.Errorf("run ze cli -c %q: %w", fleetStatusCommand, err)
	}
	if code != 0 {
		return fleetDriftStatusAnswer{}, fmt.Errorf("ze cli -c %q exited %d:\n%s%s", fleetStatusCommand, code, stdout, stderr)
	}
	var answer fleetDriftStatusAnswer
	if err := json.Unmarshal([]byte(stdout), &answer); err != nil {
		return fleetDriftStatusAnswer{}, fmt.Errorf("decode %q answer: %w:\n%s", fleetStatusCommand, err, stdout)
	}
	return answer, nil
}

func waitDriftFleetStatus(
	ctx context.Context,
	daemon *cachedBootDaemon,
	env []string,
	desc string,
	pred func(fleetDriftStatusAnswer) bool,
) (fleetDriftStatusAnswer, error) {
	var last fleetDriftStatusAnswer
	var lastErr error
	reached := cachedBootWait(ctx, daemon, fleetHoldFollowAttempts, func() bool {
		last, lastErr = askDriftFleetStatus(ctx, env)
		if lastErr != nil {
			return false
		}
		if !last.DaemonAnswered {
			return false
		}
		return pred(last)
	})
	if !reached {
		return fleetDriftStatusAnswer{}, fmt.Errorf("%s did not reach expected fleet status (err=%v): %+v", desc, lastErr, last)
	}
	return last, nil
}

func readDaemonChainKey(ctx context.Context, daemon *caTrustDaemon, key string) (fleet.ChainState, error) {
	stdout, stderr, code, err := rawCommandStreams(ctx, daemon.dir, daemon.env,
		"ze", "data", "--path", daemon.dbPath, "cat", key)
	if err != nil {
		return fleet.ChainState{}, err
	}
	if code != 0 {
		return fleet.ChainState{}, fmt.Errorf("ze data cat %s exited %d: %s", key, code, stderr)
	}
	var st fleet.ChainState
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &st); err != nil {
		return fleet.ChainState{}, fmt.Errorf("decode %s (%q): %w", key, strings.TrimSpace(stdout), err)
	}
	return st, nil
}

func waitDaemonChainKey(
	ctx context.Context,
	daemon *caTrustDaemon,
	key, wantVersion, wantChain, desc string,
) error {
	var last fleet.ChainState
	var lastErr error
	ok := Poll(ctx, fleetDriftStoreChainPollAttempts, fleetStatusPollDelay, func() bool {
		last, lastErr = readDaemonChainKey(ctx, daemon, key)
		if lastErr != nil {
			return false
		}
		return last.Version == wantVersion && last.Chain == wantChain
	})
	if !ok {
		return fmt.Errorf("%s: store key %s did not converge to {version:%s chain:%s} (got %+v, err=%v)",
			desc, key, wantVersion, wantChain, last, lastErr)
	}
	return nil
}

func waitHubClientDisconnected(ctx context.Context, s *fleetHoldScenario, device string) error {
	disconnected := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		raw, err := storageLiveRequest(ctx, s.hubOperator, "show fleet device name "+device+fleetJSONCompactSuffix)
		if err != nil {
			return false
		}
		var detail struct {
			Connected *bool `json:"connected"`
		}
		if decodeAlarmJSON(raw, &detail) != nil {
			return false
		}
		if detail.Connected == nil {
			return false
		}
		return !*detail.Connected
	})
	if !disconnected {
		return fmt.Errorf("hub never marked %s disconnected", device)
	}
	return nil
}

func verifyLivePingMismatchAndDiffReanchor(
	ctx context.Context,
	s *fleetHoldScenario,
	v1Bytes, v2Bytes []byte,
	v1, v2, c2 string,
) error {
	sess, err := dialFleetDriftSession(ctx, s.hubAddress, caTrustClientSecret)
	if err != nil {
		return err
	}
	defer func() {
		_ = sess.close() //nolint:errcheck // best-effort cleanup
	}()

	// Step A: ping with a stale chain hash receives mismatched C2 from the live hub.
	pingReply, err := sess.ping(ctx, fleetDriftStaleChain)
	if err != nil {
		return fmt.Errorf("mismatch ping: %w", err)
	}
	if pingReply.ChainHash != c2 {
		return fmt.Errorf("mismatch ping returned chain-hash=%q, want %q", pingReply.ChainHash, c2)
	}

	// Step B: without reconnecting, request resync=="diff" over historical base V1.
	diffResp, err := sess.fetch(ctx, fleet.ConfigFetchRequest{
		Version:   v1,
		ChainHash: fleetDriftStaleChain,
		Resync:    fleet.ResyncModeDiff,
	})
	if err != nil {
		return fmt.Errorf("reanchor diff fetch: %w", err)
	}
	if diffResp.Mode != fleet.FetchModeDiff {
		return fmt.Errorf("reanchor diff fetch mode=%q, want %q", diffResp.Mode, fleet.FetchModeDiff)
	}
	if !diffResp.Reanchor {
		return errors.New("reanchor diff fetch did not set reanchor=true for drifted client chain")
	}
	if diffResp.BaseVersion != v1 {
		return fmt.Errorf("reanchor diff fetch base=%q, want %q", diffResp.BaseVersion, v1)
	}
	if diffResp.Version != v2 {
		return fmt.Errorf("reanchor diff fetch target=%q, want %q", diffResp.Version, v2)
	}

	delta, err := base64.StdEncoding.DecodeString(diffResp.Diff)
	if err != nil {
		return fmt.Errorf("decode reanchor diff payload: %w", err)
	}
	reconstructed, err := fleet.ApplyDiff(v1Bytes, delta, rpc.MaxMessageSize)
	if err != nil {
		return fmt.Errorf("apply reanchor diff: %w", err)
	}
	if !bytes.Equal(reconstructed, v2Bytes) {
		return fmt.Errorf("reanchor diff reconstructed version %s, want exact V2 bytes (%s)",
			fleet.VersionHash(reconstructed), v2)
	}
	wantReanchorChain, err := fleet.FoldDiff(fleet.FoldFull(v1Bytes), v1, v2, delta)
	if err != nil {
		return fmt.Errorf("fold reanchor diff chain: %w", err)
	}
	if diffResp.ChainHash != wantReanchorChain {
		return fmt.Errorf("reanchor diff chain-hash=%q, want %q", diffResp.ChainHash, wantReanchorChain)
	}

	ackReply, err := sess.ack(ctx, fleet.ConfigAck{
		Version:   v2,
		OK:        true,
		ChainHash: wantReanchorChain,
	})
	if err != nil {
		return fmt.Errorf("reanchor diff ack: %w", err)
	}
	if ackReply.ChainHash != wantReanchorChain {
		return fmt.Errorf("reanchor ack reply chain-hash=%q, want %q", ackReply.ChainHash, wantReanchorChain)
	}

	// Step C: subsequent ping on the SAME connection agrees on the re-anchored chain.
	pingConverged, err := sess.ping(ctx, wantReanchorChain)
	if err != nil {
		return fmt.Errorf("post-reanchor ping: %w", err)
	}
	if pingConverged.ChainHash != wantReanchorChain {
		return fmt.Errorf("post-reanchor ping chain-hash=%q, want %q", pingConverged.ChainHash, wantReanchorChain)
	}
	hubChainKey := pluginserver.ManagedClientChainKey(caTrustClientName)
	return waitDaemonChainKey(ctx, s.hub, hubChainKey, v2, wantReanchorChain, "hub chain after live ping mismatch diff reanchor")
}

func verifyHubFullFallbackModes(
	ctx context.Context,
	s *fleetHoldScenario,
	v2, v3, c3Full string,
) error {
	sess, err := dialFleetDriftSession(ctx, s.hubAddress, caTrustClientSecret)
	if err != nil {
		return err
	}
	defer func() {
		_ = sess.close() //nolint:errcheck // best-effort cleanup
	}()

	// Missing base version falls back directly to Mode=="full" in one fetch.
	missingBaseResp, err := sess.fetch(ctx, fleet.ConfigFetchRequest{
		Version:   fleetDriftMissingBaseVersion,
		ChainHash: fleetDriftStaleChain,
		Resync:    fleet.ResyncModeDiff,
	})
	if err != nil {
		return fmt.Errorf("missing-base fetch: %w", err)
	}
	if missingBaseResp.Mode != fleet.FetchModeFull {
		return fmt.Errorf("missing-base fetch mode=%q, want %q", missingBaseResp.Mode, fleet.FetchModeFull)
	}
	if missingBaseResp.Version != v3 {
		return fmt.Errorf("missing-base fetch version=%q, want %q", missingBaseResp.Version, v3)
	}
	if missingBaseResp.ChainHash != c3Full {
		return fmt.Errorf("missing-base fetch chain=%q, want %q", missingBaseResp.ChainHash, c3Full)
	}

	// Explicit resync=="full" returns Mode=="full" even when historical base V2 exists.
	explicitFullResp, err := sess.fetch(ctx, fleet.ConfigFetchRequest{
		Version:   v2,
		ChainHash: fleetDriftStaleChain,
		Resync:    fleet.ResyncModeFull,
	})
	if err != nil {
		return fmt.Errorf("explicit full replay fetch: %w", err)
	}
	if explicitFullResp.Mode != fleet.FetchModeFull {
		return fmt.Errorf("explicit full replay fetch mode=%q, want %q", explicitFullResp.Mode, fleet.FetchModeFull)
	}
	if explicitFullResp.Version != v3 {
		return fmt.Errorf("explicit full replay fetch version=%q, want %q", explicitFullResp.Version, v3)
	}
	if explicitFullResp.ChainHash != c3Full {
		return fmt.Errorf("explicit full replay fetch chain=%q, want %q", explicitFullResp.ChainHash, c3Full)
	}
	return nil
}

// managedFleetDriftHashChainDriver validates spec-fleet-10 end to end against a
// real ze hub daemon and real managed ze device daemon on disk ZeFS: initial
// FoldFull chain anchoring on both ends, live V1->V2 diff delivery with
// FoldDiff convergence and show fleet status reporting, live ping chain-hash
// mismatch detection and reanchor healing without reconnect, and automatic
// full-replay fallback converging both ends to FoldFull(target).
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDriftHashChainDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, fleetDriftHashChainFixtureName, args, true, restoreClientHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			v1Bytes := []byte(fleetDriftConfig(s, fleetDriftRouterIDV1))
			v2Bytes := []byte(fleetDriftConfig(s, fleetDriftRouterIDV2))
			v3Bytes := []byte(fleetDriftFullOnlyConfig(s, fleetDriftRouterIDV3))

			v1 := fleet.VersionHash(v1Bytes)
			v2 := fleet.VersionHash(v2Bytes)
			v3 := fleet.VersionHash(v3Bytes)
			c1 := fleet.FoldFull(v1Bytes)
			c2Full := fleet.FoldFull(v2Bytes)
			c3Full := fleet.FoldFull(v3Bytes)

			delta12, worth12, err := fleet.ComputeDiff(v1Bytes, v2Bytes)
			if err != nil {
				return fmt.Errorf("compute V1->V2 diff: %w", err)
			}
			if !worth12 {
				return fmt.Errorf("expected V1->V2 diff to be smaller than target (len(delta)=%d, len(v2)=%d)", len(delta12), len(v2Bytes))
			}
			c2Diff, err := fleet.FoldDiff(c1, v1, v2, delta12)
			if err != nil {
				return fmt.Errorf("fold V1->V2 diff chain: %w", err)
			}
			if c2Diff == c2Full {
				return errors.New("V1->V2 FoldDiff unexpectedly matched FoldFull(V2)")
			}
			_, worth23, diffErr := fleet.ComputeDiff(v2Bytes, v3Bytes)
			if diffErr != nil {
				return fmt.Errorf("compute V2->V3 diff: %w", diffErr)
			}
			if worth23 {
				return errors.New("expected V2->V3 diff to fall back to full (worthSending=true)")
			}

			hubChainKey := pluginserver.ManagedClientChainKey(caTrustClientName)
			clientChainKey := zefs.KeyManagedChainState.Pattern

			// Step 1: Import V1 on hub, boot real managed device, verify FoldFull(V1)
			// anchors on both client and hub stores and in show fleet status.
			if err := s.hubImportBytes(ctx, v1Bytes); err != nil {
				return fmt.Errorf("import V1 on hub: %w", err)
			}

			if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
				st1, err := waitDriftFleetStatus(ctx, daemon, s.deviceOperator.env, "initial V1 anchor",
					func(ans fleetDriftStatusAnswer) bool {
						return ans.ActiveHash == v1 &&
							ans.BaselineHash == v1 &&
							ans.Connection == fleetStatusConnected &&
							ans.ChainHash == c1 &&
							ans.ChainState == managed.ChainStateInSync
					})
				if err != nil {
					return err
				}
				if st1.LastResync != managed.LastResyncFullReplay {
					return fmt.Errorf("initial V1 last-resync=%q, want %q", st1.LastResync, managed.LastResyncFullReplay)
				}
				if err := waitDaemonChainKey(ctx, s.device, clientChainKey, v1, c1, "device chain after V1"); err != nil {
					return err
				}
				if err := waitDaemonChainKey(ctx, s.hub, hubChainKey, v1, c1, "hub chain after V1"); err != nil {
					return err
				}

				// Step 2: Live hub import V1->V2 delivers Mode=="diff"; both ends
				// converge to FoldDiff chain C2 and show fleet status reports diff.
				if err := s.hubImportBytes(ctx, v2Bytes); err != nil {
					return fmt.Errorf("import V2 on hub: %w", err)
				}
				st2, err := waitDriftFleetStatus(ctx, daemon, s.deviceOperator.env, "live V1->V2 diff push",
					func(ans fleetDriftStatusAnswer) bool {
						return ans.ActiveHash == v2 &&
							ans.BaselineHash == v2 &&
							ans.ChainHash == c2Diff &&
							ans.ChainState == managed.ChainStateInSync &&
							ans.LastResync == managed.LastResyncDiff
					})
				if err != nil {
					return err
				}
				if st2.Diverged {
					return fmt.Errorf("device reported diverged after V2 diff apply: %+v", st2)
				}
				if err := waitDaemonChainKey(ctx, s.device, clientChainKey, v2, c2Diff, "device chain after V2 diff"); err != nil {
					return err
				}
				return waitDaemonChainKey(ctx, s.hub, hubChainKey, v2, c2Diff, "hub chain after V2 diff")
			}); err != nil {
				return err
			}

			// Verify stopped-daemon show fleet status still reports persisted C2.
			offlineStatus, err := askDriftFleetStatus(ctx, s.deviceOperator.env)
			if err != nil {
				return fmt.Errorf("offline show fleet status: %w", err)
			}
			if offlineStatus.DaemonAnswered {
				return fmt.Errorf("offline status unexpectedly answered by daemon: %+v", offlineStatus)
			}
			if offlineStatus.ChainHash != c2Diff {
				return fmt.Errorf("offline chain-hash=%q, want %q", offlineStatus.ChainHash, c2Diff)
			}
			if offlineStatus.ChainState != managed.ChainStateNoDaemon {
				return fmt.Errorf("offline chain-state=%q, want %q", offlineStatus.ChainState, managed.ChainStateNoDaemon)
			}

			if err := waitHubClientDisconnected(ctx, s, caTrustClientName); err != nil {
				return err
			}

			// Step 3: Live ping chain-hash mismatch heals over the same connection
			// via diff reanchor against the real running ze hub daemon.
			if err := verifyLivePingMismatchAndDiffReanchor(ctx, s, v1Bytes, v2Bytes, v1, v2, c2Diff); err != nil {
				return err
			}
			if err := waitHubClientDisconnected(ctx, s, caTrustClientName); err != nil {
				return err
			}

			// Step 3b + Step 4: Plant a stale chain hash on the stopped device,
			// boot it so hub+device heal to FoldFull(V2) via current reanchor,
			// then import V3 (full-replay fallback) and verify both ends converge
			// to FoldFull(V3) with last-resync=="full-replay".
			staleJSON, err := json.Marshal(fleet.ChainState{Version: v2, Chain: fleetDriftStaleChain})
			if err != nil {
				return fmt.Errorf("marshal stale chain state: %w", err)
			}
			if err := s.deviceOperator.writeKey(ctx, clientChainKey, string(staleJSON)); err != nil {
				return fmt.Errorf("write stale client chain state: %w", err)
			}

			if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
				if _, err := waitDriftFleetStatus(ctx, daemon, s.deviceOperator.env, "reanchor convergence to FoldFull(V2)",
					func(ans fleetDriftStatusAnswer) bool {
						return ans.ActiveHash == v2 &&
							ans.ChainHash == c2Full &&
							ans.ChainState == managed.ChainStateInSync
					}); err != nil {
					return err
				}
				if err := waitDaemonChainKey(ctx, s.device, clientChainKey, v2, c2Full, "device chain after current reanchor"); err != nil {
					return err
				}
				if err := waitDaemonChainKey(ctx, s.hub, hubChainKey, v2, c2Full, "hub chain after current reanchor"); err != nil {
					return err
				}

				// Step 4: Import V3 (uncompressible diff -> full replay fallback).
				if err := s.hubImportBytes(ctx, v3Bytes); err != nil {
					return fmt.Errorf("import V3 on hub: %w", err)
				}
				if _, err := waitDriftFleetStatus(ctx, daemon, s.deviceOperator.env, "full-replay convergence to V3",
					func(ans fleetDriftStatusAnswer) bool {
						return ans.ActiveHash == v3 &&
							ans.BaselineHash == v3 &&
							ans.ChainHash == c3Full &&
							ans.ChainState == managed.ChainStateInSync &&
							ans.LastResync == managed.LastResyncFullReplay
					}); err != nil {
					return err
				}
				if err := waitDaemonChainKey(ctx, s.device, clientChainKey, v3, c3Full, "device chain after V3 full replay"); err != nil {
					return err
				}
				return waitDaemonChainKey(ctx, s.hub, hubChainKey, v3, c3Full, "hub chain after V3 full replay")
			}); err != nil {
				return err
			}

			if err := waitHubClientDisconnected(ctx, s, caTrustClientName); err != nil {
				return err
			}
			if err := verifyHubFullFallbackModes(ctx, s, v2, v3, c3Full); err != nil {
				return err
			}

			return storageLiveOK("fleet drift hash chain anchored, applied diff update, healed reply mismatch, and fell back to full replay")
		})
}

func verifyResyncAlarmOpenDetails(
	occ *pluginserver.AlarmOccurrence,
	clientChain, expectedChain, raw string,
) error {
	if err := requireAlarmIDAndSince("config-resync-failed", occ); err != nil {
		return fmt.Errorf("%w: %s", err, raw)
	}
	if occ.Severity != pluginserver.AlarmSeverityWarning {
		return fmt.Errorf("config-resync-failed severity=%q, want warning: %s", occ.Severity, raw)
	}
	if occ.Scope != pluginserver.AlarmScopeDevice {
		return fmt.Errorf("config-resync-failed scope=%q, want device: %s", occ.Scope, raw)
	}
	if occ.HiddenBehind != "" {
		return fmt.Errorf("online config-resync-failed hidden-behind=%q, want empty: %s", occ.HiddenBehind, raw)
	}
	ts := occ.Since.UTC().Format(time.RFC3339)
	for _, want := range []string{
		caTrustClientName,
		ts,
		fleetDriftDiffFailureReason,
		fleetDriftCommitFailureReason,
		fleetDriftDoctorCode,
	} {
		if !strings.Contains(occ.Summary, want) {
			return fmt.Errorf("config-resync-failed summary missing %q: %q", want, occ.Summary)
		}
	}
	if !strings.HasSuffix(occ.Summary, fleetDriftDoctorFix) {
		return fmt.Errorf("config-resync-failed summary does not end with %q: %q", fleetDriftDoctorFix, occ.Summary)
	}
	for _, want := range []string{
		"diff=" + fleetDriftDiffFailureReason,
		"full=" + fleetDriftCommitFailureReason,
		"client-chain=" + clientChain,
		"expected-chain=" + expectedChain,
		"code=" + fleetDriftDoctorCode,
		"fix=" + fleetDriftDoctorFix,
	} {
		if !strings.Contains(occ.Detail, want) {
			return fmt.Errorf("config-resync-failed detail missing %q: %q", want, occ.Detail)
		}
	}
	return nil
}

func verifyResyncAlarmHiddenBehindOffline(
	ctx context.Context,
	s *fleetHoldScenario,
	orig *pluginserver.AlarmOccurrence,
) (pluginserver.AlarmOccurrence, error) {
	offlinePage, rawOffline, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
		if p.Total != 2 {
			return false
		}
		if len(p.Alarms) != 2 {
			return false
		}
		_, found := findOpenAlarm(p.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
		return found
	}, "device-offline opened while config-resync-failed active")
	if err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	if offlinePage.Counts.Visible != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline counts.visible=%d, want 1: %s", offlinePage.Counts.Visible, rawOffline)
	}
	if offlinePage.Counts.Hidden != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline counts.hidden=%d, want 1: %s", offlinePage.Counts.Hidden, rawOffline)
	}

	offlineAlarm, okOff := findOpenAlarm(offlinePage.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
	if !okOff {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("device-offline missing from alarm list: %s", rawOffline)
	}
	hiddenResync, okResync := findOpenAlarm(offlinePage.Alarms, managed.AlarmKindConfigResyncFailed, caTrustClientName, "")
	if !okResync {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("config-resync-failed missing while offline: %s", rawOffline)
	}
	if err := requireSameAlarmIdentity("offline config-resync-failed", orig, &hiddenResync); err != nil {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("%w: %s", err, rawOffline)
	}
	if hiddenResync.HiddenBehind != offlineAlarm.ID {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline config-resync-failed hidden-behind=%q, want %q: %s",
			hiddenResync.HiddenBehind, offlineAlarm.ID, rawOffline)
	}

	hiddenPage, rawHidden, err := s.queryAlarmList(ctx, fleetAlarmPipeHiddenTrue)
	if err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	if len(hiddenPage.Alarms) != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("hidden true returned %d alarms, want 1: %s", len(hiddenPage.Alarms), rawHidden)
	}
	if hiddenPage.Alarms[0].Kind != managed.AlarmKindConfigResyncFailed {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("hidden true kind=%q, want %q: %s", hiddenPage.Alarms[0].Kind, managed.AlarmKindConfigResyncFailed, rawHidden)
	}
	if hiddenPage.Alarms[0].HiddenBehind != offlineAlarm.ID {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("hidden true hidden-behind=%q, want %q: %s", hiddenPage.Alarms[0].HiddenBehind, offlineAlarm.ID, rawHidden)
	}

	visiblePage, rawVisible, err := s.queryAlarmList(ctx, fleetAlarmPipeHiddenFalse)
	if err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	if len(visiblePage.Alarms) != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("hidden false returned %d alarms, want 1: %s", len(visiblePage.Alarms), rawVisible)
	}
	if visiblePage.Alarms[0].Kind != pluginserver.AlarmKindDeviceOffline {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("hidden false kind=%q, want %q: %s", visiblePage.Alarms[0].Kind, pluginserver.AlarmKindDeviceOffline, rawVisible)
	}
	return offlineAlarm, nil
}

// managedFleetResyncFailedAlarmDriver validates spec-fleet-10 AC-1 and AC-2
// against a real ze hub daemon: a non-verify failure across diff resync and
// full replay opens Warning config-resync-failed without opening
// config-rejected, disconnecting past offline-delay hides it behind
// device-offline (selectable via | hidden true), and reconnecting with a
// successful full replay clears both alarms while retaining overlap history.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetResyncFailedAlarmDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, fleetResyncFailedAlarmFixture, args, false, fleetAlarmCausesHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			v1Bytes := []byte(fleetDriftConfig(s, fleetDriftRouterIDV1))
			v2Bytes := []byte(fleetDriftConfig(s, fleetDriftRouterIDV2))
			v1 := fleet.VersionHash(v1Bytes)
			v2 := fleet.VersionHash(v2Bytes)
			c1 := fleet.FoldFull(v1Bytes)
			c2Full := fleet.FoldFull(v2Bytes)

			if err := s.hubImportBytes(ctx, v1Bytes); err != nil {
				return fmt.Errorf("import V1 on hub: %w", err)
			}

			// Step 1: Connect edge-01 and converge initially on V1.
			sess, err := dialFleetDriftSession(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return err
			}
			defer func() {
				if sess != nil {
					_ = sess.close() //nolint:errcheck // best-effort cleanup
				}
			}()

			initResp, err := sess.fetch(ctx, fleet.ConfigFetchRequest{})
			if err != nil {
				return fmt.Errorf("initial V1 fetch: %w", err)
			}
			if initResp.Version != v1 {
				return fmt.Errorf("initial V1 fetch version=%q, want %q", initResp.Version, v1)
			}
			if initResp.ChainHash != c1 {
				return fmt.Errorf("initial V1 fetch chain=%q, want %q", initResp.ChainHash, c1)
			}
			if _, err := sess.ack(ctx, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1}); err != nil {
				return fmt.Errorf("initial V1 ack: %w", err)
			}
			if _, _, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				return p.Total == 0 && len(p.Alarms) == 0
			}, "initial zero alarms after V1 convergence"); err != nil {
				return err
			}

			// Step 2: Import V2 on hub, run diff fetch followed by full-replay fetch,
			// and reject non-verify with RejectKind=="commit".
			if err := s.hubImportBytes(ctx, v2Bytes); err != nil {
				return fmt.Errorf("import V2 on hub: %w", err)
			}
			diffResp, err := sess.fetch(ctx, fleet.ConfigFetchRequest{
				Version:   v1,
				ChainHash: c1,
				Resync:    fleet.ResyncModeDiff,
			})
			if err != nil {
				return fmt.Errorf("V2 diff fetch: %w", err)
			}
			if diffResp.Mode != fleet.FetchModeDiff {
				return fmt.Errorf("V2 diff fetch mode=%q, want %q", diffResp.Mode, fleet.FetchModeDiff)
			}
			fullResp, err := sess.fetch(ctx, fleet.ConfigFetchRequest{
				Version:   v1,
				ChainHash: c1,
				Resync:    fleet.ResyncModeFull,
			})
			if err != nil {
				return fmt.Errorf("V2 full replay fetch: %w", err)
			}
			if fullResp.Mode != fleet.FetchModeFull {
				return fmt.Errorf("V2 full replay fetch mode=%q, want %q", fullResp.Mode, fleet.FetchModeFull)
			}
			if fullResp.ChainHash != c2Full {
				return fmt.Errorf("V2 full replay fetch chain=%q, want %q", fullResp.ChainHash, c2Full)
			}
			if _, err := sess.ack(ctx, fleet.ConfigAck{
				Version:    v2,
				OK:         false,
				Error:      fleetDriftCommitFailureReason,
				RejectKind: fleet.RejectKindCommit,
				ChainHash:  c1,
			}); err != nil {
				return fmt.Errorf("V2 commit-failure ack: %w", err)
			}

			// Step 3: Verify config-resync-failed is OPEN in show fleet alarm and
			// config-rejected is NOT open.
			openPage, rawOpen, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				_, found := findOpenAlarm(p.Alarms, managed.AlarmKindConfigResyncFailed, caTrustClientName, "")
				return found
			}, "config-resync-failed alarm opened")
			if err != nil {
				return err
			}
			if openPage.Total != 1 {
				return fmt.Errorf("expected exactly 1 open alarm, got %d: %s", openPage.Total, rawOpen)
			}
			if _, hasRej := findOpenAlarm(openPage.Alarms, pluginserver.AlarmKindConfigRejected, caTrustClientName, ""); hasRej {
				return fmt.Errorf("non-verify commit failure unexpectedly opened config-rejected: %s", rawOpen)
			}
			origResync, _ := findOpenAlarm(openPage.Alarms, managed.AlarmKindConfigResyncFailed, caTrustClientName, "")
			if err := verifyResyncAlarmOpenDetails(&origResync, c1, c2Full, rawOpen); err != nil {
				return err
			}

			kindPage, rawKind, err := s.queryAlarmList(ctx, fleetDriftPipeKindResyncFailed)
			if err != nil {
				return err
			}
			if kindPage.Total != 1 {
				return fmt.Errorf("kind config-resync-failed total=%d, want 1: %s", kindPage.Total, rawKind)
			}
			if len(kindPage.Alarms) != 1 {
				return fmt.Errorf("kind config-resync-failed len=%d, want 1: %s", len(kindPage.Alarms), rawKind)
			}
			if kindPage.Alarms[0].ID != origResync.ID {
				return fmt.Errorf("kind config-resync-failed id=%q, want %q: %s", kindPage.Alarms[0].ID, origResync.ID, rawKind)
			}

			// Step 4: Stop edge-01 and verify config-resync-failed hides behind device-offline.
			if err := sess.close(); err != nil {
				return fmt.Errorf("disconnect edge-01: %w", err)
			}
			sess = nil

			offlineAlarm, err := verifyResyncAlarmHiddenBehindOffline(ctx, s, &origResync)
			if err != nil {
				return err
			}

			// Step 5: Reconnect edge-01 with fault cleared, apply V2, send ok:true
			// ack with C2Full, and verify both alarms clear while history retains overlap.
			sess, err = dialFleetDriftSession(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return fmt.Errorf("reconnect edge-01: %w", err)
			}
			reconnFetch, err := sess.fetch(ctx, fleet.ConfigFetchRequest{
				Version:   v1,
				ChainHash: c1,
				Resync:    fleet.ResyncModeFull,
			})
			if err != nil {
				return fmt.Errorf("reconnect full fetch: %w", err)
			}
			if reconnFetch.Version != v2 {
				return fmt.Errorf("reconnect fetch version=%q, want %q", reconnFetch.Version, v2)
			}
			if reconnFetch.ChainHash != c2Full {
				return fmt.Errorf("reconnect fetch chain=%q, want %q", reconnFetch.ChainHash, c2Full)
			}
			if _, err := sess.ack(ctx, fleet.ConfigAck{Version: v2, OK: true, ChainHash: c2Full}); err != nil {
				return fmt.Errorf("reconnect convergence ack: %w", err)
			}

			if _, _, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				return p.Total == 0 && len(p.Alarms) == 0
			}, "both device-offline and config-resync-failed cleared after convergence"); err != nil {
				return err
			}

			histPage, rawHist, err := queryAlarmCauseHistory(ctx, s, caTrustClientName)
			if err != nil {
				return err
			}
			nResync, resyncEv := findAlarmCauseHistoryEvent(histPage.Events, origResync.ID)
			if nResync != 1 {
				return fmt.Errorf("history has %d rows for config-resync-failed (%s), want 1: %s", nResync, origResync.ID, rawHist)
			}
			if !resyncEv.Cleared {
				return fmt.Errorf("config-resync-failed history row cleared=false, want true: %s", rawHist)
			}
			if !resyncEv.Hidden {
				return fmt.Errorf("config-resync-failed history row hidden=false, want true: %s", rawHist)
			}
			if resyncEv.HiddenBehind != offlineAlarm.ID {
				return fmt.Errorf("config-resync-failed history row hidden-behind=%q, want %q: %s", resyncEv.HiddenBehind, offlineAlarm.ID, rawHist)
			}

			return storageLiveOK("fleet resync-failed alarm opened on non-verify failure, hid behind device-offline, and cleared on convergence")
		})
}
