// Design: docs/architecture/testing/interop.md -- complete LOCAL_PREF intervals permit UPDATE batching.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestLocalPrefStripAcceptsBatchedFence drives the real checker with identical
// normalized RIB answers while the destination wire varies. The initial pair
// precedes EOR; the third subject and fence share only their final UPDATE, in
// either NLRI order, because routes in one UPDATE share one attribute set and
// arrive together.
// RFC 4271 Section 4.3: "An UPDATE message is used to advertise feasible routes that share common path attributes to a peer, or to withdraw multiple unfeasible routes from service (see 3.1)."
func TestLocalPrefStripAcceptsBatchedFence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		preference int
		final      []byte
		want       string
	}{
		{"third then fence", -1, []byte{2, 255}, ""},
		{"fence then third", -1, []byte{255, 2}, ""},
		{"leaked zero with third first", 0, []byte{2, 255}, "LOCAL_PREF appeared on the actual external destination wire"},
		{"leaked zero with fence first", 0, []byte{255, 2}, "LOCAL_PREF appeared on the actual external destination wire"},
		{"leaked 200 with third first", 200, []byte{2, 255}, "LOCAL_PREF appeared on the actual external destination wire"},
		{"leaked 200 with fence first", 200, []byte{255, 2}, "LOCAL_PREF appeared on the actual external destination wire"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := localPrefBatchedTestFrames(t, tc.preference, tc.final)
			localPrefBatchedTestCheck(t, frames, localPrefTestReverse(t), tc.want)
		})
	}
}

// TestLocalPrefStripBatchedFenceRetainsControls changes one interval control at
// a time while leaving the daemons' normalized RIB answers unchanged. Batching
// cannot substitute for a missing subject, EOR, complete fence or reverse rail.
func TestLocalPrefStripBatchedFenceRetainsControls(t *testing.T) {
	for _, tc := range []struct {
		name          string
		final         []byte
		initialThird  bool
		removeEOR     bool
		afterFence    bool
		removeReverse bool
		want          string
	}{
		{name: "missing third subject", final: []byte{255}, want: "LOCAL_PREF FIFO fence arrived before the UPDATE completing all three subjects"},
		{name: "duplicate third subject", final: []byte{2, 2, 255}, want: "unexpected or duplicate LOCAL_PREF subject"},
		{name: "missing fence", final: []byte{2}, want: "LOCAL_PREF completion has no full original-session barrier"},
		{name: "duplicate fence", final: []byte{2, 255, 255}, want: "second LOCAL_PREF FIFO fence in the declared interval"},
		{name: "foreign NLRI", final: []byte{2, 3, 255}, want: "unexpected or duplicate LOCAL_PREF subject"},
		{name: "third shares the initial pair UPDATE", final: []byte{255}, initialThird: true, want: "third LOCAL_PREF subject preceded initial pair or EOR"},
		{name: "missing EOR", final: []byte{2, 255}, removeEOR: true, want: "third LOCAL_PREF subject preceded initial pair or EOR"},
		{name: "subject after fence", final: []byte{2, 255}, afterFence: true, want: "unexpected or duplicate LOCAL_PREF subject"},
		{name: "missing reverse barrier", final: []byte{2, 255}, removeReverse: true, want: "LOCAL_PREF completion has no full original-session barrier"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := localPrefBatchedTestFrames(t, -1, tc.final)
			if tc.initialThird {
				frames[2] = append(frames[2], 24, 10, 54, 2)
				binary.BigEndian.PutUint16(frames[2][16:18], uint16(len(frames[2])))
			}
			if tc.removeEOR {
				frames = append(frames[:3], frames[4:]...)
			}
			if tc.afterFence {
				frames = append(frames, frames[2])
			}
			reverse := localPrefTestReverse(t)
			if tc.removeReverse {
				reverse = localPrefTestRows(t, localPrefBatchedTestRows(t, reverse)[:2],
					"172.30.0.5:179", "172.30.0.2:179", localPrefReverseBoundary)
			}
			localPrefBatchedTestCheck(t, frames, reverse, tc.want)
		})
	}
}

func localPrefBatchedTestCheck(t *testing.T, frames [][]byte, reverse, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	lab := &localPrefWireLab{
		wire:    localPrefTestRows(t, frames, "172.30.0.2:179", "172.30.0.5:179", localPrefSourceBoundary),
		reverse: reverse,
	}
	// RFC 4271 Section 5.1.5: preserve the destination-wire absence obligation.
	err := checkLocalPrefStrip(ctx, &interoplab.CheckContext{
		Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}, Lab: lab,
	})
	if want != "" {
		if err == nil {
			t.Fatalf("checker accepted a broken batched interval; want %q with unchanged normalized RIB", want)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("checker refused the wrong control: %v; want %q", err, want)
		}
		return
	}
	if err != nil {
		t.Fatalf("checker rejected a complete clean batched interval with unchanged normalized RIB: %v", err)
	}
	if lab.wireReads != 2 {
		t.Fatalf("checker read %d wire intervals, want both directions", lab.wireReads)
	}
}

func localPrefBatchedTestFrames(t *testing.T, preference int, final []byte) [][]byte {
	t.Helper()
	frames := localPrefBatchedTestRows(t, localPrefTestCapture(t, preference))
	if len(frames) != 6 {
		t.Fatalf("original fixture has %d frames, want OPEN, KEEPALIVE, EOR, pair, third and fence", len(frames))
	}
	// Keep the original third subject's complete header and attributes. Only
	// its NLRI population changes, and the BGP frame length follows those bytes.
	batched := bytes.Clone(frames[4][:len(frames[4])-4])
	for _, subnet := range final {
		batched = append(batched, 24, 10, 54, subnet)
	}
	binary.BigEndian.PutUint16(batched[16:18], uint16(len(batched)))
	return [][]byte{frames[0], frames[1], frames[3], frames[2], batched}
}

func localPrefBatchedTestRows(t *testing.T, capture string) [][]byte {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(capture))
	var frames [][]byte
	for {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if row.Complete != nil {
			return frames
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
}
