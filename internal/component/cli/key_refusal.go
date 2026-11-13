// Design: docs/architecture/cli/error-surface.md -- The operator error
// Overview: fix_frame.go -- FrameFix, the one framer of editor fix lines
// Related: model_commands_edit.go -- cmdSet, createListEntry, listEntryMove, cmdInsert and runActivation refuse through these

package cli

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FramedKeyRefusal answers err with its fix replaced by the command the
// operator typed, verb and args relative to at, with the refused key replaced
// by its correction (AC-25, AC-29 to AC-31). A key refusal names the path
// through the refused key (config.ValidateListKey), so the key's place in
// args is found from that path: the word at that place must be the refused
// key, or the line does not hold it and err is answered as it is, as is an
// error that holds no value refusal.
func FramedKeyRefusal(err error, verb string, at, args, operator []string, placeholders ...int) error {
	refusal, ok := valueRefusal(err)
	if !ok {
		return err
	}
	segments := refusal.Segments()
	index := len(segments) - 1 - len(at)
	if index < 0 || index >= len(args) {
		return err
	}
	typed := slices.Concat(at, args)
	if !slices.Equal(typed[:len(segments)], segments) {
		return err
	}
	return FramedWordRefusal(err, verb, at, args, index, operator, placeholders...)
}

// FramedWordRefusal answers err with its fix replaced by the command the
// operator typed, verb and args relative to at, with args[index], the refused
// word, replaced by its correction. An error that holds no value refusal, or
// an index outside args, answers err as it is.
func FramedWordRefusal(err error, verb string, at, args []string, index int, operator []string, placeholders ...int) error {
	refusal, ok := valueRefusal(err)
	if !ok {
		return err
	}
	if index < 0 || index >= len(args) {
		return err
	}
	fix := correctionFix(verb, at, args, index, refusal).withPlaceholders(placeholders...)
	return refusal.WithFix(FrameFix(fix, operator))
}

// InEntryKeyLeafRefusal answers the refusal of a `set` whose path, from the
// root, ends at the first key leaf of a list entry it names (`redistribute
// connected source static`), or nil when path does not. An entry's key is its
// name: the set parser would store the leaf as a second value beside the key,
// and only rename changes the key of an entry that exists (AC-33, AC-34).
// at is the position the operator's words are relative to, operator the
// position the fix is framed for.
func InEntryKeyLeafRefusal(schema *config.Schema, path, at, operator []string) error {
	if len(path) < 2 {
		return nil
	}
	leaf := path[len(path)-1]
	entryPath := path[:len(path)-1]
	entry, endsAtEntry, err := config.WalkListEntryPath(schema, entryPath)
	if err != nil {
		return err
	}
	if !endsAtEntry || entry.End != len(entryPath) || entry.Node.FirstKeyLeafName() != leaf {
		return nil
	}

	var problem textbuf.Buffer
	for i, word := range path {
		if i > 0 {
			problem.Byte(' ')
		}
		problem.Str(configtext.QuoteToken(word))
	}
	problem.Str(": ").Str(leaf).Str(" is the key of list ").Str(entry.ListName).
		Str(" and cannot be set inside an entry")
	var why textbuf.Buffer
	why.Str("rename changes the key of an existing entry, and set would store ").Str(leaf).
		Str(" as a second value")

	listPath := slices.Concat(entry.ParentPath, []string{entry.ListName})
	fix := FixCommand{Verb: cmdRename}
	if len(at) <= len(entry.ParentPath) && slices.Equal(listPath[:len(at)], at) {
		fix.At = at
		fix.Args = slices.Concat(listPath[len(at):], keyWords(entry, entryPath), []string{"to", "<new-key>"})
	} else {
		// The operator stands inside the entry: the rename names it from the
		// root, behind `top` (FrameFix).
		fix.Args = slices.Concat(listPath, keyWords(entry, entryPath), []string{"to", "<new-key>"})
	}
	return opererr.New(problem.String(), why.String(), FrameFix(fix.withPlaceholders(len(fix.Args)-1), operator))
}

// keyWords answers the words of entryPath that address the entry.
func keyWords(entry config.ListEntryTarget, entryPath []string) []string {
	return entryPath[len(entry.ParentPath)+1 : entry.End]
}
