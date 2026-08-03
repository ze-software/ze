package yang

import (
	"math"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxUint64 is goyang's sentinel for "no max-elements constraint" (unbounded).
var maxUint64 = uint64(math.MaxUint64)

// TestValidationError verifies ValidationError type.
//
// VALIDATES: ValidationError contains expected fields.
// PREVENTS: Missing context in error reporting.
func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Path:       "bgp/local-as",
		Type:       ErrTypeRange,
		Message:    "value 0 is outside range 1..4294967295",
		Expected:   "1..4294967295",
		Got:        "0",
		LineNumber: 42,
	}

	assert.Equal(t, "bgp/local-as", err.Path)
	assert.Equal(t, ErrTypeRange, err.Type)
	assert.Contains(t, err.Error(), "bgp/local-as")
	assert.Contains(t, err.Error(), "range")
	assert.Contains(t, err.Error(), "42")
}

func TestCheckCardinality(t *testing.T) {
	tests := []struct {
		name    string
		min     uint64
		max     uint64
		count   uint64
		wantErr bool
		errType string
	}{
		{"within bounds", 1, 10, 5, false, ""},
		{"at max", 0, 10, 10, false, ""},
		{"at min", 2, 0, 2, false, ""},
		{"over max", 0, 10, 11, true, "too many"},
		{"under min", 2, 0, 1, true, "too few"},
		{"unbounded (goyang sentinel)", 0, maxUint64, 1000, false, ""},
		{"exactly one", 1, 1, 1, false, ""},
		{"exactly one but zero", 1, 1, 0, true, "too few"},
		{"exactly one but two", 1, 1, 2, true, "too many"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &gyang.Entry{
				ListAttr: &gyang.ListAttr{
					MinElements: tt.min,
					MaxElements: tt.max,
				},
			}
			var errs []ValidationError
			checkCardinality("test/path", entry, tt.count, &errs)
			if tt.wantErr {
				assert.NotEmpty(t, errs, "expected cardinality error")
				assert.Equal(t, ErrTypeCardinality, errs[0].Type)
				assert.Contains(t, errs[0].Message, tt.errType)
			} else {
				assert.Empty(t, errs, "expected no cardinality error")
			}
		})
	}
}

func TestCheckCardinalityNilListAttr(t *testing.T) {
	// VALIDATES: No panic when ListAttr is nil.
	// PREVENTS: NPE on entries without cardinality constraints.
	entry := &gyang.Entry{}
	var errs []ValidationError
	checkCardinality("test/path", entry, 5, &errs)
	assert.Empty(t, errs)
}

// TestLeafListItems covers every shape a leaf-list takes in the tree map.
//
// VALIDATES: a multi-member leaf-list (Tree.ToMap stores it as []string) is
// flattened to its members so cardinality and per-item validation run on it.
// PREVENTS: the regression where only the single-member string shape was
// handled, so a []string leaf-list silently skipped max-elements enforcement.
func TestLeafListItems(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{"single member string", "a1", []string{"a1"}},
		{"space-separated string", "a1 a2 a3", []string{"a1", "a2", "a3"}},
		{"empty string", "", nil},
		{"string slice (multi-member)", []string{"a1", "a2"}, []string{"a1", "a2"}},
		{"empty string slice", []string{}, []string{}},
		{"any slice of strings", []any{"a1", "a2"}, []string{"a1", "a2"}},
		{"any slice with non-string", []any{"a1", 2}, []string{"a1", "2"}},
		{"unsupported type", 42, nil},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, leafListItems(tt.value))
		})
	}
}

// leafListEntry builds a container whose single child is a leaf-list of strings
// with the given cardinality bounds, shaped the way walkTree expects to find it.
func leafListEntry(min, max uint64) *gyang.Entry {
	child := &gyang.Entry{
		Name:     "names",
		Kind:     gyang.LeafEntry,
		Type:     &gyang.YangType{Kind: gyang.Ystring, Name: "string"},
		ListAttr: &gyang.ListAttr{MinElements: min, MaxElements: max},
	}
	return &gyang.Entry{
		Name: "root",
		Kind: gyang.DirectoryEntry,
		Dir:  map[string]*gyang.Entry{"names": child},
	}
}

