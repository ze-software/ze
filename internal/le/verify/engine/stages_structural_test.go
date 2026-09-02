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
	for _, mode := range []string{Mode, ChangedMode} {
		var positions []int
		for index, one := range StagesForMode(mode) {
			if one.Identity.Command != "staticcheck-feature-matrix" {
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

// VALIDATES: the structural set is non-empty in both modes, names only stages
// that run, and never marks a stage structural in the cheaper mode alone.
// PREVENTS: a population that refuses no commit, and a red that blocks or does
// not depending on which mode recorded it.
//
// What it does NOT prevent, and should not be read as preventing: a stage
// renamed out of the structural set. That defect is now impossible rather than
// caught. The flag lives on the stage, so a rename moves the name and its
// membership together. It was possible while the commit gate kept its own list
// of the same eight names -- and ai/rules/precommit-verify.md claimed a test
// held the two in agreement when no such test existed. Deleting the second
// list is what fixed it; these assertions guard the properties that survive.
func TestStructuralStagesAreMembersOfThePopulation(t *testing.T) {
	for _, mode := range []string{Mode, ChangedMode} {
		population := StagesForMode(mode)
		if len(population) == 0 {
			t.Fatalf("mode %q declares no stage", mode)
		}
		named := make(map[string]bool, len(population))
		for _, one := range population {
			named[one.Identity.Name] = true
		}

		structural := Structural(mode)
		if len(structural) == 0 {
			t.Errorf("mode %q marks no stage structural, so a broken tree would only take a debt row", mode)
		}
		for name := range structural {
			if !named[name] {
				t.Errorf("mode %q calls %q structural, but no stage of that name runs", mode, name)
			}
		}
	}
}

// TestStructuralIsASubsetOfFull keeps the cheaper mode honest: a stage that is
// structural in one mode and merely advisory in the other would make the same
// red block a commit or not depending on which run recorded it.
func TestStructuralIsASubsetOfFull(t *testing.T) {
	full := Structural(Mode)
	for name := range Structural(ChangedMode) {
		if !full[name] {
			t.Errorf("%q is structural in the changed mode and not in the full one", name)
		}
	}
}
