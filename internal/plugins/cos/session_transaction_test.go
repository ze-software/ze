// Design: docs/architecture/traffic/cos-dynamic.md -- synchronous CoA ownership

//go:build ze_l2tp

package cos

import (
	"encoding/json"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	coreCos "github.com/ze-software/ze/internal/core/cos"
)

// TestCoSFailedChangeKeepsInstalledProfile proves a failed backend write cannot
// publish the requested profile as the session's installed profile.
func TestCoSFailedChangeKeepsInstalledProfile(t *testing.T) {
	bus, backend := setupTestHandler(t)
	key := sessionKey{tunnelID: 41, sessionID: 42}
	sessionStore.Store(key, sessionCoSState{accessInterface: "eth0.42", profileName: "residential"})
	t.Cleanup(func() { sessionStore.Delete(key) })
	backend.err = errors.New("backend refused map")
	emitCoSChange(t, bus, &l2tpevents.SessionCoSChangePayload{
		TunnelID: 41, SessionID: 42, AccessInterface: "eth0.42", ProfileName: "business",
	})
	value, loaded := sessionStore.Load(key)
	if !loaded {
		t.Fatal("failed apply lost installed session profile")
	}
	state, valid := value.(sessionCoSState)
	if !valid {
		t.Fatalf("session CoS state has type %T", value)
	}
	if state.profileName != "residential" {
		t.Fatalf("failed apply changed installed session profile: %#v", value)
	}
}

type transactionMaps struct {
	ingress    map[uint32]uint32
	egress     map[uint32]uint32
	writes     int
	failWrites int
}

func setupCoSTransaction(t *testing.T) (*cosHandler, *transactionMaps) {
	t.Helper()
	state := &transactionMaps{ingress: map[uint32]uint32{2: 4}, egress: map[uint32]uint32{4: 2}}
	h := newCosHandler(newTestBus(), func(_ string, ingress, egress map[uint32]uint32) error {
		state.writes++
		state.ingress, state.egress = maps.Clone(ingress), maps.Clone(egress)
		if state.failWrites > 0 {
			state.failWrites--
			return errors.New("map write failed after changing the backend")
		}
		return nil
	}, nil)
	h.readLink = func(string) (*iface.InterfaceInfo, error) {
		return &iface.InterfaceInfo{Name: "eth0.42", Type: "vlan", IngressQoSMap: state.ingress, EgressQoSMap: state.egress}, nil
	}
	previous := dynamicHandler
	dynamicHandler = h
	coreCos.Replace(map[string]coreCos.Profile{"business": {IngressMap: map[uint32]uint32{1: 6}, EgressMap: map[uint32]uint32{6: 1}}})
	t.Cleanup(func() {
		h.stop()
		dynamicHandler = previous
		coreCos.Replace(nil)
		sessionStore.Delete(sessionKey{tunnelID: 41, sessionID: 42})
	})
	return h, state
}

