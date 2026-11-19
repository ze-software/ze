// Design: docs/architecture/plugin/rib-storage-design.md -- exact recovery metadata.
package update

import "testing"

func TestRecoveryAndDestinationOwnerMetadataRefuseUnrecordedAuthority(t *testing.T) {
	for _, value := range []any{nil, 0, float64(1), "0", "-1", "18446744073709551616"} {
		if _, err := destinationOwnerFromMeta(map[string]any{"destination-owner": value}); err == nil {
			t.Errorf("accepted destination owner %#v", value)
		}
		if _, err := recoveryOwnerFromMeta(map[string]any{"recovery-source": "192.0.2.1", "recovery-source-owner": value}); err == nil {
			t.Errorf("accepted recovery source owner %#v", value)
		}
	}
	const exact = uint64(18446744073709551615)
	for _, parse := range []func(map[string]any) (uint64, error){destinationOwnerFromMeta, recoveryOwnerFromMeta} {
		got, err := parse(map[string]any{"destination-owner": "18446744073709551615", "recovery-source": "192.0.2.1", "recovery-source-owner": "18446744073709551615"})
		if err != nil || got != exact {
			t.Fatalf("decimal metadata lost uint64 identity: got %d, error %v", got, err)
		}
		got, err = parse(nil)
		if err != nil || got != 0 {
			t.Fatalf("ordinary operator command changed: got %d, error %v", got, err)
		}
	}
}
