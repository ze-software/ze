// Design: docs/architecture/forked-route-install.md -- producer disconnect ownership.
// Related: sysrib_execution_fixture.go -- authenticated daemon and child placement.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribProducerOwner        = "route-producer"
	sysribProducerFixture      = "plugin/sysrib-forked-route-producer"
	sysribProducerIdentityFile = "sysrib-route-producer.identity"
	sysribProducerLink         = "zesrprod"
	sysribProducerConnected    = "198.19.20.0/24"
	sysribProducerStatic       = "198.19.21.0/24"
	sysribProducerStaticHidden = "198.19.22.0/24"
	sysribProducerStaticZero   = "198.19.23.0/24"
	sysribProducerSDK          = "198.19.24.0/24"
	sysribProducerSDKHidden    = "198.19.25.0/24"
	sysribProducerOther        = "198.19.26.0/24"
	sysribProducerWithheld     = "198.19.27.0/24"
	sysribProducerForeign      = "198.19.28.0/24"
)

// sysribProducerIdentity is only an observation rendezvous. The observer also
// checks the authenticated daemon, configured Running slot, procfs ancestry,
// executable and exact argv before it may signal this PID.
type sysribProducerIdentity struct {
	PID    int `json:"pid"`
	Daemon int `json:"daemon"`
}

func sysribProducerSDKDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("route producer fixture takes no arguments")
	}
	p, err := newObserver(sysribProducerOwner)
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // Run retains transport failures.
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	started := make(chan struct{})
	finished := make(chan error, 1)
	p.OnAllPluginsReady(func() error {
		close(started)
		// One fixture lifecycle worker remains alive after its input is accepted.
		// The other observer, rather than this producer, owns daemon shutdown.
		go func() {
			err := sysribProducerPublish(workerCtx, p)
			if err != nil {
				ReportFailure(err)
				finished <- errors.Join(err, p.Close())
				return
			}
			<-workerCtx.Done()
			finished <- nil
		}()
		return nil
	})
	runErr := p.Run(ctx, &sdk.Registration{})
	cancel()
	select {
	case <-started:
		return errors.Join(runErr, <-finished)
	default:
		return runErr
	}
}

func sysribProducerPublish(ctx context.Context, p *sdk.Plugin) error {
	if err := sysribInstall(ctx, p,
		rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: sysribProducerSDK, RouteType: 6},
		rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: sysribProducerSDKHidden, RouteType: 6, HasTag: true, Tag: 4242},
	); err != nil {
		return err
	}
	data, err := requireDone02(ctx, p, "plugin session ping")
	if err != nil {
		return err
	}
	var session struct {
		PID int `json:"pid"`
	}
	if err := decode02(data, &session); err != nil {
		return err
	}
	identity, err := json.Marshal(sysribProducerIdentity{PID: os.Getpid(), Daemon: session.PID})
	if err != nil {
		return err
	}
	if err := os.WriteFile(sysribProducerIdentityFile+".pending", identity, 0o600); err != nil {
		return err
	}
	return os.Rename(sysribProducerIdentityFile+".pending", sysribProducerIdentityFile)
}

func sysribProducerDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("forked producer kernel fixture takes no arguments")
	}
	return Observe(ctx, "sysrib-producer-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, p, "forked"); err != nil {
			return err
		}
		if err := sysribExecutionPluginPlacement(ctx, p, "static-owner", "static", "forked"); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribProducerLink, "198.19.20.254/24"); err != nil {
			return err
		}
		if _, err := importCommandOutput(ctx, "ip", "route", "add", "blackhole", sysribProducerForeign, "proto", "42"); err != nil {
			return err
		}
		// The paired carrier joins daemon shutdown before its dedicated
		// cleanup driver removes this device and the foreign route. Failed runs
		// retain their namespace owner's cleanup rather than deleting live FIB targets.
		if err := waitRIBWinner(ctx, p, sysribProducerConnected, "connected"); err != nil {
			return err
		}
		if err := sysribInstall(ctx, p,
			rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribProducerOther, RouteType: 6, Instance: 601},
			rpc.RouteInstallEntry{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribProducerWithheld, RouteType: 6, Instance: 601, IsBGP: true, IsEBGP: true},
		); err != nil {
			return err
		}
		daemon, err := sysribProducerDaemon(ctx, p)
		if err != nil {
			return err
		}
		sdkPID, err := sysribProducerPID(ctx, p, daemon, 0)
		if err != nil {
			return err
		}
		if err := sysribProducerStaticPresent(ctx, p); err != nil {
			return err
		}
		if err := sysribProducerSDKPresent(ctx, p); err != nil {
			return err
		}
		if err := sysribProducerSurvivors(ctx, p); err != nil {
			return err
		}
		children, detail, err := sysribExecutionPluginChildren(ctx, daemon, "static")
		if err != nil {
			return err
		}
		if len(children) != 1 {
			return fmt.Errorf("static producer children %v; %s", children, detail)
		}
		if err := sysribProducerKill(ctx, children[0]); err != nil {
			return err
		}
		if err := sysribProducerAbsent(ctx, p, "static", sysribProducerStatic, sysribProducerStaticHidden, sysribProducerStaticZero); err != nil {
			return err
		}
		if err := sysribProducerSDKPresent(ctx, p); err != nil {
			return err
		}
		if err := sysribProducerSurvivors(ctx, p); err != nil {
			return err
		}
		if _, err := requireDone02(ctx, p, "request plugin restart static-owner"); err != nil {
			return err
		}
		if err := sysribExecutionPluginPlacement(ctx, p, "static-owner", "static", "forked"); err != nil {
			return err
		}
		if err := sysribProducerStaticPresent(ctx, p); err != nil {
			return err
		}
		if err := sysribProducerKill(ctx, sdkPID); err != nil {
			return err
		}
		if err := sysribProducerAbsent(ctx, p, "ospf", sysribProducerSDK, sysribProducerSDKHidden); err != nil {
			return err
		}
		if err := sysribProducerStaticPresent(ctx, p); err != nil {
			return err
		}
		if err := sysribProducerSurvivors(ctx, p); err != nil {
			return err
		}
		if _, err := requireDone02(ctx, p, "request plugin restart "+sysribProducerOwner); err != nil {
			return err
		}
		if _, err := sysribProducerPID(ctx, p, daemon, sdkPID); err != nil {
			return err
		}
		if err := sysribProducerSDKPresent(ctx, p); err != nil {
			return err
		}
		if err := sysribProducerSurvivors(ctx, p); err != nil {
			return err
		}
		if err := sysribHealthReady(ctx, p); err != nil {
			return err
		}
		return importSay("real forked static and SDK producer disconnects withdraw only their admitted and hidden paths, preserve OS/foreign/withheld/other-owner controls and recover through fresh producers")
	})
}