// TestWalkTreeLeafListCardinality drives leaf-list cardinality through walkTree,
// the real entry point, instead of calling checkCardinality directly.
//
// This is the coverage shape the defect hid behind. TestCheckCardinality above
// constructs a synthetic gyang.Entry and calls the helper, so it passes whether
// or not walkTree ever REACHES the helper -- and reaching it is the part that was
// broken. ai/rules/evidence.md: drive a guard from its entry point,
// never the helper alone.
//
// VALIDATES: walkTree applies BOTH bounds of a leaf-list's YANG cardinality --
// max-elements when over, and min-elements when under, INCLUDING when the
// leaf-list is present but empty.
// PREVENTS: min-elements being inert. walkTree guarded the cardinality call with
// `if len(items) > 0`, so a present-but-empty leaf-list skipped the check
// entirely and 0 < min never reported. max-elements was unaffected (an
// over-count is necessarily non-empty), which is why the max-side .ci test
// passed while the min side silently enforced nothing.
func TestWalkTreeLeafListCardinality(t *testing.T) {
	for _, tt := range []struct {
		name    string
		min     uint64
		max     uint64
		value   any
		wantErr string // "" = expect no cardinality error
	}{
		{"within bounds", 2, 4, []string{"a", "b", "c"}, ""},
		{"at min", 2, 4, []string{"a", "b"}, ""},
		{"at max", 2, 4, []string{"a", "b", "c", "d"}, ""},
		{"over max", 2, 4, []string{"a", "b", "c", "d", "e"}, "too many entries"},
		{"under min", 2, 4, []string{"a"}, "too few entries"},
		// The RED case: present but empty. Skipped entirely before the fix.
		{"empty leaf-list violates min", 2, 4, []string{}, "too few entries"},
		{"empty string leaf-list violates min", 1, maxUint64, "", "too few entries"},
		{"empty is fine when no min", 0, 4, []string{}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			entry := leafListEntry(tt.min, tt.max)
			var errs []ValidationError
			v.walkTree("root", entry, map[string]any{"names": tt.value}, &errs)

			var got []string
			for _, e := range errs {
				if e.Type == ErrTypeCardinality {
					got = append(got, e.Message)
				}
			}
			if tt.wantErr == "" {
				assert.Empty(t, got, "expected no cardinality error from walkTree")
				return
			}
			require.NotEmpty(t, got, "walkTree did not reach checkCardinality; the bound is inert")
			assert.Contains(t, got[0], tt.wantErr)
		})
	}
}

// TestWalkTreeAbsentLeafListMinElements covers the OTHER half of the bound: a
// min-bounded leaf-list omitted entirely.
//
// VALIDATES: walkTree reports "too few entries: 0" for a leaf-list with
// min-elements > 0 that is absent from the data, and stays silent when the
// leaf-list has no min bound or is present and satisfied.
// PREVENTS: the common case staying inert. Omitting the leaf is what an
// operator actually writes -- far more likely than `foo [ ]` -- and NOTHING
// else catches it: goyang's `case *LeafList:` synthesizes a Leaf without
// copying Mandatory (and *LeafList has no Mandatory field), so Entry.Mandatory
// is TSUnset forever for a leaf-list and walkTree's mandatory loop can never
// fire on one. Checking only the explicitly-empty form would have left the
// defect this spec is named for half-open.
func TestWalkTreeAbsentLeafListMinElements(t *testing.T) {
	for _, tt := range []struct {
		name    string
		min     uint64
		data    map[string]any
		wantErr bool
	}{
		{"absent with min violates", 1, map[string]any{}, true},
		{"absent with min 2 violates", 2, map[string]any{}, true},
		{"absent with no min is fine", 0, map[string]any{}, false},
		{"present and satisfied is fine", 1, map[string]any{"names": []string{"a"}}, false},
		{"present but empty still violates", 1, map[string]any{"names": []string{}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			entry := leafListEntry(tt.min, 4)
			var errs []ValidationError
			v.walkTree("root", entry, tt.data, &errs)

			var got []string
			for _, e := range errs {
				if e.Type == ErrTypeCardinality {
					got = append(got, e.Message)
				}
			}
			if !tt.wantErr {
				assert.Empty(t, got, "expected no cardinality error")
				return
			}
			require.NotEmpty(t, got, "walkTree did not report the absent min-bounded leaf-list")
			assert.Contains(t, got[0], "too few entries")
		})
	}
}

