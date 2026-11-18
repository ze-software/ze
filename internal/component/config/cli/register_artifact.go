// Design: docs/architecture/api/commands.md -- daemon-owned private artifact commands.
// Related: artifact_rpc.go -- authenticated adapters over the sole artifact owner.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-upload-begin", Handler: artifactHandler(artifactUploadBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-text-begin", Handler: artifactHandler(artifactTextBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-upload-append", Handler: artifactHandler(artifactUploadAppend)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-upload-finish", Handler: artifactHandler(artifactUploadFinish)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-upload-abort", Handler: artifactHandler(artifactTransferAbort)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-download-begin", Handler: artifactHandler(artifactDownloadBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-source-begin", Handler: artifactHandler(artifactSourceBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-transfer-read", Handler: artifactHandler(artifactTransferRead), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-list", Handler: artifactHandler(artifactList), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-info", Handler: artifactHandler(artifactInfo), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-source", Handler: artifactHandler(artifactSource), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-delete", Handler: artifactHandler(artifactDelete)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-validate", Handler: artifactHandler(artifactValidate)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-review", Handler: artifactHandler(artifactReview)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-save", Handler: artifactHandler(artifactSave)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-configs", Handler: artifactHandler(artifactConfigs), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-history", Handler: artifactHandler(artifactHistory), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-diff", Handler: artifactHandler(artifactDiff), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-set", Handler: artifactHandler(artifactSet)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-activate", Handler: artifactHandler(artifactActivate)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-deactivate", Handler: artifactHandler(artifactDeactivate)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-replace", Handler: artifactHandler(artifactReplace)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-rollback", Handler: artifactHandler(artifactRollback)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-restore-full", Handler: artifactRestoreFull},
	)
	command.RegisterShape([]string{"show config artifact list"}, command.ShapeTab)
	command.RegisterShape([]string{"show config artifact source"}, command.ShapeBlob)
	command.RegisterShape([]string{"show config artifact info", "show config artifact transfer read",
		"request config artifact upload begin", "request config artifact text begin", "request config artifact upload append",
		"request config artifact upload finish", "request config artifact upload abort", "request config artifact download begin",
		"request config artifact source begin", "request config artifact validate", "request config artifact review",
		"request config artifact save", "delete config artifact"}, command.ShapeDoc)
	command.RegisterShape([]string{"show config artifact config list", "show config artifact history"}, command.ShapeTab)
	command.RegisterShape([]string{"show config artifact diff", "request config artifact set", "request config artifact activate",
		"request config artifact deactivate", "request config artifact replace", "request config artifact rollback",
		"request config artifact restore full"}, command.ShapeDoc)
}
