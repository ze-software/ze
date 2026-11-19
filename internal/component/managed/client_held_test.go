// VALIDATES: a holding device sends its hub one copy of the configuration it
// runs (config-held): exactly the bytes it hashed, only when the hub holds no
// copy of that version, under 1 MiB, within its own deadline, never again for
// a version whose upload failed, and never again on a connection whose hub
// does not take the verb. The hold ack says the device holds.
// PREVENTS: a hold the hub cannot show, an upload that stalls the
// notification loop past the hub's push bound, and an old hub flooded with a
// verb it refuses.

package managed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// heldAnswer is how the test hub answers a request with an error, or not at
// all: silent sends no answer; otherwise the answer is an error with code
// (empty for none) and message.
type heldAnswer struct {
	silent  bool
	code    string
	message string
}

// heldPipe is holdPipe for a hub whose answer may be a heldAnswer.
// Its cleanup MUST cancel the hub worker, close both pipe ends and join the
// worker before returning.
func heldPipe(t *testing.T, answer func(*rpc.Request) any) (*rpc.MuxConn, <-chan *rpc.Request) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	clientEnd, hubEnd := net.Pipe()
	seen := make(chan *rpc.Request, 16)
	done := make(chan struct{})
	rc := rpc.NewConn(hubEnd, hubEnd)
	go func() {
		defer close(done)
		for {
			req, err := rc.ReadRequest(ctx)
			if err != nil {
				return // The test closed the pipe.
			}
			select {
			case seen <- req:
			case <-ctx.Done():
				return
			}
			if sendHeldAnswer(ctx, rc, req, answer(req)) != nil {
				return
			}
		}
	}()
	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	t.Cleanup(func() {
		cancel()
		_ = clientEnd.Close()
		_ = hubEnd.Close()
		_ = mc.Close()
		select {
		case <-done:
		case <-time.After(holdWait):
			t.Error("the test's hub worker did not join after its pipe closed")
		}
	})
	return mc, seen
}

// sendHeldAnswer answers req with result, or with the error or silence a
// heldAnswer asks for.
func sendHeldAnswer(ctx context.Context, rc *rpc.Conn, req *rpc.Request, result any) error {
	refusal, refused := result.(heldAnswer)
	if !refused {
		return rc.SendResult(ctx, req.ID, result)
	}
	if refusal.silent {
		return nil
	}
	if refusal.code == "" {
		return rc.SendError(ctx, req.ID, refusal.message)
	}
	return rc.SendCodedError(ctx, req.ID, refusal.code, refusal.message)
}

// hubHolding is a hub that holds holdHubConfig for the device, names
// heldVersion as the copy it holds, and answers config-held with onHeld.
func hubHolding(heldVersion string, onHeld any) func(*rpc.Request) any {
	return func(req *rpc.Request) any {
		switch req.Method {
		case fleet.VerbConfigFetch:
			return fleet.ConfigFetchResponse{
				Version:     fleet.VersionHash([]byte(holdHubConfig)),
				Config:      base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
				HeldVersion: heldVersion,
			}
		case fleet.VerbConfigHeld:
			return onHeld
		}
		return struct{}{}
	}
}

// methodsOf answers the methods of the next count requests the hub received.
func methodsOf(t *testing.T, seen <-chan *rpc.Request, count int) []string {
	t.Helper()
	methods := make([]string, 0, count)
	for range count {
		methods = append(methods, nextRequest(t, seen).Method)
	}
	return methods
}

// holdOnce runs one fetch of cfg over mc, which holds.
func holdOnce(t *testing.T, mc *rpc.MuxConn, cfg *ClientConfig) {
	t.Helper()
	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
}

