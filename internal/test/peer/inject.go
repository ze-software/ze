// Design: docs/architecture/testing/ci-format.md — stress injection from ze-test peer
// Overview: peer.go — test peer runtime and mode dispatch
// Related: message.go — BGP wire constants and KEEPALIVE builder reused here
//
// Inject mode streams a fully-formed BGP UPDATE byte image (built in RAM,
// sized up-front from prefix count + family) over an established session.
// Replaces the previous Python+scapy / bngblaster replay pipeline with a
// single in-process generator. Used by test/stress/scenarios.
//
// Reference: RFC 4271 §4.3 (UPDATE), RFC 4760 §3 (MP_REACH_NLRI),
// RFC 6793 (4-byte ASN AS_PATH encoding).

package peer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

const (
	bgpMaxMsgLen = 4096 // RFC 4271 §4
	bgpEORLen    = 23   // empty UPDATE: marker(16)+len(2)+type(1)+wdr(2)+attr(2)
	// Path attribute flag bytes used here.
	flagWKTrans       = 0x40 // well-known, transitive
	flagOptNonTransEx = 0x90 // optional, non-transitive, extended length

	keepaliveInterval = 30 * time.Second
)

// InjectSpec describes the UPDATE stream a ze-test peer in ModeInject
// emits after the OPEN handshake.
type InjectSpec struct {
	// Prefix is the base network (e.g. 10.0.0.0/24 or 2001:db8::/48).
	// All generated prefixes are sequential from this base at the same length.
	Prefix netip.Prefix
	// Count is how many prefixes to emit.
	Count int
	// NextHop must match Prefix's family.
	NextHop netip.Addr
	// ASN is used for a single-segment AS_SEQUENCE (4-byte encoding).
	ASN uint32
	// EndOfRIB appends an empty UPDATE after the last message. Default true.
	EndOfRIB bool
	// Dwell keeps the session open this long after the last byte is written,
	// sending KEEPALIVE every 30s. 0 = hold until ctx.Done or peer hangs up.
	Dwell time.Duration
}

// BuildUpdates constructs the complete byte image for the inject stream.
// A single slice is allocated up-front at the exact final size; the hot
// loop performs offset writes only. Returns the bytes and the message
// count (UPDATEs + optional EOR) for telemetry.
func BuildUpdates(spec InjectSpec) ([]byte, int, error) {
	if spec.Count < 0 {
		return nil, 0, errors.New("count must be >= 0")
	}
	if !spec.Prefix.IsValid() {
		return nil, 0, errors.New("invalid prefix")
	}
	if !spec.NextHop.IsValid() {
		return nil, 0, errors.New("invalid next hop")
	}
	if spec.Prefix.Addr().Is4() != spec.NextHop.Is4() {
		return nil, 0, errors.New("prefix and next-hop family mismatch")
	}
	if spec.Prefix.Addr().Is4() {
		return buildV4Unicast(spec)
	}
	return buildV6Unicast(spec)
}

