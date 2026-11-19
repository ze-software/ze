// Design: docs/architecture/config/yang-config-design.md -- config editor
// Related: model_commands.go -- dispatchCommandWords, where the gate runs
// Related: model.go -- SetConfigAuthorizer, the field the gate reads

package cli

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConfigAuthorizer answers whether the session's user may perform a
// configuration change, asked with a config permission string
// (plugin.ConfigAuth*, with the configuration name appended for a session on
// a named stored configuration). The caller binds the user, the remote address
// and the profile; the editor holds no authorization type of its own.
type ConfigAuthorizer func(permission string) bool

// configPermissions maps every editor verb that changes the candidate, the
// store or the running configuration to the permission string an authorizer
// is asked about. It is the one declaration of that mapping: the SSH session
// editor gates its dispatch with it and the web terminal reads it through
// ConfigPermission. A verb absent here is ungated, so a new mutating verb in
// dispatchCommandWords needs its entry (TestEveryMutatingEditorVerbHasAPermission
// refuses one without).
var configPermissions = map[string]string{
	cmdSet:        plugin.ConfigAuthSet,
	cmdActivate:   plugin.ConfigAuthSet,
	cmdDelete:     plugin.ConfigAuthDelete,
	cmdDeactivate: plugin.ConfigAuthDelete,
	cmdCommit:     plugin.ConfigAuthCommit,
	cmdDiscard:    plugin.ConfigAuthDiscard,
	cmdDisconnect: plugin.ConfigAuthDiscard,
	cmdRollback:   plugin.ConfigAuthRollback,
	cmdRename:     plugin.ConfigAuthRename,
	cmdCopy:       plugin.ConfigAuthAdd,
	cmdInsert:     plugin.ConfigAuthAdd,
	cmdSave:       plugin.ConfigAuthSave,
	cmdLoad:       plugin.ConfigAuthSave,
}

// ConfigPermission answers the permission string an editor verb is authorized
// with, and gated false for a verb that changes nothing (navigation, display,
// help), which no surface authorizes.
func ConfigPermission(verb string) (permission string, gated bool) {
	permission, gated = configPermissions[verb]
	return permission, gated
}

// authorizeConfigVerb refuses a gated verb the installed authorizer denies,
// before any handler touches the editor.
//
// Guard: a nil configAuthorizer means no check. Only a surface that
// authenticates nobody leaves it nil (the attached console, and `ze config
// edit` in its file mode `-f` and backup mode `--backup`; plain `ze config
// edit` opens an SSH session); the SSH session factory always installs one, failing
// closed when the connection carries no authorizer.
func (m *Model) authorizeConfigVerb(verb string) error {
	if m.configAuthorizer == nil {
		return nil
	}
	permission, gated := ConfigPermission(verb)
	if !gated {
		return nil
	}
	if m.configName != "" {
		var tb textbuf.Buffer
		permission = tb.Str(permission).Byte(' ').Str(m.configName).String()
	}
	if m.configAuthorizer(permission) {
		return nil
	}
	return configRefusal(permission)
}

// configRefusal is the editor's access-control refusal. It wraps
// pluginserver.ErrUnauthorized so every adapter classifies it as the
// dispatcher's refusal, and its first line reads as the dispatcher's does:
// plugin.UnauthorizedMessage, then the permission that was denied.
func configRefusal(permission string) error {
	var problem, fix textbuf.Buffer
	problem.Str(plugin.UnauthorizedMessage).Str(": ").Str(permission)
	fix.Str("ask an administrator for a profile whose edit section allows `").
		Str(permission).Str("`")
	return opererr.Wrap(pluginserver.ErrUnauthorized,
		problem.String(),
		"your authorization profile does not allow this configuration change",
		fix.String())
}
