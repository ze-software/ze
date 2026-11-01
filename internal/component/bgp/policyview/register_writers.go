// Design: docs/plugin-development/protocol.md -- The Route Fields of filter-update
// Related: fields_json.go -- the writers registered here
// Related: register.go -- the readers, registered first (file order), that each writer reads through

package policyview

import (
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// The plugins that register an attribute's event formatter outside the
// engine. Their names appear in the Stage 1 refusal of a filter that declares
// the field in a binary that does not link them (spec R-10).
const (
	providerCommunity = "bgp-filter-community"
	providerRR        = "bgp-rr"
	providerAIGP      = "bgp-aigp"
)

// The BGP field writers, one per field that has a reader (register.go), so
// every field a BGP attachment carries can travel in a filter-update. An
// attribute field writes through the formatter its attribute registered for
// UPDATE events, looked up when the message is built, so a formatter a plugin
// registers is reached without naming the plugin here.
func init() {
	mustRegisterWriter(policy.FieldPrefix, writePrefix, 0, "")
	mustRegisterWriter(policy.FieldFamily, writeFamily, 0, "")
	mustRegisterWriter(policy.FieldProtocol, writeProtocol, 0, "")
	mustRegisterWriter(policy.FieldNextHop, writeNextHop, 0, "")
	mustRegisterWriter(policy.FieldOriginAS, writeOriginAS, 0, "")
	mustRegisterWriter(policy.FieldASPath, writeASPath, attribute.AttrASPath, "")
	mustRegisterWriter(policy.FieldMED, writeMED, attribute.AttrMED, "")
	mustRegisterWriter(policy.FieldLocalPreference, writeLocalPreference, attribute.AttrLocalPref, "")
	mustRegisterWriter(policy.FieldOrigin, formatted(attribute.AttrOrigin), attribute.AttrOrigin, "")
	mustRegisterWriter(policy.FieldAtomicAggregate, formatted(attribute.AttrAtomicAggregate), attribute.AttrAtomicAggregate, "")
	mustRegisterWriter(policy.FieldAggregator, formatted(attribute.AttrAggregator), attribute.AttrAggregator, "")
	mustRegisterWriter(policy.FieldCommunity, formatted(attribute.AttrCommunity), attribute.AttrCommunity, providerCommunity)
	mustRegisterWriter(policy.FieldLargeCommunity, formatted(attribute.AttrLargeCommunity), attribute.AttrLargeCommunity, providerCommunity)
	mustRegisterWriter(policy.FieldExtendedCommunity, formatted(attribute.AttrExtCommunity), attribute.AttrExtCommunity, providerCommunity)
	mustRegisterWriter(policy.FieldOriginatorID, formatted(attribute.AttrOriginatorID), attribute.AttrOriginatorID, providerRR)
	mustRegisterWriter(policy.FieldClusterList, formatted(attribute.AttrClusterList), attribute.AttrClusterList, providerRR)
	mustRegisterWriter(policy.FieldAIGP, formatted(attribute.AttrAIGP), attribute.AttrAIGP, providerAIGP)
}
