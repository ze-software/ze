package localdatacoverage

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInPrivateWorkspaceIsolatesAndCleansEveryExit(t *testing.T) {
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Setenv("ZE_CONFIG_DIR", "prior-config-directory")
	scenarioError := errors.New("scenario failed")

	for _, testCase := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "error", err: scenarioError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var workspace string
			err := inPrivateWorkspace(func(work string) error {
				workspace = work
				currentDirectory, getErr := os.Getwd()
				if getErr != nil {
					return getErr
				}
				if currentDirectory != work {
					t.Fatalf("working directory = %q, want private workspace %q", currentDirectory, work)
				}
				if configDirectory := os.Getenv("ZE_CONFIG_DIR"); configDirectory != work {
					t.Fatalf("ZE_CONFIG_DIR = %q, want private workspace %q", configDirectory, work)
				}
				if writeErr := os.WriteFile(filepath.Join(work, "fixture"), []byte("private"), 0o600); writeErr != nil {
					return writeErr
				}
				return testCase.err
			})
			if !errors.Is(err, testCase.err) {
				t.Fatalf("inPrivateWorkspace error = %v, want %v", err, testCase.err)
			}
			currentDirectory, getErr := os.Getwd()
			if getErr != nil {
				t.Fatalf("get restored working directory: %v", getErr)
			}
			if currentDirectory != originalDirectory {
				t.Fatalf("restored working directory = %q, want %q", currentDirectory, originalDirectory)
			}
			if configDirectory := os.Getenv("ZE_CONFIG_DIR"); configDirectory != "prior-config-directory" {
				t.Fatalf("restored ZE_CONFIG_DIR = %q", configDirectory)
			}
			if _, statErr := os.Stat(workspace); !os.IsNotExist(statErr) {
				t.Fatalf("private workspace still exists: %q, stat error %v", workspace, statErr)
			}
		})
	}
}

func TestInPrivateWorkspaceSerializesProcessState(t *testing.T) {
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- inPrivateWorkspace(func(string) error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	secondStarted := make(chan struct{})
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		secondDone <- inPrivateWorkspace(func(string) error {
			close(secondEntered)
			return nil
		})
	}()
	<-secondStarted

	serialized := true
	select {
	case <-secondEntered:
		serialized = false
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first workspace: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second workspace: %v", err)
	}
	if !serialized {
		t.Fatal("a second private workspace entered while the first owned process cwd and environment")
	}
}

func TestInPrivateWorkspaceJoinsScenarioAndCleanupErrors(t *testing.T) {
	callerDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get caller working directory: %v", err)
	}
	parent, err := os.MkdirTemp("", "ze-local-data-cleanup-test-")
	if err != nil {
		t.Fatalf("create disposable caller directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(callerDirectory)
		_ = os.RemoveAll(parent)
	})
	disposableCaller := filepath.Join(parent, "caller")
	if err := os.Mkdir(disposableCaller, 0o700); err != nil {
		t.Fatalf("create caller directory: %v", err)
	}
	if err := os.Chdir(disposableCaller); err != nil {
		t.Fatalf("enter caller directory: %v", err)
	}
	t.Setenv("ZE_CONFIG_DIR", "caller-config")

	scenarioError := errors.New("scenario failed")
	var workspace string
	result := inPrivateWorkspace(func(work string) error {
		workspace = work
		if err := os.RemoveAll(disposableCaller); err != nil {
			return err
		}
		return scenarioError
	})
	if err := os.Chdir(callerDirectory); err != nil {
		t.Fatalf("recover caller working directory: %v", err)
	}

	if !errors.Is(result, scenarioError) {
		t.Fatalf("joined error = %v, want scenario error", result)
	}
	if result == nil || !strings.Contains(result.Error(), "restore working directory") {
		t.Fatalf("joined error omitted cleanup failure: %v", result)
	}
	if configDirectory := os.Getenv("ZE_CONFIG_DIR"); configDirectory != "caller-config" {
		t.Fatalf("restored ZE_CONFIG_DIR = %q", configDirectory)
	}
	if _, statErr := os.Stat(workspace); !os.IsNotExist(statErr) {
		t.Fatalf("private workspace still exists after cleanup error: %q, stat error %v", workspace, statErr)
	}
}

