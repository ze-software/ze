package iface

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestConfiguredInterfaceReferenceNames checks which names the
// configured-interface source offers. Method: build one interface section
// holding an entry of several kinds, a veth with a peer and an ethernet with a
// VLAN unit and a plain unit, and require the exact sorted set: every entry,
// the veth peer, the VLAN unit as <entry>.<vlan-id>, nothing for the plain unit
// or the loopback container, and the empty set for a config with no interface
// section.
func TestConfiguredInterfaceReferenceNames(t *testing.T) {
	root := map[string]any{
		"interface": map[string]any{
			"backend": "netlink",
			"ethernet": map[string]any{
				"eth0": map[string]any{
					"unit": map[string]any{
						"0":   map[string]any{},
						"100": map[string]any{"vlan-id": "100"},
					},
				},
			},
			"dummy":    map[string]any{"capture0": map[string]any{}},
			"veth":     map[string]any{"v0": map[string]any{"peer": "v0p"}},
			"bridge":   map[string]any{"br0": map[string]any{}},
			"loopback": map[string]any{"unit": map[string]any{"0": map[string]any{}}},
		},
	}
	got := configuredInterfaceReference(root, nil).Names
	want := []string{"br0", "capture0", "eth0", "eth0.100", "v0", "v0p"}
	if !slices.Equal(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
	if got := configuredInterfaceReference(map[string]any{}, nil).Names; len(got) != 0 {
		t.Errorf("a config with no interface section offered %v, want none", got)
	}
}

// TestConfiguredInterfaceReferenceCheck checks the commit judge of the
// source. Method: accept every offered name, then refuse an undeclared name,
// an undeclared VLAN unit and a name in a config that declares no interface,
// and require each refusal to name the value, the fix and, where any exist,
// the declared choices.
func TestConfiguredInterfaceReferenceCheck(t *testing.T) {
	root := map[string]any{
		"interface": map[string]any{
			"ethernet": map[string]any{"eth0": map[string]any{}, "eth1": map[string]any{}},
		},
	}
	answer := configuredInterfaceReference(root, nil)
	for _, name := range answer.Names {
		if err := answer.Check(name); err != nil {
			t.Errorf("Check(%q) refused an offered name: %v", name, err)
		}
	}

	cases := []struct {
		name  string
		root  map[string]any
		value string
		want  []string
	}{
		{
			name:  "undeclared interface",
			root:  root,
			value: "eth9",
			want:  []string{`interface "eth9" is not configured`, "set interface ethernet eth9", `name one of "eth0 eth1"`},
		},
		{
			name:  "undeclared VLAN unit",
			root:  root,
			value: "eth0.200",
			want:  []string{`interface "eth0.200" is not configured`, "set interface ethernet eth0 unit 200 vlan-id 200"},
		},
		{
			name:  "no interface declared",
			root:  map[string]any{},
			value: "eth9",
			want:  []string{`interface "eth9" is not configured`, "this config declares no interface"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := configuredInterfaceReference(tc.root, nil).Check(tc.value)
			if err == nil {
				t.Fatalf("Check(%q) accepted a name the config does not declare", tc.value)
			}
			for _, part := range tc.want {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("Check(%q) = %q, want it to contain %q", tc.value, err.Error(), part)
				}
			}
		})
	}
}

// TestConfiguredInterfaceKindsAreTheSchemaLists keeps configuredInterfaceKinds
// equal to the schema it copies. Method: read the lists directly under the
// interface container of the compiled schema and require the same set, so a
// kind the schema gains, or loses, fails here rather than being refused as a
// mirror target.
func TestConfiguredInterfaceKindsAreTheSchemaLists(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	node, err := schema.Lookup(configRootInterface)
	if err != nil {
		t.Fatalf("lookup %s: %v", configRootInterface, err)
	}
	container, ok := node.(*config.ContainerNode)
	if !ok {
		t.Fatalf("%s is %T, want a container", configRootInterface, node)
	}
	var lists []string
	for _, child := range container.Children() {
		if container.Get(child).Kind() == config.NodeList {
			lists = append(lists, child)
		}
	}
	slices.Sort(lists)
	kinds := slices.Sorted(slices.Values(configuredInterfaceKinds))
	if !slices.Equal(lists, kinds) {
		t.Errorf("schema lists under %s = %v, configuredInterfaceKinds = %v", configRootInterface, lists, kinds)
	}
}
