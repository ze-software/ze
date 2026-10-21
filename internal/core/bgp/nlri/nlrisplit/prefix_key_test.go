package nlrisplit

import (
	"bytes"
	"errors"
	"slices"
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

// labeledNLRI builds one labeled NLRI: the ADD-PATH identifier when id is
// non-nil, the length octet, the label field, rd and prefix. labels is the
// stack depth of an announcement; zero builds a withdrawal whose one
// Compatibility field holds compat. bits is the prefix length.
func labeledNLRI(id []byte, labels int, compat uint32, rd []byte, bits int, prefix []byte) []byte {
	out := append([]byte(nil), id...)
	field := 3 * labels
	if labels == 0 {
		field = 3
	}
	out = append(out, byte(8*field+8*len(rd)+bits))
	if labels == 0 {
		out = append(out, byte(compat>>16), byte(compat>>8), byte(compat))
	}
	for i := range labels {
		bottom := byte(0)
		if i == labels-1 {
			bottom = 1
		}
		out = append(out, 0x00, byte(0x10+i), 0x40|bottom)
	}
	out = append(out, rd...)
	return append(out, prefix...)
}

// withdrawalWriterFamilies is the set the writer and the key inverse answer
// for: labeled unicast and MPLS VPN, IPv4 and IPv6.
var withdrawalWriterFamilies = []family.Family{
	{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
	{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
	{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
	{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
}

// TestWithdrawalWriterField checks the field-level withdrawal writer against
// the per-key inverse, entry by entry. Goal: one declaration of the RFC 8277
// Section 2.4 framing, so the forward rail's converted withdrawal and the owed
// or recovered withdrawal of the same route are the same bytes after the path
// identifier. Method: build fields of several NLRIs with one and three labels,
// with and without ADD-PATH, announcement and withdrawal framed, convert each
// with the registered writer and compare with WithdrawalNLRIFromKey over each
// entry's keyLabeled / keyVPN key; then feed it every shape keyLabeled and
// keyVPN refuse, a destination one octet short, and count allocations.
func TestWithdrawalWriterField(t *testing.T) {
	// Not parallel: testing.AllocsPerRun refuses a parallel test.
	mu.RLock()
	registered := make([]family.Family, 0, len(splitters))
	for fam := range splitters {
		registered = append(registered, fam)
	}
	mu.RUnlock()
	for _, fam := range registered {
		want := slices.Contains(withdrawalWriterFamilies, fam)
		if got := GetWithdrawalWriter(fam) != nil; got != want {
			t.Errorf("%s: GetWithdrawalWriter answers %v, want %v", fam, got, want)
		}
	}

	rd := []byte{0, 0, 0xfd, 0xe8, 0, 0, 0, 1}
	for _, fam := range withdrawalWriterFamilies {
		var famRD []byte
		key := keyLabeled
		if fam.SAFI == family.SAFIVPN {
			famRD, key = rd, keyVPN
		}
		writer := GetWithdrawalWriter(fam)
		for _, addPath := range []bool{false, true} {
			var ids [][]byte
			if addPath {
				ids = [][]byte{{0, 0, 0, 7}, {0, 0, 0, 8}, {0, 0, 0, 9}}
			} else {
				ids = [][]byte{nil, nil, nil}
			}
			for _, withdraw := range []bool{false, true} {
				name := fam.String()
				if addPath {
					name += "/add-path"
				}
				if withdraw {
					name += "/withdraw-framed"
				}
				t.Run(name, func(t *testing.T) {
					// A /22 announced with host bits set, a /24 behind three
					// labels, and a /0.
					depth := []int{1, 3, 1}
					if withdraw {
						depth = []int{0, 0, 0}
					}
					entries := [][]byte{
						labeledNLRI(ids[0], depth[0], 0x123456, famRD, 22, []byte{10, 1, 7}),
						labeledNLRI(ids[1], depth[1], 0x000001, famRD, 24, []byte{10, 2, 3}),
						labeledNLRI(ids[2], depth[2], 0x800000, famRD, 0, nil),
					}
					var field, want []byte
					head := 0
					if addPath {
						head = 4
					}
					for i, entry := range entries {
						field = append(field, entry...)
						scratch := make([]byte, PrefixKeyScratchSize)
						k, err := key(entry[head:], scratch, withdraw)
						if err != nil {
							t.Fatalf("entry %d key: %v", i, err)
						}
						one := make([]byte, len(k)+3)
						n, err := WithdrawalNLRIFromKey(fam, k, one)
						if err != nil {
							t.Fatalf("entry %d inverse: %v", i, err)
						}
						want = append(want, ids[i]...)
						want = append(want, one[:n]...)
					}
					dst := make([]byte, len(field))
					n, err := writer(dst, field, addPath, withdraw)
					if err != nil {
						t.Fatalf("writer: %v", err)
					}
					if !bytes.Equal(dst[:n], want) {
						t.Fatalf("writer:\n got %X\nwant %X", dst[:n], want)
					}
					if _, err := writer(make([]byte, n-1), field, addPath, withdraw); err == nil {
						t.Fatal("a destination one octet short was written")
					}
					allocs := testing.AllocsPerRun(100, func() {
						if _, err := GetWithdrawalWriter(fam)(dst, field, addPath, withdraw); err != nil {
							t.Fatal(err)
						}
					})
					if allocs != 0 {
						t.Fatalf("writer allocates %.0f times per call", allocs)
					}
				})
			}
		}
	}

	refused := map[string]struct {
		fam   family.Family
		field []byte
	}{
		"truncated entry":         {withdrawalWriterFamilies[0], []byte{56, 0, 0x10, 0x41, 10}},
		"no bottom-of-stack bit":  {withdrawalWriterFamilies[0], []byte{56, 0, 0x10, 0x40, 0, 0x11, 0x40, 10}},
		"fewer than 24 bits":      {withdrawalWriterFamilies[0], []byte{16, 0, 0x10}},
		"VPN under 64 bits after": {withdrawalWriterFamilies[2], []byte{56, 0, 0x10, 0x41, 0, 0, 0, 0}},
	}
	for name, tt := range refused {
		dst := make([]byte, 64)
		if _, err := GetWithdrawalWriter(tt.fam)(dst, tt.field, false, false); err == nil {
			t.Errorf("%s: %X was written", name, tt.field)
		}
		scratch := make([]byte, PrefixKeyScratchSize)
		if _, err := GetPrefixKey(tt.fam)(tt.field, scratch, false); err == nil {
			t.Errorf("%s: the route key accepts %X, so the writer refuses more than the key", name, tt.field)
		}
	}
}

// TestWithdrawalNLRIFromKey checks the per-key inverse answers for exactly the
// writer's families and gives the withdrawal framing that the family's own
// withdrawal key reads back to the same key. Method: key an announcement with
// one and with three labels, invert it, key the result as a withdrawal.
func TestWithdrawalNLRIFromKey(t *testing.T) {
	t.Parallel()
	mu.RLock()
	registered := make([]family.Family, 0, len(splitters))
	for fam := range splitters {
		registered = append(registered, fam)
	}
	mu.RUnlock()
	for _, fam := range registered {
		if slices.Contains(withdrawalWriterFamilies, fam) {
			continue
		}
		if _, err := WithdrawalNLRIFromKey(fam, []byte{8, 10}, make([]byte, 8)); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: inverse answers %v, want ErrUnsupported", fam, err)
		}
	}
	rd := []byte{0, 0, 0xfd, 0xe8, 0, 0, 0, 1}
	for _, fam := range withdrawalWriterFamilies {
		var famRD []byte
		if fam.SAFI == family.SAFIVPN {
			famRD = rd
		}
		for _, labels := range []int{1, 3} {
			announced := labeledNLRI(nil, labels, 0, famRD, 24, []byte{10, 2, 3})
			scratch := make([]byte, PrefixKeyScratchSize)
			key, err := GetPrefixKey(fam)(announced, scratch, false)
			if err != nil {
				t.Fatalf("%s/%d: %v", fam, labels, err)
			}
			key = bytes.Clone(key)
			dst := make([]byte, len(key)+3)
			n, err := WithdrawalNLRIFromKey(fam, key, dst)
			if err != nil {
				t.Fatalf("%s/%d: inverse: %v", fam, labels, err)
			}
			want := labeledNLRI(nil, 0, 0x800000, famRD, 24, []byte{10, 2, 3})
			if !bytes.Equal(dst[:n], want) {
				t.Fatalf("%s/%d: inverse\n got %X\nwant %X", fam, labels, dst[:n], want)
			}
			back, err := GetPrefixKey(fam)(dst[:n], make([]byte, PrefixKeyScratchSize), true)
			if err != nil {
				t.Fatalf("%s/%d: withdrawal key: %v", fam, labels, err)
			}
			if !bytes.Equal(back, key) {
				t.Fatalf("%s/%d: round trip key %X, want %X", fam, labels, back, key)
			}
			if _, err := WithdrawalNLRIFromKey(fam, key, make([]byte, n-1)); !errors.Is(err, errWithdrawalDestination) {
				t.Fatalf("%s/%d: a destination one octet short answered %v", fam, labels, err)
			}
		}
		// A key whose bits leave no room for the Compatibility field in the
		// one-octet length is refused as that, not as a short destination.
		// 231 bits plus the 24 of the field is 255, the largest length octet.
		edge := append([]byte{231}, make([]byte, 29)...)
		if _, err := WithdrawalNLRIFromKey(fam, edge, make([]byte, 64)); err != nil {
			t.Fatalf("%s: a 231-bit key answered %v, want it written", fam, err)
		}
		long := append([]byte{232}, make([]byte, 29)...)
		if _, err := WithdrawalNLRIFromKey(fam, long, make([]byte, 64)); !errors.Is(err, errWithdrawalLength) {
			t.Fatalf("%s: a 232-bit key answered %v, want the length refusal", fam, err)
		}
	}
}
