// Design: docs/architecture/l2tp/subscriber-session-model.md -- packed birth boundary
package subscriber

import (
	"testing"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

func TestL2TPBirthPackedBoundaryRoundTrip(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	if !birth.IsValid() {
		t.Fatal("minted birth is absent")
	}
	text := birth.String()
	parsed, err := subscriberid.ParseL2TPBirth(text)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != birth {
		t.Fatal("boundary changed packed identity")
	}
	owners := map[subscriberid.L2TPBirth]int{birth: 1}
	if owners[parsed] != 1 {
		t.Fatal("packed value does not preserve comparable identity")
	}
	encoded, err := birth.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var decoded subscriberid.L2TPBirth
	if err := decoded.UnmarshalText(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded != birth {
		t.Fatal("text observer changed packed identity")
	}
}

func TestL2TPBirthRefusesNoncanonicalBoundaryAndPreservesReceiver(t *testing.T) {
	original, err := subscriberid.ParseL2TPBirth("l2tp-0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"", "l2tp-1-7", "l2tp-00000000000000000000000000000000", "l2tp-0123456789ABCDEF0123456789abcdef", "l2tp-0123456789abcdef0123456789abcdef0", "pppoe-0123456789abcdef0123456789abcdef"} {
		t.Run(text, func(t *testing.T) {
			if _, err := subscriberid.ParseL2TPBirth(text); err == nil {
				t.Fatal("invalid external identity accepted")
			}
			after := original
			if err := after.UnmarshalText([]byte(text)); err == nil {
				t.Fatal("invalid text mutation accepted")
			}
			if after != original {
				t.Fatal("failed parse mutated an existing birth")
			}
		})
	}
	var absent subscriberid.L2TPBirth
	if absent.IsValid() {
		t.Fatal("zero is a real birth")
	}
	if absent.String() != "" {
		t.Fatal("zero fabricated an external identity")
	}
	if _, err := absent.MarshalText(); err == nil {
		t.Fatal("zero serialized as real identity")
	}
	var missing *subscriberid.L2TPBirth
	if err := missing.UnmarshalText([]byte(original.String())); err == nil {
		t.Fatal("nil receiver accepted")
	}
}
