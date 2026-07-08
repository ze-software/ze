// Design: (none -- test for community-name decorator)

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommunityNameDecorator verifies well-known standard communities in
// "ASN:value" form are annotated with their names, while ordinary communities,
// bare names, and malformed input produce no annotation.
//
// VALIDATES: L119 decorators-v2 -- community-name decorator maps a well-known
// community value to its RFC name.
// PREVENTS: annotating ordinary communities (which already render as ASN:value)
// or erroring on non-community input.
func TestCommunityNameDecorator(t *testing.T) {
	d := newCommunityNameDecorator()
	assert.Equal(t, "community-name", d.Name())

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no-export", "65535:65281", "no-export"},         // 0xFFFFFF01
		{"no-advertise", "65535:65282", "no-advertise"},   // 0xFFFFFF02
		{"blackhole", "65535:666", "blackhole"},           // 0xFFFF029A
		{"ordinary community not named", "65000:100", ""}, // 0xFDE80064
		{"already a bare name", "no-export", ""},          // no colon to parse
		{"malformed low value", "65000:notnum", ""},       // graceful
		{"malformed high value", "notnum:100", ""},        // graceful
		{"missing colon", "65000", ""},                    // graceful
		{"out-of-range 16-bit part", "70000:1", ""},       // graceful (ParseUint 16-bit fails)
		{"empty value", "", ""},                           // graceful
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.Decorate(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
