// Design: docs/architecture/api/text-parser.md -- immutable physical source header.
package textparse

import "testing"

// The real formatter's header order must expose all four independent source
// values, without integer truncation. A connection token in route contents is
// deliberately powerless. This checks the complete parser used by the roles,
// plus the narrower existing readers of the same reserved header.
// MUTATION: treat connection-id as the first route token.
func TestSourcePhysicalProvenanceReadsExactOriginalHeader(t *testing.T) {
	const header = "peer 192.0.2.93 remote as 64500 received update 27"
	const original = uint64(18446744073709551614)
	for _, fields := range []string{
		" source-policy-epoch 17 source-receipt 19 peer-owner 23 connection-id 18446744073709551614",
		" connection-id 18446744073709551614 source-receipt 19 peer-owner 23 source-policy-epoch 17",
	} {
		epoch, receipt, owner, connection, err := SourcePhysicalProvenance(header + fields + " origin igp connection-id 99")
		if err != nil {
			t.Fatal(err)
		}
		if epoch != 17 {
			t.Fatalf("policy epoch = %d", epoch)
		}
		if receipt != 19 {
			t.Fatalf("source receipt = %d", receipt)
		}
		if owner != 23 {
			t.Fatalf("original owner = %d", owner)
		}
		if connection != original {
			t.Fatalf("physical connection = %d; receipt and owner are independent", connection)
		}
		plainEpoch, plainReceipt, plainOwner, err := SourceProvenance(header + fields + " origin igp")
		if err != nil {
			t.Fatal(err)
		}
		if plainEpoch != epoch {
			t.Fatal("the narrower reader lost the policy epoch")
		}
		if plainReceipt != receipt {
			t.Fatal("the narrower reader lost the source receipt")
		}
		if plainOwner != owner {
			t.Fatal("the narrower reader lost the original owner")
		}
	}
	for _, suffix := range []string{"", " origin igp connection-id 99", " source-receipt 19 origin igp connection-id 99"} {
		_, _, _, connection, err := SourcePhysicalProvenance(header + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if connection != 0 {
			t.Fatalf("route contents invented physical authority: %d", connection)
		}
	}
}

// Malformed original physical facts invalidate the whole header, even for an
// old consumer that asks only for its epoch. No partial admission survives.
// MUTATION: ignore malformed or repeated connection-id pairs.
func TestSourcePhysicalProvenanceRefusesMalformedCompleteHeader(t *testing.T) {
	const header = "peer 192.0.2.93 remote as 64500 received update 27 source-policy-epoch 17 source-receipt 19 peer-owner 23"
	for _, suffix := range []string{
		" connection-id", " connection-id -1", " connection-id 1.5",
		" connection-id 18446744073709551616", " connection-id 31 connection-id 32",
		" connection-id 31 source-receipt 20", " connection-id 31 peer-owner 24",
	} {
		t.Run(suffix, func(t *testing.T) {
			epoch, receipt, owner, connection, err := SourcePhysicalProvenance(header + suffix)
			if err == nil {
				t.Fatal("malformed complete provenance accepted")
			}
			if epoch != 0 {
				t.Fatal("partial epoch survived refusal")
			}
			if receipt != 0 {
				t.Fatal("partial source receipt survived refusal")
			}
			if owner != 0 {
				t.Fatal("partial owner survived refusal")
			}
			if connection != 0 {
				t.Fatal("partial connection survived refusal")
			}
			if _, _, _, err := SourceProvenance(header + suffix); err == nil {
				t.Fatal("narrow provenance reader skipped malformed physical facts")
			}
			if _, err := SourcePolicyEpoch(header + suffix); err == nil {
				t.Fatal("epoch reader skipped malformed physical facts")
			}
		})
	}
}