// TestHeldUploadHashesWhatItSends proves the copy is the configuration whose
// hash it names: one hold against a hub that holds no copy.
//
// VALIDATES: A-1 and AC-1. The config-held payload decodes to exactly the
// bytes the device runs, and its version is their hash.
// PREVENTS: a copy whose hash the hub recomputes to another value, so adopt
// would never end the hold.
//
// MUTATION: send active.Hash() from the first read with bytes from another
// read, or drop the base64 encoding, and this fails.
func TestHeldUploadHashesWhatItSends(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding("", struct{}{}))

	holdOnce(t, mc, &cfg)

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	upload := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigHeld, upload.Method)
	var held fleet.ConfigHeld
	require.NoError(t, json.Unmarshal(upload.Params, &held))
	sent, err := base64.StdEncoding.DecodeString(held.Config)
	require.NoError(t, err)
	assert.Equal(t, holdDeviceConfig, string(sent))
	assert.Equal(t, fleet.VersionHash(sent), held.Version)
}

// TestHoldUploadsWhenHubHoldsOther proves a hub copy of another version is
// replaced: the hub names a held version that is not the active one.
//
// VALIDATES: AC-3. The device sends a copy of the version it runs now.
// PREVENTS: a hub showing an older local edit after the device's operator
// edited again.
//
// MUTATION: skip the upload whenever HeldVersion is set and this fails.
func TestHoldUploadsWhenHubHoldsOther(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding(fleet.VersionHash([]byte("an older local edit\n")), struct{}{}))

	holdOnce(t, mc, &cfg)

	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, seen, 3))
}

// TestHoldSkipsUploadWhenHubHoldsActive proves a copy is sent once per hub
// record: the hub names the active version as the copy it holds.
//
// VALIDATES: AC-2 and E-9. The device sends no copy, holds and acks.
// PREVENTS: a 1 MiB upload at every hub write while a device holds.
//
// MUTATION: drop the HeldVersion check of uploadHeld and this fails.
func TestHoldSkipsUploadWhenHubHoldsActive(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding(fleet.VersionHash([]byte(holdDeviceConfig)), struct{}{}))

	holdOnce(t, mc, &cfg)

	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigAck}, methodsOf(t, seen, 2))
	assert.Empty(t, seen, "the device sent the hub a copy it already holds")
}

// TestHoldUploadsAtBound proves the bound admits a configuration of exactly
// fleet.HeldConfigMax bytes.
//
// VALIDATES: the last valid size of the boundary row.
// PREVENTS: an off-by-one bound refusing the largest configuration a hub keeps.
func TestHoldUploadsAtBound(t *testing.T) {
	store := holdDevice(t, strings.Repeat("x", fleet.HeldConfigMax), holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding("", struct{}{}))

	holdOnce(t, mc, &cfg)

	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, seen, 3))
}

// TestHoldUploadRefusedOverBound proves a configuration one byte over the
// bound is not sent and the ack says why.
//
// VALIDATES: AC-4 and E-5. No config-held; the ack holds, says no copy was
// sent and names the bound; the connection is kept (fetchAndProcess answers
// nil).
// PREVENTS: a frame that keeps the notification loop busy past the hub's push
// bound.
//
// MUTATION: drop the fleet.HeldConfigMax check of uploadHeld and this fails.
func TestHoldUploadRefusedOverBound(t *testing.T) {
	store := holdDevice(t, strings.Repeat("x", fleet.HeldConfigMax+1), holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding("", struct{}{}))

	holdOnce(t, mc, &cfg)

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	told := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigAck, told.Method, "the device sent a copy over the bound")
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(told.Params, &ack))
	assert.True(t, ack.Holding)
	assert.Contains(t, ack.Error, "no copy was sent")
	assert.Contains(t, ack.Error, "larger than the 1048576 bytes")
}

// TestHeldUploadDeadlineBelowPushBound proves the upload's deadline ends
// before the hub gives up on a config-changed sent during it.
//
// VALIDATES: the deadline row of the boundary table.
// PREVENTS: a hub closing the connection of every device whose upload is slow.
func TestHeldUploadDeadlineBelowPushBound(t *testing.T) {
	assert.Less(t, heldUploadTimeout, pluginserver.ManagedPushTimeout)
	assert.Less(t, heldUploadTimeout+configAckTimeout, pluginserver.ManagedPushTimeout,
		"the upload and the hold ack after it, both stalled, outlast the hub's push bound")
}

