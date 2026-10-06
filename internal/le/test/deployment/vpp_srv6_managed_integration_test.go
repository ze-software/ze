//go:build integration

// Design: docs/architecture/testing/interop.md -- managed VPP restart evidence.
// Related: vpp_srv6_integration_test.go -- the shared native container and build.
// Related: vpp_srv6_managed_probe_integration_linux_test.go -- existing plugin ingress.
package testdeployment

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// vppSRv6Managed uses the same image, binary and sockets, but Ze now owns VPP.
// This deliberately does not claim that external-mode connector recovery works.
func vppSRv6Managed(t *testing.T, v *VPP, container, work string) {
	t.Helper()
	pattern := "^vpp -c /run/vpp/startup.conf$"
	if output, ok := v.containerText(container, "pkill", "-TERM", "-f", pattern); !ok {
		t.Fatal("stop external VPP before managed phase: ", output)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, alive := v.containerText(container, "pgrep", "-f", pattern); !alive {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatal("external VPP did not exit")
		}
		// sleep(poll): pgrep observes the externally owned process exit.
		time.Sleep(100 * time.Millisecond)
	}
	if err := v.writeConfig(work, "srv6-managed", vppSRv6ManagedConfig); err != nil {
		t.Fatal(err)
	}
	if err := v.stageConfig(container, "srv6-managed"); err != nil {
		t.Fatal(err)
	}
	// External plugins have no stdout relay. Their complete Go test transcript
	// is retained in the shared scratch mount, including the final PASS line.
	transcript, err := os.Create(filepath.Join(work, "srv6-managed-daemon.log"))
	if err != nil {
		t.Fatal("create managed daemon transcript: ", err)
	}
	defer func() {
		if err := transcript.Close(); err != nil {
			t.Error("close managed daemon transcript: ", err)
		}
	}()
	seen := newCollector("VPP ready, reinitializing backend")
	cmd := exec.CommandContext(context.Background(), "docker", v.evidenceDaemonArgs(container, "srv6-managed", 0)...)
	daemon, err := startWatched(cmd, "ze-managed> ", seen, io.MultiWriter(v.Progress, transcript))
	if err != nil {
		t.Fatal(err)
	}
	defer stopVPPProcess(daemon, seen)
	defer func() {
		// Preserve the real child's log and generated input before container
		// cleanup, including failures before its API socket becomes ready.
		for _, artifact := range []struct{ source, name string }{
			{"/var/log/vpp/vpp.log", "srv6-managed-vpp.log"},
			{"/etc/vpp/startup.conf", "srv6-managed-startup.conf"},
		} {
			if output, ok := v.dockerText("cp", container+":"+artifact.source, filepath.Join(work, artifact.name)); !ok {
				t.Logf("retain managed VPP %s: %s", artifact.source, output)
			}
		}
	}()
	deadline = time.Now().Add(150 * time.Second)
	logPath := filepath.Join(work, "srv6-managed.log")
	for {
		data, err := os.ReadFile(logPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		transcript := string(data)
		if strings.Contains(transcript, "--- FAIL:") || strings.Contains(transcript, "\nFAIL\n") {
			t.Fatal("managed probe failed:\n", transcript, "\ndaemon: ", seen.tailLines())
		}
		if strings.Contains(transcript, "srv6 managed lifecycle passed") && strings.Contains(transcript, "\nPASS\n") {
			t.Log("managed probe transcript:\n", transcript)
			break
		}
		select {
		case <-daemon.done:
			t.Fatal("managed daemon exited:\n", transcript, "\ndaemon: ", seen.tailLines())
		default:
		}
		if !time.Now().Before(deadline) {
			t.Fatal("managed proof deadline; requires configured hugepages, /usr/bin/vpp and writable /etc/vpp:\n", transcript, "\ndaemon: ", seen.tailLines())
		}
		// sleep(poll): await the actual external test process's final verdict.
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("managed VPP restart, tenant tables, replay and packet proof passed")
	vppSRv6StopDaemon(t, v, container, daemon, seen)
}

const vppSRv6ManagedConfig = `plugin {
    external srv6-evidence {
        run "exec /run/vpp/srv6-probe -test.run ^TestVPPSRv6ManagedProbe$ -test.v -test.timeout 140s -vpp-srv6-plugin > /run/vpp/srv6-managed.log 2>&1";
        encoder json;
    }
}
vpp {
    enabled true;
    external false;
    api-socket /run/vpp/api.sock;
    lcp { enabled false; }
    cpu { poll-sleep 1ms; }
    memory { main-heap 64M; buffers 1024; hugepage-size 2M; }
    stats { segment-size 16M; socket-path /run/vpp/stats.sock; }
}
fib { vpp { enabled true; } }
`
