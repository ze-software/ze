// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact scope wire DTO.
// Related: xfrm_state_linux.go -- state attribute construction and readback.
// Upstream: https://github.com/vishvananda/netlink, v1.3.1.

package netlink

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// XfrmStateScopeFlags carries the negotiated scope flags in the private ABI.
type XfrmStateScopeFlags uint32

const (
	XfrmStateScopeNonInitial   XfrmStateScopeFlags = 1
	XfrmStateScopeMaxSelectors                     = 64
	XfrmZEABI                  uint32              = 0x5a450001
)

// XfrmStateScope is a raw wire DTO. WriteTo validates every field before writing;
// ParseXfrmStateScope validates the complete payload before publishing a result.
type XfrmStateScope struct {
	Flags     XfrmStateScopeFlags
	Selectors []nl.XfrmSelector
}

func (s *XfrmStateScope) WriteTo(buf []byte, off int) (int, error) {
	if err := s.validate(); err != nil {
		return 0, err
	}
	if off < 0 {
		return 0, errors.New("XFRM state scope output offset is negative")
	}
	if off > len(buf) {
		return 0, errors.New("XFRM state scope output offset exceeds the buffer")
	}
	size := 8 + len(s.Selectors)*nl.SizeofXfrmSelector
	if len(buf)-off < size {
		return 0, errors.New("XFRM state scope output buffer is too short")
	}
	native.PutUint32(buf[off:], uint32(s.Flags))
	native.PutUint32(buf[off+4:], uint32(len(s.Selectors)))
	for i := range s.Selectors {
		copy(buf[off+8+i*nl.SizeofXfrmSelector:], s.Selectors[i].Serialize())
	}
	return size, nil
}

func ParseXfrmStateScope(payload []byte) (*XfrmStateScope, error) {
	if len(payload) < 8 {
		return nil, errors.New("XFRM state scope header is truncated")
	}
	count := native.Uint32(payload[4:8])
	if count == 0 {
		return nil, errors.New("XFRM state scope contains no selectors")
	}
	if count > XfrmStateScopeMaxSelectors {
		return nil, errors.New("XFRM state scope exceeds 64 selectors")
	}
	if len(payload) != 8+int(count)*nl.SizeofXfrmSelector {
		return nil, errors.New("XFRM state scope length does not match its selector count")
	}
	scope := &XfrmStateScope{Flags: XfrmStateScopeFlags(native.Uint32(payload)),
		Selectors: make([]nl.XfrmSelector, int(count))}
	for i := range scope.Selectors {
		copy(scope.Selectors[i].Serialize(), payload[8+i*nl.SizeofXfrmSelector:8+(i+1)*nl.SizeofXfrmSelector])
	}
	if err := scope.validate(); err != nil {
		return nil, err
	}
	return scope, nil
}

// validate checks both boundaries: a raw DTO can be changed after a read.
func (s *XfrmStateScope) validate() error {
	if s == nil {
		return errors.New("XFRM state scope is nil")
	}
	if s.Flags & ^XfrmStateScopeNonInitial != 0 {
		return errors.New("XFRM state scope contains unknown flags")
	}
	if len(s.Selectors) == 0 {
		return errors.New("XFRM state scope contains no selectors")
	}
	if len(s.Selectors) > XfrmStateScopeMaxSelectors {
		return errors.New("XFRM state scope exceeds 64 selectors")
	}
	for i := range s.Selectors {
		if err := validateScopeSelector(&s.Selectors[i]); err != nil {
			return fmt.Errorf("XFRM state scope selector %d: %w", i, err)
		}
	}
	return nil
}

func validateScopeSelector(s *nl.XfrmSelector) error {
	bits := 0
	switch s.Family {
	case unix.AF_INET:
		bits = 32
	case unix.AF_INET6:
		bits = 128
	default:
		return fmt.Errorf("unknown address family %d", s.Family)
	}
	if !scopeAddressValid(s.Saddr, int(s.PrefixlenS), bits) {
		return errors.New("source prefix is not canonical")
	}
	if !scopeAddressValid(s.Daddr, int(s.PrefixlenD), bits) {
		return errors.New("destination prefix is not canonical")
	}
	if !scopePortValid(s.Sport, s.SportMask) {
		return errors.New("source port or mask is not canonical")
	}
	if !scopePortValid(s.Dport, s.DportMask) {
		return errors.New("destination port or mask is not canonical")
	}
	if s.Ifindex < 0 {
		return errors.New("interface index is negative")
	}
	if s.User != 0 {
		return errors.New("user selector is unsupported")
	}
	if s.Pad != [3]byte{} {
		return errors.New("selector padding is nonzero")
	}
	if s.SportMask != 0 || s.DportMask != 0 {
		switch s.Proto {
		case unix.IPPROTO_TCP, unix.IPPROTO_UDP, unix.IPPROTO_UDPLITE,
			unix.IPPROTO_SCTP, unix.IPPROTO_ICMP, unix.IPPROTO_ICMPV6:
		default:
			return fmt.Errorf("protocol %d has no supported constrained port fields", s.Proto)
		}
	}
	if s.Proto == unix.IPPROTO_ICMP && s.Family != unix.AF_INET {
		return errors.New("ICMP selector is not IPv4")
	}
	if s.Proto == unix.IPPROTO_ICMPV6 && s.Family != unix.AF_INET6 {
		return errors.New("ICMPv6 selector is not IPv6")
	}
	return nil
}

