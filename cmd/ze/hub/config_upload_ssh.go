//go:build ze_ssh

// Design: docs/architecture/appliance/remote-operations.md -- authenticated SSH configuration upload.
// Related: config_upload.go -- daemon mutation admission, validation and commit.

package hub

import (
	"context"
	"io"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/configupload"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func registerSSHConfigUpload(server *zessh.Server, owner *configMutationOwner, recorder audit.Recorder) error {
	if owner == nil {
		return opererr.New("SSH configuration mutation owner is missing", "The daemon cannot safely accept configuration uploads", "ze start --help")
	}
	return server.RegisterSubsystem(configupload.Subsystem, func(ctx context.Context, channel io.ReadWriteCloser, caller plugin.CallerIdentity) error {
		if caller.Authorizer == nil {
			return configUploadDenied()
		}
		if !caller.Authorizer.Authorize(caller.Username, caller.RemoteAddr, api.ConfigAuthCommit, false) {
			return configUploadDenied()
		}
		content, err := configupload.Read(channel)
		if err != nil {
			return opererr.New("configuration upload frame is incomplete or invalid", "The daemon requires a nonempty configuration of at most four MiB followed by SSH EOF; no replacement was applied", "./ze-host appliance config-push <name>")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := owner.Upload(ctx, content); err != nil {
			return err
		}
		if recorder != nil {
			if err := recorder.Record(audit.Entry{Actor: caller.Username, RemoteAddr: caller.RemoteAddr, Surface: audit.SSH, Action: audit.ActionConfigUpload, Detail: "uploaded configuration", Outcome: audit.OutcomeSuccess}); err != nil {
				slogutil.Logger("hub.ssh").Warn("audit record failed", "code", "doctor-audit-record-failed", "action", audit.ActionConfigUpload, "user", caller.Username, "error", err)
			}
		}
		_, err = io.WriteString(channel, configupload.Receipt)
		return err
	})
}

func configUploadDenied() error {
	return opererr.New("configuration upload is not authorized", "This authenticated SSH identity has no configuration commit permission; connect using an account authorized to commit configuration", "ze cli --user <authorized-user>")
}
