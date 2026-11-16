// Design: docs/architecture/web-interface.md -- caller-local output controls.
package web

import "github.com/ze-software/ze/internal/component/command"

// cliLocalOutputControl projects an implemented caller operation from its
// command-owner declaration. Unknown future operations are not offered merely
// because they are caller-local; the catalog parity guard detects that gap.
type cliLocalOutputControl struct {
	Name        string
	Description string
	Label       string
}

func cliLocalOutputControls() []cliLocalOutputControl {
	var controls []cliLocalOutputControl
	for _, operator := range command.PipeOperatorCatalog() {
		if !operator.LocalOnly {
			continue
		}
		if operator.Name != "save" {
			continue
		}
		controls = append(controls, cliLocalOutputControl{
			Name: operator.Name, Description: operator.Description, Label: "Download",
		})
	}
	return controls
}
