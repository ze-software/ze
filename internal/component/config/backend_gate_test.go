package config

import (
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// VALIDATES: ValidateBackendFeatures returns no error when the active
//
//	backend is listed by the annotated list node.
//
// PREVENTS: False positives on a supported combination.
func TestValidateBackendFeatures_SupportedAccepts(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{
		"bridge": {"netlink"},
	})
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "netlink",
			"bridge": map[string]any{
				"br0": map[string]any{"name": "br0"},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "netlink", "/interface/backend")
	assert.Empty(t, errs, "netlink backend should accept bridge")
}

// VALIDATES: ValidateBackendFeatures emits one error per YANG path whose
//
//	ze:backend annotation excludes the active backend.
//
// PREVENTS: Silent acceptance of an unsupported feature reaching Apply.
func TestValidateBackendFeatures_UnsupportedRejects(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{
		"bridge": {"netlink"},
	})
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"bridge": map[string]any{
				"br0": map[string]any{"name": "br0"},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1, "expected exactly one rejection")
	msg := errs[0].Error()
	assert.Contains(t, msg, "/interface/bridge", "error must name the YANG path")
	assert.Contains(t, msg, `"vpp"`, "error must name the active backend")
	assert.Contains(t, msg, "netlink", "error must list supporting backends")
}

// VALIDATES: Walker aggregates all unsupported features in a single pass.
// PREVENTS: First-error-wins masking other mismatches (user has to fix
//
//	one at a time).
func TestValidateBackendFeatures_AggregatesMultiple(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{
		"bridge":    {"netlink"},
		"tunnel":    {"netlink"},
		"wireguard": {"netlink"},
	})
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"bridge": map[string]any{
				"br0": map[string]any{"name": "br0"},
			},
			"tunnel": map[string]any{
				"t0": map[string]any{"name": "t0"},
			},
			"wireguard": map[string]any{
				"wg0": map[string]any{"name": "wg0"},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 3, "expected three rejections, got: %v", errs)

	paths := make(map[string]bool, len(errs))
	for _, e := range errs {
		for _, p := range []string{"/interface/bridge", "/interface/tunnel", "/interface/wireguard"} {
			if strings.Contains(e.Error(), p) {
				paths[p] = true
			}
		}
	}
	assert.True(t, paths["/interface/bridge"], "bridge path missing from errors")
	assert.True(t, paths["/interface/tunnel"], "tunnel path missing from errors")
	assert.True(t, paths["/interface/wireguard"], "wireguard path missing from errors")
}

