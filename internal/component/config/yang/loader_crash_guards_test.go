package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

const rfc7950AugmentSection = "RFC 7950 Section 7.17"

// resolveModuleTexts loads every YANG module text into a fresh Loader and
// returns the resolved module set or the first error from parsing or Resolve.
func resolveModuleTexts(t *testing.T, texts ...string) (*Resolved, error) {
	t.Helper()
	loader := NewLoader()
	for i, text := range texts {
		if err := loader.AddModuleFromText(moduleFileName(text, i), text); err != nil {
			return nil, err
		}
	}
	return loader.Resolve()
}

// TestLoaderRefusesCyclicGrouping proves that Loader.Resolve, StageProcess,
// and yang.Join refuse direct and indirect grouping cycles (same module,
// submodule, cross-module import, nested grouping using outer grouping, and
// uses-augment cycles) with ErrCyclicGrouping before goyang runs, while
// accepting acyclic chains, diamond DAGs, shadowed nested groupings, and
// DefaultLoader.
//
// VALIDATES: AC-1, RFC 7950 Section 7.12 grouping cycle detection before goyang ToEntry.
// PREVENTS: unrecoverable goroutine stack overflow in goyang ToEntry on cyclic groupings.
func TestLoaderRefusesCyclicGrouping(t *testing.T) {
	refusedCases := []struct {
		name  string
		texts []string
		wants []string
	}{
		{
			name: "direct self-use",
			texts: []string{`module cyc-self {
  namespace "urn:test:cyc-self";
  prefix cs;
  grouping g {
    uses g;
  }
}`},
			wants: []string{"module cyc-self", "cyc-self.yang:", "grouping g", "g -> g"},
		},
		{
			name: "direct self-use with own prefix",
			texts: []string{`module cyc-self-pfx {
  namespace "urn:test:cyc-self-pfx";
  prefix csp;
  grouping g {
    uses csp:g;
  }
}`},
			wants: []string{"module cyc-self-pfx", "cyc-self-pfx.yang:", "grouping g", "g -> g"},
		},
		{
			name: "indirect cycle same module",
			texts: []string{`module cyc-indirect {
  namespace "urn:test:cyc-indirect";
  prefix ci;
  grouping a {
    uses b;
  }
  grouping b {
    uses c;
  }
  grouping c {
    uses a;
  }
}`},
			wants: []string{"module cyc-indirect", "cyc-indirect.yang:", "grouping c", "a -> b -> c -> a"},
		},
		{
			name: "indirect cycle through container",
			texts: []string{`module cyc-container {
  namespace "urn:test:cyc-container";
  prefix cc;
  grouping a {
    container wrap {
      uses b;
    }
  }
  grouping b {
    uses a;
  }
}`},
			wants: []string{"module cyc-container", "cyc-container.yang:", "a -> b -> a"},
		},
		{
			name: "indirect cycle across included submodule",
			texts: []string{
				`module cyc-main {
  namespace "urn:test:cyc-main";
  prefix cm;
  include cyc-sub;
  grouping a {
    uses b;
  }
}`,
				`submodule cyc-sub {
  belongs-to cyc-main {
    prefix cm;
  }
  grouping b {
    uses a;
  }
}`,
			},
			wants: []string{"cyc-main:a", "cyc-sub:b"},
		},
		{
			name: "indirect cycle across sibling submodules",
			texts: []string{
				`module cyc-sib-main {
  namespace "urn:test:cyc-sib-main";
  prefix csm;
  include cyc-sib-one;
  include cyc-sib-two;
}`,
				`submodule cyc-sib-one {
  belongs-to cyc-sib-main {
    prefix csm;
  }
  grouping a {
    uses b;
  }
}`,
				`submodule cyc-sib-two {
  belongs-to cyc-sib-main {
    prefix csm;
  }
  grouping b {
    uses a;
  }
}`,
			},
			wants: []string{"cyc-sib-one:a", "cyc-sib-two:b"},
		},
		{
			name: "indirect cycle across imported modules",
			texts: []string{
				`module cyc-imp-a {
  namespace "urn:test:cyc-imp-a";
  prefix ca;
  import cyc-imp-b {
    prefix cb;
  }
  grouping a {
    uses cb:b;
  }
}`,
				`module cyc-imp-b {
  namespace "urn:test:cyc-imp-b";
  prefix cb;
  import cyc-imp-a {
    prefix ca;
  }
  grouping b {
    uses ca:a;
  }
}`,
			},
			wants: []string{"cyc-imp-a:a", "cyc-imp-b:b"},
		},
		{
			name: "nested grouping using outer grouping",
			texts: []string{`module cyc-nested {
  namespace "urn:test:cyc-nested";
  prefix cn;
  grouping outer {
    grouping inner {
      uses outer;
    }
    leaf x {
      type string;
    }
  }
}`},
			wants: []string{"module cyc-nested", "cyc-nested.yang:", "outer -> inner -> outer"},
		},
		{
			name: "nested grouping inside container using outer grouping",
			texts: []string{`module cyc-nested-cont {
  namespace "urn:test:cyc-nested-cont";
  prefix cnc;
  grouping outer {
    container wrap {
      grouping inner {
        uses outer;
      }
    }
  }
}`},
			wants: []string{"module cyc-nested-cont", "cyc-nested-cont.yang:", "outer -> inner -> outer"},
		},
		{
			name: "local grouping inside container self-use",
			texts: []string{`module cyc-local-cont {
  namespace "urn:test:cyc-local-cont";
  prefix clc;
  container top {
    grouping local {
      uses local;
    }
  }
}`},
			wants: []string{"module cyc-local-cont", "cyc-local-cont.yang:", "grouping local", "local -> local"},
		},
		{
			name: "uses-augment inside grouping closing cycle",
			texts: []string{`module cyc-uses-aug {
  namespace "urn:test:cyc-uses-aug";
  prefix cua;
  grouping b {
    container sub {
      leaf x {
        type string;
      }
    }
  }
  grouping a {
    uses b {
      augment "sub" {
        uses a;
      }
    }
  }
}`},
			wants: []string{"module cyc-uses-aug", "cyc-uses-aug.yang:", "a -> a"},
		},
	}

	for _, tc := range refusedCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveModuleTexts(t, tc.texts...)
			require.Nil(t, resolved)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrCyclicGrouping)
			for _, want := range tc.wants {
				assert.Contains(t, err.Error(), want)
			}
		})
	}

	t.Run("StageProcess refuses cyclic grouping", func(t *testing.T) {
		loader := NewLoader()
		require.NoError(t, loader.AddModuleFromText("stage-cyc.yang", `module stage-cyc {
  namespace "urn:test:stage-cyc";
  prefix sc;
  grouping g {
    uses g;
  }
}`))
		err := loader.StageProcess()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicGrouping)
	})

	t.Run("Resolve skips post-Process checks on cyclic grouping", func(t *testing.T) {
		resolved, err := resolveModuleTexts(t,
			`module ext-decl {
  namespace "urn:test:ext-decl";
  prefix ed;
  extension tag {
    argument name;
  }
}`,
			`module cyc-no-spurious {
  namespace "urn:test:cyc-no-spurious";
  prefix cns;
  import ext-decl {
    prefix ed;
  }
  typedef base-str {
    type string {
      length "1..20";
    }
  }
  leaf name {
    ed:tag "ok";
    type base-str {
      length "min..10";
    }
  }
  grouping g {
    uses g;
  }
}`)
		require.Nil(t, resolved)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicGrouping)
		assert.NotErrorIs(t, err, ErrUndeclaredExtension)
		assert.NotErrorIs(t, err, ErrLengthOrder)
	})

	t.Run("yang.Join refuses external plugin cyclic grouping", func(t *testing.T) {
		compiled := joinCompiled(t)
		ext := acmeModule("acme-cyc-group-conf", "urn:acme:cyc-group", `module acme-cyc-group-conf {
  namespace "urn:acme:cyc-group";
  prefix acg;
  grouping g {
    uses g;
  }
  container acme-cyc-group {
    uses g;
  }
}`)
		joined, err := Join(compiled, []ExternalModule{ext}, joinValidators())
		require.Nil(t, joined)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicGrouping)
		var refusal *opererr.Error
		require.ErrorAs(t, err, &refusal)
		assert.Contains(t, err.Error(), "acme")
		assert.Contains(t, err.Error(), "acme-cyc-group-conf")
	})

	t.Run("yang.Join multi-module blames cyclic grouping plugin not valid sibling", func(t *testing.T) {
		compiled := joinCompiled(t)
		good := acmeModule("acme-good-len-conf", "urn:acme:good-len", `module acme-good-len-conf {
  namespace "urn:acme:good-len";
  prefix agl;
  typedef base-str {
    type string {
      length "1..20";
    }
  }
  container acme-good-len {
    leaf label {
      type base-str {
        length "min..10";
      }
    }
  }
}`)
		bad := ExternalModule{
			Plugin:    "beta",
			Module:    "beta-cyc-group-conf",
			Namespace: "urn:beta:cyc-group",
			Text: `module beta-cyc-group-conf {
  namespace "urn:beta:cyc-group";
  prefix bcg;
  grouping g {
    uses g;
  }
  container beta-cyc-group {
    uses g;
  }
}`,
		}
		joined, err := Join(compiled, []ExternalModule{good, bad}, joinValidators())
		require.Nil(t, joined)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicGrouping)
		assert.NotErrorIs(t, err, ErrLengthOrder)
		var refusal *opererr.Error
		require.ErrorAs(t, err, &refusal)
		assert.Contains(t, err.Error(), "beta")
		assert.Contains(t, err.Error(), "beta-cyc-group-conf")
		assert.NotContains(t, err.Error(), "acme-good-len-conf")
	})

	acceptedCases := []struct {
		name  string
		texts []string
	}{
		{
			name: "acyclic grouping chain",
			texts: []string{`module ok-chain {
  namespace "urn:test:ok-chain";
  prefix oc;
  grouping base {
    leaf a { type string; }
  }
  grouping mid {
    uses base;
    leaf b { type string; }
  }
  grouping top {
    uses mid;
  }
  container c {
    uses top;
  }
}`},
		},
		{
			name: "diamond grouping DAG",
			texts: []string{`module ok-diamond {
  namespace "urn:test:ok-diamond";
  prefix od;
  grouping shared {
    leaf id { type string; }
  }
  grouping left {
    container l { uses shared; }
  }
  grouping right {
    container r { uses shared; }
  }
  grouping combined {
    uses left;
    uses right;
  }
  container c {
    uses combined;
  }
}`},
		},
		{
			name: "shadowed nested grouping resolves to inner definition",
			texts: []string{`module ok-shadow {
  namespace "urn:test:ok-shadow";
  prefix os;
  grouping g {
    grouping g {
      leaf inner-leaf { type string; }
    }
    uses g;
  }
  container c {
    uses g;
  }
}`},
		},
	}

	for _, tc := range acceptedCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveModuleTexts(t, tc.texts...)
			require.NoError(t, err)
			require.NotNil(t, resolved)
		})
	}

	t.Run("DefaultLoader resolves cleanly", func(t *testing.T) {
		resolved, err := DefaultLoader()
		require.NoError(t, err)
		require.NotNil(t, resolved)
	})
}

