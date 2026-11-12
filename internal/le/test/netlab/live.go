// Design: docs/guide/netlab.md -- prove the rendered topology on live nodes.
package testnetlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type stateRow struct {
	State string `json:"state"`
}

type routeRow struct {
	Destination string `json:"dst"`
	Gateway     string `json:"gateway"`
	Type        string `json:"type"`
}

func statesReady(rows []stateRow, expected string, count int) bool {
	if count <= 0 {
		return false
	}
	if len(rows) != count {
		return false
	}
	for _, row := range rows {
		if row.State != expected {
			return false
		}
	}
	return true
}

func staticReady(rows []routeRow, nextHop string) bool {
	var next, discard bool
	for _, row := range rows {
		if row.Destination == "10.42.0.0/24" {
			next = row.Gateway == nextHop
		}
		if row.Destination == "10.43.0.0/24" {
			discard = row.Type == "blackhole"
		}
	}
	if !next {
		return false
	}
	return discard
}

type liveIGP struct {
	Passive bool `yaml:"passive"`
	BFD     bool `yaml:"bfd"`
}

type liveInterface struct {
	OSPF *liveIGP `yaml:"ospf"`
	ISIS *liveIGP `yaml:"isis"`
}

type liveNode struct {
	Hostname   string          `yaml:"hostname"`
	Module     []string        `yaml:"module"`
	Interfaces []liveInterface `yaml:"interfaces"`
	Loopback   struct {
		IPv4 string `yaml:"ipv4"`
	} `yaml:"loopback"`
	BGP struct {
		Neighbors []struct {
			IPv4 string `yaml:"ipv4"`
		} `yaml:"neighbors"`
	} `yaml:"bgp"`
	Routing struct {
		Static []struct {
			IPv4    string `yaml:"ipv4"`
			Nexthop struct {
				IPv4 string `yaml:"ipv4"`
			} `yaml:"nexthop"`
		} `yaml:"static"`
	} `yaml:"routing"`
}

