// Design: docs/architecture/api/update-syntax.md -- the mvpn NLRI section of `update text`
// RFC: rfc/short/rfc6514.md -- MCAST-VPN NLRI (SAFI 5), route types 5, 6 and 7
// Related: config.go -- the config route parser, which reads the same tokens
// Related: types.go -- MVPN NLRI route type codes

package mvpn

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMVPNMissingSourceAS = errors.New("mvpn shared-join and source-join require a source-as")
	errMVPNMissingSource   = errors.New("mvpn nlri requires a source (rp for shared-join, source otherwise)")
	errMVPNMissingGroup    = errors.New("mvpn nlri requires a group")
)

// mvpnFields holds one MCAST-VPN NLRI as its tokens name it, before encoding.
// The route type decides which fields the wire form carries: RFC 6514 Section
// 4.6 gives route types 6 and 7 a Source AS field that Section 4.5 does not give
// route type 5.
type mvpnFields struct {
	routeType byte
	rd        [8]byte
	sourceAS  uint32
	source    netip.Addr
	group     netip.Addr
}

// parseMVPNFields reads the MCAST-VPN tokens both entry points carry:
// `<route-type> [rp|source <addr>] [group <addr>] [rd <value>] [source-as <asn>]`.
//
// One parser, because the config route parser and the `update text` NLRI encoder
// take the SAME tokens from the operator. They had drifted before: `update text`
// answered "family not supported in text mode" for a family the config path
// wrote every day (ai/rules/principles.md).
//
// isIPv6 is the NLRI's address family. RFC 6514 Section 4: "The value of the AFI
// field in the MP_REACH_NLRI/MP_UNREACH_NLRI attribute that carries the
// MCAST-VPN NLRI determines whether the multicast source and multicast group
// addresses carried in the S-PMSI A-D routes, Source Active A-D routes, and
// C-multicast routes are IPv4 or IPv6 addresses (AFI 1 indicates IPv4 addresses,
// AFI 2 indicates IPv6 addresses)." So an address of the other family is refused
// by name rather than encoded under an AFI that contradicts it.
func parseMVPNFields(content []string, isIPv6 bool) (mvpnFields, error) {
	if len(content) == 0 {
		return mvpnFields{}, errMVPNMissingRouteType
	}

	var f mvpnFields
	switch content[0] {
	case "source-ad":
		f.routeType = byte(MVPNSourceActive)
	case "shared-join":
		f.routeType = byte(MVPNSharedTreeJoin)
	case "source-join":
		f.routeType = byte(MVPNSourceTreeJoin)
	default:
		return mvpnFields{}, fmt.Errorf("unknown MVPN route type %q", content[0])
	}

	var source, group, rdText string
	sourceASSet := false
	for i := 1; i < len(content); i += 2 {
		key := content[i]
		if i+1 >= len(content) {
			return mvpnFields{}, fmt.Errorf("missing value for %s", key)
		}
		value := content[i+1]
		switch key {
		case "rp", "source":
			source = value
		case "group":
			group = value
		case "rd":
			rdText = value
		case "source-as":
			// asn.Parse reads all three RFC 5396 spellings.
			number, err := asn.Parse(value)
			if err != nil {
				return mvpnFields{}, fmt.Errorf("mvpn source-as %q: %w", value, err)
			}
			f.sourceAS = number
			sourceASSet = true
		default:
			return mvpnFields{}, fmt.Errorf("unknown MVPN keyword: %s", key)
		}
	}

	if rdText == "" {
		return mvpnFields{}, errMVPNMissingRD
	}
	rd, err := rdStringToBytes(rdText)
	if err != nil {
		return mvpnFields{}, fmt.Errorf("mvpn rd: %w", err)
	}
	f.rd = rd

	// RFC 6514 Section 4.6: a Shared Tree Join and a Source Tree Join NLRI carry
	// a "Source AS (4 octets)" field. An absent source-as would be written as
	// AS 0, which a reader cannot tell from an AS the operator chose, so it is
	// refused here (ai/rules/principles.md).
	if !sourceASSet && (f.routeType == byte(MVPNSharedTreeJoin) || f.routeType == byte(MVPNSourceTreeJoin)) {
		return mvpnFields{}, errMVPNMissingSourceAS
	}

	if source == "" {
		return mvpnFields{}, errMVPNMissingSource
	}
	f.source, err = mvpnAddr(source, isIPv6)
	if err != nil {
		return mvpnFields{}, fmt.Errorf("mvpn source: %w", err)
	}

	if group == "" {
		return mvpnFields{}, errMVPNMissingGroup
	}
	f.group, err = mvpnAddr(group, isIPv6)
	if err != nil {
		return mvpnFields{}, fmt.Errorf("mvpn group: %w", err)
	}

	return f, nil
}

// mvpnAddr parses one C-S, C-RP or C-G address and holds it to the NLRI's AFI.
func mvpnAddr(text string, isIPv6 bool) (netip.Addr, error) {
	addr, err := netip.ParseAddr(text)
	if err != nil {
		return netip.Addr{}, err
	}
	if addr.Unmap().Is4() == isIPv6 {
		want := "IPv4"
		if isIPv6 {
			want = "IPv6"
		}
		return netip.Addr{}, fmt.Errorf("%q is not an %s address, which this family's AFI requires", text, want)
	}
	return addr.Unmap(), nil
}

// EncodeNLRIHex encodes one MCAST-VPN NLRI from CLI-style tokens and returns
// uppercase hex. It implements the InProcessNLRIEncoder signature for the plugin
// registry, which is how `send bgp <peer> update text ... nlri ipv4/mvpn add
// shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as
// 65000` reaches this package (update/update_text_nlri.go
// parseRegistryNLRISection).
func EncodeNLRIHex(family string, args []string) (string, error) {
	fields, err := parseMVPNFields(args, strings.HasPrefix(family, "ipv6/"))
	if err != nil {
		return "", err
	}

	wire, err := mvpnNLRI(fields.routeType, fields.rd, fields.sourceAS, fields.source, fields.group)
	if err != nil {
		return "", err
	}
	return textbuf.StringHexUpper(wire), nil
}
