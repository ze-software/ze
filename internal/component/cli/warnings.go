// Design: (none -- predates documentation)

package cli

import "codeberg.org/thomas-mangin/ze/internal/component/cli/contract"

// LoginWarning is a single warning shown at CLI login.
// Type alias of contract.LoginWarning so ssh, web, and hub use the same type.
type LoginWarning = contract.LoginWarning
