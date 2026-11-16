//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- browser config carriers.

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/test/labconfig"
)

// browserCorpusLoad is the content form opened by a parsed load command. Its
// payload is absent until the workflow supplies the selected file or paste.
type browserCorpusLoad struct {
	source  string
	step    loadStep
	payload bool
}

// browserCorpus owns one offline editor while reading a browser workflow.
// The caller MUST close editor after replay; it never dispatches daemon actions.
type browserCorpus struct {
	editor    *editorcli.Editor
	path      []string
	input     string
	pending   *browserCorpusLoad
	sequences []loadSequence
	applied   []sweepCarrier
}

// browserSequences derives file/paste composition from parsed actions. A source
// is consumed only by a recognized load form followed by its Apply action;
// unknown composition leaves standalone validation active and records a finding.
func (s *sweepContext) browserSequences(path string, loaded map[string]bool) []loadSequence {
	body, err := os.ReadFile(path)
	if err != nil {
		s.report.find(path + ": " + err.Error())
		return nil
	}
	workflow, err := webtesting.ParseWBFile(string(body))
	if err != nil {
		s.report.find(path + ": " + err.Error())
		return nil
	}
	last := -1
	for i, action := range workflow.Actions {
		if action.Kind == "click" {
			if action.Values["id"] == "cli-load-apply" {
				last = i
			}
		}
	}
	if last < 0 {
		return nil
	}
	last = browserCommitBoundary(workflow.Actions, last)
	if workflow.ServerKind() != webtesting.WBServerDaemon {
		s.report.find(path + ": browser load composition requires the daemon baseline; the selected server has no declared configuration")
		return nil
	}
	editor, err := editorcli.NewEditorFromContent(labconfig.WebDaemon(workflow.Plugins), path)
	if err != nil {
		s.report.find(path + ": open browser baseline: " + err.Error())
		return nil
	}
	defer func() {
		if err := editor.Close(); err != nil {
			s.t.Errorf("close browser corpus editor: %v", err)
		}
	}()
	reader := &browserCorpus{editor: editor}
	for i := range last + 1 {
		action := &workflow.Actions[i]
		if err := reader.action(s.root, path, action, loaded); err != nil {
			s.report.find(path + ":line " + strconv.Itoa(action.Line) + ": " + err.Error())
			break
		}
	}
	for i := range reader.applied {
		s.judgeCarrier(&reader.applied[i])
	}
	return reader.sequences
}

// browserCommitBoundary extends replay only for an actually submitted commit.
// Filled text is not a command until the parsed Enter action submits it.
func browserCommitBoundary(actions []webtesting.WBAction, last int) int {
	input := ""
	for i, action := range actions {
		if action.Values["id"] != "terminal-input" {
			continue
		}
		if action.Kind == "fill" {
			input = action.Values["value"]
			continue
		}
		if action.Kind != "press" {
			continue
		}
		if action.Values["key"] != "Enter" {
			continue
		}
		words, err := editorcli.ParseCommandWords(input)
		input = ""
		if err != nil {
			continue // A malformed line cannot be an accepted commit command.
		}
		tokens := words.Tokens()
		// Every top consumes a token of this finite, parsed command.
		for len(tokens) > 1 {
			if tokens[0] != "top" {
				break
			}
			tokens = tokens[1:]
		}
		if len(tokens) == 0 {
			continue
		}
		if tokens[0] == "commit" {
			if i > last {
				last = i
			}
		}
	}
	return last
}

