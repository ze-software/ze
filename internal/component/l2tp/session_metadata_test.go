package l2tp

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

func TestStoreAndLoadSessionMetadata(t *testing.T) {
	meta := &AuthMetadata{
		FramedIP:            netip.MustParseAddr("10.0.0.5"),
		FramedNetmask:       net.CIDRMask(32, 32),
		FramedPool:          "gold",
		SessionTimeout:      600,
		IdleTimeout:         120,
		FilterID:            "rate:20M/5M",
		AcctInterimInterval: 60,
	}
	StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 2), meta)
	defer ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 2))

	got := LoadSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 2))
	if got == nil {
		t.Fatal("expected metadata, got nil")
	}
	if got.FramedIP != meta.FramedIP {
		t.Errorf("FramedIP = %v, want %v", got.FramedIP, meta.FramedIP)
	}
	if got.FramedPool != "gold" {
		t.Errorf("FramedPool = %q, want %q", got.FramedPool, "gold")
	}
	if got.SessionTimeout != 600 {
		t.Errorf("SessionTimeout = %d, want 600", got.SessionTimeout)
	}
}

func TestLoadSessionMetadataMultipleConsumers(t *testing.T) {
	StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(3), 4), &AuthMetadata{FramedPool: "silver"})
	defer ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(3), 4))

	got := LoadSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(3), 4))
	if got == nil {
		t.Fatal("first load should return metadata")
	}

	got2 := LoadSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(3), 4))
	if got2 == nil {
		t.Fatal("second load should also return metadata (multiple consumers)")
	}
	if got2.FramedPool != "silver" {
		t.Errorf("FramedPool = %q, want %q", got2.FramedPool, "silver")
	}
}

func TestLoadSessionMetadataNotStored(t *testing.T) {
	got := LoadSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(99), 99))
	if got != nil {
		t.Error("expected nil for unstored session")
	}
}

func TestStoreNilMetadata(t *testing.T) {
	StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(5), 6), nil)
	got := LoadSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(5), 6))
	if got != nil {
		t.Error("expected nil after storing nil metadata")
	}
}

func TestClearSessionMetadata(t *testing.T) {
	StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(7), 8), &AuthMetadata{SessionTimeout: 300})
	ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(7), 8))
	got := LoadSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(7), 8))
	if got != nil {
		t.Error("expected nil after clear")
	}
}
