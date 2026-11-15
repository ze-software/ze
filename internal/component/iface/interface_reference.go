// Design: docs/architecture/iface/logical-name-resolution.md -- a mirror destination names an interface the config declares
// Related: register_interface_reference.go -- the registration of this source
// Related: config_mirror.go -- mirrorDestination, which binds the name this source accepted

package iface

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConfiguredInterfaceReference names the ze:reference source of a leaf whose
// value must be an interface the interface section of the same config
// declares. Commit refuses any other name, so a mistyped name, or one whose
// entry the same commit deletes, never reaches the apply.
const ConfiguredInterfaceReference = "configured-interface"

// configuredInterfaceKinds names the lists directly under the interface
// container whose entries are interfaces. It is a copy of a fact the schema
// holds, and TestConfiguredInterfaceKindsAreTheSchemaLists compares the two, so
// a kind the schema gains fails that test until it is named here.
var configuredInterfaceKinds = []string{"ethernet", "dummy", "veth", "bridge", "tunnel", "wireguard", "xfrm", "pppoe-client"}

// configuredInterfaceChoicesMax bounds how many declared names a refusal
// lists, so a config with hundreds of interfaces keeps a readable message.
const configuredInterfaceChoicesMax = 8

// configuredInterfaceReference answers the interface names one config
// declares: the entry of every interface kind, the peer end of every veth, and
// every VLAN unit as <entry>.<vlan-id>, the kernel name unitOSName gives it.
// The loopback container is not a list entry, and it is not offered.
//
// The answer reads the config alone, never the kernel, so `ze config validate`
// gives the verdict the daemon gives. A declared name whose device is absent is
// accepted here, and the apply decides what to do without it.
//
// Safe for concurrent use: it only reads root.
func configuredInterfaceReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	names := configuredInterfaceNames(root)
	return yang.ReferenceAnswer{
		Names: names,
		Check: func(value string) error { return checkConfiguredInterface(value, names) },
	}
}

// configuredInterfaceNames returns the sorted, deduplicated interface names
// the interface section of root declares.
func configuredInterfaceNames(root map[string]any) []string {
	section, _ := root[configRootInterface].(map[string]any) // no interface section declares no interface
	names := make([]string, 0, len(configuredInterfaceKinds))
	for _, kind := range configuredInterfaceKinds {
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
	return slices.Compact(names)
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
