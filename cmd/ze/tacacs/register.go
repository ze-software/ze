// Register the tacacs root command with the cmd/ze dispatcher.

package tacacs

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("tacacs", cmdregistry.Meta{
		Description: "TACACS+ client helpers",
		Mode:        "offline",
		Subs:        "",
	})
}
