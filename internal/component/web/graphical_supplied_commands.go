// Design: docs/architecture/web-components.md -- registered caller-input forms.
package web

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// suppliedInspectionCommand is admitted metadata joined to the actual model.
// Its zero value is invalid; only suppliedInspectionCommands constructs it.
// All fields are private and request-owned, and uses do not mutate them.
type suppliedInspectionCommand struct {
	path       string
	input      registry.SuppliedInput
	defs       []command.ArgDef
	encodedMax int
	decodedMax int
	view       GraphicalSuppliedInputs
}

func buildGraphicalSuppliedPanel(trees AdminCommandTree) *GraphicalSuppliedPanel {
	if trees == nil {
		return nil
	}
	commands, err := suppliedInspectionCommands(trees)
	panel := &GraphicalSuppliedPanel{}
	if err != nil {
		panel.Error = suppliedInspectionRefusal("The registered inspection inputs cannot be displayed.", err).Error()
		return panel
	}
	for _, registered := range commands {
		panel.Choices = append(panel.Choices, GraphicalSuppliedChoice{Path: registered.path, Label: registered.view.Summary})
	}
	if len(commands) > 0 {
		panel.Input = &commands[0].view
	}
	return panel
}

// suppliedInspectionCommands reads the local descriptor registry and the same
// current command tree the other operational forms use. It holds no command
// inventory, formatter or configuration-domain dependency.
func suppliedInspectionCommands(trees AdminCommandTree) ([]*suppliedInspectionCommand, error) {
	if trees == nil {
		return nil, errors.New("the operational command tree was not connected")
	}
	tree := trees()
	if tree == nil {
		return nil, errors.New("the operational command tree is unavailable")
	}
	var offered []*suppliedInspectionCommand
	for _, entry := range command.ListLocal() {
		if entry.Meta.SuppliedInput == nil {
			continue
		}
		if entry.Meta.NotReadOnly {
			continue
		}
		verb, _, _ := strings.Cut(entry.Path, " ")
		if !command.IsReadOnlyVerb(verb) {
			continue
		}
		node := adminNodeAt(tree, strings.Fields(entry.Path))
		if node == nil {
			continue
		}
		if node.WireMethod == "" {
			continue
		}
		kind := command.KindLocal
		handler, rest := command.LookupLocalData(strings.Fields(entry.Path))
		if handler != nil {
			if len(rest) == 0 {
				kind = command.KindLocalData
			}
		}
		listed, err := command.OffersCommand(command.CommandRegistration{
			Path: entry.Path, Kind: kind, DaemonPath: entry.Meta.DaemonPath,
		}, command.SurfaceWebAdmin)
		if err != nil {
			return nil, err
		}
		if !listed {
			continue
		}
		admitted, err := newSuppliedInspectionCommand(entry, node)
		if err != nil {
			return nil, err
		}
		offered = append(offered, admitted)
	}
	return offered, nil
}

func newSuppliedInspectionCommand(entry command.LocalCommandEntry, node *command.Node) (*suppliedInspectionCommand, error) {
	input := *entry.Meta.SuppliedInput
	input.Fields = slices.Clone(input.Fields)
	defs := slices.Clone(node.ArgDefs)
	var content *command.ArgDef
	for index := range defs {
		if defs[index].Name() == input.ContentArg {
			content = &defs[index]
		}
	}
	if content == nil {
		return nil, fmt.Errorf("%s: the supplied content argument is absent from its model", entry.Path)
	}
	if content.Kind() != command.ArgString {
		return nil, fmt.Errorf("%s: the supplied content argument is not a string", entry.Path)
	}
	var declared uint64
	for part := range content.Lengths() {
		declared = max(declared, part.Max)
	}
	if declared < 4 {
		return nil, fmt.Errorf("%s: its model declares no usable supplied-content bound", entry.Path)
	}
	encodedMax := int(min(declared, uint64(command.ContentEncodedMax)))
	admitted := &suppliedInspectionCommand{
		path: entry.Path, input: input, defs: defs,
		encodedMax: encodedMax, decodedMax: encodedMax / 4 * 3,
		view: GraphicalSuppliedInputs{
			Command: entry.Path, Summary: node.ShortHelp, Description: node.Description,
			Limit: suppliedInspectionNumber(encodedMax / 4 * 3),
		},
	}
	if admitted.view.Summary == "" {
		admitted.view.Summary = entry.Meta.ShortHelp
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = []string{input.ContentArg}
	}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if field == "" {
			return nil, fmt.Errorf("%s: a supplied source has no field name", entry.Path)
		}
		if seen[field] {
			return nil, fmt.Errorf("%s: a supplied source field is repeated", entry.Path)
		}
		if field == webCommandField {
			return nil, fmt.Errorf("%s: a supplied source collides with the command selection", entry.Path)
		}
		seen[field] = true
		fileField := field + "-file"
		if len(input.Fields) == 0 {
			fileField = "file"
		}
		admitted.view.Sources = append(admitted.view.Sources, GraphicalSuppliedSource{
			Label: humanizeFieldLabel(field), Field: field, FileField: fileField,
			ContentID: "graphical-supplied-" + field, FileID: "graphical-supplied-" + fileField,
		})
	}
	for _, def := range defs {
		if def.Name() == input.ContentArg {
			continue
		}
		if seen[def.Name()] {
			return nil, fmt.Errorf("%s: a source field collides with a declared argument", entry.Path)
		}
		admitted.view.Options = append(admitted.view.Options, GraphicalSuppliedOption{
			Name: def.Name(), Label: def.ShortHelp(), Description: def.Description(),
			Flag: def.Kind() == command.ArgFlag, Required: def.Mandatory(),
			Choices: slices.Collect(def.EnumValues()),
		})
	}
	if input.NameArg != "" {
		found := false
		for _, def := range defs {
			if def.Name() == input.NameArg {
				if def.Kind() != command.ArgString {
					return nil, fmt.Errorf("%s: its diagnostic label is not a string", entry.Path)
				}
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("%s: its diagnostic label is absent from the model", entry.Path)
		}
	}
	return admitted, nil
}

func suppliedInspectionSelection(trees AdminCommandTree, selected string) (*suppliedInspectionCommand, error) {
	if len(selected) > maxCommandLength {
		return nil, suppliedInspectionRefusal("The inspection selection is too long.", nil)
	}
	commands, err := suppliedInspectionCommands(trees)
	if err != nil {
		return nil, suppliedInspectionRefusal("The inspection choices are unavailable.", err)
	}
	for _, registered := range commands {
		if registered.path == selected {
			return registered, nil
		}
	}
	return nil, suppliedInspectionRefusal("The selected command is not an offered supplied-input inspection.", nil)
}

func suppliedInspectionNumber(value int) string {
	digits := strconv.Itoa(value)
	var output strings.Builder
	for index, digit := range digits {
		if index > 0 {
			if (len(digits)-index)%3 == 0 {
				output.WriteByte(',')
			}
		}
		output.WriteRune(digit)
	}
	return output.String()
}
