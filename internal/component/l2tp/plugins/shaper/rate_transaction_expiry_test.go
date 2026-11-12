package l2tpshaper

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/traffic"
)

type rateExpiryBackend struct {
	*mockBackend
	failRestore atomic.Bool
}

func (b *rateExpiryBackend) Apply(ctx context.Context, desired map[string]traffic.InterfaceQoS) error {
	if b.failRestore.Load() {
		if desired["ppp-expiry"].Qdisc.RateBps == 10_000_000 {
			return errors.New("test backend refused previous shaping restoration")
		}
	}
	return b.mockBackend.Apply(ctx, desired)
}

var rateExpiryRegistration sync.Once
var rateExpiryCurrent atomic.Pointer[rateExpiryBackend]

func newRateExpiryOwner(t *testing.T) (*shaperPlugin, *rateExpiryBackend, func()) {
	t.Helper()
	backend := &rateExpiryBackend{mockBackend: setupMockBackend(t)}
	rateExpiryCurrent.Store(backend)
	rateExpiryRegistration.Do(func() {
		if err := traffic.RegisterBackend("test-rate-expiry", func() (traffic.Backend, error) {
			return rateExpiryCurrent.Load(), nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	if err := traffic.LoadBackend("test-rate-expiry"); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	owner := &shaperPlugin{rateNow: func() time.Time { return now }}
	owner.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 10_000_000, UploadRate: 5_000_000})
	owner.onSessionUp(&l2tpevents.SessionUpPayload{TunnelID: 1, SessionID: 2, Interface: "ppp-expiry"})
	return owner, backend, func() { now = now.Add(2 * rateTransactionLifetime) }
}

func rateExpiryCommand(t *testing.T, owner *shaperPlugin, operation, token string, download, upload uint64) (string, error) {
	t.Helper()
	request := rateRequest{Operation: operation, Token: token}
	if operation == "prepare" {
		request.TunnelID, request.SessionID, request.Interface = 1, 2, "ppp-expiry"
		request.DownloadRate, request.UploadRate = download, upload
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := owner.rateOperation([]string{"transaction", string(encoded)}, "192.0.2.1")
	if err != nil {
		return "", err
	}
	values, valid := result.(map[string]string)
	if !valid {
		t.Fatalf("transaction answer type: %T", result)
	}
	return values["token"], nil
}

func wantRateExpiryState(t *testing.T, backend *rateExpiryBackend, download, upload uint64) {
	t.Helper()
	qos, found := backend.getApplied("ppp-expiry")
	if !found {
		t.Fatal("the fixture has no installed shaping")
	}
	if qos.Qdisc.RateBps != download || qos.Ingress.RateBps != upload {
		t.Errorf("installed shaping: got %d/%d, want %d/%d", qos.Qdisc.RateBps, qos.Ingress.RateBps, download, upload)
	}
}

// TestRateLostPrepareReplyExpires models a successful prepare whose reply the
// coordinator lost. A private clock advances beyond its bound without sleeping,
// and a different request must obtain the slot and actually apply its rate pair.
func TestRateLostPrepareReplyExpires(t *testing.T) {
	owner, backend, expire := newRateExpiryOwner(t)
	if _, err := rateExpiryCommand(t, owner, "prepare", "", 20_000_000, 8_000_000); err != nil {
		t.Fatal(err)
	}
	expire()
	token, err := rateExpiryCommand(t, owner, "prepare", "", 30_000_000, 12_000_000)
	if err != nil {
		t.Fatalf("lost prepare reply permanently occupied the owner: %v", err)
	}
	wantRateExpiryState(t, backend, 10_000_000, 5_000_000)
	if _, err := rateExpiryCommand(t, owner, "apply", token, 0, 0); err != nil {
		t.Fatal(err)
	}
	wantRateExpiryState(t, backend, 30_000_000, 12_000_000)
}

// TestRateExpiredAppliedTransactionRestoresBeforePrepare loses the finalization
// of an applied change. The next request must restore the full original pair
// before preparing, so the new transaction's rollback target is trustworthy.
func TestRateExpiredAppliedTransactionRestoresBeforePrepare(t *testing.T) {
	owner, backend, expire := newRateExpiryOwner(t)
	token, err := rateExpiryCommand(t, owner, "prepare", "", 20_000_000, 8_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rateExpiryCommand(t, owner, "apply", token, 0, 0); err != nil {
		t.Fatal(err)
	}
	wantRateExpiryState(t, backend, 20_000_000, 8_000_000)
	expire()
	next, err := rateExpiryCommand(t, owner, "prepare", "", 30_000_000, 12_000_000)
	if err != nil {
		t.Fatalf("expired applied transaction blocked recovery: %v", err)
	}
	wantRateExpiryState(t, backend, 10_000_000, 5_000_000)
	if _, err := rateExpiryCommand(t, owner, "apply", next, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := rateExpiryCommand(t, owner, "rollback", next, 0, 0); err != nil {
		t.Fatal(err)
	}
	wantRateExpiryState(t, backend, 10_000_000, 5_000_000)
}

// TestRateExpiredRestoreFailureKeepsRetryState refuses compensation once. The
// old token and snapshot must remain pending until a retry restores them; no
// later request may hide that failure by dropping the occupied slot.
func TestRateExpiredRestoreFailureKeepsRetryState(t *testing.T) {
	owner, backend, expire := newRateExpiryOwner(t)
	token, err := rateExpiryCommand(t, owner, "prepare", "", 20_000_000, 8_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rateExpiryCommand(t, owner, "apply", token, 0, 0); err != nil {
		t.Fatal(err)
	}
	expire()
	backend.failRestore.Store(true)
	if _, err := rateExpiryCommand(t, owner, "prepare", "", 30_000_000, 12_000_000); err == nil {
		t.Fatal("a failed expired restoration was reported as recovered")
	}
	if owner.ratePending == nil {
		t.Fatal("failed restoration discarded its retry snapshot")
	}
	if owner.ratePending.transaction().token != token {
		t.Fatal("failed restoration replaced the original pending token")
	}
	wantRateExpiryState(t, backend, 20_000_000, 8_000_000)
	backend.failRestore.Store(false)
	if _, err := rateExpiryCommand(t, owner, "prepare", "", 30_000_000, 12_000_000); err != nil {
		t.Fatalf("repair could not retry retained restoration: %v", err)
	}
	wantRateExpiryState(t, backend, 10_000_000, 5_000_000)
}
