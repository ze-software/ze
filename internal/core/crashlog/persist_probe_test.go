package crashlog

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestProbeWritableSurvivesConcurrentProbers drives the writability probe from
// several goroutines against ONE directory, which is what several ze processes
// sharing a crash directory do.
//
// VALIDATES: every prober answers true, and each cleans up only its own file.
// PREVENTS: a fixed probe name. With one, the first prober to finish removes
// the file the second has just created, the second's os.Remove then fails, and
// it reports a directory it can write to perfectly well as unwritable. The
// same fixed name also made the file appear and disappear under any reader
// walking that tree, which failed one ExaBGP case at random on every run.
//
// A run of this case against the fixed-name version fails on the count below:
// with 8 probers the collisions are near certain, and one false answer is
// enough.
func TestProbeWritableSurvivesConcurrentProbers(t *testing.T) {
	shared := t.TempDir()

	const probers = 8
	answers := make([]bool, probers)
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(probers)
	for index := range probers {
		go func() {
			defer done.Done()
			start.Wait()
			answers[index] = probeWritable(shared)
		}()
	}
	start.Done()
	done.Wait()

	for index, answer := range answers {
		if !answer {
			t.Errorf("prober %d read a writable directory as unwritable", index)
		}
	}

	// Every prober removes what it created, so the directory is empty again.
	// A leftover says a name was reused and one prober deleted another's file.
	entries, err := os.ReadDir(shared)
	if err != nil {
		t.Fatalf("read shared directory: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, filepath.Join(shared, entry.Name()))
		}
		t.Errorf("probe files left behind: %v", names)
	}
}

// TestProbeWritableRefusesADirectoryItCannotWrite pins the answer the probe
// exists to give.
//
// VALIDATES: an unwritable directory answers false.
// PREVENTS: the unique-name change turning the probe into a function that
// always says yes. resolveCrashDir walks a candidate list and takes the first
// that answers true, so a probe that never refuses would pick a directory no
// crash file can be written to, and the crash log would be lost at the moment
// it is needed.
func TestProbeWritableRefusesADirectoryItCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into a mode 0500 directory, so the refusal cannot be observed")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatalf("create read-only directory: %v", err)
	}
	if probeWritable(locked) {
		t.Error("a directory this process cannot write to was reported writable")
	}
}
