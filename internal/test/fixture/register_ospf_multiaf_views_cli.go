// Design: docs/functional-tests.md -- compiled fixtures a .ci drives
// Related: routing_fixture.go -- ospfMultiAFViewsTagged, ospfMultiAFViewsSelfOriginated (the state this waits for)
// Related: register_drain_signal.go -- sshclient.ExecCommand against the daemon's SSH server

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/core/env"
)

func init() {
	Register("ospf/ospf-multiaf-views-cli", ospfMultiAFViewsCLIReady)
}

// ospfMultiAFViewsCLIReady is the barrier test/ospf/ospf-multiaf-views-cli.ci runs
// before its `ze cli -c` steps. It asserts nothing about the client: those steps are the
// proof, and the runner runs them as the daemon's own user, as an operator on the box.
//
// The steps run once, so they need the state they judge to exist first. The daemon's
// readiness file is written before its SSH listener accepts, and the OSPFv3 engines
// originate their LSAs after startup, so neither the listener nor this router's
// ipv4-unicast LSA is there when the daemon is ready. This waits until the daemon's SSH
// session answers the whole database view with a tagged record for each configured family,
// the ipv4-unicast one holding an LSA this router originated: the state
// ospfMultiAFViewsScenario waits for through the plugin's dispatch.
//
// It speaks SSH in this process (sshclient.ExecCommand) rather than through `ze cli`: the
// client opens the daemon's store, which refuses a caller that does not own it, and under
// the runner's netns launch mode this fixture runs as root while the daemon and its store
// belong to ZE_TEST_UID. The in-process client reads no store, and the address, user and
// password come from the .ci's option=env lines, the same ones the `ze cli` steps read.
func ospfMultiAFViewsCLIReady(ctx context.Context, _ []string) error {
	environment := os.Environ()
	creds := sshclient.Credentials{
		Host:     addrLoopback,
		Port:     env.LookupIn(environment, "ze.ssh.port"),
		Username: env.LookupIn(environment, "ze.ssh.username"),
		Auth:     env.LookupIn(environment, "ze.ssh.password"),
	}
	if creds.Port == "" {
		return errors.New("the environment names no ze.ssh.port: declare it with option=env in the .ci")
	}
	if creds.Username == "" {
		return errors.New("the environment names no ze.ssh.username: declare it with option=env in the .ci")
	}
	if creds.Auth == "" {
		return errors.New("the environment names no ze.ssh.password: declare it with option=env in the .ci")
	}

	const command = "show ospf ipv6 database | json"
	var lastErr error
	if Poll(ctx, 100, 200*time.Millisecond, func() bool {
		answer, err := sshclient.ExecCommand(creds, command)
		if err != nil {
			lastErr = err
			return false
		}
		var records []map[string]any
		if err := json.Unmarshal([]byte(answer), &records); err != nil {
			lastErr = fmt.Errorf("not a JSON list of records: %w; output: %s", err, answer)
			return false
		}
		lastErr = ospfMultiAFViewsTagged(command, records, "")
		if lastErr != nil {
			return false
		}
		lastErr = ospfMultiAFViewsSelfOriginated(records)
		return lastErr == nil
	}) {
		fmt.Println("READY: the daemon's SSH session answers a tagged database record for each family, ipv4-unicast holding this router's LSA")
		return nil
	}
	if lastErr == nil {
		return fmt.Errorf("%s over SSH: no attempt completed: %w", command, ctx.Err())
	}
	return fmt.Errorf("%s over SSH never held a tagged record for each family with this router's ipv4-unicast LSA: %w", command, lastErr)
}