func cosTransaction(t *testing.T, operation, token string) (string, error) {
	t.Helper()
	request := sessionTransactionRequest{Operation: operation, Token: token}
	if operation == "prepare" {
		request.TunnelID, request.SessionID = 41, 42
		request.Interface, request.Profile = "eth0.42", "business"
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	status, data, err := executeCommand("test", sessionCommand, []string{"transaction", string(payload)}, "192.0.2.1")
	if err != nil {
		return "", err
	}
	if status != "done" {
		t.Fatalf("status %q: %#v", status, data)
	}
	result, valid := data.(sessionTransactionResult)
	if !valid {
		t.Fatalf("session transaction returned %T", data)
	}
	return result.Token, nil
}

// TestCoSTransactionRestoresActualMaps proves prepare has no effects and
// rollback restores backend state captured before a confirmed application.
func TestCoSTransactionRestoresActualMaps(t *testing.T) {
	_, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	if backend.writes != 0 {
		t.Fatal("prepare changed backend state")
	}
	if _, err := cosTransaction(t, "apply", token); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(backend.egress, map[uint32]uint32{6: 1}) {
		t.Fatalf("apply maps: %v", backend.egress)
	}
	if _, err := cosTransaction(t, "rollback", token); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(backend.egress, map[uint32]uint32{4: 2}) {
		t.Fatalf("restored maps: %v", backend.egress)
	}
	if _, loaded := sessionStore.Load(sessionKey{tunnelID: 41, sessionID: 42}); loaded {
		t.Fatal("rollback invented prior session CoS state")
	}
}

// TestCoSPartialWriteIsCompensated proves an error returned after mutation
// restores the previous actual maps before the owner reports failure.
func TestCoSPartialWriteIsCompensated(t *testing.T) {
	_, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	backend.failWrites = 1
	if _, err := cosTransaction(t, "apply", token); err == nil {
		t.Fatal("partial backend write was acknowledged")
	}
	if !maps.Equal(backend.ingress, map[uint32]uint32{2: 4}) {
		t.Fatalf("partial write left maps changed: %v", backend.ingress)
	}
	if _, loaded := sessionStore.Load(sessionKey{tunnelID: 41, sessionID: 42}); loaded {
		t.Fatal("failed write published session CoS state")
	}
}

// TestCoSCommittedTokenCanCompensate proves a later owner's failed commit can
// still compensate this owner's already committed part of the same request.
func TestCoSCommittedTokenCanCompensate(t *testing.T) {
	_, backend := setupCoSTransaction(t)
	key := sessionKey{tunnelID: 41, sessionID: 42}
	sessionStore.Store(key, sessionCoSState{accessInterface: "eth0.42", profileName: "previous", staticEgress: map[uint32]uint32{0: 0}})
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"apply", "apply", "commit", "commit", "rollback", "rollback"} {
		if _, err := cosTransaction(t, operation, token); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
	}
	if !maps.Equal(backend.egress, map[uint32]uint32{4: 2}) {
		t.Fatalf("committed rollback left changed maps: %v", backend.egress)
	}
	value, loaded := sessionStore.Load(key)
	if !loaded {
		t.Fatal("rollback lost prior CoS state")
	}
	state, valid := value.(sessionCoSState)
	if !valid {
		t.Fatalf("session CoS state has type %T", value)
	}
	if state.profileName != "previous" {
		t.Fatalf("rollback state: %#v", value)
	}
}

// TestCoSOwnerNotificationDoesNotWriteAgain proves post-commit notifications
// preserve the successful backend outcome without a second programming call.
func TestCoSOwnerNotificationDoesNotWriteAgain(t *testing.T) {
	h, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"apply", "commit"} {
		if _, err := cosTransaction(t, operation, token); err != nil {
			t.Fatal(err)
		}
	}
	h.onCoSChange(&l2tpevents.SessionCoSChangePayload{TunnelID: 41, SessionID: 42, AccessInterface: "eth0.42", ProfileName: "business"})
	if backend.writes != 1 {
		t.Fatalf("notification caused %d writes, want one total", backend.writes)
	}
}

// TestCoSOwnerReceiptDoesNotHideProfileReload proves a receipt skips only its
// confirmed maps. A later legacy event must apply a reloaded profile, whether
// the owner's first notification was delivered or lost.
func TestCoSOwnerReceiptDoesNotHideProfileReload(t *testing.T) {
	for _, firstDelivered := range []bool{true, false} {
		name := "notification lost"
		if firstDelivered {
			name = "notification delivered"
		}
		t.Run(name, func(t *testing.T) {
			h, backend := setupCoSTransaction(t)
			token, err := cosTransaction(t, "prepare", "")
			if err != nil {
				t.Fatal(err)
			}
			for _, operation := range []string{"apply", "commit"} {
				if _, err := cosTransaction(t, operation, token); err != nil {
					t.Fatal(err)
				}
			}
			notification := &l2tpevents.SessionCoSChangePayload{TunnelID: 41, SessionID: 42,
				AccessInterface: "eth0.42", ProfileName: "business"}
			if firstDelivered {
				h.onCoSChange(notification)
			}
			if backend.writes != 1 {
				t.Fatalf("first notification caused %d writes, want one total", backend.writes)
			}
			updated := coreCos.Profile{IngressMap: map[uint32]uint32{1: 7}, EgressMap: map[uint32]uint32{7: 1}}
			if err := installCoSProfiles(cosProfiles{"business": updated}); err != nil {
				t.Fatal(err)
			}
			h.onCoSChange(notification)
			if !maps.Equal(backend.ingress, updated.IngressMap) {
				t.Fatalf("reloaded ingress maps were suppressed: got %v, want %v", backend.ingress, updated.IngressMap)
			}
			if !maps.Equal(backend.egress, updated.EgressMap) {
				t.Fatalf("reloaded egress maps were suppressed: got %v, want %v", backend.egress, updated.EgressMap)
			}
			if backend.writes != 2 {
				t.Fatalf("reloaded profile caused %d writes, want two total", backend.writes)
			}
		})
	}
}

