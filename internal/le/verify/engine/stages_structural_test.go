package verifyengine

import (
	"strconv"
	"testing"
)

// VALIDATES: the Staticcheck feature matrix runs as staticcheckParts stages,
// one per piece, consecutive in the population, in both modes.
// PREVENTS: two pieces landing on one CI shard, which puts most of the matrix
// back on a single job's clock. `.github/workflows/verify.yml` deals the stage
// list round robin, so consecutive stages reach different shards and separated
// ones can collide.
func TestTheStaticcheckMatrixRunsOnePiecePerStage(t *testing.T) {
	for _, mode := range []string{Mode, FastMode} {
		var positions []int
		for index, one := range StagesForMode(mode) {
			if one.Identity.Command != "go staticcheck" {
				continue
			}
			positions = append(positions, index)
			want := []string{"check", "part", strconv.Itoa(len(positions)), "of", strconv.Itoa(staticcheckParts)}
			for argument := range want {
				if argument >= len(one.Identity.Args) || one.Identity.Args[argument] != want[argument] {
					t.Errorf("mode %q stage %q carries args %q, want %q",
						mode, one.Identity.Name, one.Identity.Args, want)
					break
				}
			}
		}
		if len(positions) != staticcheckParts {
			t.Fatalf("mode %q runs %d matrix pieces, want %d", mode, len(positions), staticcheckParts)
		}
		for piece := 1; piece < len(positions); piece++ {
			if positions[piece] != positions[piece-1]+1 {
				t.Errorf("mode %q separates matrix piece %d from its predecessor: positions %v",
					mode, piece+1, positions)
			}
		}
	}
}

// TestPatchStampsRunsInBothModes proves `go patch-stamps check` is a stage of
// the full and the changed population (AC-26, AC-27): a stale review stamp
// refuses ./le verify whichever mode certifies the tree.
func TestPatchStampsRunsInBothModes(t *testing.T) {
	for _, mode := range []string{Mode, FastMode} {
		found := false
		for _, one := range StagesForMode(mode) {
			if one.Identity.Command == "go patch-stamps" && len(one.Identity.Args) == 1 && one.Identity.Args[0] == "check" {
				found = true
			}
		}
		if !found {
			t.Errorf("mode %q does not run go patch-stamps check", mode)
		}
	}
}