// TestUnansweredAckReturnsWithinBound proves a hub that takes a config-ack
// and never answers it does not hold the device: the ack, sent on a context
// with no deadline, ends at configAckTimeout.
//
// VALIDATES: the config-ack round-trip is bounded on the device.
// PREVENTS: a stalled hub keeping the notification loop in one ack for ever,
// so the device answers no config-changed and never reconnects.
//
// MUTATION: call CallRPC with ctx instead of the deadline context in
// SendConfigAck: the ack does not return within its bound.
func TestUnansweredAckReturnsWithinBound(t *testing.T) {
	mc, seen := heldPipe(t, func(*rpc.Request) any { return heldAnswer{silent: true} })
	returned := make(chan error, 1)
	started := time.Now()
	go func() { // Ends when the pipe closes, if the bound is missing.
		returned <- SendConfigAck(context.Background(), mc, fleet.ConfigAck{Version: "v", OK: true})
	}()

	require.Equal(t, fleet.VerbConfigAck, nextRequest(t, seen).Method)
	select {
	case err := <-returned:
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.GreaterOrEqual(t, time.Since(started), configAckTimeout, "the ack ended before its bound")
	case <-time.After(configAckTimeout + holdWait):
		t.Fatal("an ack the hub never answered did not return within its bound")
	}
}

// TestHeldUploadDeadline proves a hub that takes the copy and never answers
// does not hold the device: the upload ends at its own deadline, and the ack
// follows while the hub still waits on the device.
//
// VALIDATES: AC-5 and A-10. Under a fetch bounded by the hub's push bound
// (pluginserver.ManagedPushTimeout), the upload fails on its deadline, the
// ack still reaches the hub, fetchAndProcess answers nil, and the next hold
// sends no copy of the same version.
// PREVENTS: a stalled upload keeping the notification loop from answering a
// config-changed until the hub closes the connection.
//
// MUTATION: call CallRPC with ctx instead of the deadline context: the upload
// lasts until the push bound, and the ack, sent on the expired context, never
// reaches the hub.
func TestHeldUploadDeadline(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding("", heldAnswer{silent: true}))
	ctx, cancel := context.WithTimeout(t.Context(), pluginserver.ManagedPushTimeout)
	defer cancel()

	require.NoError(t, fetchAndProcess(ctx, mc, &cfg))

	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, seen, 3))
	assert.Equal(t, versionOf([]byte(holdDeviceConfig)), cfg.held.failed)
	assert.Contains(t, cfg.held.failure, context.DeadlineExceeded.Error(), "the upload did not end on its deadline")
	holdOnce(t, mc, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigAck}, methodsOf(t, seen, 2),
		"the device sent again a copy whose upload timed out")
}

