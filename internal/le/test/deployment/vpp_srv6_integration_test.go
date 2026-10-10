//go:build integration

// Design: docs/architecture/testing/interop.md -- real VPP service-route forwarding.
// Related: vppevidence.go -- shared image, build, socket and daemon machinery.
// Related: vpp_srv6_probe_integration_linux_test.go -- independent packet oracle.
package testdeployment

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestVPPSRv6ServiceRoute drives the real daemon with BGP UPDATEs and observes
// VPP's state and emitted Ethernet frames, rather than a model's API replies.
// MUTATION: omit SrPolicyAdd in (*govppSRv6Backend).acquirePolicy; the first installed-state assertion
// fails because steering cannot bind to a nonexistent policy. Rebuild the daemon
// through this test for each discrimination run; go test MUST use -count=1.
func TestVPPSRv6ServiceRoute(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("real VPP proof requires Docker: ", err)
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	v := newVPP(root)
	if err := v.prepareDaemon(); err != nil {
		t.Fatal(err)
	}
	work, err := scratchDir(root, "vpp-srv6-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("retained evidence directory:", work)
	probe := filepath.Join(work, "srv6-probe")
	if err := v.runBuild([]string{"test", "-c", "-tags", "integration", "-o", probe,
		"./internal/le/test/deployment"}, errors.New("building the SRv6 packet probe failed")); err != nil {
		t.Fatal(err)
	}
	if err := v.writeEvidenceScratch(work); err != nil {
		t.Fatal(err)
	}
	container := vppEvidenceContainerName() + "-srv6"
	if err := v.startContainer(container, work); err != nil {
		t.Fatal(err)
	}
	defer removeContainer(container)
	if err := v.startVPP(container, work); err != nil {
		t.Fatal(err)
	}
	version, err := v.query(container, "show version")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("VPP image:", v.Image, "version:", version)
	port, err := v.freePort()
	if err != nil {
		t.Fatal(err)
	}
	if err := v.writeConfig(work, "srv6", vppSRv6Config); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	// The probe owns its BGP listener until exit. startWatched MUST be paired
	// with stopVPPProcess, which also drains its collector after stopping it.
	cmd := exec.CommandContext(ctx, "docker", dockerExec, container,
		vppMount+"/srv6-probe", "-test.run", "^TestVPPSRv6(ServiceRouteProbe|StateOracle|PacketOracle)$", "-test.v",
		"-test.timeout", "150s", "-vpp-srv6-port", strconv.Itoa(port))
	seen := newCollector("srv6 peer listening", "srv6 ze restart requested", "srv6 forwarding proof passed")
	peer, err := startWatched(cmd, "srv6> ", seen, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer stopVPPProcess(peer, seen)
	if !await(seen, "srv6 peer listening", peer, 30*time.Second) {
		t.Fatal("SRv6 probe did not become ready: ", seen.tailLines())
	}
	daemon, daemonSeen, err := v.startEvidenceDaemon(container, "srv6", port)
	if err != nil {
		t.Fatal(err)
	}
	defer stopVPPProcess(daemon, daemonSeen)
	if !await(seen, "srv6 ze restart requested", peer, 60*time.Second) {
		t.Fatal("confirmed-ownership restart barrier: ", seen.tailLines(), " daemon: ", daemonSeen.tailLines())
	}
	vppSRv6StopDaemon(t, v, container, daemon, daemonSeen)
	daemon, daemonSeen, err = v.startEvidenceDaemon(container, "srv6", port)
	if err != nil {
		t.Fatal(err)
	}
	defer stopVPPProcess(daemon, daemonSeen)
	select {
	case <-peer.done:
		seen.wait()
		if !cmd.ProcessState.Success() {
			t.Fatal("SRv6 probe failed: ", cmd.ProcessState, " probe: ", seen.tailLines(), " daemon: ", daemonSeen.tailLines())
		}
	case <-ctx.Done():
		t.Fatal("SRv6 proof deadline: ", seen.tailLines(), " daemon: ", daemonSeen.tailLines())
	}
	if !seen.saw("srv6 forwarding proof passed") {
		t.Fatal("probe exited without completing packet/state assertions: ", seen.tailLines())
	}
	vppSRv6StopDaemon(t, v, container, daemon, daemonSeen)
	vppSRv6Managed(t, v, container, work)
}

// vppSRv6StopDaemon signals Ze itself, not merely its attached docker client.
// The caller MUST wait for the daemon before reusing its configuration store.
func vppSRv6StopDaemon(t *testing.T, v *VPP, container string, daemon *running, seen *collector) {
	t.Helper()
	if output, ok := v.containerText(container, "pkill", "-TERM", "-f", "/src/"+filepath.ToSlash(daemonRel(v.Goarch))); !ok {
		t.Fatal("stop Ze process: ", output)
	}
	select {
	case <-daemon.done:
		stopVPPProcess(daemon, seen)
	case <-time.After(20 * time.Second):
		t.Fatal("Ze did not stop before restart: ", seen.tailLines())
	}
}

const vppSRv6Config = `connected {}
interface { backend netlink; }
plugin {
    external srv6-connected {
        run "exec /run/vpp/srv6-probe -test.run ^TestVPPSRv6ConnectedProbe$ -test.v -test.timeout 150s -vpp-srv6-connected > /run/vpp/srv6-connected.log 2>&1";
        encoder json;
    }
}
bgp {
    peer peer1 {
        connection {
            remote { ip 127.0.0.1; }
            local { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 1; remote 1; }
            router-id 1.2.3.4;
            family { ipv4/unicast { prefix { maximum 10000; } } }
            capability {
                graceful-restart disable;
                nexthop ipv4/unicast { nhafi ipv6; }
            }
        }
        behavior { group-updates disable; }
    }
}
` + vppExternalConfig + `fib {
    vpp { enabled true; }
}
`