func TestInPrivateWorkspaceRestoresUnsetConfigDirectory(t *testing.T) {
	previous, wasSet := os.LookupEnv("ZE_CONFIG_DIR")
	if err := os.Unsetenv("ZE_CONFIG_DIR"); err != nil {
		t.Fatalf("unset ZE_CONFIG_DIR: %v", err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("ZE_CONFIG_DIR", previous)
		} else {
			_ = os.Unsetenv("ZE_CONFIG_DIR")
		}
	})

	if err := inPrivateWorkspace(func(string) error { return nil }); err != nil {
		t.Fatalf("inPrivateWorkspace: %v", err)
	}
	if value, set := os.LookupEnv("ZE_CONFIG_DIR"); set {
		t.Fatalf("ZE_CONFIG_DIR remained set to %q", value)
	}
}

func TestProtocolDocumentRequiresExactShape(t *testing.T) {
	valid := map[string]any{"protocol": "Hub Architecture", "version": "1.0"}
	if err := validateProtocolDocument(valid); err != nil {
		t.Fatalf("valid protocol: %v", err)
	}
	for name, payload := range map[string]any{
		"not object":  []any{"Hub Architecture", "1.0"},
		"missing":     map[string]any{"protocol": "Hub Architecture"},
		"extra":       map[string]any{"protocol": "Hub Architecture", "version": "1.0", "status": "ok"},
		"wrong value": map[string]any{"protocol": "Hub Architecture", "version": "2.0"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProtocolDocument(payload); err == nil {
				t.Fatal("malformed protocol document was accepted")
			}
		})
	}
}

func TestCountDocumentRequiresExactShape(t *testing.T) {
	valid := map[string]any{"count": float64(3), "pipe": []any{map[string]any{"op": "count"}}}
	if err := validateCountDocument(valid, 3); err != nil {
		t.Fatalf("valid count document: %v", err)
	}
	for name, payload := range map[string]any{
		"document extra":  map[string]any{"count": float64(3), "pipe": []any{map[string]any{"op": "count"}}, "rows": float64(3)},
		"step extra":      map[string]any{"count": float64(3), "pipe": []any{map[string]any{"op": "count", "arg": "all"}}},
		"wrong count":     map[string]any{"count": float64(2), "pipe": []any{map[string]any{"op": "count"}}},
		"wrong op":        map[string]any{"count": float64(3), "pipe": []any{map[string]any{"op": "sum"}}},
		"multiple steps":  map[string]any{"count": float64(3), "pipe": []any{map[string]any{"op": "count"}, map[string]any{"op": "json"}}},
		"not object":      []any{float64(3)},
		"missing count":   map[string]any{"pipe": []any{map[string]any{"op": "count"}}},
		"missing pipe":    map[string]any{"count": float64(3)},
		"scalar pipe":     map[string]any{"count": float64(3), "pipe": "count"},
		"non-object step": map[string]any{"count": float64(3), "pipe": []any{"count"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateCountDocument(payload, 3); err == nil {
				t.Fatal("malformed count document was accepted")
			}
		})
	}
}

func TestRowsRejectMalformedMembers(t *testing.T) {
	valid := map[string]any{"key": "wanted"}
	for name, malformed := range map[string]any{
		"scalar after valid row": "not-a-row",
		"null after valid row":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			payload := map[string]any{"rows": []any{valid, malformed}}
			if _, err := rows(payload, "rows"); err == nil {
				t.Fatal("mixed valid and malformed rows were accepted")
			}
		})
	}
}

func TestAnyRowRejectsMalformedMembersBeforePredicate(t *testing.T) {
	valid := map[string]any{"key": "wanted"}
	for name, malformed := range map[string]any{
		"scalar after match": "not-a-row",
		"null after match":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			predicateCalled := false
			matched, err := anyRow([]any{valid, malformed}, func(row map[string]any) bool {
				predicateCalled = true
				return row["key"] == "wanted"
			})
			if err == nil {
				t.Fatalf("anyRow() = %v, nil; want malformed-row error", matched)
			}
			if predicateCalled {
				t.Fatal("predicate ran before the complete row list was validated")
			}
		})
	}
}

