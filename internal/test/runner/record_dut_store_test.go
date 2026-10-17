package runner

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDUTStoreOption: a .ci with no option=storage hands its DUT the
// in-memory selector, option=storage:value=disk hands it nothing (the live
// store), value=memory is the default made explicit, and any other value is
// refused naming the two that work. Driven through parseAndAdd, the entry
// point discovery calls, then the env entry runTest and runOrchestrated add.
func TestDUTStoreOption(t *testing.T) {
	cases := []struct {
		name    string
		lines   []string
		wantEnv string
	}{
		{"default is memory", nil, dutStoreEnvMemory},
		{"disk opts out", []string{"option=storage:value=disk"}, ""},
		{"memory explicit", []string{"option=storage:value=memory"}, dutStoreEnvMemory},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines := append([]string{}, c.lines...)
			lines = append(lines, "cmd=foreground:seq=1:exec=true", "expect=exit:code=0")
			rec, err := parseCILines(t, lines...)
			require.NoError(t, err)
			require.Equal(t, c.wantEnv, rec.DUTStore.env())
		})
	}
}

// TestDUTStoreOptionRefusesUnknownValue: a typo is refused at parse time with
// the accepted values, rather than silently running the DUT on memory.
func TestDUTStoreOptionRefusesUnknownValue(t *testing.T) {
	_, err := parseCILines(t, "option=storage:value=dsik", "cmd=foreground:seq=1:exec=true", "expect=exit:code=0")
	require.Error(t, err)
	require.True(t, errors.Is(err, errOptionStorageValue), "err = %v", err)
}
