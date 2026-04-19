package mcp

import (
	"errors"
	"testing"
	"time"
)

func TestSessionRegistryCreateGet(t *testing.T) {
	r := newSessionRegistry(5*time.Minute, 0, 0)
	defer r.Close()

	s, err := r.Create("2025-06-18")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID() == "" {
		t.Fatal("session ID is empty")
	}
	if s.ProtocolVersion() != "2025-06-18" {
		t.Fatalf("protocolVersion = %q, want 2025-06-18", s.ProtocolVersion())
	}
	got, ok := r.Get(s.ID())
	if !ok {
		t.Fatal("Get: session not found")
	}
	if got.ID() != s.ID() {
		t.Fatalf("Get returned different session: %q vs %q", got.ID(), s.ID())
	}
}

func TestSessionRegistryDelete(t *testing.T) {
	r := newSessionRegistry(5*time.Minute, 0, 0)
	defer r.Close()

	s, err := r.Create("v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !r.Delete(s.ID()) {
		t.Fatal("Delete returned false on live session")
	}
	if _, ok := r.Get(s.ID()); ok {
		t.Fatal("Get returned ok after Delete")
	}
	if r.Delete(s.ID()) {
		t.Fatal("Delete returned true twice")
	}
}

func TestSessionRegistryGetRefreshesLastSeen(t *testing.T) {
	r := newSessionRegistry(5*time.Minute, 0, 0)
	defer r.Close()

	fakeNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return fakeNow }

	s, err := r.Create("v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	originalSeen := s.lastSeenAt

	fakeNow = fakeNow.Add(30 * time.Second)
	_, ok := r.Get(s.ID())
	if !ok {
		t.Fatal("Get returned false for live session")
	}
	if !s.lastSeenAt.After(originalSeen) {
		t.Fatalf("lastSeenAt not refreshed: was %v, now %v", originalSeen, s.lastSeenAt)
	}
}

func TestSessionRegistrySweepExpires(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	defer r.Close()

	fakeNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return fakeNow }

	s1, _ := r.Create("v1")
	s2, _ := r.Create("v1")

	fakeNow = fakeNow.Add(30 * time.Second)
	r.sweep()
	if r.Len() != 2 {
		t.Fatalf("sweep before expiry: Len = %d, want 2", r.Len())
	}

	fakeNow = fakeNow.Add(2 * time.Minute)
	r.sweep()
	if r.Len() != 0 {
		t.Fatalf("sweep after expiry: Len = %d, want 0", r.Len())
	}
	if !s1.closed.Load() || !s2.closed.Load() {
		t.Fatal("expired sessions not closed")
	}
}

func TestSessionRegistryCloseIsIdempotent(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	r.Close()
	r.Close() // must not panic or block
}

func TestSessionRegistryCreateAfterCloseFails(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	r.Close()
	if _, err := r.Create("v1"); err == nil {
		t.Fatal("Create after Close returned nil error")
	}
}

func TestSessionSendAndDrain(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	defer r.Close()

	s, err := r.Create("v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for i := range 3 {
		frame := []byte{byte('a' + i)}
		if err := s.Send(frame); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	for i := range 3 {
		frame := <-s.Outbound()
		if len(frame) != 1 || frame[0] != byte('a'+i) {
			t.Fatalf("frame %d = %v, want %q", i, frame, string(byte('a'+i)))
		}
	}
}

func TestSessionSendAfterCloseFails(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	defer r.Close()

	s, err := r.Create("v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.Close()

	if err := s.Send([]byte("x")); err == nil {
		t.Fatal("Send after Close returned nil error")
	}
}

func TestSessionSendQueueFull(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	r.queueSize = 2
	defer r.Close()

	s, err := r.Create("v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Override queue manually since Create uses the registry value at time of
	// call. Cleaner: recreate with a small queue.
	s.outbound = make(chan []byte, 2)

	for range 2 {
		if err := s.Send([]byte("x")); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	if err := s.Send([]byte("x")); err == nil {
		t.Fatal("Send to full queue returned nil error")
	}
}

func TestValidSessionID(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", false},
		{"ascii printable", "abc123XYZ_-", true},
		{"space invalid", "abc def", false},
		{"tab invalid", "abc\tdef", false},
		{"newline invalid", "abc\ndef", false},
		{"delete invalid", "abc\x7Fdef", false},
		{"non-ascii invalid", "café", false},
		{"tilde valid (0x7E)", "~abc~", true},
		{"bang valid (0x21)", "!abc!", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validSessionID(tc.input); got != tc.want {
				t.Fatalf("validSessionID(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestGenerateSessionIDUnique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for range 100 {
		id, err := generateSessionID()
		if err != nil {
			t.Fatalf("generateSessionID: %v", err)
		}
		if len(id) != sessionIDEncodedLen {
			t.Fatalf("len(id) = %d, want %d", len(id), sessionIDEncodedLen)
		}
		if !validSessionID(id) {
			t.Fatalf("generated id %q is not valid", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestSessionRegistryEnforcesMaxSessions(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 2)
	defer r.Close()

	if _, err := r.Create("v1"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := r.Create("v1"); err != nil {
		t.Fatalf("second Create: %v", err)
	}
	_, err := r.Create("v1")
	if !errors.Is(err, errSessionLimitReached) {
		t.Fatalf("third Create: err=%v, want errSessionLimitReached", err)
	}
}

func TestSessionRegistryMaxLifetimeEvictsEvenWhenTouched(t *testing.T) {
	// Regression for pass-4 finding 1 (session-hold DoS): an absolute
	// lifetime cap must evict a session whose lastSeenAt is fresh (kept so
	// by an active GET SSE heartbeat in production).
	r := newSessionRegistry(time.Hour, 5*time.Second, 0)
	defer r.Close()

	fakeNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return fakeNow }

	s, err := r.Create("v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Advance past maxLifetime while keeping lastSeenAt fresh on every tick.
	for range 3 {
		fakeNow = fakeNow.Add(3 * time.Second)
		s.Touch(fakeNow)
	}
	if r.Len() != 1 {
		t.Fatalf("pre-sweep Len = %d, want 1", r.Len())
	}
	// Lifetime exceeded (9 s > 5 s); sweep must evict despite fresh touch.
	r.sweep()
	if r.Len() != 0 {
		t.Fatalf("post-sweep Len = %d, want 0 (maxLifetime exceeded)", r.Len())
	}
	if !s.closed.Load() {
		t.Fatal("evicted session not closed")
	}
}

func TestSessionRegistryNegativeMaxSessionsDisablesCap(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, -1)
	defer r.Close()

	for i := range 10 {
		if _, err := r.Create("v1"); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}
	if r.Len() != 10 {
		t.Fatalf("Len = %d, want 10", r.Len())
	}
}

func TestSessionRegistryTTLClamping(t *testing.T) {
	cases := []struct {
		name    string
		request time.Duration
		want    time.Duration
	}{
		{"zero -> default", 0, defaultSessionTTL},
		{"below minimum -> clamped to min", 30 * time.Second, minSessionTTL},
		{"above maximum -> clamped to max", 48 * time.Hour, maxSessionTTL},
		{"within range -> unchanged", 2 * time.Hour, 2 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newSessionRegistry(tc.request, 0, 0)
			defer r.Close()
			if r.ttl != tc.want {
				t.Fatalf("ttl = %v, want %v", r.ttl, tc.want)
			}
		})
	}
}
