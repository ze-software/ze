// Design: docs/architecture/testing/ci-format.md -- authored editor dimensions.

package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestETDimensionsReachScreens observes the actual rendered terminal, not the
// option map or model fields. The test-only expectation runs inside RunETFile
// at each authored checkpoint, after the runner has selected that model.
func TestETDimensionsReachScreens(t *testing.T) {
	const observer = "dimension-proof"
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
			return
		}
		delete(validExpectationTypes, observer)
	})
	for _, dimensions := range []struct {
		name    string
		options string
		columns int
		rows    int
	}{
		{name: "defaults", columns: 76, rows: 24},
		{name: "width", options: "option=width:value=101\n", columns: 97, rows: 24},
		{name: "height", options: "option=height:value=37\n", columns: 76, rows: 37},
	} {
		for _, mode := range []struct {
			name    string
			options string
		}{
			{name: "file"},
			{name: "session", options: "option=session:user=owner:origin=local\n"},
			{name: "backup", options: "option=storage:value=blob\noption=session:user=owner:origin=local\n"},
			{name: "command", options: "option=mode:value=command\n"},
		} {
			t.Run(dimensions.name+"/"+mode.name, func(t *testing.T) {
				var observed []string
				validExpectationTypes[observer] = func(exp Expectation, state State) error {
					hm, ok := state.(*headlessModel)
					if !ok {
						return fmt.Errorf("screen observer received %T", state)
					}
					screen := hm.Model().View().Content
					lines := strings.Split(ansi.Strip(screen), "\n")
					if len(lines) != dimensions.rows {
						return fmt.Errorf("%s: rendered %d rows, want %d", exp.Values["stage"], len(lines), dimensions.rows)
					}
					// The editor reserves four terminal columns around the viewport.
					if columns := ansi.StringWidth(lines[0]); columns != dimensions.columns {
						return fmt.Errorf("%s: viewport renders %d columns, want %d", exp.Values["stage"], columns, dimensions.columns)
					}
					observed = append(observed, exp.Values["stage"])
					return nil
				}
				content := "tmpfs=test.conf:terminator=END\nbgp { router-id 192.0.2.1; }\nEND\noption=file:path=test.conf\n" + dimensions.options + mode.options +
					"expect=dimension-proof:stage=initial\nrestart=editor\nexpect=dimension-proof:stage=restart\n" +
					"session=alice:user=alice,origin=ssh\nexpect=dimension-proof:stage=alice\n" +
					"session=bob:user=bob,origin=ssh\nexpect=dimension-proof:stage=bob\n" +
					"session=alice\nexpect=dimension-proof:stage=switch\n"
				path := filepath.Join(t.TempDir(), "dimensions.et")
				require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
				result := RunETFile(path, 1)
				require.True(t, result.Passed, "%s", result.Error)
				require.Equal(t, []string{"initial", "restart", "alice", "bob", "switch"}, observed)
			})
		}
	}
}