// action interprets only the parsed browser controls that carry config into
// the CLI. Other browser actions do not change this private candidate.
func (b *browserCorpus) action(root, source string, action *webtesting.WBAction, loaded map[string]bool) error {
	id := action.Values["id"]
	switch action.Kind {
	case "open":
		if action.Values["path"] != "/cli" {
			return fmt.Errorf("page %q before a browser load has no corpus composition", action.Values["path"])
		}
		b.path = nil
		b.input = ""
		b.pending = nil
	case "fill":
		switch id {
		case "terminal-input":
			b.input = action.Values["value"]
		case "cli-load-text":
			if b.pending == nil {
				return errors.New("paste content has no preceding load command")
			}
			if b.pending.source != "terminal" {
				return errors.New("paste content was supplied to a file load")
			}
			b.pending.step.body = action.Values["value"]
			b.pending.payload = true
		default:
			return fmt.Errorf("field %q before a browser load has no corpus composition", id)
		}
	case "press":
		if id == "terminal-input" {
			if action.Values["key"] == "Enter" {
				line := b.input
				b.input = ""
				return b.command(line, source, action.Line)
			}
		}
		return errors.New("key action before a browser load has no corpus composition")
	case "upload":
		if id != "cli-load-file" {
			return fmt.Errorf("upload control %q has no known configuration composition", id)
		}
		return b.upload(root, action.Values["file"])
	case "click":
		if id == "cli-load-cancel" {
			b.pending = nil
		}
		if id == "cli-load-apply" {
			return b.apply(source, action.Line, loaded)
		}
		if id != "cli-load-cancel" {
			return fmt.Errorf("button %q before a browser load has no corpus composition", id)
		}
	case "wait", "hover", "screenshot":
		// These observations neither enter a command nor submit configuration.
	default:
		return fmt.Errorf("browser action %q before a load has no corpus composition", action.Kind)
	}
	return nil
}

func (b *browserCorpus) command(line, source string, number int) error {
	words, err := editorcli.ParseCommandWords(line)
	if err != nil {
		return err
	}
	tokens := words.Tokens()
	path := b.path
	// Each top consumes one token from the parsed, finite command line.
	for len(tokens) > 1 {
		if tokens[0] != "top" {
			break
		}
		tokens = tokens[1:]
		path = nil
	}
	if len(tokens) == 0 {
		return nil
	}
	if tokens[0] == "load" {
		source, location, operation, _, err := editorcli.ParseLoadArgs(tokens[1:])
		if err != nil {
			return err
		}
		if location == "absolute" {
			path = nil
		}
		b.pending = &browserCorpusLoad{source: source, step: loadStep{path: slices.Clone(path), replace: operation == "replace"}}
		return nil
	}
	switch tokens[0] {
	case "commit":
		request, err := contract.ParseCommit(tokens[1:])
		if err != nil {
			return err
		}
		switch request.Action {
		case contract.CommitNow, contract.CommitConfirmed, contract.CommitVerify:
			b.applied = append(b.applied, sweepCarrier{
				source: source + ":commit line " + strconv.Itoa(number), body: b.editor.WorkingContent(),
			})
			return nil
		case contract.CommitAccept, contract.CommitAbort:
			return errors.New("confirmed-window lifecycle before a browser load has no corpus composition")
		case contract.CommitActionUnspecified:
			return errors.New("browser commit has no parsed subcommand")
		default:
			panic("BUG: parsed commit action is unknown")
		}
	case "configure", "show", "help", "errors", "history":
		return nil // These commands change no candidate or configuration position.
	case "set", "delete", "edit", "top", "up", "copy", "rename", "activate", "deactivate", "insert", "option":
		answer, err := b.editor.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: line, Path: b.path})
		if err != nil {
			return err
		}
		b.path = answer.Path
		return nil
	default:
		return fmt.Errorf("command %q before a browser load has no corpus composition", tokens[0])
	}
}

func (b *browserCorpus) upload(root, file string) error {
	if b.pending == nil {
		return errors.New("uploaded configuration has no preceding load command")
	}
	if b.pending.source != "file" {
		return errors.New("uploaded configuration was supplied to a terminal load")
	}
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(file))
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("uploaded configuration %s is not a regular file", file)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	b.pending.step.source = filepath.Clean(path)
	b.pending.step.body = string(body)
	b.pending.payload = true
	return nil
}

func (b *browserCorpus) apply(source string, line int, loaded map[string]bool) error {
	if b.pending == nil {
		return errors.New("load apply has no preceding load command")
	}
	if !b.pending.payload {
		return errors.New("load apply has no selected file or pasted content")
	}
	step := b.pending.step
	if b.pending.source == "file" {
		loaded[step.source] = true
	} else {
		step.source = source + ":paste line " + strconv.Itoa(line)
	}
	b.sequences = append(b.sequences, loadSequence{source: source, base: b.editor.WorkingContent(), steps: []loadStep{step}, draft: true})
	// Validation reports a rejected payload through judgeSequence. Replaying a
	// rejected load leaves this editor unchanged before any later load, as live.
	if err := loadCorpusStep(b.editor, &step); err != nil {
		b.pending = nil
		return nil
	}
	b.pending = nil
	return nil
}
