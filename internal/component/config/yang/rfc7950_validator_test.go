package yang

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minElementsListEntry builds a root directory carrying a keyed list "peer"
// bound by min-elements, so walkTree reaches checkCardinality on the list path.
func minElementsListEntry(min uint64) *gyang.Entry {
	list := &gyang.Entry{
		Name:     "peer",
		Kind:     gyang.DirectoryEntry,
		Key:      "name",
		ListAttr: &gyang.ListAttr{MinElements: min, MaxElements: maxUint64},
		Dir: map[string]*gyang.Entry{
			"name": {Name: "name", Kind: gyang.LeafEntry, Type: &gyang.YangType{Kind: gyang.Ystring, Name: "string"}},
		},
	}
	return &gyang.Entry{
		Name: "root",
		Kind: gyang.DirectoryEntry,
		Dir:  map[string]*gyang.Entry{"peer": list},
	}
}

// cardinalityMessages returns the messages of the cardinality errors walkTree
// reported for data under entry.
func cardinalityMessages(entry *gyang.Entry, data map[string]any) []string {
	v := &Validator{}
	var errs []ValidationError
	v.walkTree("root", entry, data, &errs)
	var got []string
	for _, e := range errs {
		if e.Type == ErrTypeCardinality {
			got = append(got, e.Message)
		}
	}
	return got
}

// TestRFC7950MinElementsMet drives a leaf-list and a list that carry at least
// min-elements entries through walkTree and expects no cardinality error.
//
// RFC requirement: RFC7950-7.7.5-1 positive — a leaf-list with exactly min-elements values and a list with more than min-elements entries pass walkTree with no ErrTypeCardinality.
func TestRFC7950MinElementsMet(t *testing.T) {
	got := cardinalityMessages(leafListEntry(2, maxUint64), map[string]any{"names": []string{"a", "b"}})
	assert.Empty(t, got, "leaf-list at min-elements must not report cardinality")

	got = cardinalityMessages(minElementsListEntry(1), map[string]any{
		"peer": map[string]any{
			"one": map[string]any{},
			"two": map[string]any{},
		},
	})
	assert.Empty(t, got, "list above min-elements must not report cardinality")
}

// TestRFC7950MinElementsViolated drives a leaf-list with fewer values than
// min-elements, an absent leaf-list, and a list with fewer entries than
// min-elements through walkTree and expects the "too few entries" refusal for
// each.
//
// RFC requirement: RFC7950-7.7.5-1 negative — a leaf-list with fewer values than min-elements, an absent leaf-list with min-elements 1, and a list with fewer entries than min-elements are each refused with ErrTypeCardinality "too few entries".
func TestRFC7950MinElementsViolated(t *testing.T) {
	got := cardinalityMessages(leafListEntry(2, maxUint64), map[string]any{"names": []string{"a"}})
	require.Len(t, got, 1, "leaf-list under min-elements must report exactly one cardinality error")
	assert.Contains(t, got[0], "too few entries: 1 (minimum 2)")

	got = cardinalityMessages(leafListEntry(1, maxUint64), map[string]any{})
	require.Len(t, got, 1, "absent leaf-list with min-elements must report exactly one cardinality error")
	assert.Contains(t, got[0], "too few entries: 0 (minimum 1)")

	got = cardinalityMessages(minElementsListEntry(2), map[string]any{
		"peer": map[string]any{
			"one": map[string]any{},
		},
	})
	require.Len(t, got, 1, "list under min-elements must report exactly one cardinality error")
	assert.Contains(t, got[0], "too few entries: 1 (minimum 2)")
}

// boolType is the goyang type a boolean leaf carries, so the test enters
// validateBoolean through the validateYangType switch.
var boolType = &gyang.YangType{Kind: gyang.Ybool, Name: "boolean"}

// TestRFC7950BooleanLowercaseAccepted feeds the two lexical forms the RFC
// defines through the type switch and expects both accepted.
//
// RFC requirement: RFC7950-9.5.1-1 positive — the lowercase strings "true" and "false" are accepted as boolean values with no error.
func TestRFC7950BooleanLowercaseAccepted(t *testing.T) {
	v := &Validator{}
	for _, value := range []string{"true", "false"} {
		assert.NoError(t, v.validateYangType("root/flag", boolType, value), "lowercase %q must be accepted", value)
	}
}

// TestRFC7950BooleanNotLowercaseRefused feeds capitalized and foreign spellings
// through the type switch and expects each refused as a type error.
//
// RFC requirement: RFC7950-9.5.1-1 negative — "True", "FALSE", "TRUE", "yes" and "1" are each refused with ErrTypeType and the message "expected boolean".
func TestRFC7950BooleanNotLowercaseRefused(t *testing.T) {
	v := &Validator{}
	for _, value := range []string{"True", "FALSE", "TRUE", "yes", "1"} {
		err := v.validateYangType("root/flag", boolType, value)
		require.Error(t, err, "%q must be refused", value)
		var valErr *ValidationError
		require.ErrorAs(t, err, &valErr, "%q must produce a ValidationError", value)
		assert.Equal(t, ErrTypeType, valErr.Type, "%q must be a type error", value)
		assert.Equal(t, "expected boolean", valErr.Message)
		assert.Equal(t, "root/flag", valErr.Path)
	}
}
