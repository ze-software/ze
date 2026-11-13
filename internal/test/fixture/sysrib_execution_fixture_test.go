// Design: docs/guide/route-policy.md -- the execution proof identifies the real RIB subprocess.

package fixture

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSysribExecutionParentStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
		want   int
		refuse bool
	}{
		{"kernel whitespace", "Name:\tze\nPid:\t301\nPPid:\t280\nNSpid:\t301\n", 280, false},
		{"space separator", "PPid:   42\n", 42, false},
		{"root process", "PPid:\t0\n", 0, false},
		{"missing", "Pid:\t301\nNSpid:\t301\n", 0, true},
		{"empty", "PPid:\t\n", 0, true},
		{"negative", "PPid:\t-1\n", 0, true},
		{"not numeric", "PPid:\tnone\n", 0, true},
		{"extra fields", "PPid:\t280 281\n", 0, true},
		{"duplicate", "PPid:\t280\nPPid:\t281\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sysribExecutionParentStatus([]byte(tc.status))
			if tc.refuse {
				if err == nil {
					t.Fatalf("accepted invalid parent status %q as %d", tc.status, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("parent=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestSysribExecutionParentCensus(t *testing.T) {
	root := t.TempDir()
	for name, status := range map[string]string{
		"280": "Name:\tze\nPPid:\t12\n",
		"301": "Name:\tsh\nPPid:\t280\n",
		"313": "Name:\tze\nPPid:\t301\n",
		"net": "not a process",
	} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A process that exits after ReadDir may leave no readable status.
	if err := os.Mkdir(filepath.Join(root, "999"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := sysribExecutionParentCensus(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]int{280: 12, 301: 280, 313: 301}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PID/PPID census=%v, want %v", got, want)
	}
	if err := os.WriteFile(filepath.Join(root, "301", "status"), []byte("PPid:\tinvalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sysribExecutionParentCensus(context.Background(), root); err == nil {
		t.Fatal("malformed kernel status was accepted as an absent process")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := sysribExecutionParentCensus(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled census returned %v", err)
	}
}

func TestSysribExecutionRIBArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"configured invocation", []string{"ze", "plugin", "rib"}, true},
		{"supported trailing flags", []string{"/tmp/zebin/ze", "plugin", "rib", "--log-level", "info"}, true},
		{"flag validity belongs to CLI", []string{"ze", "plugin", "rib", "--rpc"}, true},
		{"shell command is not the child", []string{"sh", "-c", "ze plugin rib"}, false},
		{"observer is not the owner", []string{"le", "test", "fixture", "plugin/sysrib-execution-parity", "forked", "kernel"}, false},
		{"another plugin", []string{"ze", "plugin", "bgp-rib"}, false},
		{"suffix inside another command", []string{"ze", "other", "plugin", "rib"}, false},
		{"missing plugin", []string{"ze", "plugin"}, false},
		{"empty argv", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sysribExecutionRIBArgs(tc.args); got != tc.want {
				t.Fatalf("RIB invocation %q = %t, want %t", tc.args, got, tc.want)
			}
		})
	}
}
