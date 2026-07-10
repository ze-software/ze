// VALIDATES: the loadavg collector parses /proc/loadavg and emits the 1/5/15
// minute load averages on the correct gauge/label tuples.
// PREVENTS: mis-parsed or mislabeled load figures reaching the metrics surface.

//go:build linux

package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/procfs"
)

// procFixture writes files into a temp dir and returns a procfs.FS rooted there,
// so a collector reads the fixture instead of the host's /proc.
func procFixture(t *testing.T, files map[string]string) procfs.FS {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := procfs.NewFS(dir)
	if err != nil {
		t.Fatalf("procfs.NewFS: %v", err)
	}
	return fs
}

func TestLoadAvgCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"loadavg": "0.50 0.75 1.00 1/234 5678\n",
	})
	reg := newRecordingRegistry()
	c := newLoadAvgCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	const name = "netdata_system_load_load_average"
	for _, tc := range []struct {
		dim  string
		want float64
	}{
		{"load1", 0.50},
		{"load5", 0.75},
		{"load15", 1.00},
	} {
		if got := reg.gauge(t, name, "system.load", tc.dim, "load"); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.dim, got, tc.want)
		}
	}
}

func TestLoadAvgCollectMissingFile(t *testing.T) {
	fs := procFixture(t, nil) // empty dir: no loadavg file
	c := newLoadAvgCollector(fs)
	c.Init(newRecordingRegistry(), "netdata")
	if err := c.Collect(); err == nil {
		t.Fatal("expected error when /proc/loadavg is absent")
	}
}
