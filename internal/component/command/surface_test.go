package command

import "testing"

// TestSurfacePredicateByKind states the target answer of OffersCommand for
// every registration kind on every surface, hidden and TUI-only included, and
// that an unspecified kind or surface is refused rather than answered.
//
// VALIDATES: the predicate the parity check (cmd/ze/hub
// TestEveryCommandOnEverySurface) compares every surface with.
// PREVENTS: a kind silently answering false, which would let a surface drop a
// whole class of commands with the check still green.
func TestSurfacePredicateByKind(t *testing.T) {
	every := map[Surface]bool{
		SurfaceSSH: true, SurfaceWebAdmin: true, SurfaceWebCompletion: true, SurfaceMCP: true,
		SurfaceREST: true, SurfaceGRPC: true, SurfaceCLI: true, SurfaceShell: true,
	}
	none := map[Surface]bool{}
	daemonOnly := map[Surface]bool{
		SurfaceSSH: true, SurfaceWebAdmin: true, SurfaceWebCompletion: true, SurfaceMCP: true,
		SurfaceREST: true, SurfaceGRPC: true, SurfaceCLI: true,
	}
	tuiOnly := map[Surface]bool{SurfaceSSH: true, SurfaceCLI: true}

	cases := []struct {
		name string
		reg  CommandRegistration
		want map[Surface]bool
	}{
		{"builtin", CommandRegistration{Path: "a b", Kind: KindBuiltin}, every},
		{"local", CommandRegistration{Path: "a b", Kind: KindLocal}, every},
		{"local-data", CommandRegistration{Path: "a b", Kind: KindLocalData}, every},
		{"plugin without YANG", CommandRegistration{Path: "a b", Kind: KindPlugin}, daemonOnly},
		{"hidden plugin", CommandRegistration{Path: "a b", Kind: KindPlugin, Hidden: true}, none},
		{"TUI-only view", CommandRegistration{Path: "a b", Kind: KindView}, tuiOnly},
	}
	for _, tc := range cases {
		for _, s := range Surfaces() {
			got, err := OffersCommand(tc.reg, s)
			if err != nil {
				t.Errorf("%s on %s: %v", tc.name, s, err)
				continue
			}
			if got != tc.want[s] {
				t.Errorf("%s on %s: offered = %v, want %v", tc.name, s, got, tc.want[s])
			}
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
