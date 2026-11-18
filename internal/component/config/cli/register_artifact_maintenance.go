// Design: docs/architecture/web-interface.md -- registered native artifact maintenance.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-bytes-begin", Handler: artifactHandler(artifactBytesBegin)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-key-list", Handler: artifactHandler(artifactKeyList), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-key-read", Handler: artifactHandler(artifactKeyRead)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-key-write", Handler: artifactHandler(artifactKeyWrite)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-key-delete", Handler: artifactHandler(artifactKeyDelete)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-check", Handler: artifactResponseHandler(artifactCheck), NotReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-repair", Handler: artifactResponseHandler(artifactRepair)},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-resume", Handler: artifactHandler(artifactSessionOperation("resume"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-diff", Handler: artifactHandler(artifactSessionOperation("diff"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-changes", Handler: artifactHandler(artifactSessionOperation("changes"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-history", Handler: artifactHandler(artifactSessionOperation("history"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-set", Handler: artifactHandler(artifactSessionOperation("set"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-activate", Handler: artifactHandler(artifactSessionOperation("activate"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-deactivate", Handler: artifactHandler(artifactSessionOperation("deactivate"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-delete", Handler: artifactHandler(artifactSessionOperation("delete"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-replace", Handler: artifactHandler(artifactSessionOperation("replace"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-merge", Handler: artifactHandler(artifactSessionOperation("merge"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-save", Handler: artifactHandler(artifactSessionOperation("save"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-discard", Handler: artifactHandler(artifactSessionOperation("discard"))},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-session-commit", Handler: artifactHandler(artifactSessionOperation("commit"))},
	)
	command.RegisterShape([]string{"show config artifact key list"}, command.ShapeTab)
	command.RegisterShape([]string{"request config artifact bytes begin", "request config artifact key read",
		"request config artifact key write", "request config artifact key delete", "show config artifact check", "request config artifact repair"}, command.ShapeDoc)
	command.RegisterShape([]string{"request config artifact session resume", "request config artifact session diff",
		"request config artifact session changes", "request config artifact session history", "request config artifact session set",
		"request config artifact session activate", "request config artifact session deactivate", "request config artifact session delete",
		"request config artifact session replace", "request config artifact session merge", "request config artifact session save",
		"request config artifact session discard", "request config artifact session commit"}, command.ShapeDoc)
}
