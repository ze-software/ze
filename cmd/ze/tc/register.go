// Register the traffic-control root command with the cmd/ze dispatcher.

package tc

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("traffic-control", cmdregistry.Meta{
		Description: "Linux tc / VPP policer helpers",
		Mode:        "offline",
		Subs:        "",
	})
}
