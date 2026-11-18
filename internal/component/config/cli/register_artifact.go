// Design: docs/architecture/api/commands.md -- daemon-owned private artifact commands.
// Related: artifact_rpc.go -- authenticated adapters over the sole artifact owner.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-system-snapshot", Handler: artifactHandler(artifactSystemSnapshot)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-upload-begin", Handler: artifactHandler(artifactUploadBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-text-begin", Handler: artifactHandler(artifactTextBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-upload-append", Handler: artifactHandler(artifactUploadAppend)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-upload-finish", Handler: artifactHandler(artifactUploadFinish)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-upload-abort", Handler: artifactHandler(artifactTransferAbort)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-download-begin", Handler: artifactHandler(artifactDownloadBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-source-begin", Handler: artifactHandler(artifactSourceBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-transfer-read", Handler: artifactHandler(artifactTransferRead), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-list", Handler: artifactHandler(artifactList), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-info", Handler: artifactHandler(artifactInfo), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-source", Handler: artifactHandler(artifactSource), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-delete", Handler: artifactHandler(artifactDelete)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-validate", Handler: artifactHandler(artifactValidate)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-review", Handler: artifactHandler(artifactReview)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-save", Handler: artifactHandler(artifactSave)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-configs", Handler: artifactHandler(artifactConfigs), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-history", Handler: artifactHandler(artifactHistory), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-diff", Handler: artifactHandler(artifactDiff), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-set", Handler: artifactHandler(artifactSet)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-activate", Handler: artifactHandler(artifactActivate)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-deactivate", Handler: artifactHandler(artifactDeactivate)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-replace", Handler: artifactHandler(artifactReplace)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-rollback", Handler: artifactHandler(artifactRollback)},
		pluginserver.RPCRegistration{WireMethod: "ze-config:artifact-restore-full", Handler: artifactRestoreFull},
	)
	command.RegisterShape([]string{"show config artifact list"}, command.ShapeTab)
	command.RegisterShape([]string{"show config artifact source"}, command.ShapeBlob)
	command.RegisterShape([]string{"show config artifact info", "show config artifact transfer read",
		"request config artifact system snapshot",
		"request config artifact upload begin", "request config artifact text begin", "request config artifact upload append",
		"request config artifact upload finish", "request config artifact upload abort", "request config artifact download begin",
		"request config artifact source begin", "request config artifact validate", "request config artifact review",
		"request config artifact save", "delete config artifact"}, command.ShapeDoc)
	command.RegisterShape([]string{"show config artifact config list", "show config artifact history"}, command.ShapeTab)
	command.RegisterShape([]string{"show config artifact diff", "request config artifact set", "request config artifact activate",
		"request config artifact deactivate", "request config artifact replace", "request config artifact rollback",
		"request config artifact restore full"}, command.ShapeDoc)
}