// buildV4Unicast writes ORIGIN + AS_PATH + NEXT_HOP + inline NLRI UPDATEs.
func buildV4Unicast(spec InjectSpec) ([]byte, int, error) {
	plen := spec.Prefix.Bits()
	if plen < 0 || plen > 32 {
		return nil, 0, fmt.Errorf("invalid IPv4 prefix length %d", plen)
	}
	plBytes := (plen + 7) / 8
	stride := 1 + plBytes
	// Path attrs: ORIGIN(4) + AS_PATH(5+4) + NEXT_HOP(3+4) = 20 bytes.
	const attrsLen = 4 + 9 + 7
	budget := bgpMaxMsgLen - HeaderLen - 2 - 2 - attrsLen
	nlriPer := budget / stride
	if nlriPer <= 0 {
		return nil, 0, errors.New("attrs exceed BGP max message size")
	}

	fullMsgs := spec.Count / nlriPer
	rem := spec.Count - fullMsgs*nlriPer
	totalMsgs := fullMsgs
	fullMsgLen := HeaderLen + 2 + 2 + attrsLen + nlriPer*stride
	partialMsgLen := 0
	if rem > 0 {
		partialMsgLen = HeaderLen + 2 + 2 + attrsLen + rem*stride
		totalMsgs++
	}
	eorLen := 0
	if spec.EndOfRIB {
		eorLen = bgpEORLen
		totalMsgs++
	}
	total := fullMsgs*fullMsgLen + partialMsgLen + eorLen
	buf := make([]byte, total)

	nh := spec.NextHop.As4()
	baseAddr := spec.Prefix.Addr().As4()
	baseInt := binary.BigEndian.Uint32(baseAddr[:])
	step := uint32(1)
	if plen < 32 {
		step = uint32(1) << (32 - plen)
	}

	off := 0
	written := 0
	writePrefix := func(at, msgLen int) int {
		copy(buf[at:at+16], Marker)
		//nolint:gosec // msgLen bounded by bgpMaxMsgLen (4096)
		binary.BigEndian.PutUint16(buf[at+16:at+18], uint16(msgLen))
		buf[at+18] = MsgUPDATE
		buf[at+19], buf[at+20] = 0, 0 // withdrawn routes length
		binary.BigEndian.PutUint16(buf[at+21:at+23], attrsLen)
		// ORIGIN IGP
		buf[at+23], buf[at+24], buf[at+25], buf[at+26] = flagWKTrans, 0x01, 0x01, 0x00
		// AS_PATH: AS_SEQUENCE, one 4-byte ASN
		buf[at+27], buf[at+28], buf[at+29] = flagWKTrans, 0x02, 0x06
		buf[at+30], buf[at+31] = 0x02, 0x01
		binary.BigEndian.PutUint32(buf[at+32:at+36], spec.ASN)
		// NEXT_HOP
		buf[at+36], buf[at+37], buf[at+38] = flagWKTrans, 0x03, 0x04
		copy(buf[at+39:at+43], nh[:])
		return at + 43 // NLRI starts here
	}
	writeNLRI := func(at, n int) {
		var tmp [4]byte
		for j := range n {
			//nolint:gosec // address wraps intentionally if count overflows /0 space
			addr := baseInt + uint32(written+j)*step
			buf[at] = byte(plen)
			binary.BigEndian.PutUint32(tmp[:], addr)
			copy(buf[at+1:at+stride], tmp[:plBytes])
			at += stride
		}
	}

	for range fullMsgs {
		nlriAt := writePrefix(off, fullMsgLen)
		writeNLRI(nlriAt, nlriPer)
		off += fullMsgLen
		written += nlriPer
	}
	if rem > 0 {
		nlriAt := writePrefix(off, partialMsgLen)
		writeNLRI(nlriAt, rem)
		off += partialMsgLen
	}
	if spec.EndOfRIB {
		copy(buf[off:off+16], Marker)
		binary.BigEndian.PutUint16(buf[off+16:off+18], bgpEORLen)
		buf[off+18] = MsgUPDATE
		// remaining 4 bytes (wdr_len + attr_len) already zero from make.
	}
	return buf, totalMsgs, nil
}

