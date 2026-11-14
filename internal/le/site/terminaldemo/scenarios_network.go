// Design: docs/contributing/terminal-demos.md -- shared network fixture primitives
// Related: scenarios_showcase.go -- one owned lab for standalone topics and showcase

package siteterminaldemo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// frrRunDir is the runtime directory the FRR daemons share.
const frrRunDir = "/run/frr"

// startFRR starts zebra and then each FRR protocol daemon in daemons inside
// namespace, every one reading the integrated frrConfigFile. zebra is ready
// when it has created zserv.api, which the protocol daemons connect to, so the
// protocol daemons start only after that file exists. The answer lists the
// pids, zebra first.
func startFRR(namespace string, daemons []string, state string, environ []string) ([]int, error) {
	if _, err := runCommand("install", []string{"-d", "-o", frrUser, "-g", frrUser, "-m", "775", frrRunDir}, commandOptions{}); err != nil {
		return nil, err
	}
	_ = os.Remove(filepath.Join(frrRunDir, "zserv.api"))
	_ = os.Remove(filepath.Join(frrRunDir, "zebra.pid"))
	for _, daemon := range daemons {
		_ = os.Remove(filepath.Join(frrRunDir, daemon+".pid"))
	}
	zebraPID, err := startCommand("ip", []string{ipNetns, commandExec, namespace, "/usr/lib/frr/zebra", "-f", frrConfigFile, "-i", "/run/frr/zebra.pid"}, environ, filepath.Join(state, "zebra.log"))
	if err != nil {
		return nil, err
	}
	for range 100 {
		if _, err := os.Stat("/run/frr/zserv.api"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	pids := make([]int, 0, 1+len(daemons))
	pids = append(pids, zebraPID)
	if _, err := os.Stat("/run/frr/zserv.api"); err != nil {
		return nil, errors.Join(fmt.Errorf("FRR zebra did not create zserv.api: %w", err), stopStartedFRR(pids))
	}
	for _, daemon := range daemons {
		pid, err := startCommand("ip", []string{ipNetns, commandExec, namespace, "/usr/lib/frr/" + daemon, "-f", frrConfigFile, "-i", "/run/frr/" + daemon + ".pid"}, environ, filepath.Join(state, daemon+".log"))
		if err != nil {
			return nil, errors.Join(err, stopStartedFRR(pids))
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// stopStartedFRR stops the daemons startFRR started before it failed. startFRR
// answers no pids on failure, so startFixtures records none and no later stop
// can reach these processes: they MUST end here. A daemon already gone is not
// a failure (terminateShowcasePID).
func stopStartedFRR(pids []int) error {
	var failed error
	for _, pid := range pids {
		failed = errors.Join(failed, terminateShowcasePID(pid))
	}
	return failed
}

// The iproute2 keywords, namespace and link names, and tape directives the
// network scenarios repeat. Each constant names the exact token the tool
// receives.
const (
	ipLink            = "link"
	ipPeer            = "peer"
	ipDev             = "dev"
	ipType            = "type"
	ipRoute           = "route"
	ipNetns           = "netns"
	envBin            = "env"
	nsCore            = "core"
	linkEdgeCore      = "edge-core"
	flagMode          = "--mode"
	tapeWaitDirective = "@wait"
	tapeTypeCommand   = "Type"
	matchKeyword      = "match"
)
