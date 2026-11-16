// Design: docs/architecture/api/text-parser.md -- reserved UPDATE provenance pairs.
package textparse

import "testing"

// TestSourcePolicyEpochChecksWholeProvenance validates the existing epoch
// reader's complete reserved header before returning any admission value.
// PREVENTS: a valid epoch hiding a malformed or repeated source receipt.
func TestSourcePolicyEpochChecksWholeProvenance(t *testing.T) {
	const header = "peer 192.0.2.1 remote as 65001 received update 42"
	for _, suffix := range []string{
		" source-policy-epoch 9 source-receipt",
		" source-policy-epoch 9 source-receipt -1",
		" source-policy-epoch 9 source-receipt 1.5",
		" source-policy-epoch 9 source-receipt 7 source-receipt 8",
		" source-policy-epoch 9 source-policy-epoch 10",
	} {
		t.Run(suffix, func(t *testing.T) {
			if _, err := SourcePolicyEpoch(header + suffix); err == nil {
				t.Fatal("malformed or repeated reserved provenance was accepted")
			}
		})
	}
	// Route contents are not the provenance header. A reader cannot discover
	// authority by searching arbitrary later attribute or metadata tokens.
	epoch, err := SourcePolicyEpoch(header + " origin igp source-policy-epoch 99 source-receipt bad")
	if err != nil || epoch != 0 {
		t.Fatalf("route contents became header provenance: epoch=%d error=%v", epoch, err)
	}
}

// TestSourceProvenancePreservesOriginalIdentity covers both reserved scalars
// without floating-point conversion or deriving a missing current identity.
func TestSourceProvenancePreservesOriginalIdentity(t *testing.T) {
	const header = "peer 192.0.2.1 remote as 65001 received update 42"
	for _, test := range []struct {
		name    string
		fields  string
		epoch   uint64
		receipt uint64
	}{
		{name: "unrecorded"},
		{name: "epoch-only", fields: " source-policy-epoch 9", epoch: 9},
		{name: "receipt-only", fields: " source-receipt 7", receipt: 7},
		{name: "exact", fields: " source-policy-epoch 9 source-receipt 18446744073709551615", epoch: 9, receipt: 18446744073709551615},
	} {
		t.Run(test.name, func(t *testing.T) {
			epoch, receipt, err := SourceProvenance(header + test.fields + " origin igp")
			if err != nil || epoch != test.epoch || receipt != test.receipt {
				t.Fatalf("provenance = (%d,%d,%v), want (%d,%d,nil)", epoch, receipt, err, test.epoch, test.receipt)
			}
		})
	}
}
