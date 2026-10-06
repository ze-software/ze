package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestRFC7950MandatoryUnderAbsentNonPresenceContainer is RED BY DESIGN until the
// owner decides how RFC7950-7.6.5-1 is met. It carries no RFC requirement tag.
//
// RFC 7950 §7.6.5: "If no such ancestor exists in the schema tree, the leaf MUST
// exist." The ancestor meant is the closest one that is not a non-presence
// container. In ze-bgp-conf.yang, bgp, session and asn are all non-presence
// containers, so bgp/session/asn/local (mandatory true) has no such ancestor and
// MUST exist whenever the bgp tree is validated.
//
// Method: validate a bgp tree that carries router-id and no session container.
// walkTree checks mandatory children only of the containers present in the data,
// so it never reaches session/asn/local and reports nothing.
func TestRFC7950MandatoryUnderAbsentNonPresenceContainer(t *testing.T) {
	v := newTestValidator(t)

	errs := v.ValidateTree("bgp", map[string]any{"router-id": "192.0.2.1"})

	found := false
	for _, e := range errs {
		if e.Type == yang.ErrTypeMissing && e.Path == "bgp/session/asn/local" {
			found = true
		}
	}
	assert.True(t, found, "a mandatory leaf under absent non-presence containers must be reported missing, got %v", errs)
}
