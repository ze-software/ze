// Design: docs/architecture/exabgp-bridge.md -- positional counted base64 UPDATE parser and wire-backed renderer.
// Related: bridge_raw_b64_stub.go -- non-ze_bgp build stub.
// Related: bridge_wire_json.go -- existing ze_bgp wire-to-ExaBGP helper.
// Related: bridge_negotiated.go -- Sessions.noteRaw session state and cached negotiated JSON bytes.

//go:build ze_bgp

package bridge

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rawWireBuildEnabled records whether this build carries the BGP wire engine.
const rawWireBuildEnabled = true

// Positional flag bitmask constants matching internal/component/bgp/format/raw_b64.go.
// Six bits are defined (0..63); bits 6 and 7 are reserved and rejected if set.
const (
	rawB64FlagSent               uint8 = 1 << 0
	rawB64FlagHasASN4            uint8 = 1 << 1
	rawB64FlagASN4               uint8 = 1 << 2
	rawB64FlagHasRemoteAS        uint8 = 1 << 3
	rawB64FlagAddPathIPv4Unicast uint8 = 1 << 4
	rawB64FlagAddPathIPv6Unicast uint8 = 1 << 5
	rawB64FlagsMax               uint8 = (1 << 6) - 1
)

// Positional grammar constants and RFC 8654 wire bounds for `base64` UPDATE lines.
const (
	rawB64KindPrefix   = "upd "
	rawUint64DigitsMax = 20
	// rawUpdateBodyMax is RFC 8654 Section 3's extended BGP message maximum
	// (65,535 octets) minus the 19-octet BGP message header.
	rawUpdateBodyMax = 65535 - 19
	// rawB64SrcStackMax is base64.StdEncoding.EncodedLen(4096), keeping standard
	// RFC 4271 UPDATE base64 decoding on the stack with zero heap allocations.
	rawB64SrcStackMax = 5464
	// rawContextCacheMax caps cached (asn4, addPath) -> ContextID entries.
	rawContextCacheMax = 1024
)

var (
	errRawB64CarriageReturn = errors.New("exabgp-bridge: base64 line contains carriage return")
	errRawB64InvalidPrefix  = errors.New("exabgp-bridge: base64 line does not start with 'upd '")
	errRawB64EmptyNumber    = errors.New("exabgp-bridge: base64 numeric field has no digits")
	errRawB64MissingSpace   = errors.New("exabgp-bridge: base64 field missing terminating space")
	errRawB64InvalidDigit   = errors.New("exabgp-bridge: base64 numeric field contains non-digit")
	errRawB64NumberOverflow = errors.New("exabgp-bridge: base64 numeric field overflows uint64")
	errRawB64UnknownFlags   = errors.New("exabgp-bridge: base64 flags has unknown high bits (> 63)")
	errRawB64ASNOverflow    = errors.New("exabgp-bridge: base64 AS number exceeds uint32")
	errRawB64MissingColon   = errors.New("exabgp-bridge: base64 counted field missing ':'")
	errRawB64EmptyCount     = errors.New("exabgp-bridge: base64 counted field length has no digits")
	errRawB64OutOfBounds    = errors.New("exabgp-bridge: base64 counted field length exceeds remaining line")
	errRawB64TrailingBytes  = errors.New("exabgp-bridge: base64 line has unexpected trailing bytes")
	errRawB64BodyTooLarge   = errors.New("exabgp-bridge: base64 UPDATE body exceeds RFC 8654 maximum (65516 bytes)")
	errRawB64InvalidBase64  = errors.New("exabgp-bridge: base64 UPDATE body is not valid base64")
)

// RawUpdate holds the parsed positional fields and decoded BGP UPDATE wire
// body from one `base64` `upd` event line.
type RawUpdate struct {
	Peer         string
	Local        string
	RouterID     string
	ExtraAddPath string
	ConnID       uint64
	LocalAS      uint32
	RemoteAS     uint32
	Flags        uint8
	Body         []byte
}

// Direction reports the ExaBGP event direction ("receive" or "send") derived
// from rawB64FlagSent.
func (r RawUpdate) Direction() string {
	if r.isSent() {
		return modeSend
	}
	return modeReceive
}

