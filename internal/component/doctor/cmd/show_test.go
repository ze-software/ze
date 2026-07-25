package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestPlatformMatchesEmpty(t *testing.T) {
	t.Parallel()
	assert.True(t, platformMatches(nil, nil), "nil platforms = match any")
	assert.True(t, platformMatches([]string{}, nil), "empty platforms = match any")
}

func TestPlatformMatchesAny(t *testing.T) {
	t.Parallel()
	assert.True(t, platformMatches([]string{"any"}, nil))
}

func TestPlatformMatchesSpecific(t *testing.T) {
	t.Parallel()
	info := &host.PlatformInfo{Type: host.PlatformDarwin}
	assert.True(t, platformMatches([]string{"darwin"}, info))
	assert.False(t, platformMatches([]string{"gokrazy"}, info))
}

func TestPlatformMatchesNilInfo(t *testing.T) {
	t.Parallel()
	assert.False(t, platformMatches([]string{"darwin"}, nil))
}

func TestNormalizeSeverity(t *testing.T) {
	t.Parallel()
	assert.Equal(t, diagnostic.SeverityError, normalizeSeverity("error"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity("warning"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity("unknown"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity(""))
}
