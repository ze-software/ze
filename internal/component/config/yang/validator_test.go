package yang

import (
	"math"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
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
