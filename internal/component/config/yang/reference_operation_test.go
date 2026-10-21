// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: reference_path.go — pathContext.child, dataParent, rootStepsFix
// Related: reference_namespace_test.go — the namespace checks this matrix crosses

package yang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessReason is why a matrix row's path loads or is refused.
type accessReason int

const (
	accessUnspecified accessReason = iota
	accessOK                       // the path names a leaf of the accessible tree
	accessTree                     // a step names no node of the accessible tree, or climbs above the root
	accessState                    // a configuration leaf requires an instance of state (Section 9.9)
	accessNamespace                // an unprefixed name takes the referring node's namespace (Section 6.4.1)
	accessPrefix                   // the prefix names a module that does not hold the node (Section 6.4)
)

// accessReasonText answers the refusal text that carries each reason. A
// refusal carries one of them.
func accessReasonText(reason accessReason) []string {
	switch reason {
	case accessTree:
		return []string{"names no node", "climbs"}
	case accessState:
		return []string{"is state"}
	case accessNamespace:
		return []string{"has no prefix, so it names module", "an unprefixed name names a node of module"}
	case accessPrefix:
		return []string{`prefix "g"`}
	case accessUnspecified, accessOK:
		return nil
	}
	return nil
}

// accessSkeleton is module a's data tree, with one slot per accessible-tree
// case of RFC 7950 Section 6.4.1: configuration (cfg), state (st), a
// top-level notification (note), a notification nested in a data node (nn),
// rpc input and output (rpcin, rpcout) and action input and output (actin,
// actout). The referring leaf goes in one slot.
const accessSkeleton = `container cfg { leaf x { type string; } {cfg} }
    container st { config false; leaf x { type string; } {st} }
    list lst { key k; leaf k { type string; } leaf v { type string; } }
    notification note { leaf x { type string; } {note} }
    container box { leaf bx { type string; }
      notification nn { leaf x { type string; } {nn} }
      action act {
        input { leaf x { type string; } leaf ix { type string; } {actin} }
        output { leaf x { type string; } leaf ox { type string; } {actout} } } }
    rpc op {
      input { leaf x { type string; } leaf ix { type string; } {rpcin} }
      output { leaf x { type string; } leaf ox { type string; } {rpcout} } }`

// accessSlots maps each slot to its schema node identifier in module a.
var accessSlots = map[string]string{
	"cfg": "/a:cfg", "st": "/a:st", "note": "/a:note", "nn": "/a:box/a:nn",
	"rpcin": "/a:op/a:input", "rpcout": "/a:op/a:output",
	"actin": "/a:box/a:act/a:input", "actout": "/a:box/a:act/a:output",
}

// accessFill answers the skeleton with fill in slot and every other slot empty.
func accessFill(slot, fill string) string {
	text := accessSkeleton
	for name := range accessSlots {
		content := ""
		if name == slot {
			content = fill
		}
		text = strings.ReplaceAll(text, "{"+name+"}", content)
	}
	return text
}

const accessHeadA = `module a { yang-version 1.1; namespace "urn:ze:test:a"; prefix a; `

// accessOrigin writes the referring leaf of a matrix row into the module set
// one way: in module a itself, in a submodule of a, augmented in by module b,
// from a grouping of module g used in a, or given by a deviation of module d.
type accessOrigin struct {
	name string
	// prefix is what a path written by this origin calls module a by.
	prefix string
	// unprefixed is the reason an unprefixed path is refused from this
	// origin, accessOK when it takes module a's namespace.
	unprefixed accessReason
	// prefixed is the reason a path prefixed with prefix is refused,
	// accessOK when prefix names module a.
	prefixed accessReason
	modules  func(slot, path string) []string
}

func accessLeaf(path string) string {
	return `leaf r { type leafref { path "` + path + `"; } }`
}

