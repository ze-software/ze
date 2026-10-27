package ribevents

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRecoveryArgsRoundTrip checks the command form the reactor writes is the
// form the RIB reads, the limit included, and that an unlimited request
// carries no limit pair.
// PREVENTS: the IPC writer and the command parser drifting apart.
func TestRecoveryArgsRoundTrip(t *testing.T) {
	source := netip.MustParseAddr("192.0.2.11")
	for _, limit := range []uint16{0, 1, 2, 65535} {
		request := RecoveryRequest{Destination: netip.MustParseAddr("192.0.2.20"), Family: family.IPv4Unicast,
			AddPath: true, SentAddPath: true, Limit: limit, Keys: []RecoveryKey{
				{NLRI: []byte{0, 0, 0, 1, 24, 198, 51, 100}},
				{NLRI: []byte{0, 0, 0, 2, 24, 198, 51, 101}, Excluded: []netip.Addr{source}},
			}}
		args := AppendRecoveryArgs(nil, &request)
		if got := slices.Contains(args, recoveryLimitKeyword); got != (limit > 0) {
			t.Fatalf("limit %d: limit keyword present %v in %q", limit, got, args)
		}
		parsed, err := ParseRecoveryArgs(args)
		if err != nil {
			t.Fatalf("limit %d: %v", limit, err)
		}
		if parsed.Limit != limit {
			t.Fatalf("limit %d: parsed %d", limit, parsed.Limit)
		}
		if parsed.Destination != request.Destination {
			t.Fatalf("destination %s, want %s", parsed.Destination, request.Destination)
		}
		if parsed.Family != request.Family {
			t.Fatalf("family %s, want %s", parsed.Family, request.Family)
		}
		if !parsed.AddPath {
			t.Fatalf("limit %d: ADD-PATH lost", limit)
		}
		if !parsed.SentAddPath {
			t.Fatalf("limit %d: sent ADD-PATH lost", limit)
		}
		if len(parsed.Keys) != len(request.Keys) {
			t.Fatalf("limit %d: %d keys, want %d", limit, len(parsed.Keys), len(request.Keys))
		}
		for i := range parsed.Keys {
			if FormatRecoveryKey(&parsed.Keys[i]) != FormatRecoveryKey(&request.Keys[i]) {
				t.Fatalf("limit %d: key %d changed", limit, i)
			}
		}
	}
}

// TestCheckRecoveryAnswerRefusesSurvivorsOverLimit checks the reactor-side
// check refuses a limited answer holding more survivors than the bound, and
// accepts one at the bound.
// PREVENTS: an RIB answer that would make the writer exceed a peer's limit.
func TestCheckRecoveryAnswerRefusesSurvivorsOverLimit(t *testing.T) {
	key := []byte{0, 0, 0, 1, 24, 198, 51, 100}
	request := RecoveryRequest{Family: family.IPv4Unicast, AddPath: true, SentAddPath: true, Limit: 2,
		Keys: []RecoveryKey{{NLRI: key}}}
	survivors := []RecoveryRoute{{PathID: 1}, {PathID: 2}, {PathID: 3}}
	if err := CheckRecoveryAnswer(&request, []RecoveryOutcome{{NLRI: key, Survivors: survivors[:2]}}); err != nil {
		t.Fatalf("answer at the bound refused: %v", err)
	}
	if err := CheckRecoveryAnswer(&request, []RecoveryOutcome{{NLRI: key, Survivors: survivors}}); err == nil {
		t.Fatal("answer over the bound accepted")
	}
	request.Limit = 0
	if err := CheckRecoveryAnswer(&request, []RecoveryOutcome{{NLRI: key, Survivors: survivors}}); err != nil {
		t.Fatalf("unlimited answer refused: %v", err)
	}
}
