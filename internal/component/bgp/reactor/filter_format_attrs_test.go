// Design: docs/architecture/api/process-protocol.md -- the filter text protocol
// Related: filter_format.go -- appendSingleAttr, appendAllAttrs, the producers under test
// Related: filter_chain.go -- policyAttr* names, the vocabulary asserted here

package reactor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// attrsFixtureSubject is the subject every advertised attribute renders into,
// in the order appendAllAttrs fixes. Each pair is written as the AppendText of
// its own type produces it (internal/core/bgp/attribute/text_append.go), so a
// value-shape change is a red here rather than a silent contract change for
// every text-mode filter.
const attrsFixtureSubject = "origin incomplete " +
	"as-path [65001 65002] " +
	"next-hop 10.0.0.1 " +
	"med 100 " +
	"local-preference 150 " +
	"atomic-aggregate " +
	"aggregator 65000:4.4.4.4 " +
	"community 65000:100 " +
	"originator-id 3.3.3.3 " +
	"cluster-list 1.1.1.1 2.2.2.2 " +
	"extended-community 0002fde800000064 " +
	"aigp 42 " +
	"large-community 65000:1:2"

// attrsFixtureWire builds one AttributesWire carrying every attribute
// attrNameToCode advertises, on a session that negotiated four-octet AS
// numbers. It is the UPDATE a filter chain is handed, not a hand-written
// subject: the defect this file exists to catch survived for the life of the
// function precisely because every test read a string a human typed.
func attrsFixtureWire(t *testing.T) *attribute.AttributesWire {
	t.Helper()

	aggregator := make([]byte, 8)
	binary.BigEndian.PutUint32(aggregator, 65000)
	copy(aggregator[4:], []byte{4, 4, 4, 4})

	aigp := []byte{0x01, 0x00, 0x0B, 0, 0, 0, 0, 0, 0, 0, 42}

	largeCommunity := make([]byte, 12)
	binary.BigEndian.PutUint32(largeCommunity[0:], 65000)
	binary.BigEndian.PutUint32(largeCommunity[4:], 1)
	binary.BigEndian.PutUint32(largeCommunity[8:], 2)

	return as4FilterWire(t, true,
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrOrigin, []byte{2}),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath, as4FilterPath4(65001, 65002)),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrMED, []byte{0, 0, 0, 100}),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrLocalPref, []byte{0, 0, 0, 150}),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrAtomicAggregate, nil),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAggregator, aggregator),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrCommunity,
			[]byte{0xFD, 0xE8, 0x00, 0x64}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrOriginatorID, []byte{3, 3, 3, 3}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrClusterList,
			[]byte{1, 1, 1, 1, 2, 2, 2, 2}),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrExtCommunity,
			[]byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrAIGP, aigp),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrLargeCommunity,
			largeCommunity),
	)
}

// TestFilterSubjectNamesEveryAdvertisedAttribute renders an UPDATE carrying all
// thirteen attributes attrNameToCode advertises, through the arm every runtime
// caller reaches (an empty declared list).
//
// VALIDATES: AC-1 -- an attribute name Ze advertises appears in the subject
// whenever the UPDATE carries that attribute, with the value shape its own
// AppendText produces and in the order appendAllAttrs fixes.
// PREVENTS: an advertised name that reaches no filter. appendSingleAttr type-
// switches on the boxed attribute, so an arm naming a type no parser builds can
// never match and drops the attribute in silence. The loop over attrNameToCode
// is what makes a fourteenth name visible here rather than silently absent.
func TestFilterSubjectNamesEveryAdvertisedAttribute(t *testing.T) {
	subject := string(AppendAttrsForFilter(nil, attrsFixtureWire(t), nil))

	for name := range attrNameToCode {
		assert.Contains(t, subject, name+" ",
			"the subject must name every attribute attrNameToCode advertises")
	}
	assert.Equal(t, attrsFixtureSubject, subject)
}

