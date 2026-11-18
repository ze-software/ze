// Design: docs/architecture/traffic/cos-dynamic.md -- command failure reports
// Related: shared_access_cleanup_registered_test.go -- registered SDK fixture

//go:build ze_l2tp

package cos

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// No new production API is needed. Capture the same bridge the real registered
// RunEngine receives; its ExecuteCommand drives the SDK callback with a peer.
// Each test owns this runner and its publisher and uses no parallel subtests.
type cosCommandReportFixture struct {
	*sharedCoSTestOwner
	bridge   *rpc.DirectBridge
	bus      ze.EventBus
	failures []error
}

func newCoSCommandReportFixture(t *testing.T) *cosCommandReportFixture {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	f := &cosCommandReportFixture{sharedCoSTestOwner: newSharedCoSTestOwner(t)}
	write := f.handler.updateQoS
	read := f.handler.readLink
	prior := dynamicHandler
	t.Cleanup(func() { dynamicHandler = prior })
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.bus = bus
	registered := registry.Lookup(Name)
	if registered == nil {
		t.Fatal("the real CoS registration is absent")
	}
	registered.ConfigureEventBus(bus)
	f.handler = dynamicHandler
	t.Cleanup(f.handler.stop)
	f.handler.updateQoS = func(name string, ingress, egress map[uint32]uint32) error {
		if err := write(name, ingress, egress); err != nil {
			return err
		}
		if len(f.failures) == 0 {
			return nil
		}
		err := f.failures[0]
		f.failures = f.failures[1:]
		return err
	}
	f.handler.readLink = read
	pluginengine.Start(t, pluginengine.TransportBridge, func(conn net.Conn) int {
		bridged, ok := conn.(*rpc.BridgedConn)
		if !ok {
			return 1
		}
		f.bridge = bridged.Bridge()
		return registered.RunEngine(conn)
	})
	if f.bridge == nil {
		t.Fatal("the real SDK runner did not receive its bridge")
	}
	return f
}

func (f *cosCommandReportFixture) command(operation string, session uint16, profile, token string) (string, error) {
	return f.commandFromPeer(operation, session, profile, token, "192.0.2.31")
}

func (f *cosCommandReportFixture) commandFromPeer(operation string, session uint16, profile, token, peer string) (string, error) {
	f.t.Helper()
	request := sessionTransactionRequest{Operation: operation, Token: token}
	if operation == "prepare" {
		request.TunnelID = sharedCoSTestTunnel
		request.SessionID = session
		request.Interface = sharedCoSTestInterface
		request.Profile = profile
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		f.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.t.Context(), 5*time.Second)
	defer cancel()
	output, err := f.bridge.ExecuteCommand(ctx, "cos-report-test", sessionCommand, []string{"transaction", string(encoded)}, peer)
	if err != nil {
		return "", err
	}
	if output.Status != rpc.StatusDone {
		return "", errors.New(string(output.Data))
	}
	var result sessionTransactionResult
	if err := json.Unmarshal(output.Data, &result); err != nil {
		f.t.Fatal(err)
	}
	return result.Token, nil
}

func (f *cosCommandReportFixture) cleanup() error {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.t.Context(), 5*time.Second)
	defer cancel()
	out, err := f.bridge.ExecuteCommand(ctx, "cos-cleanup-test", cleanupCommandForReview, nil, "")
	if err != nil {
		return err
	}
	if out.Status != rpc.StatusDone {
		return errors.New(string(out.Data))
	}
	return nil
}

func requireCoSCommandWarning(t *testing.T, subject string, causes ...string) string {
	t.Helper()
	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("command left %d active warnings, want one: %+v", len(warnings), warnings)
	}
	entry := warnings[0]
	if entry.Code != "doctor-cos-cleanup-unconfirmed" {
		t.Fatalf("wrong cleanup code: %+v", entry)
	}
	if entry.Fix != "ze request class-of-service cleanup" {
		t.Fatalf("warning lost the reachable cleanup command: %+v", entry)
	}
	if entry.Subject == "" {
		t.Fatal("command warning lost its original owner")
	}
	if subject != "" {
		if entry.Subject != subject {
			t.Fatal("retry replaced the original owner warning")
		}
	}
	for _, cause := range causes {
		if !strings.Contains(entry.Message, cause) {
			t.Fatalf("warning omitted actual cause %q: %q", cause, entry.Message)
		}
	}
	return entry.Subject
}

