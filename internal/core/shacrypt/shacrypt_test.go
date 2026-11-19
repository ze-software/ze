// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: shacrypt.go -- Parse and Hash.Matches, the producers these tests drive

package shacrypt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specVectors are the test vectors published with Ulrich Drepper's
// specification "Unix crypt using SHA-256 and SHA-512" (the tests[] tables of
// its sha256crypt.c and sha512crypt.c reference code). Each entry is the hash
// crypt(3) writes and the password it was made from. The specification feeds a
// salt setting such as "rounds=10$roundstoolow" and prints the clamped form; a
// stored hash is always that printed form, so the printed form is what Ze parses.
var specVectors = []struct {
	hash     string
	password string
}{
	{"$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5", "Hello world!"},
	{"$5$rounds=10000$saltstringsaltst$3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2.opqey6IcA", "Hello world!"},
	{"$5$rounds=5000$toolongsaltstrin$Un/5jzAHMgOGZ5.mWJpuVolil07guHPvOW8mGRcvxa5", "This is just a test"},
	{
		"$5$rounds=1400$anotherlongsalts$Rx.j8H.h8HjEDGomFU8bDkXm3XIUnzyxf12oP84Bnq1",
		"a very much longer text to encrypt.  This one even stretches over morethan one line.",
	},
	{"$5$rounds=77777$short$JiO1O3ZpDAxGJeaDIuqCoEFysAe1mZNJRs3pw0KQRd/", "we have a short salt string but not a short password"},
	{"$5$rounds=123456$asaltof16chars..$gP3VQ/6X7UUEW3HkBn2w1/Ptq2jxPyzV/cZKmF/wJvD", "a short string"},
	{"$5$rounds=1000$roundstoolow$yfvwcWrQ8l/K0DAWyuPMDNHpIVlTQebY9l/gL972bIC", "the minimum number is still observed"},
	{
		"$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1",
		"Hello world!",
	},
	{
		"$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v.",
		"Hello world!",
	},
	{
		"$6$rounds=5000$toolongsaltstrin$lQ8jolhgVRVhY4b5pZKaysCLi0QBxGoNeKQzQ3glMhwllF7oGDZxUhx1yxdYcz/e1JSbq3y6JMxxl8audkUEm0",
		"This is just a test",
	},
	{
		"$6$rounds=1400$anotherlongsalts$POfYwTEok97VWcjxIiSOjiykti.o/pQs.wPvMxQ6Fm7I6IoYN3CmLs66x9t0oSwbtEW7o7UmJEiDwGqd8p4ur1",
		"a very much longer text to encrypt.  This one even stretches over morethan one line.",
	},
	{
		"$6$rounds=77777$short$WuQyW2YR.hBNpjjRhpYD/ifIw05xdfeEyQoMxIXbkvr0gge1a1x3yRULJ5CCaUeOxFmtlcGZelFl5CxtgfiAc0",
		"we have a short salt string but not a short password",
	},
	{
		"$6$rounds=123456$asaltof16chars..$BtCwjqMJGx5hrJhZywWvt0RLE8uZ4oPwcelCjmw2kSYu.Ec6ycULevoBK25fs2xXgMNrCzIMVcgEJAstJeonj1",
		"a short string",
	},
	{
		"$6$rounds=1000$roundstoolow$kUMsbe306n21p9R.FRkW3IGn.S9NPN0x50YhH1xhLsPuWGsUSklZt58jaTfF4ZEQpyUNGc0dqbpBYYBaHHrsX.",
		"the minimum number is still observed",
	},
}

// TestSpecVectorsMatch: every published vector verifies with its own password.
//
// VALIDATES: the SHA-256 and SHA-512 computations, the default and explicit
// rounds, the 16-character salt and the final byte permutation, against the
// specification's own answers.
// PREVENTS: an algorithm that verifies hashes it wrote itself but refuses every
// hash another system wrote, which would lock out every migrated account.
func TestSpecVectorsMatch(t *testing.T) {
	for _, vector := range specVectors {
		parsed, err := Parse(vector.hash)
		require.NoError(t, err, "Parse(%q)", vector.hash)
		assert.True(t, parsed.Matches([]byte(vector.password)), "Matches for %q", vector.hash)
	}
}

// TestSpecVectorsRefuseWrongPassword: a vector refuses a password that differs
// from its own by one byte, by a missing byte, or entirely.
//
// VALIDATES: Matches compares the digest, so the positive test above cannot be
// passed by code that accepts everything.
func TestSpecVectorsRefuseWrongPassword(t *testing.T) {
	for _, vector := range specVectors {
		parsed, err := Parse(vector.hash)
		require.NoError(t, err)
		changed := []byte(vector.password)
		changed[0] ^= 0x01
		assert.False(t, parsed.Matches(changed), "one changed byte must not match %q", vector.hash)
		assert.False(t, parsed.Matches([]byte(vector.password[1:])), "a shorter password must not match %q", vector.hash)
		assert.False(t, parsed.Matches(nil), "an empty password must not match %q", vector.hash)
	}
}

