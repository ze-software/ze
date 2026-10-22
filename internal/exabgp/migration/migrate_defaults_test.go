package migration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// migrateNeighborOutput parses one ExaBGP config and answers the serialized
// ze config the migration writes for it.
func migrateNeighborOutput(t *testing.T, input string) string {
	t.Helper()
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")
	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")
	return SerializeTree(result.Tree)
}

// requireLine fails unless output holds the line `<key> <value>` whole, so a
// want of 30 is not satisfied by 300.
func requireLine(t *testing.T, output, key, value string) {
	t.Helper()
	for line := range strings.SplitSeq(output, "\n") {
		if strings.TrimSpace(line) == key+" "+value {
			return
		}
	}
	t.Fatalf("migrated config holds no line %q %q:\n%s", key, value, output)
}

// TestMigrateHoldTimeStatesExaBGPDefault proves a neighbor with no hold-time
// migrates to a peer that offers ExaBGP's 180 seconds rather than ze's 90.
//
// VALIDATES: timer > receive-hold-time is written for every neighbor, with the
// stated value when there is one and 180 when there is none.
// PREVENTS: A migrated session offering 90 s where the ExaBGP one offered 180 s.
func TestMigrateHoldTimeStatesExaBGPDefault(t *testing.T) {
	tests := []struct {
		name string
		hold string // the neighbor's hold-time line, empty for none
		want string
	}{
		{name: "absent takes ExaBGP default", hold: "", want: "180"},
		{name: "stated value kept", hold: "\thold-time 30\n", want: "30"},
		{name: "stated zero kept", hold: "\thold-time 0\n", want: "0"},
		{name: "stated 90 kept", hold: "\thold-time 90\n", want: "90"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "neighbor 10.0.0.1 {\n\tlocal-as 65001\n\tpeer-as 65002\n" + tt.hold + "}\n"
			requireLine(t, migrateNeighborOutput(t, input), "receive-hold-time", tt.want)
		})
	}
}

// TestMigrateHoldTimeFromTemplate proves a hold-time a template states reaches
// the peer, and the default applies only when neither states one.
//
// VALIDATES: The inherited hold-time wins over ExaBGP's default.
// PREVENTS: The default overwriting a template's value.
func TestMigrateHoldTimeFromTemplate(t *testing.T) {
	input := `
template {
	neighbor base {
		hold-time 45
	}
}
neighbor 10.0.0.1 {
	inherit base
	local-as 65001
	peer-as 65002
}
`
	requireLine(t, migrateNeighborOutput(t, input), "receive-hold-time", "45")
}

// TestMigrateGracefulRestartTimeFromHoldTime proves Graceful Restart enabled
// without a time migrates to the Restart Time ExaBGP advertises: the hold time,
// capped at 4095.
//
// VALIDATES: bare and zero graceful-restart take the hold time; a stated time
// is kept; the cap applies at the twelve-bit boundary.
// PREVENTS: The migrated peer advertising ze's 120 where ExaBGP advertised 180.
func TestMigrateGracefulRestartTimeFromHoldTime(t *testing.T) {
	tests := []struct {
		name string
		hold string // the neighbor's hold-time line, empty for none
		gr   string // the graceful-restart statement
		want string
	}{
		{name: "bare, no hold-time", hold: "", gr: "graceful-restart", want: "180"},
		{name: "bare, stated hold-time", hold: "\thold-time 60\n", gr: "graceful-restart", want: "60"},
		{name: "zero, stated hold-time", hold: "\thold-time 90\n", gr: "graceful-restart 0", want: "90"},
		{name: "stated time kept", hold: "\thold-time 60\n", gr: "graceful-restart 120", want: "120"},
		{name: "hold at the cap", hold: "\thold-time 4095\n", gr: "graceful-restart", want: "4095"},
		{name: "hold one past the cap", hold: "\thold-time 4096\n", gr: "graceful-restart", want: "4095"},
		{name: "hold at uint16 max", hold: "\thold-time 65535\n", gr: "graceful-restart", want: "4095"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "neighbor 10.0.0.1 {\n\tlocal-as 65001\n\tpeer-as 65002\n" + tt.hold +
				"\tcapability {\n\t\t" + tt.gr + "\n\t}\n}\n"
			requireLine(t, migrateNeighborOutput(t, input), "graceful-restart", tt.want)
		})
	}
}
