// Design: docs/architecture/testing/interop.md -- actual-destination LOCAL_PREF proof.
package bgp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestLocalPrefStripChecksDestinationWire keeps the peer's normalized RIB the
// same for stripped and leaked inputs. Only the actual destination wire changes.
// These are checker-boundary regressions, not evidence of a native GoBGP run.
func TestLocalPrefStripChecksDestinationWire(t *testing.T) {
	clean := localPrefTestCapture(t, -1)
	for _, tc := range []struct {
		name string
		wire string
		bad  bool
	}{
		{"stripped", clean, false},
		{"leaked200", localPrefTestCapture(t, 200), true},
		{"leaked0", localPrefTestCapture(t, 0), true},
		{"missing", "", true},
		{"malformed", "{", true},
		{"truncated", clean[:len(clean)-2], true},
		{"wrong connection", strings.ReplaceAll(clean, "172.30.0.5:179", "172.30.0.3:179"), true},
		{"missing completion", clean[:strings.LastIndex(strings.TrimSpace(clean), "\n")+1], true},
		{"suppressed third", strings.ReplaceAll(clean, "180a3602", "180a3603"), true},
		{"replaced source", strings.ReplaceAll(clean, "ac1e0009", "ac1e0002"), true},
		{"missing fence", strings.ReplaceAll(clean, "180a36ff", "180a36fe"), true},
		// RFC 4271 Section 4.3 lets Ze pack NLRI sharing path attributes into one
		// UPDATE. Those routes arrive together, so the fence is accepted in the
		// UPDATE that completes the three subjects, in either NLRI order, and is
		// refused in an UPDATE earlier than the third subject's.
		{"third and fence batched", localPrefTestCaptureTail(t, -1, []byte{24, 10, 54, 2, 24, 10, 54, 255}), false},
		{"fence before third batched", localPrefTestCaptureTail(t, -1, []byte{24, 10, 54, 255, 24, 10, 54, 2}), false},
		{"fence UPDATE before third UPDATE", localPrefTestCaptureTail(t, -1, []byte{24, 10, 54, 255}, []byte{24, 10, 54, 2}), true},
		{"batched leaked200", localPrefTestCaptureTail(t, 200, []byte{24, 10, 54, 2, 24, 10, 54, 255}), true},
		{"observed batched frames", localPrefTestObservedCapture(t), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()
			lab := &localPrefWireLab{wire: tc.wire, reverse: localPrefTestReverse(t)}
			err := checkLocalPrefStrip(ctx, &interoplab.CheckContext{
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}, Lab: lab,
			})
			if (err != nil) != tc.bad {
				t.Fatalf("checker error=%v; want refusal=%t with unchanged normalized RIB", err, tc.bad)
			}
			if !tc.bad && lab.wireReads == 0 {
				t.Fatal("checker certified stripping without reading the destination connection")
			}
		})
	}
}

// TestLocalPrefStripHoldsInjectorUntilZeEstablished proves assertion 16
// through the whole checker: with either destination short of Established in
// Ze's own peer list, checkLocalPrefStrip fails at assertion 16, names that
// destination, never releases the injector and never reads the wire.
//
// Method: the stripped capture that passes TestLocalPrefStripChecksDestinationWire,
// with one destination reported in connect. Both daemons still report their
// sessions Established, so only Ze's peer state can hold the release.
func TestLocalPrefStripHoldsInjectorUntilZeEstablished(t *testing.T) {
	for _, destination := range []string{"172.30.0.3", "172.30.0.10"} {
		t.Run(destination, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()
			lab := &localPrefWireLab{wire: localPrefTestCapture(t, -1), reverse: localPrefTestReverse(t), notEstablished: destination}
			err := checkLocalPrefStrip(ctx, &interoplab.CheckContext{
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}, Lab: lab,
			})
			if err == nil {
				t.Fatalf("destination %s not Established in ze, and the checker passed", destination)
			}
			if !strings.Contains(err.Error(), "assertion 16:") || !strings.Contains(err.Error(), destination) {
				t.Fatalf("err=%v, want assertion 16 naming %s", err, destination)
			}
			if lab.releases != 0 {
				t.Fatalf("injector released %d times while %s was not Established", lab.releases, destination)
			}
			if lab.wireReads != 0 {
				t.Fatalf("checker read the destination wire %d times past a failed release", lab.wireReads)
			}
		})
	}
}

