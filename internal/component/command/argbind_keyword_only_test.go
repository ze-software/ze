package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validationShapedDefs answers the leaves of a made-up command that reads a
// file or the bytes of one: a mandatory file and an optional keyword-only
// content whose length refuses a short word. No shipped command has this
// shape (show config validation declares the file leaf alone); it is the case
// where a stray word after the file would otherwise fill the payload.
func validationShapedDefs() []ArgDef {
	return []ArgDef{
		mustArgDef(NewStringArg("file", []UintRange{{Min: 1, Max: 64}}, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("content", []UintRange{{Min: 4, Max: 64}}, nil, ArgOptions{KeywordOnly: true})),
	}
}

// writeShapedDefs answers the leaves of a command shaped like update data key:
// a mandatory key and a mandatory keyword-only content.
func writeShapedDefs() []ArgDef {
	return []ArgDef{
		mustArgDef(NewStringArg("key", []UintRange{{Min: 1, Max: 64}}, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("content", []UintRange{{Min: 4, Max: 64}}, nil, ArgOptions{Mandatory: true, KeywordOnly: true})),
	}
}

// TestKeywordOnlyLeafTakesNoStrayWord proves a word no keyword introduced is
// never the value of a ze:keyword-only leaf.
//
// GOAL: `file a b` leaves b to the handler, which counts two files, whether or
// not the content leaf would accept b as a value.
// METHOD: judge the line with a stray word the content leaf accepts and one it
// refuses, and read what the binding answers for content.
// VALIDATES: the keyword-only skip of positionalDef and of the open count
// (awaitsPositional).
// PREVENTS: a second file name stored as the content, or refused as a bad
// content the operator never typed.
func TestKeywordOnlyLeafTakesNoStrayWord(t *testing.T) {
	for _, stray := range []string{"second.conf", "b"} {
		t.Run(stray, func(t *testing.T) {
			validated, err := ValidateArgs([]string{"file", "a", stray}, validationShapedDefs(), nil)
			require.NoError(t, err, "the stray word is the handler's to judge, not a content fault")
			_, found := validated.Value("content")
			assert.False(t, found, "the stray word was bound to the keyword-only leaf")
			assert.Equal(t, []string{stray}, validated.PositionalTokens())
		})
	}
}

// TestKeywordOnlyLeafIsNotBlamedForAStrayWord proves a stray word beside a
// keyword-only leaf is refused by the leaf that was open to it.
//
// GOAL: with a count leaf still open, a word that is no number is refused as a
// bad count, and the refusal says nothing about content.
// METHOD: judge one stray word against an optional count and a keyword-only
// content that would accept the word.
// VALIDATES: positionalDef, awaitsPositional, positionalError.
// PREVENTS: the looser keyword-only leaf swallowing a word typed for another
// leaf.
func TestKeywordOnlyLeafIsNotBlamedForAStrayWord(t *testing.T) {
	defs := []ArgDef{
		mustArgDef(NewUintArg("count", 8, nil, ArgOptions{})),
		mustArgDef(NewStringArg("content", nil, nil, ArgOptions{KeywordOnly: true})),
	}
	_, err := ValidateArgs([]string{"stray"}, defs, nil)
	require.Error(t, err, "the stray word was accepted, so a keyword-only leaf took it")
	assert.Contains(t, err.Error(), "unsigned integer", "the refusal is the count leaf's own")
	assert.NotContains(t, err.Error(), "content")
}

// TestKeywordOnlyLeafBindsAfterItsKeyword proves the keyed form still binds
// and is still judged.
//
// VALIDATES: phase 1 of ValidateArgs is unchanged for a keyword-only leaf.
// PREVENTS: a keyword-only leaf nobody can fill, or one whose value escapes
// its declared length.
func TestKeywordOnlyLeafBindsAfterItsKeyword(t *testing.T) {
	validated, err := ValidateArgs([]string{"file", "a", "content", "QUJD"}, validationShapedDefs(), nil)
	require.NoError(t, err)
	value, found := validated.Keyed("content")
	require.True(t, found)
	assert.Equal(t, "QUJD", value)

	_, err = ValidateArgs([]string{"file", "a", "content", "x"}, validationShapedDefs(), nil)
	require.Error(t, err, "a content below the declared length is refused")
	assert.Contains(t, err.Error(), "content")
}

// TestMandatoryKeywordOnlyLeafMissingIsNamedByKeyword proves a mandatory
// keyword-only leaf left out is reported missing, with its keyword form, even
// when a stray word sits where a positional value would.
//
// VALIDATES: phase 3 of ValidateArgs, MissingArgumentError.Usage.
// PREVENTS: `<key> <word>` writing the word as the content.
func TestMandatoryKeywordOnlyLeafMissingIsNamedByKeyword(t *testing.T) {
	for name, args := range map[string][]string{
		"nothing typed for it": {"k"},
		"a stray word":         {"k", "QUJDRA=="},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ValidateArgs(args, writeShapedDefs(), nil)
			require.Error(t, err, "the line was accepted with no content keyword")
			missing, ok := errors.AsType[*MissingArgumentError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, "content", missing.Leaf)
			assert.Equal(t, "content <content>", missing.Usage())
		})
	}
}

// TestUsageWritesKeywordOfMandatoryKeywordOnlyLeaf proves the usage line shows
// a mandatory keyword-only leaf as its keyword and then its value.
//
// VALIDATES: appendLeafTokens.
// PREVENTS: a usage line offering `<content>` as a position the validator
// never reads.
func TestUsageWritesKeywordOfMandatoryKeywordOnlyLeaf(t *testing.T) {
	node := &Node{Name: "key", WireMethod: "ze-test:update-data-key", ArgDefs: writeShapedDefs()}
	got := UsageLine(Usage([]string{"update", "data", "key"}, node))
	assert.Equal(t, "update data key <key> content <content>", got)

	optional := &Node{Name: "validation", WireMethod: "ze-test:show-config-validation", ArgDefs: validationShapedDefs()}
	got = UsageLine(Usage([]string{"show", "config", "validation"}, optional))
	assert.Equal(t, "show config validation <file> [content <content>]", got)
}

// TestCompleterOffersKeywordOfKeywordOnlyEnum proves completion offers the
// keyword of a keyword-only leaf and never a bare value of it.
//
// VALIDATES: the ArgDefs branch of TreeCompleter completion.
// PREVENTS: a completed bare value the validator then leaves unbound.
func TestCompleterOffersKeywordOfKeywordOnlyEnum(t *testing.T) {
	root := &Node{Children: map[string]*Node{
		"show": {Name: "show", Children: map[string]*Node{
			"thing": {
				Name:       "thing",
				WireMethod: "ze-test:show-thing",
				ArgDefs: []ArgDef{
					mustArgDef(NewEnumArg("mode", []string{"blocked", "full"}, ArgOptions{Mandatory: true, KeywordOnly: true})),
				},
			},
		}},
	}}
	texts := make(map[string]bool)
	for _, s := range NewTreeCompleter(root).Complete("show thing ") {
		texts[s.Text] = true
	}
	assert.True(t, texts["mode"], "the keyword is not offered")
	assert.False(t, texts["blocked"], "a bare value is offered")
	assert.False(t, texts["full"], "a bare value is offered")
}
