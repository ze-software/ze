package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

// corpusCarrier is one config body the repository's tests feed to ze.
type corpusCarrier struct {
	source string // file path, plus the tmpfs or stdin block name for a .ci body
	body   string
}

// TestFormatCorpusRoundTrip formats every config body the repository's tests
// carry: each `.conf` under test/, and each `.ci` tmpfs file ending `.conf`
// and stdin=config block. test/draft is skipped, as every repo-wide gate does.
//
// VALIDATES: AC-10. For every hierarchical body that parses, the formatted
// text parses, its tree lowers to the same map and prints the same set form
// (values, inactive flags, every list's entry order) as the source's, formatting it
// again changes no byte, and its comment texts are the source's, as a
// multiset (commentTextsOf).
// PREVENTS: fmt writing a file that reads back with another meaning, or one a
// second fmt rewrites. R-6: a walk that silently finds no carriers fails.
func TestFormatCorpusRoundTrip(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}

	carriers := collectCorpusCarriers(t, filepath.Join("..", "..", "..", "..", "test"))
	var hierarchical, parsed, orderedLists, comments int
	for _, c := range carriers {
		if config.DetectFormat(c.body) != config.FormatHierarchical {
			continue
		}
		hierarchical++
		source, err := config.NewParser(schema).Parse(c.body)
		if err != nil {
			continue // A body written to be refused, or one a test feeds another reader.
		}
		parsed++

		// fmtTree is fmt's own path, hashing a plaintext password in source
		// in place, so source then holds the meaning fmt wrote.
		formatted, _, err := fmtTree(source, schema)
		if err != nil {
			t.Errorf("%s: fmt refuses a body that parses: %v", c.source, err)
			continue
		}
		reparsed, err := config.NewParser(schema).Parse(formatted)
		if err != nil {
			t.Errorf("%s: formatted text does not parse: %v\n%s", c.source, err, formatted)
			continue
		}
		if !reflect.DeepEqual(source.ToMap(), reparsed.ToMap()) {
			t.Errorf("%s: formatted text lowers to another map\n%s", c.source, formatted)
		}
		// One set line is one statement, carrying its value, its inactive flag
		// and an ordered leaf-list's whole sequence. The multiset diagnoses
		// changed statements; exact bytes also pin every list's entry order.
		before, after := config.SerializeSet(source, schema), config.SerializeSet(reparsed, schema)
		if only := setLinesOnlyInOne(before, after); only != "" {
			t.Errorf("%s: formatted text prints another set form:%s", c.source, only)
		}
		if before != after {
			t.Errorf("%s: formatted text changes set statement order:\n%s\nwant:\n%s", c.source, after, before)
		}
		// The set-line multiset above cannot see two entries of an
		// `ordered-by user` list swapped, and their order is the value.
		for _, diff := range userOrderedListDiffs(source, reparsed, schema, "", &orderedLists) {
			t.Errorf("%s: %s\n%s", c.source, diff, formatted)
		}
		if again, _, err := fmtTree(reparsed, schema); err != nil || again != formatted {
			t.Errorf("%s: second fmt changes the output:\n%s\nvs\n%s", c.source, formatted, again)
		}
		sourceComments := commentTextsOf(c.body)
		comments += len(sourceComments)
		if formattedComments := commentTextsOf(formatted); !slices.Equal(sourceComments, formattedComments) {
			t.Errorf("%s: formatted text carries other comments: source %q, formatted %q\n%s",
				c.source, sourceComments, formattedComments, formatted)
		}
	}

	t.Logf("carriers %d, hierarchical %d, parsed and checked %d, ordered-by user lists of two or more entries %d, comments %d",
		len(carriers), hierarchical, parsed, orderedLists, comments)
	// The comment check proves nothing unless the corpus carries comments.
	if comments == 0 {
		t.Fatalf("no parsed carrier holds a comment: the comment check saw nothing")
	}
	// The order check proves nothing unless the corpus carries a user-ordered
	// list with two entries or more.
	if orderedLists == 0 {
		t.Fatalf("no carrier holds an ordered-by user list of two entries or more: the order check saw nothing")
	}
	// R-6: the floor is read from this walk, not written down: at least half of
	// the hierarchical bodies the tests carry must parse, or the walk, the
	// schema or the build tags are wrong and the test proves nothing.
	if parsed == 0 || parsed*2 < hierarchical {
		t.Fatalf("only %d of %d hierarchical carriers parse: check the test/ walk and the build tags", parsed, hierarchical)
	}
}