// Goal: an apply with failed immediate compensation publishes one warning and
// the advertised cleanup route restores only that pending failed transaction.
// Method: drive the actual registered SDK command, then its cleanup command.
func TestCoSLegacySharedFailedApplyCompensationHasCommandReport(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63230)
	token, err := f.command("prepare", 63230, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	f.failures = []error{errors.New("command profile write failed"), errors.New("command immediate compensation failed")}
	if _, err := f.command("apply", 0, "", token); err == nil {
		t.Fatal("failed native apply was acknowledged")
	}
	requireCoSCommandWarning(t, "", "command profile write failed", "command immediate compensation failed")
	writes := f.writes
	if err := f.cleanup(); err != nil {
		t.Fatalf("advertised cleanup could not recover failed command: %v", err)
	}
	if f.writes != writes+1 {
		t.Fatal("command cleanup did not compensate once")
	}
	f.requireMaps(f.shared)
	if len(report.Warnings()) != 0 {
		t.Fatal("confirmed command recovery kept its warning")
	}
	if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 63230}); !retained {
		t.Fatal("command cleanup removed its healthy prior subscriber")
	}
	registeredCoSCleanupReviewDown(t, f.bus, 63230)
	f.requireMaps(f.original)
}

// Goal: explicit rollback failures refresh the same owner warning until actual
// original compensation succeeds, then the same token retries as a native noop.
func TestCoSLegacySharedRollbackFailureRetainsCommandReport(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63231)
	token, err := f.command("prepare", 63231, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.failures = []error{errors.New("first command rollback failed")}
	if _, err := f.command("rollback", 0, "", token); err == nil {
		t.Fatal("failed rollback was acknowledged")
	}
	subject := requireCoSCommandWarning(t, "", "first command rollback failed")
	f.failures = []error{errors.New("second command rollback failed")}
	if _, err := f.command("rollback", 0, "", token); err == nil {
		t.Fatal("failed rollback retry was acknowledged")
	}
	requireCoSCommandWarning(t, subject, "first command rollback failed", "second command rollback failed")
	writes := f.writes
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes+1 {
		t.Fatal("confirmed rollback did not compensate once")
	}
	f.requireMaps(f.shared)
	if len(report.Warnings()) != 0 {
		t.Fatal("confirmed rollback kept its original warning")
	}
	writes = f.writes
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("completed rollback replayed native compensation")
	}
	registeredCoSCleanupReviewDown(t, f.bus, 63231)
	f.requireMaps(f.original)
}

// Goal: healthy transactions and policy refusal do not claim native uncertainty.
// Method: keep a healthy pending apply through cleanup, roll it back normally,
// then refuse an unknown policy before any native write.
func TestCoSLegacySharedHealthyCommandsHaveNoCleanupReport(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63232)
	token, err := f.command("prepare", 63232, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	writes := f.writes
	if _, err := f.commandFromPeer("apply", 0, "", token, "192.0.2.32"); err == nil {
		t.Fatal("foreign fixture peer used the original token")
	}
	if f.writes != writes {
		t.Fatal("foreign-peer refusal wrote native maps")
	}
	if len(report.Warnings()) != 0 {
		t.Fatal("foreign-peer refusal manufactured native cleanup")
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	writes = f.writes
	if err := f.cleanup(); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("cleanup rewrote a healthy pending transaction")
	}
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.requireMaps(f.shared)
	writes = f.writes
	if _, err := f.command("prepare", 63232, "unknown-command-profile", ""); err == nil {
		t.Fatal("unknown profile was accepted")
	}
	if f.writes != writes {
		t.Fatal("policy refusal wrote native state")
	}
	if len(report.Warnings()) != 0 {
		t.Fatal("healthy command or policy refusal manufactured cleanup work")
	}
	registeredCoSCleanupReviewDown(t, f.bus, 63232)
	f.requireMaps(f.original)
}
