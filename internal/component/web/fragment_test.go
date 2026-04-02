package web

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestCollectFormFields_RequiredAndSuggest verifies that required and suggest fields
// are collected from a ListNode with the correct paths.
// VALIDATES: Required + suggest + unique fields collected and categorized.
// PREVENTS: Field collection silently dropping required/suggest paths.
func TestCollectFormFields_RequiredAndSuggest(t *testing.T) {
	listNode := config.List(config.TypeString,
		config.Field("connection", config.Container(
			config.Field("remote", config.Container(
				config.Field("ip", config.Leaf(config.TypeString)),
			)),
			config.Field("local", config.Container(
				config.Field("ip", config.Leaf(config.TypeString)),
			)),
		)),
		config.Field("session", config.Container(
			config.Field("asn", config.Container(
				config.Field("local", config.Leaf(config.TypeUint32)),
				config.Field("remote", config.Leaf(config.TypeUint32)),
			)),
		)),
	)
	listNode.Required = [][]string{
		{"connection", "remote", "ip"},
		{"session", "asn", "remote"},
	}
	listNode.Suggest = [][]string{
		{"connection", "local", "ip"},
	}
	listNode.Unique = [][]string{
		{"connection/remote/ip"},
	}

	required := collectRequiredFields(listNode)
	assert.Equal(t, []string{"connection/remote/ip", "session/asn/remote"}, required)

	suggest := collectSuggestFields(listNode)
	assert.Equal(t, []string{"connection/local/ip"}, suggest)

	unique := collectUniqueFields(listNode)
	assert.Equal(t, []string{"connection/remote/ip"}, unique)
}