// commentTextsOf answers the text after `#` of every comment in config text,
// sorted, so two texts holding the same comments in another order compare
// equal. A `#` inside a quoted value opens no comment; a backslash inside
// quotes escapes the next byte, as the config tokenizer reads them, and the
// carriage return of a CRLF line end is not part of the comment.
func commentTextsOf(text string) []string {
	var texts []string
	var quote byte
	for i := 0; i < len(text); i++ {
		ch := text[i]
		switch {
		case quote != 0 && ch == '\\':
			i++ // The escaped byte cannot close the quote.
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '"' || ch == '\'':
			quote = ch
		case ch == '#':
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			texts = append(texts, strings.TrimSuffix(text[i+1:i+end], "\r"))
			i += end
		}
	}
	slices.Sort(texts)
	return texts
}

// userOrderedListDiffs walks source and reparsed side by side under the schema
// node cp, at every depth, and names each `ordered-by user` list whose entry
// keys come back in another sequence. The exact set comparison above checks
// every list kind's display order; this walk additionally counts the corpus's
// user-ordered lists, whose entry order is their meaning. checked counts those
// with two entries or more. The recursion is bounded by the schema's depth.
func userOrderedListDiffs(source, reparsed *config.Tree, cp config.ChildProvider, path string, checked *int) []string {
	var diffs []string
	for _, name := range source.ListNames() {
		node, ok := cp.Get(name).(*config.ListNode)
		if !ok {
			continue
		}
		listPath := path + " " + name
		sourceEntries := source.GetListOrdered(name)
		sourceKeys := orderedKeys(sourceEntries)
		reparsedKeys := orderedKeys(reparsed.GetListOrdered(name))
		if node.UserOrdered && len(sourceKeys) > 1 {
			*checked++
			if !slices.Equal(sourceKeys, reparsedKeys) {
				diffs = append(diffs, "ordered-by user list"+listPath+" entry order "+
					strings.Join(sourceKeys, ",")+" reads back as "+strings.Join(reparsedKeys, ","))
			}
		}
		reparsedEntries := reparsed.GetList(name)
		for _, entry := range sourceEntries {
			if other, ok := reparsedEntries[entry.Key]; ok {
				diffs = append(diffs, userOrderedListDiffs(entry.Value, other, node, listPath+" "+entry.Key, checked)...)
			}
		}
	}
	for _, name := range source.ContainerNames() {
		node, ok := cp.Get(name).(config.ChildProvider)
		if !ok {
			continue
		}
		if other := reparsed.GetContainer(name); other != nil {
			diffs = append(diffs, userOrderedListDiffs(source.GetContainer(name), other, node, path+" "+name, checked)...)
		}
	}
	return diffs
}

// orderedKeys lists the entry keys of a list in the order the tree holds them.
func orderedKeys(entries []struct {
	Key   string
	Value *config.Tree
},
) []string {
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.Key)
	}
	return keys
}

// setLinesOnlyInOne lists the set lines only one of the two set forms prints,
// counting repeats, and answers "" when both print the same lines.
func setLinesOnlyInOne(source, formatted string) string {
	sourceLines := strings.Split(source, "\n")
	formattedLines := strings.Split(formatted, "\n")
	count := make(map[string]int, len(sourceLines))
	for _, line := range sourceLines {
		count[line]++
	}
	for _, line := range formattedLines {
		count[line]--
	}
	var only []string
	for line, n := range count {
		if n > 0 {
			only = append(only, "source only: "+line)
		}
		if n < 0 {
			only = append(only, "formatted only: "+line)
		}
	}
	if len(only) == 0 {
		return ""
	}
	slices.Sort(only)
	return "\n" + strings.Join(only, "\n")
}

// collectCorpusCarriers walks root for `.conf` files and `.ci` bodies.
func collectCorpusCarriers(t *testing.T, root string) []corpusCarrier {
	t.Helper()
	var carriers []corpusCarrier
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "draft" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".conf":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			carriers = append(carriers, corpusCarrier{source: path, body: string(data)})
		case ".ci":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			parsed, err := tmpfs.Parse(bytes.NewReader(data))
			if err != nil {
				return nil //nolint:nilerr // A .ci the tmpfs reader refuses carries no body to format.
			}
			for _, f := range parsed.Files {
				if strings.HasSuffix(f.Path, ".conf") {
					carriers = append(carriers, corpusCarrier{source: path + ":" + f.Path, body: string(f.Content)})
				}
			}
			if body, ok := parsed.StdinBlocks["config"]; ok {
				carriers = append(carriers, corpusCarrier{source: path + ":stdin=config", body: string(body)})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return carriers
}