func scopeAddressValid(address nl.XfrmAddress, prefix, bits int) bool {
	if prefix > bits {
		return false
	}
	for bit := prefix; bit < 128; bit++ {
		if address[bit/8]&(1<<uint(7-bit%8)) != 0 {
			return false
		}
	}
	return true
}

func scopePortValid(port, mask uint16) bool {
	value, bits := nl.Swap16(port), nl.Swap16(mask)
	inverse := ^bits
	return value & ^bits == 0 && inverse&(inverse+1) == 0
}

// XfrmZEFeature carries individual enforcement features reported by GETDEFAULT.
type XfrmZEFeature uint32

const (
	XfrmZECapStateScope XfrmZEFeature = 1 << iota
	XfrmZECapICMPQuote
	XfrmZECapFragmentContext
	XfrmZECapFragmentGuard
	XfrmZECapICMPFinal
)

func (f XfrmZEFeature) Has(required XfrmZEFeature) bool {
	return required != 0 && f&required == required
}

// XfrmZECapabilities is the read-only capability receipt of Ze's kernel ABI.
type XfrmZECapabilities struct {
	Features     XfrmZEFeature
	MaxSelectors uint32
	GuardRefs    uint32
	ABI          uint32
}

func ParseXfrmZECapabilities(payload []byte) (*XfrmZECapabilities, error) {
	if len(payload) < 3 {
		return nil, errors.New("XFRM GETDEFAULT reply is truncated")
	}
	if len(payload) < 4 {
		return nil, ErrXfrmZECapabilitiesAbsent
	}
	attrs, err := nl.ParseRouteAttr(payload[4:])
	if err != nil {
		return nil, fmt.Errorf("XFRM GETDEFAULT attributes: %w", err)
	}
	var caps *XfrmZECapabilities
	for _, attr := range attrs {
		if attr.Attr.Type != nl.XFRMA_ZE_CAPS {
			continue
		}
		if caps != nil {
			return nil, errors.New("XFRM GETDEFAULT has duplicate capability receipts")
		}
		if len(attr.Value) != 16 {
			return nil, errors.New("XFRM capability receipt has an invalid length")
		}
		caps = &XfrmZECapabilities{Features: XfrmZEFeature(native.Uint32(attr.Value)),
			MaxSelectors: native.Uint32(attr.Value[4:]), GuardRefs: native.Uint32(attr.Value[8:]),
			ABI: native.Uint32(attr.Value[12:])}
		if caps.ABI != XfrmZEABI {
			return nil, fmt.Errorf("XFRM capability ABI %#x differs from %#x", caps.ABI, XfrmZEABI)
		}
		if caps.MaxSelectors != XfrmStateScopeMaxSelectors {
			return nil, fmt.Errorf("XFRM selector limit %d differs from 64", caps.MaxSelectors)
		}
		if caps.Features & ^(XfrmZECapStateScope|XfrmZECapICMPQuote|XfrmZECapFragmentContext|XfrmZECapFragmentGuard|XfrmZECapICMPFinal) != 0 {
			return nil, errors.New("XFRM capability receipt contains unknown feature bits")
		}
	}
	if caps == nil {
		return nil, ErrXfrmZECapabilitiesAbsent
	}
	return caps, nil
}

// ErrXfrmZECapabilitiesAbsent means GETDEFAULT answered without Ze's receipt. An
// ignored attribute echoed by an old kernel does not establish enforcement.
var ErrXfrmZECapabilitiesAbsent = errors.New("XFRM GETDEFAULT returned no Ze capability receipt")

// XfrmZECapabilitiesGet reads the live namespace; it never caches enforcement.
func XfrmZECapabilitiesGet() (*XfrmZECapabilities, error) {
	return pkgHandle.XfrmZECapabilitiesGet()
}

func (h *Handle) XfrmZECapabilitiesGet() (*XfrmZECapabilities, error) {
	const getDefault = 0x28
	req := h.newNetlinkRequest(getDefault, unix.NLM_F_REQUEST)
	req.AddRawData([]byte{0, 0, 0})
	messages, err := req.Execute(unix.NETLINK_XFRM, getDefault)
	if err != nil {
		return nil, fmt.Errorf("XFRM GETDEFAULT: %w", err)
	}
	if len(messages) != 1 {
		return nil, fmt.Errorf("XFRM GETDEFAULT returned %d messages, want one", len(messages))
	}
	return ParseXfrmZECapabilities(messages[0])
}