// TestDigestOfAnotherHashRefused: the salt and the rounds take part in the
// digest, so a digest moved onto another salt or rounds value matches nothing.
func TestDigestOfAnotherHashRefused(t *testing.T) {
	// The digest of "$6$saltstring$..." under a salt one character different.
	moved := "$6$saltstrinG$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	parsed, err := Parse(moved)
	require.NoError(t, err)
	assert.False(t, parsed.Matches([]byte("Hello world!")), "the salt is part of the digest")

	// The digest of the default 5000 rounds, presented as 5001 rounds.
	rounds := "$6$rounds=5001$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	parsed, err = Parse(rounds)
	require.NoError(t, err)
	assert.False(t, parsed.Matches([]byte("Hello world!")), "the rounds are part of the digest")
}

// TestExplicitDefaultRoundsEqualsImplicit: "rounds=5000$" names the default, so
// it gives the same digest as no rounds field at all.
func TestExplicitDefaultRoundsEqualsImplicit(t *testing.T) {
	explicit := "$6$rounds=5000$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	parsed, err := Parse(explicit)
	require.NoError(t, err)
	assert.True(t, parsed.Matches([]byte("Hello world!")))
}

// TestParseRefusesMalformed: a value that crypt(3) can never have written is
// refused at Parse, with the reason, rather than parsed into a hash that
// matches nothing.
//
// VALIDATES: the shape bounds of the specification. Rounds lie in 1000 to
// 999999999 (crypt(3) prints the clamped value, so a stored hash outside that
// range was not written by it), the salt holds at most 16 characters, and the
// digest has exactly 43 (SHA-256) or 86 (SHA-512) characters of the crypt
// alphabet.
func TestParseRefusesMalformed(t *testing.T) {
	digest512 := strings.Repeat("a", 86)
	digest256 := strings.Repeat("a", 43)
	cases := []struct {
		name  string
		input string
	}{
		{"rounds below the minimum", "$6$rounds=999$salt$" + digest512},
		{"rounds above the maximum", "$6$rounds=1000000000$salt$" + digest512},
		{"rounds with a leading zero", "$6$rounds=01000$salt$" + digest512},
		{"rounds not a number", "$6$rounds=abc$salt$" + digest512},
		{"rounds empty", "$6$rounds=$salt$" + digest512},
		{"rounds with a sign", "$6$rounds=+5000$salt$" + digest512},
		{"rounds with no salt after it", "$6$rounds=5000"},
		{"salt longer than 16", "$6$" + strings.Repeat("s", 17) + "$" + digest512},
		{"no digest separator", "$6$saltonly"},
		{"SHA-512 digest too short", "$6$salt$" + digest256},
		{"SHA-512 digest too long", "$6$salt$" + digest512 + "a"},
		{"SHA-256 digest too long", "$5$salt$" + digest512},
		{"digest outside the alphabet", "$6$salt$" + strings.Repeat("a", 85) + "-"},
		{"digest with a trailing separator", "$6$salt$" + digest512 + "$"},
	}
	for _, c := range cases {
		_, err := Parse(c.input)
		assert.Error(t, err, c.name)
		assert.NotErrorIs(t, err, ErrNotSHACrypt, "%s is a SHA-crypt hash that is malformed", c.name)
		assert.False(t, IsHash(c.input), c.name)
	}
}

// TestParseRefusesOtherFormats: a value that is not a SHA-crypt hash at all
// answers ErrNotSHACrypt, so a caller can try the next format.
func TestParseRefusesOtherFormats(t *testing.T) {
	for _, input := range []string{
		"",
		"plaintext",
		"$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
		"$1$salt$abcdefghijklmnopqrstuv",
		"$7$salt$" + strings.Repeat("a", 86),
		"6$salt$" + strings.Repeat("a", 86),
	} {
		_, err := Parse(input)
		require.ErrorIs(t, err, ErrNotSHACrypt, "Parse(%q)", input)
		assert.False(t, IsHash(input), "IsHash(%q)", input)
	}
}

// TestIsHashAcceptsEveryVector: the shape check agrees with Parse on every
// published hash, the ones at the salt limit and the rounds minimum included.
func TestIsHashAcceptsEveryVector(t *testing.T) {
	for _, vector := range specVectors {
		assert.True(t, IsHash(vector.hash), "IsHash(%q)", vector.hash)
	}
	assert.True(t, IsHash("$6$rounds=999999999$s$"+strings.Repeat("a", 86)), "the maximum rounds is accepted")
	assert.True(t, IsHash("$6$$"+strings.Repeat("a", 86)), "an empty salt is accepted")
}

// TestZeroHashMatchesNothing: a Hash that Parse did not build matches no
// password, so a caller that skipped Parse fails closed.
func TestZeroHashMatchesNothing(t *testing.T) {
	var zero Hash
	assert.False(t, zero.Matches([]byte("")))
	assert.False(t, zero.Matches([]byte("Hello world!")))
}

// TestOverlongPasswordRefused: a password over the length bound matches
// nothing. The specification hashes the password once for each of its bytes,
// so the work grows with the square of its length, and a login attempt must not
// be able to choose that cost.
func TestOverlongPasswordRefused(t *testing.T) {
	parsed, err := Parse(specVectors[7].hash)
	require.NoError(t, err)
	assert.False(t, parsed.Matches([]byte(strings.Repeat("x", passwordLenMax+1))))
}
