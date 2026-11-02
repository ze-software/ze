package command

import "testing"

// TestSurfacePredicateByKind states the target answer of OffersCommand for
// every registration kind on every surface, hidden, streaming and TUI-only
// included, with a reason beside each answer, and
// that an unspecified kind or surface is refused rather than answered.
//
// VALIDATES: the predicate the parity check (cmd/ze/hub
// TestEveryCommandOnEverySurface) compares every surface with.
// PREVENTS: a kind silently answering false, which would let a surface drop a
// whole class of commands with the check still green.
func TestSurfacePredicateByKind(t *testing.T) {
	must, mustNot, streaming, tui := OfferMust, OfferMustNot, OfferStreaming, OfferTUIOnly
	every := map[Surface]Offer{
		SurfaceSSH: must, SurfaceWebAdmin: must, SurfaceWebCompletion: must, SurfaceMCP: must,
		SurfaceREST: must, SurfaceGRPC: must, SurfaceCLI: must, SurfaceShell: must,
	}
	none := map[Surface]Offer{
		SurfaceSSH: mustNot, SurfaceWebAdmin: mustNot, SurfaceWebCompletion: mustNot, SurfaceMCP: mustNot,
		SurfaceREST: mustNot, SurfaceGRPC: mustNot, SurfaceCLI: mustNot, SurfaceShell: mustNot,
	}
	daemonOnly := map[Surface]Offer{
		SurfaceSSH: must, SurfaceWebAdmin: must, SurfaceWebCompletion: must, SurfaceMCP: must,
		SurfaceREST: must, SurfaceGRPC: must, SurfaceCLI: must, SurfaceShell: mustNot,
	}
	// A monitor command streams on the three terminal surfaces; its one-shot
	// surfaces answer by its kind (AC-9 judges what that one answer carries).
	streams := map[Surface]Offer{
		SurfaceSSH: streaming, SurfaceWebAdmin: must, SurfaceWebCompletion: must, SurfaceMCP: must,
		SurfaceREST: must, SurfaceGRPC: must, SurfaceCLI: streaming, SurfaceShell: streaming,
	}
	tuiOnly := map[Surface]Offer{
		SurfaceSSH: tui, SurfaceWebAdmin: mustNot, SurfaceWebCompletion: mustNot, SurfaceMCP: mustNot,
		SurfaceREST: mustNot, SurfaceGRPC: mustNot, SurfaceCLI: tui, SurfaceShell: mustNot,
	}

	cases := []struct {
		name string
		reg  CommandRegistration
		want map[Surface]Offer
	}{
		{"builtin", CommandRegistration{Path: "a b", Kind: KindBuiltin}, every},
		{"local", CommandRegistration{Path: "a b", Kind: KindLocal}, every},
		{"local-data", CommandRegistration{Path: "a b", Kind: KindLocalData}, every},
		{"plugin without YANG", CommandRegistration{Path: "a b", Kind: KindPlugin}, daemonOnly},
		{"hidden plugin", CommandRegistration{Path: "a b", Kind: KindPlugin, Hidden: true}, none},
		{"streaming builtin", CommandRegistration{Path: "a b", Kind: KindBuiltin, Streaming: true}, streams},
		{"hidden streaming", CommandRegistration{Path: "a b", Kind: KindBuiltin, Streaming: true, Hidden: true}, none},
		{"declared TUI-only view", CommandRegistration{Path: "a b", Kind: KindView, TUIOnly: true}, tuiOnly},
	}
	for _, tc := range cases {
		for _, s := range Surfaces() {
			got, err := AnswerSurface(tc.reg, s)
			if err != nil {
				t.Errorf("%s on %s: %v", tc.name, s, err)
				continue
			}
			if got.Offer != tc.want[s] {
				t.Errorf("%s on %s: %s, want %s", tc.name, s, got.Offer, tc.want[s])
			}
			if got.Reason == "" {
				t.Errorf("%s on %s: %s with no reason, want the reason a failure line prints", tc.name, s, got.Offer)
			}
			listed, err := OffersCommand(tc.reg, s)
			if err != nil {
				t.Errorf("%s on %s: OffersCommand: %v", tc.name, s, err)
				continue
			}
			if listed != tc.want[s].Listed() {
				t.Errorf("%s on %s: OffersCommand = %v, want %v", tc.name, s, listed, tc.want[s].Listed())
			}
		}
	}

	// AC-13: a view with no RPC that its registration does not declare TUI-only
	// is refused on every surface, so the parity check fails on it.
	for _, s := range Surfaces() {
		if _, err := AnswerSurface(CommandRegistration{Path: "a b", Kind: KindView}, s); err == nil {
			t.Errorf("an undeclared TUI-only view on %s was answered, want a refusal", s)
		}
	}
	if _, err := OffersCommand(CommandRegistration{Path: "a b"}, SurfaceSSH); err == nil {
		t.Error("an unspecified kind was answered, want a refusal")
	}
	if _, err := OffersCommand(CommandRegistration{Path: "a b", Kind: KindBuiltin}, SurfaceUnspecified); err == nil {
		t.Error("an unspecified surface was answered, want a refusal")
	}
	if len(Surfaces()) != int(SurfaceShell) {
		t.Errorf("Surfaces() lists %d surfaces, want every declared surface (%d)", len(Surfaces()), SurfaceShell)
	}
}

// TestListedOnDaemonSurfaces states the answer a producer feeding every daemon
// surface one list (the plugin command registry) reads: a visible plugin
// command is listed, a hidden one is not, and a registration the predicate
// refuses is refused here too.
//
// VALIDATES: the registry's listing asks AnswerSurface rather than its own
// Hidden check.
// PREVENTS: a hidden command reaching a daemon surface through the registry.
func TestListedOnDaemonSurfaces(t *testing.T) {
	cases := []struct {
		name string
		reg  CommandRegistration
		want bool
	}{
		{"visible plugin", CommandRegistration{Path: "a b", Kind: KindPlugin}, true},
		{"hidden plugin", CommandRegistration{Path: "a b", Kind: KindPlugin, Hidden: true}, false},
		{"streaming builtin", CommandRegistration{Path: "a b", Kind: KindBuiltin, Streaming: true}, true},
	}
	for _, tc := range cases {
		got, err := ListedOnDaemonSurfaces(tc.reg)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: listed = %v, want %v", tc.name, got, tc.want)
		}
	}
	// A declared TUI-only view is listed on SSH and `ze cli` but not on the
	// one-shot daemon surfaces, so one list cannot feed them all.
	if _, err := ListedOnDaemonSurfaces(CommandRegistration{Path: "a b", Kind: KindView, TUIOnly: true}); err == nil {
		t.Error("a TUI-only view was answered for every daemon surface, want a refusal")
	}
	if _, err := ListedOnDaemonSurfaces(CommandRegistration{Path: "a b"}); err == nil {
		t.Error("an unspecified kind was answered, want a refusal")
	}
}