// localPrefTestCapture sends the third subject and the fence in UPDATEs of
// their own, the unbatched order.
func localPrefTestCapture(t *testing.T, preference int) string {
	t.Helper()
	return localPrefTestCaptureTail(t, preference, []byte{24, 10, 54, 2}, []byte{24, 10, 54, 255})
}

// localPrefTestCaptureTail sends OPEN, KEEPALIVE, EOR and the initial pair, then
// one UPDATE per tail NLRI block, every UPDATE with the same attributes.
func localPrefTestCaptureTail(t *testing.T, preference int, tail ...[]byte) string {
	t.Helper()
	open, err := speakerOpen(65001, 90, net.ParseIP("172.30.0.2"), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 10, 2, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xec, 0x40, 3, 4, 172, 30, 0, 9}
	if preference >= 0 {
		attrs = append(attrs, 0x40, 5, 4, 0, 0, 0, byte(preference))
	}
	frames := [][]byte{
		open, speakerKeepalive(), speakerEOR(),
		speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 10, 54, 0, 24, 10, 54, 1})),
	}
	for _, nlri := range tail {
		frames = append(frames, speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, nlri)))
	}
	return localPrefTestRows(t, frames, "172.30.0.2:179", "172.30.0.5:179", "local-pref-subjects-through-fifo-fence")
}

