// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP startup prerequisites
// Related: register.go -- installs these pure candidate readers

package pppoevpp

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	vppdataplane "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp"
	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const pppoeVPPPluginCode = "doctor-vpp-pppoe-plugin"

func registerStartup() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: pppoeVPPPluginCode, Title: "VPP PPPoE plugin APIs unavailable",
		Description: "PPPoE forwarding requires the running VPP to load pppoe_plugin.so with Ze's retained patches. The check reads the live API message table and compares every generated PPPoE name and CRC. A missing or stock API is incompatible; an unavailable socket remains unknown. Build the patched VPP artifact with ./le build vpp run, install that artifact, and check again.",
		Examples:    []string{"ze doctor", "ze show diagnostic " + pppoeVPPPluginCode}, HostVerdict: true,
	}); err != nil {
		panic("BUG: cannot register VPP PPPoE diagnostic: " + err.Error())
	}
	if err := config.RegisterTreeValidator("pppoe-vpp-control-plane", validateControlPlane); err != nil {
		panic("BUG: cannot register VPP PPPoE prerequisite validation: " + err.Error())
	}
	if err := vppcomp.RegisterStartupFeature("pppoe", vppcomp.StartupFeature{
		Plugin: "pppoe_plugin.so", Section: "pppoe", Flags: []string{"enable-pass-nd-and-dhcpv6"},
		ConfigReads: []string{"interface", "pppoe"}, Enabled: startupRequired,
		Runtime: &vppcomp.StartupRuntime{Messages: pppoeapi.AllMessages(), Code: pppoeVPPPluginCode,
			Fix: "ze show diagnostic " + pppoeVPPPluginCode},
	}); err != nil {
		panic("BUG: cannot register VPP PPPoE startup requirements: " + err.Error())
	}
}

func validateControlPlane(tree *config.Tree) error {
	_, err := startupRequired(tree)
	return err
}

// startupRequired is a pure candidate reader shared by verify and startup.
// Registration does not open a connector, resolve an interface or start VPP.
func startupRequired(tree *config.Tree) (bool, error) {
	if tree == nil {
		return false, nil
	}
	access := tree.GetContainer("pppoe")
	if access == nil {
		return false, nil
	}
	interfaces := access.GetList("interface")
	if len(interfaces) == 0 {
		return false, nil
	}
	backend, err := pppoe.ConfiguredSessionBackendName(tree)
	if err != nil {
		return false, err
	}
	if backend != vppdataplane.Name {
		return false, nil
	}
	paths := make([]string, 0, len(interfaces))
	for name := range interfaces {
		paths = append(paths, "/pppoe/interface/"+name)
	}
	slices.Sort(paths)
	subject := strings.Join(paths, ", ")
	enabled, err := prerequisiteBool(tree, "vpp/enabled")
	if err != nil {
		return false, err
	}
	if !enabled {
		return false, opererr.New("PPPoE access "+subject+" requires VPP to be enabled",
			"The selected VPP provider cannot create subscriber forwarding interfaces while VPP is disabled.", "set vpp enabled true")
	}
	lcp, err := prerequisiteBool(tree, "vpp/lcp/enabled")
	if err != nil {
		return false, err
	}
	if !lcp {
		return false, opererr.New("PPPoE access "+subject+" requires VPP Linux Control Plane pairs",
			"Discovery and PPP control frames need the Linux control device paired with each VPP access interface.", "set vpp lcp enabled true")
	}
	return true, nil
}

func prerequisiteBool(tree *config.Tree, path string) (bool, error) {
	parent, leafName, hasParent := strings.CutLast(path, "/")
	if !hasParent {
		return false, prerequisiteSchemaError(errors.New("prerequisite path has no parent"))
	}
	if container := tree.GetContainerPath(parent); container != nil {
		if value, present := container.Get(leafName); present {
			return parsePrerequisiteBool(path, value)
		}
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return false, prerequisiteSchemaError(err)
	}
	node, err := schema.Lookup(path)
	if err != nil {
		return false, prerequisiteSchemaError(err)
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		return false, prerequisiteSchemaError(fmt.Errorf("%s is not a leaf", path))
	}
	return parsePrerequisiteBool(path, leaf.Default)
}

func parsePrerequisiteBool(path, value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, prerequisiteSchemaError(fmt.Errorf("%s has invalid boolean value %q", path, value))
	}
}

func prerequisiteSchemaError(cause error) error {
	return opererr.Wrap(cause, "VPP PPPoE prerequisites cannot be checked",
		"The candidate and its schema must declare whether VPP and Linux Control Plane pairs are enabled.", "ze doctor")
}
