package runner

import (
	"fmt"
	"testing"
)

func TestColoredCharDiff(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
	}{
		{
			name:     "single word change",
			expected: `{"announce":{"ipv6 flow":{"value":1}}}`,
			actual:   `{"announce":{"ipv6/flow":{"value":1}}}`,
		},
		{
			name:     "multiple field changes",
			expected: `{"name":"alice","age":25,"city":"paris"}`,
			actual:   `{"name":"bob","age":30,"city":"london"}`,
		},
		{
			name:     "insertion",
			expected: `{"a":1}`,
			actual:   `{"a":1,"b":2}`,
		},
		{
			name:     "deletion",
			expected: `{"a":1,"b":2,"c":3}`,
			actual:   `{"a":1,"c":3}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColoredCharDiff(tt.expected, tt.actual)
			// Just verify it produces output without panicking
			if result == "" && tt.expected != tt.actual {
				t.Error("expected non-empty diff")
			}
		})
	}
}

// ExampleColoredCharDiff demonstrates the diff output.
func ExampleColoredCharDiff() {
	// Single change
	exp := `{"ipv6 flow":1}`
	act := `{"ipv6/flow":1}`
	fmt.Println("Single change:")
	fmt.Println(ColoredCharDiff(exp, act))
	fmt.Println()

	// Multiple changes
	exp2 := `{"name":"alice","city":"paris"}`
	act2 := `{"name":"bob","city":"london"}`
	fmt.Println("Multiple changes:")
	fmt.Println(ColoredCharDiff(exp2, act2))
}
