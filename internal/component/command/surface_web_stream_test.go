// Design: docs/architecture/api/commands.md -- browser terminal streaming.

package command

import "testing"

// TestBrowserTerminalOffersStreamsWithoutChangingAdmin checks the boundary
// between the browser terminal and one-shot admin forms from one registration.
// MUTATION: remove SurfaceWebCompletion from Surface.terminal; completion drops
// the streaming-only command while the terminal can still execute it.
func TestBrowserTerminalOffersStreamsWithoutChangingAdmin(t *testing.T) {
	registration := CommandRegistration{Path: "monitor fixture", Kind: KindBuiltin, Streaming: true, StreamingOnly: true}
	terminal, err := AnswerSurface(registration, SurfaceWebCompletion)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Offer != OfferStreaming {
		t.Fatalf("browser terminal offer = %s, want streaming", terminal.Offer)
	}
	admin, err := AnswerSurface(registration, SurfaceWebAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Offer != OfferMustNot {
		t.Fatalf("one-shot web admin offer = %s, want not offered", admin.Offer)
	}
}
