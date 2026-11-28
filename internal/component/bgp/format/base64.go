// Design: docs/architecture/api/json-format.md -- base64 positional space-delimited UPDATE line.
// Related: text_update.go -- appendMessageTyped.
// Related: text.go -- appendRouterID.

package format

import (
	"encoding/base64"
	"slices"
	"strconv"
	"strings"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Positional flag bitmask constants carried in the `<flags>` field of a
// `base64` `upd` line. Six bits are defined (`0..63`); bits 6 and 7 are
// reserved and always zero.
const (
	base64FlagSent               uint8 = 1 << 0
	base64FlagHasASN4            uint8 = 1 << 1
	base64FlagASN4               uint8 = 1 << 2
	base64FlagHasRemoteAS        uint8 = 1 << 3
	base64FlagAddPathIPv4Unicast uint8 = 1 << 4
	base64FlagAddPathIPv6Unicast uint8 = 1 << 5
)

// base64KindPrefix opens every `base64` UPDATE line: three-byte kind closed by
// a single space so readers can dispatch on one 4-byte load.
const (
	base64KindPrefix  = "upd "
	base64AbsentToken = '-'
)

// appendBase64Update formats a BGP UPDATE into Ze's positional base64 wire
// line with zero key names, zero quotes, zero backslashes, and one trailing
// newline:
//
//	upd <flags> <local-as> <remote-as> <conn-id> <peer> <local> <router-id> <extra-addpath> <n>:<b64-body>\n
//
// Absent metadata tokens are written as `-` and an empty body as `0:` so the
// positional field count never varies. Metadata tokens are short and bounded;
// the base64 body keeps its `<n>:` byte length prefix so readers know the exact
// payload size up to RFC 8654's 65,516-byte limit before reading or allocating.
// Peer addresses reuse PeerInfo's cached strings when present, base64 encodes
// msg.RawBytes directly into buf without intermediate allocations, and
// bgpfilter.ApplyToUpdate is never called.
func appendBase64Update(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, direction rpc.MessageDirection) []byte {
	var flags uint8
	if direction == rpc.DirectionSent {
		flags |= base64FlagSent
	}

	encCtx, known := msg.SourceEncodingContext()
	if !known {
		encCtx = nil
	}
	if encCtx != nil {
		flags |= base64FlagHasASN4
		if encCtx.ASN4() {
			flags |= base64FlagASN4
		}
		if encCtx.AddPathFor(family.IPv4Unicast) {
			flags |= base64FlagAddPathIPv4Unicast
		}
		if encCtx.AddPathFor(family.IPv6Unicast) {
			flags |= base64FlagAddPathIPv6Unicast
		}
	}

	var localAS, remoteAS uint32
	var connID uint64
	if peer != nil {
		localAS = peer.LocalAS
		if asn, hasRemote := peer.PeerAS.ASN(); hasRemote {
			flags |= base64FlagHasRemoteAS
			remoteAS = asn
		}
		connID = peer.ConnectionID
	}

	buf = append(buf, base64KindPrefix...)
	buf = strconv.AppendUint(buf, uint64(flags), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, uint64(localAS), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, uint64(remoteAS), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, connID, 10)
	buf = append(buf, ' ')

	buf = appendBase64PeerAddr(buf, peer)
	buf = append(buf, ' ')
	buf = appendBase64LocalAddr(buf, peer)
	buf = append(buf, ' ')
	buf = appendBase64RouterID(buf, peer)
	buf = append(buf, ' ')
	buf = appendBase64ExtraAddPath(buf, encCtx)
	buf = append(buf, ' ')

	b64Len := base64.StdEncoding.EncodedLen(len(msg.RawBytes))
	buf = strconv.AppendUint(buf, uint64(b64Len), 10)
	buf = append(buf, ':')
	if b64Len > 0 {
		start := len(buf)
		buf = slices.Grow(buf, b64Len+1)
		buf = buf[:start+b64Len]
		base64.StdEncoding.Encode(buf[start:], msg.RawBytes)
	}
	return append(buf, '\n')
}

// appendBase64PeerAddr writes `<peer>` (reusing cached AddressStr when set) or
// `-` when peer or Address is absent.
func appendBase64PeerAddr(buf []byte, peer *plugin.PeerInfo) []byte {
	if peer == nil {
		return append(buf, base64AbsentToken)
	}
	if !peer.Address.IsValid() {
		return append(buf, base64AbsentToken)
	}
	if peer.AddressStr != "" {
		return append(buf, peer.AddressStr...)
	}
	return peer.Address.AppendTo(buf)
}

// appendBase64LocalAddr writes `<local>` (reusing cached LocalAddressStr when
// set) or `-` when peer or LocalAddress is absent.
func appendBase64LocalAddr(buf []byte, peer *plugin.PeerInfo) []byte {
	if peer == nil {
		return append(buf, base64AbsentToken)
	}
	if !peer.LocalAddress.IsValid() {
		return append(buf, base64AbsentToken)
	}
	if peer.LocalAddressStr != "" {
		return append(buf, peer.LocalAddressStr...)
	}
	return peer.LocalAddress.AppendTo(buf)
}

// appendBase64RouterID writes `<router-id>` (this speaker's BGP identifier,
// matching bgp.peer.router-id) or `-` when absent.
func appendBase64RouterID(buf []byte, peer *plugin.PeerInfo) []byte {
	if peer == nil {
		return append(buf, base64AbsentToken)
	}
	if peer.RouterID == 0 {
		return append(buf, base64AbsentToken)
	}
	return appendRouterID(buf, peer.RouterID)
}

// appendBase64ExtraAddPath writes `<csv-families>` for any non-unicast
// address families with ADD-PATH enabled in encCtx, sorted deterministically by
// family name, or `-` when none exist. Zero heap allocations on the common
// path (no extra ADD-PATH families).
func appendBase64ExtraAddPath(buf []byte, encCtx *bgpctx.EncodingContext) []byte {
	if encCtx == nil {
		return append(buf, base64AbsentToken)
	}
	if !encCtx.AnyAddPath() {
		return append(buf, base64AbsentToken)
	}
	enc := encCtx.Encoding()
	if enc == nil {
		return append(buf, base64AbsentToken)
	}
	if len(enc.AddPathMode) == 0 {
		return append(buf, base64AbsentToken)
	}

	var extraArr [16]family.Family
	extra := extraArr[:0]
	for fam := range enc.AddPathMode {
		if fam == family.IPv4Unicast {
			continue
		}
		if fam == family.IPv6Unicast {
			continue
		}
		if !encCtx.AddPathFor(fam) {
			continue
		}
		extra = append(extra, fam)
	}
	if len(extra) == 0 {
		return append(buf, base64AbsentToken)
	}

	slices.SortFunc(extra, compareFamilyNames)

	for i, fam := range extra {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = fam.AppendTo(buf)
	}
	return buf
}

// compareFamilyNames orders two families deterministically by their canonical
// "<afi>/<safi>" spelling, breaking ties by numeric AFI/SAFI order.
func compareFamilyNames(a, b family.Family) int {
	if cmp := strings.Compare(a.String(), b.String()); cmp != 0 {
		return cmp
	}
	if family.FamilyLess(a, b) {
		return -1
	}
	if family.FamilyLess(b, a) {
		return 1
	}
	return 0
}
