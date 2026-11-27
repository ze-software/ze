// Design: docs/architecture/api/json-format.md -- base64 positional counted UPDATE line.
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
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Positional flag bitmask constants carried in the `<flags>` field of a
// `base64` `upd` line. Six bits are defined (`0..63`); bits 6 and 7 are
// reserved and always zero.
const (
	rawB64FlagSent               uint8 = 1 << 0
	rawB64FlagHasASN4            uint8 = 1 << 1
	rawB64FlagASN4               uint8 = 1 << 2
	rawB64FlagHasRemoteAS        uint8 = 1 << 3
	rawB64FlagAddPathIPv4Unicast uint8 = 1 << 4
	rawB64FlagAddPathIPv6Unicast uint8 = 1 << 5
)

// rawB64KindPrefix opens every `base64` UPDATE line: three-byte kind closed by
// a single space so readers can dispatch on one 4-byte load.
const rawB64KindPrefix = "upd "

// appendRawB64Update formats a BGP UPDATE into Ze's positional counted base64
// wire line with zero key names, zero quotes, zero backslashes, and one
// trailing newline:
//
//	upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>\n
//
// Absent counted fields are written as `0:` so the positional field count never
// varies. Base64 encodes msg.RawBytes directly into buf without intermediate
// allocations, and bgpfilter.ApplyToUpdate is never called.
func appendRawB64Update(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, direction rpc.MessageDirection) []byte {
	var flags uint8
	if direction == rpc.DirectionSent {
		flags |= rawB64FlagSent
	}

	encCtx, known := msg.SourceEncodingContext()
	if !known {
		encCtx = nil
	}
	if encCtx != nil {
		flags |= rawB64FlagHasASN4
		if encCtx.ASN4() {
			flags |= rawB64FlagASN4
		}
		if encCtx.AddPathFor(family.IPv4Unicast) {
			flags |= rawB64FlagAddPathIPv4Unicast
		}
		if encCtx.AddPathFor(family.IPv6Unicast) {
			flags |= rawB64FlagAddPathIPv6Unicast
		}
	}

	var localAS, remoteAS uint32
	var connID uint64
	if peer != nil {
		localAS = peer.LocalAS
		if asn, hasRemote := peer.PeerAS.ASN(); hasRemote {
			flags |= rawB64FlagHasRemoteAS
			remoteAS = asn
		}
		connID = peer.ConnectionID
	}

	buf = append(buf, rawB64KindPrefix...)
	buf = strconv.AppendUint(buf, uint64(flags), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, uint64(localAS), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, uint64(remoteAS), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, connID, 10)
	buf = append(buf, ' ')

	buf = appendRawB64PeerAddr(buf, peer)
	buf = append(buf, ' ')
	buf = appendRawB64LocalAddr(buf, peer)
	buf = append(buf, ' ')
	buf = appendRawB64RouterID(buf, peer)
	buf = append(buf, ' ')
	buf = appendRawB64ExtraAddPath(buf, encCtx)
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

// appendRawB64PeerAddr writes `<n>:<peer>` or `0:` when peer or Address is absent.
func appendRawB64PeerAddr(buf []byte, peer *plugin.PeerInfo) []byte {
	if peer == nil {
		return append(buf, '0', ':')
	}
	if !peer.Address.IsValid() {
		return append(buf, '0', ':')
	}
	var addrBuf [64]byte
	return textbuf.CountedBytes(buf, peer.Address.AppendTo(addrBuf[:0]))
}

// appendRawB64LocalAddr writes `<n>:<local>` or `0:` when peer or LocalAddress is absent.
func appendRawB64LocalAddr(buf []byte, peer *plugin.PeerInfo) []byte {
	if peer == nil {
		return append(buf, '0', ':')
	}
	if !peer.LocalAddress.IsValid() {
		return append(buf, '0', ':')
	}
	var addrBuf [64]byte
	return textbuf.CountedBytes(buf, peer.LocalAddress.AppendTo(addrBuf[:0]))
}

// appendRawB64RouterID writes `<n>:<router-id>` (this speaker's BGP identifier,
// matching bgp.peer.router-id) or `0:` when absent.
func appendRawB64RouterID(buf []byte, peer *plugin.PeerInfo) []byte {
	if peer == nil {
		return append(buf, '0', ':')
	}
	if peer.RouterID == 0 {
		return append(buf, '0', ':')
	}
	var ridBuf [16]byte
	return textbuf.CountedBytes(buf, appendRouterID(ridBuf[:0], peer.RouterID))
}

// appendRawB64ExtraAddPath writes `<n>:<csv-families>` for any non-unicast
// address families with ADD-PATH enabled in encCtx, sorted deterministically by
// family name, or `0:` when none exist. Zero heap allocations on the common
// path (no extra ADD-PATH families).
func appendRawB64ExtraAddPath(buf []byte, encCtx *bgpctx.EncodingContext) []byte {
	if encCtx == nil {
		return append(buf, '0', ':')
	}
	if !encCtx.AnyAddPath() {
		return append(buf, '0', ':')
	}
	enc := encCtx.Encoding()
	if enc == nil {
		return append(buf, '0', ':')
	}
	if len(enc.AddPathMode) == 0 {
		return append(buf, '0', ':')
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
		return append(buf, '0', ':')
	}

	slices.SortFunc(extra, compareFamilyNames)

	var textArr [256]byte
	text := textArr[:0]
	for i, fam := range extra {
		if i > 0 {
			text = append(text, ',')
		}
		text = fam.AppendTo(text)
	}
	return textbuf.CountedBytes(buf, text)
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