// TestCoSTransactionRefusesForeignPeer proves tokens cannot be used from
// another RADIUS client's context, even when that caller knows the token.
func TestCoSTransactionRefusesForeignPeer(t *testing.T) {
	_, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(sessionTransactionRequest{Operation: "apply", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = executeCommand("test", sessionCommand, []string{"transaction", string(payload)}, "192.0.2.2")
	if err == nil {
		t.Fatal("another RADIUS client used the token")
	}
	if backend.writes != 0 {
		t.Fatal("refused foreign peer changed maps")
	}
}

// TestCoSTransactionRejectsInvalidArguments proves invalid command payloads
// cannot prepare or write an access VLAN.
func TestCoSTransactionRejectsInvalidArguments(t *testing.T) {
	h, backend := setupCoSTransaction(t)
	cases := map[string][]string{
		"no arguments":       nil,
		"missing keyword":    {`{"operation":"prepare"}`},
		"wrong keyword":      {"payload", `{}`},
		"unknown field":      {"transaction", `{"operation":"prepare","typo":1}`},
		"trailing object":    {"transaction", `{} {}`},
		"unknown operation":  {"transaction", `{"operation":"invented"}`},
		"followup selection": {"transaction", `{"operation":"apply","token":"missing","session-id":0}`},
		"prepare with token": {"transaction", `{"operation":"prepare","token":"wrong","session-id":42,"interface":"eth0.42","profile":"business"}`},
		"missing interface":  {"transaction", `{"operation":"prepare","session-id":42,"profile":"business"}`},
		"missing session":    {"transaction", `{"operation":"prepare","interface":"eth0.42","profile":"business"}`},
		"missing profile":    {"transaction", `{"operation":"prepare","session-id":42,"interface":"eth0.42","profile":"unknown"}`},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := executeCommand("test", sessionCommand, args, "192.0.2.1"); err == nil {
				t.Fatal("invalid transaction was accepted")
			}
			if h.pending != nil {
				t.Fatal("invalid transaction acquired owner state")
			}
			if backend.writes != 0 {
				t.Fatal("invalid transaction changed backend maps")
			}
		})
	}
}

// TestCoSTransactionKeepsFailedRollbackPending proves a second prepare cannot
// erase the snapshot still needed after backend compensation fails.
func TestCoSTransactionKeepsFailedRollbackPending(t *testing.T) {
	_, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cosTransaction(t, "apply", token); err != nil {
		t.Fatal(err)
	}
	backend.failWrites = 1
	if _, err := cosTransaction(t, "rollback", token); err == nil {
		t.Fatal("failed rollback was acknowledged")
	}
	if _, err := cosTransaction(t, "prepare", ""); err == nil {
		t.Fatal("new prepare erased failed rollback snapshot")
	}
	if _, err := cosTransaction(t, "rollback", token); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(backend.egress, map[uint32]uint32{4: 2}) {
		t.Fatalf("retry did not restore original maps: %v", backend.egress)
	}
}

// TestCoSExpiredTransactionRestoresMaps proves bounded retention can recover a
// transaction whose coordinator disappeared after its apply.
func TestCoSExpiredTransactionRestoresMaps(t *testing.T) {
	h, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cosTransaction(t, "apply", token); err != nil {
		t.Fatal(err)
	}
	h.pending.prepared().expires = time.Now().Add(-time.Second)
	if _, err := cosTransaction(t, "prepare", ""); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(backend.egress, map[uint32]uint32{4: 2}) {
		t.Fatalf("expired transaction left changed maps: %v", backend.egress)
	}
}

// TestCoSSessionDownInvalidatesTransaction proves delayed commands cannot
// recreate a subscriber's dynamic CoS after its departure event.
func TestCoSSessionDownInvalidatesTransaction(t *testing.T) {
	h, backend := setupCoSTransaction(t)
	token, err := cosTransaction(t, "prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cosTransaction(t, "apply", token); err != nil {
		t.Fatal(err)
	}
	h.onSessionDown(&l2tpevents.SessionDownPayload{TunnelID: 41, SessionID: 42})
	if _, err := cosTransaction(t, "apply", token); err == nil {
		t.Fatal("departed session's token remained usable")
	}
	if !maps.Equal(backend.egress, map[uint32]uint32{4: 2}) {
		t.Fatalf("session-down did not restore static maps: %v", backend.egress)
	}
}

// TestCoSSessionCommandRegistered proves the acknowledged owner route exists.
func TestCoSSessionCommandRegistered(t *testing.T) {
	for _, decl := range commandDecls() {
		if decl.Name == "request class-of-service session" {
			return
		}
	}
	t.Fatal("missing synchronous class-of-service session command")
}