var accessOrigins = []accessOrigin{
	{name: "plain", prefix: "a", unprefixed: accessOK, prefixed: accessOK,
		modules: func(slot, path string) []string {
			return []string{accessHeadA + accessFill(slot, accessLeaf(path)) + ` }`}
		}},
	{name: "submodule", prefix: "a", unprefixed: accessOK, prefixed: accessOK,
		modules: func(slot, path string) []string {
			return []string{accessHeadA + `include s; }`,
				`submodule s { yang-version 1.1; belongs-to a { prefix a; } ` + accessFill(slot, accessLeaf(path)) + ` }`}
		}},
	{name: "augment", prefix: "a", unprefixed: accessNamespace, prefixed: accessOK,
		modules: func(slot, path string) []string {
			return []string{accessHeadA + accessFill(slot, "") + ` }`,
				`module b { yang-version 1.1; namespace "urn:ze:test:b"; prefix b; import a { prefix a; }
                augment "` + accessSlots[slot] + `" { ` + accessLeaf(path) + ` } }`}
		}},
	{name: "grouping", prefix: "g", unprefixed: accessOK, prefixed: accessPrefix,
		modules: func(slot, path string) []string {
			return []string{`module g { yang-version 1.1; namespace "urn:ze:test:g"; prefix g;
                grouping grp { ` + accessLeaf(path) + ` } }`,
				accessHeadA + `import g { prefix g; } ` + accessFill(slot, "uses g:grp;") + ` }`}
		}},
	{name: "deviation", prefix: "a", unprefixed: accessOK, prefixed: accessOK,
		modules: func(slot, path string) []string {
			return []string{accessHeadA + accessFill(slot, `leaf r { type string; }`) + ` }`,
				`module d { yang-version 1.1; namespace "urn:ze:test:d"; prefix d; import a { prefix a; }
                deviation "` + accessSlots[slot] + `/a:r" { deviate replace { type leafref { path "` + path + `"; } } } }`}
		}},
}