// TestLoaderRefusesCyclicIdentity proves that Loader.Resolve, StageProcess,
// and yang.Join refuse direct and indirect identity base cycles (same module,
// submodule, sibling submodules, and cross-module import) with
// ErrCyclicIdentity before goyang runs, while accepting acyclic chains,
// multi-base diamond DAGs, and DefaultLoader.
//
// VALIDATES: AC-2, RFC 7950 Section 7.18.2 identity base cycle detection before goyang addChildren.
// PREVENTS: unrecoverable goroutine stack overflow in goyang addChildren on cyclic identities.
func TestLoaderRefusesCyclicIdentity(t *testing.T) {
	refusedCases := []struct {
		name  string
		texts []string
		wants []string
	}{
		{
			name: "direct self-base",
			texts: []string{`module id-self {
  namespace "urn:test:id-self";
  prefix is;
  identity a {
    base a;
  }
}`},
			wants: []string{"module id-self", "id-self.yang:", "identity a", "a -> a"},
		},
		{
			name: "direct self-base with own prefix",
			texts: []string{`module id-self-pfx {
  namespace "urn:test:id-self-pfx";
  prefix isp;
  identity a {
    base isp:a;
  }
}`},
			wants: []string{"module id-self-pfx", "id-self-pfx.yang:", "identity a", "a -> a"},
		},
		{
			name: "indirect cycle same module",
			texts: []string{`module id-indirect {
  namespace "urn:test:id-indirect";
  prefix ii;
  identity a {
    base b;
  }
  identity b {
    base c;
  }
  identity c {
    base a;
  }
}`},
			wants: []string{"module id-indirect", "id-indirect.yang:", "identity c", "a -> b -> c -> a"},
		},
		{
			name: "indirect cycle across included submodule",
			texts: []string{
				`module id-main {
  namespace "urn:test:id-main";
  prefix im;
  include id-sub;
  identity a {
    base b;
  }
}`,
				`submodule id-sub {
  belongs-to id-main {
    prefix im;
  }
  identity b {
    base a;
  }
}`,
			},
			wants: []string{"id-main:a", "id-sub:b"},
		},
		{
			name: "indirect cycle across sibling submodules",
			texts: []string{
				`module id-sib-main {
  namespace "urn:test:id-sib-main";
  prefix ism;
  include id-sib-one;
  include id-sib-two;
}`,
				`submodule id-sib-one {
  belongs-to id-sib-main {
    prefix ism;
  }
  identity a {
    base b;
  }
}`,
				`submodule id-sib-two {
  belongs-to id-sib-main {
    prefix ism;
  }
  identity b {
    base a;
  }
}`,
			},
			wants: []string{"id-sib-one:a", "id-sib-two:b"},
		},
		{
			name: "indirect cycle across imported modules",
			texts: []string{
				`module id-imp-a {
  namespace "urn:test:id-imp-a";
  prefix ia;
  import id-imp-b {
    prefix ib;
  }
  identity a {
    base ib:b;
  }
}`,
				`module id-imp-b {
  namespace "urn:test:id-imp-b";
  prefix ib;
  import id-imp-a {
    prefix ia;
  }
  identity b {
    base ia:a;
  }
}`,
			},
			wants: []string{"id-imp-a:a", "id-imp-b:b"},
		},
	}

	for _, tc := range refusedCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveModuleTexts(t, tc.texts...)
			require.Nil(t, resolved)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrCyclicIdentity)
			for _, want := range tc.wants {
				assert.Contains(t, err.Error(), want)
			}
		})
	}

	t.Run("StageProcess refuses cyclic identity", func(t *testing.T) {
		loader := NewLoader()
		require.NoError(t, loader.AddModuleFromText("stage-id-cyc.yang", `module stage-id-cyc {
  namespace "urn:test:stage-id-cyc";
  prefix sic;
  identity i {
    base i;
  }
}`))
		err := loader.StageProcess()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicIdentity)
	})

	t.Run("Resolve skips post-Process checks on cyclic identity", func(t *testing.T) {
		resolved, err := resolveModuleTexts(t,
			`module ext-id-decl {
  namespace "urn:test:ext-id-decl";
  prefix eid;
  extension tag {
    argument name;
  }
}`,
			`module id-cyc-no-spurious {
  namespace "urn:test:id-cyc-no-spurious";
  prefix icns;
  import ext-id-decl {
    prefix eid;
  }
  typedef base-str {
    type string {
      length "1..20";
    }
  }
  leaf name {
    eid:tag "ok";
    type base-str {
      length "min..10";
    }
  }
  identity loop {
    base loop;
  }
}`)
		require.Nil(t, resolved)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicIdentity)
		assert.NotErrorIs(t, err, ErrUndeclaredExtension)
		assert.NotErrorIs(t, err, ErrLengthOrder)
	})

	t.Run("yang.Join refuses external plugin cyclic identity", func(t *testing.T) {
		compiled := joinCompiled(t)
		ext := acmeModule("acme-cyc-id-conf", "urn:acme:cyc-id", `module acme-cyc-id-conf {
  namespace "urn:acme:cyc-id";
  prefix aci;
  identity loop {
    base loop;
  }
  container acme-cyc-id {
    leaf kind { type identityref { base loop; } }
  }
}`)
		joined, err := Join(compiled, []ExternalModule{ext}, joinValidators())
		require.Nil(t, joined)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicIdentity)
		var refusal *opererr.Error
		require.ErrorAs(t, err, &refusal)
		assert.Contains(t, err.Error(), "acme")
		assert.Contains(t, err.Error(), "acme-cyc-id-conf")
	})

	t.Run("yang.Join multi-module blames cyclic identity plugin not valid sibling", func(t *testing.T) {
		compiled := joinCompiled(t)
		good := acmeModule("acme-good-id-len-conf", "urn:acme:good-id-len", `module acme-good-id-len-conf {
  namespace "urn:acme:good-id-len";
  prefix agil;
  typedef base-str {
    type string {
      length "1..20";
    }
  }
  container acme-good-id-len {
    leaf label {
      type base-str {
        length "min..10";
      }
    }
  }
}`)
		bad := ExternalModule{
			Plugin:    "beta",
			Module:    "beta-cyc-id-conf",
			Namespace: "urn:beta:cyc-id",
			Text: `module beta-cyc-id-conf {
  namespace "urn:beta:cyc-id";
  prefix bci;
  identity loop {
    base loop;
  }
  container beta-cyc-id {
    leaf kind { type identityref { base loop; } }
  }
}`,
		}
		joined, err := Join(compiled, []ExternalModule{good, bad}, joinValidators())
		require.Nil(t, joined)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCyclicIdentity)
		assert.NotErrorIs(t, err, ErrLengthOrder)
		var refusal *opererr.Error
		require.ErrorAs(t, err, &refusal)
		assert.Contains(t, err.Error(), "beta")
		assert.Contains(t, err.Error(), "beta-cyc-id-conf")
		assert.NotContains(t, err.Error(), "acme-good-id-len-conf")
	})

	acceptedCases := []struct {
		name  string
		texts []string
	}{
		{
			name: "acyclic identity chain",
			texts: []string{`module ok-id-chain {
  namespace "urn:test:ok-id-chain";
  prefix oic;
  identity root;
  identity mid { base root; }
  identity leaf-id { base mid; }
}`},
		},
		{
			name: "multi-base diamond identity DAG",
			texts: []string{`module ok-id-diamond {
  yang-version 1.1;
  namespace "urn:test:ok-id-diamond";
  prefix oid;
  identity root;
  identity left { base root; }
  identity right { base root; }
  identity both {
    base left;
    base right;
  }
}`},
		},
	}

	for _, tc := range acceptedCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveModuleTexts(t, tc.texts...)
			require.NoError(t, err)
			require.NotNil(t, resolved)
		})
	}

	t.Run("DefaultLoader resolves cleanly", func(t *testing.T) {
		resolved, err := DefaultLoader()
		require.NoError(t, err)
		require.NotNil(t, resolved)
	})
}

