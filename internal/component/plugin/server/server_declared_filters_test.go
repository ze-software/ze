package server

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDeclaredFiltersWaitsForStageOne distinguishes an allocated process from
// its completed declaration by querying the real process-manager lookup before
// registration and after converting empty and nonempty Stage 1 messages.
func TestDeclaredFiltersWaitsForStageOne(t *testing.T) {
	const name = "test-filter-registration-lifecycle"

	t.Run("before-registration", func(t *testing.T) {
		s := sessionReadyTestServer()
		proc := runningProcess(s, name, name)
		if proc.Registration() == nil {
			t.Fatal("NewProcess must supply the initial registration this regression exercises")
		}
		if proc.Registration().Done {
			t.Fatal("new process already completed Stage 1")
		}
		declared, known := s.declaredFilters(name)
		if known {
			t.Fatal("process before Stage 1 answered an authoritative filter declaration")
		}
		if declared != nil {
			t.Fatalf("process before Stage 1 returned declarations: %v", declared)
		}
	})

	t.Run("completed-empty-registration", func(t *testing.T) {
		s := sessionReadyTestServer()
		proc := runningProcess(s, name, name)
		proc.SetRegistration(registrationFromRPC(&rpc.DeclareRegistrationInput{}))
		declared, known := s.declaredFilters(name)
		if !known {
			t.Fatal("completed empty Stage 1 declaration was deferred")
		}
		if len(declared) != 0 {
			t.Fatalf("empty Stage 1 declaration returned filters: %v", declared)
		}
	})

	t.Run("completed-filter-registration", func(t *testing.T) {
		s := sessionReadyTestServer()
		proc := runningProcess(s, name, name)
		proc.SetRegistration(registrationFromRPC(&rpc.DeclareRegistrationInput{
			Filters: []rpc.FilterDecl{{Name: "egress", Direction: rpc.FilterExport}},
		}))
		declared, known := s.declaredFilters(name)
		if !known {
			t.Fatal("completed Stage 1 filter declaration was deferred")
		}
		if len(declared) != 1 {
			t.Fatalf("completed Stage 1 returned %d filters, want 1", len(declared))
		}
		if declared[0].Name != "egress" {
			t.Fatalf("declared filter name = %q, want egress", declared[0].Name)
		}
		if declared[0].Direction != rpc.FilterExport {
			t.Fatalf("declared filter direction = %v, want export", declared[0].Direction)
		}
	})
}