func TestTreeNodesRejectMalformedNodesAndChildren(t *testing.T) {
	valid := []any{map[string]any{"name": "root", "children": []any{map[string]any{"name": "leaf"}}}}
	nodes, err := treeNodes(valid)
	if err != nil {
		t.Fatalf("valid tree: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("flattened nodes = %d, want 2", len(nodes))
	}
	for name, values := range map[string][]any{
		"non-object node":             {"leaf"},
		"scalar after valid node":     {map[string]any{"name": "root"}, "leaf"},
		"null after valid node":       {map[string]any{"name": "root"}, nil},
		"non-array children":          {map[string]any{"name": "root", "children": map[string]any{}}},
		"non-object descendant":       {map[string]any{"name": "root", "children": []any{"leaf"}}},
		"null descendant after valid": {map[string]any{"name": "root", "children": []any{map[string]any{"name": "leaf"}, nil}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := treeNodes(values); err == nil {
				t.Fatal("malformed tree was accepted")
			}
		})
	}
}

func TestCollisionRowsRequireNamedObjectSiblings(t *testing.T) {
	valid := []any{map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, map[string]any{"name": "shutdown"}}}}
	total, err := collisionAffected(valid)
	if err != nil {
		t.Fatalf("valid collisions: %v", err)
	}
	if total != 2 {
		t.Fatalf("affected siblings = %d, want 2", total)
	}
	validRow := map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, map[string]any{"name": "shutdown"}}}
	for name, values := range map[string][]any{
		"non-object row":          {"collision"},
		"scalar after valid row":  {validRow, "collision"},
		"null after valid row":    {validRow, nil},
		"missing siblings":        {map[string]any{"max-chars": float64(2)}},
		"scalar siblings":         {map[string]any{"max-chars": float64(2), "siblings": "show"}},
		"null siblings":           {map[string]any{"max-chars": float64(2), "siblings": nil}},
		"scalar sibling member":   {map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, "shutdown"}}},
		"null sibling member":     {map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, nil}}},
		"missing sibling name":    {map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, map[string]any{}}}},
		"empty sibling name":      {map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, map[string]any{"name": ""}}}},
		"non-string sibling name": {map[string]any{"max-chars": float64(2), "siblings": []any{map[string]any{"name": "show"}, map[string]any{"name": float64(2)}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := collisionAffected(values); err == nil {
				t.Fatal("malformed collision was accepted")
			}
		})
	}
}

func TestEvidenceAndMarkersAreTheCurrentPopulation(t *testing.T) {
	want := []Invocation{
		{Command: "show config dump %s | json compact", Evidence: "show config dump"},
		{Command: "show config history pipe-local.conf | json compact", Evidence: "show config history"},
		{Command: "show config ls | json compact", Evidence: "show config ls"},
		{Command: "show schema list | json compact", Evidence: "show schema list"},
		{Command: "show schema methods | json compact", Evidence: "show schema methods"},
		{Command: "show schema events | count | json compact", Evidence: "show schema events"},
		{Command: "show schema handlers | count | json compact", Evidence: "show schema handlers"},
		{Command: "show schema protocol | json compact", Evidence: "show schema protocol"},
		{Command: "show data ls --path %s | json compact", Evidence: "show data ls"},
		{Command: "show data registered | json compact", Evidence: "show data registered"},
		{Command: "show yang tree --commands | json compact", Evidence: "show yang tree"},
		{Command: "show yang tree --config | json compact", Evidence: "show yang tree"},
		{Command: "show yang completion --min-prefix 2 | json compact", Evidence: "show yang completion"},
		{Command: "show env list | json compact", Evidence: "show env list"},
		{Command: "show env get ze.cli.format | json compact", Evidence: "show env get"},
		{Command: "show env registered | json compact", Evidence: "show env registered"},
		{Command: "show plugins | json compact", Evidence: "show plugins"},
	}
	if got := Evidence(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Evidence() = %#v, want %#v", got, want)
	}
	if CompletionMarker != "OK: 16/16 local-data commands and local one-shot save" {
		t.Fatalf("CompletionMarker = %q", CompletionMarker)
	}
	for _, invocation := range want {
		if got, marker := Marker(invocation.Evidence), "COVERED: "+invocation.Evidence+" [done]"; got != marker {
			t.Fatalf("Marker(%q) = %q, want %q", invocation.Evidence, got, marker)
		}
	}
}

func TestMarkerIsPrefixSafe(t *testing.T) {
	shorter := Marker("show config")
	longer := Marker("show config history")
	if strings.HasPrefix(longer, shorter) {
		t.Fatalf("short evidence marker %q is a prefix of %q", shorter, longer)
	}
	if !strings.HasSuffix(shorter, " [done]") || !strings.HasSuffix(longer, " [done]") {
		t.Fatalf("markers do not carry terminal delimiters: %q, %q", shorter, longer)
	}
}
