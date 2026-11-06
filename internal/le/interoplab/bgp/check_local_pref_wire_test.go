// Design: docs/architecture/testing/interop.md -- actual-destination LOCAL_PREF proof.
package bgp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
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

func localPrefTestCapture(t *testing.T, preference int) string {
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
		speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 10, 54, 2})),
		speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 10, 54, 255})),
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

type localPrefWireLab struct {
	noEvidenceLab
	wire, reverse string
	wireReads     int
}

func (lab *localPrefWireLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
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
