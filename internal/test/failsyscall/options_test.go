// VALIDATES: the keyword grammar refuses every shape that would arm a filter
// selecting something other than what the caller named.
//
// PREVENTS: a .ci file whose misspelled keyword is skipped, which would fail
// EVERY socket in the daemon while reporting that it armed correctly.

package failsyscall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOptionsReadsEveryKeyword(t *testing.T) {
	parsed, err := parseOptions([]string{
		"syscall", "recvfrom", "errno", "ENETDOWN", "length", "1500", "--", "ze", "-",
	})
	require.NoError(t, err)
	assert.Equal(t, "recvfrom", parsed.SyscallName)
	assert.Equal(t, "ENETDOWN", parsed.ErrnoName)
	assert.Equal(t, 1500, parsed.ReadLength)
	assert.Equal(t, []string{"ze", "-"}, parsed.Command)
}

func TestParseOptionsLeavesLengthUnsetWhenAbsent(t *testing.T) {
	parsed, err := parseOptions([]string{"syscall", "recvmsg", "errno", "ENETDOWN", "--", "ze"})
	require.NoError(t, err)
	assert.Equal(t, lengthUnset, parsed.ReadLength)
}

// TestParseOptionsAcceptsTheCeiling pins the boundary from the other side. A
// ceiling written one byte low would refuse a length the filter can carry, and
// only a case ON the limit tells that apart from a ceiling that is right.
func TestParseOptionsAcceptsTheCeiling(t *testing.T) {
	parsed, err := parseOptions([]string{"syscall", "recvfrom", "errno", "ENETDOWN", "length", "1048576", "--", "ze"})
	require.NoError(t, err)
	assert.Equal(t, readLengthMax, parsed.ReadLength)
}

func TestParseOptionsRefusesWhatWouldArmTheWrongFilter(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no separator", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "ze"}, "-- <command>"},
		{"no command", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "--"}, "-- <command>"},
		{"no syscall", []string{"errno", "ENETDOWN", "--", "ze"}, "`syscall <name>` is required"},
		{"no errno", []string{"syscall", "recvfrom", "--", "ze"}, "`errno <NAME>` is required"},
		{"misspelled keyword", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "len", "1500", "--", "ze"}, "unknown keyword len"},
		{"keyword with no value", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "length", "--", "ze"}, "keyword length has no value"},
		{"length is not a number", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "length", "big", "--", "ze"}, "length big is not a number"},
		{"length is negative", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "length", "-1", "--", "ze"}, "length -1 is negative"},
		{"length is above the ceiling", []string{"syscall", "recvfrom", "errno", "ENETDOWN", "length", "1048577", "--", "ze"}, "is above 1048576"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := parseOptions(one.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), one.want)
		})
	}
}