// VALIDATES: Nodes without a ze:backend annotation accept every backend.
// PREVENTS: Accidental implicit-restriction when annotations are missing.
func TestValidateBackendFeatures_UnrestrictedAccepts(t *testing.T) {
	// ethernet has no annotation -> unrestricted
	schema := buildSyntheticIfaceSchema(map[string][]string{})
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"ethernet": map[string]any{
				"eth0": map[string]any{"name": "eth0"},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	assert.Empty(t, errs, "unrestricted ethernet should accept vpp")
}

// VALIDATES: A more specific (descendant) annotation accepting the active
//
//	backend suppresses an outer annotation that would reject.
//
// PREVENTS: Outer-only wins, which would make per-case override impossible.
func TestValidateBackendFeatures_ChoiceCaseOverride(t *testing.T) {
	schema := NewSchema()
	iface := Container(
		Field("backend", Leaf(TypeString)),
		Field("tunnel", &ListNode{
			KeyType:  TypeString,
			KeyName:  "name",
			Backend:  []string{"netlink"},
			children: map[string]Node{"name": Leaf(TypeString), "vxlan": &ContainerNode{children: map[string]Node{"id": Leaf(TypeUint32)}, order: []string{"id"}, Backend: []string{"netlink", "vpp"}}},
			order:    []string{"name", "vxlan"},
		}),
	)
	schema.Define("interface", iface)
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"tunnel": map[string]any{
				"t0": map[string]any{
					"name":  "t0",
					"vxlan": map[string]any{"id": "42"},
				},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	assert.Empty(t, errs, "per-case override should suppress outer rejection: %v", errs)
}

// VALIDATES: An empty active backend produces a single clear error.
// PREVENTS: Silent acceptance when the user forgot to configure a backend.
func TestValidateBackendFeatures_EmptyActiveBackend(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{"bridge": {"netlink"}})
	tree := map[string]any{
		"interface": map[string]any{
			"bridge": map[string]any{"br0": map[string]any{"name": "br0"}},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "", "/interface/backend")
	require.Len(t, errs, 1, "expected single error for empty backend")
	assert.Contains(t, errs[0].Error(), "/interface/backend", "must point at the backend leaf path")
}

// VALIDATES: When componentRoot is absent from the tree, walker returns nil.
// PREVENTS: Spurious errors when the user has no config for a given component.
func TestValidateBackendFeatures_AbsentComponentRoot(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{"bridge": {"netlink"}})
	tree := map[string]any{
		"bgp": map[string]any{},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	assert.Empty(t, errs, "absent component root should produce no errors")
}

// VALIDATES: Duplicate backend names in ze:backend are de-duplicated by
//
//	the schema reader (getBackendExtension).
//
// PREVENTS: False positives from a user seeing the same name twice.
func TestGetBackendExtension_Deduplicates(t *testing.T) {
	// Synthetic schema: build a ListNode directly, simulating schema-builder
	// output after getBackendExtension deduplication.
	list := &ListNode{
		KeyType:  TypeString,
		KeyName:  "name",
		Backend:  []string{"netlink"}, // dedup already applied at build
		children: map[string]Node{"name": Leaf(TypeString)},
		order:    []string{"name"},
	}
	schema := NewSchema()
	schema.Define("interface", Container(
		Field("backend", Leaf(TypeString)),
		Field("bridge", list),
	))
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"bridge":  map[string]any{"br0": map[string]any{"name": "br0"}},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "supported: netlink")
}

// VALIDATES: ValidateBackendFeaturesJSON parses a raw JSON string then
//
//	delegates to the core walker.
//
// PREVENTS: Duplicate JSON-parse logic at every component call site.
func TestValidateBackendFeaturesJSON(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{"bridge": {"netlink"}})
	data := `{"interface":{"backend":"vpp","bridge":{"br0":{"name":"br0"}}}}`
	errs := ValidateBackendFeaturesJSON(data, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "/interface/bridge")
}

// VALIDATES: Malformed JSON produces one parse error, not a crash.
// PREVENTS: Caller panics on bad section data.
func TestValidateBackendFeaturesJSON_BadJSON(t *testing.T) {
	schema := buildSyntheticIfaceSchema(map[string][]string{"bridge": {"netlink"}})
	errs := ValidateBackendFeaturesJSON("{{not-json}", schema, "interface", "netlink", "/interface/backend")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "parse")
}

// VALIDATES: Walker evaluates list entries independently. A per-case
//
//	override inside entry A does NOT suppress the list-level
//	rejection for entry B.
//
// PREVENTS: regression of the "cross-entry descendant-accepts leak" where
//
//	one accepting entry masked unsupported features in sibling
//	entries.
func TestValidateBackendFeatures_PerEntryIndependence(t *testing.T) {
	// Tunnel list annotated netlink-only. The `vxlan` case accepts both
	// netlink and vpp; other cases (here: a plain container without
	// override) carry no override.
	schema := NewSchema()
	tunnel := &ListNode{
		KeyType: TypeString,
		KeyName: "name",
		Backend: []string{"netlink"},
		children: map[string]Node{
			"name": Leaf(TypeString),
			"vxlan": &ContainerNode{
				children: map[string]Node{"id": Leaf(TypeUint32)},
				order:    []string{"id"},
				Backend:  []string{"netlink", "vpp"},
			},
			"gre": &ContainerNode{
				children: map[string]Node{"key": Leaf(TypeUint32)},
				order:    []string{"key"},
			},
		},
		order: []string{"name", "vxlan", "gre"},
	}
	schema.Define("interface", Container(
		Field("backend", Leaf(TypeString)),
		Field("tunnel", tunnel),
	))

	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"tunnel": map[string]any{
				"t-ok":  map[string]any{"name": "t-ok", "vxlan": map[string]any{"id": "42"}},
				"t-bad": map[string]any{"name": "t-bad", "gre": map[string]any{"key": "7"}},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1, "t-bad must emit one error; t-ok is accepted via vxlan override: %v", errs)
	msg := errs[0].Error()
	assert.Contains(t, msg, "/interface/tunnel/t-bad")
	assert.NotContains(t, msg, "/interface/tunnel/t-ok")
}

