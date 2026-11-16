// Design: docs/architecture/iface/logical-name-resolution.md -- a leaf that configures an interface Ze owns names one the config declares
// Related: register_interface_reference.go -- the registration of this source
// Related: interface_targets.go -- interfaceTargets, which binds the name this source accepted

package iface

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConfiguredInterfaceReference names the ze:reference source of a leaf whose
// value must be an interface the interface section of the same config
// declares. Commit refuses any other name, so a mistyped name, or one whose
// entry the same commit deletes, never reaches the apply.
const ConfiguredInterfaceReference = "configured-interface"

// configuredInterfaceChoicesMax bounds how many declared names a refusal
// lists, so a config with hundreds of interfaces keeps a readable message.
const configuredInterfaceChoicesMax = 8

// configuredInterfaceReference answers the interface names one config
// declares: the entry of every supported interface kind, the peer end of a
// veth, and every VLAN unit as <entry>.<vlan-id>, the name unitOSName gives it.
// The loopback container is not a list entry, and it is not offered.
//
// The answer reads the config and compiled schema, never the kernel, so
// validation and completion use the same supported kinds. A declared name
// whose device is absent is accepted here; apply decides what to do with it.
//
// Safe for concurrent use: it reads root and the shared immutable schema.
func configuredInterfaceReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	names, err := configuredInterfaceNames(root)
	if err != nil {
		return yang.ReferenceAnswer{Check: func(value string) error {
			return opererr.Wrap(err, fmt.Sprintf("Interface reference %q could not be checked", value),
				"The compiled interface schema is unavailable, so no interface reference can be verified.",
				"ze doctor")
		}}
	}
	return yang.ReferenceAnswer{
		Names: names,
		Check: func(value string) error { return checkConfiguredInterface(value, names) },
	}
}

// configuredInterfaceKinds derives the supported lists from the current
// compiled schema. This runs when a reference is read, after schema loading:
// source registration never calls it. The schema cache tracks module changes,
// so a newly registered kind requires no second list and is never frozen out.
func configuredInterfaceKinds() ([]string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, err
	}
	node, err := schema.Lookup(configRootInterface)
	if err != nil {
		return nil, err
	}
	container, ok := node.(*config.ContainerNode)
	if !ok {
		return nil, fmt.Errorf("interface schema node is %T, want a container", node)
	}
	children := container.Children()
	kinds := make([]string, 0, len(children))
	for _, child := range children {
		if container.Get(child).Kind() == config.NodeList {
			kinds = append(kinds, child)
		}
	}
	slices.Sort(kinds)
	return kinds, nil
}

// configuredInterfaceNames returns the sorted, deduplicated interface names
// the interface section of root declares under the current schema's lists.
func configuredInterfaceNames(root map[string]any) ([]string, error) {
	kinds, err := configuredInterfaceKinds()
	if err != nil {
		return nil, err
	}
	section, _ := root[configRootInterface].(map[string]any) // no interface section declares no interface
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		entries, _ := section[kind].(map[string]any) // an absent kind declares no entry
		for name, value := range entries {
			names = append(names, name)
			entry, _ := value.(map[string]any) // an empty entry has no peer and no unit
			if kind == "veth" {
				names = appendVethPeerName(names, entry)
			}
			names = appendVLANUnitNames(names, name, entry)
		}
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

// appendVethPeerName appends the peer end of a veth entry, a device ze creates
// beside the entry's own.
func appendVethPeerName(names []string, entry map[string]any) []string {
	peer, _ := entry["peer"].(string) // a veth with no peer leaf names no second device
	if peer == "" {
		return names
	}
	return append(names, peer)
}

// appendVLANUnitNames appends the kernel name of every unit of entry that
// carries a vlan-id. A unit without one is its parent's own device, whose name
// is already in names.
func appendVLANUnitNames(names []string, parent string, entry map[string]any) []string {
	units, _ := entry["unit"].(map[string]any) // an entry with no unit declares no VLAN
	for _, value := range units {
		unit, _ := value.(map[string]any)     // an empty unit carries no vlan-id
		vlanID, _ := unit["vlan-id"].(string) // a unit with no vlan-id is its parent's device
		if vlanID == "" {
			continue
		}
		names = append(names, parent+"."+vlanID)
	}
	return names
}

// checkConfiguredInterface accepts value when names holds it, and otherwise
// says which name is missing, why ze cannot use it, and how to fix it. It
// accepts every name in names, as ReferenceAnswer.Check requires.
func checkConfiguredInterface(value string, names []string) error {
	if slices.Contains(names, value) {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("interface ").Quoted(value).Str(" is not configured, so ze has no device to use for it: ")
	if dot := strings.LastIndexByte(value, '.'); dot > 0 {
		// A dotted name is a VLAN unit, which its parent entry declares.
		parent, vlanID := value[:dot], value[dot+1:]
		tb.Str("declare the VLAN with set interface ethernet ").Str(parent).Str(" unit ").Str(vlanID).Str(" vlan-id ").Str(vlanID)
	} else {
		tb.Str("declare it with set interface ethernet ").Str(value)
	}
	if len(names) == 0 {
		tb.Str(", because this config declares no interface")
		return errors.New(tb.String())
	}
	shown := names[:min(len(names), configuredInterfaceChoicesMax)]
	tb.Str(", or name one of ").Quoted(strings.Join(shown, " "))
	if len(names) > len(shown) {
		tb.Str(" (").Int(int64(len(names) - len(shown))).Str(" more configured)")
	}
	return errors.New(tb.String())
}