// isSent reports whether this UPDATE was sent by Ze rather than received.
func (r RawUpdate) isSent() bool {
	return r.Flags&rawB64FlagSent != 0
}

// hasASN4 reports whether the UPDATE carried an explicit negotiated ASN4 bit.
func (r RawUpdate) hasASN4() bool {
	return r.Flags&rawB64FlagHasASN4 != 0
}

// asn4 reports the 4-byte ASN width bit (valid when hasASN4 is true).
func (r RawUpdate) asn4() bool {
	return r.Flags&rawB64FlagASN4 != 0
}

// hasRemoteAS reports whether the peer's remote AS was known (false for dynamic
// peers with `peer-as auto` before OPEN negotiation).
func (r RawUpdate) hasRemoteAS() bool {
	return r.Flags&rawB64FlagHasRemoteAS != 0
}

// addPathIPv4Unicast reports whether IPv4 unicast ADD-PATH is active for this
// UPDATE's direction.
func (r RawUpdate) addPathIPv4Unicast() bool {
	return r.Flags&rawB64FlagAddPathIPv4Unicast != 0
}

// addPathIPv6Unicast reports whether IPv6 unicast ADD-PATH is active for this
// UPDATE's direction.
func (r RawUpdate) addPathIPv6Unicast() bool {
	return r.Flags&rawB64FlagAddPathIPv6Unicast != 0
}

// rawContextKey identifies one encoding context shape decoded from a base64
// UPDATE's flags and extra-addpath field.
type rawContextKey struct {
	asn4         bool
	ipv4AP       bool
	ipv6AP       bool
	extraAddPath string
}

// rawContextCache bounds and caches resolved (asn4, addPath) -> ContextID
// mappings so RenderRawUpdate never calls bgpctx.Registry.Register per UPDATE.
// Safe for concurrent use.
type rawContextCache struct {
	mu      sync.RWMutex
	entries map[rawContextKey]bgpctx.ContextID
}

var sharedRawContextCache rawContextCache

// SupportsRawWire reports whether this binary was compiled with the BGP engine
// (`ze_bgp`) and can decode `base64` UPDATE lines directly from wire bytes.
func SupportsRawWire() bool {
	return rawWireBuildEnabled
}

// ReadRawB64Update parses one positional counted `upd` line and base64-decodes
// its UPDATE body into scratch (spilling to a fresh slice for jumbo RFC 8654
// UPDATEs larger than cap(scratch)).
//
// Wire grammar (internal/component/bgp/format/raw_b64.go):
//
//	upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>[\n]
//
// Returns an error without panicking on any malformed input or body past RFC
// 8654's 65,516-byte limit.
func ReadRawB64Update(line string, scratch []byte) (RawUpdate, error) {
	if line != "" && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if line != "" && line[len(line)-1] == '\r' {
		return RawUpdate{}, errRawB64CarriageReturn
	}
	if len(line) < len(rawB64KindPrefix) {
		return RawUpdate{}, errRawB64InvalidPrefix
	}
	if line[:len(rawB64KindPrefix)] != rawB64KindPrefix {
		return RawUpdate{}, errRawB64InvalidPrefix
	}
	rest := line[len(rawB64KindPrefix):]

	flagsVal, rest, err := cutRawUint64(rest)
	if err != nil {
		return RawUpdate{}, err
	}
	if flagsVal > uint64(rawB64FlagsMax) {
		return RawUpdate{}, errRawB64UnknownFlags
	}

	localASVal, rest, err := cutRawUint64(rest)
	if err != nil {
		return RawUpdate{}, err
	}
	if localASVal > math.MaxUint32 {
		return RawUpdate{}, errRawB64ASNOverflow
	}

	remoteASVal, rest, err := cutRawUint64(rest)
	if err != nil {
		return RawUpdate{}, err
	}
	if remoteASVal > math.MaxUint32 {
		return RawUpdate{}, errRawB64ASNOverflow
	}

	connID, rest, err := cutRawUint64(rest)
	if err != nil {
		return RawUpdate{}, err
	}

	peer, rest, err := cutRawCountedText(rest, true)
	if err != nil {
		return RawUpdate{}, err
	}
	local, rest, err := cutRawCountedText(rest, true)
	if err != nil {
		return RawUpdate{}, err
	}
	routerID, rest, err := cutRawCountedText(rest, true)
	if err != nil {
		return RawUpdate{}, err
	}
	extraAddPath, rest, err := cutRawCountedText(rest, true)
	if err != nil {
		return RawUpdate{}, err
	}
	b64Body, rest, err := cutRawCountedText(rest, false)
	if err != nil {
		return RawUpdate{}, err
	}
	if rest != "" {
		return RawUpdate{}, errRawB64TrailingBytes
	}

	body, err := decodeRawB64Body(b64Body, scratch)
	if err != nil {
		return RawUpdate{}, err
	}

	flags := uint8(flagsVal)

	return RawUpdate{
		Peer:         peer,
		Local:        local,
		RouterID:     routerID,
		ExtraAddPath: extraAddPath,
		ConnID:       connID,
		LocalAS:      uint32(localASVal),
		RemoteAS:     uint32(remoteASVal),
		Flags:        flags,
		Body:         body,
	}, nil
}

