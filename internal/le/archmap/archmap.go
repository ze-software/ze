// Design: docs/architecture/core-design.md -- the architecture lists, generated
//
// Package archmap owns the component and plugin inventories inside
// ai/INSTRUCTIONS.md. Those lists drift whenever a directory is added, moved or
// removed, and a hand-maintained list in an auto-loaded instruction file went
// stale without anybody noticing: the instructions named components that no
// longer existed.
//
// So the volatile lists are generated and the prose around them is not. Each
// list sits between a marker pair, and only what is between the markers is
// ever rewritten.
//
// Detail: report.go holds the answer, actions.go the command surface.
package archmap

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// instructionsFile is the one file this tool rewrites, and lineWidth is the
// column the name lists wrap at.
const (
	instructionsFile = "ai/INSTRUCTIONS.md"
	lineWidth        = 78
)

// source is one generated list: the name its markers carry, and the directory
// whose immediate subdirectories it holds.
type source struct {
	Name string
	Path string
}

// sources are the three lists, in the order they are rewritten. The order
// reaches the answer, because the blocks of the report are reported in it.
var sources = []source{
	{Name: "components", Path: "internal/component"},
	{Name: "system-plugins", Path: "internal/plugins"},
	{Name: "bgp-plugins", Path: "internal/component/bgp/plugins"},
}

// Dirs answers the immediate subdirectory names of base, sorted.
//
// A base that cannot be read is an error rather than an empty list. An empty
// list would render as a block saying zero directories, which is a claim about
// the tree rather than a report that the tree could not be read.
func Dirs(base string) ([]string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, item := range entries {
		if item.IsDir() {
			names = append(names, item.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Wrap greedily fills text to width, breaking only at a space.
//
// A name is never split, whatever it holds and however long it is: the words
// here are directory names, and half of one on each of two lines is not a name
// a reader can look up. That is what the script asked textwrap for with
// break_long_words=False and break_on_hyphens=False.
func Wrap(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var tb textbuf.Buffer
	line := len(words[0])
	tb.Str(words[0])
	for _, word := range words[1:] {
		if line+1+len(word) > width {
			tb.Byte('\n').Str(word)
			line = len(word)
			continue
		}
		tb.Byte(' ').Str(word)
		line += 1 + len(word)
	}
	return tb.String()
}

// block renders one generated list: the count, the path it came from, and the
// wrapped names.
func block(tree string, from source) (string, Block, error) {
	names, err := Dirs(filepath.Join(tree, filepath.FromSlash(from.Path)))
	if err != nil {
		return "", Block{}, err
	}

	var tb textbuf.Buffer
	tb.Int(int64(len(names))).Str(" directories under `").Str(from.Path).Str("/`:\n\n")
	tb.Str(Wrap(strings.Join(names, ", "), lineWidth)).Byte('\n')
	return tb.String(), Block{Name: from.Name, Path: from.Path, Directories: len(names)}, nil
}

// Render answers what the instructions file should hold, and what each block
// now carries.
//
// A missing marker pair is an error. The rewrite is bounded by those markers,
// so a file that does not carry them is not a file this tool knows how to
// edit, and guessing where the list belongs would rewrite prose.
func Render(tree, content string) (string, []Block, error) {
	blocks := make([]Block, 0, len(sources))

	for _, from := range sources {
		var tb textbuf.Buffer
		begin := tb.Str("<!-- BEGIN GENERATED: arch-").Str(from.Name).String()
		tb.Reset()
		end := tb.Str("<!-- END GENERATED: arch-").Str(from.Name).Str(" -->").String()

		start := strings.Index(content, begin)
		stop := strings.Index(content, end)
		if start == -1 || stop == -1 {
			tb.Reset()
			return "", nil, errors.New(tb.Str(instructionsFile).Str(": marker pair for arch-").
				Str(from.Name).Str(" not found").String())
		}
		// The BEGIN marker carries a comment of its own, so the rewrite starts
		// after the comment CLOSES rather than after the marker's name.
		closing := strings.Index(content[start:], "-->")
		if closing == -1 {
			tb.Reset()
			return "", nil, errors.New(tb.Str(instructionsFile).Str(": the BEGIN marker for arch-").
				Str(from.Name).Str(" is not closed").String())
		}
		open := start + closing + len("-->")
		if stop < open {
			tb.Reset()
			return "", nil, errors.New(tb.Str(instructionsFile).Str(": the markers for arch-").
				Str(from.Name).Str(" are in the wrong order").String())
		}

		body, info, err := block(tree, from)
		if err != nil {
			return "", nil, err
		}
		blocks = append(blocks, info)

		tb.Reset()
		content = tb.Str(content[:open]).Byte('\n').Str(body).Str(content[stop:]).String()
	}
	return content, blocks, nil
}

// Check reports whether the instructions file is current, and writes nothing.
func Check(tree string) (Report, error) {
	report, _, err := judge(tree)
	return report, err
}

// Update rewrites the instructions file when it is stale, and answers what it
// did. A file that is already current is left alone, so a run that changes
// nothing has no mtime to explain.
func Update(tree string) (Report, error) {
	report, rendered, err := judge(tree)
	if err != nil || !report.Stale {
		return report, err
	}

	path := filepath.Join(tree, filepath.FromSlash(instructionsFile))
	if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
		return Report{}, err
	}
	report.Written = true
	return report, nil
}

// judge reads the file, renders what it should hold, and answers both.
func judge(tree string) (Report, string, error) {
	path := filepath.Join(tree, filepath.FromSlash(instructionsFile))
	current, err := os.ReadFile(path) // #nosec G304 -- a fixed path under the checkout
	if err != nil {
		return Report{}, "", err
	}

	rendered, blocks, err := Render(tree, string(current))
	if err != nil {
		return Report{}, "", err
	}
	return Report{
		File:   instructionsFile,
		Blocks: blocks,
		Stale:  rendered != string(current),
	}, rendered, nil
}