// localPrefTestObservedCapture replays the frames Ze sent toward GoBGP in a lab
// run (2026-11-09) that batched the third subject 10.54.2.0/24 and the fence
// 10.54.255.0/24 into one UPDATE, which the old observer refused as a fence
// preceding the subjects.
func localPrefTestObservedCapture(t *testing.T) string {
	t.Helper()
	var frames [][]byte
	for _, text := range []string{
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002B0104FDE9005AAC1E00020E020C01040001000141040000FDE9",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003702000000184001010040020A02020000FDE90000FDEC400304AC1E0009180A3600180A3601",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003702000000184001010040020A02020000FDE90000FDEC400304AC1E0009180A3602180A36FF",
	} {
		frame, err := hex.DecodeString(text)
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	return localPrefTestRows(t, frames, "172.30.0.2:179", "172.30.0.5:179", "local-pref-subjects-through-fifo-fence")
}

func localPrefTestReverse(t *testing.T) string {
	t.Helper()
	open, err := speakerOpen(65003, 90, net.ParseIP("172.30.0.5"), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	return localPrefTestRows(t, [][]byte{open, speakerKeepalive(), speakerEOR(), speakerKeepalive()},
		"172.30.0.5:179", "172.30.0.2:179", "keepalive-after-local-pref-fifo-fence")
}

func localPrefTestRows(t *testing.T, frames [][]byte, source, target, boundary string) string {
	t.Helper()
	var capture strings.Builder
	encoder := json.NewEncoder(&capture)
	for _, frame := range frames {
		if err := encoder.Encode(map[string]string{"original": hex.EncodeToString(frame)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := encoder.Encode(map[string]any{"complete": map[string]any{
		"source": source, "target": target, "frames": len(frames), "boundary": boundary,
	}}); err != nil {
		t.Fatal(err)
	}
	return capture.String()
}

// localPrefWireLab answers every query checkLocalPrefStrip makes. Ze's peer
// list holds both destinations Established unless notEstablished names one,
// which it then reports in connect. The injector accepts only the release
// write, counted in releases.
type localPrefWireLab struct {
	noEvidenceLab
	wire, reverse  string
	notEstablished string
	wireReads      int
	releases       int
}

func (lab *localPrefWireLab) Exec(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if peer == peerInject && slices.Equal(command, []string{"sh", "-c", ": > " + localPrefRelease}) {
		lab.releases++
		return interoplab.CommandResult{}, nil
	}
	return interoplab.CommandResult{}, fmt.Errorf("unexpected peer %s exec %q", peer, command)
}

func (lab *localPrefWireLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	if peer == "ze" {
		if !slices.Equal(command, zeCommand("show bgp peer list")) {
			return "", fmt.Errorf("unexpected ze query %q", command)
		}
		states := map[string]string{"172.30.0.3": peerStateEstablished, "172.30.0.10": peerStateEstablished}
		if lab.notEstablished != "" {
			states[lab.notEstablished] = "connect"
		}
		document := map[string]map[string]map[string]string{"peers": {}}
		for address, state := range states {
			document["peers"][address] = map[string]string{"state": state}
		}
		body, err := json.Marshal(document)
		return string(body), err
	}
	if peer == peerSpeaker {
		lab.wireReads++
		switch strings.Join(command, " ") {
		case "cat /tmp/extended-ze.jsonl":
			return lab.wire, nil
		case "cat /tmp/extended-frr.jsonl":
			return lab.reverse, nil
		default:
			return "", fmt.Errorf("unexpected capture query %q", command)
		}
	}
	if peer == peerFRR {
		if strings.Contains(strings.Join(command, " "), "neighbor") {
			return "BGP state = Established", nil
		}
		return fmt.Sprintf(`{"prefix":%q,"paths":[{"valid":true}]}`, strings.Fields(command[2])[4]), nil
	}
	if peer == peerGoBGP {
		if len(command) > 1 && command[1] == gobgpNeighbor {
			return "BGP state = Established", nil
		}
		return gobgpTable(
			gobgpRouteLine("10.54.0.0/24", "172.30.0.9", "65001 65004", "[{Origin: i}]"),
			gobgpRouteLine("10.54.1.0/24", "172.30.0.9", "65001 65004", "[{Origin: i}]"),
			gobgpRouteLine("10.54.2.0/24", "172.30.0.9", "65001 65004", "[{Origin: i}]"),
		), nil
	}
	return "", fmt.Errorf("unexpected peer %s query %q", peer, command)
}

// localPrefRecordingDirectory holds the reviewed recording of one lab run of
// bgp-local-pref-strip-gobgp, copied from the session scratch
// (interoplab.RecordingDirectory) after a run in which Ze packed the third
// subject and the fence into one UPDATE.
const localPrefRecordingDirectory = "testdata/bgp-local-pref-strip-gobgp"

// TestLocalPrefStripReplaysTheLabRecording replays the committed recording of
// a passing lab run through the scenario's registered checker
// (interoplab.ReplayScenario), so the checker is judged on the bytes a real
// GoBGP run produced, with no Docker.
// VALIDATES: AC-8. The checker passes on the recorded run exactly as the lab
// did; on the pre-fix observer of 7081cfdfa7, which refused a fence packed
// after the third subject in one UPDATE, the same replay fails. The
// hand-built frames of TestLocalPrefStripChecksDestinationWire stay: they
// cover polarities no lab run produced.
// PREVENTS: an observer change judged only by a lab rerun, as the 7081cfdfa7
// fix was.
func TestLocalPrefStripReplaysTheLabRecording(t *testing.T) {
	checkout, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	recordings, err := filepath.Glob(filepath.Join(localPrefRecordingDirectory, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recordings) != 1 {
		t.Fatalf("%s holds %d recordings, want exactly one. Fix: run `INTEROP_SCENARIO=bgp-local-pref-strip-gobgp ./le test integration interop`, "+
			"copy the recording it names from <session scratch>/%s/bgp/ into %s, review it like any fixture, and commit it",
			localPrefRecordingDirectory, len(recordings), interoplab.RecordingDirectory, localPrefRecordingDirectory)
	}
	result, err := interoplab.ReplayScenario(t.Context(), checkout, recordings[0])
	if err != nil {
		t.Fatalf("replay of %s: %v", recordings[0], err)
	}
	if !result.Lab.Passed {
		t.Fatalf("the committed recording is of a failed lab run (%s); commit one of a passing run", result.Lab.Error)
	}
	if result.Replay != result.Lab {
		t.Errorf("the checker replayed verdict %#v on the recording of a lab run whose verdict was %#v", result.Replay, result.Lab)
	}
}