// cutRawUint64 parses one decimal unsigned integer closed by a single space in
// a single pass without allocating.
func cutRawUint64(s string) (val uint64, rest string, err error) {
	for i := range len(s) {
		c := s[i]
		if c == ' ' {
			if i == 0 {
				return 0, "", errRawB64EmptyNumber
			}
			return val, s[i+1:], nil
		}
		if c < '0' {
			return 0, "", errRawB64InvalidDigit
		}
		if c > '9' {
			return 0, "", errRawB64InvalidDigit
		}
		if i >= rawUint64DigitsMax {
			return 0, "", errRawB64NumberOverflow
		}
		digit := uint64(c - '0')
		if val > (math.MaxUint64-digit)/10 {
			return 0, "", errRawB64NumberOverflow
		}
		val = val*10 + digit
	}
	return 0, "", errRawB64MissingSpace
}

// cutRawCountedText parses one `<byte-len>:<bytes>` field without allocating,
// verifying bounds against remaining input before slicing.
func cutRawCountedText(s string, expectSpace bool) (val, rest string, err error) {
	var size uint64
	for i := range len(s) {
		c := s[i]
		if c == ':' {
			if i == 0 {
				return "", "", errRawB64EmptyCount
			}
			afterColon := s[i+1:]
			if size > uint64(len(afterColon)) {
				return "", "", errRawB64OutOfBounds
			}
			val = afterColon[:size]
			rem := afterColon[size:]
			if !expectSpace {
				return val, rem, nil
			}
			if rem == "" {
				return "", "", errRawB64MissingSpace
			}
			if rem[0] != ' ' {
				return "", "", errRawB64MissingSpace
			}
			return val, rem[1:], nil
		}
		if c < '0' {
			return "", "", errRawB64InvalidDigit
		}
		if c > '9' {
			return "", "", errRawB64InvalidDigit
		}
		if i >= rawUint64DigitsMax {
			return "", "", errRawB64NumberOverflow
		}
		digit := uint64(c - '0')
		if size > (math.MaxUint64-digit)/10 {
			return "", "", errRawB64NumberOverflow
		}
		size = size*10 + digit
	}
	return "", "", errRawB64MissingColon
}

