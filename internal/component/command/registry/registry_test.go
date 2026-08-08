package registry

import (
	"errors"
	"testing"
)

func TestListRootBySection(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	RegisterRoot("cli", Meta{Description: "CLI", Section: SectionOperations})
	RegisterRoot("config", Meta{Description: "Config", Section: SectionConfiguration})
	RegisterRoot("doctor", Meta{Description: "Doctor", Section: SectionSystem})
	RegisterRoot("schema", Meta{Description: "Schema", Section: SectionConfiguration})
	RegisterRoot("orphan", Meta{Description: "No section set"})

	sections := ListRootBySection()

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	// Verify order: operations, configuration, system.
	wantOrder := []string{SectionOperations, SectionConfiguration, SectionSystem}
	for i, se := range sections {
		if se.Section != wantOrder[i] {
			t.Errorf("section %d: want %q, got %q", i, wantOrder[i], se.Section)
		}
	}

	// Operations has 1 command.
	if len(sections[0].Commands) != 1 || sections[0].Commands[0].Name != "cli" {
		t.Errorf("operations: want [cli], got %v", sections[0].Commands)
	}

	// Configuration has 2 commands, sorted.
	if len(sections[1].Commands) != 2 {
		t.Fatalf("configuration: want 2 commands, got %d", len(sections[1].Commands))
	}
	if sections[1].Commands[0].Name != "config" || sections[1].Commands[1].Name != "schema" {
		t.Errorf("configuration: want [config, schema], got [%s, %s]",
			sections[1].Commands[0].Name, sections[1].Commands[1].Name)
	}

	// System has doctor + orphan (empty section defaults to system).
	if len(sections[2].Commands) != 2 {
		t.Fatalf("system: want 2 commands, got %d", len(sections[2].Commands))
	}
	names := map[string]bool{}
	for _, c := range sections[2].Commands {
		names[c.Name] = true
	}
	if !names["doctor"] || !names["orphan"] {
		t.Errorf("system: want doctor+orphan, got %v", sections[2].Commands)
	}
}

func TestSectionTitle(t *testing.T) {
	if got := SectionTitle(SectionOperations); got == "" {
		t.Error("SectionTitle(SectionOperations) returned empty")
	}
	if got := SectionTitle("nonexistent"); got != "" {
		t.Errorf("SectionTitle(nonexistent) = %q, want empty", got)
	}
}

func okRootHandler(_ *RuntimeContext, _ []string) int { return 0 }

func TestRegisterRootHandlerRejectsEmptyName(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	err := RegisterRootHandler("", okRootHandler, Meta{})
	if !errors.Is(err, ErrRootHandlerEmptyName) {
		t.Fatalf("expected ErrRootHandlerEmptyName, got %v", err)
	}
	if HasRootHandler("") {
		t.Error("empty-name handler should not be registered")
	}
}

func TestRegisterRootHandlerRejectsNilHandler(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	err := RegisterRootHandler("interface", nil, Meta{})
	if !errors.Is(err, ErrRootHandlerNilHandler) {
		t.Fatalf("expected ErrRootHandlerNilHandler, got %v", err)
	}
	if HasRootHandler("interface") {
		t.Error("nil handler should not be registered")
	}
}

func TestRegisterRootHandlerRejectsDuplicateOwner(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	if err := RegisterRootHandler("interface", okRootHandler, Meta{Description: "first"}); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	secondCalled := false
	second := func(_ *RuntimeContext, _ []string) int { secondCalled = true; return 9 }
	err := RegisterRootHandler("interface", second, Meta{Description: "second"})
	if !errors.Is(err, ErrRootHandlerDuplicate) {
		t.Fatalf("expected ErrRootHandlerDuplicate, got %v", err)
	}

	// The first registration must still win; the second must not have replaced it.
	h := LookupRoot("interface")
	if h == nil {
		t.Fatal("LookupRoot(interface) returned nil after duplicate attempt")
	}
	if code := h(nil, nil); code != 0 || secondCalled {
		t.Errorf("duplicate handler overrode the original: code=%d secondCalled=%v", code, secondCalled)
	}
}

func TestRegisterRootHandlerAlsoRegistersMeta(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	meta := Meta{Description: "Manage OS network interfaces", Section: SectionConfiguration}
	if err := RegisterRootHandler("interface", okRootHandler, meta); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Metadata must flow into the same root-command listing used by help, so an
	// owner-backed root appears in `ze help` exactly like a metadata-only root.
	roots := ListRoot()
	var found *RootCommand
	for i := range roots {
		if roots[i].Name == "interface" {
			found = &roots[i]
			break
		}
	}
	if found == nil {
		t.Fatal("owner-backed root absent from ListRoot()")
	}
	if found.Meta.Description != meta.Description || found.Meta.Section != meta.Section {
		t.Errorf("ListRoot meta = %+v, want %+v", found.Meta, meta)
	}
}