// TestHeldUploadAnswersConfigChangedWithinPushBound proves a config-changed
// the hub sends while the device's upload stalls is answered before the hub
// gives up on it: the hub calls config-changed under its own push bound, as
// the managed server does, during an upload it never answers.
//
// VALIDATES: AC-5, the other half: the second config-changed call returns
// without error, so the hub keeps the connection.
// PREVENTS: a hub closing the connection of every device whose upload is slow.
//
// MUTATION: call CallRPC with ctx instead of the deadline context in
// uploadHeld: the loop answers nothing until the push bound, and the second
// call fails on its deadline.
func TestHeldUploadAnswersConfigChangedWithinPushBound(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	ctx, cancel := context.WithCancel(t.Context())
	clientEnd, hubEnd := net.Pipe()
	hub := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	uploading := make(chan struct{}, 1)
	hubDone := make(chan struct{})
	go func() { // The hub's worker: it answers every request but config-held.
		defer close(hubDone)
		for {
			select {
			case req, ok := <-hub.Requests():
				if !ok {
					return
				}
				switch req.Method {
				case fleet.VerbConfigFetch:
					_ = hub.SendResult(ctx, req.ID, hubHolding("", nil)(req))
				case fleet.VerbConfigHeld:
					uploading <- struct{}{}
				default:
					_ = hub.SendOK(ctx, req.ID)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	loopDone := make(chan struct{})
	go func() { // The device's notification loop, until the test ends.
		defer close(loopDone)
		_ = notificationLoop(ctx, mc, &cfg, nil)
	}()
	t.Cleanup(func() {
		cancel()
		_ = clientEnd.Close()
		_ = hubEnd.Close()
		_ = mc.Close()
		_ = hub.Close()
		for _, done := range []chan struct{}{hubDone, loopDone} {
			select {
			case <-done:
			case <-time.After(holdWait):
				t.Error("a test worker did not join after its pipe closed")
			}
		}
	})
	push := func() error {
		pushCtx, done := context.WithTimeout(ctx, pluginserver.ManagedPushTimeout)
		defer done()
		_, err := hub.CallRPC(pushCtx, fleet.VerbConfigChanged, fleet.ConfigChanged{Version: fleet.VersionHash([]byte(holdHubConfig))})
		return err
	}

	require.NoError(t, push())
	select {
	case <-uploading:
	case <-time.After(holdWait):
		t.Fatal("the device sent no copy after the first config-changed")
	}
	require.NoError(t, push(), "the device did not answer a config-changed sent during its upload within the hub's push bound")
}

// TestHeldUploadNotRetriedAfterFailure proves a refused copy is not sent
// again for that version in the run, and a new version is.
//
// VALIDATES: R-1 and AC-8 on the device. After the hub refuses version D with
// fleet.HeldRefusedCode, the next hold sends no copy; after a local edit the
// next hold sends a copy of the new version on the same connection.
// PREVENTS: the same refused megabyte sent at every hub write.
//
// MUTATION: drop the heldUploads.failed check and the second hold uploads.
func TestHeldUploadNotRetriedAfterFailure(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding("", heldAnswer{code: fleet.HeldRefusedCode, message: "the copy does not parse"}))

	holdOnce(t, mc, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, seen, 3))
	holdOnce(t, mc, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigAck}, methodsOf(t, seen, 2))

	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte("device { edited again; }\n"), 0))
	holdOnce(t, mc, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, seen, 3))
}

// TestHeldUploadOncePerConnectionOldHub proves a hub that does not take the
// verb gets one try per connection.
//
// VALIDATES: AC-6 and R-6. After an error answer with no HeldRefusedCode (a
// hub built before config-held answers it as an unknown method), a new
// version sends no copy on that connection; a new connection tries again.
// PREVENTS: an old hub answering an unknown method at every hub write.
//
// MUTATION: drop the heldUploads.refusedOn check and the second hold uploads.
func TestHeldUploadOncePerConnectionOldHub(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	oldHub := hubHolding("", heldAnswer{message: "unknown method: " + fleet.VerbConfigHeld})
	mc, seen := heldPipe(t, oldHub)

	holdOnce(t, mc, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, seen, 3))
	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte("device { edited again; }\n"), 0))
	holdOnce(t, mc, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigAck}, methodsOf(t, seen, 2))

	next, nextSeen := heldPipe(t, oldHub)
	holdOnce(t, next, &cfg)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigHeld, fleet.VerbConfigAck}, methodsOf(t, nextSeen, 3))
}

// TestHoldAckSaysHolding proves the hold ack tells the hub the device holds.
//
// VALIDATES: E-10. The ack carries ok:false, holding:true, the hold text and
// the clause that a copy was sent.
// PREVENTS: a hub that cannot tell a hold from a configuration the device
// could not validate.
//
// MUTATION: drop Holding from holdFetched's ack and this fails.
func TestHoldAckSaysHolding(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubHolding("", struct{}{}))

	holdOnce(t, mc, &cfg)

	methodsOf(t, seen, 2)
	told := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigAck, told.Method)
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(told.Params, &ack))
	assert.False(t, ack.OK)
	assert.True(t, ack.Holding)
	assert.True(t, strings.HasPrefix(ack.Error, holdRefusalPrefix))
	assert.Contains(t, ack.Error, "a copy was sent to the hub")
}