// decodeRawB64Body decodes a base64-encoded BGP UPDATE body into scratch when
// it fits, or into a fresh slice up to rawUpdateBodyMax (RFC 8654).
func decodeRawB64Body(b64Body string, scratch []byte) ([]byte, error) {
	if b64Body == "" {
		return nil, nil //nolint:nilnil // empty base64 body is a valid zero-byte payload
	}
	if len(b64Body) > base64.StdEncoding.EncodedLen(rawUpdateBodyMax) {
		return nil, errRawB64BodyTooLarge
	}
	if strings.IndexByte(b64Body, '\r') >= 0 {
		return nil, errRawB64InvalidBase64
	}
	if strings.IndexByte(b64Body, '\n') >= 0 {
		return nil, errRawB64InvalidBase64
	}

	maxDecoded := base64.StdEncoding.DecodedLen(len(b64Body))
	var dst []byte
	if maxDecoded <= cap(scratch) {
		dst = scratch[:maxDecoded]
	} else {
		dst = make([]byte, maxDecoded)
	}

	var srcStack [rawB64SrcStackMax]byte
	var src []byte
	if len(b64Body) <= len(srcStack) {
		copy(srcStack[:], b64Body)
		src = srcStack[:len(b64Body)]
	} else {
		src = []byte(b64Body)
	}

	n, err := base64.StdEncoding.Decode(dst, src)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRawB64InvalidBase64, err)
	}
	if n > rawUpdateBodyMax {
		return nil, errRawB64BodyTooLarge
	}
	return dst[:n], nil
}

// resolveRawContextID returns a cached bgpctx.ContextID for the given ASN4 and
// ADD-PATH configuration, registering at most once per distinct key up to
// rawContextCacheMax cached entries.
func resolveRawContextID(asn4, ipv4AP, ipv6AP bool, extraAddPath string) (bgpctx.ContextID, error) {
	if asn4 && !ipv4AP && !ipv6AP && extraAddPath == "" {
		return bgpctx.APIContextID, nil
	}
	key := rawContextKey{
		asn4:         asn4,
		ipv4AP:       ipv4AP,
		ipv6AP:       ipv6AP,
		extraAddPath: extraAddPath,
	}
	return sharedRawContextCache.getOrRegister(key)
}

func (c *rawContextCache) getOrRegister(key rawContextKey) (bgpctx.ContextID, error) {
	c.mu.RLock()
	id, ok := c.entries[key]
	c.mu.RUnlock()
	if ok {
		return id, nil
	}

	ec := buildRawEncodingContext(key)
	registered, err := bgpctx.Registry.Register(ec)
	if err != nil {
		return 0, fmt.Errorf("exabgp-bridge: register base64 encoding context: %w", err)
	}

	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[rawContextKey]bgpctx.ContextID)
	}
	if existing, found := c.entries[key]; found {
		c.mu.Unlock()
		return existing, nil
	}
	if len(c.entries) < rawContextCacheMax {
		c.entries[key] = registered
	}
	c.mu.Unlock()
	return registered, nil
}

// buildRawEncodingContext constructs an EncodingContext carrying the ASN4 width
// and per-family ADD-PATH flags extracted from a base64 UPDATE line.
func buildRawEncodingContext(key rawContextKey) *bgpctx.EncodingContext {
	if !key.ipv4AP && !key.ipv6AP && key.extraAddPath == "" {
		return bgpctx.EncodingContextForASN4(key.asn4)
	}
	addPath := make(map[family.Family]bool)
	if key.ipv4AP {
		addPath[family.IPv4Unicast] = true
	}
	if key.ipv6AP {
		addPath[family.IPv6Unicast] = true
	}
	for rest := key.extraAddPath; rest != ""; {
		token, next, _ := strings.Cut(rest, ",")
		rest = next
		if token == "" {
			continue
		}
		fam, ok := parseRawAddPathFamily(token)
		if !ok {
			bridgeLog().Warn("exabgp-bridge: ignoring unknown extra-addpath family in base64 update", "family", token)
			continue
		}
		addPath[fam] = true
	}
	return bgpctx.EncodingContextWithAddPath(key.asn4, addPath)
}

// parseRawAddPathFamily resolves a canonical `<afi>/<safi>` family token (or
// numeric fallback `afi-<N>/safi-<M>`) emitted by appendRawB64ExtraAddPath.
func parseRawAddPathFamily(token string) (family.Family, bool) {
	if fam, ok := family.LookupFamily(token); ok {
		return fam, true
	}
	afiPart, safiPart, split := strings.Cut(token, "/")
	if !split {
		return family.Family{}, false
	}
	afi, ok := parseRawAFIToken(afiPart)
	if !ok {
		return family.Family{}, false
	}
	safi, ok := parseRawSAFIToken(safiPart)
	if !ok {
		return family.Family{}, false
	}
	return family.Family{AFI: afi, SAFI: safi}, true
}

