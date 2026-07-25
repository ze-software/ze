package filter_remove_private_as

import (
	"encoding/binary"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: configured filter returns a modify directive when AS_PATH contains private ASNs.
// PREVENTS: route passing unchanged when remove-private-as is configured.
func TestRemovePrivateASFilterUpdate(t *testing.T) {
	defs := map[string]*removePrivateASDef{"STRIP": {name: "STRIP", mode: removeModeStrip}}
	defsByName.Store(&defs)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "STRIP",
		PeerAS: 65001,
		Update: "origin igp as-path [64496 64512 64497]",
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if out.Update != "as-path [64496 64497] remove-private strip" {
		t.Fatalf("update = %q", out.Update)
	}

	out = handleFilterUpdate(&sdk.FilterUpdateInput{Filter: "STRIP", PeerAS: 65001, Update: "origin igp as-path [64496 64497]"})
	if out.Action != sdk.FilterAccept {
		t.Fatalf("action without private ASN = %v, want accept", out.Action)
	}
}

// VALIDATES: raw AS4_PATH private ASNs trigger the reactor directive even when AS_PATH text does not.
// PREVENTS: RFC 6996 AS4_PATH requirement being skipped.
func TestRemovePrivateASFilterUpdateAS4Path(t *testing.T) {
	defs := map[string]*removePrivateASDef{"STRIP": {name: "STRIP", mode: removeModeStrip}}
	defsByName.Store(&defs)

	raw := buildRawPayloadWithAS4Path(4200000000)
	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "STRIP",
		PeerAS: 65001,
		Update: "origin igp as-path 23456",
		Raw:    raw,
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if out.Update != "remove-private strip" {
		t.Fatalf("update = %q", out.Update)
	}
}

func buildRawPayloadWithAS4Path(asn uint32) []byte {
	as4Value := make([]byte, 6)
	as4Value[0] = 2
	as4Value[1] = 1
	binary.BigEndian.PutUint32(as4Value[2:], asn)
	attr := append([]byte{0xC0, 17, byte(len(as4Value))}, as4Value...)
	payload := make([]byte, 4+len(attr))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attr)))
	copy(payload[4:], attr)
	return payload
}
