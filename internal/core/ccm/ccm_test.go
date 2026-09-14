// VALIDATES: the CCM mode against the twenty-four packet vectors published in Section 8
// of RFC 3610, in both directions, plus the parameter refusals of Section 2 and the
// length-field bound of Section 2.1.
// PREVENTS: a CBC-MAC that pads or flags a block wrongly, a counter block whose flags
// or counter field sit at the wrong offset, an authentication field XORed with the
// wrong keystream block, and an Open that answers plaintext for a tag that fails.
package ccm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"testing"
)

// packetVector is one row of RFC 3610 Section 8. Each vector's input carries eight
// cleartext header octets, which are the additional data, followed by the message. The
// published output repeats those octets, so want holds the part after them: the
// ciphertext with its authentication field.
type packetVector struct {
	num       int
	key       string
	nonce     string
	aad       string
	plaintext string
	want      string
	tagOctets int
}

// rfc3610Vectors are the twenty-four packet vectors of RFC 3610 Section 8. Every one
// uses a thirteen octet nonce, so L is 2, and the tag is eight or ten octets.
var rfc3610Vectors = []packetVector{
	{num: 1, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000003020100a0a1a2a3a4a5", aad: "0001020304050607", plaintext: "08090a0b0c0d0e0f101112131415161718191a1b1c1d1e", want: "588c979a61c663d2f066d0c2c0f989806d5f6b61dac38417e8d12cfdf926e0", tagOctets: 8},               // nonce 13
	{num: 2, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000004030201a0a1a2a3a4a5", aad: "0001020304050607", plaintext: "08090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", want: "72c91a36e135f8cf291ca894085c87e3cc15c439c9e43a3ba091d56e10400916", tagOctets: 8},           // nonce 13
	{num: 3, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000005040302a0a1a2a3a4a5", aad: "0001020304050607", plaintext: "08090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20", want: "51b1e5f44a197d1da46b0f8e2d282ae871e838bb64da8596574adaa76fbd9fb0c5", tagOctets: 8},       // nonce 13
	{num: 4, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000006050403a0a1a2a3a4a5", aad: "000102030405060708090a0b", plaintext: "0c0d0e0f101112131415161718191a1b1c1d1e", want: "a28c6865939a9a79faaa5c4c2a9d4a91cdac8c96c861b9c9e61ef1", tagOctets: 8},                       // nonce 13
	{num: 5, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000007060504a0a1a2a3a4a5", aad: "000102030405060708090a0b", plaintext: "0c0d0e0f101112131415161718191a1b1c1d1e1f", want: "dcf1fb7b5d9e23fb9d4e131253658ad86ebdca3e51e83f077d9c2d93", tagOctets: 8},                   // nonce 13
	{num: 6, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000008070605a0a1a2a3a4a5", aad: "000102030405060708090a0b", plaintext: "0c0d0e0f101112131415161718191a1b1c1d1e1f20", want: "6fc1b011f006568b5171a42d953d469b2570a4bd87405a0443ac91cb94", tagOctets: 8},               // nonce 13
	{num: 7, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "00000009080706a0a1a2a3a4a5", aad: "0001020304050607", plaintext: "08090a0b0c0d0e0f101112131415161718191a1b1c1d1e", want: "0135d1b2c95f41d5d1d4fec185d166b8094e999dfed96c048c56602c97acbb7490", tagOctets: 10},          // nonce 13
	{num: 8, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "0000000a090807a0a1a2a3a4a5", aad: "0001020304050607", plaintext: "08090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", want: "7b75399ac0831dd2f0bbd75879a2fd8f6cae6b6cd9b7db24c17b4433f434963f34b4", tagOctets: 10},      // nonce 13
	{num: 9, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "0000000b0a0908a0a1a2a3a4a5", aad: "0001020304050607", plaintext: "08090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20", want: "82531a60cc24945a4b8279181ab5c84df21ce7f9b73f42e197ea9c07e56b5eb17e5f4e", tagOctets: 10},  // nonce 13
	{num: 10, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "0000000c0b0a09a0a1a2a3a4a5", aad: "000102030405060708090a0b", plaintext: "0c0d0e0f101112131415161718191a1b1c1d1e", want: "07342594157785152b074098330abb141b947b566aa9406b4d999988dd", tagOctets: 10},                 // nonce 13
	{num: 11, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "0000000d0c0b0aa0a1a2a3a4a5", aad: "000102030405060708090a0b", plaintext: "0c0d0e0f101112131415161718191a1b1c1d1e1f", want: "676bb20380b0e301e8ab79590a396da78b834934f53aa2e9107a8b6c022c", tagOctets: 10},             // nonce 13
	{num: 12, key: "c0c1c2c3c4c5c6c7c8c9cacbcccdcecf", nonce: "0000000e0d0c0ba0a1a2a3a4a5", aad: "000102030405060708090a0b", plaintext: "0c0d0e0f101112131415161718191a1b1c1d1e1f20", want: "c0ffa0d6f05bdb67f24d43a4338d2aa4bed7b20e43cd1aa31662e7ad65d6db", tagOctets: 10},         // nonce 13
	{num: 13, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "00412b4ea9cdbe3c9696766cfa", aad: "0be1a88bace018b1", plaintext: "08e8cf97d820ea258460e96ad9cf5289054d895ceac47c", want: "4cb97f86a2a4689a877947ab8091ef5386a6ffbdd080f8e78cf7cb0cddd7b3", tagOctets: 8},              // nonce 13
	{num: 14, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "0033568ef7b2633c9696766cfa", aad: "63018f76dc8a1bcb", plaintext: "9020ea6f91bdd85afa0039ba4baff9bfb79c7028949cd0ec", want: "4ccb1e7ca981befaa0726c55d378061298c85c92814abc33c52ee81d7d77c08a", tagOctets: 8},          // nonce 13
	{num: 15, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "00103fe41336713c9696766cfa", aad: "aa6cfa36cae86b40", plaintext: "b916e0eacc1c00d7dcec68ec0b3bbb1a02de8a2d1aa346132e", want: "b1d23a2220ddc0ac900d9aa03c61fcf4a559a4417767089708a776796edb723506", tagOctets: 8},      // nonce 13
	{num: 16, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "00764c63b8058e3c9696766cfa", aad: "d0d0735c531e1becf049c244", plaintext: "12daac5630efa5396f770ce1a66b21f7b2101c", want: "14d253c3967b70609b7cbb7c499160283245269a6f49975bcadeaf", tagOctets: 8},                      // nonce 13
	{num: 17, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "00f8b678094e3b3c9696766cfa", aad: "77b60f011c03e1525899bcae", plaintext: "e88b6a46c78d63e52eb8c546efb5de6f75e9cc0d", want: "5545ff1a085ee2efbf52b2e04bee1e2336c73e3f762c0c7744fe7e3c", tagOctets: 8},                  // nonce 13
	{num: 18, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "00d560912d3f703c9696766cfa", aad: "cd9044d2b71fdb8120ea60c0", plaintext: "6435acbafb11a82e2f071d7ca4a5ebd93a803ba87f", want: "009769ecabdf48625594c59251e6035722675e04c847099e5ae0704551", tagOctets: 8},              // nonce 13
	{num: 19, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "0042fff8f1951c3c9696766cfa", aad: "d85bc7e69f944fb8", plaintext: "8a19b950bcf71a018e5e6701c91787659809d67dbedd18", want: "bc218daa947427b6db386a99ac1aef23ade0b52939cb6a637cf9bec2408897c6ba", tagOctets: 10},         // nonce 13
	{num: 20, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "00920f40e56cdc3c9696766cfa", aad: "74a0ebc9069f5b37", plaintext: "1761433c37c5a35fc1f39f406302eb907c6163be38c98437", want: "5810e6fd25874022e80361a478e3e9cf484ab04f447efff6f0a477cc2fc9bf548944", tagOctets: 10},     // nonce 13
	{num: 21, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "0027ca0c7120bc3c9696766cfa", aad: "44a3aa3aae6475ca", plaintext: "a434a8e58500c6e41530538862d686ea9e81301b5ae4226bfa", want: "f2beed7bc5098e83feb5b31608f8e29c38819a89c8e776f1544d4151a4ed3a8b87b9ce", tagOctets: 10}, // nonce 13
	{num: 22, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "005b8ccbcd9af83c9696766cfa", aad: "ec46bb63b02520c33c49fd70", plaintext: "b96b49e21d621741632875db7f6c9243d2d7c2", want: "31d750a09da3ed7fddd49a2032aabf17ec8ebf7d22c8088c666be5c197", tagOctets: 10},                 // nonce 13
	{num: 23, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "003ebe94044b9a3c9696766cfa", aad: "47a65ac78b3d594227e85e71", plaintext: "e2fcfbb880442c731bf95167c8ffd7895e337076", want: "e882f1dbd38ce3eda7c23f04dd65071eb41342acdf7e00dccec7ae52987d", tagOctets: 10},             // nonce 13
	{num: 24, key: "d7828d13b2b0bdc325a76236df93cc6b", nonce: "008d493b30ae8b3c9696766cfa", aad: "6e37a6ef546d955d34ab6059", plaintext: "abf21c0b02feb88f856df4a37381bce3cc128517d4", want: "f32905b88a641b04b9c9ffb58cc390900f3da12ab16dce9e82efa16da62059", tagOctets: 10},         // nonce 13
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return b
}

func vectorAEAD(t *testing.T, v packetVector) (aead cipher.AEAD, nonce, aad, plaintext, want []byte) {
	t.Helper()
	block, err := aes.NewCipher(mustHex(t, v.key))
	if err != nil {
		t.Fatalf("vector %d: aes.NewCipher: %v", v.num, err)
	}
	nonce = mustHex(t, v.nonce)
	built, err := New(block, v.tagOctets, len(nonce))
	if err != nil {
		t.Fatalf("vector %d: New with a %d octet tag and a %d octet nonce: %v",
			v.num, v.tagOctets, len(nonce), err)
	}
	return built, nonce, mustHex(t, v.aad), mustHex(t, v.plaintext), mustHex(t, v.want)
}

// TestSealMatchesRFC3610Vectors seals each published vector and compares the whole
// output, so both the CTR keystream and the encrypted authentication field are pinned.
func TestSealMatchesRFC3610Vectors(t *testing.T) {
	for _, v := range rfc3610Vectors {
		aead, nonce, aad, plaintext, want := vectorAEAD(t, v)
		got := aead.Seal(nil, nonce, plaintext, aad)
		if !bytes.Equal(got, want) {
			t.Errorf("vector %d: Seal = %x, want %x", v.num, got, want)
		}
	}
}

// TestOpenMatchesRFC3610Vectors opens each published output and compares the recovered
// message, which proves the MAC is recomputed over the plaintext rather than carried.
func TestOpenMatchesRFC3610Vectors(t *testing.T) {
	for _, v := range rfc3610Vectors {
		aead, nonce, aad, plaintext, want := vectorAEAD(t, v)
		got, err := aead.Open(nil, nonce, want, aad)
		if err != nil {
			t.Errorf("vector %d: Open = %v, want the message", v.num, err)
			continue
		}
		if !bytes.Equal(got, plaintext) {
			t.Errorf("vector %d: Open = %x, want %x", v.num, got, plaintext)
		}
	}
}

// TestOpenRefusesEveryAlteredOctet flips one bit in each region a vector has: the
// message, the authentication field and the additional data. Each is covered by the MAC
// of RFC 3610 Section 2.2, so each must be refused.
func TestOpenRefusesEveryAlteredOctet(t *testing.T) {
	v := rfc3610Vectors[0]
	aead, nonce, aad, _, want := vectorAEAD(t, v)

	for _, tc := range []struct {
		region string
		index  int
	}{
		{"message", 0},
		{"authentication field", len(want) - 1},
	} {
		damaged := bytes.Clone(want)
		damaged[tc.index] ^= 0x01
		if _, err := aead.Open(nil, nonce, damaged, aad); err == nil {
			t.Errorf("a flipped bit in the %s was accepted, want ErrOpen", tc.region)
		}
	}

	damagedAAD := bytes.Clone(aad)
	damagedAAD[0] ^= 0x01
	if _, err := aead.Open(nil, nonce, want, damagedAAD); err == nil {
		t.Error("a flipped bit in the additional data was accepted, want ErrOpen")
	}
}

// TestOpenRevealsNothingForABadTag holds the rule of RFC 3610 Section 2.5: a failed
// authentication answers no plaintext at all, not a short or partial one.
func TestOpenRevealsNothingForABadTag(t *testing.T) {
	v := rfc3610Vectors[0]
	aead, nonce, aad, _, want := vectorAEAD(t, v)
	damaged := bytes.Clone(want)
	damaged[len(damaged)-1] ^= 0xFF
	got, err := aead.Open(nil, nonce, damaged, aad)
	if err == nil {
		t.Fatal("Open accepted a damaged authentication field, want ErrOpen")
	}
	if got != nil {
		t.Errorf("Open answered %x beside the error, want nothing", got)
	}
}

// TestRoundTripAtEveryPermittedTagLength covers the tag lengths RFC 3610 Section 2
// allows but Section 8 does not publish a vector for, at the eleven octet nonce IPsec
// uses, and includes the empty message and empty additional data.
func TestRoundTripAtEveryPermittedTagLength(t *testing.T) {
	block, err := aes.NewCipher(bytes.Repeat([]byte{0x2b}, 16))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	nonce := bytes.Repeat([]byte{0x17}, 11)

	for _, tagOctets := range []int{4, 6, 8, 10, 12, 14, 16} {
		aead, err := New(block, tagOctets, len(nonce))
		if err != nil {
			t.Fatalf("New with a %d octet tag: %v", tagOctets, err)
		}
		for _, message := range [][]byte{nil, []byte("a"), bytes.Repeat([]byte{0x5a}, 64)} {
			for _, aad := range [][]byte{nil, []byte("associated")} {
				sealed := aead.Seal(nil, nonce, message, aad)
				if len(sealed) != len(message)+tagOctets {
					t.Fatalf("tag %d: Seal answered %d octets, want %d",
						tagOctets, len(sealed), len(message)+tagOctets)
				}
				got, err := aead.Open(nil, nonce, sealed, aad)
				if err != nil {
					t.Fatalf("tag %d: Open: %v", tagOctets, err)
				}
				if !bytes.Equal(got, message) {
					t.Fatalf("tag %d: Open = %x, want %x", tagOctets, got, message)
				}
			}
		}
	}
}

// TestSealAppendsToItsDestination proves Seal extends the slice it is given rather than
// overwriting it, which is what lets a caller put the IV in front of the ciphertext.
func TestSealAppendsToItsDestination(t *testing.T) {
	block, err := aes.NewCipher(bytes.Repeat([]byte{0x2b}, 16))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	nonce := bytes.Repeat([]byte{0x17}, 11)
	aead, err := New(block, 16, len(nonce))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	prefix := []byte("IV......")
	got := aead.Seal(prefix, nonce, []byte("message"), nil)
	if !bytes.HasPrefix(got, []byte("IV......")) {
		t.Errorf("Seal answered %x, want it to start with the destination it was given", got)
	}
	if len(got) != len(prefix)+len("message")+16 {
		t.Errorf("Seal answered %d octets, want %d", len(got), len(prefix)+len("message")+16)
	}
}

// TestNewRefusesAParameterRFC3610DoesNotAllow drives the refusals of Section 2. Every
// one fails closed: New answers an error and no mode, so nothing can be keyed at a
// length the peer does not use.
func TestNewRefusesAParameterRFC3610DoesNotAllow(t *testing.T) {
	block, err := aes.NewCipher(bytes.Repeat([]byte{0x2b}, 16))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	for _, tc := range []struct {
		name        string
		tagOctets   int
		nonceOctets int
		want        error
	}{
		{"a two octet tag", 2, 11, ErrTagOctets},
		{"an odd tag", 9, 11, ErrTagOctets},
		{"an eighteen octet tag", 18, 11, ErrTagOctets},
		{"a six octet nonce", 16, 6, ErrNonceOctets},
		{"a fourteen octet nonce", 16, 14, ErrNonceOctets},
	} {
		aead, err := New(block, tc.tagOctets, tc.nonceOctets)
		if err != tc.want {
			t.Errorf("New with %s = %v, want %v", tc.name, err, tc.want)
		}
		if aead != nil {
			t.Errorf("New with %s answered a mode beside the error", tc.name)
		}
	}
}

// TestOpenRefusesDataShorterThanItsTag covers the boundary where the authentication
// field cannot be split off at all.
func TestOpenRefusesDataShorterThanItsTag(t *testing.T) {
	block, err := aes.NewCipher(bytes.Repeat([]byte{0x2b}, 16))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	nonce := bytes.Repeat([]byte{0x17}, 11)
	aead, err := New(block, 16, len(nonce))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := aead.Open(nil, nonce, bytes.Repeat([]byte{0x00}, 15), nil); err != ErrOpen {
		t.Errorf("Open over fifteen octets under a sixteen octet tag = %v, want ErrOpen", err)
	}
}

// TestAdditionalDataLengthEncodingsMatchRFC3610 drives the three forms of Section 2.2
// at their boundaries. The long forms are checked on the encoder rather than through a
// four gigabyte message.
func TestAdditionalDataLengthEncodingsMatchRFC3610(t *testing.T) {
	for _, tc := range []struct {
		octets int
		want   string
	}{
		{1, "0001"},
		{0xFEFF, "feff"},
		{0xFF00, "fffe0000ff00"},
		{0xFFFFFFFF, "fffeffffffff"},
	} {
		var prefix [10]byte
		if got := hex.EncodeToString(additionalDataLength(&prefix, tc.octets)); got != tc.want {
			t.Errorf("additionalDataLength(%d) = %s, want %s", tc.octets, got, tc.want)
		}
	}
}
