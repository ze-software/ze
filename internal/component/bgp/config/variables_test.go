package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		localAS  uint32
		remoteAS uint32
		remoteIP string
		want     string
	}{
		{"no_variables", "filter-in", 65000, 64512, "10.0.0.1", "filter-in"},
		{"remote_as", "$remote_as", 65000, 64512, "10.0.0.1", "64512"},
		{"local_as", "$local_as", 65000, 64512, "10.0.0.1", "65000"},
		{"remote_ip", "$remote_ip", 65000, 64512, "10.0.0.1", "10.0.0.1"},
		{"combined", "filter-$remote_as-$remote_ip", 65000, 64512, "10.0.0.1", "filter-64512-10.0.0.1"},
		{"community_format", "$local_as:100", 65000, 64512, "10.0.0.1", "65000:100"},
		{"unknown_var", "$unknown", 65000, 64512, "10.0.0.1", "$unknown"},
		{"empty_string", "", 65000, 64512, "10.0.0.1", ""},
		{"ipv6_remote_ip", "$remote_ip", 65000, 64512, "2001:db8::1", "2001:db8::1"},
		{"as_in_as_path", "$remote_as", 65000, 4294967295, "10.0.0.1", "4294967295"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveVariables(tt.input, tt.localAS, tt.remoteAS, tt.remoteIP)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveFilterChain(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		got := ResolveFilterChain(nil, 65000, 64512, "10.0.0.1")
		assert.Nil(t, got)
	})

	t.Run("no_variables", func(t *testing.T) {
		input := []string{"filter-in", "filter-out"}
		got := ResolveFilterChain(input, 65000, 64512, "10.0.0.1")
		assert.Equal(t, input, got)
	})

	t.Run("with_variables", func(t *testing.T) {
		input := []string{"filter-$remote_as", "rpki-check"}
		got := ResolveFilterChain(input, 65000, 64512, "10.0.0.1")
		assert.Equal(t, []string{"filter-64512", "rpki-check"}, got)
	})
}

func TestResolveASPathStrings(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		got := ResolveASPathStrings(nil, 65000, 64512, "10.0.0.1")
		assert.Nil(t, got)
	})

	t.Run("no_variables", func(t *testing.T) {
		input := []string{"65000", "65001"}
		got := ResolveASPathStrings(input, 65000, 64512, "10.0.0.1")
		assert.Equal(t, input, got)
	})

	t.Run("with_remote_as", func(t *testing.T) {
		input := []string{"$remote_as", "65001"}
		got := ResolveASPathStrings(input, 65000, 64512, "10.0.0.1")
		assert.Equal(t, []string{"64512", "65001"}, got)
	})

	t.Run("with_local_as", func(t *testing.T) {
		input := []string{"$local_as"}
		got := ResolveASPathStrings(input, 65000, 64512, "10.0.0.1")
		assert.Equal(t, []string{"65000"}, got)
	})
}

func TestResolveCommunityStrings(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		got := ResolveCommunityStrings(nil, 65000, 64512, "10.0.0.1")
		assert.Nil(t, got)
	})

	t.Run("with_variables", func(t *testing.T) {
		input := []string{"$local_as:100", "no-export"}
		got := ResolveCommunityStrings(input, 65000, 64512, "10.0.0.1")
		assert.Equal(t, []string{"65000:100", "no-export"}, got)
	})
}
