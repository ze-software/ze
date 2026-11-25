// Design: docs/functional-tests.md -- config refusals retain the applied tree.
// Related: test/reload/pppoe-vpp-backend-reload-refused.ci -- commit and reload boundaries.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/pppoe-vpp-backend-refused", pppoeBackendRefusal, StopsDaemon)
}

// pppoeBackendRefusal reaches the real editor commit and SIGHUP loader. Neither
// may publish the candidate, and a valid reload afterwards must still apply.
func pppoeBackendRefusal(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("reload/pppoe-vpp-backend-refused needs a gRPC address and TLS certificate path")
	}
	return Observe(ctx, "pppoe-backend-refusal", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		ctx, cancel := context.WithTimeout(ctx, WaitBudget(70, 30*time.Second))
		defer cancel()
		before, err := pppoeBackendRunning(ctx, plugin)
		if err != nil {
			return err
		}
		interfaces, ok := before["interface"].(map[string]any)
		if !ok {
			return fmt.Errorf("initial interface backend is not kernel: %v", before["interface"])
		}
		if interfaces["backend"] != "kernel" {
			return fmt.Errorf("initial interface backend is not kernel: %v", before["interface"])
		}
		if err := pppoeBackendCommit(ctx, args[0], args[1]); err != nil {
			return err
		}
		if err := pppoeBackendUnchanged(ctx, plugin, before); err != nil {
			return fmt.Errorf("after the refused commit: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: PPPoE backend commit refused before changing the running configuration") //nolint:errcheck // Assertions above determine the scenario result.
		if err := liveStagesReload(ctx, plugin, "config2.conf"); err != nil {
			return err
		}
		if err := pppoeBackendReloadOutcome(ctx, plugin, "failed"); err != nil {
			return err
		}
		if err := pppoeBackendUnchanged(ctx, plugin, before); err != nil {
			return fmt.Errorf("after the refused reload completed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: PPPoE backend reload refused and the running configuration stayed unchanged") //nolint:errcheck // Assertions above determine the scenario result.
		if err := liveStagesReload(ctx, plugin, "config3.conf"); err != nil {
			return err
		}
		if err := pppoeBackendReloadOutcome(ctx, plugin, "applied"); err != nil {
			return err
		}
		after, err := pppoeBackendRunning(ctx, plugin)
		if err != nil {
			return err
		}
		access, ok := after["pppoe"].(map[string]any)
		if !ok {
			return fmt.Errorf("valid reload did not publish the new PPPoE ac-name: %v", after["pppoe"])
		}
		if access["ac-name"] != "after" {
			return fmt.Errorf("valid reload did not publish the new PPPoE ac-name: %v", after["pppoe"])
		}
		fmt.Fprintln(os.Stderr, "OK: a subsequent supported PPPoE configuration applied") //nolint:errcheck // The running tree above determines success.
		return nil
	})
}

// pppoeBackendCommit edits a real candidate over authenticated TLS. A missing
// commit hook would accept it, so the exact refusal is part of the assertion.
func pppoeBackendCommit(ctx context.Context, address, certificate string) error {
	conn, err := configRefusalClient(address, certificate)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // Closing a read/test client changes no assertion.
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer secret")
	client := zepb.NewZeConfigServiceClient(conn)
	if !Poll(ctx, WaitAttempts(40, plugin01PollDelay, 40), plugin01PollDelay, func() bool {
		_, err = client.GetRunningConfig(ctx, &zepb.Empty{})
		// refuse-by-default: guards Poll -- a completed RPC opens the listener readiness barrier; false keeps it closed.
		return err == nil
	}) {
		return fmt.Errorf("gRPC config listener did not become ready: %w", err)
	}
	created, err := client.EnterSession(ctx, &zepb.Empty{})
	if err != nil {
		return err
	}
	id := created.GetSessionId()
	if id == "" {
		return errors.New("gRPC EnterSession returned no session ID")
	}
	for _, setting := range []struct{ path, value string }{
		{"interface.backend", "vpp"},
		{"pppoe.interface.access0.max-sessions", "10"},
	} {
		answer, err := client.SetConfig(ctx, &zepb.ConfigSetRequest{SessionId: id, Path: setting.path, Value: setting.value})
		if err != nil {
			return fmt.Errorf("set %s: %w", setting.path, err)
		}
		if !answer.GetSuccess() {
			return fmt.Errorf("set %s refused the candidate before commit: %s", setting.path, answer.GetMessage())
		}
	}
	_, err = client.CommitSession(ctx, &zepb.CommitRequest{SessionId: id})
	if status.Code(err) != codes.InvalidArgument {
		return fmt.Errorf("PPPoE backend commit status = %s, want InvalidArgument: %w", status.Code(err), err)
	}
	text := status.Convert(err).Message()
	findingLines, err := pppoeBackendFinding(text)
	if err != nil {
		return err
	}
	finding := strings.Join(findingLines, "\n")
	for _, want := range []string{pppoeBackendPath, `backend "vpp"`, "supported: kernel", "no PPPoE session data plane"} {
		if !strings.Contains(finding, want) {
			return fmt.Errorf("PPPoE commit refusal lacks %q: %s", want, text)
		}
	}
	if strings.TrimSpace(findingLines[len(findingLines)-1]) != "fix: set interface backend kernel" {
		return fmt.Errorf("PPPoE commit refusal does not end with its repair command: %s", text)
	}
	_, err = client.DiscardSession(ctx, &zepb.SessionRequest{SessionId: id})
	return err
}

const pppoeBackendPath = "/pppoe/interface/access0"

// pppoeBackendFinding isolates the PPPoE backend refusal block from a
// multi-finding commit validation error so every assertion judges that finding
// itself rather than a sibling plugin's refusal.
func pppoeBackendFinding(text string) ([]string, error) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.Contains(line, pppoeBackendPath) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("PPPoE commit refusal lacks %q: %s", pppoeBackendPath, text)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "  config: ") {
			end = i
			break
		}
		if strings.HasPrefix(lines[i], "  config validation failed:") {
			end = i
			break
		}
	}
	return lines[start:end], nil
}

// pppoeBackendRunning reads the daemon's applied parsed tree, not the file that
// a refused SIGHUP has already replaced with an invalid candidate.
func pppoeBackendRunning(ctx context.Context, plugin *sdk.Plugin) (map[string]any, error) {
	var tree map[string]any
	var answer string
	var err error
	if !Poll(ctx, liveStagesPollAttempts, liveStagesPollDelay, func() bool {
		answer, err = Dispatch(ctx, plugin, "show config running", &tree)
		if err != nil {
			return false
		}
		if answer != statusDone {
			return false
		}
		return tree != nil
	}) {
		return nil, fmt.Errorf("show config running did not become ready: status=%q tree=%v: %w", answer, tree, err)
	}
	return tree, nil
}

func pppoeBackendUnchanged(ctx context.Context, plugin *sdk.Plugin, before map[string]any) error {
	after, err := pppoeBackendRunning(ctx, plugin)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("running configuration changed: before=%v after=%v", before, after)
	}
	return nil
}

func pppoeBackendReloadOutcome(ctx context.Context, plugin *sdk.Plugin, want string) error {
	var result struct {
		Outcome string `json:"last-outcome"`
	}
	answer, err := Dispatch(ctx, plugin, "show reload-status", &result)
	if err != nil {
		return err
	}
	if answer != statusDone {
		return fmt.Errorf("reload status = %q, outcome = %q, want %s", answer, result.Outcome, want)
	}
	if result.Outcome != want {
		return fmt.Errorf("reload status = %q, outcome = %q, want %s", answer, result.Outcome, want)
	}
	return nil
}
