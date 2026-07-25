package peer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
)

type fakePDBClient struct {
	results map[uint32]peeringdb.PrefixCounts
	err     error
}

func (f *fakePDBClient) LookupASN(_ context.Context, asn uint32) (peeringdb.PrefixCounts, error) {
	if f.err != nil {
		return peeringdb.PrefixCounts{}, f.err
	}
	if r, ok := f.results[asn]; ok {
		return r, nil
	}
	return peeringdb.PrefixCounts{}, errors.New("not found")
}

func TestPrefixUpdateStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := lookupPrefixCounts(ctx, &fakePDBClient{}, 65001, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestPrefixUpdateLookupUsesCallerContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	slow := &fakePDBClient{err: context.DeadlineExceeded}
	_, err := lookupPrefixCounts(ctx, slow, 65001, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestPrefixUpdateSuspiciousSkipped(t *testing.T) {
	counts := peeringdb.PrefixCounts{IPv4: 0, IPv6: 0}
	if !counts.Suspicious() {
		t.Fatal("zero counts should be suspicious")
	}
}

func TestPrefixUpdateMarginApplied(t *testing.T) {
	tests := []struct {
		count  uint32
		margin uint8
		want   uint32
	}{
		{1000, 10, 1100},
		{1000, 0, 1000},
		{1000, 100, 2000},
		{100, 20, 120},
	}
	for _, tt := range tests {
		got := peeringdb.ApplyMargin(tt.count, tt.margin)
		if got != tt.want {
			t.Errorf("ApplyMargin(%d, %d) = %d, want %d", tt.count, tt.margin, got, tt.want)
		}
	}
}

func TestPrefixUpdateValidatePeeringDBURL(t *testing.T) {
	for _, u := range []string{
		"https://www.peeringdb.com",
		"http://localhost:8080",
	} {
		if err := validatePeeringDBURL(u); err != nil {
			t.Errorf("valid URL %q rejected: %v", u, err)
		}
	}
	for _, u := range []string{
		"ftp://peeringdb.com",
		"://bad",
	} {
		if err := validatePeeringDBURL(u); err == nil {
			t.Errorf("invalid URL %q accepted", u)
		}
	}
}

func TestWaitForPeeringDBRateLimitRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForPeeringDBRateLimit(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
