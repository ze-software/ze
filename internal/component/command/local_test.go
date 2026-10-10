package command

import (
	"errors"
	"testing"
)

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
// THE CASES ARE SYNTHETIC BECAUSE THE RULE IS. This package has no CLI to ask
// what is declared; the predicate is the whole seam, so a fake one tests the
// rule and nothing else. The live registrations are covered by
// TestLocalHandlerDoesNotSwallowDeclaredChildren (cmd/ze/internal/cmdutil) and
// end to end, against the daemon's own answer, by
// test/ui/cli-verb-daemon-dispatch.ci.
func TestLookupLocalRefusesToSwallowADeclaredChild(t *testing.T) {
	ResetLocalForTest()
	defer ResetLocalForTest()

	MustRegisterLocal("show thing", func(ValidatedArgs) int { return 7 })
	declared := func(path string) (bool, error) {
		return path == "show thing" || path == "show thing brief" || path == "show thing name detail", nil
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
			handler, args, err := LookupLocal(tt.words, declared)
			if err != nil {
				t.Fatalf("LookupLocal(%q) answered error %v from a declaration source that has none", tt.words, err)
			}
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
		if handler, _, _ := LookupLocal([]string{"show", "thing"}, nil); handler != nil {
			t.Error("LookupLocal served a handler with no way to check what it shadows: a dispatch guard with no data must fail closed")
		}
	})

	// A declaration source that cannot answer (the YANG schema was refused)
	// is a guard with no data: the match is refused, never served as if
	// nothing were declared below it, and the cause reaches the caller.
	t.Run("a declaration source error refuses and is returned", func(t *testing.T) {
		errSchema := errors.New("schema refused")
		failing := func(string) (bool, error) { return false, errSchema }
		handler, args, err := LookupLocal([]string{"show", "thing", "eth0"}, failing)
		if handler != nil {
			t.Fatalf("LookupLocal served the local handler with args %q although the declaration source failed: fail-open", args)
		}
		if !errors.Is(err, errSchema) {
			t.Fatalf("LookupLocal answered error %v, want the declaration source's error", err)
		}
	})
}
