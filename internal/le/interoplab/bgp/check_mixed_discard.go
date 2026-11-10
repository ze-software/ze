// Design: docs/architecture/testing/interop.md -- real withdrawal and strict sender wire shape.
package bgp

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	mixedReplayPrefix    = "10.0.0.0/24"
	mixedSeedPrefix      = "198.51.100.0/24"
	mixedAnnouncedPrefix = "203.0.113.0/24"
	mixedFencePrefix     = "198.51.101.0/24"
)

// checkMixedUpdateWire observes the seed in FRR before releasing the ONE mixed
// source UPDATE. The source MUST await the scoped release route; the checker
// MUST NOT send it before both the BGP and installed seed have been measured.
func checkMixedUpdateWire(ctx context.Context, check *interoplab.CheckContext) error {
	const name = "bgp-rfc7606-relay-shape-frr"
	fail := func(assertion int, err error) error { return checkerFailure(ctx, check.Lab, name, assertion, err) }
	base := netip.MustParseAddr(baseIPv4Prefix + "0")
	if !check.Network.IPv4.IsValid() || check.Network.IPv4.Addr() != base {
		return fail(1, fmt.Errorf("scenario runs on network %s, and the injected NEXT_HOP octets name a peer only on %s", check.Network.IPv4, base))
	}
	neighbor := networkHostAddress(check.Network, 11)
	session := operation{kind: opFRRSession, argument: neighbor}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(2, err)
	}
	for _, host := range []uint8{9, 11} {
		if err := waitZePeerState(ctx, check.Lab, networkHostAddress(check.Network, host), 60*time.Second); err != nil {
			return fail(3, err)
		}
	}
	for _, prefix := range []string{mixedReplayPrefix, mixedSeedPrefix} {
		if err := mixedWaitBGP(ctx, check.Lab, prefix, true); err != nil {
			return fail(4, err)
		}
		if err := mixedWaitInstalled(ctx, check.Lab, prefix); err != nil {
			return fail(5, err)
		}
	}
	route, err := queryFRRIPv4Route(ctx, check.Lab, mixedReplayPrefix)
	if err != nil {
		return fail(6, err)
	}
	if err := requireFRRASPath(route, "65004"); err != nil {
		return fail(6, err)
	}
	// The existing raw peer matches the announced NLRI section, not a timer
	// or an arbitrary occurrence of these octets in an attribute.
	command := zeCommand("send bgp " + networkHostAddress(check.Network, 9) + " update text origin igp nhop " + networkHostAddress(check.Network, 2) + " nlri add 192.0.2.0/24")
	if _, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command)); err != nil {
		return fail(7, err)
	}
	for _, prefix := range []string{mixedAnnouncedPrefix, mixedFencePrefix} {
		if err := mixedWaitBGP(ctx, check.Lab, prefix, true); err != nil {
			return fail(8, err)
		}
		if err := mixedWaitInstalled(ctx, check.Lab, prefix); err != nil {
			return fail(9, err)
		}
	}
	if err := mixedWaitBGP(ctx, check.Lab, mixedSeedPrefix, false); err != nil {
		return fail(10, err)
	}
	var lastAnswer string
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "strict split UPDATE capture through source fence"}, func(probe context.Context) (bool, error) {
		answer, err := check.Lab.Query(probe, peerSpeaker2, []string{cmdCat, extendedCaptureBase + "-ze.jsonl"}, nil)
		if err != nil {
			return false, err
		}
		lastAnswer = answer
		if err := requireMixedSplitWire(answer); err != nil {
			return false, err
		}
		return true, nil
	}, func(done bool) bool { return done })
	if err != nil {
		return fail(11, interoplab.WithLastAnswer(err, lastAnswer))
	}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(12, err)
	}
	return nil
}

func mixedWaitBGP(ctx context.Context, lab interoplab.CheckerLab, prefix string, present bool) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "FRR exact route " + prefix}, func(probe context.Context) (string, error) {
		return lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", showTableJSON}, nil)
	}, func(answer string) bool {
		found, measured := jsonHasRoute(answer, prefix)
		return measured && found == present
	})
	return err
}

func mixedWaitInstalled(ctx context.Context, lab interoplab.CheckerLab, prefix string) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "FRR installs " + prefix}, func(probe context.Context) (string, error) {
		return lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show ip route bgp json"}, nil)
	}, func(answer string) bool {
		if prefix == mixedFencePrefix {
			// The installed trailing fence and removal of the previously
			// installed seed MUST be observed in the same native table.
			var table map[string]json.RawMessage
			if err := json.Unmarshal([]byte(answer), &table); err != nil {
				return false
			}
			if _, exists := table[mixedSeedPrefix]; exists {
				return false
			}
		}
		return requireRouteInstalledVia(answer, prefix, baseIPv4Prefix+"9", "bgp") == nil
	})
	return err
}

