//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- complete immutable state scope.
// Related: rfc4301_state_scope_readback_integration_linux_test.go -- raw GETSA and dump verification.
// RFC naming: untagged -- real kernel discrimination records remain owed.

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// rfcScopeLimitPairs makes each selector distinct while retaining a closed IKE
// combination: 64 local UDP services and one exact remote source port. The 65th
// value is reserved for the raw ABI refusal test, never passed to the creator.
func rfcScopeLimitPairs(t *testing.T, count int) []tsPair {
	t.Helper()
	local := mustCIDRNet(t, "10.2.0.0/16")
	remote := mustCIDRNet(t, "10.1.0.0/16")
	pairs := make([]tsPair, count)
	for index := range pairs {
		pairs[index] = tsPair{
			I: tsSelector{Net: local, Proto: sadSelProtoUDP,
				Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: uint16(10000 + index)}},
			R: tsSelector{Net: remote, Proto: sadSelProtoUDP,
				Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelSourcePort}},
		}
	}
	return pairs
}

// The largest accepted scope exceeds the kernel's ordinary small dump buffer.
// Both GETSA replies and the first fresh dump must contain every exact selector
// of each directional state, without retrying a partial or empty first dump.
//
//ze:guest-root
func TestRFC4301MaximumStateScopeSurvivesFirstDump(t *testing.T) {
	newEspPolLab(t)
	espPolSPD(t)
	pairs := rfcScopeLimitPairs(t, rfcScopeSelectorMaximum)
	children, _ := rfcScopeChildren(t, pairs)
	child := children[0]
	// No SAD dump has run in this fresh namespace before these direct GETSA reads.
	rfcScopeWantState(t, child, pairs, true, false)
	rfcScopeWantState(t, child, pairs, false, false)
	// rfcScopeWantState consumes one fresh raw dump and fails if this SPI is omitted.
	rfcScopeWantState(t, child, pairs, true, true)
	rfcScopeWantState(t, child, pairs, false, true)
}

func rfcScopeLimitPayload(t *testing.T, pairs []tsPair) []byte {
	t.Helper()
	value := make([]byte, 8, 8+len(pairs)*nl.SizeofXfrmSelector)
	binary.NativeEndian.PutUint32(value[4:8], uint32(len(pairs)))
	for _, pair := range pairs {
		value = append(value, rfcScopeExpectedSelector(t, pair, true)...)
	}
	return value
}

// rfcScopeLimitRequest uses the existing UAPI structs, not a new Go scope API.
// The valid 64-entry control and invalid 65-entry request differ only in SPI
// and scope payload. No policy is needed to test the kernel's state publication.
func rfcScopeLimitRequest(spi uint32, scope []byte) *nl.NetlinkRequest {
	info := &nl.XfrmUsersaInfo{
		Family: unix.AF_INET, Mode: uint8(netlink.XFRM_MODE_TUNNEL),
		Reqid: defaultReqID, ReplayWindow: 32, Flags: nl.XFRM_STATE_AF_UNSPEC,
		Lft: nl.XfrmLifetimeCfg{
			SoftByteLimit: nl.XFRM_INF, HardByteLimit: nl.XFRM_INF,
			SoftPacketLimit: nl.XFRM_INF, HardPacketLimit: nl.XFRM_INF,
		},
	}
	info.Saddr.FromIP(net.ParseIP(espPolPeerOuter))
	info.Id.Daddr.FromIP(net.ParseIP(espPolZeOuter))
	info.Id.Spi, info.Id.Proto = nl.Swap32(spi), uint8(netlink.XFRM_PROTO_ESP)
	algo := nl.XfrmAlgoAEAD{AlgKeyLen: 36 * 8, AlgICVLen: 128, AlgKey: make([]byte, 36)}
	copy(algo.AlgName[:], "rfc4106(gcm(aes))")
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_NEWSA, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	req.AddData(info)
	req.AddData(nl.NewRtAttr(nl.XFRMA_ALG_AEAD, algo.Serialize()))
	req.AddData(nl.NewRtAttr(rfcScopeAttribute, scope))
	return req
}

// A canonical full 65-selector payload exceeds xfrma_policy's NLA_BINARY bound.
// Linux 7.3 lib/nlattr.c::validate_nla returns ERANGE before state construction.
// A valid 64-selector NEWSA and its readback are controls for every other field.
//
//ze:guest-root
func TestRFC4301StateScopeRejectsSixtyFifthSelectorBeforePublication(t *testing.T) {
	newEspPolLab(t)
	const controlSPI, refusedSPI = 0x6f530064, 0x6f530065
	accepted := rfcScopeLimitPayload(t, rfcScopeLimitPairs(t, rfcScopeSelectorMaximum))
	control := rfcScopeLimitRequest(controlSPI, accepted)
	if _, err := control.Execute(unix.NETLINK_XFRM, 0); err != nil {
		t.Fatalf("install the canonical 64-selector NEWSA control: %v", err)
	}
	before := rfcScopeStateMessages(t, controlSPI, net.ParseIP(espPolZeOuter), false)
	if len(before) != 1 {
		t.Fatalf("64-selector positive control GETSA returned %d states, want one", len(before))
	}
	if len(before[0]) < nl.SizeofXfrmUsersaInfo {
		t.Fatal("the 64-selector control returned no complete state header")
	}
	rejected := rfcScopeLimitPayload(t, rfcScopeLimitPairs(t, rfcScopeSelectorMaximum+1))
	request := rfcScopeLimitRequest(refusedSPI, rejected)
	_, err := request.Execute(unix.NETLINK_XFRM, 0)
	if !errors.Is(err, unix.ERANGE) {
		t.Fatalf("canonical 65-selector NEWSA returned %v, want ERANGE before publication", err)
	}
	missing := nl.NewNetlinkRequest(nl.XFRM_MSG_GETSA, unix.NLM_F_REQUEST)
	id := &nl.XfrmUsersaId{Spi: nl.Swap32(refusedSPI), Family: unix.AF_INET, Proto: uint8(netlink.XFRM_PROTO_ESP)}
	id.Daddr.FromIP(net.ParseIP(espPolZeOuter))
	missing.AddData(id)
	if _, err := missing.Execute(unix.NETLINK_XFRM, nl.XFRM_MSG_NEWSA); !errors.Is(err, unix.ESRCH) {
		t.Fatalf("GETSA for refused SPI %#x returned %v, want ESRCH with no state published", refusedSPI, err)
	}
	after := rfcScopeStateMessages(t, controlSPI, net.ParseIP(espPolZeOuter), false)
	if len(after) != 1 {
		t.Fatalf("64-selector control GETSA returned %d states after refusal, want one", len(after))
	}
	if len(after[0]) < nl.SizeofXfrmUsersaInfo {
		t.Fatal("the existing control lost its complete state header after refusal")
	}
	if !bytes.Equal(before[0][nl.SizeofXfrmUsersaInfo:], after[0][nl.SizeofXfrmUsersaInfo:]) {
		t.Fatal("refusing the 65-selector state changed the existing control's raw attributes")
	}
	actual := rfcScopeAttributeValue(t, after[0][nl.SizeofXfrmUsersaInfo:], rfcScopeAttribute)
	if !bytes.Equal(actual, accepted) {
		t.Fatalf("the valid control lost its full canonical 64-selector scope: got %x, want %x", actual, accepted)
	}
}
