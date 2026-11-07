// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client show command
// Related: pppoe_client.go -- the client whose status this answers
// Related: rate_command.go -- commandDecls and executeCommand, which route here
// Related: cmd/show_pppoe_client.go -- the daemon handler that forwards here
//
// The PPPoE clients run in the iface plugin's own process, like the rate
// tracker: runEngine holds them, and a plugin run external holds them in the
// subprocess. So the plugin answers their status as a command of its own,
// and the daemon handler forwards to it through the dispatcher.

package iface

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// PPPoEClientCommand is the plugin command the iface plugin answers with the
// status of every configured PPPoE client, sorted by name. The name is the
// YANG path of `show interface pppoe-client`.
const PPPoEClientCommand = "show interface pppoe-client"

// errPPPoEClientsNotRunning refuses a status call that reaches a process
// where runEngine never published its clients.
var errPPPoEClientsNotRunning = errors.New("PPPoE client status unavailable: the interface plugin has not started its PPPoE clients")

// pppoeClientSet is the running engine's client map and the lock that
// guards it. runEngine publishes it once; reconcilePPPoEClients changes the
// map only under mu.
type pppoeClientSet struct {
	mu     *sync.Mutex
	active map[string]*PPPoEClient
}

// globalPPPoEClients is the set runEngine published, nil before it runs.
var globalPPPoEClients atomic.Pointer[pppoeClientSet]

// pppoeClientAnswer is the answer of PPPoEClientCommand over set.
func pppoeClientAnswer(set *pppoeClientSet) ([]PPPoEClientStatus, error) {
	if set == nil {
		return nil, errPPPoEClientsNotRunning
	}
	set.mu.Lock()
	clients := make([]*PPPoEClient, 0, len(set.active))
	for _, client := range set.active {
		clients = append(clients, client)
	}
	set.mu.Unlock()

	statuses := make([]PPPoEClientStatus, 0, len(clients))
	for _, client := range clients {
		statuses = append(statuses, client.status())
	}
	slices.SortFunc(statuses, func(a, b PPPoEClientStatus) int { return strings.Compare(a.Name, b.Name) })
	return statuses, nil
}
