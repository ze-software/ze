// Design: docs/architecture/testing/qemu-integration.md -- native host-driver push boundary.
package testqemu

import (
	"context"
	"errors"
	"strings"
)

func (installer *Installer) configPushCommand(ctx context.Context, image installISOImage, identity []string, preview bool) (commandResult, error) {
	args := []string{zeApplianceArea, "config-push"}
	if preview {
		args = append(args, "--dry-run")
	}
	args = append(args, InstallApplianceName)
	result, err := installer.run(ctx, commandSpec{Name: image.Ze, Args: args, Dir: installer.Tree, Env: installEnvSet(image.Env, identity...)})
	if err != nil {
		// Child output and launch errors may carry credentials or a complete
		// replacement payload. Only fixed phase diagnostics leave the proof.
		return result, errors.New("native appliance config-push command did not finish")
	}
	if result.Code != 0 {
		return result, errors.New("native appliance config-push command was refused")
	}
	if preview {
		return result, nil
	}
	if strings.TrimSpace(result.Stdout) != "config applied to "+InstallApplianceName {
		return result, errors.New("native appliance config-push command has no exact success receipt")
	}
	return result, nil
}
