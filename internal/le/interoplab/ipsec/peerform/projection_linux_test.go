//go:build linux

// Design: docs/architecture/testing/interop.md -- readback must identify this fixture's SA.
// Related: socket_linux.go -- vendor projection; replay_test.go -- extended-window bounds.
package main

import (
	"encoding/binary"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestProjectStateAcceptsBothNonESNReplayRepresentations keeps large windows in scope.
func TestProjectStateAcceptsBothNonESNReplayRepresentations(t *testing.T) {
	legacy := nl.XfrmReplayState{OSeq: 0xfedcba98}
	for _, attribute := range []*nl.RtAttr{
		nl.NewRtAttr(nl.XFRMA_REPLAY_VAL, legacy.Serialize()),
		nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, extendedReplayFixture()),
	} {
		got, err := projectState(stateMessageFixture(0, attribute))
		if err != nil {
			t.Fatal(err)
		}
		if got.key != fixtureKey() {
			t.Fatal("vendor readback changed the exact state identity")
		}
		if got.sequence != 0xfedcba98 {
			t.Fatalf("non-ESN OSeq = %#x, want all 32 bits", got.sequence)
		}
	}
}

// TestProjectStatePreservesNonESNSequence covers both kernel replay metadata forms.
func TestProjectStatePreservesNonESNSequence(t *testing.T) {
	legacy := nl.XfrmReplayState{OSeq: 0xfedcba98}
	for _, attribute := range []*nl.RtAttr{
		nl.NewRtAttr(nl.XFRMA_REPLAY_VAL, legacy.Serialize()),
		nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, extendedReplayFixture()),
	} {
		got, err := projectState(stateMessageFixture(0, attribute))
		if err != nil {
			t.Fatal(err)
		}
		if got.key != fixtureKey() {
			t.Fatalf("projected identity = %+v, want %+v", got.key, fixtureKey())
		}
		if got.reqid != 7 {
			t.Fatalf("projected reqid = %d, want 7", got.reqid)
		}
		if got.sequence != 0xfedcba98 {
			t.Fatalf("projected OSeq = %#x, want full unsigned value", got.sequence)
		}
		if got.encapsulation.form != formBare {
			t.Fatal("unencapsulated state was not projected as bare ESP")
		}
	}
}

// TestProjectStateRefusesESNAndOtherContexts keeps unsupported identity away from mutation.
func TestProjectStateRefusesESNAndOtherContexts(t *testing.T) {
	legacy := nl.XfrmReplayState{OSeq: 41}
	replay := nl.NewRtAttr(nl.XFRMA_REPLAY_VAL, legacy.Serialize())
	for _, test := range []struct {
		name    string
		message []byte
	}{
		{"esn-flag-with-legacy", stateMessageFixture(nl.XFRM_STATE_ESN, replay)},
		{"esn-flag-with-extended", stateMessageFixture(nl.XFRM_STATE_ESN, nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, extendedReplayFixture()))},
		{"marked", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_MARK, make([]byte, 8)))},
		{"output-marked", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_SET_MARK, make([]byte, 4)))},
		{"output-mark-mask", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_SET_MARK_MASK, make([]byte, 4)))},
		{"if-id", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_IF_ID, make([]byte, 4)))},
		{"offloaded", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_OFFLOAD_DEV, make([]byte, 8)))},
		{"inbound", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_SA_DIR, []byte{1}))},
		{"short-direction", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_SA_DIR, nil))},
		{"duplicate-direction", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_SA_DIR, []byte{2}), nl.NewRtAttr(nl.XFRMA_SA_DIR, []byte{2}))},
		{"missing-replay", stateMessageFixture(0)},
		{"short-replay", stateMessageFixture(0, nl.NewRtAttr(nl.XFRMA_REPLAY_VAL, make([]byte, 11)))},
		{"duplicate-replay", stateMessageFixture(0, replay, replay)},
		{"both-replay-forms", stateMessageFixture(0, replay, nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, extendedReplayFixture()))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := projectState(test.message); err == nil {
				t.Fatal("unsupported state projection was accepted")
			}
		})
	}
}

// TestProjectStateReadsUDPPorts interprets only encapsulation metadata, never keys.
func TestProjectStateReadsUDPPorts(t *testing.T) {
	legacy := nl.XfrmReplayState{OSeq: 41}
	encap := nl.XfrmEncapTmpl{EncapType: unix.UDP_ENCAP_ESPINUDP, EncapSport: nl.Swap16(4500), EncapDport: nl.Swap16(4501)}
	got, err := projectState(stateMessageFixture(0,
		nl.NewRtAttr(nl.XFRMA_REPLAY_VAL, legacy.Serialize()),
		nl.NewRtAttr(nl.XFRMA_ENCAP, encap.Serialize()),
		nl.NewRtAttr(nl.XFRMA_SA_DIR, []byte{2}),
	))
	if err != nil {
		t.Fatal(err)
	}
	want := encapsulation{form: formUDP, sourcePort: 4500, targetPort: 4501}
	if got.encapsulation != want {
		t.Fatalf("encapsulation = %+v, want %+v", got.encapsulation, want)
	}
}

// TestProjectStateRejectsMalformedExtendedHeader exercises the projection before any socket.
func TestProjectStateRejectsMalformedExtendedHeader(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"header-short", func(value []byte) []byte { return value[:23] }},
		{"bitmap-short", func(value []byte) []byte { return value[:28] }},
		{"oseq-high", func(value []byte) []byte { binary.NativeEndian.PutUint32(value[12:16], 1); return value }},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := stateMessageFixture(0, nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, test.edit(extendedReplayFixture())))
			if _, err := projectState(message); err == nil {
				t.Fatal("invalid extended replay metadata reached a usable projection")
			}
		})
	}
}

func stateMessageFixture(flags uint8, attributes ...*nl.RtAttr) []byte {
	info := nl.XfrmUsersaInfo{Family: unix.AF_INET, Mode: 1, Reqid: 7, Flags: flags}
	key := fixtureKey()
	source, target := key.source.As4(), key.target.As4()
	copy(info.Saddr[:], source[:])
	copy(info.Id.Daddr[:], target[:])
	info.Id.Spi, info.Id.Proto = nl.Swap32(key.spi), unix.IPPROTO_ESP
	message := append([]byte(nil), info.Serialize()...)
	for _, attribute := range attributes {
		message = append(message, attribute.Serialize()...)
	}
	return message
}
