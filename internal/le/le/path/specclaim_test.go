package lepath

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/sessiondir"
)

// VALIDATES: ParseSpecClaimMarker reads back the holder of every name
// SpecClaimMarker writes, and refuses every other name under tmp/session.
// PREVENTS: the claim enumerator taking another kind of marker, or the
// session directory's own owner file, for a claim whose holder it then judges.
func TestParseSpecClaimMarkerInvertsSpecClaimMarker(t *testing.T) {
	root := t.TempDir()
	sessions := []string{
		"parent-sid",
		"972de0b2-472a-4cf3-9820-6385b6ff38e0",
		"v1.2_session.with-dots_and-several-dashes",
		"id",
	}
	agents := []string{"", "a3893b49781991623", "agent-with-several-dashes"}
	for _, session := range sessions {
		for _, agent := range agents {
			path, err := SpecClaimMarker(root, session, agent)
			if session == "id" && agent == "" {
				// .session-id is the directory owner file, never a claim.
				if err == nil {
					t.Errorf("SpecClaimMarker(id) = %q, want a refusal", path)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			parsedSession, parsedAgent, ok := ParseSpecClaimMarker(filepath.Base(path))
			if !ok || parsedSession != session || parsedAgent != agent {
				t.Errorf("ParseSpecClaimMarker(%q) = %q, %q, %v; want %q, %q, true",
					filepath.Base(path), parsedSession, parsedAgent, ok, session, agent)
			}
		}
	}
	for _, name := range []string{
		sessiondir.IDFile,
		".session-",
		".session-+agent",
		".session-sid+",
		".session-sid+a+b",
		".lsp-loaded-sid",
		".sid-by-pid-44-123",
		"session-sid",
	} {
		if session, agent, ok := ParseSpecClaimMarker(name); ok {
			t.Errorf("ParseSpecClaimMarker(%q) = %q, %q, true; want refused", name, session, agent)
		}
	}
}
