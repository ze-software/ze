// Design: docs/architecture/mrt.md — capability-derived identities and bounded stream evidence.
package mrt_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestMRTContextCapabilityASNBoundaries passes literal OPEN capabilities through
// the public writer/reader, then distinguishes real AS4 identities from AS_TRANS
// and truncated low words in both directions. No negotiated context is injected.
func TestMRTContextCapabilityASNBoundaries(t *testing.T) {
	cases := []struct {
		name                  string
		actual, header        uint32
		openAS                uint16
		as4, wantUnavailable  bool
	}{
		{"largest-two-octet", 65535, 65535, 65535, false, false},
		{"first-four-octet", 65536, 65536, 23456, true, false},
		{"largest-four-octet", 0xffffffff, 0xffffffff, 23456, true, false},
		{"capability-overrides-my-as", 70001, 70001, 65001, true, false},
		{"narrow-as-trans", 65536, 23456, 23456, false, false},
		{"narrow-as-trans-largest", 0xffffffff, 23456, 23456, false, false},
		{"small-as-is-not-as-trans", 65535, 23456, 65535, false, true},
		{"wide-as-trans-is-not-identity", 65536, 23456, 23456, true, true},
		{"low-word-is-not-identity", 70001, 4465, 23456, false, true},
		{"same-low-word-different-as", 70001, 135537, 23456, true, true},
		{"my-as-is-not-capability-identity", 70001, 65001, 65001, true, true},
	}
	for _, tc := range cases {
		for _, local := range []bool{false, true} {
			for _, sent := range []bool{false, true} {
				name := tc.name + "/peer"
				if local {
					name = tc.name + "/local"
				}
				if sent {
					name += "/sent"
				} else {
					name += "/received"
				}
				t.Run(name, func(t *testing.T) {
					h := mrt.BGP4MPHeader{PeerAS: 65001, LocalAS: 65000, AFI: 1,
						PeerIP: []byte{192, 0, 2, 1}, LocalIP: []byte{192, 0, 2, 2}}
					peerASN, localASN := uint32(65001), uint32(65000)
					peerMyAS, localMyAS := uint16(65001), uint16(65000)
					if local {
						localASN, localMyAS, h.LocalAS = tc.actual, tc.openAS, tc.header
					} else {
						peerASN, peerMyAS, h.PeerAS = tc.actual, tc.openAS, tc.header
					}
					var stream bytes.Buffer
					contextBoundaryRecord(&stream, &h, 4, contextBoundaryOpen(peerMyAS, peerASN, false))
					contextBoundaryRecord(&stream, &h, 7, contextBoundaryOpen(localMyAS, localASN, true))
					subtype := uint16(8)
					if tc.as4 {
						subtype = 9
					}
					if sent {
						subtype += 2
					}
					update := buildBGPMessage(2, mixedUpdate(sent))
					contextBoundaryRecord(&stream, &h, subtype, update)
					seen := 0
					err := mrt.ReadFrom(&stream, &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, record *mrt.MessageRecord) error {
						if record.BGPMessage.Bytes[18] != 2 {
							return nil
						}
						seen++
						if !bytes.Equal(record.BGPMessage.Bytes, update) {
							t.Fatal("writer/reader changed the UPDATE")
						}
						parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
						if err != nil {
							return err
						}
						contextBoundaryPrefixes(t, parsed.Update, sent)
						return nil
					}})
					if seen != 1 {
						t.Fatalf("UPDATE callbacks=%d, want 1; error=%v", seen, err)
					}
					if tc.wantUnavailable {
						if !errors.Is(err, mrt.ErrContextUnavailable) {
							t.Fatalf("mismatched identity supplied negotiation: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// TestMRTContextExhaustion refuses the 65537th endpoint without silently evicting
// an earlier handshake. Existing endpoints can finish/restart at the limit;
// actual NOTIFICATION and state-change records release a slot for a new peer.
func TestMRTContextExhaustion(t *testing.T) {
	for _, release := range []string{"none", "notification", "state"} {
		t.Run(release, func(t *testing.T) {
			var stream bytes.Buffer
			h := mrt.BGP4MPHeader{PeerAS: 65001, LocalAS: 65000, AFI: 1,
				PeerIP: []byte{10, 0, 0, 0}, LocalIP: []byte{192, 0, 2, 2}}
			remote := contextBoundaryOpen(65001, 65001, false)
			local := contextBoundaryOpen(65000, 65000, true)
			update := buildBGPMessage(2, mixedUpdate(false))
			for n := range 65536 {
				binary.BigEndian.PutUint32(h.PeerIP, 0x0a000000+uint32(n))
				contextBoundaryRecord(&stream, &h, 4, remote)
			}
			// Complete the oldest and newest entries while the store is full.
			for _, address := range []uint32{0x0a000000, 0x0a00ffff} {
				binary.BigEndian.PutUint32(h.PeerIP, address)
				contextBoundaryRecord(&stream, &h, 7, local)
				contextBoundaryRecord(&stream, &h, 9, update)
			}
			// A new epoch for an existing key must not consume another slot.
			contextBoundaryRecord(&stream, &h, 4, remote)
			contextBoundaryRecord(&stream, &h, 7, local)
			contextBoundaryRecord(&stream, &h, 9, update)
			wantMessages := 65543
			switch release {
			case "notification":
				contextBoundaryRecord(&stream, &h, 4, buildBGPMessage(3, []byte{6, 4}))
				wantMessages++
			case "state":
				buf := make([]byte, 64)
				n := mrt.WriteBGP4MPStateChange(buf, 12, &h, true, mrt.FSMEstablished, mrt.FSMIdle)
				mrt.WriteCommonHeader(buf, 0, 1, mrt.TypeBGP4MP, mrt.BGP4MPStateChangeAS4, uint32(n))
				stream.Write(buf[:12+n])
			}
			binary.BigEndian.PutUint32(h.PeerIP, 0x0a010000)
			contextBoundaryRecord(&stream, &h, 4, remote)
			contextBoundaryRecord(&stream, &h, 7, local)
			contextBoundaryRecord(&stream, &h, 9, update)
			seen, updates := 0, 0
			err := mrt.ReadFrom(&stream, &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, record *mrt.MessageRecord) error {
				seen++
				if record.BGPMessage.Bytes[18] != 2 {
					return nil
				}
				updates++
				parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
				if err != nil {
					return err
				}
				contextBoundaryPrefixes(t, parsed.Update, false)
				return nil
			}})
			if release == "none" {
				if err == nil || !strings.Contains(err.Error(), "session context limit (65536) exceeded") {
					t.Fatalf("overflow must be explicit, got %v", err)
				}
				if seen != wantMessages || updates != 3 {
					t.Fatalf("before overflow messages/updates=%d/%d, want %d/3", seen, updates, wantMessages)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if seen != wantMessages+3 || updates != 4 {
				t.Fatalf("after slot release messages/updates=%d/%d, want %d/4", seen, updates, wantMessages+3)
			}
		})
	}
}

// contextBoundaryOpen keeps MyAS and capability 65 independent, and advertises
// opposite per-family ADD-PATH directions using literal capability bytes.
func contextBoundaryOpen(myAS uint16, asn uint32, sent bool) []byte {
	mode4, mode6 := byte(1), byte(2)
	if sent {
		mode4, mode6 = 2, 1
	}
	caps := []byte{1, 4, 0, 1, 0, 1, 1, 4, 0, 2, 0, 1, 65, 4}
	caps = binary.BigEndian.AppendUint32(caps, asn)
	caps = append(caps, 69, 8, 0, 1, 1, mode4, 0, 2, 1, mode6)
	body := []byte{4, byte(myAS >> 8), byte(myAS), 0, 90, 192, 0, 2, 1, byte(2 + len(caps)), 2, byte(len(caps))}
	return buildBGPMessage(1, append(body, caps...))
}

func contextBoundaryRecord(stream *bytes.Buffer, h *mrt.BGP4MPHeader, subtype uint16, wire []byte) {
	buf := make([]byte, 64+len(wire))
	n := mrt.WriteBGP4MPMessage(buf, 12, h, mrt.IsAS4Subtype(subtype), wire)
	mrt.WriteCommonHeader(buf, 0, 1, mrt.TypeBGP4MP, subtype, uint32(n))
	stream.Write(buf[:12+n])
}

func contextBoundaryPrefixes(t *testing.T, u *mrt.ParsedUpdate, sent bool) {
	t.Helper()
	if !slices.Equal(u.AnnouncedPrefixes, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}) ||
		!slices.Equal(u.WithdrawnPrefixes, []netip.Prefix{netip.MustParsePrefix("10.1.0.0/24")}) {
		t.Fatalf("classic prefixes=%v/%v", u.AnnouncedPrefixes, u.WithdrawnPrefixes)
	}
	reach, err := mrt.ParseMPReach(mrt.FindAttribute(u.Attributes, 14).Value, u.AddPathFor(2, 1))
	if err != nil {
		t.Fatal(err)
	}
	unreach, err := mrt.ParseMPUnreach(mrt.FindAttribute(u.Attributes, 15).Value, u.AddPathFor(2, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(reach.Prefixes, []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}) ||
		!slices.Equal(unreach.Prefixes, []netip.Prefix{netip.MustParsePrefix("2001:db9::/32")}) {
		t.Fatalf("MP prefixes=%v/%v", reach.Prefixes, unreach.Prefixes)
	}
	annIDs, wdIDs := reach.PathIDs, unreach.PathIDs
	if sent {
		annIDs, wdIDs = u.AnnouncedPathIDs, u.WithdrawnPathIDs
		if len(reach.PathIDs)+len(unreach.PathIDs) != 0 {
			t.Fatal("ordinary MP invented identifiers")
		}
	} else if len(u.AnnouncedPathIDs)+len(u.WithdrawnPathIDs) != 0 {
		t.Fatal("ordinary classic invented identifiers")
	}
	if !slices.Equal(annIDs, []uint32{0x01020304}) || !slices.Equal(wdIDs, []uint32{0x05060708}) {
		t.Fatalf("identifiers=%x/%x", annIDs, wdIDs)
	}
}
