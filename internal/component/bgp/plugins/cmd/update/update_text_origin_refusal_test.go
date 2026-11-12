package update

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/suggest"
)

// TestUpdateTextOriginEnumRefusal proves a refused origin names the command
// the operator typed, not the schema path the validator judged it at.
//
// GOAL: `update text origin bogus ...` reads `update text origin: "bogus" is
// not an accepted value`, lists the origin values with their help, and gives
// the command retyped with the placeholder as its fix; `update cursor` names
// its own words.
// METHOD: make the YANG validator answer the walk's enum refusal at
// bgp peer update attribute origin, then parse the line through both parsers'
// shared attribute reader.
// VALIDATES: AC-22 (originRefusal, parseCommonAttributeText).
// PREVENTS: `invalid origin: enum error at bgp/peer/update/attribute/origin`,
// a path no operator typed and a refusal with no fix.
func TestUpdateTextOriginEnumRefusal(t *testing.T) {
	values := []suggest.Value{
		{Name: "igp", Help: "Learned from an interior protocol"},
		{Name: "egp", Help: "Learned from EGP"},
		{Name: "incomplete", Help: "Learned some other way"},
	}
	walk := yang.EnumRefusal(yang.RefusedValue{
		Path:  []string{"bgp", "peer", "update", "attribute", "origin"},
		Leaf:  "origin",
		Value: "bogus",
	}, values)
	v := newTestValidator()
	v.errs[yangPathOrigin] = walk
	plugin.SetYANGValidator(v)
	defer plugin.SetYANGValidator(nil)

	const why = "origin accepts only these values:\n" +
		"  igp         Learned from an interior protocol\n" +
		"  egp         Learned from EGP\n" +
		"  incomplete  Learned some other way\n"

	_, err := ParseUpdateText([]string{"origin", "bogus", "nlri", "add", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Equal(t, `update text origin: "bogus" is not an accepted value`+"\n"+why+
		"fix: update text origin <igp|egp|incomplete> nlri add 10.0.0.0/24", err.Error())
	assert.NotContains(t, err.Error(), "bgp/peer/update/attribute/origin")
	_, typed := errors.AsType[*yang.ValidationError](err)
	assert.True(t, typed, "the refusal stays the typed validation error")
	_, operator := errors.AsType[*opererr.Error](err)
	assert.True(t, operator, "the web notice reads the three parts")

	var attrs parsedAttrs
	_, err = parseCommonAttributeText(updateCursorCommand, kwOrigin, []string{"origin", "bogus"}, 0, &attrs)
	require.Error(t, err)
	assert.Equal(t, `update cursor origin: "bogus" is not an accepted value`+"\n"+why+
		"fix: update cursor origin <igp|egp|incomplete>", err.Error())
}