// VALIDATES: Narrowest-wins applies to reject-at-inner-suppresses-reject-at-outer,
//
//	not only accept-at-inner-suppresses-reject-at-outer.
//
// PREVENTS: double-emission when a YANG author places rejecting annotations
//
//	at two nested levels (e.g. list + inner container); user should
//	see one error from the narrowest annotation, not two.
func TestValidateBackendFeatures_NestedRejectsDoNotDoubleEmit(t *testing.T) {
	schema := NewSchema()
	schema.Define("interface", Container(
		Field("backend", Leaf(TypeString)),
		Field("bridge", &ListNode{
			KeyType: TypeString,
			KeyName: "name",
			Backend: []string{"netlink"},
			children: map[string]Node{
				"name": Leaf(TypeString),
				"stp": &ContainerNode{
					children: map[string]Node{"enabled": Leaf(TypeBool)},
					order:    []string{"enabled"},
					Backend:  []string{"netlink"}, // nested reject
				},
			},
			order: []string{"name", "stp"},
		}),
	))
	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"bridge": map[string]any{
				"br0": map[string]any{
					"name": "br0",
					"stp":  map[string]any{"enabled": "true"},
				},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1, "nested reject must emit exactly once at the narrowest path: %v", errs)
	assert.Contains(t, errs[0].Error(), "/interface/bridge/br0/stp")
	assert.NotContains(t, errs[0].Error(), "only the narrowest path, not /interface/bridge")
}

// VALIDATES: InlineListNode carries ze:backend through schema build and
//
//	the walker descends into entries with per-entry emission.
//
// PREVENTS: regression in yangToInlineListWithKey dropping the Backend
//
//	field, or the walker missing the *InlineListNode case.
func TestValidateBackendFeatures_InlineListNodeAnnotation(t *testing.T) {
	inline := InlineList(TypePrefix,
		Field("next-hop", Leaf(TypeIP)),
	)
	inline.Backend = []string{"netlink"}

	schema := NewSchema()
	schema.Define("interface", Container(
		Field("backend", Leaf(TypeString)),
		Field("route", inline),
	))

	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"route": map[string]any{
				"10.0.0.0/8": map[string]any{"next-hop": "192.0.2.1"},
			},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1, "inline-list with rejecting annotation must emit per-entry error")
	assert.Contains(t, errs[0].Error(), "/interface/route/10.0.0.0/8")
	assert.Contains(t, errs[0].Error(), "netlink")
}

// VALIDATES: FlexNode carries ze:backend through schema build and the
//
//	walker both reads the own annotation and recurses into
//	children.
//
// PREVENTS: regression in yangToFlex dropping the Backend field or
//
//	backendAnnotation missing the *FlexNode case.
func TestValidateBackendFeatures_FlexNodeAnnotation(t *testing.T) {
	flex := Flex(
		Field("enabled", Leaf(TypeBool)),
	)
	flex.Backend = []string{"netlink"}

	schema := NewSchema()
	schema.Define("interface", Container(
		Field("backend", Leaf(TypeString)),
		Field("feature", flex),
	))

	tree := map[string]any{
		"interface": map[string]any{
			"backend": "vpp",
			"feature": map[string]any{"enabled": "true"},
		},
	}
	errs := ValidateBackendFeatures(tree, schema, "interface", "vpp", "/interface/backend")
	require.Len(t, errs, 1, "flex with rejecting annotation must emit at its own path")
	assert.Contains(t, errs[0].Error(), "/interface/feature")
	assert.Contains(t, errs[0].Error(), "netlink")
}