func sysribProducerDaemon(ctx context.Context, p *sdk.Plugin) (int, error) {
	data, err := requireDone02(ctx, p, "plugin session ping")
	if err != nil {
		return 0, err
	}
	var session struct {
		PID int `json:"pid"`
	}
	if err := decode02(data, &session); err != nil {
		return 0, err
	}
	if session.PID < 1 {
		return 0, errors.New("authenticated daemon returned an invalid PID")
	}
	return session.PID, nil
}

func sysribProducerPID(ctx context.Context, p *sdk.Plugin, daemon, previous int) (int, error) {
	var identity sysribProducerIdentity
	var last error
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		data, err := os.ReadFile(sysribProducerIdentityFile)
		if err != nil {
			last = err
			return false
		}
		if err := json.Unmarshal(data, &identity); err != nil {
			last = err
			return false
		}
		if identity.PID == previous {
			return false
		}
		last = sysribProducerCheckIdentity(ctx, p, daemon, identity)
		return last == nil
	}) {
		return 0, fmt.Errorf("new authenticated SDK producer identity not observed (previous PID %d): %w", previous, last)
	}
	return identity.PID, importSay("SDK route-install producer has a live authenticated daemon descendant PID " + strconv.Itoa(identity.PID))
}

func sysribProducerCheckIdentity(ctx context.Context, p *sdk.Plugin, daemon int, identity sysribProducerIdentity) error {
	if identity.Daemon != daemon {
		return fmt.Errorf("producer belongs to daemon %d, want %d", identity.Daemon, daemon)
	}
	if identity.PID < 2 {
		return fmt.Errorf("invalid producer PID %d", identity.PID)
	}
	parents, err := sysribExecutionParentCensus(ctx, "/proc")
	if err != nil {
		return err
	}
	parent := parents[identity.PID]
	descendant := false
	for range 16 {
		if parent == daemon {
			descendant = true
			break
		}
		if parent < 2 {
			break
		}
		parent = parents[parent]
	}
	if !descendant {
		return fmt.Errorf("producer PID %d is not a descendant of daemon %d", identity.PID, daemon)
	}
	root := "/proc/" + strconv.Itoa(identity.PID)
	raw, err := os.ReadFile(root + "/cmdline")
	if err != nil {
		return err
	}
	argv := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	if len(argv) != 4 {
		return fmt.Errorf("producer PID %d argv %q does not name the exact SDK producer", identity.PID, argv)
	}
	if filepath.Base(argv[0]) != "le" {
		return fmt.Errorf("producer executable argv %q is not le", argv[0])
	}
	for i, want := range []string{"test", "fixture", sysribProducerFixture} {
		if argv[i+1] != want {
			return fmt.Errorf("producer argv %q does not match %q", argv, want)
		}
	}
	executable, err := sysribProcessExecutable(ctx, identity.PID)
	if err != nil {
		return err
	}
	selfExecutable, err := sysribProcessExecutable(ctx, os.Getpid())
	if err != nil {
		return err
	}
	if executable != selfExecutable {
		return fmt.Errorf("producer executable %q differs from fixture %q", executable, selfExecutable)
	}
	data, err := requireDone02(ctx, p, "system subsystem list")
	if err != nil {
		return err
	}
	var state struct {
		Subsystems []struct {
			Name    string `json:"name"`
			Running bool   `json:"running"`
			Stage   string `json:"stage"`
		} `json:"subsystems"`
	}
	if err := decode02(data, &state); err != nil {
		return err
	}
	for _, owner := range state.Subsystems {
		if owner.Name != sysribProducerOwner {
			continue
		}
		if !owner.Running {
			return fmt.Errorf("SDK producer is not Running: %s", data)
		}
		if owner.Stage != "Running" {
			return fmt.Errorf("SDK producer has not completed startup: %s", data)
		}
		return nil
	}
	return fmt.Errorf("configured SDK producer is absent: %s", data)
}

