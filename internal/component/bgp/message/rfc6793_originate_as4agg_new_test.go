package message

import "testing"

// TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker drives every
// builder that writes an AGGREGATOR, with a non-mappable aggregating AS, toward
// a peer that negotiated four-octet AS support. The same input toward an OLD
// speaker gains an AS4_AGGREGATOR
// (TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker), so its absence
// here is decided by the destination being a NEW speaker.
//
// RFC requirement: RFC6793-4.1-6 positive -- an originated UPDATE toward a NEW speaker carries
// no AS4_AGGREGATOR, even for a non-mappable aggregating AS: the AGGREGATOR itself carries
// the real four-octet AS number.
func TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker(t *testing.T) {
	for _, originator := range rfc6793Aggregators {
		t.Run(originator.name, func(t *testing.T) {
			ub := NewUpdateBuilder(rfc6793OriginLocalAS, false, true, false)
			attrs := originator.build(t, ub, rfc6793OriginNonMappable)

			if got, found := as4AggregatorASN(t, attrs); found {
				t.Errorf("AS4_AGGREGATOR AS = %d, want none: the peer negotiated four-octet AS numbers", got)
			}
			if got := aggregatorASN(t, attrs, true); got != rfc6793OriginNonMappable {
				t.Errorf("AGGREGATOR AS = %d, want %d", got, rfc6793OriginNonMappable)
			}
		})
	}
}