// accessRows are the paths each slot's referring leaf tries, written with
// "{p}" where a prefix goes, and the reason each loads or is refused when the
// leaf is written in module a.
var accessRows = []struct {
	slot   string
	path   string
	reason accessReason
}{
	// Configuration: "The root node has all top-level configuration data
	// nodes in all modules as children." No rpc, action or notification.
	{"cfg", "/{p}cfg/{p}x", accessOK},
	{"cfg", "../{p}x", accessOK},
	{"cfg", "../../{p}cfg/{p}x", accessOK},
	{"cfg", "../../../{p}x", accessTree},
	{"cfg", "/{p}st/{p}x", accessState},
	{"cfg", "/{p}op/{p}x", accessTree},
	{"cfg", "/{p}note/{p}x", accessTree},
	{"cfg", "/{p}box/{p}nn/{p}x", accessTree},
	{"cfg", "/{p}box/{p}act/{p}x", accessTree},
	{"cfg", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// State: "The root node has all top-level data nodes in all modules as
	// children."
	{"st", "/{p}cfg/{p}x", accessOK},
	{"st", "/{p}st/{p}x", accessOK},
	{"st", "../{p}x", accessOK},
	{"st", "/{p}op/{p}x", accessTree},
	{"st", "/{p}note/{p}x", accessTree},
	{"st", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// A top-level notification: "the root node has the node representing
	// the notification being defined and all top-level data nodes".
	{"note", "/{p}note/{p}x", accessOK},
	{"note", "../{p}x", accessOK},
	{"note", "../../{p}cfg/{p}x", accessOK},
	{"note", "/{p}st/{p}x", accessOK},
	{"note", "/{p}op/{p}x", accessTree},
	{"note", "/{p}box/{p}nn/{p}x", accessTree},
	{"note", "../../../{p}x", accessTree},
	{"note", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// A nested notification: "Otherwise, the root node has all top-level
	// data nodes in all modules as children." The instance holds the
	// hierarchy of its ancestors (Section 7.16.2), so the notification is a
	// child of box and of no other node.
	{"nn", "../{p}x", accessOK},
	{"nn", "../../{p}bx", accessOK},
	{"nn", "/{p}box/{p}nn/{p}x", accessOK},
	{"nn", "/{p}box/{p}bx", accessOK},
	{"nn", "/{p}note/{p}x", accessTree},
	{"nn", "/{p}nn/{p}x", accessTree},
	{"nn", "../../../{p}cfg/{p}x", accessOK},
	{"nn", "../../../../{p}x", accessTree},
	{"nn", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// Rpc input: "for an RPC, the root node also has the node representing
	// the RPC operation being defined as a child.  The node representing the
	// operation being defined has the operation's input parameters as
	// children." Probes L5, P1 and P3.
	{"rpcin", "/{p}op/{p}x", accessOK},
	{"rpcin", "/{p}op/{p}ix", accessOK},
	{"rpcin", "../{p}ix", accessOK},
	{"rpcin", "/{p}op/{p}ox", accessTree},
	{"rpcin", "../{p}ox", accessTree},
	{"rpcin", "/{p}op/{p}input/{p}x", accessTree},
	{"rpcin", "../../{p}cfg/{p}x", accessOK},
	{"rpcin", "/{p}cfg/{p}x", accessOK},
	{"rpcin", "/{p}st/{p}x", accessOK},
	{"rpcin", "/{p}note/{p}x", accessTree},
	{"rpcin", "../../../{p}x", accessTree},
	{"rpcin", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// Rpc output: "...has the operation's output parameters as children."
	// Probe P2.
	{"rpcout", "/{p}op/{p}x", accessOK},
	{"rpcout", "/{p}op/{p}ox", accessOK},
	{"rpcout", "../{p}ox", accessOK},
	{"rpcout", "/{p}op/{p}ix", accessTree},
	{"rpcout", "../{p}ix", accessTree},
	{"rpcout", "/{p}op/{p}output/{p}x", accessTree},
	{"rpcout", "../../{p}cfg/{p}x", accessOK},
	{"rpcout", "/{p}cfg/{p}x", accessOK},
	{"rpcout", "/{p}st/{p}x", accessOK},
	{"rpcout", "/{p}note/{p}x", accessTree},
	{"rpcout", "../../../{p}x", accessTree},
	{"rpcout", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// Action input: the root has top-level data nodes only, and the action
	// is a child of box carrying its input parameters (Section 7.15.2).
	// Probes Q1 ("../../bx") and Q2 ("../../../bx").
	{"actin", "../{p}x", accessOK},
	{"actin", "../{p}ix", accessOK},
	{"actin", "../{p}ox", accessTree},
	{"actin", "../../{p}bx", accessOK},
	{"actin", "../../../{p}bx", accessTree},
	{"actin", "/{p}box/{p}act/{p}ix", accessOK},
	{"actin", "/{p}box/{p}act/{p}input/{p}x", accessTree},
	{"actin", "/{p}act/{p}x", accessTree},
	{"actin", "/{p}op/{p}x", accessTree},
	{"actin", "../../../{p}cfg/{p}x", accessOK},
	{"actin", "../../../../{p}x", accessTree},
	{"actin", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},

	// Action output.
	{"actout", "../{p}x", accessOK},
	{"actout", "../{p}ox", accessOK},
	{"actout", "../{p}ix", accessTree},
	{"actout", "../../{p}bx", accessOK},
	{"actout", "../../../{p}bx", accessTree},
	{"actout", "/{p}box/{p}act/{p}ox", accessOK},
	{"actout", "/{p}box/{p}act/{p}output/{p}x", accessTree},
	{"actout", "/{p}act/{p}x", accessTree},
	{"actout", "/{p}op/{p}x", accessTree},
	{"actout", "../../../{p}cfg/{p}x", accessOK},
	{"actout", "../../../../{p}x", accessTree},
	{"actout", "/{p}lst[{p}k = current()/../{p}x]/{p}v", accessOK},
}

// TestLeafrefAccessibleTreeMatrix walks every accessible-tree case RFC 7950
// Section 6.4.1 enumerates (configuration, state, top-level and nested
// notification, rpc input and output, action input and output), each written
// from module a, a submodule of a, an augment in module b, a grouping of
// module g used in a, and a deviation in module d, with absolute, relative
// and current() paths, unprefixed and prefixed. Each row loads or is refused
// with its reason: the tree reason of the row, or the reason its origin gives
// the prefix form (an augmenting module's unprefixed name is in its own
// namespace; a grouping's own prefix names the grouping's module).
//
// VALIDATES: leafrefTarget, pathStart, keyExprTarget and topLevelParents over
// the accessible tree, through Loader.Resolve (RFC 7950 Section 6.4.1, every
// accessible-tree paragraph; Section 9.9.2: the context node is the node the
// path is defined for).
// PREVENTS: an rpc or action path resolved one level off because input and
// output were read as data nodes (probes Q1, Q2, L5, P1-P3), and an rpc,
// action or notification read as a top-level data node.
func TestLeafrefAccessibleTreeMatrix(t *testing.T) {
	for _, origin := range accessOrigins {
		for _, row := range accessRows {
			for _, prefixed := range []bool{false, true} {
				prefix, extra := "", origin.unprefixed
				if prefixed {
					prefix, extra = origin.prefix+":", origin.prefixed
				}
				path := strings.ReplaceAll(row.path, "{p}", prefix)
				t.Run(origin.name+"/"+row.slot+"/"+path, func(t *testing.T) {
					err := loadModuleTexts(t, origin.modules(row.slot, path)...)
					if row.reason == accessOK && extra == accessOK {
						require.NoError(t, err)
						return
					}
					require.Error(t, err)
					texts := append(accessReasonText(row.reason), accessReasonText(extra)...)
					assert.True(t, containsAny(err.Error(), texts), "refusal %q carries none of %q", err.Error(), texts)
				})
			}
		}
	}
}

// containsAny reports whether text holds any of subs.
func containsAny(text string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(text, sub) {
			return true
		}
	}
	return false
}

// TestLeafrefRootFixNamesEveryImport loads an unprefixed path whose nodes
// belong to two modules the referring module imports neither of (probe
// N13c). The refusal names both imports, and following it loads.
//
// VALIDATES: rootStepsFix collects every missing import (RFC 7950 Section
// 6.4: "all prefixes used MUST be present in the XPath context").
// PREVENTS: a fix that names one import, so following it meets a second
// refusal (probe P5).
func TestLeafrefRootFixNamesEveryImport(t *testing.T) {
	const zb = `module ze-b-conf { namespace "urn:ze:test:zb"; prefix zb; container bsec { container in { leaf y { type string; } } } }`
	const zc = `module ze-c-conf { namespace "urn:ze:test:zc"; prefix zc; import ze-b-conf { prefix zb; }
        augment "/zb:bsec/zb:in" { leaf z { type string; } } }`
	err := loadModuleTexts(t, `module ze-a-conf { namespace "urn:ze:test:za"; prefix za;
        leaf r { type leafref { path "/bsec/in/z"; } } }`, zb, zc)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `add "import ze-b-conf { prefix zb; }" and "import ze-c-conf { prefix zc; }" to module ze-a-conf and write "/zb:bsec/zb:in/zc:z"`)

	require.NoError(t, loadModuleTexts(t, `module ze-a-conf { namespace "urn:ze:test:za"; prefix za;
        import ze-b-conf { prefix zb; } import ze-c-conf { prefix zc; }
        leaf r { type leafref { path "/zb:bsec/zb:in/zc:z"; } } }`, zb, zc))
}

// TestLeafrefRootFixHolderIsReferringModule loads "/zb:sec/own", where only
// the referring module's own sec holds own (probe L11). The refusal says the
// referring module holds the path, without claiming an unprefixed name named
// another module, and following it loads.
//
// VALIDATES: rootStepsFix wording when the holder is ctx.namespace.
// PREVENTS: a refusal that contradicts itself.
func TestLeafrefRootFixHolderIsReferringModule(t *testing.T) {
	const zb = `module ze-b-conf { namespace "urn:ze:test:zb"; prefix zb; container sec { leaf y { type string; } } }`
	const head = `module ze-a-conf { namespace "urn:ze:test:za"; prefix za; import ze-b-conf { prefix zb; }
        container sec { leaf own { type string; } } leaf r { type leafref { path `
	err := loadModuleTexts(t, zb, head+`"/zb:sec/own"; } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `module ze-a-conf, the module of the referring node, holds this path: write "/za:sec/za:own"`)
	assert.NotContains(t, err.Error(), "and module ze-a-conf holds")

	require.NoError(t, loadModuleTexts(t, zb, head+`"/za:sec/za:own"; } } }`))
}

// TestLeafrefOtherInstanceOfOperationGrouping loads an action from a
// grouping used in two containers, whose input leaf names the action of the
// other container. Only the action being defined is in the accessible tree,
// so the use in box1 is refused, while a path to its own parameter loads.
//
// VALIDATES: pathContext.child matches the operation being defined by its
// place in the schema, not by the statement both uses share (RFC 7950
// Section 6.4.1: "the root node has top-level data nodes in all modules as
// children"; Section 7.15.2, the action instance holds one action).
// PREVENTS: another instance of the same action read as the operation being
// defined.
func TestLeafrefOtherInstanceOfOperationGrouping(t *testing.T) {
	module := func(path string) string {
		return `module m { yang-version 1.1; namespace "urn:ze:test:m"; prefix m;
            grouping ga { action act { input { leaf x { type string; }
              leaf r { type leafref { path "` + path + `"; } } } } }
            container box1 { uses ga; } container box2 { uses ga; } }`
	}
	err := loadModuleTexts(t, module("/box2/act/x"))
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), "/m/box1/act/input/r")
	assert.Contains(t, err.Error(), `names no node "act"`)

	require.NoError(t, loadModuleTexts(t, module("../x")))
}

// TestLeafrefOperationOutsideTreeNamed loads paths whose step names an rpc,
// an action or a notification that is not the operation the referring leaf
// is defined in. Each is refused, and the refusal says what the step names
// and why the accessible tree does not hold it, rather than that the module's
// top level holds no node of that name.
//
// VALIDATES: leafrefTarget names the operation a step reaches outside the
// accessible tree (RFC 7950 Section 6.4.1: "Additionally, for an RPC, the
// root node also has the node representing the RPC operation being defined
// as a child.").
// PREVENTS: a refusal claiming module a's top level holds no "op2" when it
// holds the rpc op2.
func TestLeafrefOperationOutsideTreeNamed(t *testing.T) {
	const head = `module a { yang-version 1.1; namespace "urn:ze:test:a"; prefix a;
        rpc op2 { input { leaf x { type string; } } }
        notification note { leaf x { type string; } }
        container box { action act { input { leaf x { type string; } } } } `
	cases := []struct {
		name, body, want string
	}{
		{"rpc from rpc", `rpc op1 { input { leaf r { type leafref { path "/op2/x"; } } } }`, `"op2" is an rpc`},
		{"rpc prefixed", `rpc op1 { input { leaf r { type leafref { path "/a:op2/a:x"; } } } }`, `"a:op2" is an rpc`},
		{"notification from config", `container cfg { leaf r { type leafref { path "/note/x"; } } }`, `"note" is a notification`},
		{"action from config", `container cfg { leaf r { type leafref { path "/box/act/x"; } } }`, `"act" is an action`},
		{"rpc relative", `container cfg { leaf r { type leafref { path "../../op2/x"; } } }`, `"op2" is an rpc`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadModuleTexts(t, head+tc.body+` }`)
			require.ErrorIs(t, err, ErrLeafrefPath)
			assert.Contains(t, err.Error(), tc.want)
			assert.Contains(t, err.Error(), "only the operation the referring leaf is defined in (RFC 7950 Section 6.4.1)")
			assert.NotContains(t, err.Error(), "holds none called")
		})
	}
}
