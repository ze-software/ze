package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestPolicyFieldMismatchRefused: a chain whose filter reads a field the
// attachment's routes do not carry is refused at load, naming the filter, the
// field and the attachment (AC-10). Method: declare a community-reading filter
// type and an attachment carrying only prefix and family in a private
// vocabulary, then check one chain that fits and one that does not.
func TestPolicyFieldMismatchRefused(t *testing.T) {
	v := newVocabulary()
	prefix := v.mustRegisterField("prefix", RouteView{})
	family := v.mustRegisterField("family", RouteView{})
	community := v.mustRegisterField("community", RouteView{})
	tag := v.mustRegisterField("tag", RouteView{})

	v.mustRegisterFilterType("community-match", []Field{community}, nil)
	v.mustRegisterFilterType("prefix-list", []Field{prefix, family}, nil)
	v.mustRegisterFilterType("tagger", nil, []Field{tag})
	v.mustRegisterAttachment("isis", "isis import", []Field{prefix, family}, nil, false)

	tree := config.NewTree()
	tree.AddListEntry("prefix-list", "LOOPBACKS", config.NewTree())
	tree.AddListEntry("community-match", "NO-EXPORT", config.NewTree())
	reg := &Registry{instances: map[string]Instance{
		"LOOPBACKS": {Name: "LOOPBACKS", Type: "prefix-list"},
		"NO-EXPORT": {Name: "NO-EXPORT", Type: "community-match"},
		"SET-TAG":   {Name: "SET-TAG", Type: "tagger"},
	}}

	require.NoError(t, v.checkChain("isis import", reg, []string{"LOOPBACKS"}))

	err := v.checkChain("isis import", reg, []string{"LOOPBACKS", "NO-EXPORT"})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, `"NO-EXPORT"`)
	assert.Contains(t, msg, "community-match")
	assert.Contains(t, msg, "community")
	assert.Contains(t, msg, "isis import")

	err = v.checkChain("isis import", reg, []string{"SET-TAG"})
	require.Error(t, err, "a filter setting a field the attachment does not allow is refused")
	assert.Contains(t, err.Error(), "tag")

	err = v.checkChain("ospf import", reg, []string{"LOOPBACKS"})
	require.Error(t, err, "an attachment nobody registered cannot be judged")
}

// TestPolicyFieldDuplicateRegistrationPanics: a second registration of one
// field is a Ze defect, refused at init.
func TestPolicyFieldDuplicateRegistrationPanics(t *testing.T) {
	v := newVocabulary()
	v.mustRegisterField("prefix", RouteView{})
	assert.Panics(t, func() { v.mustRegisterField("prefix", RouteView{}) })
}