// TestLoaderRefusesNonAugmentableTarget proves that Loader.Resolve and
// yang.Join refuse augments targeting non-augmentable node kinds (leaf,
// leaf-list, anydata, anyxml, rpc, action) across top-level, cross-module,
// chained, uses-expanded, and uses-local augment forms without panicking,
// while accepting valid augments targeting all seven RFC 7950 Section 7.17
// permitted kinds (container, list, choice, case, input, output, notification).
//
// VALIDATES: AC-3, RFC 7950 Section 7.17 augment target node kind enforcement in goyang.
// PREVENTS: nil-map panic in goyang Entry.merge on leaf/leaf-list targets and silent merging into non-augmentable nodes.
func TestLoaderRefusesNonAugmentableTarget(t *testing.T) {
	refusedCases := []struct {
		name  string
		texts []string
		wants []string
	}{
		{
			name: "top-level augment targeting leaf",
			texts: []string{`module aug-leaf {
  namespace "urn:test:aug-leaf";
  prefix al;
  container c {
    leaf a { type string; }
  }
  augment "/c/a" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-leaf.yang:", "/c/a", "leaf", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting leaf-list",
			texts: []string{`module aug-leaflist {
  namespace "urn:test:aug-leaflist";
  prefix all;
  container c {
    leaf-list ll { type string; }
  }
  augment "/c/ll" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-leaflist.yang:", "/c/ll", "leaf-list", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting anydata",
			texts: []string{`module aug-anydata {
  yang-version 1.1;
  namespace "urn:test:aug-anydata";
  prefix aad;
  container c {
    anydata ad;
  }
  augment "/c/ad" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-anydata.yang:", "/c/ad", "anydata", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting anyxml",
			texts: []string{`module aug-anyxml {
  namespace "urn:test:aug-anyxml";
  prefix aax;
  container c {
    anyxml ax;
  }
  augment "/c/ax" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-anyxml.yang:", "/c/ax", "anyxml", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting rpc",
			texts: []string{`module aug-rpc {
  namespace "urn:test:aug-rpc";
  prefix ar;
  rpc do-op {
    input {
      leaf x { type string; }
    }
  }
  augment "/do-op" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-rpc.yang:", "/do-op", "rpc", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting action with input",
			texts: []string{`module aug-action {
  yang-version 1.1;
  namespace "urn:test:aug-action";
  prefix aa;
  container c {
    action act {
      input {
        leaf x { type string; }
      }
    }
  }
  augment "/c/act" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-action.yang:", "/c/act", "action", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting empty action",
			texts: []string{`module aug-action-empty {
  yang-version 1.1;
  namespace "urn:test:aug-action-empty";
  prefix aae;
  container c {
    action act;
  }
  augment "/c/act" {
    leaf b { type string; }
  }
}`},
			wants: []string{"aug-action-empty.yang:", "/c/act", "action", rfc7950AugmentSection},
		},
		{
			name: "cross-module augment targeting leaf",
			texts: []string{
				`module aug-target-mod {
  namespace "urn:test:aug-target-mod";
  prefix atm;
  container c {
    leaf a { type string; }
  }
}`,
				`module aug-ext-mod {
  namespace "urn:test:aug-ext-mod";
  prefix aem;
  import aug-target-mod { prefix atm; }
  augment "/atm:c/atm:a" {
    leaf b { type string; }
  }
}`,
			},
			wants: []string{"aug-ext-mod.yang:", "/atm:c/atm:a", "leaf", rfc7950AugmentSection},
		},
		{
			name: "chained augment targeting leaf added by earlier augment",
			texts: []string{`module aug-chained {
  namespace "urn:test:aug-chained";
  prefix ac;
  container c;
  augment "/c" {
    leaf added { type string; }
  }
  augment "/c/added" {
    leaf bad { type string; }
  }
}`},
			wants: []string{"aug-chained.yang:", "/c/added", "leaf", rfc7950AugmentSection},
		},
		{
			name: "top-level augment targeting uses-expanded leaf",
			texts: []string{`module aug-uses-expanded {
  namespace "urn:test:aug-uses-expanded";
  prefix aue;
  grouping g {
    leaf from-group { type string; }
  }
  container c {
    uses g;
  }
  augment "/c/from-group" {
    leaf bad { type string; }
  }
}`},
			wants: []string{"aug-uses-expanded.yang:", "/c/from-group", "leaf", rfc7950AugmentSection},
		},
		{
			name: "uses-local augment targeting leaf",
			texts: []string{`module aug-uses-local {
  namespace "urn:test:aug-uses-local";
  prefix aul;
  grouping g {
    leaf a { type string; }
  }
  container c {
    uses g {
      augment "a" {
        leaf bad { type string; }
      }
    }
  }
}`},
			wants: []string{"aug-uses-local.yang:", "augment a", "leaf", rfc7950AugmentSection},
		},
	}

	for _, tc := range refusedCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveModuleTexts(t, tc.texts...)
			require.Nil(t, resolved)
			require.Error(t, err)
			for _, want := range tc.wants {
				assert.Contains(t, err.Error(), want)
			}
		})
	}

	t.Run("yang.Join refuses external plugin augment targeting compiled-in leaf", func(t *testing.T) {
		compiled := joinCompiled(t)
		ext := acmeModule("acme-bad-aug", "urn:acme:bad-aug", `module acme-bad-aug {
  namespace "urn:acme:bad-aug";
  prefix aba;
  import ze-join-base-conf { prefix base; }
  augment "/base:system/base:host" {
    leaf extra { type string; }
  }
}`)
		joined, err := Join(compiled, []ExternalModule{ext}, joinValidators())
		require.Nil(t, joined)
		require.Error(t, err)
		var refusal *opererr.Error
		require.ErrorAs(t, err, &refusal)
		assert.Contains(t, err.Error(), "acme")
		assert.Contains(t, err.Error(), "acme-bad-aug")
		assert.Contains(t, err.Error(), "leaf")
		assert.Contains(t, err.Error(), rfc7950AugmentSection)
	})

	t.Run("all seven RFC 7950 Section 7.17 permitted target kinds succeed", func(t *testing.T) {
		resolved, err := resolveModuleTexts(t, `module aug-permitted {
  yang-version 1.1;
  namespace "urn:test:aug-permitted";
  prefix ap;

  grouping grp {
    container inner {
      leaf base-leaf { type string; }
    }
  }

  container c {
    list item {
      key "id";
      leaf id { type string; }
    }
    choice mode {
      case alpha {
        leaf a { type string; }
      }
    }
    action act {
      input {
        leaf in-a { type string; }
      }
      output {
        leaf out-a { type string; }
      }
    }
    uses grp {
      augment "inner" {
        leaf local-added { type string; }
      }
    }
  }

  rpc explicit-op {
    input {
      leaf in-x { type string; }
    }
    output {
      leaf out-x { type string; }
    }
  }

  rpc implicit-op;

  notification ev {
    leaf msg { type string; }
  }

  augment "/c" {
    leaf added-to-container { type string; }
  }
  augment "/c/item" {
    leaf added-to-list { type string; }
  }
  augment "/c/mode" {
    case beta {
      leaf b { type string; }
    }
  }
  augment "/c/mode/alpha" {
    leaf added-to-case { type string; }
  }
  augment "/explicit-op/input" {
    leaf added-to-explicit-input { type string; }
  }
  augment "/explicit-op/output" {
    leaf added-to-explicit-output { type string; }
  }
  augment "/implicit-op/input" {
    leaf added-to-implicit-input { type string; }
  }
  augment "/implicit-op/output" {
    leaf added-to-implicit-output { type string; }
  }
  augment "/c/act/input" {
    leaf added-to-action-input { type string; }
  }
  augment "/c/act/output" {
    leaf added-to-action-output { type string; }
  }
  augment "/ev" {
    leaf added-to-notification { type string; }
  }
}`)
		require.NoError(t, err)
		require.NotNil(t, resolved)

		mod := resolved.GetEntry("aug-permitted")
		require.NotNil(t, mod)
		assert.NotNil(t, mod.Dir["c"].Dir["added-to-container"], "container augment child present")
		assert.NotNil(t, mod.Dir["c"].Dir["item"].Dir["added-to-list"], "list augment child present")
		assert.NotNil(t, mod.Dir["c"].Dir["mode"].Dir["beta"], "choice augment case present")
		assert.NotNil(t, mod.Dir["c"].Dir["mode"].Dir["alpha"].Dir["added-to-case"], "case augment child present")
		assert.NotNil(t, mod.Dir["c"].Dir["inner"].Dir["local-added"], "uses-local container augment child present")
		assert.NotNil(t, mod.Dir["explicit-op"].RPC.Input.Dir["added-to-explicit-input"], "explicit rpc input augment child present")
		assert.NotNil(t, mod.Dir["explicit-op"].RPC.Output.Dir["added-to-explicit-output"], "explicit rpc output augment child present")
		assert.NotNil(t, mod.Dir["implicit-op"].RPC.Input.Dir["added-to-implicit-input"], "implicit rpc input augment child present")
		assert.NotNil(t, mod.Dir["implicit-op"].RPC.Output.Dir["added-to-implicit-output"], "implicit rpc output augment child present")
		assert.NotNil(t, mod.Dir["c"].Dir["act"].RPC.Input.Dir["added-to-action-input"], "action input augment child present")
		assert.NotNil(t, mod.Dir["c"].Dir["act"].RPC.Output.Dir["added-to-action-output"], "action output augment child present")
		assert.NotNil(t, mod.Dir["ev"].Dir["added-to-notification"], "notification augment child present")
	})

	t.Run("DefaultLoader resolves cleanly", func(t *testing.T) {
		resolved, err := DefaultLoader()
		require.NoError(t, err)
		require.NotNil(t, resolved)
	})
}