// VALIDATES: Multiple ze:backend statements on the same YANG entry are
//
//	unioned, not "first wins" or "last wins".
//
// PREVENTS: schema-author surprise when a grouping uses ze:backend and the
//
//	enclosing node adds another.
func TestGetBackendExtension_MergesMultipleStatements(t *testing.T) {
	entry := &gyang.Entry{
		Exts: []*gyang.Statement{
			{Keyword: "ze:backend", Argument: "netlink"},
			{Keyword: "ze:backend", Argument: "vpp netlink"},
		},
	}
	got := getBackendExtension(entry)
	assert.Equal(t, []string{"netlink", "vpp"}, got,
		"multiple ze:backend statements must merge with dedup in first-seen order")
}

// TestBackendExtensionNames_AllAnnotationsNameKnownBackends walks every
// loaded YANG entry, collects every ze:backend argument, and asserts each
// name is one of the backends registered by the components that ship in
// ze today. Catches typos (e.g. "netfilter" instead of "nft") at test
// time rather than at commit time when a user hits a silently-unreachable
// annotation. Also catches an annotation left over after a backend name
// is renamed.
//
// VALIDATES: spec-backend-feature-gate AC-7, AC-10.
// PREVENTS: silent mis-annotation that lets a feature slip through the
//
//	commit-time gate on every backend.
func TestBackendExtensionNames_AllAnnotationsNameKnownBackends(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	// Known backends today. Update this when a new backend is registered.
	// iface: netlink, vpp ; firewall: nft ; traffic: tc.
	knownBackends := map[string]bool{
		"netlink": true,
		"vpp":     true,
		"nft":     true,
		"tc":      true,
	}

	seen := map[string][]string{} // name -> paths carrying it
	for _, modName := range loader.ModuleNames() {
		entry := loader.GetEntry(modName)
		if entry == nil {
			continue
		}
		walkForAnnotations(entry, modName, seen)
	}
	for name, paths := range seen {
		assert.Truef(t, knownBackends[name],
			"ze:backend %q on %v references an unknown backend; update knownBackends or fix the annotation",
			name, paths)
	}
}

// walkForAnnotations descends entry's YANG tree collecting every ze:backend
// argument into seen, mapped back to the tree path that carried it. Test-only
// helper so the consistency check can report where a bad name came from.
func walkForAnnotations(entry *gyang.Entry, path string, seen map[string][]string) {
	if entry == nil {
		return
	}
	for _, ext := range entry.Exts {
		if ext.Keyword != "ze:backend" && !strings.HasSuffix(ext.Keyword, ":backend") {
			continue
		}
		for name := range strings.FieldsSeq(ext.Argument) {
			seen[name] = append(seen[name], path)
		}
	}
	for childName, child := range entry.Dir {
		walkForAnnotations(child, path+"/"+childName, seen)
	}
}

// buildSyntheticIfaceSchema returns a schema with an "interface" container
// holding a `backend` leaf and one child list per entry in listAnnotations.
// Lists with empty annotation slice are unrestricted; lists with a
// non-empty slice carry ze:backend with the given supporting-backend names.
// Used by walker tests to avoid loading the real YANG.
func buildSyntheticIfaceSchema(listAnnotations map[string][]string) *Schema {
	s := NewSchema()

	fields := []FieldDef{
		Field("backend", Leaf(TypeString)),
		Field("ethernet", &ListNode{
			KeyType:  TypeString,
			KeyName:  "name",
			children: map[string]Node{"name": Leaf(TypeString)},
			order:    []string{"name"},
		}),
	}
	for _, name := range []string{"bridge", "tunnel", "wireguard", "veth"} {
		l := &ListNode{
			KeyType:  TypeString,
			KeyName:  "name",
			children: map[string]Node{"name": Leaf(TypeString)},
			order:    []string{"name"},
		}
		if ann, ok := listAnnotations[name]; ok {
			l.Backend = ann
		}
		fields = append(fields, Field(name, l))
	}
	s.Define("interface", Container(fields...))
	return s
}
