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
		hasBlock, err := statementHasBlock(moduleSource{module: &yang.Module{Name: "m"}, text: text, parsed: true}, statement)
		require.NoError(t, err, tc.statement)
		assert.Equal(t, tc.hasBlock, hasBlock, tc.statement)
	}
	statements, err := yang.Parse("module m { refine x; }", "m.yang")
	require.NoError(t, err)
	_, err = statementHasBlock(moduleSource{module: &yang.Module{Name: "m"}}, statements[0].SubStatements()[0])
	assert.Error(t, err, "a module whose text the loader never parsed is no answer")
}

// TestBlockIsReadFromTheTextTheModuleWasParsedFrom loads two module texts
// under one file name, then resolves. The first holds the statement whose
// block is asked about; the second is a different module, so goyang accepts
// both. The block must be read from the text the statement was parsed from,
// never from whichever text last carried the name.
//
// VALIDATES: sourcedModules.Parse binds each parsed module to its own text,
// so statementHasBlock cannot read another parse's text.
func TestBlockIsReadFromTheTextTheModuleWasParsedFrom(t *testing.T) {
	t.Run("valid statement beyond the end of the later text", func(t *testing.T) {
		loader := NewLoader()
		require.NoError(t, loader.AddModuleFromText("same.yang",
			`module a { namespace "urn:a"; prefix a; extension e { argument text; } `+
				`leaf x { type string; a:e "v" { refine "q" { } } } }`))
		require.NoError(t, loader.AddModuleFromText("same.yang",
			`module b { namespace "urn:b"; prefix b; leaf y { type string; } }`))
		_, resolveErr := loader.Resolve()
		assert.NoError(t, resolveErr, "refine with an empty block conforms")
	})
	t.Run("invalid statement where the later text has a block", func(t *testing.T) {
		loader := NewLoader()
		require.NoError(t, loader.AddModuleFromText("same.yang",
			`module a { namespace "urn:a"; prefix a; extension e { argument text; } `+
				`leaf x { type string; a:e "v" { refine "q"; } } }`))
		require.NoError(t, loader.AddModuleFromText("same.yang",
			`module b { namespace "urn:b"; prefix b; extension e { argument text; } `+
				`leaf x { type string; b:e "v" { refine "q" { } } } }`))
		_, err := loader.Resolve()
		require.Error(t, err, "refine with no block breaks refine-stmt")
		assert.Contains(t, err.Error(), "module a")
		assert.Contains(t, err.Error(), "requires a block")
	})
}
