// Design: docs/guide/pppoe.md -- access interfaces require a supported session backend.
// Related: yang/ze-pppoe-conf.yang -- the access-interface support declaration.

package pppoe

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
)

// validateBackendGate uses the interface backend selected by the original full
// tree and the installed provider registry. Global PPPoE settings remain usable
// when no access interface is configured. No factory or host probe runs here.
func validateBackendGate(tree *config.Tree) error {
	if tree == nil {
		return nil
	}
	access := tree.GetContainer(SubsystemName)
	if access == nil {
		return nil
	}
	if len(access.GetList("interface")) == 0 {
		return nil
	}
	backend, preferred, err := sessionBackendSelection(tree)
	if err != nil {
		return err
	}
	if _, err := dataplane.LookupSessionDataplane(backend); err == nil {
		return nil
	}
	providers := dataplane.SessionDataplaneNames()
	fix := "ze doctor"
	if index := slices.Index(providers, preferred); index >= 0 {
		providers[0], providers[index] = providers[index], providers[0]
		fix = "set interface backend " + preferred
	}
	supported := strings.Join(providers, ", ")
	if len(providers) == 0 {
		supported = "none installed"
	}
	paths := make([]string, 0, len(access.GetList("interface")))
	for name := range access.GetList("interface") {
		paths = append(paths, "/pppoe/interface/"+name)
	}
	slices.Sort(paths)
	why := "This build has no PPPoE session data plane for the selected backend, so subscriber sessions cannot forward traffic."
	if backend == "vpp" {
		why += " Build Ze with ze_vpp to include the VPP PPPoE provider."
	}
	return opererr.New(fmt.Sprintf("%s cannot use interface backend %q (supported: %s)", strings.Join(paths, ", "), backend, supported), why, fix)
}

// ConfiguredSessionBackendName reads the original candidate's interface
// backend, or its declared schema default. It never reads the active backend
// or opens a session. Safe for concurrent use with an immutable candidate.
func ConfiguredSessionBackendName(tree *config.Tree) (string, error) {
	backend, _, err := sessionBackendSelection(tree)
	return backend, err
}

func sessionBackendSelection(tree *config.Tree) (string, string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return "", "", backendSchemaError(err)
	}
	node, err := schema.Lookup("interface/backend")
	if err != nil {
		return "", "", backendSchemaError(err)
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		return "", "", backendSchemaError(errors.New("interface/backend is not a schema leaf"))
	}
	if leaf.Default == "" {
		return "", "", backendSchemaError(errors.New("interface/backend has no schema default"))
	}
	backend := leaf.Default
	if tree != nil {
		if interfaces := tree.GetContainer("interface"); interfaces != nil {
			if explicit, present := interfaces.Get("backend"); present {
				backend = explicit
			}
		}
	}
	return backend, leaf.Default, nil
}

// backendSchemaError refuses a build without the declarations needed to check
// the candidate, rather than silently accepting a backend it cannot verify.
func backendSchemaError(cause error) error {
	return opererr.Wrap(cause, "PPPoE backend configuration schema is unavailable",
		"The candidate cannot be checked for PPPoE session support",
		"Install a Ze build containing the interface and PPPoE YANG modules, then restart Ze")
}