// hubReverting is a hub that holds holdHubConfig for the device, holds a copy
// of the version the device runs, and offers the device to discard discard.
func hubReverting(discard string) func(*rpc.Request) any {
	return func(req *rpc.Request) any {
		if req.Method == fleet.VerbConfigFetch {
			return fleet.ConfigFetchResponse{
				Version:        fleet.VersionHash([]byte(holdHubConfig)),
				Config:         base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
				HeldVersion:    fleet.VersionHash([]byte(holdDeviceConfig)),
				DiscardVersion: discard,
			}
		}
		return struct{}{}
	}
}

// ackOf reads the next request the hub received, which MUST be a config-ack,
// and answers it.
func ackOf(t *testing.T, seen <-chan *rpc.Request) fleet.ConfigAck {
	t.Helper()
	told := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigAck, told.Method)
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(told.Params, &ack))
	return ack
}

// TestDiscardAppliesWhenActiveMatches proves a holding device whose hub
// offers to discard exactly the version it runs applies the hub's
// configuration through OnRevert, the push path with the stage's check, and
// acks it ok.
//
// VALIDATES: spec-fleet-7 AC-13 (it applies V through the push path and acks
// ok:true), E-4 (the check lives in fetchAgainstBaseline).
// PREVENTS: a revert the operator decided that the device never takes.
// MUTATION: drop the applyReverted branch of fetchAgainstBaseline: the device
// holds, uploads nothing (the hub holds its copy) and applies nothing.
func TestDiscardAppliesWhenActiveMatches(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubReverting(fleet.VersionHash([]byte(holdDeviceConfig))))

	holdOnce(t, mc, &cfg)

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.True(t, ack.OK, "ack error: %s", ack.Error)
	assert.False(t, ack.Holding)
	select {
	case data := <-applied:
		assert.Equal(t, holdHubConfig, string(data))
	default:
		t.Fatal("the device committed nothing although its hub offered to discard the version it runs")
	}
}

// TestDiscardIgnoredWhenActiveDiffers proves a holding device whose hub
// offers to discard another version than the one it runs, a decision taken
// on an older local edit, holds and applies nothing.
//
// VALIDATES: spec-fleet-7 AC-15 (a newer local edit always wins over an
// older decision), R-2.
// PREVENTS: a stale revert discarding a change the hub's operator never saw.
// MUTATION: apply on any discard-version in fetchAgainstBaseline instead of
// one equal to the active hash.
func TestDiscardIgnoredWhenActiveDiffers(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := heldPipe(t, hubReverting(fleet.VersionHash([]byte("an older local edit\n"))))

	holdOnce(t, mc, &cfg)

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.False(t, ack.OK)
	assert.True(t, ack.Holding)
	assert.Empty(t, applied, "the device applied its hub's configuration on a decision taken on another version")
}

// TestDiscardForOtherReportedStale proves a holding device reports a
// discard-version that names another version as a stale offer.
//
// VALIDATES: spec-fleet-7 AC-21 (revert-offered reports a stale offer).
// PREVENTS: a device status that hides why a revert decided on the hub did
// not apply.
// MUTATION: report RevertForActive for every discard-version in hubAnswerOf.
func TestDiscardForOtherReportedStale(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	var answer HubAnswer
	cfg.OnHubAnswer = func(told HubAnswer) { answer = told }
	mc, _ := heldPipe(t, hubReverting(fleet.VersionHash([]byte("an older local edit\n"))))

	holdOnce(t, mc, &cfg)

	assert.Equal(t, RevertForOther, answer.Revert)
}