// buildV6Unicast writes ORIGIN + AS_PATH + MP_REACH_NLRI (RFC 4760) UPDATEs.
// MP_REACH always uses extended-length (flag 0x90) since its value routinely
// exceeds 255 bytes.
func buildV6Unicast(spec InjectSpec) ([]byte, int, error) {
	plen := spec.Prefix.Bits()
	if plen < 0 || plen > 128 {
		return nil, 0, fmt.Errorf("invalid IPv6 prefix length %d", plen)
	}
	plBytes := (plen + 7) / 8
	stride := 1 + plBytes
	// Base attrs (ORIGIN + AS_PATH) = 4 + 9 = 13.
	const baseAttrsLen = 4 + 9
	// MP_REACH fixed part: attr flags(1)+type(1)+extlen(2)+AFI(2)+SAFI(1)+nh_len(1)+nh(16)+reserved(1) = 25.
	const mpFixed = 25
	budget := bgpMaxMsgLen - HeaderLen - 2 - 2 - baseAttrsLen - mpFixed
	nlriPer := budget / stride
	if nlriPer <= 0 {
		return nil, 0, errors.New("attrs exceed BGP max message size")
	}

	fullMsgs := spec.Count / nlriPer
	rem := spec.Count - fullMsgs*nlriPer
	totalMsgs := fullMsgs
	fullMsgLen := HeaderLen + 2 + 2 + baseAttrsLen + mpFixed + nlriPer*stride
	partialMsgLen := 0
	if rem > 0 {
		partialMsgLen = HeaderLen + 2 + 2 + baseAttrsLen + mpFixed + rem*stride
		totalMsgs++
	}
	eorLen := 0
	if spec.EndOfRIB {
		eorLen = bgpEORLen
		totalMsgs++
	}
	total := fullMsgs*fullMsgLen + partialMsgLen + eorLen
	buf := make([]byte, total)

	nh := spec.NextHop.As16()
	baseAddr := spec.Prefix.Addr().As16()
	baseBig := bigUint128(baseAddr[:])
	stepShift := 128 - plen
	off := 0
	written := 0

	writeMsg := func(at, msgLen, count int) int {
		copy(buf[at:at+16], Marker)
		//nolint:gosec // msgLen bounded by bgpMaxMsgLen (4096)
		binary.BigEndian.PutUint16(buf[at+16:at+18], uint16(msgLen))
		buf[at+18] = MsgUPDATE
		buf[at+19], buf[at+20] = 0, 0
		attrsTotal := baseAttrsLen + mpFixed + count*stride
		//nolint:gosec // attrsTotal bounded by bgpMaxMsgLen (4096)
		binary.BigEndian.PutUint16(buf[at+21:at+23], uint16(attrsTotal))
		// ORIGIN IGP
		buf[at+23], buf[at+24], buf[at+25], buf[at+26] = flagWKTrans, 0x01, 0x01, 0x00
		// AS_PATH 4-byte ASN single-segment
		buf[at+27], buf[at+28], buf[at+29] = flagWKTrans, 0x02, 0x06
		buf[at+30], buf[at+31] = 0x02, 0x01
		binary.BigEndian.PutUint32(buf[at+32:at+36], spec.ASN)
		// MP_REACH_NLRI: flags(0x90) type(14) extlen(2)
		mpValueLen := 4 + 16 + 1 + count*stride // AFI(2)+SAFI(1)+nhlen(1)+nh(16)+reserved(1)+NLRI
		buf[at+36], buf[at+37] = flagOptNonTransEx, 14
		//nolint:gosec // mpValueLen bounded by bgpMaxMsgLen (4096)
		binary.BigEndian.PutUint16(buf[at+38:at+40], uint16(mpValueLen))
		// AFI=2 SAFI=1
		buf[at+40], buf[at+41] = 0x00, 0x02
		buf[at+42] = 0x01
		// NH length + NH
		buf[at+43] = 0x10
		copy(buf[at+44:at+60], nh[:])
		buf[at+60] = 0x00 // reserved / SNPA count
		return at + 61    // NLRI start
	}
	writeNLRI := func(at, n int) {
		// Each NLRI: plen byte + plBytes of the 16-byte address (big-endian top).
		for j := range n {
			//nolint:gosec // int->uint64 is safe for the non-negative counter range we generate
			addr := addShiftedU128(baseBig, uint64(written+j), stepShift)
			buf[at] = byte(plen)
			writeU128(buf[at+1:at+1+plBytes], addr, plBytes)
			at += stride
		}
	}

	for range fullMsgs {
		nlriAt := writeMsg(off, fullMsgLen, nlriPer)
		writeNLRI(nlriAt, nlriPer)
		off += fullMsgLen
		written += nlriPer
	}
	if rem > 0 {
		nlriAt := writeMsg(off, partialMsgLen, rem)
		writeNLRI(nlriAt, rem)
		off += partialMsgLen
	}
	if spec.EndOfRIB {
		copy(buf[off:off+16], Marker)
		binary.BigEndian.PutUint16(buf[off+16:off+18], bgpEORLen)
		buf[off+18] = MsgUPDATE
	}
	return buf, totalMsgs, nil
}

// u128 stores a 128-bit IPv6 address as two uint64 halves (hi, lo) so that
// we can increment by a shifted step without big.Int allocation.
type u128 struct{ hi, lo uint64 }

func bigUint128(b []byte) u128 {
	return u128{
		hi: binary.BigEndian.Uint64(b[0:8]),
		lo: binary.BigEndian.Uint64(b[8:16]),
	}
}

// addShiftedU128 returns base + (i << shift) mod 2^128. shift is the number
// of low bits that stay constant per step (128 - prefix_length).
func addShiftedU128(base u128, i uint64, shift int) u128 {
	var addHi, addLo uint64
	switch {
	case shift >= 128:
		// i << 128 wraps to zero.
	case shift >= 64:
		addHi = i << uint(shift-64)
	default:
		addHi = i >> uint(64-shift)
		addLo = i << uint(shift)
	}
	lo := base.lo + addLo
	carry := uint64(0)
	if lo < base.lo {
		carry = 1
	}
	hi := base.hi + addHi + carry
	return u128{hi: hi, lo: lo}
}