func parseRawAFIToken(token string) (family.AFI, bool) {
	if afi, ok := family.LookupAFI(token); ok {
		return afi, true
	}
	numStr, hasPrefix := strings.CutPrefix(token, "afi-")
	if !hasPrefix {
		return 0, false
	}
	n, err := strconv.ParseUint(numStr, 10, 16)
	if err != nil {
		return 0, false
	}
	return family.AFI(n), true
}

func parseRawSAFIToken(token string) (family.SAFI, bool) {
	if safi, ok := family.LookupSAFI(token); ok {
		return safi, true
	}
	numStr, hasPrefix := strings.CutPrefix(token, "safi-")
	if !hasPrefix {
		return 0, false
	}
	n, err := strconv.ParseUint(numStr, 10, 8)
	if err != nil {
		return 0, false
	}
	return family.SAFI(n), true
}

// RenderRawUpdate renders one parsed base64 UPDATE for a script format and
// form set, consulting s for cached session state and negotiated bytes.
func (s *Sessions) RenderRawUpdate(raw RawUpdate, outFormat Format, forms Forms, grantNegotiated bool, log *slog.Logger) []byte {
	if len(raw.Body) > rawUpdateBodyMax {
		if log == nil {
			log = bridgeLog()
		}
		log.Warn("exabgp-bridge: base64 update body exceeds RFC 8654 maximum", "peer", raw.Peer, "bytes", len(raw.Body))
		return nil
	}
	if direct, ok := s.renderRawUpdateDirect(raw, outFormat, forms, grantNegotiated, log); ok {
		return direct
	}
	return s.renderRawUpdateFallback(raw, outFormat, forms, grantNegotiated, log)
}

// renderRawUpdateFallback decodes one base64 UPDATE via wireu.WireUpdate and
// format.AppendMessage/AppendSentMessage into a wire-backed Event, preserving
// full parity across all address families, EORs, error warnings, and forms.
func (s *Sessions) renderRawUpdateFallback(raw RawUpdate, outFormat Format, forms Forms, grantNegotiated bool, log *slog.Logger) []byte {
	if log == nil {
		log = bridgeLog()
	}

	negotiated, _, sessionASN4 := s.noteRaw(raw.Peer, raw.ConnID, raw.hasASN4(), raw.asn4())
	asn4 := sessionASN4
	if raw.hasASN4() {
		asn4 = raw.asn4()
	}

	contextID, err := resolveRawContextID(asn4, raw.addPathIPv4Unicast(), raw.addPathIPv6Unicast(), raw.ExtraAddPath)
	// refuse-by-default: guards RenderRawUpdate -- nil tells Fleet no line was produced for this update.
	if err != nil {
		log.Warn("exabgp-bridge: resolve encoding context failed for base64 update", "peer", raw.Peer, "error", err)
		return nil
	}

	peer := buildRawPeerInfo(raw)

	var wire wireu.WireUpdate
	wireu.InitWireUpdate(&wire, raw.Body, contextID)
	attrs, attrErr := wire.Attrs()
	if attrErr != nil {
		log.Warn("exabgp-bridge: malformed base64 update attributes", "peer", raw.Peer, "error", attrErr)
		attrs = nil
	}

	dir := rpc.DirectionReceived
	if raw.isSent() {
		dir = rpc.DirectionSent
	}
	message := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   raw.Body,
		Direction:  dir,
		WireUpdate: &wire,
		AttrsWire:  attrs,
	}

	var scratch [4096]byte
	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}
	var rendered []byte
	if raw.isSent() {
		rendered = bgpformat.AppendSentMessage(scratch[:0], peer, message, content)
	} else {
		rendered = bgpformat.AppendMessage(scratch[:0], peer, message, content)
	}

	var zebgp map[string]any
	// refuse-by-default: guards RenderRawUpdate -- nil tells Fleet no line was produced for this update.
	if err := json.Unmarshal(rendered, &zebgp); err != nil {
		log.Warn("exabgp-bridge: failed to decode rendered base64 update JSON", "peer", raw.Peer, "error", err)
		return nil
	}

	event := ReadEvent(zebgp)
	event.Peer = raw.Peer
	event.Local = raw.Local
	event.RouterID = raw.RouterID
	event.Packet = Packet{
		code: packetCodes[msgTypeUpdate],
		body: hex.EncodeToString(raw.Body),
	}
	event.attrContext = contextID
	event.Negotiated = negotiated
	event = event.ForGrant(grantNegotiated)
	return event.Lines(outFormat, forms, log)
}

