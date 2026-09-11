package nlrisplit

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestPrefixKeyMasksCIDRAndRTCPadding(t *testing.T) {
	t.Parallel()
	for _, fam := range []family.Family{family.IPv4Unicast, {AFI: family.AFIIPv4, SAFI: family.SAFIRTC}} {
		t.Run(fam.String(), func(t *testing.T) {
			t.Parallel()
			key := GetPrefixKey(fam)
			var first, second [PrefixKeyScratchSize]byte
			a, err := key([]byte{9, 192, 0}, first[:], false)
			if err != nil {
				t.Fatal(err)
			}
			b, err := key([]byte{9, 192, 127}, second[:], false)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, b) {
				t.Fatalf("padding changed prefix key: %x != %x", a, b)
			}
			b, err = key([]byte{9, 192, 128}, second[:], false)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(a, b) {
				t.Fatal("different prefix shares a key")
			}
		})
	}
}

func TestPrefixKeyFlowSpecLengthIsNotRuleIdentity(t *testing.T) {
	t.Parallel()
	key := GetPrefixKey(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec})
	short := []byte{3, 3, 0x81, 6}
	long := []byte{0xf0, 3, 3, 0x81, 6}
	a, err := key(short, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := key(long, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("length encoding changed rule: %x != %x", a, b)
	}
	long[len(long)-1] = 17
	b, err = key(long, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("different protocol filter shares a key")
	}
}