func writeU128(dst []byte, v u128, n int) {
	var tmp [16]byte
	binary.BigEndian.PutUint64(tmp[0:8], v.hi)
	binary.BigEndian.PutUint64(tmp[8:16], v.lo)
	copy(dst, tmp[:n])
}

// runActive is the entry point when Config.Dial is set. It dials the target,
// completes an active-role BGP OPEN handshake (send OPEN -> read peer OPEN ->
// send KEEPALIVE -> read peer KEEPALIVE), then hands off to doInject.
// Only supported with Mode == ModeInject.
func (p *Peer) runActive(ctx context.Context) Result {
	if p.config.Mode != ModeInject {
		return Result{Error: errors.New("--dial is only supported with --mode inject")}
	}
	if p.config.Inject == nil {
		return Result{Error: errors.New("--dial requires an inject spec")}
	}
	// Signal "ready" so callers waiting on Ready() unblock; there is no
	// listener to bind when dialing.
	p.readyOnce.Do(func() { close(p.ready) })

	p.printf("dialing %s...\n", p.config.Dial)
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	var conn net.Conn
	var err error
	for {
		conn, err = dialer.DialContext(ctx, "tcp", p.config.Dial)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return Result{Error: fmt.Errorf("dial canceled: %w", ctx.Err())}
		}
		if time.Now().After(deadline) {
			return Result{Error: fmt.Errorf("dial %s: %w", p.config.Dial, err)}
		}
		time.Sleep(500 * time.Millisecond)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			p.printf("conn close: %v\n", cerr)
		}
	}()

	// Low-latency handshake, large-batch inject. SetNoDelay flips again
	// inside doInject before the bulk write.
	if tc, ok := conn.(*net.TCPConn); ok {
		if nerr := tc.SetNoDelay(true); nerr != nil {
			p.printf("set nodelay: %v\n", nerr)
		}
	}

	ourOpen := buildActiveOpen(p.config)
	p.printPayload("open sent", ourOpen[:HeaderLen], ourOpen[HeaderLen:])
	if _, werr := conn.Write(ourOpen); werr != nil {
		return Result{Error: fmt.Errorf("write OPEN: %w", werr)}
	}

	peerHeader, peerBody, rerr := ReadMessage(conn)
	if rerr != nil {
		return Result{Error: fmt.Errorf("read peer OPEN: %w", rerr)}
	}
	if peerHeader[18] != MsgOPEN {
		return Result{Error: fmt.Errorf("expected OPEN, got type %d", peerHeader[18])}
	}
	p.printPayload("open recv", peerHeader, peerBody)

	if _, werr := conn.Write(KeepaliveMsg()); werr != nil {
		return Result{Error: fmt.Errorf("write KEEPALIVE: %w", werr)}
	}
	kaHeader, _, kerr := ReadMessage(conn)
	if kerr != nil {
		return Result{Error: fmt.Errorf("read peer KEEPALIVE: %w", kerr)}
	}
	if kaHeader[18] != MsgKEEPALIVE {
		return Result{Error: fmt.Errorf("expected KEEPALIVE, got type %d", kaHeader[18])}
	}

	return p.doInject(ctx, conn)
}

