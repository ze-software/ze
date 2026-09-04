// Detail: inject.go -- decodeCommandList, the retired summary key

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecodeCommandListRefusesTheRetiredKey drives the guard from the function
// the CLI client reaches with the daemon's answer. `help` named the summary on
// this wire until 2026-09-03. A row that still carries it decodes with an EMPTY
// Description, which no reader can tell from a command that states no summary,
// so every command would render with a blank row and nothing would say why.
//
// The daemon and the CLI ship together, so a retired key means the two disagree
// about the wire. Reporting that beats rendering the whole tree blank.
//
// VALIDATES: AC-10 at the CLI end of the same wire.
// PREVENTS: a silent blank summary on every command in the completion menu.
func TestDecodeCommandListRefusesTheRetiredKey(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{name: "retired key with a value", payload: `{"commands":[{"value":"show env get","help":"Read one environment value"}]}`},
		{name: "retired key with an empty value", payload: `{"commands":[{"value":"show env get","help":""}]}`},
		{name: "retired key on a later row", payload: `{"commands":[{"value":"show a","description":"A."},{"value":"show env get","help":"B."}]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := decodeCommandList([]byte(tc.payload))
			require.Error(t, err, "the retired key was accepted")
			assert.Nil(t, rows, "a refused answer yields no rows")
			assert.Contains(t, err.Error(), retiredSummaryKey, "the refusal must name the retired key")
			assert.Contains(t, err.Error(), summaryKey, "the refusal must name its replacement")
			assert.Contains(t, err.Error(), "show env get", "the refusal must name the offending row")
		})
	}
}

// TestDecodeCommandListReadsBothTexts is the negative half. Without it a
// decoder that refused every answer would pass the test above, and the two
// texts have to survive the decode for the menu and the ? box to differ.
func TestDecodeCommandListReadsBothTexts(t *testing.T) {
	const payload = `{"commands":[
		{"value":"show env get","description":"Read one environment value.","long-help":"The explanation."},
		{"value":"show env set","description":"Write one environment value."}
	]}`

	rows, err := decodeCommandList([]byte(payload))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "Read one environment value.", rows[0].Description)
	assert.Equal(t, "The explanation.", rows[0].LongHelp)
	assert.Nil(t, rows[0].RetiredHelp)

	assert.Equal(t, "Write one environment value.", rows[1].Description)
	assert.Empty(t, rows[1].LongHelp, "a row that declares no explanation carries none")
}
