// Related: derived.go -- the registry these tests mutate
//
// VALIDATES: Register refuses a registration that would be silently inert, and
// one whose path reaches outside the checkout the hooks join it to.
// PREVENTS: a second artifact answering for an existing path, and an
// invalidation that removes a file outside the tree the session is working in.
//
// These tests run INSIDE the package because each one has to register, and the
// registry is process-global: an external test would leave its fixture in place
// for every later test in the binary. Saving and restoring the slice here keeps
// each case isolated, and there is no production API for removing a
// registration, which is correct -- nothing but a test ever wants one.
package derived

import (
	"strings"
	"testing"
)

// isolateRegistry empties the registry for one test and puts back whatever the
// generators' init() left in it.
func isolateRegistry(t *testing.T) {
	t.Helper()
	registry.mutex.Lock()
	saved := registry.artifacts
	registry.artifacts = nil
	registry.mutex.Unlock()

	t.Cleanup(func() {
		registry.mutex.Lock()
		registry.artifacts = saved
		registry.mutex.Unlock()
	})
}

// refusalOf runs one Register and answers the panic message it produced, or ""
// when it was accepted.
func refusalOf(artifact Artifact) (message string) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		text, ok := recovered.(string)
		if !ok {
			message = "a refusal that is not a string"
			return
		}
		message = text
	}()
	Register(artifact)
	return ""
}

// validArtifact is a registration with both halves present, so each case below
// differs from it only in the thing it is about.
func validArtifact(path string) Artifact {
	return Artifact{
		Path:    path,
		Feeds:   func(_, _ string) bool { return false },
		Rebuild: func(_ string) error { return nil },
	}
}

// TestRegisterRefusesADuplicatePath pins that one output path is claimed once.
//
// Two artifacts under one path each answer for the other's content: the first
// rebuild writes what the second predicate says is stale, and neither generator
// is wrong on its own. The refusal is a panic because the caller is an init().
func TestRegisterRefusesADuplicatePath(t *testing.T) {
	isolateRegistry(t)
	const path = "ai/duplicate-path-fixture.md"

	if message := refusalOf(validArtifact(path)); message != "" {
		t.Fatalf("the first registration was refused: %s", message)
	}
	message := refusalOf(validArtifact(path))
	if message == "" {
		t.Fatal("a second registration of one path was accepted")
	}
	if !strings.Contains(message, path) {
		t.Errorf("the refusal does not name the path it refused: %s", message)
	}
}

// TestRegisterRefusesAnUnusablePath covers the registrations that are inert or
// dangerous rather than merely wrong.
//
// The hooks join Path to a checkout root they were handed, then REMOVE the
// result. An absolute path ignores that root and a `..` segment climbs out of
// it, so either one turns an invalidation into a deletion somewhere the session
// never asked about. A half-registered artifact is the quieter failure: nothing
// invalidates it and nothing rebuilds it, with no line red to say so.
func TestRegisterRefusesAnUnusablePath(t *testing.T) {
	for _, row := range []struct {
		name     string
		artifact Artifact
		names    string
	}{
		{"empty", validArtifact(""), "no output path"},
		{"absolute", validArtifact("/etc/ze-artifact.md"), "/etc/ze-artifact.md"},
		{"climbing", validArtifact("ai/../../outside.md"), "ai/../../outside.md"},
		{"no predicate", Artifact{Path: "ai/x.md", Rebuild: func(_ string) error { return nil }}, "ai/x.md"},
		{"no rebuild", Artifact{Path: "ai/x.md", Feeds: func(_, _ string) bool { return false }}, "ai/x.md"},
	} {
		t.Run(row.name, func(t *testing.T) {
			isolateRegistry(t)
			message := refusalOf(row.artifact)
			if message == "" {
				t.Fatalf("Register accepted %#v", row.artifact.Path)
			}
			if !strings.Contains(message, row.names) {
				t.Errorf("the refusal says %q, which does not name %q", message, row.names)
			}
		})
	}

	// The discrimination: a registration with nothing wrong with it is
	// accepted, so the refusals above are about what each row changes.
	isolateRegistry(t)
	if message := refusalOf(validArtifact("ai/well-formed.md")); message != "" {
		t.Errorf("a well-formed registration was refused: %s", message)
	}
}
