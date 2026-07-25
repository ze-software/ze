package fakefib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

func TestParseRouteType(t *testing.T) {
	tests := []struct {
		input string
		want  sysribevents.RouteType
		err   bool
	}{
		{"blackhole", sysribevents.RouteTypeBlackhole, false},
		{"unreachable", sysribevents.RouteTypeUnreachable, false},
		{"prohibit", sysribevents.RouteTypeProhibit, false},
		{"invalid", 0, true},
	}
	for _, tt := range tests {
		got, err := parseRouteType(tt.input)
		if tt.err {
			assert.Error(t, err, "input=%q", tt.input)
		} else {
			require.NoError(t, err, "input=%q", tt.input)
			assert.Equal(t, tt.want, got, "input=%q", tt.input)
		}
	}
}

func TestParseLabels(t *testing.T) {
	got, err := parseLabels("100,200,300")
	require.NoError(t, err)
	assert.Equal(t, []uint32{100, 200, 300}, got)

	_, err = parseLabels("abc")
	assert.Error(t, err)
}

func TestRunEmitNoArgs(t *testing.T) {
	_, err := runEmit(nil)
	assert.ErrorIs(t, err, errUsageEmit)
}

func TestRunEmitNoEventBus(t *testing.T) {
	_, err := runEmit([]string{"add", "ipv4/unicast", "10.0.0.0/24"})
	assert.ErrorIs(t, err, errNoEventBus)
}
