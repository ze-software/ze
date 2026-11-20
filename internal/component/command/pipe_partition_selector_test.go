// Design: docs/architecture/api/commands.md — the pipe chain: declared operations and the residual chain
// Related: invocation_path.go — invocationReachesCommandPath, the selector-aware declaration match
// Related: pipe_partition.go — the partition every route runs

package command

import (
	"slices"
	"testing"
)

// selectorRowsPath is a command whose model declares a selector inside its
// path, typed between `sel` and `rows` as the peer selector of
// `show bgp peer <selector> rib` is.
const selectorRowsPath = "show sel rows"

// withSelectorRowsCommand declares received and count on selectorRowsPath and
// installs a model that anchors one mandatory selector to its `sel` keyword,
// and declares nothing for any other path.
func withSelectorRowsCommand(t *testing.T) {
	t.Helper()
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{selectorRowsPath},
		PipeFilter{Name: "received", Description: "received rows"},
		PipeFilter{Name: "count", Description: "count rows at the source"},
	)
	selector, err := NewStringArg("selector", nil, nil, ArgOptions{Mandatory: true, Anchor: "sel"})
	if err != nil {
		t.Fatalf("build the selector definition: %v", err)
	}
	withArgDefSource(t, func(path string) ([]ArgDef, error) {
		if path == selectorRowsPath {
			return []ArgDef{selector}, nil
		}
		return nil, nil
	})
}

// TestSelectorInsidePathReachesTheDeclaredOperations asks whether a command
// typed with a selector value inside its path hands its declared operations
// to its handler, on the dispatcher's read (ReadDispatchLine) and on a
// surface's own partition (ReadLocalLine), the value bare and quoted.
//
// Method: the command declares received and count on its registered path, and
// the model anchors a selector to the keyword before the value. Each line
// must yield [received, count] in chain order, and no residual refusal.
// PREVENTS: the lookup matching the typed words positionally, so the value
// hides the declaration and `| received` is refused as an unknown operation.
func TestSelectorInsidePathReachesTheDeclaredOperations(t *testing.T) {
	withSelectorRowsCommand(t)
	want := []PipeOperation{{Name: "received"}, {Name: "count"}}
	for _, input := range []string{
		"show sel edge1 rows | received | count",
		`show sel "edge 1" rows | received | count`,
	} {
		t.Run(input, func(t *testing.T) {
			line, err := ReadDispatchLine(input)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if refusal := line.ResidualRefusal(); refusal != nil {
				t.Fatalf("the chain was refused: %v", refusal)
			}
			if got := line.DeclaredOperations(); !slices.EqualFunc(got, want, samePipeOperationValue) {
				t.Errorf("dispatch declared = %+v, want %+v", got, want)
			}
			local, problem := ReadLocalLine(input, "")
			if problem != "" {
				t.Fatalf("the surface refused the chain: %s", problem)
			}
			if got := local.DeclaredOperations(); !slices.EqualFunc(got, want, samePipeOperationValue) {
				t.Errorf("surface declared = %+v, want %+v", got, want)
			}
		})
	}
}

// TestCommandWithoutSelectorKeepsItsDeclaredOperations asks whether the
// selector-aware match left the other lines as they were: the registered path
// typed with no value still hands its list, and a word the model places
// nowhere inside the path still does not reach the declaration.
//
// Method: the same command; the bare path must yield [received, count], and a
// path with an undeclared word inside it must leave received residual, where
// it is refused as unknown.
// PREVENTS: the match skipping any word between two keywords, which would
// hand a command's operations to a line the dispatcher never routes to it.
func TestCommandWithoutSelectorKeepsItsDeclaredOperations(t *testing.T) {
	withSelectorRowsCommand(t)
	line, err := ReadDispatchLine(selectorRowsPath + " | received | count")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []PipeOperation{{Name: "received"}, {Name: "count"}}
	if got := line.DeclaredOperations(); !slices.EqualFunc(got, want, samePipeOperationValue) {
		t.Errorf("declared = %+v, want %+v", got, want)
	}

	ResetPipeFiltersForTest()
	RegisterPipeFilters([]string{"show plain rows"},
		PipeFilter{Name: "received", Description: "received rows"},
	)
	stray, err := ReadDispatchLine("show plain extra rows | received")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := stray.DeclaredOperations(); len(got) != 0 {
		t.Errorf("a word the model declares nowhere reached the declaration: %+v", got)
	}
	if stray.ResidualRefusal() == nil {
		t.Error("received on an undeclared line was not refused as unknown")
	}
}

// samePipeOperationValue compares two operations by name and argument words.
func samePipeOperationValue(a, b PipeOperation) bool {
	return a.Name == b.Name && slices.Equal(a.Args, b.Args)
}