// TestDiscardRefusedApplyStillHolds proves a revert the device cannot apply,
// as when a confirmed-commit window is open, acks the refusal and leaves the
// device reported holding.
//
// VALIDATES: spec-fleet-7 AC-13 ("a device with a confirmed-commit window
// open refuses the push as today and the decision stays for its next
// fetch"), R-13.
// PREVENTS: a device status that says in sync while the device still runs
// its own configuration.
// MUTATION: report the hold false after applyFetched in applyReverted
// whatever the store says.
func TestDiscardRefusedApplyStillHolds(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	cfg.OnRevert = func([]byte, func([]byte) error) error { return errors.New("a confirmed commit is open") }
	var holding []bool
	cfg.OnHold = func(held bool) { holding = append(holding, held) }
	mc, seen := heldPipe(t, hubReverting(fleet.VersionHash([]byte(holdDeviceConfig))))

	holdOnce(t, mc, &cfg)

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "a confirmed commit is open")
	assert.Equal(t, []bool{true}, holding)
}

// TestHoldFixNamesWhatTheHubKeeps proves the hold warning names the hub
// commands only while the hub keeps a copy of the active version, and the
// manual ways out otherwise: each row runs uploadHeld against a hub and
// reads the fix holdFetched logs from its answer (heldFix).
//
// VALIDATES: review round 1 finding 1 (spec-fleet-7 AC-6: an old hub keeps
// the fleet-6 behavior, whose fix is the manual one).
// PREVENTS: a warning that names hub commands an old hub does not have, or a
// copy the hub never kept.
//
// MUTATION: log managedDivergedFix in holdFetched whatever uploadHeld answers,
// or answer kept true for every clause, and the manual rows fail.
func TestHoldFixNamesWhatTheHubKeeps(t *testing.T) {
	device := versionOf([]byte(holdDeviceConfig))
	commands := managedDivergedFix("edge-01", device.Hash())
	manual := managedDivergedManualFix("edge-01")
	tests := []struct {
		name   string
		held   string // the held-version of the fetch answer
		onHeld any    // the hub's answer to config-held
		active ConfigVersion
		fix    string
	}{
		{"old hub refuses the verb", "", heldAnswer{message: "unknown method: " + fleet.VerbConfigHeld}, device, manual},
		{"hub refuses the copy", "", heldAnswer{code: fleet.HeldRefusedCode, message: "the copy does not parse"}, device, manual},
		{"hub holds the active copy", device.Hash(), heldAnswer{silent: true}, device, commands},
		{"hub takes the copy", "", struct{}{}, device, commands},
		{"copy of a newer edit", "", struct{}{}, versionOf([]byte(holdEarlierConfig)), manual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
			access := NewBaselineAccess(store)
			cfg, _, _ := holdClient("192.0.2.1:1791", access)
			mc, _ := heldPipe(t, hubHolding(tt.held, tt.onHeld))

			_, kept := uploadHeld(context.Background(), mc, &cfg, access, tt.active, fleet.ConfigFetchResponse{HeldVersion: tt.held})

			assert.Equal(t, tt.fix, heldFix(cfg.Name, tt.active, kept))
		})
	}
}

