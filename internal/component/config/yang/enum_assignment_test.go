package yang

import (
	"errors"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enumStatement builds a `type enumeration` statement whose enum statements
// carry values, where value "" leaves an enum's value statement out.
func enumStatement(enums ...[2]string) *gyang.Type {
	root := &gyang.Type{Name: enumerationType}
	for _, enum := range enums {
		statement := &gyang.Enum{Name: enum[0]}
		if enum[1] != "" {
			statement.Value = &gyang.Value{Name: enum[1]}
		}
		root.Enum = append(root.Enum, statement)
	}
	return root
}

// TestAssignEnumValuesBoundaries drives assignEnumValues over the int32
// boundaries and the zero value of enumAssignment.
//
// VALIDATES: values from -2147483648 to 2147483647 are assigned, one outside
// is refused, and the implicit value after -2147483648 is -2147483647.
// PREVENTS: an enumeration value outside int32 entering the assignment.
func TestAssignEnumValuesBoundaries(t *testing.T) {
	assigned, err := assignEnumValues(enumStatement([2]string{"low", "-2147483648"}, [2]string{"next", ""}))
	require.NoError(t, err)
	next, held := assigned.Value("next")
	require.True(t, held)
	assert.Equal(t, int64(-2147483647), next)

	_, err = assignEnumValues(enumStatement([2]string{"high", "2147483648"}))
	assert.True(t, errors.Is(err, ErrEnumValue), "2147483648 is outside int32: %v", err)
	_, err = assignEnumValues(enumStatement([2]string{"low", "-2147483649"}))
	assert.True(t, errors.Is(err, ErrEnumValue), "-2147483649 is outside int32: %v", err)
	_, err = assignEnumValues(enumStatement([2]string{"a", ""}, [2]string{"a", "3"}))
	assert.True(t, errors.Is(err, ErrEnumValue), "a name assigned twice: %v", err)

	_, held = enumAssignment{}.Value("a")
	assert.False(t, held, "the zero value assigns no name")
	assert.Empty(t, enumAssignment{}.namesByValue())
}
