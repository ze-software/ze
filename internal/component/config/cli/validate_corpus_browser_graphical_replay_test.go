//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- native graphical import replay.

package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/web"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
)

// browserCorpusGraphical is one opened native import panel. Navigation creates
// its selected path and distinct file/paste forms; no CLI load command opens it.
// The zero value means the root selection; nil means no graphical page is open.
type browserCorpusGraphical struct {
	path  []string
	file  browserCorpusGraphicalForm
	paste browserCorpusGraphicalForm
}

// browserCorpusGraphicalForm records controls the browser actually selected.
// Opening its panel initializes both choices from the native form's authority;
// the zero value has no admitted choices and graphicalApply refuses it.
type browserCorpusGraphicalForm struct {
	load      browserCorpusLoad
	location  string
	operation string
}

// graphicalAction handles only controls owned by the native import panel.
// Unrecognized controls return unhandled so the caller retains its explicit
// unknown-composition finding and the physical file's standalone oracle.
func (b *browserCorpus) graphicalAction(root, source string, action *webtesting.WBAction, loaded map[string]bool) (bool, error) {
	id := action.Values["id"]
	if action.Kind == "open" {
		where := action.Values["path"]
		if !strings.HasPrefix(where, "/show/") {
			return false, nil
		}
		path, err := web.DecodeConfigPath(strings.TrimPrefix(where, "/show/"))
		if err != nil {
			return true, err
		}
		location, operation := web.GraphicalImportDefaults()
		b.graphical = &browserCorpusGraphical{
			path: path,
			file: browserCorpusGraphicalForm{
				load: browserCorpusLoad{source: "file"}, location: location, operation: operation,
			},
			paste: browserCorpusGraphicalForm{
				load: browserCorpusLoad{source: "terminal"}, location: location, operation: operation,
			},
		}
		b.pending = nil
		b.input = ""
		return true, nil
	}
	if !strings.HasPrefix(id, "graphical-") {
		return false, nil
	}
	if b.graphical == nil {
		return true, errors.New("graphical import control has no selected graphical page")
	}
	if action.Kind == "click" {
		switch id {
		case "graphical-actions-toggle", "graphical-edit-toggle", "graphical-import-toggle":
			return true, nil // Opening native details does not mutate the candidate.
		case "graphical-import-file-submit":
			return true, b.graphicalApply(&b.graphical.file, source, action.Line, loaded)
		case "graphical-import-paste-submit":
			return true, b.graphicalApply(&b.graphical.paste, source, action.Line, loaded)
		}
		return false, nil
	}
	if action.Kind == "fill" {
		if id == "graphical-import-paste" {
			b.graphical.paste.load.step.body = action.Values["value"]
			b.graphical.paste.load.payload = true
			return true, nil
		}
		return false, nil
	}
	if action.Kind == "upload" {
		if id == "graphical-import-file" {
			return true, b.graphical.file.load.upload(root, action.Values["file"])
		}
		return false, nil
	}
	if action.Kind != "select" {
		return false, nil
	}
	value := action.Values["value"]
	switch id {
	case "graphical-import-file-location":
		b.graphical.file.location = value
	case "graphical-import-paste-location":
		b.graphical.paste.location = value
	case "graphical-import-file-action":
		b.graphical.file.operation = value
	case "graphical-import-paste-action":
		b.graphical.paste.operation = value
	default:
		return false, nil
	}
	return true, nil
}

func (b *browserCorpus) graphicalApply(form *browserCorpusGraphicalForm, source string, line int, loaded map[string]bool) error {
	if err := b.authorize("load terminal " + form.location + " " + form.operation); err != nil {
		return err
	}
	switch form.location {
	case "absolute":
		form.load.step.path = nil
	case "relative":
		form.load.step.path = slices.Clone(b.graphical.path)
	default:
		return fmt.Errorf("graphical import has unknown location %q", form.location)
	}
	switch form.operation {
	case "merge":
		form.load.step.replace = false
	case "replace":
		form.load.step.replace = true
	default:
		return fmt.Errorf("graphical import has unknown operation %q", form.operation)
	}
	b.pending = &form.load
	return b.apply(source, line, loaded)
}

// browserImportErrorDeclaration binds only the next parsed step, never a later
// page's error text. A sole element:text assertion must name the entire actual
// visible problem; unrelated qualifiers cannot silently claim the same refusal.
func browserImportErrorDeclaration(workflow *webtesting.WBTestCase, index int) string {
	step := workflow.Steps[index]
	action := &workflow.Actions[step.ActionIndex]
	if action.Kind != "click" {
		return ""
	}
	switch action.Values["id"] {
	case "graphical-import-file-submit", "graphical-import-paste-submit", "graphical-supplied-submit":
	default:
		return ""
	}
	if index+1 >= len(workflow.Steps) {
		return ""
	}
	next := workflow.Steps[index+1]
	if next.Type != webtesting.WBStepExpect {
		return ""
	}
	expectation := &workflow.Expects[next.ExpectIndex]
	if expectation.Line <= action.Line {
		return ""
	}
	if expectation.Kind != "element" {
		return ""
	}
	if len(expectation.Values) != 1 {
		return ""
	}
	return expectation.Values["text"]
}
