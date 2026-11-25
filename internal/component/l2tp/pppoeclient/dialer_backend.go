// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- independent CPE backend
// Related: client_native_owner.go -- same real creator, writer and cleanup owner

package pppoeclient

import (
	"errors"

	"github.com/ze-software/ze/internal/component/iface"
)

// NewDialer creates a dialer with an explicit independent-backend factory.
// The factory MUST return a distinct owner on every call and MUST preserve the
// original creation/counter/native binding contracts. On refusal the factory
// MUST clean its partial allocations and return nil plus its error. Dial takes
// ownership of every successful backend and retains it until actual original
// cleanup completes.
// No allocation or discovery happens here. A nil factory is refused. The
// established zero Dialer still opens the registered kernel backend.
func NewDialer(factory func() (iface.Backend, error)) (*Dialer, error) {
	if factory == nil {
		return nil, errors.New("pppoe-client: independent backend factory is absent")
	}
	return &Dialer{backendFactory: factory}, nil
}