// TestFilterSubjectDeclaredArmNamesTheSameFive renders the same UPDATE through
// the declared-attribute arm of AppendAttrsForFilter, naming the five the
// pointer arms dropped.
//
// VALIDATES: AC-2 -- both arms share one renderer, so a filter that declares
// its attributes reads the same values as a filter that takes them all.
// PREVENTS: a fix applied to appendAllAttrs alone. The two arms differ only in
// which codes they walk; the rendering is one function and must stay one.
func TestFilterSubjectDeclaredArmNamesTheSameFive(t *testing.T) {
	declared := []string{
		policyAttrOrigin,
		policyAttrMED,
		policyAttrLocalPreference,
		policyAttrAtomicAggregate,
		policyAttrClusterList,
	}

	subject := string(AppendAttrsForFilter(nil, attrsFixtureWire(t), declared))

	assert.Equal(t, "origin incomplete med 100 local-preference 150 "+
		"atomic-aggregate cluster-list 1.1.1.1 2.2.2.2", subject)
	for _, absent := range []string{policyAttrASPath, policyAttrNextHop, policyAttrCommunity} {
		assert.NotContains(t, subject, absent,
			"the declared arm must render the declared names and nothing else")
	}
}

// TestFilterSubjectNamesClusterList renders an UPDATE whose CLUSTER_LIST holds
// two identifiers that are not in ascending order.
//
// VALIDATES: AC-12 -- the subject names cluster-list followed by dotted decimal
// identifiers in WIRE order, with no brackets, which is the shape RFC 4456
// Section 8 puts on the wire and the shape parseFilterAttrs reads back.
// PREVENTS: a route reflector policy reasoning about the clusters a route
// already traversed being handed nothing at all, and a renderer that sorts or
// brackets the list changing which cluster the filter believes came first.
func TestFilterSubjectNamesClusterList(t *testing.T) {
	attrs := as4FilterWire(t, true,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrClusterList,
			[]byte{9, 9, 9, 9, 1, 1, 1, 1}),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, nil))

	assert.Equal(t, "next-hop 10.0.0.1 cluster-list 9.9.9.9 1.1.1.1", subject)
}

// filterUnnamedAttr is an attribute whose concrete type no arm of
// appendSingleAttr names. Nothing in knownAttrParsers builds one today, which
// is the point: the switch is over an interface, so no compile-time
// exhaustiveness check can exist and only a default arm can report the miss.
type filterUnnamedAttr struct{}

func (filterUnnamedAttr) Code() attribute.AttributeCode   { return attribute.AttrOrigin }
func (filterUnnamedAttr) Flags() attribute.AttributeFlags { return attribute.FlagTransitive }
func (filterUnnamedAttr) Len() int                        { return 0 }
func (filterUnnamedAttr) WriteTo(_ []byte, _ int) int     { return 0 }
func (filterUnnamedAttr) WriteToWithContext(_ []byte, _ int, _, _ *bgpctx.EncodingContext) int {
	return 0
}

// TestAppendSingleAttrWarnsOnUnnamedType hands appendSingleAttr an attribute no
// switch arm names.
//
// VALIDATES: AC-3 -- nothing is appended, the first flag is untouched so the
// separator discipline survives, and one warning names the type.
// PREVENTS: the silence that let five arms name a type no parser builds for the
// whole life of the function. The chain still runs to a verdict on the subject
// it has, which is the degrade-and-speak shape asPathForFilter already uses in
// this file; what must not survive is reaching that verdict with nothing said.
func TestAppendSingleAttrWarnsOnUnnamedType(t *testing.T) {
	var sink bytes.Buffer
	prev := fwdLogger
	fwdLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	t.Cleanup(func() { fwdLogger = prev })

	buf, first := appendSingleAttr([]byte("origin igp"), filterUnnamedAttr{}, false)

	assert.Equal(t, "origin igp", string(buf), "an unnamed type must not leave a dangling separator")
	assert.False(t, first, "the first flag reports what was written, and nothing was")
	assert.Contains(t, sink.String(), filterUnnamedAttrPhrase)
	assert.Contains(t, sink.String(), "filterUnnamedAttr", "the line must name the type it dropped")

	// The control: a named type says nothing at all, so the line above is a
	// signal rather than one more token on every UPDATE.
	sink.Reset()
	buf, first = appendSingleAttr(nil, attribute.MED(100), true)
	assert.Equal(t, "med 100", string(buf))
	assert.False(t, first)
	assert.Empty(t, sink.String())
}

