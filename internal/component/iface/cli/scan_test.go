// Design: docs/features/interfaces.md -- interface discovery CLI
//
// VALIDATES: `interface scan` answers with DATA registered as
//   `show interface scan`, so the pipe layer renders it and the deleted
//   `--json` and `--yaml` flags have no work left to do (ai/rules/cli.md,
//   "--flag or Keyword"). The rows reaching the pipe layer are proven against a
//   live kernel in scan_linux_test.go, which is where a scan can answer.
// PREVENTS: the registration being lost, which would leave the scan reachable
//   from no pipe layer again; a declared column that is not a row field, which
//   silently drops a column from the table; and an operator the answer's shape
//   cannot support being answered instead of refused by name.

package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	ifacepkg "github.com/ze-software/ze/internal/component/iface"
)

// scanFixture is a discovered set with one row of each kind the filter keeps
// and one it drops.
func scanFixture() []ifacepkg.DiscoveredInterface {
	return []ifacepkg.DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "02:00:00:00:00:01"},
		{Name: "ze0", Type: ifaceTypeVeth, MAC: "02:00:00:00:00:02"},
	}
}

func TestTheScanAnswerIsServedInThisProcess(t *testing.T) {
	t.Parallel()

	if !command.HasLocalData(cmdPathShowInterfaceScan) {
		t.Fatalf("%q is not served in this process, so no pipe operator can reach its answer", cmdPathShowInterfaceScan)
	}
}

func TestTheDeclaredColumnsAreTheRowFields(t *testing.T) {
	t.Parallel()

	shape, declared := command.ShapeForCommand(cmdPathShowInterfaceScan)
	if !declared || shape != command.ShapeTab {
		t.Fatalf("declared shape = %v (declared=%t), want ShapeTab", shape, declared)
	}

	orders := command.ColumnsForCommand(cmdPathShowInterfaceScan)
	if len(orders) != 1 {
		t.Fatalf("column orders = %d, want 1", len(orders))
	}
	encoded, err := json.Marshal(scanFixture()[0])
	if err != nil {
		t.Fatalf("encode a row: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode a row: %v", err)
	}
	for _, name := range orders[0] {
		if _, exists := fields[name]; !exists {
			t.Errorf("declared column %q is not a field of the row: %v", name, fields)
		}
	}
	if len(orders[0]) != len(fields) {
		t.Errorf("declared columns %v do not cover every row field %v", orders[0], fields)
	}
}

// The chain is checked against the command's DECLARED shape before the scan
// runs, so this holds on every platform, kernel or none.
func TestAnUnsupportableOperatorIsRefusedByName(t *testing.T) {
	t.Parallel()

	rendered, code, served := command.ServeLocal(cmdPathShowInterfaceScan+" | resolve", "")
	if !served {
		t.Fatalf("%q is not served in this process", cmdPathShowInterfaceScan)
	}
	if code == 0 {
		t.Fatalf("an unsupportable operator was accepted: %s", rendered)
	}
	// `| resolve` acts only on a field a command declares to hold an IP
	// address, and the scan declares none.
	if !command.IsPipeError(rendered) || !strings.Contains(rendered, "resolve") {
		t.Errorf("refusal does not name the operator: %s", rendered)
	}
}

// The human default prints the payload through the renderer the registration
// uses, in the column order the command declares. It is the same payload
// `| json` and `| yaml` render, so a column order that has drifted from the
// rows shows up here rather than in an operator's terminal.
func TestTheDefaultRenderingCarriesTheDeclaredColumnOrder(t *testing.T) {
	saved := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	code := command.RenderLocalAnswer(cmdPathShowInterfaceScan, scanAnswer(scanFixture()))
	os.Stdout = saved
	if err := write.Close(); err != nil {
		t.Fatalf("close the capture: %v", err)
	}
	var out strings.Builder
	buffer := make([]byte, 4096)
	for {
		n, readErr := read.Read(buffer)
		out.Write(buffer[:n])
		if readErr != nil {
			break
		}
	}
	if code != 0 {
		t.Fatalf("RenderLocalAnswer = %d, want 0: %s", code, out.String())
	}

	rendered := out.String()
	position := -1
	for _, column := range command.ColumnsForCommand(cmdPathShowInterfaceScan)[0] {
		at := strings.Index(rendered, column)
		if at < 0 {
			t.Fatalf("the rendering has no %q column: %s", column, rendered)
		}
		if at <= position {
			t.Errorf("column %q is out of the declared order: %s", column, rendered)
		}
		position = at
	}
	for _, want := range []string{"eth0", "ze0", "02:00:00:00:00:02"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendering lost %q: %s", want, rendered)
		}
	}
}

func TestFilterManagedKeepsOnlyTheKindsZeCreates(t *testing.T) {
	t.Parallel()

	filtered := filterManaged(scanFixture())
	if len(filtered) != 1 || filtered[0].Name != "ze0" {
		t.Fatalf("filterManaged = %#v, want only the veth row", filtered)
	}
}
