// Design: docs/architecture/testing/qemu-integration.md -- native appliance configuration push.
package testqemu

import (
	"context"
	"runtime"
	"testing"
)

// TestApplianceConfigPushActionIsNative keeps the product-command proof
// addressable independently of the HTTPS mutation used by appliance-ota.
func TestApplianceConfigPushActionIsNative(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != "appliance-config-push" {
			continue
		}
		if action.Forwards {
			t.Fatal("appliance-config-push forwards words instead of owning the native proof")
		}
		if err := Actions().Accepts(action.Verb, nil); err != nil {
			t.Fatalf("native appliance-config-push invocation refused: %v", err)
		}
		return
	}
	t.Fatal("missing appliance-config-push action: HTTPS upload does not prove the SSH product command")
}

type applianceConfigPushRunner interface {
	executeApplianceConfigPush(context.Context) (InstallReport, error)
}

// TestApplianceConfigPushRefusesUnboundImage prevents a superficially green
// native proof against an image that predates the real SSH upload receiver.
func TestApplianceConfigPushRefusesUnboundImage(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Arch = runtime.GOARCH
	installer.Options.Image = "old-image.img"
	runner, ok := any(installer).(applianceConfigPushRunner)
	if !ok {
		t.Fatal("Installer has no native appliance config-push runner")
	}
	calls := 0
	installer.ops.Run = func(context.Context, commandSpec) (commandResult, error) {
		calls++
		return commandResult{}, nil
	}
	report, err := runner.executeApplianceConfigPush(t.Context())
	if err == nil || report.Verdict == InstallVerdictPass {
		t.Fatalf("unbound existing image accepted: report=%+v error=%v", report, err)
	}
	if calls != 0 {
		t.Fatalf("unbound image ran %d preparation commands before refusal", calls)
	}
}

// TestApplianceConfigPushRequiresNativeArchitecture prevents an emulated
// foreign guest from being reported as the deployment's native proof.
func TestApplianceConfigPushRequiresNativeArchitecture(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Arch = ArchAMD64
	if runtime.GOARCH == ArchAMD64 {
		installer.Options.Arch = ArchARM64
	}
	runner, ok := any(installer).(applianceConfigPushRunner)
	if !ok {
		t.Fatal("Installer has no native appliance config-push runner")
	}
	calls := 0
	installer.ops.Run = func(context.Context, commandSpec) (commandResult, error) {
		calls++
		return commandResult{}, nil
	}
	report, err := runner.executeApplianceConfigPush(t.Context())
	if err == nil || report.Verdict == InstallVerdictPass {
		t.Fatalf("foreign architecture accepted: report=%+v error=%v", report, err)
	}
	if calls != 0 {
		t.Fatalf("foreign architecture ran %d preparation commands before refusal", calls)
	}
}