func sysribProducerKill(ctx context.Context, pid int) error {
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return err
	}
	var last error
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		state, err := daemonProcessState(ctx, pid)
		last = err
		if err != nil {
			return false
		}
		if state == "" {
			return true
		}
		return strings.HasPrefix(state, "Z")
	}) {
		return nil
	}
	return fmt.Errorf("producer PID %d did not exit after SIGKILL: %w", pid, last)
}

func sysribProducerStaticPresent(ctx context.Context, p *sdk.Plugin) error {
	tag, zero := uint32(4242), uint32(0)
	if err := importCandidateState(ctx, p, sysribProducerStaticHidden, "static", 10, &tag, false, "central", "rib import"); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribProducerStaticZero, "static", 10, &zero, true, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribProducerStatic, "static", 10, nil, true, "", ""); err != nil {
		return err
	}
	for _, prefix := range []string{sysribProducerStatic, sysribProducerStaticZero} {
		if err := importKernel(ctx, prefix, 1, "6 "+prefix+" ", "proto 250 "); err != nil {
			return err
		}
	}
	return importKernel(ctx, sysribProducerStaticHidden, 0)
}

func sysribProducerSDKPresent(ctx context.Context, p *sdk.Plugin) error {
	tag := uint32(4242)
	if err := importCandidateState(ctx, p, sysribProducerSDKHidden, "ospf", 110, &tag, false, "central", "rib import"); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribProducerSDK, "ospf", 110, nil, true, "", ""); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribProducerSDK, 1, "6 "+sysribProducerSDK+" ", "proto 250 "); err != nil {
		return err
	}
	return importKernel(ctx, sysribProducerSDKHidden, 0)
}

func sysribProducerSurvivors(ctx context.Context, p *sdk.Plugin) error {
	if err := waitRIBWinner(ctx, p, sysribProducerOther, "static"); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, p, sysribProducerWithheld, "bgp"); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribProducerOther, 1, "6 "+sysribProducerOther+" ", "proto 250 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribProducerConnected, 1, "dev "+sysribProducerLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribProducerForeign, 1, "6 "+sysribProducerForeign+" ", "proto 42 "); err != nil {
		return err
	}
	return importKernel(ctx, sysribProducerWithheld, 0)
}

func sysribProducerAbsent(ctx context.Context, p *sdk.Plugin, source string, prefixes ...string) error {
	for _, prefix := range prefixes {
		var last string
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			data, err := requireDone02(ctx, p, "show rib candidates prefix "+prefix)
			if err != nil {
				last = err.Error()
				return false
			}
			last = text02(data)
			var rows []candidateRow
			if decode02(data, &rows) != nil {
				return false
			}
			for _, row := range rows {
				if row.Prefix != prefix {
					continue
				}
				if _, found := candidatePathOf(row, source); found {
					return false
				}
			}
			return true
		}) {
			return fmt.Errorf("disconnected %s producer retained %s candidate: %s", source, prefix, last)
		}
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return nil
}

func sysribProducerKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("producer kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribProducerConnected, sysribProducerStatic, sysribProducerStaticHidden, sysribProducerStaticZero, sysribProducerSDK, sysribProducerSDKHidden, sysribProducerOther, sysribProducerWithheld, sysribProducerForeign} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("producer fixture prefixes are absent from every kernel table")
}

// This runs only after daemon/await-exit. Ze must have flushed its own
// prefixes while the actual device, its OS-owned route and the foreign proto
// 42 route still exist; deleting them first could erase a stale FIB entry and
// make that absence vacuous, and would hide a stop that flushed foreign state.
func sysribProducerLinkCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("producer post-stop link cleanup takes no arguments")
	}
	for _, prefix := range []string{sysribProducerStatic, sysribProducerStaticHidden, sysribProducerStaticZero, sysribProducerSDK, sysribProducerSDKHidden, sysribProducerOther, sysribProducerWithheld} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := importKernel(ctx, sysribProducerConnected, 1, "dev "+sysribProducerLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribProducerForeign, 1, "6 "+sysribProducerForeign+" ", "proto 42 "); err != nil {
		return err
	}
	if _, err := importCommandOutput(ctx, "ip", "link", "show", "dev", sysribProducerLink); err != nil {
		return fmt.Errorf("joined daemon stop removed fixture device %s: %w", sysribProducerLink, err)
	}
	if err := importDeleteForeignBlackhole(ctx, sysribProducerForeign); err != nil {
		return err
	}
	if err := importDeleteLink(ctx, sysribProducerLink); err != nil {
		return err
	}
	return importSay("joined daemon flushed Ze producer routes while the fixture device, its OS route and the foreign proto 42 route survived before owned cleanup")
}
