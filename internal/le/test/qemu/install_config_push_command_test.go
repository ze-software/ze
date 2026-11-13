// Design: docs/architecture/testing/qemu-integration.md -- actual host-driver command boundary.
package testqemu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type configPushCommandRunner interface {
	configPushCommand(context.Context, installISOImage, []string, bool) (commandResult, error)
}

// TestApplianceConfigPushUsesCurrentHostDriver proves the positive command
// boundary rather than treating an HTTP upload as the native command proof.
func TestApplianceConfigPushUsesCurrentHostDriver(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	runner, ok := any(installer).(configPushCommandRunner)
	if !ok {
		t.Fatal("native proof does not invoke the current host-driver config-push command")
	}
	image := installISOImage{Ze: "/owned/current-ze", Env: []string{"ZE_APPLIANCE_DIR=/owned/appliances"}}
	identity := []string{"HOME=/owned/home", "SSH_AUTH_SOCK=/owned/agent.sock"}
	var calls []commandSpec
	installer.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		calls = append(calls, spec)
		return commandResult{Stdout: "config applied to " + InstallApplianceName + "\n"}, nil
	}
	for _, preview := range []bool{true, false} {
		if _, err := runner.configPushCommand(t.Context(), image, identity, preview); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("host-driver calls=%d", len(calls))
	}
	for index, call := range calls {
		want := []string{"appliance", "config-push"}
		if index == 0 {
			want = append(want, "--dry-run")
		}
		want = append(want, InstallApplianceName)
		if call.Name != image.Ze || call.Dir != installer.Tree || !reflect.DeepEqual(call.Args, want) {
			t.Fatalf("wrong current driver command: %+v", call)
		}
		for _, value := range append(slices.Clone(image.Env), identity...) {
			if !slices.Contains(call.Env, value) {
				t.Fatalf("child lost owned environment %q: %v", value, call.Env)
			}
		}
	}
}

// TestApplianceConfigPushRequiresSuccessReceipt prevents exit zero alone from
// claiming a publication and prevents a nonzero result carrying stale success.
func TestApplianceConfigPushRequiresSuccessReceipt(t *testing.T) {
	for _, result := range []commandResult{{Code: 0}, {Code: 1, Stdout: "config applied to " + InstallApplianceName + "\n"}} {
		installer := installTestInstaller(t, InstallKindHTTP)
		runner, ok := any(installer).(configPushCommandRunner)
		if !ok {
			t.Fatal("native proof has no product receipt validation")
		}
		calls := 0
		installer.ops.Run = func(context.Context, commandSpec) (commandResult, error) { calls++; return result, nil }
		image := installISOImage{Ze: "/owned/current-ze", Env: []string{"ZE_APPLIANCE_DIR=/owned/appliances"}}
		if _, err := runner.configPushCommand(t.Context(), image, nil, false); err == nil {
			t.Fatalf("unconfirmed publication accepted: %+v", result)
		}
		if calls != 1 {
			t.Fatalf("receipt assertion did not reach the driver: calls=%d", calls)
		}
	}
}

func TestApplianceConfigPushHostFailureDoesNotExposePayload(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	runner, ok := any(installer).(configPushCommandRunner)
	if !ok {
		t.Fatal("native proof has no private host-command diagnostics")
	}
	calls := 0
	installer.ops.Run = func(context.Context, commandSpec) (commandResult, error) {
		calls++
		return commandResult{Code: 1, Stderr: "private-config-password"}, errors.New("private-config-password")
	}
	image := installISOImage{Ze: "/owned/current-ze", Env: []string{"ZE_APPLIANCE_DIR=/owned/appliances"}}
	_, err := runner.configPushCommand(t.Context(), image, nil, false)
	if err == nil {
		t.Fatal("failed host command accepted")
	}
	if strings.Contains(err.Error(), "private-config-password") {
		t.Fatalf("private command payload leaked: %v", err)
	}
	if calls != 1 {
		t.Fatalf("private-error assertion did not reach the driver: calls=%d", calls)
	}
}
