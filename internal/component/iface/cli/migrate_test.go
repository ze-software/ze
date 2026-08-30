package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestParseMigrateKeywords drives the client's reader of `ze interface migrate`
// over the keyword grammar an operator types and over every way of getting it
// wrong.
//
// VALIDATES: each keyword binds its own value, the order they arrive in does
// not matter, and an unknown keyword, a repeated keyword, a keyword with no
// value, a missing required keyword and a malformed value are each refused by
// name, before any SSH round trip.
// PREVENTS: a token nobody recognized being SKIPPED. An operator who misspells
// a keyword would otherwise have a migration of an address they did not name
// sent to the daemon.
func TestParseMigrateKeywords(t *testing.T) {
	t.Parallel()

	minimal := migrateRequest{from: "eth0.0", to: "lo1.0", address: "10.0.0.1/24"}

	tests := []struct {
		name string
		args []string
		// wantGrammar says the refusal is about the shape of the command, so it
		// must quote the whole form back. A refusal about ONE value states what
		// that value should look like instead, which is the narrower answer.
		wantGrammar bool
		wantRequest migrateRequest
		wantError   string
	}{
		{
			name:        "required keywords only",
			args:        []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantRequest: minimal,
		},
		{
			name: "every keyword",
			args: []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24",
				"create", "dummy", "timeout", "45s"},
			wantRequest: migrateRequest{
				from: "eth0.0", to: "lo1.0", address: "10.0.0.1/24",
				createType: "dummy", timeout: "45s",
			},
		},
		{
			name:        "keyword order does not matter",
			args:        []string{"address", "10.0.0.1/24", "to", "lo1.0", "from", "eth0.0"},
			wantRequest: minimal,
		},
		{
			name:        "an IPv6 address is an address",
			args:        []string{"from", "eth0.100", "to", "lo2.0", "address", "fd00::1/64"},
			wantRequest: migrateRequest{from: "eth0.100", to: "lo2.0", address: "fd00::1/64"},
		},
		{
			name:        "unknown keyword",
			args:        []string{"from", "eth0.0", "bogus", "value"},
			wantGrammar: true,
			wantError:   `unknown keyword "bogus"`,
		},
		{
			name:        "a value with no keyword before it",
			args:        []string{"eth0.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantGrammar: true,
			wantError:   `unknown keyword "eth0.0"`,
		},
		{
			name:        "keyword given twice",
			args:        []string{"from", "eth0.0", "from", "eth1.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantGrammar: true,
			wantError:   `keyword "from" given twice`,
		},
		{
			name:        "keyword with no value",
			args:        []string{"from", "eth0.0", "to", "lo1.0", "address"},
			wantGrammar: true,
			wantError:   `keyword "address" has no value`,
		},
		{
			name:        "no arguments at all",
			args:        nil,
			wantGrammar: true,
			wantError:   `keyword "from" is missing`,
		},
		{
			name:        "missing to",
			args:        []string{"from", "eth0.0", "address", "10.0.0.1/24"},
			wantGrammar: true,
			wantError:   `keyword "to" is missing`,
		},
		{
			name:        "missing address",
			args:        []string{"from", "eth0.0", "to", "lo1.0"},
			wantGrammar: true,
			wantError:   `keyword "address" is missing`,
		},
		{
			name:      "invalid from format",
			args:      []string{"from", "noDot", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantError: `invalid from value "noDot"`,
		},
		{
			name:      "invalid to format",
			args:      []string{"from", "eth0.0", "to", "noDot", "address", "10.0.0.1/24"},
			wantError: `invalid to value "noDot"`,
		},
		{
			name:      "an address with no prefix length",
			args:      []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1"},
			wantError: `invalid address value "10.0.0.1"`,
		},
		{
			name:      "unknown interface type",
			args:      []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24", "create", "tunnel"},
			wantError: `invalid create value "tunnel"`,
		},
		{
			name:      "invalid timeout",
			args:      []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24", "timeout", "bad"},
			wantError: `invalid timeout value "bad"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := parseMigrateKeywords(tt.args)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				if tt.wantGrammar {
					assert.Contains(t, err.Error(), migrateGrammar,
						"a refusal about the shape of the command quotes the whole form back")
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRequest, req)
		})
	}
}

// TestMigrateCommandIsTheKeywordForm pins the exact command string the client
// sends over SSH.
//
// VALIDATES: the command names the daemon's own dispatch path, every value
// arrives after its keyword, the source and the destination are normalized to
// <name>.<unit>, and a keyword the operator did not name is left out so the
// daemon applies its own default.
// PREVENTS: a value reaching the daemon as a bare positional or under a
// spelling the daemon's parser does not hold, which is unobservable from either
// side on its own.
func TestMigrateCommandIsTheKeywordForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "required keywords only",
			args: []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			want: "request interface migrate from eth0.0 to lo1.0 address 10.0.0.1/24",
		},
		{
			name: "every keyword",
			args: []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24",
				"create", "dummy", "timeout", "45s"},
			want: "request interface migrate from eth0.0 to lo1.0 address 10.0.0.1/24 create dummy timeout 45s",
		},
		{
			name: "the operator's order does not reach the wire",
			args: []string{"address", "10.0.0.1/24", "to", "lo1.0", "from", "eth0.0"},
			want: "request interface migrate from eth0.0 to lo1.0 address 10.0.0.1/24",
		},
		{
			name: "a padded unit number is normalized",
			args: []string{"from", "eth0.007", "to", "lo1.0", "address", "10.0.0.1/24"},
			want: "request interface migrate from eth0.7 to lo1.0 address 10.0.0.1/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := parseMigrateKeywords(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, "request interface migrate "+req.arguments())
		})
	}
}

// TestMigrateCommandReachesTheDaemonHandler dispatches the command string the
// client builds through the daemon's own dispatcher.
//
// VALIDATES: the string survives every gate (*Dispatcher).Dispatch applies
// before a handler runs -- tokenize, the builtin path match, and the
// flag-shaped-token refusal -- and reaches the handler with the keyword/value
// tokens the daemon's parser reads.
// PREVENTS: the defect this grammar was written for. A client that sends a
// flag-shaped token is answered `unexpected flag ...` by firstFlagToken
// (internal/component/plugin/server/command.go) before the handler is called,
// and a client that names a path the daemon does not register never matches at
// all. Neither failure is visible from the client's own tests.
func TestMigrateCommandReachesTheDaemonHandler(t *testing.T) {
	t.Parallel()

	req, err := parseMigrateKeywords([]string{
		"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24",
		"create", "dummy", "timeout", "45s",
	})
	require.NoError(t, err)

	var got []string
	handler := func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		got = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	// The path is written out here, exactly as the production call site writes
	// it, so a client aiming at a path the daemon does not serve fails here
	// instead of moving the target with it. TestUsageRendersTheDeclaredValues
	// (internal/component/plugin/server) holds the same path against the YANG
	// tree the daemon builds its dispatch keys from.
	//
	// The daemon registers the handler with no argument definitions of its own:
	// container migrate declares its five values as modifier groups, so
	// extractArgDefs (internal/component/config/yang) finds no leaf on the
	// command node and the tokens pass through to the handler.
	d := pluginserver.NewDispatcher()
	d.RegisterWithOptions("request interface migrate", handler, "under test", pluginserver.RegisterOptions{})

	resp, err := d.Dispatch(&pluginserver.CommandContext{}, "request interface migrate "+req.arguments())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{
		"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24",
		"create", "dummy", "timeout", "45s",
	}, got)
}
