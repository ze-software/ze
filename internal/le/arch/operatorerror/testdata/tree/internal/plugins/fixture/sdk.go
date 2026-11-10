// Fixture: the SDK callback registrations.
package fixture

import (
	"errors"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func run(p *sdk.Plugin) {
	p.OnConfigVerify(func(_ []sdk.ConfigSection) error {
		return errors.New("verify refused")
	})
	p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
		return "", nil, work()
	})
}
