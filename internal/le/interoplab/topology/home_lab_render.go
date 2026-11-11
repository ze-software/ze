// Design: docs/architecture/testing/interop.md -- the home-CPE lab.
// Related: register_home_lab.go -- the scenarios that run the rendered CPE.
package topology

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/labconfig/homecpe"
)

const (
	cpeSourceFile         = "vyos.conf"
	cpeRecordFile         = "conversion-record.md"
	renderedCPEConfig     = homecpe.ConvertedFile
	renderedCPEResolvConf = "resolv.conf"
	systemResolvConf      = "/etc/resolv.conf"
)

// cpeResolvConf names the converted configuration's static server, with one
// one-second attempt, so mgmt-tunnel measures Ze's backoff, not resolver retries.
const cpeResolvConf = "nameserver " + cpeStaticNameServer + "\noptions timeout:1 attempts:1\n"

// prepareHomeCPE answers the prepare step of a scenario whose CPE holds a key
// of algorithm: it renders the CPE configuration, with the scenario's
// additions inserted, and the issued files into directory, for the nodes that
// mount them (node.prepared). A scenario with no additions runs the
// conversion as rendered.
func prepareHomeCPE(algorithm homecpe.KeyAlgorithm, additions ...cpeAddition) func(root, directory string) error {
	return func(root, directory string) error {
		return writeHomeCPE(root, directory, algorithm, additions)
	}
}

// cpeAddition is lab-only text one scenario inserts into the rendered CPE
// configuration, for a case the VyOS source cannot state, so ze-cpe.conf
// stays the conversion and its record stays true. text goes immediately
// before anchor, which the rendered configuration MUST hold exactly once.
type cpeAddition struct {
	anchor string
	text   string
}

// addToCPEConfig inserts each addition before its anchor, in order. It
// refuses an anchor the configuration holds no time or several times, because
// the addition would then land nowhere or in a place nobody chose.
func addToCPEConfig(config string, additions []cpeAddition) (string, error) {
	for _, addition := range additions {
		count := strings.Count(config, addition.anchor)
		if count != 1 {
			return "", fmt.Errorf("a lab addition to the CPE configuration is anchored on %q, which %s/%s holds %d times, want once; "+
				"re-anchor the addition on text the converted file holds once", addition.anchor, homecpe.Directory, homecpe.ConvertedFile, count)
		}
		at := strings.Index(config, addition.anchor)
		config = config[:at] + addition.text + config[at:]
	}
	return config, nil
}

// writeHomeCPE is prepareHomeCPE's step for one run.
func writeHomeCPE(root, directory string, algorithm homecpe.KeyAlgorithm, additions []cpeAddition) error {
	rendered, err := homecpe.Render(root, time.Now(), algorithm)
	if err != nil {
		return err
	}
	config, err := addToCPEConfig(rendered.Config, additions)
	if err != nil {
		return err
	}
	files := map[string][]byte{renderedCPEConfig: []byte(config), renderedCPEResolvConf: []byte(cpeResolvConf)}
	maps.Copy(files, rendered.Files)
	for name, content := range files {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // the lab containers read these files as another user
			return fmt.Errorf("create the directory of %s: %w", path, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // the lab containers read these files as another user
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// errNoPreparedPeer says a node declares prepared files but the plan holds no
// peer of that name, a declaration defect.
var errNoPreparedPeer = errors.New("the plan holds no peer for a node with prepared files")

// preparePlan runs a scenario's prepare step into a fresh directory under the
// session scratch and mounts each node's prepared files from it. A scenario
// with no prepare step is left as it is.
func preparePlan(root, suffix string, declared *scenario, plan *interoplab.ScenarioPlan) error {
	if declared.prepare == nil {
		return nil
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return fmt.Errorf("resolve the session scratch for %s: %w", plan.Source.Name, err)
	}
	directory := filepath.Join(root, paths.Scratch, "interop-topology", plan.Source.Name+"-"+suffix)
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("clear the prepared directory %s: %w", directory, err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil { //nolint:gosec // the lab containers read these files as another user
		return fmt.Errorf("create the prepared directory %s: %w", directory, err)
	}
	if err := declared.prepare(root, directory); err != nil {
		return fmt.Errorf("prepare %s: %w", plan.Source.Name, err)
	}
	for index := range declared.nodes {
		if err := mountPrepared(plan, &declared.nodes[index], directory); err != nil {
			return err
		}
	}
	return nil
}

// mountPrepared adds the prepared files of one node to its peer's mounts.
func mountPrepared(plan *interoplab.ScenarioPlan, declared *node, directory string) error {
	if len(declared.prepared) == 0 {
		return nil
	}
	for index := range plan.Peers {
		if plan.Peers[index].Name != declared.name {
			continue
		}
		for _, file := range declared.prepared {
			plan.Peers[index].Mounts = append(plan.Peers[index].Mounts, interoplab.Mount{
				Source: filepath.Join(directory, filepath.FromSlash(file.source)), Target: file.target, ReadOnly: true})
		}
		return nil
	}
	return fmt.Errorf("%w: %s in %s", errNoPreparedPeer, declared.name, plan.Source.Name)
}