// requireMixedSplitWire inspects a complete, byte-transparent FIFO prefix ending
// at the source's trailing route. FRR 10.7.1 discards later duplicate attributes
// in bgp_attr_parse; its installed route alone cannot prove this sender property.
func requireMixedSplitWire(capture string) error {
	if len(capture) > 34*1024*1024 {
		return errors.New("mixed capture exceeds bounded frame budget")
	}
	decoder := json.NewDecoder(strings.NewReader(capture))
	counts := map[string]int{}
	withdrawals := 0
	opens, keepalives, frames := 0, 0, 0
	var buffer [65535]byte
	for {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return err
		}
		frames++
		if frames > 256 {
			return errors.New("mixed capture exceeds 256 frames")
		}
		length, err := readExtendedFrame(bytes.NewReader(frame), buffer[:])
		if err != nil {
			return err
		}
		if length != len(frame) {
			return errors.New("mixed capture length differs from BGP header")
		}
		if _, err := validateSpeakerMessageHeader(frame[:19]); err != nil {
			return err
		}
		if row.Delivered != "" {
			delivered, err := hex.DecodeString(row.Delivered)
			if err != nil {
				return err
			}
			if frame[18] != bgpOpen || !bytes.Equal(frame, delivered) {
				return errors.New("mixed relay changed a frame")
			}
		}
		if frame[18] == bgpOpen {
			opens++
			if opens != 1 || frames != 1 {
				return errors.New("mixed capture has a replacement OPEN")
			}
			if err := validateSpeakerOpen(frame[19:]); err != nil {
				return err
			}
			continue
		}
		if opens != 1 {
			return errors.New("mixed capture lacks an initial valid OPEN")
		}
		switch frame[18] {
		case bgpKeepalive:
			keepalives++
			continue
		case bgpUpdate:
			if keepalives == 0 {
				return errors.New("mixed UPDATE precedes establishment")
			}
		default:
			return fmt.Errorf("unexpected BGP message type %d in mixed capture", frame[18])
		}
		update, err := parsedEmptyMPUpdate(frame[19:])
		if err != nil {
			return err
		}
		if len(update.withdrawn) != 0 {
			if counts[mixedSeedPrefix] != 1 || counts[mixedFencePrefix] != 0 || !bytes.Equal(frame[19:], []byte{0, 4, 24, 198, 51, 100, 0, 0}) {
				return fmt.Errorf("not an independently split seeded withdrawal: %x", frame[19:])
			}
			withdrawals++
			continue
		}
		if len(update.nlri) == 0 {
			if len(update.attributes) != 0 {
				return errors.New("unexpected route-free attributes on IPv4 split rail")
			}
			continue
		}
		seen := map[byte]bool{}
		for _, attr := range update.attributes {
			if seen[attr.code] {
				return fmt.Errorf("split announcement repeats attribute %d", attr.code)
			}
			seen[attr.code] = true
			if attr.code == 1 && (attr.flags != 0x40 || !bytes.Equal(attr.value, []byte{0})) {
				return errors.New("split announcement changed ORIGIN")
			}
			if attr.code == 3 && !bytes.Equal(attr.value, []byte{172, 30, 0, 9}) {
				return errors.New("split announcement changed NEXT_HOP")
			}
			if attr.code == 2 && !bytes.Equal(attr.value, []byte{2, 1, 0, 0, 0xfd, 0xec}) {
				return errors.New("split announcement changed source AS_PATH")
			}
		}
		if !seen[1] || !seen[2] || !seen[3] {
			return errors.New("split announcement lacks mandatory attributes")
		}
		for section := update.nlri; len(section) != 0; section = section[4:] {
			if len(section) < 4 || section[0] != 24 {
				return errors.New("unexpected IPv4 NLRI framing on split rail")
			}
			prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{section[1], section[2], section[3], 0}), 24).String()
			switch prefix {
			case mixedReplayPrefix, mixedSeedPrefix:
				if withdrawals != 0 || counts[mixedFencePrefix] != 0 {
					return errors.New("seed/replay repeated after mixed epoch")
				}
			case mixedAnnouncedPrefix:
				if withdrawals != 1 || counts[mixedFencePrefix] != 0 {
					return errors.New("announcement not preceded by split withdrawal")
				}
			case mixedFencePrefix:
				if withdrawals != 1 || counts[mixedAnnouncedPrefix] != 1 {
					return errors.New("source fence preceded the split pair")
				}
			default:
				return fmt.Errorf("unexpected prefix %s on measured split rail", prefix)
			}
			counts[prefix]++
		}
	}
	for _, prefix := range []string{mixedReplayPrefix, mixedSeedPrefix, mixedAnnouncedPrefix, mixedFencePrefix} {
		if counts[prefix] != 1 {
			return fmt.Errorf("wire count for %s = %d, want one", prefix, counts[prefix])
		}
	}
	if withdrawals != 1 {
		return fmt.Errorf("wire withdrawal count %d, want one", withdrawals)
	}
	return nil
}
