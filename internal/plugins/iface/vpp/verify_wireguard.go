// Design: docs/architecture/core-design.md -- VPP WireGuard plugin validation across interface and vpp config roots
// Related: register_wireguard_validator.go -- registers verifyWireguardPluginTree as an original-tree validator
// Related: doctor.go -- checkVPPWireguardPlugin shares checkWireguardPluginTree for ze doctor

package ifacevpp

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// wireguardTreeValidatorName is the original-tree validator name registered
	// by this package so commit, reload, boot and `ze config validate` all judge
	// interface wireguard against vpp plugins wireguard over the whole tree.
	wireguardTreeValidatorName = "iface-vpp-wireguard-plugin"

	// fixEnableVPPWireguardPlugin is the operator command that enables VPP's
	// wireguard_plugin.so in startup.conf.
	fixEnableVPPWireguardPlugin = "set vpp plugins wireguard true" //nolint:gosec // operator CLI fix command, not a credential

	wireguardNodeName          = "wireguard"
	wireguardSchemaDefaultWhy  = "ze reads the interface backend default from the YANG schema rather than assuming one"
	wireguardSchemaValidateFix = "ze config validate"
)

// verifyWireguardPluginTree refuses a candidate configuration that defines one
// or more WireGuard interfaces under interface backend vpp while
// vpp.plugins.wireguard is not true. Registered as an original-tree validator,
// so commit, reload, boot and `ze config validate` all run it over the whole
// tree regardless of which root changed.
func verifyWireguardPluginTree(tree *config.Tree) error {
	ifaces, err := checkWireguardPluginTree(tree)
	if err != nil {
		return err
	}
	if len(ifaces) == 0 {
		return nil
	}
	return opererr.New(
		wireguardPluginMissingProblem(ifaces),
		"VPP leaves wireguard_plugin.so disabled unless vpp plugins wireguard is true, so WireGuard interfaces cannot be programmed under interface backend vpp",
		fixEnableVPPWireguardPlugin,
	)
}

// checkWireguardPluginTree inspects the whole candidate tree and returns the
// sorted names of active WireGuard interfaces that require the VPP WireGuard
// plugin when interface backend resolves to vpp and vpp.plugins.wireguard is
// not true. Both verifyWireguardPluginTree and checkVPPWireguardPlugin call
// this function so the validator and `ze doctor` never drift.
//
// Safe for concurrent use; does not modify tree.
func checkWireguardPluginTree(tree *config.Tree) ([]string, error) {
	// refuse-by-default: guards verifyWireguardPluginTree -- a nil tree carries no configuration to check
	if tree == nil {
		return nil, nil
	}
	ifaceTree := tree.GetContainer("interface")
	// refuse-by-default: guards verifyWireguardPluginTree -- no interface container means no WireGuard interface
	if ifaceTree == nil {
		return nil, nil
	}
	// refuse-by-default: guards verifyWireguardPluginTree -- an inactive interface container configures no WireGuard interface
	if ifaceTree.IsInactive() {
		return nil, nil
	}

	ifaces := activeWireguardInterfaces(ifaceTree)
	if len(ifaces) == 0 {
		return nil, nil
	}

	backend, err := resolveInterfaceBackend(ifaceTree)
	if err != nil {
		return nil, err
	}
	if backend != backendVPP {
		return nil, nil
	}
	if wireguardPluginEnabled(tree) {
		return nil, nil
	}
	return ifaces, nil
}

// activeWireguardInterfaces returns the sorted names of active entries under
// interface wireguard.
func activeWireguardInterfaces(ifaceTree *config.Tree) []string {
	wgMap := ifaceTree.GetList(wireguardNodeName)
	if len(wgMap) == 0 {
		return nil
	}
	names := make([]string, 0, len(wgMap))
	for name, entry := range wgMap {
		if entry != nil && entry.IsInactive() {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// resolveInterfaceBackend returns the configured interface backend leaf, or
// reads the leaf's default from the loaded YANG schema when omitted. It never
// hardcodes a fallback backend literal.
func resolveInterfaceBackend(ifaceTree *config.Tree) (string, error) {
	if b, ok := ifaceTree.Get("backend"); ok {
		if !ifaceTree.IsLeafInactive("backend") && b != "" {
			return b, nil
		}
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return "", opererr.Wrap(err,
			"interface backend default could not be read: YANG schema unavailable",
			wireguardSchemaDefaultWhy,
			wireguardSchemaValidateFix,
		)
	}
	node, err := schema.Lookup("interface/backend")
	if err != nil {
		return "", opererr.Wrap(err,
			"interface backend default could not be read from the YANG schema",
			wireguardSchemaDefaultWhy,
			wireguardSchemaValidateFix,
		)
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		return "", opererr.New(
			"interface/backend in the YANG schema is not a leaf",
			wireguardSchemaDefaultWhy,
			wireguardSchemaValidateFix,
		)
	}
	if leaf.Default == "" {
		return "", opererr.New(
			"interface/backend has no default in the YANG schema",
			wireguardSchemaDefaultWhy,
			wireguardSchemaValidateFix,
		)
	}
	return leaf.Default, nil
}

// wireguardPluginEnabled reports whether vpp.plugins.wireguard is active and
// explicitly set to "true".
func wireguardPluginEnabled(tree *config.Tree) bool {
	vppTree := tree.GetContainer(backendVPP)
	if vppTree == nil {
		return false
	}
	if vppTree.IsInactive() {
		return false
	}
	plugins := vppTree.GetContainer("plugins")
	if plugins == nil {
		return false
	}
	if plugins.IsInactive() {
		return false
	}
	if plugins.IsLeafInactive(wireguardNodeName) {
		return false
	}
	v, ok := plugins.Get(wireguardNodeName)
	return ok && v == "true"
}

// wireguardPluginMissingProblem formats the operator-facing problem line naming
// every WireGuard interface that requires vpp plugins wireguard.
func wireguardPluginMissingProblem(ifaces []string) string {
	var tb textbuf.Buffer
	if len(ifaces) == 1 {
		return tb.Str("interface wireguard ").Str(ifaces[0]).
			Str(" requires vpp plugins wireguard under interface backend vpp").String()
	}
	return tb.Str("interface wireguard (").Join(ifaces, ", ").
		Str(") require vpp plugins wireguard under interface backend vpp").String()
}
