// Design: docs/architecture/api/process-protocol.md -- the plugin name a connect-back claims
// Related: tls.go -- the auth handshake that refuses a name this file does not accept
// Related: ../../config/validators.go -- ExternalPluginNameValidator, the commit-time reader of the same declaration

package ipc

import (
	"errors"
	"regexp"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PluginNameForm says, for an operator, which plugin names a connect-back
// accepts. It is the prose of pluginNamePattern and the two change together.
const PluginNameForm = "1 to 64 letters, digits and hyphens, the first a letter or a digit"

// pluginNamePattern is the ONE declaration of the names a plugin may claim in
// its auth request. The hub binds each external plugin's token to that name,
// so the config side (the `external-plugin-name` validator on
// plugin/external/name) reads this same pattern: a name the commit accepts is
// a name the connect-back accepts.
var pluginNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

// ValidPluginName reports whether name is a plugin name a connect-back
// accepts.
func ValidPluginName(name string) bool {
	return pluginNamePattern.MatchString(name)
}

// CheckPluginName refuses a plugin name a connect-back would refuse. The error
// names the value and the allowed form, so the config refusal tells the
// operator what to write instead.
func CheckPluginName(name string) error {
	if ValidPluginName(name) {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("plugin name ").Quoted(name).Str(" is refused: the hub binds the connect-back token to the name, and a name is ").
		Str(PluginNameForm)
	return errors.New(tb.String())
}