// buildRawPeerInfo constructs the PeerInfo fed to bgpformat.AppendMessage from
// one base64 UPDATE's positional session fields.
func buildRawPeerInfo(raw RawUpdate) *plugin.PeerInfo {
	peer := &plugin.PeerInfo{
		LocalAS:      raw.LocalAS,
		ConnectionID: raw.ConnID,
	}
	if raw.Peer != "" {
		if addr, err := netip.ParseAddr(raw.Peer); err == nil {
			peer.Address = addr
			peer.AddressStr = raw.Peer
		}
	}
	if raw.Local != "" {
		if addr, err := netip.ParseAddr(raw.Local); err == nil {
			peer.LocalAddress = addr
			peer.LocalAddressStr = raw.Local
		}
	}
	if raw.hasRemoteAS() {
		peer.PeerAS = rpc.RemoteASOf(raw.RemoteAS)
	}
	if raw.RouterID != "" {
		if addr, err := netip.ParseAddr(raw.RouterID); err == nil && addr.Is4() {
			b4 := addr.As4()
			peer.RouterID = binary.BigEndian.Uint32(b4[:])
		}
	}
	return peer
}

// reconnectedRawSession reports whether a base64 UPDATE's numeric connection
// identifier names a different physical connection than entry. A zero connID
// or an empty cached connection-id does not trigger invalidation so synthetic
// events without connection-id keep working (AC-8).
func reconnectedRawSession(entry peerSession, connID uint64) bool {
	if connID == 0 {
		return false
	}
	if entry.connID == "" {
		return false
	}
	return entry.connIDNum != connID
}

// noteRaw applies Q-E4 reconnect invalidation for a base64 UPDATE and answers
// the peer's cached negotiated block, its pre-serialized ExaBGP-spaced JSON
// bytes, and the session's effective 4-byte ASN width.
//
// When connID is non-zero and disagrees with the cached entry's physical
// connection-id, the stale cached block and JSON bytes are deleted immediately
// even if bgp:state was excluded from receive > allow. When connID is zero, an
// existing cached entry is preserved for backward-compatible synthetic events.
// When no cached entry exists, noteRaw returns unexchangedNegotiated() and its
// pre-serialized JSON bytes. Safe for concurrent use.
func (s *Sessions) noteRaw(peer string, connID uint64, hasASN4, asn4 bool) (negotiated map[string]any, negotiatedJSON []byte, sessionASN4 bool) {
	s.mu.Lock()
	entry, ok := s.peers[peer]
	if ok && reconnectedRawSession(entry, connID) {
		delete(s.peers, peer)
		ok = false
	}
	s.mu.Unlock()

	sessionASN4 = true
	if hasASN4 {
		sessionASN4 = asn4
	} else if ok && entry.hasASN4 {
		sessionASN4 = entry.asn4
	}

	if !ok {
		return unexchangedNegotiated(), unexchangedNegotiatedJSONBytes(), sessionASN4
	}
	return entry.block, entry.blockJSON, sessionASN4
}

var (
	unexchangedNegotiatedJSONOnce sync.Once
	unexchangedNegotiatedJSON     []byte
)

// unexchangedNegotiatedJSONBytes returns the pre-serialized ExaBGP-spaced JSON
// bytes of unexchangedNegotiated(), computed once per process lifetime.
func unexchangedNegotiatedJSONBytes() []byte {
	unexchangedNegotiatedJSONOnce.Do(func() {
		encoded, err := marshalExabgpJSON(unexchangedNegotiated())
		if err != nil {
			panic("BUG: exabgp-bridge: marshal unexchanged negotiated block: " + err.Error())
		}
		unexchangedNegotiatedJSON = encoded
	})
	return unexchangedNegotiatedJSON
}
