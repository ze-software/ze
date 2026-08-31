// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS NAS identity
// RFC: rfc/short/rfc2866.md -- NAS-IP-Address or NAS-Identifier (Section 4.1)
// RFC: rfc/short/rfc2865.md -- NAS-IP-Address or NAS-Identifier (Section 4.1)
// Related: acct.go -- the Accounting-Request attribute set
// Related: handler.go -- the Access-Request attribute set

package l2tpauthradius

import (
	"net"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/radius"
)

// appendNASIdentity appends the attributes that name this NAS to the server.
//
// RFC 2866 Section 4.1: "Either NAS-IP-Address or NAS-Identifier MUST be
// present in a RADIUS Accounting-Request."
// RFC 2865 Section 4.1: "Either NAS-IP-Address or NAS-Identifier MUST be
// present in an Access-Request."
//
// source-address and nas-identifier are both optional leaves with no default
// (yang/ze-l2tp-auth-radius-conf.yang), so a configuration that sets neither
// named no NAS at all. hostNASIdentifier answers for that configuration, which
// is how radius.NewAAA already names this device on the admin path.
//
// The fallback is narrow on purpose. An operator who set a source address is
// already conformant, so nothing is invented for that packet, and an operator
// who set a NAS-Identifier gets the text they chose.
func appendNASIdentity(attrs []radius.Attr, nasID string, sourceAddr net.IP) []radius.Attr {
	v4 := sourceAddr.To4()
	if v4 != nil {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIPAddress, Value: v4})
	}
	if nasID != "" {
		return append(attrs, radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString(nasID)})
	}
	if v4 != nil {
		return attrs
	}
	return append(attrs, radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString(hostNASIdentifier())})
}

// hostNASIdentifier answers the NAS-Identifier text of a NAS whose operator set
// neither leaf. The host name names the device, and the plugin name answers for
// a host that has none, so the attribute is never text of length zero, which
// RFC 2866 Section 5 forbids. The name is read once: it does not change under a
// running daemon, and every accounting record of every session asks for it.
var hostNASIdentifier = sync.OnceValue(func() string {
	host, err := os.Hostname()
	if err != nil {
		return Name
	}
	if host == "" {
		return Name
	}
	return host
})
