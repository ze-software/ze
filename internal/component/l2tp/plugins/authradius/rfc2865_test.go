// RFC 2865 (RADIUS) subscriber-side (L2TP NAS) behavioral requirement.
//
// VALIDATES: RFC 2865 Section 5 -- every subscriber Access-Request the L2TP RADIUS
// client builds carries a User-Name attribute.
// PREVENTS: a subscriber Access-Request being sent without User-Name, which a RADIUS
// server would reject or mis-account.

package l2tpauthradius

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

func TestRFC2865SubscriberAccessRequestUserName(t *testing.T) {
	req := ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 2,
		Method:    ppp.AuthMethodPAP,
		Username:  "subscriber@example.net",
		Response:  []byte("secret-pw"),
	}
	attrs := buildAuthAttrs(req, "nas-1", net.IPv4(10, 0, 0, 1))

	// RFC requirement: RFC2865-5-1 positive -- the subscriber Access-Request the L2TP RADIUS
	// client builds carries a User-Name attribute (Section 5, required in Access-Request).
	var got string
	present := false
	for _, a := range attrs {
		if a.Type == radius.AttrUserName {
			present = true
			got = string(a.Value)
		}
	}
	if !present {
		t.Fatal("an Access-Request MUST include a User-Name attribute")
	}
	if got != "subscriber@example.net" {
		t.Errorf("User-Name: got %q, want %q", got, "subscriber@example.net")
	}
}
