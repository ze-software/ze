// Design: docs/architecture/container/network.md -- the firewall policy a published port must pass
// Related: main.go -- the dispatch of the blocked and adopt modes
// Related: serve.go -- dialWithin, and the published mode the blocked mode is the opposite of
// Related: stages.go -- zeCLI, which the adopt mode asks the daemon through

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// blockedTimeout bounds the one attempt of the blocked mode. A packet the
// firewall drops is never answered, so the attempt ends at this bound; an
// attempt that is not dropped is answered, by the container or by a reset,
// long before it, on a loaded guest too.
const blockedTimeout = 3 * time.Second

// commandReload asks the daemon to read its configuration file again, and
// reloadedAnswer is what the daemon answers once every plugin applied it
// (internal/component/plugin/server/system.go messageConfigReloaded): the
// probe links no ze package.
const (
	commandReload  = "request reload"
	reloadedAnswer = "configuration reloaded"
)

// blocked runs on the client's side: one connection attempt to ADDRESS:PORT
// must get no answer within blockedTimeout, which is what a packet dropped
// on its way looks like from the client. A connection that opens, a refusal
// and any other failure, an unreachable network included, is a FAIL.
func blocked(address, port string) bool {
	target := net.JoinHostPort(address, port)
	connection, err := dialWithin(target, blockedTimeout)
	if err == nil {
		if closeErr := connection.Close(); closeErr != nil {
			fmt.Println("probe: the connection did not close cleanly: " + closeErr.Error())
		}
		fmt.Println("probe: FAIL the connection to " + target + " got no answer: it opened")
		return false
	}
	failure, isNetwork := errors.AsType[net.Error](err)
	if !isNetwork {
		fmt.Println("probe: FAIL the connection to " + target + " got no answer: " + err.Error())
		return false
	}
	if !failure.Timeout() {
		fmt.Println("probe: FAIL the connection to " + target + " got no answer: " + err.Error())
		return false
	}
	fmt.Println("probe: the connection to " + target + " got no answer")
	return true
}

// adopt runs on the host beside the daemon: it writes the content of the
// file replacement over config, the file the daemon started on, then asks
// the daemon to reload it and requires the daemon's reloaded answer, which
// the daemon gives once every plugin applied the new configuration.
func adopt(config, replacement string) bool {
	data, err := os.ReadFile(replacement) //nolint:gosec // cliio:allow containerprobe reads the replacement config file on disk
	if err != nil {
		return judge("the replacement configuration is read", false, err.Error())
	}
	if err := os.WriteFile(config, data, 0o644); err != nil { //nolint:gosec // cliio:allow containerprobe overwrites the daemon config file on disk
		return judge("the replacement configuration is written", false, err.Error())
	}
	answer, err := zeCLI(commandReload)
	if err != nil {
		return judge("the daemon reloaded its configuration", false, err.Error())
	}
	return judge("the daemon reloaded its configuration", strings.Contains(string(answer), reloadedAnswer), oneLine(answer))
}
