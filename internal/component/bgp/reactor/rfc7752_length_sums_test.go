package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// RFC 7752 Section 6.2.2 requires checking the sums in MP_REACH, MP_UNREACH,
// and the Node, Link and Prefix descriptors against their enclosing lengths.
// RFC requirement: RFC7752-6.2.2-2 positive -- exact-sum Node, Link and Prefix descriptor sequences survive both MP_REACH and MP_UNREACH validation byte-identically.
// RFC requirement: RFC7752-6.2.2-2 negative -- overrun NLRI totals and overrun or underrun descriptor lengths are detected in both MP attributes, rather than propagated unchanged.
func TestRFC7752LinkStateLengthSumsOnReceive(t *testing.T) {
	for _, kind := range []byte{1, 2, 3} {
		for _, withdrawn := range []bool{false, true} {
			for _, fault := range []string{"none", "nlri-overrun", "descriptor-overrun", "descriptor-underrun"} {
				wire := lsNodeNLRI(65001)
				wire[1] = kind
				switch fault {
				case "nlri-overrun":
					wire[3]++
				case "descriptor-overrun":
					wire[16]++
				case "descriptor-underrun":
					wire[16]--
				}
				attrs := mpReachAttrs(lsFam, wire)
				if withdrawn {
					attrs = mpUnreachAttrs(lsFam, wire)
				}
				body := makeUpdateBody(nil, attrs, nil)
				original := bytes.Clone(body)
				result, action, err := nlriTypeTestSession().enforceRFC7606(wireu.NewWireUpdate(body, 0))
				unchanged := result != nil && bytes.Equal(result.Payload(), original)
				if fault == "none" {
					if err != nil || action != message.RFC7606ActionNone || !unchanged {
						t.Fatalf("kind=%d withdrawn=%v: valid length sums changed: action=%v err=%v", kind, withdrawn, action, err)
					}
				} else if err == nil && action == message.RFC7606ActionNone && unchanged {
					t.Fatalf("kind=%d withdrawn=%v fault=%s propagated unchanged", kind, withdrawn, fault)
				}
			}
		}
	}
}