// nestedLeafListSchema mirrors the SHAPE the real declaration has: a
// min-bounded leaf-list inside a LIST inside a container, i.e.
// container/list[key]/leaf-list -- the skeleton of
// interface/ethernet[x]/unit[y]/ipv4/vrrp/group[g]/virtual-address.
func nestedLeafListSchema(min uint64) *gyang.Entry {
	names := &gyang.Entry{
		Name:     "names",
		Kind:     gyang.LeafEntry,
		Type:     &gyang.YangType{Kind: gyang.Ystring, Name: "string"},
		ListAttr: &gyang.ListAttr{MinElements: min, MaxElements: maxUint64},
	}
	group := &gyang.Entry{
		Name:     "group",
		Kind:     gyang.DirectoryEntry,
		ListAttr: &gyang.ListAttr{MaxElements: maxUint64}, // a LIST
		Dir:      map[string]*gyang.Entry{"names": names},
	}
	return &gyang.Entry{
		Name: "root",
		Kind: gyang.DirectoryEntry,
		Dir:  map[string]*gyang.Entry{"group": group},
	}
}

// TestWalkTreeNestedLeafListCardinality covers the nesting the flat tests do not.
//
// VALIDATES: the min-elements bound is enforced on a leaf-list reached through a
// LIST entry, and -- critically -- that an ABSENT list emits nothing.
// PREVENTS: two opposite failures. (a) The bound being enforced only at the top
// level, where the real declaration is six levels down inside a list, so the flat
// tests would pass while the shipped schema stayed inert. (b) The absence scan
// firing for a feature the operator never configured: walkTree descends into a
// list only for entries PRESENT in the data, so an absent `group` must produce
// no diagnostic at all. That second case is the availability risk of this change
// -- a config that omits a feature entirely must keep validating.
func TestWalkTreeNestedLeafListCardinality(t *testing.T) {
	for _, tt := range []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name:    "absent list emits nothing",
			data:    map[string]any{},
			wantErr: false,
		},
		{
			name:    "present list entry, leaf-list absent, violates",
			data:    map[string]any{"group": map[string]any{"g1": map[string]any{}}},
			wantErr: true,
		},
		{
			name:    "present list entry, leaf-list empty, violates",
			data:    map[string]any{"group": map[string]any{"g1": map[string]any{"names": []string{}}}},
			wantErr: true,
		},
		{
			name:    "present list entry, leaf-list satisfied",
			data:    map[string]any{"group": map[string]any{"g1": map[string]any{"names": []string{"a"}}}},
			wantErr: false,
		},
		{
			name: "two entries, only the empty one is reported",
			data: map[string]any{"group": map[string]any{
				"ok":  map[string]any{"names": []string{"a"}},
				"bad": map[string]any{},
			}},
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			var errs []ValidationError
			v.walkTree("root", nestedLeafListSchema(1), tt.data, &errs)

			var got []string
			for _, e := range errs {
				if e.Type == ErrTypeCardinality {
					got = append(got, e.Path)
				}
			}
			if !tt.wantErr {
				assert.Empty(t, got, "expected no cardinality error")
				return
			}
			require.NotEmpty(t, got, "bound not enforced through the list level")
			assert.Contains(t, got[0], "group[", "error path must name the list entry")
		})
	}
}