// TestDiscardHoldsOverLocalCommitDuringFetch proves a revert decided on the
// configuration the device ran when it fetched never overwrites a local
// commit that landed during the fetch: the hub commits a local edit into the
// device's store before it answers with a discard-version naming the old one.
//
// VALIDATES: review note N1 (spec-fleet-7 AC-15, a newer local edit always
// wins): the stage's check refuses the local edit, nothing is committed, the
// local edit stays active, the device fetches again naming the local edit,
// sends a copy of it and acks holding.
// PREVENTS: a revert discarding a change the hub's operator never saw, and a
// hold decided on the first answer, whose fetched version is the old one, so
// the hub refuses to show the copy and records the device as rejected.
//
// MUTATION: commit the revert through OnCommit in applyReverted (no check),
// and the hub's configuration is committed over the local edit. Hold over the
// first answer in refetchOverLocalCommit instead of fetching again, and the
// second fetch is missing.
func TestDiscardHoldsOverLocalCommitDuringFetch(t *testing.T) {
	const localEdit = "device { edited during the fetch; }\n"
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	var holding []bool
	cfg.OnHold = func(held bool) { holding = append(holding, held) }
	reverting := hubReverting(fleet.VersionHash([]byte(holdDeviceConfig)))
	mc, seen := heldPipe(t, func(req *rpc.Request) any {
		if req.Method == fleet.VerbConfigFetch {
			// The local commit lands after the device read the hash it sent.
			_ = store.WriteFile(fleetStatusConfigName, []byte(localEdit), 0)
		}
		return reverting(req)
	})

	holdOnce(t, mc, &cfg)

	assert.Equal(t, fleet.VersionHash([]byte(holdDeviceConfig)), fetchedVersion(t, nextRequest(t, seen)))
	assert.Equal(t, fleet.VersionHash([]byte(localEdit)), fetchedVersion(t, nextRequest(t, seen)),
		"the device did not fetch again over its newer local edit")
	upload := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigHeld, upload.Method, "the device sent no copy of its newer local edit")
	var held fleet.ConfigHeld
	require.NoError(t, json.Unmarshal(upload.Params, &held))
	assert.Equal(t, fleet.VersionHash([]byte(localEdit)), held.Version)
	ack := ackOf(t, seen)
	assert.False(t, ack.OK)
	assert.True(t, ack.Holding)
	assert.Empty(t, applied, "the revert committed the hub's configuration over the local commit")
	active, err := ActiveVersion(store)
	require.NoError(t, err)
	assert.Equal(t, versionOf([]byte(localEdit)), active)
	assert.Equal(t, []bool{true}, holding)
}

// fetchedVersion answers the version a request the hub received names, which
// MUST be a config-fetch.
func fetchedVersion(t *testing.T, req *rpc.Request) string {
	t.Helper()
	require.Equal(t, fleet.VerbConfigFetch, req.Method)
	var fetch fleet.ConfigFetchRequest
	require.NoError(t, json.Unmarshal(req.Params, &fetch))
	return fetch.Version
}

// TestDiscardFetchesAgainOnlyOnce proves the fetch made again over a local
// commit is bounded: a hub whose operator decides a revert on every version
// it is sent, against a device whose store changes during every fetch, sees
// two fetches and nothing more, and the device holds and ends the fetch with
// errActiveChangedTwice.
//
// VALIDATES: review round 2 finding 1 (the second fetch of applyReverted
// carries refetchNone).
// PREVENTS: a device that fetches without end while its store keeps changing.
//
// MUTATION: pass refetchOnce on the second fetch in refetchOverLocalCommit,
// and the device fetches a third time.
func TestDiscardFetchesAgainOnlyOnce(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	var holding []bool
	cfg.OnHold = func(held bool) { holding = append(holding, held) }
	var edits int
	mc, seen := heldPipe(t, func(req *rpc.Request) any {
		if req.Method != fleet.VerbConfigFetch {
			return struct{}{}
		}
		var fetch fleet.ConfigFetchRequest
		if json.Unmarshal(req.Params, &fetch) != nil {
			return heldAnswer{message: "bad config-fetch payload"}
		}
		edits++
		// A local commit lands during every fetch, and the operator decided a
		// revert on the version the device named.
		_ = store.WriteFile(fleetStatusConfigName, fmt.Appendf(nil, "device { edit %d; }\n", edits), 0)
		return fleet.ConfigFetchResponse{
			Version:        fleet.VersionHash([]byte(holdHubConfig)),
			Config:         base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
			HeldVersion:    fetch.Version,
			DiscardVersion: fetch.Version,
		}
	})

	err := fetchAndProcess(context.Background(), mc, &cfg)

	require.ErrorIs(t, err, errActiveChangedTwice)
	assert.Equal(t, []string{fleet.VerbConfigFetch, fleet.VerbConfigFetch}, methodsOf(t, seen, 2))
	assert.Empty(t, seen, "the device sent its hub more than two fetches")
	assert.Empty(t, applied, "the revert committed the hub's configuration over a local commit")
	assert.Equal(t, []bool{true}, holding)
}
