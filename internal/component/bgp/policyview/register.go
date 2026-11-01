// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: readers.go -- the readers registered here

package policyview

import "github.com/ze-software/ze/internal/component/policy"

// The BGP field readers, one per field the BGP import and export attachments
// carry (internal/component/bgp/config/register.go). The field names stay
// declared in the policy library; only how BGP reads each one lives here.
// metric has no reader: BGP names that fact med (spec D-8).
func init() {
	MustRegisterReader(policy.FieldPrefix, readPrefix)
	MustRegisterReader(policy.FieldFamily, readFamily)
	MustRegisterReader(policy.FieldProtocol, readProtocol)
	MustRegisterReader(policy.FieldNextHop, readNextHop)
	MustRegisterReader(policy.FieldOriginAS, readOriginAS)
	MustRegisterReader(policy.FieldCommunity, readCommunity)
	MustRegisterReader(policy.FieldLargeCommunity, readLargeCommunity)
	MustRegisterReader(policy.FieldExtendedCommunity, readExtendedCommunity)
	MustRegisterReader(policy.FieldASPath, readASPath)
	MustRegisterReader(policy.FieldLocalPreference, readLocalPreference)
	MustRegisterReader(policy.FieldOrigin, readOrigin)
	MustRegisterReader(policy.FieldAIGP, readAIGP)
	MustRegisterReader(policy.FieldMED, readMED)
	MustRegisterReader(policy.FieldAtomicAggregate, readAtomicAggregate)
	MustRegisterReader(policy.FieldAggregator, readAggregator)
	MustRegisterReader(policy.FieldOriginatorID, readOriginatorID)
	MustRegisterReader(policy.FieldClusterList, readClusterList)
}