// RunLive starts the packaged topology, proves live state and traffic, and
// tears it down even when netlab stopped after creating only some nodes.
func (c *Checker) RunLive(ctx context.Context, directory string) (err error) {
	owned, err := acquireLiveDirectory(directory)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owned.close()) }()
	directory = owned.path
	netlab, err := c.findNetlab()
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		// Read names before --cleanup removes the snapshot. A startup failure
		// can still leave a snapshot and a subset of the declared containers.
		var deployed struct {
			Nodes map[string]liveNode `yaml:"nodes"`
		}
		body, readErr := c.FS.ReadFile(filepath.Join(directory, "netlab.snapshot.yml"))
		if readErr == nil {
			err = errors.Join(err, yaml.Unmarshal(body, &deployed))
		} else if !errors.Is(readErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("read cleanup inventory: %w", readErr))
		}
		_, cleanupErr := c.liveCommand(cleanup, directory, netlab, "down", "--cleanup")
		err = errors.Join(err, cleanupErr)
		for _, node := range deployed.Nodes {
			if node.Hostname == "" {
				continue
			}
			remaining, inspectErr := c.liveCommand(cleanup, directory, "docker", "ps", "--all", "--filter", "name=^/"+regexp.QuoteMeta(node.Hostname)+"$", "--format", "{{.Names}}")
			err = errors.Join(err, inspectErr)
			if strings.TrimSpace(remaining) != "" {
				err = errors.Join(err, fmt.Errorf("netlab teardown left container %s", node.Hostname))
			}
		}
	}()
	if _, err := c.liveCommand(ctx, directory, netlab, "up", "--no-config", "topology.yml"); err != nil {
		return err
	}
	if _, err := c.liveCommand(ctx, directory, netlab, "initial"); err != nil {
		return err
	}
	body, err := c.FS.ReadFile(filepath.Join(directory, "netlab.snapshot.yml"))
	if err != nil {
		return fmt.Errorf("read live topology snapshot: %w", err)
	}
	var snapshot struct {
		Nodes map[string]liveNode `yaml:"nodes"`
	}
	if err := yaml.Unmarshal(body, &snapshot); err != nil {
		return fmt.Errorf("decode live topology: %w", err)
	}
	if len(snapshot.Nodes) != 3 {
		return fmt.Errorf("reference lab has %d nodes, want 3", len(snapshot.Nodes))
	}
	for _, name := range []string{"r1", "r2", "r3"} {
		node, exists := snapshot.Nodes[name]
		if !exists {
			return fmt.Errorf("reference lab is missing %s", name)
		}
		modules := []string{"bgp"}
		if name != "r3" {
			modules = append(modules, "ospf", "isis", "bfd", "routing")
		}
		for _, module := range modules {
			if !slices.Contains(node.Module, module) {
				return fmt.Errorf("reference node %s is missing declared module %s", name, module)
			}
		}
	}
	for _, name := range sortedNames(snapshot.Nodes) {
		node := snapshot.Nodes[name]
		if node.Hostname == "" {
			return fmt.Errorf("node %s has no container hostname", name)
		}
		if err := c.proveNode(ctx, directory, name, node); err != nil {
			return err
		}
	}
	for _, sourceName := range sortedNames(snapshot.Nodes) {
		source := snapshot.Nodes[sourceName]
		for _, targetName := range sortedNames(snapshot.Nodes) {
			if sourceName == targetName {
				continue
			}
			target := snapshot.Nodes[targetName]
			if source.Loopback.IPv4 == "" {
				return fmt.Errorf("%s has no source loopback", sourceName)
			}
			if target.Loopback.IPv4 == "" {
				return fmt.Errorf("%s has no destination loopback", targetName)
			}
			_, _, err := interoplab.Wait(ctx, liveWait(sourceName+" ping "+targetName), func(probe context.Context) (string, error) {
				return c.liveCommand(probe, directory, "docker", "exec", source.Hostname, "ping", "-I", address(source.Loopback.IPv4), "-c", "1", "-W", "1", address(target.Loopback.IPv4))
			}, func(string) bool { return true })
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Checker) proveNode(ctx context.Context, directory, name string, node liveNode) error {
	bgp, ospf, isis, bfd := 0, 0, 0, 0
	for _, peer := range node.BGP.Neighbors {
		if peer.IPv4 != "" {
			bgp++
		}
	}
	for _, intf := range node.Interfaces {
		if intf.OSPF != nil {
			if !intf.OSPF.Passive {
				ospf++
				if intf.OSPF.BFD {
					bfd++
				}
			}
		}
		if intf.ISIS != nil {
			if !intf.ISIS.Passive {
				isis++
			}
		}
	}
	for _, proof := range []struct {
		module, command, state string
		count                  int
	}{
		{"bgp", "show bgp", "established", bgp},
		{"ospf", "show ospf neighbor", "full", ospf},
		{"isis", "show isis neighbor", "up", isis},
		{"bfd", "show bfd sessions", "up", bfd},
	} {
		if !slices.Contains(node.Module, proof.module) {
			continue
		}
		if proof.count == 0 {
			return fmt.Errorf("%s declares %s but has no adjacency to prove", name, proof.module)
		}
		_, _, err := interoplab.Wait(ctx, liveWait(name+" "+proof.module), func(probe context.Context) ([]stateRow, error) {
			body, err := c.liveCommand(probe, directory, "docker", "exec", node.Hostname, "ze", "cli", "-c", proof.command+" | json compact")
			if err != nil {
				return nil, err
			}
			if proof.module == "bgp" {
				var response struct {
					Peers []stateRow `json:"peers"`
				}
				err := json.Unmarshal([]byte(body), &response)
				return response.Peers, err
			}
			var rows []stateRow
			err = json.Unmarshal([]byte(body), &rows)
			return rows, err
		}, func(rows []stateRow) bool { return statesReady(rows, proof.state, proof.count) })
		if err != nil {
			return err
		}
	}
	if name != "r1" {
		return nil
	}
	nextHop := ""
	for _, route := range node.Routing.Static {
		if route.IPv4 == "10.42.0.0/24" {
			nextHop = route.Nexthop.IPv4
		}
	}
	if nextHop == "" {
		return errors.New("r1 static next hop is absent from the live topology")
	}
	_, _, err := interoplab.Wait(ctx, liveWait(name+" static routes"), func(probe context.Context) ([]routeRow, error) {
		body, err := c.liveCommand(probe, directory, "docker", "exec", node.Hostname, "ip", "-j", "route", "show")
		if err != nil {
			return nil, err
		}
		var rows []routeRow
		err = json.Unmarshal([]byte(body), &rows)
		return rows, err
	}, func(rows []routeRow) bool { return staticReady(rows, nextHop) })
	return err
}

func liveWait(description string) interoplab.WaitOptions {
	return interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: 250 * time.Millisecond, Description: description}
}

func address(prefix string) string { before, _, _ := strings.Cut(prefix, "/"); return before }

func (c *Checker) liveCommand(ctx context.Context, directory string, argv ...string) (string, error) {
	fmt.Fprintln(os.Stderr, "+", strings.Join(argv, " ")) //nolint:errcheck // CLI progress
	result := c.Execute(Command{Context: ctx, Argv: argv, Env: c.Env, Dir: directory})
	if result.Err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(argv, " "), result.Err, result.Stderr)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("%s exited %d: %s%s", strings.Join(argv, " "), result.Code, result.Stdout, result.Stderr)
	}
	return result.Stdout, nil
}
