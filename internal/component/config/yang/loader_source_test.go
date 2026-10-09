package yang

import (
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatementHasBlockReadsTheModuleText parses module texts whose last
// substatement closes with ";" or with an empty "{}", behind each argument
// form RFC 7950 Section 6.1.3 allows (none, unquoted, double-quoted with an
// escaped quote, single-quoted, and quoted strings joined by "+") and behind
// comments, and requires statementHasBlock to tell the two closings apart.
//
// VALIDATES: statementHasBlock, which goyang's Statement cannot answer.
func TestStatementHasBlockReadsTheModuleText(t *testing.T) {
	cases := []struct {
		statement string
		hasBlock  bool
	}{
		{`input;`, false},
		{`input {}`, true},
		{`refine x;`, false},
		{"refine x\t{ }", true},
		{`refine "a \" ; {";`, false},
		{`refine 'a {' {}`, true},
		{`refine "a" + 'b' /* ; */ + "c" // ;` + "\n" + `{}`, true},
		{"refine \"é\"\n;", false},
	}
	for _, tc := range cases {
		text := "module m { é; " + tc.statement + " }"
		statements, err := yang.Parse(text, "m.yang")
		require.NoError(t, err, tc.statement)
		subs := statements[0].SubStatements()
		statement := subs[len(subs)-1]
		hasBlock, err := statementHasBlock(map[string]string{"m.yang": text}, statement)
		require.NoError(t, err, tc.statement)
		assert.Equal(t, tc.hasBlock, hasBlock, tc.statement)
	}
	statements, err := yang.Parse("module m { refine x; }", "m.yang")
	require.NoError(t, err)
	_, err = statementHasBlock(map[string]string{}, statements[0].SubStatements()[0])
	assert.Error(t, err, "a text never recorded is no answer")
}