func TestLookupRootReturnsNilWhenUnregistered(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	if LookupRoot("nope") != nil {
		t.Error("LookupRoot for unregistered name should be nil")
	}
}

func TestLookupRootDispatchPreservesArgOrder(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	var gotArgs []string
	handler := func(_ *RuntimeContext, args []string) int {
		gotArgs = args
		return 42
	}
	if err := RegisterRootHandler("interface", handler, Meta{}); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	in := []string{"set", "eth0", "mtu", "9000"}
	code := LookupRoot("interface")(&RuntimeContext{}, in)
	if code != 42 {
		t.Errorf("exit code = %d, want 42", code)
	}
	if len(gotArgs) != len(in) {
		t.Fatalf("handler received %d args, want %d", len(gotArgs), len(in))
	}
	for i := range in {
		if gotArgs[i] != in[i] {
			t.Errorf("arg %d = %q, want %q", i, gotArgs[i], in[i])
		}
	}
}

func TestStorageAsTypeAssertsRuntimeStorage(t *testing.T) {
	type fakeStore struct{ name string }

	rctx := &RuntimeContext{ResolveStorage: func() any { return fakeStore{name: "blob"} }}
	got, ok := StorageAs[fakeStore](rctx)
	if !ok || got.name != "blob" {
		t.Fatalf("StorageAs = (%+v, %v), want ({blob}, true)", got, ok)
	}

	// Wrong type yields zero value and false, not a panic.
	if _, ok := StorageAs[*fakeStore](rctx); ok {
		t.Error("StorageAs to a mismatched type should report false")
	}

	// Nil context and nil resolver are safe.
	if _, ok := StorageAs[fakeStore](nil); ok {
		t.Error("StorageAs(nil) should report false")
	}
	if _, ok := StorageAs[fakeStore](&RuntimeContext{}); ok {
		t.Error("StorageAs with nil ResolveStorage should report false")
	}
}

// VALIDATES: LookupLocal refuses a handler registered ABOVE a declared command
// the same argv reaches, keeps every trailing word that declares nothing, and
// serves no handler at all when it has no declaration source to judge with.
// PREVENTS: a short local registration capturing the whole subtree below it.
// `show interface` (internal/component/iface/cli/register.go) is registered at
// two words and ze-iface-interface-cmd.yang declares seven commands under it,
// so `ze show interface brief` reached cmdShow (internal/component/iface/cli/
// show.go), which reads its first argument as an interface NAME and looked for
// an interface called "brief". All seven were published in
// docs/guide/command-reference.md and none of them worked.
//
// THE CASES ARE SYNTHETIC BECAUSE THE RULE IS. This package is a leaf and has
// no CLI to ask what is declared; the predicate is the whole seam, so a fake one
// tests the rule and nothing else. The live registrations are covered by
// TestLocalHandlerDoesNotSwallowDeclaredChildren (cmd/ze/internal/cmdutil) and
// end to end, against the daemon's own answer, by
// test/ui/cli-verb-daemon-dispatch.ci.
func TestLookupLocalRefusesToSwallowADeclaredChild(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	MustRegisterLocal("show thing", func(_ []string) int { return 7 })
	declared := func(path string) bool {
		return path == "show thing" || path == "show thing brief" || path == "show thing name detail"
	}

	tests := []struct {
		name  string
		words []string
		want  []string // nil with wantNil false means "handler with no args"
		nil_  bool
	}{
		{name: "the registered path itself still serves", words: []string{"show", "thing"}},
		{name: "a value that declares nothing stays an argument", words: []string{"show", "thing", "eth0"}, want: []string{"eth0"}},
		{name: "a declared child is refused", words: []string{"show", "thing", "brief"}, nil_: true},
		{name: "a declared child with a value tail is refused", words: []string{"show", "thing", "brief", "extra"}, nil_: true},
		{name: "a declared grandchild is refused", words: []string{"show", "thing", "name", "detail"}, nil_: true},
		{name: "an undeclared word under a declared sibling stays an argument", words: []string{"show", "thing", "name"}, want: []string{"name"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, args := LookupLocal(tt.words, declared)
			if tt.nil_ {
				if handler != nil {
					t.Fatalf("LookupLocal(%q) served the local handler with args %q: it swallowed a declared command", tt.words, args)
				}
				return
			}
			if handler == nil {
				t.Fatalf("LookupLocal(%q) served no handler: nothing declared below the registered path", tt.words)
			}
			if len(args) != len(tt.want) {
				t.Fatalf("args = %q, want %q", args, tt.want)
			}
			for i := range args {
				if args[i] != tt.want[i] {
					t.Errorf("args[%d] = %q, want %q", i, args[i], tt.want[i])
				}
			}
		})
	}

	t.Run("no declaration source serves nothing", func(t *testing.T) {
		if handler, _ := LookupLocal([]string{"show", "thing"}, nil); handler != nil {
			t.Error("LookupLocal served a handler with no way to check what it shadows: a dispatch guard with no data must fail closed")
		}
	})
}
