// Design: (none -- test utility, no architecture doc)

package golden

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"testing"
)

// AssertCoversNames fails when a capture and the live set it must cover
// disagree, naming every name on either side of the difference.
//
// live is the authority: the routes a server serves, read from the server
// rather than typed into the test. captured is what the capture reaches. A
// route the capture cannot exercise fails here rather than being left out. A
// capture that covers part of the surface reports green and proves nothing
// about the rest.
//
// kind names one element ("route"), and specVar names the Go variable a
// failure tells the reader to edit.
func AssertCoversNames(t *testing.T, kind, specVar string, live, captured []string) {
	t.Helper()

	for _, finding := range nameCoverage(kind, specVar, live, captured) {
		t.Error(finding)
	}
}

// nameCoverage returns one finding per disagreement, in a fixed order, so a run
// reports the same findings in the same sequence. It holds no *testing.T, which
// is what lets this package's own tests drive it.
func nameCoverage(kind, specVar string, live, captured []string) []error {
	liveSet := make(map[string]bool, len(live))
	for _, name := range live {
		liveSet[name] = true
	}

	capturedSet := make(map[string]bool, len(captured))
	for _, name := range captured {
		capturedSet[name] = true
	}

	var findings []error

	for _, name := range sortedKeys(liveSet) {
		if !capturedSet[name] {
			findings = append(findings, fmt.Errorf(
				"%s %q is live but no golden case reaches it; add one to %s", kind, name, specVar))
		}
	}

	for _, name := range sortedKeys(capturedSet) {
		if !liveSet[name] {
			findings = append(findings, fmt.Errorf(
				"%s names %s %q, which is not live", specVar, kind, name))
		}
	}

	return findings
}

// AssertCoversDir fails when the fixture directory on disk and the fixtures a
// capture writes disagree, naming every file on either side of the difference.
//
// dir is the fixture root and captured holds the paths the capture compares,
// each one as the capture builds it. A fixture whose case is deleted stays on
// disk. The next reader counts it as coverage that no case produces. A fixture
// a case expects and disk does not hold is named here with the rest. The first
// case that reaches it would otherwise Fatal alone.
func AssertCoversDir(t *testing.T, dir, specVar string, captured []string) {
	t.Helper()

	onDisk, err := filesUnder(dir)
	if err != nil {
		t.Fatalf("read the fixture directory %s: %v", dir, err)
	}

	for _, finding := range dirCoverage(specVar, onDisk, captured) {
		t.Error(finding)
	}
}

// filesUnder returns every file below dir, each one as a path starting at dir.
func filesUnder(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			files = append(files, p)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// dirCoverage returns one finding per disagreement, in a fixed order. It holds
// no *testing.T, which is what lets this package's own tests drive it.
func dirCoverage(specVar string, onDisk, captured []string) []error {
	diskSet := make(map[string]bool, len(onDisk))
	for _, p := range onDisk {
		diskSet[p] = true
	}

	capturedSet := make(map[string]bool, len(captured))
	for _, p := range captured {
		capturedSet[p] = true
	}

	var findings []error

	for _, p := range sortedKeys(diskSet) {
		if !capturedSet[p] {
			findings = append(findings, fmt.Errorf(
				"fixture %s is on disk and no case in %s writes it; delete it or restore its case", p, specVar))
		}
	}

	for _, p := range sortedKeys(capturedSet) {
		if !diskSet[p] {
			findings = append(findings, fmt.Errorf(
				"%s captures %s, which is not on disk; capture it with -update-golden", specVar, p))
		}
	}

	return findings
}

// AssertUniqueNames fails when a name repeats, naming it. Two cases that write
// one fixture leave the second overwriting the first, and the capture reports
// green over bytes nobody compared.
func AssertUniqueNames(t *testing.T, kind, specVar string, names []string) {
	t.Helper()

	for _, finding := range repeatedNames(kind, specVar, names) {
		t.Error(finding)
	}
}

// repeatedNames returns one finding per repeated name, in a fixed order.
func repeatedNames(kind, specVar string, names []string) []error {
	count := make(map[string]int, len(names))
	for _, name := range names {
		count[name]++
	}

	repeated := make([]string, 0, len(names))

	for name, n := range count {
		if n > 1 {
			repeated = append(repeated, name)
		}
	}

	sort.Strings(repeated)

	findings := make([]error, 0, len(repeated))
	for _, name := range repeated {
		findings = append(findings, fmt.Errorf(
			"%s gives %d cases the %s name %q; one fixture cannot hold two captures",
			specVar, count[name], kind, name))
	}

	return findings
}