// TestFilterSubjectCallersPassNoDeclaredList reads every non-test call to
// AppendUpdateForFilter in the REPOSITORY and asserts each passes a nil
// declared list.
//
// The whole repository rather than this package, because AppendUpdateForFilter
// is exported: a caller in any package can narrow the subject, and a scan of
// one directory would answer for none of them. A file is parsed only when its
// bytes hold the name, so the scan costs one read of each source file.
//
// VALIDATES: A-1 -- the render-everything arm builds the chain subject on every
// session, so an attribute added to it reaches every filter at once.
// PREVENTS: a caller narrowing the subject with a declared list. medRemoveHasWork
// reads the metric's presence from the subject and holds no second reading of
// the wire, so a narrowed subject would silently take that gate's answer away.
// The count is asserted too: a fourth caller is a decision this invariant needs
// to be re-taken over, not a line to slip past.
func TestFilterSubjectCallersPassNoDeclaredList(t *testing.T) {
	root, sources := repositorySources(t)

	fset := token.NewFileSet()
	var sites []string
	for _, relative := range sources {
		if strings.HasSuffix(relative, "_test.go") {
			continue
		}
		path := filepath.Join(root, relative)
		source, readErr := os.ReadFile(path) // #nosec G304 -- path is one git ls-files answered.
		if errors.Is(readErr, fs.ErrNotExist) {
			// git tracks the path and the working tree no longer holds it,
			// which several sessions sharing this checkout produce routinely. A
			// file that is not there holds no call site. Every other read error
			// still fails, so this skips one named case rather than all of them.
			continue
		}
		require.NoError(t, readErr)
		if !bytes.Contains(source, []byte("AppendUpdateForFilter")) {
			continue
		}
		file, parseErr := parser.ParseFile(fset, relative, source, 0)
		require.NoError(t, parseErr)

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !callsAppendUpdateForFilter(call) {
				return true
			}
			where := fset.Position(call.Pos()).String()
			sites = append(sites, where)
			require.Len(t, call.Args, 4, "AppendUpdateForFilter takes four arguments at "+where)
			ident, isIdent := call.Args[3].(*ast.Ident)
			require.True(t, isIdent, "the declared list at "+where+" must be the nil literal")
			assert.Equal(t, "nil", ident.Name, "the declared list at "+where+" must be nil")
			return true
		})
	}

	assert.Len(t, sites, 3, "three call sites build the chain subject: %v", sites)
}

// repositorySources returns the repository root and every Go file git holds
// under it, tracked or newly written, with the ignored paths left out.
//
// git rather than a directory walk, and the reason is measured: several
// sessions share this checkout and each keeps a whole copy of the tree under
// tmp/, which a walk reads as twelve more call sites. tmp/ is ignored, so the
// one question git already answers -- what is the repository's own source --
// is the exact scope this scan needs, and it needs no skip list to go stale.
func repositorySources(t *testing.T) (string, []string) {
	t.Helper()
	top, err := exec.CommandContext(t.Context(), "git", "rev-parse", "--show-toplevel").Output()
	require.NoError(t, err, "the scan needs the repository root; a wrong root reports no call site and reads as a pass")
	root := strings.TrimSpace(string(top))

	// Run from the root, so every path git prints is relative to it. Run from
	// the package directory instead and git lists only that directory.
	list := exec.CommandContext(t.Context(), "git", "ls-files", "--cached", "--others", "--exclude-standard", "--", "*.go")
	list.Dir = root
	listed, err := list.Output()
	require.NoError(t, err)
	files := strings.Split(strings.TrimSpace(string(listed)), "\n")
	require.NotEmpty(t, files[0], "git listed no Go file, so this scan proves nothing")
	return root, files
}

// callsAppendUpdateForFilter reports whether call names AppendUpdateForFilter,
// spelled bare inside this package or qualified from another.
func callsAppendUpdateForFilter(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name == "AppendUpdateForFilter"
	case *ast.SelectorExpr:
		return fun.Sel.Name == "AppendUpdateForFilter"
	}
	return false
}