// buildActiveOpen constructs the OPEN message we send when acting as the
// active BGP role. We advertise the single address family implied by the
// inject spec plus the 4-byte ASN capability so ze's OPEN negotiation can
// accept 32-bit ASNs. No capability mirroring: we pick the minimum set.
func buildActiveOpen(cfg *Config) []byte {
	asn := uint32(0)
	family := uint16(1) // AFI=1 IPv4 by default
	if cfg.Inject != nil {
		asn = cfg.Inject.ASN
		if cfg.Inject.Prefix.Addr().Is6() {
			family = 2
		}
	}
	// Capabilities (RFC 5492 bundled in one type-2 optional parameter):
	//   MP-BGP: code 1, len 4, AFI(2) + reserved(1=0) + SAFI(1)
	//   4-byte ASN: code 65, len 4, ASN(4)
	var asnBytes [4]byte
	binary.BigEndian.PutUint32(asnBytes[:], asn)
	caps := make([]byte, 0, 16)
	caps = append(caps, 1, 4, byte(family>>8), byte(family), 0, 1, 65, 4)
	caps = append(caps, asnBytes[:]...)

	// Optional parameter wrapper: type 2 (Capability), length, caps.
	optParam := make([]byte, 0, 2+len(caps))
	optParam = append(optParam, 2, byte(len(caps)))
	optParam = append(optParam, caps...)

	// Router ID: derive from --dial host (active-side local identity is
	// less important than being unique; a fixed 1.1.1.1 fallback works if
	// the dial addr is not parseable).
	routerID := [4]byte{1, 1, 1, 1}
	if cfg.Dial != "" {
		if host, _, sperr := net.SplitHostPort(cfg.Dial); sperr == nil {
			if addr, perr := netip.ParseAddr(host); perr == nil {
				if addr.Is4() {
					routerID = addr.As4()
				}
			}
		}
	}

	// OPEN body: version(1) + AS(2) + hold(2) + id(4) + optlen(1) + optparam
	// 2-byte ASN field: use AS_TRANS (23456) for >16-bit ASNs; the real
	// ASN is carried in the 4-byte ASN capability above.
	asn2 := uint16(23456)
	if asn <= 65535 {
		asn2 = uint16(asn) //nolint:gosec // bounds checked
	}
	body := make([]byte, 0, 10+len(optParam))
	body = append(body, 4, byte(asn2>>8), byte(asn2), 0, 180)
	body = append(body, routerID[:]...)
	body = append(body, byte(len(optParam)))
	body = append(body, optParam...)

	msgLen := HeaderLen + len(body)
	msg := make([]byte, 0, msgLen)
	msg = append(msg, Marker...)
	msg = append(msg, byte(msgLen>>8), byte(msgLen), MsgOPEN)
	msg = append(msg, body...)
	return msg
}

// doInject is the ModeInject connection handler. Called from
// handleConnection after OPEN + KEEPALIVE have been exchanged.
func (p *Peer) doInject(ctx context.Context, conn net.Conn) Result {
	if p.config.Inject == nil {
		return Result{Error: errors.New("inject mode requires an InjectSpec (--inject-prefix/--inject-count/--inject-nexthop/--inject-asn)")}
	}
	spec := *p.config.Inject
	if !spec.EndOfRIB { // default-on for stress use
		spec.EndOfRIB = true
	}

	t0 := time.Now()
	buf, nMsgs, err := BuildUpdates(spec)
	if err != nil {
		return Result{Error: fmt.Errorf("build updates: %w", err)}
	}
	p.printf("inject built: %d messages, %d bytes in %s\n", nMsgs, len(buf), time.Since(t0))

	// Big sequential write: let the kernel batch. Peer turns Nagle off
	// for interactive tests; flip it on for bulk throughput.
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(false)
	}

	tWrite := time.Now()
	if _, err := conn.Write(buf); err != nil {
		return Result{Error: fmt.Errorf("inject write: %w", err)}
	}
	elapsed := time.Since(tWrite)
	mbps := float64(len(buf)) / elapsed.Seconds() / 1e6
	p.printf("inject sent: %d bytes in %s (%.1f MB/s)\n", len(buf), elapsed, mbps)

	return p.injectDwell(ctx, conn, spec.Dwell)
}

// injectDwell keeps the session alive after the stream has been written.
// Sends KEEPALIVE every 30s, drains peer-side messages, and exits on
// context cancel, dwell expiry, or peer disconnect.
func (p *Peer) injectDwell(ctx context.Context, conn net.Conn, dwell time.Duration) Result {
	var deadline <-chan time.Time
	if dwell > 0 {
		t := time.NewTimer(dwell)
		defer t.Stop()
		deadline = t.C
	}

	// Drain peer-side messages so ze's socket buffer never blocks us.
	errCh := make(chan error, 1)
	go func() {
		for {
			if _, _, err := ReadMessage(conn); err != nil {
				errCh <- err
				return
			}
		}
	}()

	ka := time.NewTicker(keepaliveInterval)
	defer ka.Stop()
	for {
		select {
		case <-ctx.Done():
			return Result{Success: true}
		case <-deadline:
			return Result{Success: true}
		case <-ka.C:
			if _, err := conn.Write(KeepaliveMsg()); err != nil {
				return Result{Error: fmt.Errorf("dwell keepalive: %w", err)}
			}
		case err := <-errCh:
			if errors.Is(err, io.EOF) || isConnReset(err) {
				return Result{Success: true}
			}
			return Result{Error: fmt.Errorf("dwell read: %w", err)}
		}
	}
}
